package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// fakeShipLogistics is a hand-rolled LogisticsClient fake — the one system
// boundary AdminShipOrder/AdminShipOrderManual cross. storeRepo stays real
// Postgres (see realdb_test.go's rationale: it's a concrete type, not an
// interface, so the order/status assertions below run against the actual
// repository methods from Task 9).
type fakeShipLogistics struct {
	createOrderCalls int
	getOrderCalls    int
	lastCreateReq    CreateShipmentRequest

	createOrderFn func(ctx context.Context, req CreateShipmentRequest) (Shipment, error)
	getOrderFn    func(ctx context.Context, biteshipOrderID string) (Shipment, error)
}

var _ LogisticsClient = (*fakeShipLogistics)(nil)

func (f *fakeShipLogistics) GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	return nil, ErrShippingUnavailable
}

func (f *fakeShipLogistics) CreateOrder(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
	f.createOrderCalls++
	f.lastCreateReq = req
	if f.createOrderFn != nil {
		return f.createOrderFn(ctx, req)
	}
	return Shipment{}, errors.New("fakeShipLogistics: CreateOrder not stubbed")
}

func (f *fakeShipLogistics) GetOrder(ctx context.Context, biteshipOrderID string) (Shipment, error) {
	f.getOrderCalls++
	if f.getOrderFn != nil {
		return f.getOrderFn(ctx, biteshipOrderID)
	}
	return Shipment{}, errors.New("fakeShipLogistics: GetOrder not stubbed")
}

// seedSenderConfig writes the four sender fields AdminShipOrder reads out of
// system_config. Idempotent (ON CONFLICT DO UPDATE), safe to call per test
// against the shared real-DB fixture.
func seedSenderConfig(t *testing.T, repo *repository.Repository) {
	t.Helper()
	ctx := context.Background()
	vals := map[string]string{
		"app_name":          "Toko Contoh",
		"app_contact_phone": "081200000000",
		"app_address":       "Jl. Contoh No. 1",
		"app_kode_pos":      "12345",
	}
	for k, v := range vals {
		if err := repo.UpsertSystemConfig(ctx, k, v, false); err != nil {
			t.Fatalf("seed system config %s: %v", k, err)
		}
	}
}

func newShipOrderTestService(t *testing.T, logistics LogisticsClient) (*Service, *repository.Repository) {
	t.Helper()
	_, repo := newRealDBService(t)
	seedSenderConfig(t, repo)
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, logistics, nil, nil)
	return svc, repo
}

// createShippableOrder builds a real cart with one physical item through the
// normal MintCart/AddItem path, then seeds status/address/courier fields
// directly via SQL to land it in the precondition AdminShipOrder expects
// (checkout itself is out of scope here).
func createShippableOrder(t *testing.T, svc *Service, repo *repository.Repository, status string, withCourier bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ('book', $1, 50000, 10, 'published', 500) RETURNING id`,
		"Ship Order Test Book "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	studentID := insertCheckoutStudent(t, repo, "Ship Order Test Student", "shiptest_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	addr := `{"penerima":"Budi Test","telepon":"081200000000","alamat":"Jl. Contoh No. 1","kode_pos":"12345"}`
	courierCode, serviceCode := "", ""
	if withCourier {
		courierCode, serviceCode = "jne", "reg"
	}
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = $1, shipping_address = $2::jsonb,
		     courier_code = NULLIF($3, ''), courier_service_code = NULLIF($4, '')
		 WHERE id = $5`,
		status, addr, courierCode, serviceCode, order.ID,
	); err != nil {
		t.Fatalf("seed order state: %v", err)
	}

	return order.ID
}

