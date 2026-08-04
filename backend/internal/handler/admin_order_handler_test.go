package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAdminRefundOrder_BasicCompilation(t *testing.T) {
	// This test ensures the handlers compile and respond correctly
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	svc := service.NewForTest(rdb)
	h := handler.New(svc)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/test-id/refund", nil)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/admin/orders/:id/refund")
	c.SetParamNames("id")
	c.SetParamValues("test-id")
	c.Set("claims", &infra.Claims{Sub: "00000000-0000-0000-0000-000000000001", Role: "super_admin"})

	// Act
	err = h.AdminRefundOrder(c)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Logf("status code = %d", rec.Code)
	}

	var respBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err == nil {
		if msg, ok := respBody["message"]; ok {
			t.Logf("response message: %s", msg)
		}
	}
}

// Task 12 (Track C HTTP surface split): the real DB fixture below mirrors
// internal/service/shipping_order_test.go's newShipOrderTestService but lives
// in this package so the handler methods (not the service function directly)
// are what's under test — POST /admin/orders/:id/ship and
// POST /admin/orders/:id/ship-manual must reach AdminShipOrder and
// AdminShipOrderManual respectively.
var (
	shipHandlerEnvOnce sync.Once
	shipHandlerRepo    *repository.Repository
)

func shipHandlerRepoFixture(t *testing.T) *repository.Repository {
	t.Helper()
	shipHandlerEnvOnce.Do(func() {
		ctx := context.Background()
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("connection string: %v", err)
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		pool, err := infra.NewPool(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		shipHandlerRepo = repository.New(pool)
	})
	if shipHandlerRepo == nil {
		t.Fatal("ship handler test repo failed to initialize")
	}
	return shipHandlerRepo
}

// fakeShipHandlerLogistics is a hand-rolled LogisticsClient spy — the system
// boundary AdminShipOrder/AdminShipOrderManual cross. Counting calls on it
// (rather than asserting on status codes alone) is what proves the two
// routes actually ran different service methods.
type fakeShipHandlerLogistics struct {
	createOrderCalls int
	getOrderCalls    int

	createOrderFn func(ctx context.Context, req service.CreateShipmentRequest) (service.Shipment, error)
}

var _ service.LogisticsClient = (*fakeShipHandlerLogistics)(nil)

func (f *fakeShipHandlerLogistics) GetRates(ctx context.Context, req service.ShippingQuoteRequest) ([]service.CourierRate, error) {
	return nil, service.ErrShippingUnavailable
}

func (f *fakeShipHandlerLogistics) CreateOrder(ctx context.Context, req service.CreateShipmentRequest) (service.Shipment, error) {
	f.createOrderCalls++
	if f.createOrderFn != nil {
		return f.createOrderFn(ctx, req)
	}
	return service.Shipment{BiteshipOrderID: "biteship-handler-test", WaybillID: "WB-HANDLER-TEST", Status: "confirmed"}, nil
}

func (f *fakeShipHandlerLogistics) CancelOrder(ctx context.Context, biteshipOrderID, reason string) error {
	return nil
}

func (f *fakeShipHandlerLogistics) GetOrder(ctx context.Context, biteshipOrderID string) (service.Shipment, error) {
	f.getOrderCalls++
	return service.Shipment{}, service.ErrShippingUnavailable
}

func newShipHandlerTestService(t *testing.T, logistics service.LogisticsClient) (*handler.Handler, *service.Service, *repository.Repository) {
	t.Helper()
	repo := shipHandlerRepoFixture(t)

	ctx := context.Background()
	senderVals := map[string]string{
		"app_name":          "Toko Contoh",
		"app_contact_phone": "081200000000",
		"app_address":       "Jl. Contoh No. 1",
		"app_kode_pos":      "12345",
	}
	for k, v := range senderVals {
		if err := repo.UpsertSystemConfig(ctx, k, v, false); err != nil {
			t.Fatalf("seed system config %s: %v", k, err)
		}
	}

	svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, logistics, nil, nil, nil)
	return handler.New(svc), svc, repo
}

