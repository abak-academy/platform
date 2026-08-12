package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/repository"
)

// killShipment reproduces what AdminCancelShipment and the cancelled-parcel
// webhook leave behind: the booking is dead at Biteship, but orders.status
// stays 'shipped' because the status is never walked back (FR-C-15).
func killShipment(t *testing.T, repo *repository.Repository, orderID uuid.UUID, status string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE orders SET shipment_status = $1 WHERE id = $2`, status, orderID,
	); err != nil {
		t.Fatalf("kill shipment: %v", err)
	}
}

func shipOnce(t *testing.T, svc *Service, repo *repository.Repository, fake *fakeShipLogistics) uuid.UUID {
	t.Helper()
	orderID := createShippableOrder(t, svc, repo, "paid", true)
	if err := svc.AdminShipOrder(context.Background(), orderID.String(), "", ""); err != nil {
		t.Fatalf("first AdminShipOrder: %v", err)
	}
	return orderID
}

// The bug: a cancelled booking left the order at 'shipped', and both ship
// paths gated on status alone, so there was no way to put the parcel on
// another courier — the only action still offered was a refund.
func TestAdminShipOrder_rebooksAfterTheBookingIsCancelled(t *testing.T) {
	booked := 0
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			booked++
			if booked == 1 {
				return Shipment{BiteshipOrderID: "biteship-dead", WaybillID: "WB-DEAD", Status: "confirmed"}, nil
			}
			return Shipment{BiteshipOrderID: "biteship-live", WaybillID: "WB-LIVE", Status: "confirmed"}, nil
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	ctx := context.Background()

	orderID := shipOnce(t, svc, repo, fake)
	if fake.lastCreateReq.ReferenceID != orderID.String() {
		t.Fatalf("first booking reference = %q, want the bare order uuid (live bookings must keep resolving)",
			fake.lastCreateReq.ReferenceID)
	}

	killShipment(t, repo, orderID, "cancelled")

	if err := svc.AdminShipOrder(ctx, orderID.String(), "", ""); err != nil {
		t.Fatalf("re-ship after cancellation: %v", err)
	}

	// Biteship rejects a reused reference_id and echoes the conflicting
	// booking back, so re-using the bare uuid would have adopted the DEAD
	// shipment instead of creating a new one.
	if want := orderID.String() + "-2"; fake.lastCreateReq.ReferenceID != want {
		t.Errorf("re-book reference = %q, want %q", fake.lastCreateReq.ReferenceID, want)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.TrackingNumber != "WB-LIVE" {
		t.Errorf("tracking_number = %q, want the new waybill WB-LIVE", got.TrackingNumber)
	}
	if got.BiteshipOrderID == nil || *got.BiteshipOrderID != "biteship-live" {
		t.Errorf("biteship_order_id = %v, want biteship-live", got.BiteshipOrderID)
	}
	if got.ShipmentStatus != nil {
		t.Errorf("shipment_status = %v, want NULL — 'cancelled' described the previous booking, "+
			"and leaving it keeps the order in the failed queue", *got.ShipmentStatus)
	}
	if got.ShipmentAttempt != 2 {
		t.Errorf("shipment_attempt = %d, want 2", got.ShipmentAttempt)
	}
}

// Every failure status is an exit, not just 'cancelled' — a rejected or
// returned parcel is equally undeliverable and equally stuck.
func TestAdminShipOrder_rebooksAfterEveryFailureStatus(t *testing.T) {
	for _, status := range ShipmentFailureStatuses {
		t.Run(status, func(t *testing.T) {
			fake := &fakeShipLogistics{
				createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
					return Shipment{BiteshipOrderID: "bs-" + status, WaybillID: "WB-" + status, Status: "confirmed"}, nil
				},
			}
			svc, repo := newShipOrderTestService(t, fake)
			orderID := shipOnce(t, svc, repo, fake)
			killShipment(t, repo, orderID, status)

			if err := svc.AdminShipOrder(context.Background(), orderID.String(), "", ""); err != nil {
				t.Fatalf("re-ship after %q: %v", status, err)
			}
		})
	}
}

// The other half of the guard: opening re-ship for dead parcels must not open
// it for live ones. Booking a second courier for a parcel already in transit
// is a real-world dispatch nobody asked for.
func TestAdminShipOrder_refusesWhileTheShipmentIsAlive(t *testing.T) {
	for _, status := range []string{"", "confirmed", "allocated", "picking_up", "in_transit", "on_hold", "delivered"} {
		t.Run("status="+status, func(t *testing.T) {
			fake := &fakeShipLogistics{
				createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
					return Shipment{BiteshipOrderID: "bs-live", WaybillID: "WB-live", Status: "confirmed"}, nil
				},
			}
			svc, repo := newShipOrderTestService(t, fake)
			orderID := shipOnce(t, svc, repo, fake)
			if status != "" {
				killShipment(t, repo, orderID, status)
			}

			before := fake.createOrderCalls
			err := svc.AdminShipOrder(context.Background(), orderID.String(), "", "")
			if !errors.Is(err, ErrOrderNotShippable) {
				t.Fatalf("re-ship with shipment_status %q: want ErrOrderNotShippable, got %v", status, err)
			}
			if fake.createOrderCalls != before {
				t.Errorf("a refused re-ship must not reach the courier: %d extra CreateOrder calls",
					fake.createOrderCalls-before)
			}
		})
	}
}

// The manual escape hatch (FR-C-9) was blocked by the same guard, which is why
// a dead shipment had no exit at all: the automated path could not re-book and
// the human override could not either.
func TestAdminShipOrderManual_rebooksAfterTheBookingIsCancelled(t *testing.T) {
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			return Shipment{BiteshipOrderID: "biteship-dead", WaybillID: "WB-DEAD", Status: "confirmed"}, nil
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	ctx := context.Background()

	orderID := shipOnce(t, svc, repo, fake)
	killShipment(t, repo, orderID, "courier_not_found")

	if err := svc.AdminShipOrderManual(ctx, orderID.String(), "MANUAL-9"); err != nil {
		t.Fatalf("manual re-ship after cancellation: %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.TrackingNumber != "MANUAL-9" {
		t.Errorf("tracking_number = %q, want MANUAL-9", got.TrackingNumber)
	}
	if got.WaybillSource == nil || *got.WaybillSource != "manual" {
		t.Errorf("waybill_source = %v, want manual", got.WaybillSource)
	}
	// The dead Biteship id must go, or a late webhook for the cancelled
	// booking would resolve to this order and overwrite the manual resi.
	if got.BiteshipOrderID != nil && *got.BiteshipOrderID != "" {
		t.Errorf("biteship_order_id = %v, want cleared", *got.BiteshipOrderID)
	}
	if got.ShipmentStatus != nil {
		t.Errorf("shipment_status = %v, want NULL", *got.ShipmentStatus)
	}
}

// A re-book that fails must not burn its attempt number. If it did, a retry
// after a network timeout on a booking Biteship actually created would send a
// fresh reference_id, skip the duplicate check, and dispatch a second courier.
func TestAdminShipOrder_failedRebookRetriesTheSameReference(t *testing.T) {
	failNext := false
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			if failNext {
				return Shipment{}, errors.New("biteship unreachable")
			}
			return Shipment{BiteshipOrderID: "biteship-dead", WaybillID: "WB-DEAD", Status: "confirmed"}, nil
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	ctx := context.Background()

	orderID := shipOnce(t, svc, repo, fake)
	killShipment(t, repo, orderID, "cancelled")

	failNext = true
	if err := svc.AdminShipOrder(ctx, orderID.String(), "", ""); err == nil {
		t.Fatal("re-book should have failed")
	}
	firstRef := fake.lastCreateReq.ReferenceID

	if err := svc.AdminShipOrder(ctx, orderID.String(), "", ""); err == nil {
		t.Fatal("second re-book should have failed too")
	}
	if fake.lastCreateReq.ReferenceID != firstRef {
		t.Errorf("retry reference = %q, want the same %q — a fresh reference would book a second courier",
			fake.lastCreateReq.ReferenceID, firstRef)
	}
}
