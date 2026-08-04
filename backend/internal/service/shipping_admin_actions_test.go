package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestAdminRefreshShipment_LandsTheRefetchedStatus covers the manual pull an
// admin needs when a webhook never arrives: the same re-fetch the webhook
// performs, triggered by hand.
func TestAdminRefreshShipment_LandsTheRefetchedStatus(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "delivered", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	orderID := seedShippedOrder(t, svc, repo, "biteship-refresh")
	ctx := context.Background()

	if err := svc.AdminRefreshShipment(ctx, orderID.String()); err != nil {
		t.Fatalf("AdminRefreshShipment: %v", err)
	}
	if fake.getOrderCalls != 1 {
		t.Fatalf("want exactly 1 GetOrder call, got %d", fake.getOrderCalls)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.ShipmentStatus == nil || *got.ShipmentStatus != "delivered" {
		t.Fatalf("want shipment_status delivered, got %v", got.ShipmentStatus)
	}
	if got.Status == "completed" {
		t.Error("a manual refresh must not advance orders.status any more than the webhook does (FR-C-15)")
	}
}

// Pressing refresh twice must not produce a second identical timeline entry —
// the button is the one place a human can trivially generate a replay.
func TestAdminRefreshShipment_IsIdempotentAcrossPresses(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "delivered", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	orderID := seedShippedOrder(t, svc, repo, "biteship-refresh-twice")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := svc.AdminRefreshShipment(ctx, orderID.String()); err != nil {
			t.Fatalf("press %d: %v", i+1, err)
		}
	}

	events, err := repo.ListShipmentEvents(ctx, orderID)
	if err != nil {
		t.Fatalf("ListShipmentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event after 3 presses, got %d", len(events))
	}
}