func TestAdminShipOrder_HappyPath(t *testing.T) {
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			return Shipment{BiteshipOrderID: "biteship-abc", WaybillID: "WB-123", Status: "confirmed"}, nil
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", true)
	ctx := context.Background()

	if err := svc.AdminShipOrder(ctx, orderID.String()); err != nil {
		t.Fatalf("AdminShipOrder: %v", err)
	}
	if fake.createOrderCalls != 1 {
		t.Errorf("want 1 CreateOrder call, got %d", fake.createOrderCalls)
	}
	if fake.lastCreateReq.CourierCode != "jne" || fake.lastCreateReq.ServiceCode != "reg" {
		t.Errorf("want request booked under jne/reg, got %s/%s", fake.lastCreateReq.CourierCode, fake.lastCreateReq.ServiceCode)
	}
	if fake.lastCreateReq.DestinationPostalCode != "12345" {
		t.Errorf("want destination postal code 12345, got %q", fake.lastCreateReq.DestinationPostalCode)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.Status != "shipped" {
		t.Errorf("want status shipped, got %q", got.Status)
	}
	if got.TrackingNumber != "WB-123" {
		t.Errorf("want tracking number WB-123, got %q", got.TrackingNumber)
	}
	if got.BiteshipOrderID == nil || *got.BiteshipOrderID != "biteship-abc" {
		t.Errorf("want biteship_order_id biteship-abc, got %v", got.BiteshipOrderID)
	}
	if got.WaybillSource == nil || *got.WaybillSource != "biteship" {
		t.Errorf("want waybill_source biteship, got %v", got.WaybillSource)
	}
}

// TestAdminShipOrder_DuplicateBookingRetryConverges covers FR-C-7: Biteship
// reports the pickup as already booked, so the code adopts the echoed id via
// GetOrder instead of booking a second one. A second AdminShipOrder call
// (simulating an admin retry) must find the order already shipped and bail
// out on the status guard — zero further CreateOrder calls, identical end
// state.
func TestAdminShipOrder_DuplicateBookingRetryConverges(t *testing.T) {
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			return Shipment{}, &ShipmentAlreadyBookedError{ExistingBiteshipOrderID: "existing-xyz"}
		},
		getOrderFn: func(ctx context.Context, biteshipOrderID string) (Shipment, error) {
			return Shipment{BiteshipOrderID: biteshipOrderID, WaybillID: "WB-EXIST", Status: "confirmed"}, nil
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", true)
	ctx := context.Background()

	if err := svc.AdminShipOrder(ctx, orderID.String()); err != nil {
		t.Fatalf("first AdminShipOrder: %v", err)
	}
	first, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID after first attempt: %v", err)
	}

	err = svc.AdminShipOrder(ctx, orderID.String())
	if !errors.Is(err, ErrOrderNotShippable) {
		t.Fatalf("second AdminShipOrder: want ErrOrderNotShippable, got %v", err)
	}

	if fake.createOrderCalls != 1 {
		t.Errorf("want exactly 1 CreateOrder call across both attempts, got %d", fake.createOrderCalls)
	}
	if fake.getOrderCalls != 1 {
		t.Errorf("want exactly 1 GetOrder call, got %d", fake.getOrderCalls)
	}

	second, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID after second attempt: %v", err)
	}
	if second.Status != "shipped" || second.TrackingNumber != "WB-EXIST" ||
		second.BiteshipOrderID == nil || *second.BiteshipOrderID != "existing-xyz" {
		t.Fatalf("unexpected end state: status=%q tracking=%q biteshipID=%v", second.Status, second.TrackingNumber, second.BiteshipOrderID)
	}
	if second.Status != first.Status || second.TrackingNumber != first.TrackingNumber ||
		*second.BiteshipOrderID != *first.BiteshipOrderID {
		t.Errorf("end state changed after the rejected second attempt: first=%+v second=%+v", first, second)
	}
}

func TestAdminShipOrder_GenericBookingFailureLeavesOrderUntouched(t *testing.T) {
	wantReason := "biteship: courier rejected pickup window"
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			return Shipment{}, errors.New(wantReason)
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", true)
	ctx := context.Background()

	err := svc.AdminShipOrder(ctx, orderID.String())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), wantReason) {
		t.Errorf("want error to contain %q, got %q", wantReason, err.Error())
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.Status != "paid" {
		t.Errorf("want status still paid, got %q", got.Status)
	}
	if got.TrackingNumber != "" {
		t.Errorf("want tracking_number still empty, got %q", got.TrackingNumber)
	}
	if fake.createOrderCalls != 1 {
		t.Errorf("want 1 CreateOrder call, got %d", fake.createOrderCalls)
	}
}

func TestAdminShipOrder_NoCourierCodeRefusedBeforeClientCall(t *testing.T) {
	fake := &fakeShipLogistics{}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", false)

	err := svc.AdminShipOrder(context.Background(), orderID.String())
	if !errors.Is(err, ErrNoCarrierCode) {
		t.Fatalf("want ErrNoCarrierCode, got %v", err)
	}
	if fake.createOrderCalls != 0 {
		t.Errorf("want 0 CreateOrder calls, got %d", fake.createOrderCalls)
	}
}

// TestAdminShipOrder_EmptyExistingIDRefusesRatherThanGetOrderEmpty guards the
// unverified assumption flagged on biteshipOrderErrorResponse (adapter/
// biteship.go): if Biteship's real duplicate-detection field name differs
// from what was assumed, ExistingBiteshipOrderID arrives empty and the code
// must refuse rather than call GetOrder("").
func TestAdminShipOrder_EmptyExistingIDRefusesRatherThanGetOrderEmpty(t *testing.T) {
	fake := &fakeShipLogistics{
		createOrderFn: func(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
			return Shipment{}, &ShipmentAlreadyBookedError{ExistingBiteshipOrderID: ""}
		},
	}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", true)
	ctx := context.Background()

	err := svc.AdminShipOrder(ctx, orderID.String())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want GetOrder never called with an empty id, got %d calls", fake.getOrderCalls)
	}
	got, gerr := repo.GetOrderByID(ctx, orderID)
	if gerr != nil {
		t.Fatalf("GetOrderByID: %v", gerr)
	}
	if got.Status != "paid" {
		t.Errorf("want status still paid, got %q", got.Status)
	}
}

func TestAdminShipOrderManual_StampsManualWithZeroClientCalls(t *testing.T) {
	fake := &fakeShipLogistics{}
	svc, repo := newShipOrderTestService(t, fake)
	orderID := createShippableOrder(t, svc, repo, "paid", false)
	ctx := context.Background()

	if err := svc.AdminShipOrderManual(ctx, orderID.String(), "JNE123456"); err != nil {
		t.Fatalf("AdminShipOrderManual: %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.Status != "shipped" {
		t.Errorf("want status shipped, got %q", got.Status)
	}
	if got.TrackingNumber != "JNE123456" {
		t.Errorf("want tracking number JNE123456, got %q", got.TrackingNumber)
	}
	if got.WaybillSource == nil || *got.WaybillSource != "manual" {
		t.Errorf("want waybill_source manual, got %v", got.WaybillSource)
	}
	if fake.createOrderCalls != 0 || fake.getOrderCalls != 0 {
		t.Errorf("want zero client calls, got create=%d get=%d", fake.createOrderCalls, fake.getOrderCalls)
	}
}
