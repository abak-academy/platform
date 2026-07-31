package adapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

func TestBiteshipClient_CreateOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/orders" {
			t.Errorf("expected path /v1/orders, got %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "test-api-key" {
			t.Errorf("expected authorization header 'test-api-key', got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		if reqBody["reference_id"] != "order-123" {
			t.Errorf("expected reference_id order-123, got %v", reqBody["reference_id"])
		}
		// POST /v1/orders wants codes, not the display names PatchCart persists.
		if reqBody["courier_company"] != "jne" {
			t.Errorf("expected courier_company jne, got %v", reqBody["courier_company"])
		}
		if reqBody["courier_type"] != "reg" {
			t.Errorf("expected courier_type reg, got %v", reqBody["courier_type"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"id":           "biteship-order-abc",
			"reference_id": "order-123",
			"status":       "confirmed",
			"courier": map[string]interface{}{
				"waybill_id":  "JNE123456789",
				"driver_name": "",
			},
		})
	}))
	defer ts.Close()

	client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

	req := service.CreateShipmentRequest{
		ReferenceID:             "order-123",
		OriginContactName:       "Budi Test",
		OriginContactPhone:      "081200000000",
		OriginAddress:           "Jl. Contoh No. 1",
		OriginPostalCode:        "12440",
		DestinationContactName:  "Siti Test",
		DestinationContactPhone: "081300000000",
		DestinationAddress:      "Jl. Contoh No. 2",
		DestinationPostalCode:   "12240",
		CourierCode:             "jne",
		ServiceCode:             "reg",
		Items: []service.ShipmentItem{
			{Name: "Buku Ujian", Value: 50000, Quantity: 1, WeightGrams: 500},
		},
	}

	shipment, err := client.CreateOrder(t.Context(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if shipment.BiteshipOrderID != "biteship-order-abc" {
		t.Errorf("expected BiteshipOrderID biteship-order-abc, got %q", shipment.BiteshipOrderID)
	}
	if shipment.WaybillID != "JNE123456789" {
		t.Errorf("expected WaybillID JNE123456789, got %q", shipment.WaybillID)
	}
	if shipment.Status != "confirmed" {
		t.Errorf("expected Status confirmed, got %q", shipment.Status)
	}
}

// TestBiteshipClient_CreateOrder_DuplicateReference uses Biteship's documented
// 40002060 response shape (https://biteship.com/id/docs/api/orders/create),
// which nests the echoed order id under "details", not at the top level.
func TestBiteshipClient_CreateOrder_DuplicateReference(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"code":    40002060,
			"error":   "Reference id has already been used before. Please input other reference id",
			"details": map[string]interface{}{
				"order_id":     "biteship-order-existing",
				"waybill_id":   "1028309128390",
				"reference_id": "order-123",
			},
		})
	}))
	defer ts.Close()

	client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

	req := service.CreateShipmentRequest{ReferenceID: "order-123", CourierCode: "jne", ServiceCode: "reg"}

	_, err := client.CreateOrder(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error for duplicate reference_id, got nil")
	}
	if !errors.Is(err, service.ErrShipmentAlreadyBooked) {
		t.Fatalf("expected errors.Is(err, service.ErrShipmentAlreadyBooked), got: %v", err)
	}

	var alreadyBooked *service.ShipmentAlreadyBookedError
	if !errors.As(err, &alreadyBooked) {
		t.Fatalf("expected errors.As to find *service.ShipmentAlreadyBookedError, got: %v", err)
	}
	if alreadyBooked.ExistingBiteshipOrderID != "biteship-order-existing" {
		t.Errorf("expected echoed existing order id biteship-order-existing, got %q", alreadyBooked.ExistingBiteshipOrderID)
	}
}