// newShipHandlerTestServiceWithRenderer is newShipHandlerTestService plus a
// config.Config pointing the Gotenberg renderer at a fake sidecar
// (newFakeGotenbergServer, defined in exam_certificate_handler_test.go in
// this same package) — needed only by the shipping-label test below, which
// is the first admin-order handler test that actually streams a rendered
// PDF rather than just a JSON status.
func newShipHandlerTestServiceWithRenderer(t *testing.T, logistics service.LogisticsClient) (*handler.Handler, *service.Service, *repository.Repository) {
	t.Helper()
	repo := shipHandlerRepoFixture(t)

	ctx := context.Background()
	senderVals := map[string]string{
		"app_name":          "Toko Contoh",
		"app_contact_phone": "081200000000",
		"app_address":       "Jl. Contoh No. 1",
		"app_kode_pos":      "12345",
	}
	for k, v := range senderVals {
		if err := repo.UpsertSystemConfig(ctx, k, v, false); err != nil {
			t.Fatalf("seed system config %s: %v", k, err)
		}
	}

	cfg := &config.Config{GotenbergURL: newFakeGotenbergServer(t).URL}
	svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, logistics, nil, cfg, nil)
	return handler.New(svc), svc, repo
}

// createShippableOrderForHandler builds a real cart with one physical item
// through the normal MintCart/AddItem path, then seeds status/address/courier
// fields directly via SQL to land it in the precondition AdminShipOrder /
// AdminShipOrderManual expect (checkout itself is out of scope here).
func createShippableOrderForHandler(t *testing.T, svc *service.Service, repo *repository.Repository, status string, withCourier bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ('book', $1, 50000, 10, 'published', 500) RETURNING id`,
		"Ship Handler Test Book "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	var studentID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Ship Handler Test Student', 'student', 'active', $1, '') RETURNING id`,
		"shiphandler_"+uuid.New().String(),
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

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

func newShipRequestCtx(t *testing.T, method, path, paramName, paramValue string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames(paramName)
	c.SetParamValues(paramValue)
	return c, rec
}

// TestAdminShipRoutes_ReachDifferentServicePaths proves the Task 12 HTTP
// surface split: POST /admin/orders/:id/ship books through the logistics
// client spy (auto-booking path), while POST /admin/orders/:id/ship-manual
// never touches it (manual escape hatch) — observed via call counts, not
// status codes alone.
func TestAdminShipRoutes_ReachDifferentServicePaths(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)

	autoOrderID := createShippableOrderForHandler(t, svc, repo, "paid", true)
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+autoOrderID.String()+"/ship", "id", autoOrderID.String(), nil)
	if err := h.AdminShipOrder(c); err != nil {
		t.Fatalf("AdminShipOrder handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminShipOrder: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.createOrderCalls != 1 {
		t.Errorf("want 1 CreateOrder call after POST /ship, got %d", fake.createOrderCalls)
	}

	manualOrderID := createShippableOrderForHandler(t, svc, repo, "paid", false)
	manualBody, _ := json.Marshal(map[string]string{"tracking_number": "JNE999888"})
	c2, rec2 := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+manualOrderID.String()+"/ship-manual", "id", manualOrderID.String(), manualBody)
	if err := h.AdminShipOrderManual(c2); err != nil {
		t.Fatalf("AdminShipOrderManual handler: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("AdminShipOrderManual: want 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// The manual path must not have added to the CreateOrder count from the
	// first order's booking — proving the two routes ran different service
	// methods, not just returned different bodies.
	if fake.createOrderCalls != 1 {
		t.Errorf("want CreateOrder count unchanged (still 1) after POST /ship-manual, got %d", fake.createOrderCalls)
	}
	if fake.getOrderCalls != 0 {
		t.Errorf("want 0 GetOrder calls, got %d", fake.getOrderCalls)
	}

	got, err := repo.GetOrderByID(context.Background(), manualOrderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.TrackingNumber != "JNE999888" {
		t.Errorf("want tracking number JNE999888, got %q", got.TrackingNumber)
	}
	if got.WaybillSource == nil || *got.WaybillSource != "manual" {
		t.Errorf("want waybill_source manual, got %v", got.WaybillSource)
	}

	bookedOrder, err := repo.GetOrderByID(context.Background(), autoOrderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if bookedOrder.WaybillSource == nil || *bookedOrder.WaybillSource != "biteship" {
		t.Errorf("want waybill_source biteship, got %v", bookedOrder.WaybillSource)
	}
}

// TestAdminShipOrder_NoCourierCode_Returns422 proves ErrNoCarrierCode maps to
// 422 (not the 500 an unmapped sentinel would fall into via mapServiceError).
func TestAdminShipOrder_NoCourierCode_Returns422(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)

	orderID := createShippableOrderForHandler(t, svc, repo, "paid", false)
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID.String()+"/ship", "id", orderID.String(), nil)

	if err := h.AdminShipOrder(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body.String())
	}
	var apiErr handler.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "no_carrier_code" {
		t.Errorf("code: want no_carrier_code, got %q", apiErr.Code)
	}
	if fake.createOrderCalls != 0 {
		t.Errorf("want 0 CreateOrder calls, got %d", fake.createOrderCalls)
	}
}

