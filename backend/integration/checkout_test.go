package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkoutWithKey issues POST /api/v1/orders/:id/checkout with the Idempotency-Key header.
func checkoutWithKey(t *testing.T, env *testEnv, orderID, token, idempKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/orders/"+orderID+"/checkout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", idempKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// sendWebhookToURL builds and POSTs a Midtrans notification to the given base URL.
// Midtrans retries a notification with the same transaction_id and no custom
// header — pass the same transactionID twice to simulate a duplicate delivery.
// Pass signature="" to omit signature_key from the JSON body.
func sendWebhookToURL(t *testing.T, baseURL, orderID, transactionID, signature, grossAmount string) *http.Response {
	t.Helper()
	body := map[string]string{
		"transaction_status": "settlement",
		"order_id":           orderID,
		"transaction_id":     transactionID,
		"gross_amount":       grossAmount,
		"status_code":        "200",
	}
	if signature != "" {
		body["signature_key"] = signature
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		baseURL+"/api/v1/webhooks/payment",
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// strictPaymentClient rejects empty signatures, so a missing signature_key fails verification.
type strictPaymentClient struct{}

func (strictPaymentClient) CreatePayment(ctx context.Context, req service.PaymentRequest) (service.PaymentResponse, error) {
	return service.PaymentResponse{}, nil
}

func (strictPaymentClient) QueryStatus(ctx context.Context, reference string) (service.PaymentStatus, error) {
	return service.PaymentStatus{}, nil
}

func (strictPaymentClient) VerifySignature(payload []byte, signature string) bool {
	return signature != ""
}

// capturingPaymentClient captures the payment request for assertion.
type capturingPaymentClient struct {
	lastPaymentRequest *service.PaymentRequest
}

func (c *capturingPaymentClient) CreatePayment(ctx context.Context, req service.PaymentRequest) (service.PaymentResponse, error) {
	c.lastPaymentRequest = &req
	return service.PaymentResponse{GatewayRef: "noop-" + req.OrderID}, nil
}

func (c *capturingPaymentClient) QueryStatus(ctx context.Context, reference string) (service.PaymentStatus, error) {
	return service.PaymentStatus{}, nil
}

func (c *capturingPaymentClient) VerifySignature(payload []byte, signature string) bool {
	return true
}

// webhookServer returns a test server wired with a strict payment client for signature verification tests.
func webhookServer(t *testing.T, env *testEnv, payment service.PaymentClient) *httptest.Server {
	t.Helper()
	repo := repository.New(env.pool)
	cfg := &config.Config{CORSOrigins: []string{"*"}}
	svc := service.NewWithStore(repo, repo, env.rdb, env.signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{}, payment,
		&service.NoopLogisticsClient{}, nil, cfg, nil)
	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, env.signer)
	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts
}

// twoServiceLogisticsClient returns two rates for the same carrier at different
// prices, to prove PatchCart matches on courier+service, not carrier alone.
type twoServiceLogisticsClient struct{}

func (twoServiceLogisticsClient) GetRates(ctx context.Context, req service.ShippingQuoteRequest) ([]service.CourierRate, error) {
	return []service.CourierRate{
		{Courier: "JNE", Service: "REG", EstimatedDays: 3, Price: 15000},
		{Courier: "JNE", Service: "YES", EstimatedDays: 1, Price: 30000},
	}, nil
}

func (twoServiceLogisticsClient) CreateOrder(context.Context, service.CreateShipmentRequest) (service.Shipment, error) {
	return service.Shipment{}, service.ErrShippingUnavailable
}

func (twoServiceLogisticsClient) CancelOrder(context.Context, string, string) error {
	return service.ErrShippingUnavailable
}

func (twoServiceLogisticsClient) GetOrder(context.Context, string) (service.Shipment, error) {
	return service.Shipment{}, service.ErrShippingUnavailable
}

// logisticsServer returns a test server wired with the given logistics client,
// for tests that need to control which courier rates are quoted.
func logisticsServer(t *testing.T, env *testEnv, logistics service.LogisticsClient) *httptest.Server {
	t.Helper()
	repo := repository.New(env.pool)
	cfg := &config.Config{CORSOrigins: []string{"*"}}
	svc := service.NewWithStore(repo, repo, env.rdb, env.signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &capturingPaymentClient{},
		logistics, nil, cfg, nil)
	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, env.signer)
	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts
}

// doJSONAt is doJSON against an arbitrary base URL, for tests that stand up
// their own *httptest.Server (e.g. with a custom logistics client) instead of
// using env.server.
func doJSONAt(t *testing.T, baseURL, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, baseURL+path, &buf)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// drainClose discards and closes a response body.
func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestCheckout(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	t.Run("FR-INT-07 mint cart idempotency — one cart row", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")

		resp1 := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body1 := decodeBody(t, resp1)
		require.Equal(t, http.StatusCreated, resp1.StatusCode)
		orderID1, ok := body1["id"].(string)
		require.True(t, ok, "body must contain 'id' field, got: %v", body1)
		require.NotEmpty(t, orderID1)

		resp2 := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body2 := decodeBody(t, resp2)
		require.Equal(t, http.StatusOK, resp2.StatusCode)
		orderID2, ok := body2["id"].(string)
		require.True(t, ok, "body must contain 'id' field, got: %v", body2)

		assert.Equal(t, orderID1, orderID2, "second MintCart must return same order id")

		var count int
		err := env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM orders WHERE student_id=$1 AND status='cart'`, userID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "exactly one cart row must exist (idx_orders_student_cart)")
	})

	t.Run("FR-INT-08 add item writes order_item row", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Matematika", 50000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		resp2 := env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token)
		require.Equal(t, http.StatusCreated, resp2.StatusCode)
		drainClose(resp2)

		var count int
		err := env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM order_item WHERE order_id=$1 AND product_id=$2`,
			orderID, productID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "order_item row must exist for order/product pair")
	})

	t.Run("FR-INT-09 patch cart persists shipping/courier fields", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Fisika", 30000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// shipping_address is []byte in the handler (decoded as base64 JSON); it's omitted to
		// keep the test simple and avoid base64 encoding in the test body. A courier selection
		// still requires a full destination so the server can price it, so a real address is sent.
		provinceID, cityID, districtID := seedRegionIDs(t, env)
		patchResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":     "JNE",
				"service":     "REG",
				"province_id": provinceID,
				"city_id":     cityID,
				"district_id": districtID,
				"kode_pos":    "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp.StatusCode)
		drainClose(patchResp)

		var courier string
		err := env.pool.QueryRow(ctx,
			`SELECT selected_courier FROM orders WHERE id=$1`, orderID,
		).Scan(&courier)
		require.NoError(t, err)
		assert.Equal(t, "JNE", courier)
	})

	t.Run("FR-INT-10 checkout sets payment_pending, gateway_ref, decrements stock, Redis key", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku IPA", 40000)

		var stockBefore int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT stock FROM product WHERE id=$1`, productID,
		).Scan(&stockBefore))

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		provinceID, cityID, districtID := seedRegionIDs(t, env)
		drainClose(env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token))

		idempKey := fmt.Sprintf("test-checkout-%d", time.Now().UnixNano())
		coResp := checkoutWithKey(t, env, orderID, token, idempKey)
		coBody := decodeBody(t, coResp)
		require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", coBody)

		gatewayRef, _ := coBody["gateway_ref"].(string)
		assert.NotEmpty(t, gatewayRef)
		assert.Equal(t, "noop-"+orderID, gatewayRef)

		var status, gwRef string
		var paymentExpiresAt time.Time
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status, gateway_ref, payment_expires_at FROM orders WHERE id=$1`, orderID,
		).Scan(&status, &gwRef, &paymentExpiresAt))
		assert.Equal(t, "payment_pending", status)
		assert.Equal(t, "noop-"+orderID, gwRef)
		assert.True(t, paymentExpiresAt.After(time.Now()), "payment_expires_at must be in the future")

		var stockAfter int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT stock FROM product WHERE id=$1`, productID,
		).Scan(&stockAfter))
		assert.Equal(t, stockBefore-1, stockAfter, "stock must be decremented by 1 after checkout")

		exists, err := env.rdb.Exists(ctx, "idempotency:checkout:"+idempKey).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists, "idempotency:checkout:<key> must exist in Redis")
	})

	t.Run("FR-INT-11 webhook flips order to paid, inserts outbox row and webhook_log", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Kimia", 25000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		provinceID, cityID, districtID := seedRegionIDs(t, env)
		drainClose(env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token))

		coResp := checkoutWithKey(t, env, orderID, token,
			fmt.Sprintf("co-%d", time.Now().UnixNano()))
		require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", decodeBody(t, coResp))

		// A validly-signed webhook settles the order. strictPaymentClient verifies
		// a non-empty signature — an unsigned webhook can no longer settle anything.
		strict := webhookServer(t, env, strictPaymentClient{})
		transactionID := fmt.Sprintf("wh-%d", time.Now().UnixNano())
		whResp := sendWebhookToURL(t, strict.URL, orderID, transactionID, "any-sig", fmt.Sprintf("%.2f", float64(25000)))
		whBody := decodeBody(t, whResp)
		require.Equal(t, http.StatusOK, whResp.StatusCode, "webhook failed: %v", whBody)

		var status string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&status))
		assert.Equal(t, "paid", status)

		var outboxCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid'`, orderID,
		).Scan(&outboxCount))
		assert.Equal(t, 1, outboxCount, "exactly one outbox OrderPaid row")

		var logCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM webhook_log WHERE gateway_ref=$1`, orderID,
		).Scan(&logCount))
		assert.Equal(t, 1, logCount, "webhook_log row must exist")
	})

	t.Run("FR-INT-12 webhook missing signature_key returns 401 invalid_signature", func(t *testing.T) {
		strict := webhookServer(t, env, strictPaymentClient{})
		whResp := sendWebhookToURL(t, strict.URL,
			"00000000-0000-0000-0000-000000000000",
			fmt.Sprintf("key-%d", time.Now().UnixNano()),
			"", /* omit signature_key */
			"0.00")
		whBody := decodeBody(t, whResp)
		require.Equal(t, http.StatusUnauthorized, whResp.StatusCode)
		assert.Equal(t, "invalid_signature", whBody["code"])
	})

	t.Run("FR-INT-13 webhook idempotency — duplicate delivery produces single outbox row", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Biologi", 20000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		provinceID, cityID, districtID := seedRegionIDs(t, env)
		drainClose(env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token))

		coResp := checkoutWithKey(t, env, orderID, token,
			fmt.Sprintf("co-%d", time.Now().UnixNano()))
		require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", decodeBody(t, coResp))

		strict := webhookServer(t, env, strictPaymentClient{})
		// Midtrans retries reuse the same transaction_id — that's what makes
		// this a "duplicate delivery" rather than two independent events.
		transactionID := fmt.Sprintf("dedup-%d", time.Now().UnixNano())

		wh1 := sendWebhookToURL(t, strict.URL, orderID, transactionID, "sig1", fmt.Sprintf("%.2f", float64(20000)))
		wh1Body := decodeBody(t, wh1)
		require.Equal(t, http.StatusOK, wh1.StatusCode, "first delivery: %v", wh1Body)

		wh2 := sendWebhookToURL(t, strict.URL, orderID, transactionID, "sig2", fmt.Sprintf("%.2f", float64(20000)))
		wh2Body := decodeBody(t, wh2)
		require.Equal(t, http.StatusOK, wh2.StatusCode, "second delivery: %v", wh2Body)

		var status string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&status))
		assert.Equal(t, "paid", status)

		var outboxCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid'`, orderID,
		).Scan(&outboxCount))
		assert.Equal(t, 1, outboxCount, "duplicate delivery must not insert a second outbox row")
	})

	t.Run("FR-SHIP-01..10 total invariant — add→quote→select→mutate→invalidate→re-quote→checkout", func(t *testing.T) {
		// Use a fresh env for this test to avoid state pollution.
		testEnv := newTestEnv(t)
		ctx := context.Background()

		userID := seedUser(t, testEnv, "student", "active", false)
		token := authToken(t, testEnv, userID, "student")

		// Seed a physical product with weight.
		var productID string
		err := testEnv.pool.QueryRow(ctx,
			`INSERT INTO product (type, name, price, stock, status, weight_grams)
			 VALUES ($1, $2, $3, 100, 'published', $4) RETURNING id`,
			"book", "Buku Pengiriman", 100000, 500,
		).Scan(&productID)
		require.NoError(t, err)

		// Step 1: Create cart and add physical item.
		resp := testEnv.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(testEnv.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// Step 2: Quote shipping (NoopLogisticsClient returns JNE 15000, TIKI 25000).
		shippingResp := testEnv.doJSON(t, http.MethodPost, "/api/v1/orders/shipping",
			map[string]any{
				"destination_postal_code": "12345",
				"weight_grams":            500,
			}, token)
		require.Equal(t, http.StatusOK, shippingResp.StatusCode)
		shippingBody := decodeBody(t, shippingResp)
		rates, ok := shippingBody["rates"].([]any)
		require.True(t, ok, "rates must be an array, got: %T", shippingBody["rates"])
		require.Len(t, rates, 2, "expected 2 courier rates from noop logistics client")

		// Extract first rate (JNE, price 15000).
		ratesMap := rates[0].(map[string]any)
		priceVal, ok := ratesMap["price"]
		require.True(t, ok, "rates must have price field, got keys: %v", ratesMap)
		var selectedPrice int64
		switch v := priceVal.(type) {
		case float64:
			selectedPrice = int64(v)
		case int:
			selectedPrice = int64(v)
		default:
			require.Fail(t, "Price must be a number, got: %T", v)
		}
		require.Equal(t, int64(15000), selectedPrice)

		// Step 3: PATCH cart to select courier and set shipping cost.
		// Get a valid province_id from the database.
		var provinceID string
		err1 := testEnv.pool.QueryRow(ctx,
			`SELECT id FROM province LIMIT 1`,
		).Scan(&provinceID)
		require.NoError(t, err1, "must have at least one province seeded")

		// Get a valid city_id for this province.
		var cityID string
		err2 := testEnv.pool.QueryRow(ctx,
			`SELECT id FROM city WHERE province_id = $1 LIMIT 1`, provinceID,
		).Scan(&cityID)
		require.NoError(t, err2, "must have at least one city for this province")

		// Get a valid district_id for this city.
		var districtID string
		err3 := testEnv.pool.QueryRow(ctx,
			`SELECT id FROM district WHERE city_id = $1 LIMIT 1`, cityID,
		).Scan(&districtID)
		require.NoError(t, err3, "must have at least one district for this city")

		patchResp := testEnv.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp.StatusCode)
		drainClose(patchResp)

		// Step 4: Get order and verify total = subtotal - discount + shipping_cost.
		orderResp := testEnv.doJSON(t, http.MethodGet, "/api/v1/orders/"+orderID, nil, token)
		require.Equal(t, http.StatusOK, orderResp.StatusCode)
		orderBody := decodeBody(t, orderResp)

		subtotal, ok := orderBody["subtotal"].(float64)
		require.True(t, ok, "subtotal must be float64, got: %T", orderBody["subtotal"])

		discountVal := orderBody["discount"]
		discount := 0.0
		if discountVal != nil {
			if v, ok := discountVal.(float64); ok {
				discount = v
			}
		}

		shippingCost, ok := orderBody["shipping_cost"].(float64)
		require.True(t, ok, "shipping_cost must be float64, got: %T", orderBody["shipping_cost"])

		total, ok := orderBody["total"].(float64)
		require.True(t, ok, "total must be float64, got: %T", orderBody["total"])

		require.Equal(t, 100000.0, subtotal, "subtotal should be 100000 (1x 100000)")
		require.Equal(t, 0.0, discount, "discount should be 0 (no promo)")
		require.Equal(t, 15000.0, shippingCost, "shipping_cost should be 15000 (selected rate)")

		expectedTotal := subtotal - discount + shippingCost
		assert.Equal(t, expectedTotal, total, "total must equal subtotal - discount + shipping_cost")

		// Step 5: Update item qty (this should clear shipping).
		itemsArray := orderBody["items"].([]any)
		require.Len(t, itemsArray, 1)
		item := itemsArray[0].(map[string]any)
		itemID := item["id"].(string)

		qtyResp := testEnv.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID+"/items/"+itemID,
			map[string]any{"qty": 2}, token)
		require.Equal(t, http.StatusNoContent, qtyResp.StatusCode)

		// Step 6: Verify shipping is cleared and total is recomputed.
		orderResp2 := testEnv.doJSON(t, http.MethodGet, "/api/v1/orders/"+orderID, nil, token)
		require.Equal(t, http.StatusOK, orderResp2.StatusCode)
		order2 := decodeBody(t, orderResp2)

		subtotal2, ok := order2["subtotal"].(float64)
		require.True(t, ok, "subtotal must be float64")
		discount2Val := order2["discount"]
		discount2 := 0.0
		if discount2Val != nil {
			if v, ok := discount2Val.(float64); ok {
				discount2 = v
			}
		}
		shippingCost2, ok := order2["shipping_cost"].(float64)
		require.True(t, ok, "shipping_cost must be float64")
		total2, ok := order2["total"].(float64)
		require.True(t, ok, "total must be float64")
		selectedCourier2Val := order2["selected_courier"]
		selectedCourier2 := ""
		if selectedCourier2Val != nil {
			if v, ok := selectedCourier2Val.(string); ok {
				selectedCourier2 = v
			}
		}

		require.Equal(t, 200000.0, subtotal2, "subtotal should be 200000 (2x 100000)")
		require.Equal(t, 0.0, discount2, "discount still 0")
		require.Equal(t, 0.0, shippingCost2, "shipping_cost must be cleared after qty change")
		require.Equal(t, "", selectedCourier2, "selected_courier must be cleared")

		expectedTotal2 := subtotal2 - discount2 + shippingCost2
		assert.Equal(t, expectedTotal2, total2, "total must equal subtotal - discount + shipping_cost (0)")

		// This is the critical assertion: if the pre-image bug existed (total retained
		// the old shipping_cost in the SET clause), this would fail.
		assert.Equal(t, 200000.0, total2, "total should be exactly subtotal (200000) since shipping_cost is 0")

		// Step 7: Re-quote shipping (for new total weight).
		shippingResp2 := testEnv.doJSON(t, http.MethodPost, "/api/v1/orders/shipping",
			map[string]any{
				"destination_postal_code": "12345",
				"weight_grams":            1000, // doubled weight
			}, token)
		require.Equal(t, http.StatusOK, shippingResp2.StatusCode)
		shippingBody2 := decodeBody(t, shippingResp2)
		rates2 := shippingBody2["rates"].([]any)
		require.Len(t, rates2, 2)

		// Step 8: Re-select a different courier (TIKI, price 25000).
		ratesMap2 := rates2[1].(map[string]any)
		priceVal2, ok := ratesMap2["price"]
		require.True(t, ok, "rates must have price field")
		var selectedPrice2 int64
		switch v := priceVal2.(type) {
		case float64:
			selectedPrice2 = int64(v)
		case int:
			selectedPrice2 = int64(v)
		default:
			require.Fail(t, "Price must be a number, got: %T", v)
		}
		require.Equal(t, int64(25000), selectedPrice2)

		patchResp2 := testEnv.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "TIKI",
				"service":       "ONS",
				"shipping_cost": 25000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp2.StatusCode)
		drainClose(patchResp2)

		// Verify total is recalculated correctly.
		orderResp3 := testEnv.doJSON(t, http.MethodGet, "/api/v1/orders/"+orderID, nil, token)
		require.Equal(t, http.StatusOK, orderResp3.StatusCode)
		order3 := decodeBody(t, orderResp3)

		subtotal3, ok := order3["subtotal"].(float64)
		require.True(t, ok, "subtotal must be float64")
		shippingCost3, ok := order3["shipping_cost"].(float64)
		require.True(t, ok, "shipping_cost must be float64")
		total3, ok := order3["total"].(float64)
		require.True(t, ok, "total must be float64")

		require.Equal(t, 200000.0, subtotal3)
		require.Equal(t, 25000.0, shippingCost3, "shipping_cost should be 25000 (new selection)")
		expectedTotal3 := subtotal3 + shippingCost3
		assert.Equal(t, expectedTotal3, total3, "total must be subtotal + shipping_cost = 225000")

		// Step 9: Checkout succeeds with correct total, capturing the payment request.
		capturingClient := &capturingPaymentClient{}
		ts := webhookServer(t, testEnv, capturingClient)
		defer ts.Close()

		checkoutReq, err := http.NewRequest(http.MethodPost,
			ts.URL+"/api/v1/orders/"+orderID+"/checkout", nil)
		require.NoError(t, err)
		checkoutReq.Header.Set("Authorization", "Bearer "+token)
		checkoutReq.Header.Set("Idempotency-Key", fmt.Sprintf("ship-checkout-%d", time.Now().UnixNano()))
		checkoutResp, err := http.DefaultClient.Do(checkoutReq)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, checkoutResp.StatusCode)
		drainClose(checkoutResp)

		// Step 10: Verify Midtrans payment request includes shipping ItemDetail with correct amount.
		require.NotNil(t, capturingClient.lastPaymentRequest, "payment request must be captured")
		paymentReq := capturingClient.lastPaymentRequest

		// Verify amount matches order total.
		require.Equal(t, int64(225000), paymentReq.Amount, "payment amount must equal order total (225000)")

		// Find and verify shipping item in request.
		var shippingItem *service.ItemDetail
		for i := range paymentReq.Items {
			if paymentReq.Items[i].Category == "Shipping" {
				shippingItem = &paymentReq.Items[i]
				break
			}
		}
		require.NotNil(t, shippingItem, "payment request must include Shipping ItemDetail")
		assert.Equal(t, "shipping", shippingItem.ID, "shipping item ID must be 'shipping'")
		assert.Equal(t, int64(25000), shippingItem.Price, "shipping item price must match selected rate (25000)")
		assert.EqualValues(t, 1, shippingItem.Qty, "shipping item qty must be 1")

		// Verify order table reflects correct state.
		var orderStatus string
		var finalShippingCost float64
		var finalTotal float64
		err = testEnv.pool.QueryRow(ctx,
			`SELECT status, shipping_cost, total FROM orders WHERE id=$1`, orderID,
		).Scan(&orderStatus, &finalShippingCost, &finalTotal)
		require.NoError(t, err)

		assert.Equal(t, "payment_pending", orderStatus, "order should be payment_pending after checkout")
		assert.Equal(t, 25000.0, finalShippingCost, "shipping_cost should persist as 25000")
		assert.Equal(t, 225000.0, finalTotal, "final total should still be 225000")
	})

	t.Run("FR-09 checkout rejects physical order with unset shipping_cost via real Checkout()", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Tanpa Ongkir", 100000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// No shipping quote/selection has been made — the real Service.Checkout must
		// reject this via ErrShippingRequired (mapped to 400 invalid_request), not a shim.
		checkoutResp := checkoutWithKey(t, env, orderID, token, fmt.Sprintf("ship-guard-%d", time.Now().UnixNano()))
		checkoutBody := decodeBody(t, checkoutResp)
		require.Equal(t, http.StatusBadRequest, checkoutResp.StatusCode)
		assert.Equal(t, "invalid_request", checkoutBody["code"])

		var statusAfterReject string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&statusAfterReject))
		assert.Equal(t, "cart", statusAfterReject, "order must remain in cart status after a rejected checkout")

		// Setting shipping_cost via the same PATCH cart path must let the real
		// Checkout() proceed to payment_pending.
		provinceID, cityID, districtID := seedRegionIDs(t, env)
		drainClose(env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token))

		checkoutResp2 := checkoutWithKey(t, env, orderID, token, fmt.Sprintf("ship-guard-ok-%d", time.Now().UnixNano()))
		require.Equal(t, http.StatusOK, checkoutResp2.StatusCode)
		drainClose(checkoutResp2)

		var statusAfterCheckout string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&statusAfterCheckout))
		assert.Equal(t, "payment_pending", statusAfterCheckout, "order should be payment_pending once shipping_cost is set")
	})

	t.Run("PR41-01 forged shipping_cost is ignored — server recomputes from the courier quote", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Ongkir Forge", 100000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// Client selects the real "JNE" courier (noop rate = 15000) but forges a
		// shipping_cost of 1 to try to pay almost nothing for shipping.
		provinceID, cityID, districtID := seedRegionIDs(t, env)
		patchResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 1.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp.StatusCode)
		drainClose(patchResp)

		var persistedShippingCost float64
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT shipping_cost FROM orders WHERE id=$1`, orderID,
		).Scan(&persistedShippingCost))
		assert.Equal(t, 15000.0, persistedShippingCost, "server must recompute shipping_cost from the live courier quote, not trust the client")

		coResp := checkoutWithKey(t, env, orderID, token, fmt.Sprintf("forge-guard-%d", time.Now().UnixNano()))
		coBody := decodeBody(t, coResp)
		require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", coBody)

		var finalTotal float64
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT total FROM orders WHERE id=$1`, orderID,
		).Scan(&finalTotal))
		assert.Equal(t, 115000.0, finalTotal, "total must reflect the real 15000 rate, not the forged 1")
	})

	t.Run("PR41-01 unknown courier name is rejected, not silently trusted", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Ongkir Unknown", 100000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		provinceID, cityID, districtID := seedRegionIDs(t, env)
		patchResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "NOT-A-REAL-COURIER",
				"shipping_cost": 1.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		patchBody := decodeBody(t, patchResp)
		require.Equal(t, http.StatusUnprocessableEntity, patchResp.StatusCode)
		assert.Equal(t, "invalid_courier_selection", patchBody["code"])
		drainClose(patchResp)
	})

	t.Run("PR41-03 empty address hierarchy strings are rejected, not written as an FK-violating patch", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Alamat Kosong", 100000)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// Mirrors what an incomplete profile sends today: courier selected, but
		// province/city/district still empty strings rather than omitted.
		patchResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   "",
				"city_id":       "",
				"district_id":   "",
				"kode_pos":      "12345",
			}, token)
		patchBody := decodeBody(t, patchResp)
		require.Equal(t, http.StatusUnprocessableEntity, patchResp.StatusCode, "must be a validation error, not a 500 FK crash: %v", patchBody)
		assert.Equal(t, "incomplete_address", patchBody["code"])
		drainClose(patchResp)
	})

	t.Run("PR41-followup-1 matches courier+service, not carrier alone", func(t *testing.T) {
		ls := logisticsServer(t, env, twoServiceLogisticsClient{})

		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		productID := seedProduct(t, env, "book", "Buku Dua Servis JNE", 100000)

		resp := doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID := body["id"].(string)

		drainClose(doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		// twoServiceLogisticsClient quotes JNE/REG=15000 and JNE/YES=30000 for
		// any destination. Select the SECOND service (YES) while forging
		// shipping_cost to neither price — if the server matched on carrier
		// alone it would silently persist the FIRST same-carrier rate (REG,
		// 15000) instead of the one actually selected.
		provinceID, cityID, districtID := seedRegionIDs(t, env)
		patchResp := doJSONAt(t, ls.URL, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"courier":       "JNE",
				"service":       "YES",
				"shipping_cost": 1.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp.StatusCode, "patch failed: %v", decodeBody(t, patchResp))
		drainClose(patchResp)

		var persistedCost float64
		var persistedCourier, persistedService *string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT shipping_cost, selected_courier, selected_service FROM orders WHERE id=$1`, orderID,
		).Scan(&persistedCost, &persistedCourier, &persistedService))
		require.NotNil(t, persistedCourier)
		require.NotNil(t, persistedService)
		assert.Equal(t, 30000.0, persistedCost, "must price the exact JNE/YES rate selected, not JNE/REG's 15000")
		assert.Equal(t, "JNE", *persistedCourier)
		assert.Equal(t, "YES", *persistedService)

		// A second, independent cart selecting the FIRST service (REG) must
		// persist REG's own price — proving the match isn't "always take the
		// last/first rate" but genuinely keyed on courier+service.
		userID2 := seedUser(t, env, "student", "active", false)
		token2 := authToken(t, env, userID2, "student")
		productID2 := seedProduct(t, env, "book", "Buku Dua Servis JNE 2", 100000)

		resp2 := doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders", nil, token2)
		body2 := decodeBody(t, resp2)
		require.Equal(t, http.StatusCreated, resp2.StatusCode)
		orderID2 := body2["id"].(string)

		drainClose(doJSONAt(t, ls.URL, http.MethodPost, "/api/v1/orders/"+orderID2+"/items",
			map[string]any{"product_id": productID2, "qty": 1}, token2))

		patchResp2 := doJSONAt(t, ls.URL, http.MethodPatch, "/api/v1/orders/"+orderID2,
			map[string]any{
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 1.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token2)
		require.Equal(t, http.StatusOK, patchResp2.StatusCode, "patch failed: %v", decodeBody(t, patchResp2))
		drainClose(patchResp2)

		var persistedCost2 float64
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT shipping_cost FROM orders WHERE id=$1`, orderID2,
		).Scan(&persistedCost2))
		assert.Equal(t, 15000.0, persistedCost2, "must price the exact JNE/REG rate selected, not JNE/YES's 30000")
	})
}

