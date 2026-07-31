package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// shipWebhookTestHexKey is a fixed 32-byte AES key for the webhook secret's
// encrypt/decrypt round-trip in these tests only.
const shipWebhookTestHexKey = "b86f5b43fed1f62de1e965486f8e9a6e7dc96e64eab3e955d854648b9666f19d"

// fakeShipWebhookLogistics is a hand-rolled LogisticsClient fake — the
// system boundary HandleShippingWebhook crosses to re-fetch the
// authoritative status (FR-C-12).
type fakeShipWebhookLogistics struct {
	getOrderCalls int
	getOrderFn    func(ctx context.Context, biteshipOrderID string) (Shipment, error)
}

var _ LogisticsClient = (*fakeShipWebhookLogistics)(nil)

func (f *fakeShipWebhookLogistics) GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	return nil, ErrShippingUnavailable
}

func (f *fakeShipWebhookLogistics) CreateOrder(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
	return Shipment{}, errors.New("fakeShipWebhookLogistics: CreateOrder not stubbed")
}

func (f *fakeShipWebhookLogistics) GetOrder(ctx context.Context, biteshipOrderID string) (Shipment, error) {
	f.getOrderCalls++
	if f.getOrderFn != nil {
		return f.getOrderFn(ctx, biteshipOrderID)
	}
	return Shipment{}, errors.New("fakeShipWebhookLogistics: GetOrder not stubbed")
}

// newShippingWebhookTestService builds a Service with a real config.Config
// carrying a ConfigEncryptionKey — unlike newShipOrderTestService's nil cfg,
// HandleShippingWebhook must decrypt biteship_webhook_secret, which
// panics on a nil cfg.
func newShippingWebhookTestService(t *testing.T, logistics LogisticsClient) (*Service, *repository.Repository) {
	t.Helper()
	_, repo := newRealDBService(t)
	cfg := &config.Config{ConfigEncryptionKey: shipWebhookTestHexKey}
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, logistics, nil, cfg, nil)
	return svc, repo
}

// seedWebhookSecret encrypts and stores biteship_webhook_secret directly,
// mirroring what UpdateSystemConfig would persist.
func seedWebhookSecret(t *testing.T, repo *repository.Repository, secret string) {
	t.Helper()
	encrypted, err := EncryptConfigValue(shipWebhookTestHexKey, secret)
	if err != nil {
		t.Fatalf("encrypt webhook secret: %v", err)
	}
	if err := repo.UpsertSystemConfig(context.Background(), "biteship_webhook_secret", encrypted, true); err != nil {
		t.Fatalf("seed webhook secret: %v", err)
	}
}