// An order shipped by the manual escape hatch has no Biteship order to ask
// about. Refusing is the honest answer; GetOrder("") would address nothing.
func TestAdminRefreshShipment_RefusesOrderWithNoBiteshipBooking(t *testing.T) {
	fake := &fakeShipWebhookLogistics{}
	svc, repo := newShippingWebhookTestService(t, fake)
	ctx := context.Background()

	orderID := seedShippedOrder(t, svc, repo, "biteship-to-be-cleared")
	if err := repo.SetShippedManual(ctx, orderID, "MANUAL-123"); err != nil {
		t.Fatalf("SetShippedManual: %v", err)
	}
	// SetShippedManual leaves biteship_order_id in place on this row, so clear
	// it explicitly to model an order that was never booked through Biteship.
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET biteship_order_id = NULL WHERE id = $1`, orderID); err != nil {
		t.Fatalf("clear biteship_order_id: %v", err)
	}

	err := svc.AdminRefreshShipment(ctx, orderID.String())
	if !errors.Is(err, ErrNoBiteshipBooking) {
		t.Fatalf("want ErrNoBiteshipBooking, got %v", err)
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want 0 GetOrder calls, got %d", fake.getOrderCalls)
	}
}

// TestAdminCancelShipment_CancelsThenSyncs proves the cancel action does both
// halves: it tells Biteship, and it lands whatever Biteship then reports —
// rather than assuming the cancellation took and writing "cancelled" locally.
func TestAdminCancelShipment_CancelsThenSyncs(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "cancelled"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	orderID := seedShippedOrder(t, svc, repo, "biteship-cancel")
	ctx := context.Background()

	if err := svc.AdminCancelShipment(ctx, orderID.String(), "salah alamat"); err != nil {
		t.Fatalf("AdminCancelShipment: %v", err)
	}

	if fake.cancelCalls != 1 {
		t.Fatalf("want exactly 1 CancelOrder call, got %d", fake.cancelCalls)
	}
	if fake.cancelledID != "biteship-cancel" {
		t.Errorf("cancelled the wrong Biteship order: %q", fake.cancelledID)
	}
	if fake.cancelReason != "salah alamat" {
		t.Errorf("reason not forwarded, got %q", fake.cancelReason)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.ShipmentStatus == nil || *got.ShipmentStatus != "cancelled" {
		t.Fatalf("want shipment_status cancelled, got %v", got.ShipmentStatus)
	}
	// Cancelling a shipment is not refunding an order (#72). Walking
	// orders.status back is a separate decision that needs a human.
	if got.Status != "shipped" {
		t.Errorf("orders.status must be left alone by a shipment cancel, got %q", got.Status)
	}
}

// If Biteship refuses the cancellation, nothing local may claim it happened.
func TestAdminCancelShipment_FailureLeavesTheOrderUntouched(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		cancelErr: errors.New("biteship says no"),
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	orderID := seedShippedOrder(t, svc, repo, "biteship-cancel-fails")
	ctx := context.Background()

	if err := svc.AdminCancelShipment(ctx, orderID.String(), "berubah pikiran"); err == nil {
		t.Fatal("want an error when Biteship refuses the cancellation")
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("must not sync after a failed cancel, got %d GetOrder calls", fake.getOrderCalls)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.ShipmentStatus != nil {
		t.Errorf("want shipment_status untouched, got %v", *got.ShipmentStatus)
	}
}

// TestGetOrderTracking_PrefersTheCourierScanLog proves the dialog shows the
// carrier's own checkpoints when they are available, newest first.
func TestGetOrderTracking_PrefersTheCourierScanLog(t *testing.T) {
	early := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 8, 3, 17, 30, 0, 0, time.UTC)
	fake := &fakeShipWebhookLogistics{
		trackFn: func(ctx context.Context, waybillID, courierCode string) (WaybillTracking, error) {
			return WaybillTracking{
				Status: "delivered",
				History: []TrackingEntry{
					{Status: "picked", OccurredAt: early, Note: "Diambil dari gudang"},
					{Status: "delivered", OccurredAt: late, Note: "Diterima Budi"},
				},
			}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	orderID := seedShippedOrder(t, svc, repo, "biteship-tracking-view")
	ctx := context.Background()
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET courier_code = 'jne', selected_courier = 'JNE' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("seed courier code: %v", err)
	}

	view, err := svc.GetOrderTracking(ctx, orderID.String())
	if err != nil {
		t.Fatalf("GetOrderTracking: %v", err)
	}
	if view.Source != "courier" {
		t.Errorf("source = %q, want courier", view.Source)
	}
	if view.Status != "delivered" {
		t.Errorf("status = %q, want delivered", view.Status)
	}
	if len(view.History) != 2 || view.History[0].Status != "delivered" {
		t.Fatalf("want newest checkpoint first, got %+v", view.History)
	}
	if view.Waybill != "WB-1" {
		t.Errorf("waybill = %q, want the order's resi", view.Waybill)
	}
}

// When the carrier cannot answer, the parcel is still describable from the
// events the webhook recorded — degrading is better than an empty dialog, but
// it must be labelled so nobody reads a thin history as the carrier's.
func TestGetOrderTracking_FallsBackToStoredEventsAndSaysSo(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		trackFn: func(ctx context.Context, waybillID, courierCode string) (WaybillTracking, error) {
			return WaybillTracking{}, errors.New("courier unreachable")
		},
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "in_transit", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	orderID := seedShippedOrder(t, svc, repo, "biteship-tracking-fallback")
	ctx := context.Background()
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET courier_code = 'jne' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("seed courier code: %v", err)
	}
	if err := svc.HandleShippingWebhook(ctx, shipWebhookBody(t, "biteship-tracking-fallback"), "correct-secret"); err != nil {
		t.Fatalf("seed an event: %v", err)
	}

	view, err := svc.GetOrderTracking(ctx, orderID.String())
	if err != nil {
		t.Fatalf("GetOrderTracking: %v", err)
	}
	if view.Source != "local" {
		t.Errorf("source = %q, want local", view.Source)
	}
	if len(view.History) == 0 {
		t.Error("want the stored events rather than an empty dialog")
	}
}

// An order with no resi has nothing to track, and saying so is better than
// opening an empty dialog.
func TestGetOrderTracking_RefusesAnOrderWithNoWaybill(t *testing.T) {
	fake := &fakeShipWebhookLogistics{}
	svc, repo := newShippingWebhookTestService(t, fake)
	ctx := context.Background()
	orderID := seedShippedOrder(t, svc, repo, "biteship-tracking-nowaybill")
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET tracking_number = '' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("clear tracking number: %v", err)
	}

	if _, err := svc.GetOrderTracking(ctx, orderID.String()); !errors.Is(err, ErrNoTrackingNumber) {
		t.Fatalf("want ErrNoTrackingNumber, got %v", err)
	}
}