// TestShippingPreservedAcrossDigitalItemMutations exercises the real, HTTP-backed
// Service.AddItem/UpdateItemQty/RemoveItem (not a test-local shim) to prove that
// adding, changing the quantity of, and removing a digital item on a cart that
// already has a physical shipment quoted does not clear shipping_cost/selected_courier,
// while mutating the physical item still clears it.
func TestShippingPreservedAcrossDigitalItemMutations(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	userID := seedUser(t, env, "student", "active", false)
	token := authToken(t, env, userID, "student")

	var bookID string
	require.NoError(t, env.pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ($1, $2, $3, 100, 'published', $4) RETURNING id`,
		"book", "Buku Preserve", 100000, 500,
	).Scan(&bookID))
	courseID := seedProduct(t, env, "course", "Kursus Preserve", 50000)

	resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
	body := decodeBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	orderID := body["id"].(string)

	drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
		map[string]any{"product_id": bookID, "qty": 1}, token))

	provinceID, cityID, districtID := seedRegionIDs(t, env)
	drainClose(env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
		map[string]any{
			"courier":       "JNE",
			"service":       "REG",
			"shipping_cost": 15000.0,
			"province_id":   provinceID,
			"city_id":       cityID,
			"district_id":   districtID,
			"kode_pos":      "12345",
		}, token))

	assertShipping := func(wantCost float64, wantCourier string, msg string) {
		t.Helper()
		var cost float64
		var courier *string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT shipping_cost, selected_courier FROM orders WHERE id=$1`, orderID,
		).Scan(&cost, &courier))
		assert.Equal(t, wantCost, cost, msg+" (shipping_cost)")
		got := ""
		if courier != nil {
			got = *courier
		}
		assert.Equal(t, wantCourier, got, msg+" (selected_courier)")
	}

	assertShipping(15000.0, "JNE", "after courier selection")

	// Adding a digital item must not touch the shipment already quoted for the physical item.
	addResp := env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
		map[string]any{"product_id": courseID, "qty": 1}, token)
	require.Equal(t, http.StatusCreated, addResp.StatusCode)
	drainClose(addResp)
	assertShipping(15000.0, "JNE", "after adding digital item")

	var courseItemID string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM order_item WHERE order_id=$1 AND product_id=$2`, orderID, courseID,
	).Scan(&courseItemID))

	// Changing the digital item's qty must not touch shipping either. Digital
	// products are capped at qty 1 (see service.ValidateItemQty), so this
	// exercises the update codepath at the only qty a digital item can hold.
	qtyResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID+"/items/"+courseItemID,
		map[string]any{"qty": 1}, token)
	require.Equal(t, http.StatusNoContent, qtyResp.StatusCode)
	assertShipping(15000.0, "JNE", "after changing digital item qty")

	// Removing the digital item must not touch shipping either.
	rmResp := env.doJSON(t, http.MethodDelete, "/api/v1/orders/"+orderID+"/items/"+courseItemID, nil, token)
	require.Equal(t, http.StatusNoContent, rmResp.StatusCode)
	assertShipping(15000.0, "JNE", "after removing digital item")

	// Mutating the physical item is the one action that must clear shipping.
	var bookItemID string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM order_item WHERE order_id=$1 AND product_id=$2`, orderID, bookID,
	).Scan(&bookItemID))
	rmBookResp := env.doJSON(t, http.MethodDelete, "/api/v1/orders/"+orderID+"/items/"+bookItemID, nil, token)
	require.Equal(t, http.StatusNoContent, rmBookResp.StatusCode)
	assertShipping(0, "", "after removing the physical item")
}

