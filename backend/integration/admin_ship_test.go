package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bookingLogisticsClient is a LogisticsClient stub whose CreateOrder succeeds.
// Every logistics stub already in this suite (stubLogisticsClient,
// twoServiceLogisticsClient) returns ErrShippingUnavailable from CreateOrder,
// so the successful /ship auto-book path (FR-C-6) has never run against a
// real Postgres row through the full HTTP stack — this closes that gap.
type bookingLogisticsClient struct{}

func (bookingLogisticsClient) GetRates(context.Context, service.ShippingQuoteRequest) ([]service.CourierRate, error) {
	return []service.CourierRate{
		{Courier: "JNE", Service: "REG", EstimatedDays: 3, Price: 15000, CourierCode: "jne", ServiceCode: "reg"},
	}, nil
}

func (bookingLogisticsClient) CreateOrder(context.Context, service.CreateShipmentRequest) (service.Shipment, error) {
	return service.Shipment{
		BiteshipOrderID: "biteship-order-int-test-1",
		WaybillID:       "WB-INT-TEST-1",
		Status:          "confirmed",
	}, nil
}

func (bookingLogisticsClient) TrackWaybill(context.Context, string, string) (service.WaybillTracking, error) {
	return service.WaybillTracking{}, service.ErrShippingUnavailable
}

func (bookingLogisticsClient) CancelOrder(context.Context, string, string) error {
	return service.ErrShippingUnavailable
}

func (bookingLogisticsClient) GetOrder(context.Context, string) (service.Shipment, error) {
	return service.Shipment{}, service.ErrShippingUnavailable
}

// seedSenderConfig writes the four sender fields AdminShipOrder reads out of
// system_config, mirroring internal/service/shipping_order_test.go's fixture
// — unmistakably fake values per repo policy on committed fixtures.
func seedSenderConfig(t *testing.T, env *testEnv) {
	t.Helper()
	ctx := context.Background()
	repo := repository.New(env.pool)
	vals := map[string]string{
		"app_name":          "Toko Contoh",
		"app_contact_phone": "081200000000",
		"app_address":       "Jl. Contoh No. 1",
		"app_kode_pos":      "12345",
	}
	for k, v := range vals {
		require.NoError(t, repo.UpsertSystemConfig(ctx, k, v, false))
	}
}