// TestBiteshipClient_CreateOrder_DuplicateReference_TopLevelFallback covers
// the legacy/defensive shape: if Biteship (or a sandbox variant) ever echoes
// order_id at the top level instead of nested under "details", the id must
// still be recovered.
func TestBiteshipClient_CreateOrder_DuplicateReference_TopLevelFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"code":     40002060,
			"error":    "duplicate reference_id",
			"order_id": "biteship-order-existing-legacy",
		})
	}))
	defer ts.Close()

	client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

	req := service.CreateShipmentRequest{ReferenceID: "order-123", CourierCode: "jne", ServiceCode: "reg"}

	_, err := client.CreateOrder(t.Context(), req)
	var alreadyBooked *service.ShipmentAlreadyBookedError
	if !errors.As(err, &alreadyBooked) {
		t.Fatalf("expected errors.As to find *service.ShipmentAlreadyBookedError, got: %v", err)
	}
	if alreadyBooked.ExistingBiteshipOrderID != "biteship-order-existing-legacy" {
		t.Errorf("expected fallback to top-level order_id biteship-order-existing-legacy, got %q", alreadyBooked.ExistingBiteshipOrderID)
	}
}

// TestBiteshipClient_CreateOrder_DuplicateReference_NoIDEchoed proves that
// when neither details.order_id nor a top-level order_id is present,
// ExistingBiteshipOrderID comes back empty so the caller's own guard
// (shipping_order.go) still engages instead of calling GetOrder("").
func TestBiteshipClient_CreateOrder_DuplicateReference_NoIDEchoed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"code":    40002060,
			"error":   "duplicate reference_id",
		})
	}))
	defer ts.Close()

	client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

	req := service.CreateShipmentRequest{ReferenceID: "order-123", CourierCode: "jne", ServiceCode: "reg"}

	_, err := client.CreateOrder(t.Context(), req)
	var alreadyBooked *service.ShipmentAlreadyBookedError
	if !errors.As(err, &alreadyBooked) {
		t.Fatalf("expected errors.As to find *service.ShipmentAlreadyBookedError, got: %v", err)
	}
	if alreadyBooked.ExistingBiteshipOrderID != "" {
		t.Errorf("expected empty ExistingBiteshipOrderID when no id is echoed, got %q", alreadyBooked.ExistingBiteshipOrderID)
	}
}

func TestBiteshipClient_GetOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/orders/biteship-order-abc" {
			t.Errorf("expected path /v1/orders/biteship-order-abc, got %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "test-api-key" {
			t.Errorf("expected authorization header 'test-api-key', got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"id":      "biteship-order-abc",
			"status":  "delivered",
			"courier": map[string]interface{}{
				"waybill_id":  "JNE123456789",
				"driver_name": "Pak Agus",
			},
		})
	}))
	defer ts.Close()

	client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

	shipment, err := client.GetOrder(t.Context(), "biteship-order-abc")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if shipment.Status != "delivered" {
		t.Errorf("expected Status delivered, got %q", shipment.Status)
	}
	if shipment.WaybillID != "JNE123456789" {
		t.Errorf("expected WaybillID JNE123456789, got %q", shipment.WaybillID)
	}
	if shipment.CourierDriverName != "Pak Agus" {
		t.Errorf("expected CourierDriverName Pak Agus, got %q", shipment.CourierDriverName)
	}
}