// TestShippingResponseWireFormat verifies that the /orders/shipping endpoint
// serializes CourierRate with lowercase snake_case JSON keys ("courier", "service",
// "estimated_days", "price"), not PascalCase. This is a regression test to catch
// if JSON struct tags are accidentally removed from service.CourierRate.
func TestShippingResponseWireFormat(t *testing.T) {
	env := newTestEnv(t)

	userID := seedUser(t, env, "student", "active", false)
	token := authToken(t, env, userID, "student")
	productID := seedProduct(t, env, "book", "Buku Wire Format Test", 50000)

	// Create an order and add an item.
	resp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
	body := decodeBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	orderID := body["id"].(string)

	drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
		map[string]any{"product_id": productID, "qty": 1}, token))

	// Request shipping rates.
	shippingResp := env.doJSON(t, http.MethodPost, "/api/v1/orders/shipping",
		map[string]any{
			"destination_postal_code": "12345",
			"weight_grams":            500,
		}, token)
	require.Equal(t, http.StatusOK, shippingResp.StatusCode)

	// Read raw response body to verify JSON structure.
	bodyBytes, err := io.ReadAll(shippingResp.Body)
	require.NoError(t, err)
	shippingResp.Body.Close()

	// Unmarshal into map to inspect keys as they appear in JSON.
	var responseBody map[string]any
	err = json.Unmarshal(bodyBytes, &responseBody)
	require.NoError(t, err)

	// Extract rates array.
	rates, ok := responseBody["rates"].([]any)
	require.True(t, ok, "response must have 'rates' array")
	require.Len(t, rates, 2, "expected 2 rates from noop client")

	// Verify each rate has lowercase snake_case keys, not PascalCase.
	for i, rate := range rates {
		rateMap, ok := rate.(map[string]any)
		require.True(t, ok, "rate[%d] must be a map", i)

		// Assert lowercase keys exist.
		_, hasCourier := rateMap["courier"]
		_, hasService := rateMap["service"]
		_, hasEstimatedDays := rateMap["estimated_days"]
		_, hasPrice := rateMap["price"]

		require.True(t, hasCourier, "rate[%d] must have 'courier' key, keys: %v", i, keys(rateMap))
		require.True(t, hasService, "rate[%d] must have 'service' key, keys: %v", i, keys(rateMap))
		require.True(t, hasEstimatedDays, "rate[%d] must have 'estimated_days' key, keys: %v", i, keys(rateMap))
		require.True(t, hasPrice, "rate[%d] must have 'price' key, keys: %v", i, keys(rateMap))

		// Assert PascalCase keys do NOT exist (regression check).
		_, hasPascalCourier := rateMap["Courier"]
		_, hasPascalService := rateMap["Service"]
		_, hasPascalEstimatedDays := rateMap["EstimatedDays"]
		_, hasPascalPrice := rateMap["Price"]

		require.False(t, hasPascalCourier, "rate[%d] must NOT have PascalCase 'Courier' key", i)
		require.False(t, hasPascalService, "rate[%d] must NOT have PascalCase 'Service' key", i)
		require.False(t, hasPascalEstimatedDays, "rate[%d] must NOT have PascalCase 'EstimatedDays' key", i)
		require.False(t, hasPascalPrice, "rate[%d] must NOT have PascalCase 'Price' key", i)
	}
}

// keys returns the keys of a map for debugging.
func keys(m map[string]any) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