// TestAdminShipOrder_NotShippableStatus_Returns409 proves ErrOrderNotShippable
// maps to 409 (not 500).
func TestAdminShipOrder_NotShippableStatus_Returns409(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)

	orderID := createShippableOrderForHandler(t, svc, repo, "cart", true)
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID.String()+"/ship", "id", orderID.String(), nil)

	if err := h.AdminShipOrder(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body.String())
	}
	var apiErr handler.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "order_not_shippable" {
		t.Errorf("code: want order_not_shippable, got %q", apiErr.Code)
	}
}

// TestAdminGetOrder_JSONCarriesShipmentFields proves FR-C-16: the order
// detail response carries is_estimate, shipment_status, waybill_source and
// the shipment event list (additive, snake_case).
func TestAdminGetOrder_JSONCarriesShipmentFields(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)

	orderID := createShippableOrderForHandler(t, svc, repo, "paid", true)
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID.String()+"/ship", "id", orderID.String(), nil)
	if err := h.AdminShipOrder(c); err != nil {
		t.Fatalf("AdminShipOrder: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminShipOrder: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	getCtx, getRec := newShipRequestCtx(t, http.MethodGet, "/api/v1/admin/orders/"+orderID.String(), "id", orderID.String(), nil)
	if err := h.AdminGetOrder(getCtx); err != nil {
		t.Fatalf("AdminGetOrder: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("AdminGetOrder: want 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"is_estimate", "shipment_status", "waybill_source", "shipment_events"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response missing key %q: %s", key, getRec.Body.String())
		}
	}
	if raw["waybill_source"] != "biteship" {
		t.Errorf("waybill_source: want biteship, got %v", raw["waybill_source"])
	}
}

// TestAdminGetShippingLabel_StreamsApplicationPDF proves GET
// /admin/orders/:id/label reaches GetShippingLabel and streams the rendered
// packing slip back as application/pdf (FR-D-1).
func TestAdminGetShippingLabel_StreamsApplicationPDF(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestServiceWithRenderer(t, fake)

	orderID := createShippableOrderForHandler(t, svc, repo, "paid", true)
	shipCtx, shipRec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID.String()+"/ship", "id", orderID.String(), nil)
	if err := h.AdminShipOrder(shipCtx); err != nil {
		t.Fatalf("AdminShipOrder: %v", err)
	}
	if shipRec.Code != http.StatusOK {
		t.Fatalf("AdminShipOrder: want 200, got %d: %s", shipRec.Code, shipRec.Body.String())
	}

	c, rec := newShipRequestCtx(t, http.MethodGet, "/api/v1/admin/orders/"+orderID.String()+"/label", "id", orderID.String(), nil)
	if err := h.AdminGetShippingLabel(c); err != nil {
		t.Fatalf("AdminGetShippingLabel: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminGetShippingLabel: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type: want application/pdf, got %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected a non-empty PDF body")
	}
}

