package adapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestBiteshipClient_CreateOrder_DuplicateReference(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"code":     40002060,
			"error":    "duplicate reference_id",
			"order_id": "biteship-order-existing",
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
