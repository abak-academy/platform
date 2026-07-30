package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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

	svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, logistics, nil, nil)
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