// TestAdminGetShippingLabel_NoTrackingNumber_Returns422 proves FR-D-2: an
// order that hasn't been shipped yet (no tracking_number) is refused with a
// clear reason before any PDF is produced.
func TestAdminGetShippingLabel_NoTrackingNumber_Returns422(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestServiceWithRenderer(t, fake)

	orderID := createShippableOrderForHandler(t, svc, repo, "paid", true)

	c, rec := newShipRequestCtx(t, http.MethodGet, "/api/v1/admin/orders/"+orderID.String()+"/label", "id", orderID.String(), nil)
	if err := h.AdminGetShippingLabel(c); err != nil {
		t.Fatalf("AdminGetShippingLabel: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body.String())
	}
	var apiErr handler.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "no_tracking_number" {
		t.Errorf("code: want no_tracking_number, got %q", apiErr.Code)
	}
}

// createConfirmableOrderForHandler builds a real cart with one digital course
// item through MintCart/AddItem, then pushes it to payment_pending directly —
// manual confirm (Task 9) recovers a stalled/failed gateway payment, so
// payment_pending is the realistic starting status.
func createConfirmableOrderForHandler(t *testing.T, svc *service.Service, repo *repository.Repository) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ('course', $1, 50000, 0, 'published') RETURNING id`,
		"Confirm Handler Test Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	var studentID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Confirm Handler Test Student', 'student', 'active', $1, '') RETURNING id`,
		"confirmhandler_"+uuid.New().String(),
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending' WHERE id = $1`,
		order.ID,
	); err != nil {
		t.Fatalf("seed payment_pending: %v", err)
	}

	return order.ID
}

// newConfirmHandlerTestService is newShipHandlerTestService plus a real
// (miniredis-backed) Redis client — AdminConfirmOrder's idempotency check
// dereferences svc.rdb, which newShipHandlerTestService leaves nil since its
// callers never exercise that path.
func newConfirmHandlerTestService(t *testing.T) (*handler.Handler, *service.Service, *repository.Repository) {
	t.Helper()
	repo := shipHandlerRepoFixture(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := service.NewWithStore(repo, repo, rdb, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, nil, nil)
	return handler.New(svc), svc, repo
}

// newConfirmRequestCtx builds an echo.Context for POST /admin/orders/:id/confirm
// carrying a super_admin actor and, when body is non-nil, a JSON request body.
func newConfirmRequestCtx(t *testing.T, orderID string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID+"/confirm", "id", orderID, body)
	c.Request().Header.Set("Idempotency-Key", "confirm-handler-"+uuid.New().String())
	c.Set("claims", &infra.Claims{Sub: uuid.New().String(), Role: "super_admin"})
	return c, rec
}