// seedShippedOrder creates a physical order already shipped via the
// Biteship path, carrying biteshipOrderID, so the webhook can find it via
// GetOrderByBiteshipOrderID.
func seedShippedOrder(t *testing.T, svc *Service, repo *repository.Repository, biteshipOrderID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ('book', $1, 50000, 10, 'published', 500) RETURNING id`,
		"Ship Webhook Test Book "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	studentID := insertCheckoutStudent(t, repo, "Ship Webhook Test Student", "shipwebhook_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if err := repo.SetShippedBiteship(ctx, order.ID, "WB-1", biteshipOrderID); err != nil {
		t.Fatalf("seed shipped order: %v", err)
	}
	return order.ID
}

// shipWebhookBody builds the real Biteship ping shape: order_id plus an
// untrusted status. It deliberately carries no timestamp — production code
// must never source occurred_at from the body (FR-C-12).
func shipWebhookBody(t *testing.T, orderID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"order_id": orderID,
		"status":   "confirmed", // deliberately untrusted — see FR-C-12
	})
	if err != nil {
		t.Fatalf("marshal webhook body: %v", err)
	}
	return body
}

func TestHandleShippingWebhook_WrongSignatureRejectedNothingWritten(t *testing.T) {
	fake := &fakeShipWebhookLogistics{}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	orderID := seedShippedOrder(t, svc, repo, "biteship-wrong-sig")
	ctx := context.Background()

	body := shipWebhookBody(t, "biteship-wrong-sig")
	err := svc.HandleShippingWebhook(ctx, body, "totally-wrong-signature")
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature, got %v", err)
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want 0 GetOrder calls, got %d", fake.getOrderCalls)
	}

	got, gerr := repo.GetOrderByID(ctx, orderID)
	if gerr != nil {
		t.Fatalf("GetOrderByID: %v", gerr)
	}
	if got.ShipmentStatus != nil {
		t.Errorf("want shipment_status untouched, got %v", *got.ShipmentStatus)
	}
	events, everr := repo.ListShipmentEvents(ctx, orderID)
	if everr != nil {
		t.Fatalf("ListShipmentEvents: %v", everr)
	}
	if len(events) != 0 {
		t.Errorf("want 0 shipment events, got %d", len(events))
	}
}

// TestHandleShippingWebhook_EmptySecretRejectsAnyHeader is the fail-closed
// assertion FR-C-11 exists to guard: an unset/empty configured secret must
// reject every request, including a header that happens to be empty too —
// the exact shape of the no-op verifier this repo shipped once (841cd84).
func TestHandleShippingWebhook_EmptySecretRejectsAnyHeader(t *testing.T) {
	fake := &fakeShipWebhookLogistics{}
	svc, repo := newShippingWebhookTestService(t, fake)
	// biteship_webhook_secret intentionally left unset.
	orderID := seedShippedOrder(t, svc, repo, "biteship-empty-secret")
	ctx := context.Background()

	body := shipWebhookBody(t, "biteship-empty-secret")
	for _, header := range []string{"", "anything-at-all"} {
		err := svc.HandleShippingWebhook(ctx, body, header)
		if !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("header %q: want ErrInvalidSignature, got %v", header, err)
		}
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want 0 GetOrder calls, got %d", fake.getOrderCalls)
	}

	got, gerr := repo.GetOrderByID(ctx, orderID)
	if gerr != nil {
		t.Fatalf("GetOrderByID: %v", gerr)
	}
	if got.ShipmentStatus != nil {
		t.Errorf("want shipment_status untouched, got %v", *got.ShipmentStatus)
	}
}

// TestHandleShippingWebhook_RefetchedStatusLandsNotBodyStatus proves FR-C-12:
// the body claims "confirmed" (see shipWebhookBody) but the re-fetch reports
// "delivered" — the re-fetched value must be what lands, never the body's.
func TestHandleShippingWebhook_RefetchedStatusLandsNotBodyStatus(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "delivered", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	orderID := seedShippedOrder(t, svc, repo, "biteship-refetch")
	ctx := context.Background()

	body := shipWebhookBody(t, "biteship-refetch")
	if err := svc.HandleShippingWebhook(ctx, body, "correct-secret"); err != nil {
		t.Fatalf("HandleShippingWebhook: %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.ShipmentStatus == nil || *got.ShipmentStatus != "delivered" {
		t.Fatalf("want shipment_status delivered, got %v", got.ShipmentStatus)
	}
	if got.Status == "completed" {
		t.Error("orders.status must not be auto-advanced by the webhook (FR-C-15)")
	}

	events, err := repo.ListShipmentEvents(ctx, orderID)
	if err != nil {
		t.Fatalf("ListShipmentEvents: %v", err)
	}
	if len(events) != 1 || events[0].Status != "delivered" {
		t.Fatalf("want 1 event with status delivered, got %+v", events)
	}
}

// TestHandleShippingWebhook_ReplayIsInert proves FR-C-13: a replayed webhook
// with identical content must not create a second event row and must still
// report success — the UNIQUE (order_id, status, occurred_at) constraint
// plus ON CONFLICT DO NOTHING makes the second insert a no-op. The body
// carries no timestamp (shipWebhookBody never has one) — occurred_at must
// come from a deterministic fallback, not the request body.
func TestHandleShippingWebhook_ReplayIsInert(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "delivered", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	orderID := seedShippedOrder(t, svc, repo, "biteship-replay")
	ctx := context.Background()

	body := shipWebhookBody(t, "biteship-replay")

	if err := svc.HandleShippingWebhook(ctx, body, "correct-secret"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := svc.HandleShippingWebhook(ctx, body, "correct-secret"); err != nil {
		t.Fatalf("replayed call: want success (inert), got error: %v", err)
	}

	events, err := repo.ListShipmentEvents(ctx, orderID)
	if err != nil {
		t.Fatalf("ListShipmentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want exactly 1 event after replay, got %d", len(events))
	}
}

// TestHandleShippingWebhook_ReplayInertAcrossManyDeliveriesNoBodyTimestamp is
// the case that broke the replay guard before this fix: the webhook body
// carries no timestamp whatsoever (the real Biteship ping shape) and the
// stubbed GetOrder response also has a zero StatusUpdatedAt, so occurredAt
// must fall back to something stable for the order rather than time.Now().
// Delivering the same webhook 3 times must still land exactly 1 event row.
func TestHandleShippingWebhook_ReplayInertAcrossManyDeliveriesNoBodyTimestamp(t *testing.T) {
	fake := &fakeShipWebhookLogistics{
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			// No StatusUpdatedAt set — mirrors an adapter that couldn't parse
			// a timestamp out of Biteship's re-fetch response.
			return Shipment{BiteshipOrderID: biteshipOrderID, Status: "delivered", WaybillID: "WB-1"}, nil
		},
	}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	orderID := seedShippedOrder(t, svc, repo, "biteship-replay-no-ts")
	ctx := context.Background()

	body := shipWebhookBody(t, "biteship-replay-no-ts")

	for i := 0; i < 3; i++ {
		if err := svc.HandleShippingWebhook(ctx, body, "correct-secret"); err != nil {
			t.Fatalf("delivery %d: want success, got error: %v", i+1, err)
		}
	}

	events, err := repo.ListShipmentEvents(ctx, orderID)
	if err != nil {
		t.Fatalf("ListShipmentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want exactly 1 event after 3 identical deliveries with no timestamp anywhere, got %d", len(events))
	}
}

func TestHandleShippingWebhook_UnknownBiteshipOrderIDNotFound(t *testing.T) {
	fake := &fakeShipWebhookLogistics{}
	svc, repo := newShippingWebhookTestService(t, fake)
	seedWebhookSecret(t, repo, "correct-secret")
	ctx := context.Background()

	body := shipWebhookBody(t, "no-such-biteship-order")
	err := svc.HandleShippingWebhook(ctx, body, "correct-secret")
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("want ErrOrderNotFound, got %v", err)
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want 0 GetOrder calls for an unknown order id, got %d", fake.getOrderCalls)
	}
}