// checkoutAt issues POST /:id/checkout with an Idempotency-Key against an
// arbitrary base URL — checkoutWithKey (checkout_test.go) is hardcoded to
// env.server.URL, which doesn't fit a test standing up its own logistics-
// wired server via logisticsServer.
func checkoutAt(t *testing.T, baseURL, orderID, token, idempKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/orders/"+orderID+"/checkout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", idempKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// adminConfirmAt issues POST /admin/orders/:id/confirm with an Idempotency-Key
// against an arbitrary base URL, for the same reason as checkoutAt above.
// FR-25/FR-26 require a payment_proof_url under payment_proof/, so this sends
// a fixture key rather than an empty body.
func adminConfirmAt(t *testing.T, baseURL, orderID, token, idempKey string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(map[string]string{
		"payment_proof_url": "payment_proof/int-test/" + idempKey + ".jpg",
	}))
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/admin/orders/"+orderID+"/confirm", &buf)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", idempKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestAdminShipAutoBook_BooksAtCarrierAndPersistsWaybill exercises FR-C-6 end
// to end against real Postgres: cart -> patch (persists courier_code/
// courier_service_code per FR-C-2) -> checkout -> admin confirm -> admin ship,
// then asserts the booked waybill, biteship_order_id and waybill_source land
// in the row exactly as a successful Biteship booking would produce them.
func TestAdminShipAutoBook_BooksAtCarrierAndPersistsWaybill(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	seedSenderConfig(t, env)

	ls := logisticsServer(t, env, bookingLogisticsClient{})

	adminID := seedUser(t, env, "super_admin", "active", false)
	adminToken := authToken(t, env, adminID, "super_admin")

	studentID := seedUser(t, env, "student", "active", false)
	studentToken := authToken(t, env, studentID, "student")
	productID := seedProduct(t, env, "book", "Buku Auto-Book Test", 100000)

	resp := doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders", nil, studentToken)
	body := decodeBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	orderID := body["id"].(string)

	drainClose(doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
		map[string]any{"product_id": productID, "qty": 1}, studentToken))

	provinceID, cityID, districtID := seedRegionIDs(t, env)
	patchResp := doJSONAt(t, ls.URL, http.MethodPatch, "/api/v1/orders/"+orderID,
		map[string]any{
			"courier":       "JNE",
			"service":       "REG",
			"shipping_cost": 1.0,
			"province_id":   provinceID,
			"city_id":       cityID,
			"district_id":   districtID,
			"kode_pos":      "12345",
			"shipping_address": map[string]string{
				"penerima": "Budi Test",
				"telepon":  "081200000000",
				"alamat":   "Jl. Contoh No. 1",
				"kode_pos": "12345",
			},
		}, studentToken)
	require.Equal(t, http.StatusOK, patchResp.StatusCode, "patch failed: %v", decodeBody(t, patchResp))
	drainClose(patchResp)

	var courierCode, serviceCode *string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT courier_code, courier_service_code FROM orders WHERE id=$1`, orderID,
	).Scan(&courierCode, &serviceCode))
	require.NotNil(t, courierCode, "PatchCart must persist the rate's CourierCode (FR-C-2)")
	require.NotNil(t, serviceCode)
	assert.Equal(t, "jne", *courierCode)
	assert.Equal(t, "reg", *serviceCode)

	coResp := checkoutAt(t, ls.URL, orderID, studentToken, fmt.Sprintf("idemp-shipauto-%d", time.Now().UnixNano()))
	coBody := decodeBody(t, coResp)
	require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", coBody)

	confirmResp := adminConfirmAt(t, ls.URL, orderID, adminToken, fmt.Sprintf("idemp-confirm-shipauto-%d", time.Now().UnixNano()))
	confirmBody := decodeBody(t, confirmResp)
	require.Equal(t, http.StatusOK, confirmResp.StatusCode, "admin confirm failed: %v", confirmBody)

	var preShipStatus string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, orderID,
	).Scan(&preShipStatus))
	require.Equal(t, "paid", preShipStatus, "order must be paid before shipping")

	shipResp := doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/admin/orders/"+orderID+"/ship", nil, adminToken)
	shipBody := decodeBody(t, shipResp)
	require.Equal(t, http.StatusOK, shipResp.StatusCode, "ship failed: %v", shipBody)

	var status, trackingNumber string
	var biteshipOrderID, waybillSource *string
	var shippedAt *time.Time
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT status, tracking_number, biteship_order_id, waybill_source, shipped_at
		 FROM orders WHERE id=$1`, orderID,
	).Scan(&status, &trackingNumber, &biteshipOrderID, &waybillSource, &shippedAt))

	assert.Equal(t, "shipped", status)
	assert.Equal(t, "WB-INT-TEST-1", trackingNumber, "tracking_number must be the carrier's waybill_id")
	require.NotNil(t, biteshipOrderID)
	assert.Equal(t, "biteship-order-int-test-1", *biteshipOrderID)
	require.NotNil(t, waybillSource)
	assert.Equal(t, "biteship", *waybillSource)
	assert.NotNil(t, shippedAt)
}

// TestAdminShipAutoBook_NoCarrierCodeRefusedOverHTTP exercises the FR-C-3
// refusal path through the real router and mapServiceError, not just the
// service-level unit test — an order that predates Track C (or was priced by
// the flat-rate fallback) carries no courier_code, and /ship must refuse with
// the specific reason rather than a generic 500, leaving the order untouched.
func TestAdminShipAutoBook_NoCarrierCodeRefusedOverHTTP(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	seedSenderConfig(t, env)

	adminID := seedUser(t, env, "super_admin", "active", false)
	adminToken := authToken(t, env, adminID, "super_admin")
	studentID := seedUser(t, env, "student", "active", false)

	var orderID string
	require.NoError(t, env.pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total, shipping_address)
		 VALUES ($1, 'paid', 50000, 50000, $2::jsonb) RETURNING id`,
		studentID,
		`{"penerima":"Budi Test","telepon":"081200000000","alamat":"Jl. Contoh No. 1","kode_pos":"12345"}`,
	).Scan(&orderID))

	resp := env.doJSON(t, http.MethodPost, "/api/v1/admin/orders/"+orderID+"/ship", nil, adminToken)
	respBody := decodeBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %v", respBody)

	var status string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, orderID,
	).Scan(&status))
	assert.Equal(t, "paid", status, "order must be left untouched when no courier code is on record")
}