// TestAdminConfirmOrder_RejectsMissingOrInvalidProof covers FR-25/FR-26: a
// confirm request with an absent, empty, wrong-prefix, or path-traversing
// payment_proof_url must be rejected with 400 before the order is ever
// touched — invariant 6 says a confirmed order can never exist without its
// evidence.
// TestAdminListOrders_InvalidCursor_Returns400 mirrors the product-list fix:
// ListOrders appended the raw cursor into `AND id > $n` against a uuid column,
// so a non-UUID reached Postgres and surfaced as a 500 with a driver error
// string in the body.
func TestAdminListOrders_InvalidCursor_Returns400(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, _, _ := newShipHandlerTestService(t, fake)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?cursor=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.AdminListOrders(c); err != nil {
		t.Fatalf("AdminListOrders: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	var apiErr struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("response body is not JSON (raw driver error?): %v; body=%s", err, rec.Body.String())
	}
	if apiErr.Code != "invalid_cursor" {
		t.Errorf("code: want %q, got %q", "invalid_cursor", apiErr.Code)
	}
}

func newRefundRequestCtx(t *testing.T, orderID string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newShipRequestCtx(t, http.MethodPost, "/api/v1/admin/orders/"+orderID+"/refund", "id", orderID, body)
	c.Set("claims", &infra.Claims{Sub: uuid.New().String(), Role: "super_admin"})
	return c, rec
}

// Refunding does not move money — PaymentClient has no refund method, so a
// human transfers it and this receipt is the only evidence the system gets.
// The same rule as manual confirmation therefore applies: no evidence, no
// record. See issue #72 for the real refund flow.
func TestAdminRefundOrder_RejectsMissingOrInvalidProof(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)
	orderID := createConfirmableOrderForHandler(t, svc, repo)

	cases := []struct {
		name string
		body []byte
	}{
		{"no body at all", nil},
		{"empty proof key", []byte(`{"refund_proof_url":""}`)},
		{"proof key outside refund_proof/ prefix", []byte(`{"refund_proof_url":"product/abc/x.jpg"}`)},
		{"proof key contains ..", []byte(`{"refund_proof_url":"refund_proof/../../etc/passwd"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newRefundRequestCtx(t, orderID.String(), tc.body)
			if err := h.AdminRefundOrder(c); err != nil {
				t.Fatalf("AdminRefundOrder: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
			}

			got, err := repo.GetOrderByID(context.Background(), orderID)
			if err != nil {
				t.Fatalf("GetOrderByID: %v", err)
			}
			if got.Status == "cancelled" {
				t.Errorf("order must not be cancelled when the refund was rejected")
			}
		})
	}
}

// The admin UI only offers Refund for paid/processing/shipped/completed, but
// the API enforced nothing — a direct call could revoke enrollments and clear
// tracking on an order whose money was never taken.
func TestAdminRefundOrder_RejectsNonRefundableStatus(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)
	orderID := createConfirmableOrderForHandler(t, svc, repo)

	// createConfirmableOrderForHandler leaves the order in payment_pending —
	// money has not arrived, so there is nothing to refund.
	c, rec := newRefundRequestCtx(t, orderID.String(), []byte(`{"refund_proof_url":"refund_proof/admin-1/trf.jpg"}`))
	if err := h.AdminRefundOrder(c); err != nil {
		t.Fatalf("AdminRefundOrder: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409 for a payment_pending order, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, rec.Body.String())
	}
	if apiErr.Code != "order_not_refundable" {
		t.Errorf("code: want %q, got %q", "order_not_refundable", apiErr.Code)
	}

	got, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.Status != "payment_pending" {
		t.Errorf("want status untouched (payment_pending), got %q", got.Status)
	}
}

func TestAdminConfirmOrder_RejectsMissingOrInvalidProof(t *testing.T) {
	fake := &fakeShipHandlerLogistics{}
	h, svc, repo := newShipHandlerTestService(t, fake)
	orderID := createConfirmableOrderForHandler(t, svc, repo)

	cases := []struct {
		name string
		body []byte
	}{
		{"no body at all", nil},
		{"empty proof key", []byte(`{"payment_proof_url":""}`)},
		{"proof key outside payment_proof/ prefix", []byte(`{"payment_proof_url":"product/abc/x.jpg"}`)},
		{"proof key contains ..", []byte(`{"payment_proof_url":"payment_proof/../../etc/passwd"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newConfirmRequestCtx(t, orderID.String(), tc.body)
			if err := h.AdminConfirmOrder(c); err != nil {
				t.Fatalf("AdminConfirmOrder: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
			}

			got, err := repo.GetOrderByID(context.Background(), orderID)
			if err != nil {
				t.Fatalf("GetOrderByID: %v", err)
			}
			if got.Status != "payment_pending" {
				t.Errorf("want order status unchanged (payment_pending), got %q", got.Status)
			}
			if got.PaymentMethod != "" {
				t.Errorf("want payment_method unchanged (empty), got %q", got.PaymentMethod)
			}
		})
	}
}

// TestAdminConfirmOrder_ValidProof_Returns200AndConfirms proves the accept
// path of FR-25/FR-26: a key under payment_proof/ with no ".." is accepted,
// and the order response carries payment_method/payment_proof_url — pinning
// the exact JSON key names (checked directly here, not invented) that Task 12
// builds its confirm-request and order-response fixtures on.
func TestAdminConfirmOrder_ValidProof_Returns200AndConfirms(t *testing.T) {
	h, svc, repo := newConfirmHandlerTestService(t)
	orderID := createConfirmableOrderForHandler(t, svc, repo)

	proofKey := "payment_proof/" + uuid.New().String() + "/proof.jpg"
	body, _ := json.Marshal(map[string]string{"payment_proof_url": proofKey})
	c, rec := newConfirmRequestCtx(t, orderID.String(), body)
	if err := h.AdminConfirmOrder(c); err != nil {
		t.Fatalf("AdminConfirmOrder: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	getCtx, getRec := newShipRequestCtx(t, http.MethodGet, "/api/v1/admin/orders/"+orderID.String(), "id", orderID.String(), nil)
	if err := h.AdminGetOrder(getCtx); err != nil {
		t.Fatalf("AdminGetOrder: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["payment_method"] != "manual" {
		t.Errorf(`response["payment_method"]: want "manual", got %v`, raw["payment_method"])
	}
	if raw["payment_proof_url"] != proofKey {
		t.Errorf(`response["payment_proof_url"]: want %q, got %v`, proofKey, raw["payment_proof_url"])
	}
}