// TestBiteshipClient_OrderResponse_ToleratesUpdatedAtFormats drives real
// httptest responses through CreateOrder and GetOrder for every plausible
// (and implausible) updated_at encoding Biteship might send. None of them may
// fail the parse — the waybill/order id/status must still come through.
// Unix seconds, "2006-01-02 15:04:05"-style strings and RFC 3339 are all
// genuinely parsed (they're plausible encodings); absent, null, and anything
// else fall back to the zero value so shipping_webhook.go's deterministic
// fallback chain gets a chance to run.
func TestBiteshipClient_OrderResponse_ToleratesUpdatedAtFormats(t *testing.T) {
	cases := []struct {
		name          string
		updatedAtJSON string // raw JSON literal spliced into the body; "" omits the key entirely
		wantZero      bool
		wantParsed    time.Time
	}{
		{
			name:          "unix integer",
			updatedAtJSON: `1722384000`,
			wantZero:      false,
			wantParsed:    time.Unix(1722384000, 0).UTC(),
		},
		{
			name:          "non-RFC3339 string",
			updatedAtJSON: `"2026-07-31 10:00:00"`,
			wantZero:      false,
			wantParsed:    time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
		},
		{name: "absent", updatedAtJSON: "", wantZero: true},
		{name: "null", updatedAtJSON: `null`, wantZero: true},
		{name: "unrecognisable garbage string", updatedAtJSON: `"not-a-timestamp"`, wantZero: true},
		{
			name:          "valid RFC3339",
			updatedAtJSON: `"2026-07-31T10:00:00Z"`,
			wantZero:      false,
			wantParsed:    time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"success":true,"id":"biteship-order-abc","status":"confirmed"`
			if tc.updatedAtJSON != "" {
				body += `,"updated_at":` + tc.updatedAtJSON
			}
			body += `,"courier":{"waybill_id":"JNE123456789","driver_name":"Pak Agus"}}`

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(body))
			}))
			defer ts.Close()

			client := NewBiteshipClient(&mockRepository{}, "test-api-key", ts.URL, http.DefaultClient)

			t.Run("CreateOrder", func(t *testing.T) {
				shipment, err := client.CreateOrder(t.Context(), service.CreateShipmentRequest{
					ReferenceID: "order-123", CourierCode: "jne", ServiceCode: "reg",
				})
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				assertOrderParsed(t, shipment, tc.wantZero, tc.wantParsed)
			})

			t.Run("GetOrder", func(t *testing.T) {
				shipment, err := client.GetOrder(t.Context(), "biteship-order-abc")
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				assertOrderParsed(t, shipment, tc.wantZero, tc.wantParsed)
			})
		})
	}
}

func assertOrderParsed(t *testing.T, shipment service.Shipment, wantZero bool, wantParsed time.Time) {
	t.Helper()
	if shipment.BiteshipOrderID != "biteship-order-abc" {
		t.Errorf("expected BiteshipOrderID biteship-order-abc, got %q", shipment.BiteshipOrderID)
	}
	if shipment.WaybillID != "JNE123456789" {
		t.Errorf("expected WaybillID JNE123456789, got %q", shipment.WaybillID)
	}
	if shipment.Status != "confirmed" {
		t.Errorf("expected Status confirmed, got %q", shipment.Status)
	}
	if wantZero {
		if !shipment.StatusUpdatedAt.IsZero() {
			t.Errorf("expected zero StatusUpdatedAt, got %v", shipment.StatusUpdatedAt)
		}
		return
	}
	if !shipment.StatusUpdatedAt.Equal(wantParsed) {
		t.Errorf("expected StatusUpdatedAt %v, got %v", wantParsed, shipment.StatusUpdatedAt)
	}
}

func TestParsePricing_ReadsCourierAndServiceCodes(t *testing.T) {
	c := &BiteshipClient{}
	rates := c.parsePricing([]biteshipPricingItem{
		{
			CourierName:        "JNE",
			CourierServiceName: "Reguler",
			CourierCode:        "jne",
			CourierServiceCode: "reg",
			Price:              10000,
			Duration:           "1 - 2 days",
		},
	})

	if len(rates) != 1 {
		t.Fatalf("want 1 rate, got %d", len(rates))
	}
	if rates[0].CourierCode != "jne" {
		t.Errorf("expected CourierCode jne, got %q", rates[0].CourierCode)
	}
	if rates[0].ServiceCode != "reg" {
		t.Errorf("expected ServiceCode reg, got %q", rates[0].ServiceCode)
	}
}

func TestNoopLogisticsClient_CreateOrderAndGetOrder(t *testing.T) {
	n := &service.NoopLogisticsClient{}

	if _, err := n.CreateOrder(t.Context(), service.CreateShipmentRequest{}); !errors.Is(err, service.ErrShippingUnavailable) {
		t.Fatalf("expected errors.Is(err, service.ErrShippingUnavailable), got: %v", err)
	}
	if _, err := n.GetOrder(t.Context(), "any-id"); !errors.Is(err, service.ErrShippingUnavailable) {
		t.Fatalf("expected errors.Is(err, service.ErrShippingUnavailable), got: %v", err)
	}
}
