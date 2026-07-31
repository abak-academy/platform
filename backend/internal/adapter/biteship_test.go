package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

// mockRepository implements configReader for testing.
type mockRepository struct {
	systemConfigRows []repository.SystemConfigRow
}

func (m *mockRepository) ListSystemConfig(ctx context.Context) ([]repository.SystemConfigRow, error) {
	return m.systemConfigRows, nil
}

func TestBiteshipClient_GetRates_Success(t *testing.T) {
	// Test data
	originPostalCode := "12440"
	destPostalCode := "12240"
	weightGrams := 1000

	mockRepo := &mockRepository{
		systemConfigRows: []repository.SystemConfigRow{
			{
				Key:   "app_kode_pos",
				Value: originPostalCode,
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/rates/couriers" {
			t.Errorf("expected path /v1/rates/couriers, got %s", r.URL.Path)
		}

		// Verify authorization header
		authHeader := r.Header.Get("authorization")
		if authHeader != "test-api-key" {
			t.Errorf("expected authorization header 'test-api-key', got %q", authHeader)
		}

		// Verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if reqBody["origin_postal_code"] != originPostalCode {
			t.Errorf("expected origin_postal_code %q, got %v", originPostalCode, reqBody["origin_postal_code"])
		}
		if reqBody["destination_postal_code"] != destPostalCode {
			t.Errorf("expected destination_postal_code %q, got %v", destPostalCode, reqBody["destination_postal_code"])
		}

		// Verify items array
		itemsRaw := reqBody["items"]
		itemsInterface := itemsRaw.([]interface{})
		if len(itemsInterface) != 1 {
			t.Errorf("expected 1 item, got %d", len(itemsInterface))
		}

		// Verify couriers parameter
		couriers := reqBody["couriers"]
		if couriers == nil || couriers == "" {
			t.Error("expected couriers parameter to be set")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"pricing": []map[string]interface{}{
				{
					"courier_name":         "JNE",
					"courier_service_name": "Regular",
					"price":                15000,
					"duration":             "3",
				},
				{
					"courier_name":         "TIKI",
					"courier_service_name": "OnS",
					"price":                25000,
					"duration":             "1",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewBiteshipClient(
		mockRepo,
		"test-api-key",
		ts.URL,
		http.DefaultClient,
	)

	req := service.ShippingQuoteRequest{
		DestinationPostalCode: destPostalCode,
		WeightGrams:           weightGrams,
	}

	rates, err := client.GetRates(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(rates) != 2 {
		t.Errorf("expected 2 rates, got %d", len(rates))
	}

	if rates[0].Courier != "JNE" {
		t.Errorf("expected first courier JNE, got %s", rates[0].Courier)
	}
	if rates[0].Service != "Regular" {
		t.Errorf("expected first service Regular, got %s", rates[0].Service)
	}
	if rates[0].EstimatedDays != 3 {
		t.Errorf("expected first EstimatedDays=3, got %d", rates[0].EstimatedDays)
	}
	if rates[0].Price != 15000 {
		t.Errorf("expected first Price=15000, got %d", rates[0].Price)
	}

	if rates[1].Courier != "TIKI" {
		t.Errorf("expected second courier TIKI, got %s", rates[1].Courier)
	}
	if rates[1].Service != "OnS" {
		t.Errorf("expected second service OnS, got %s", rates[1].Service)
	}
	if rates[1].EstimatedDays != 1 {
		t.Errorf("expected second EstimatedDays=1, got %d", rates[1].EstimatedDays)
	}
	if rates[1].Price != 25000 {
		t.Errorf("expected second Price=25000, got %d", rates[1].Price)
	}
}

func TestBiteshipClient_GetRates_MissingOriginConfig(t *testing.T) {
	mockRepo := &mockRepository{
		systemConfigRows: []repository.SystemConfigRow{
			// No app_kode_pos configured
		},
	}

	client := NewBiteshipClient(
		mockRepo,
		"test-api-key",
		"https://api.biteship.com",
		http.DefaultClient,
	)

	req := service.ShippingQuoteRequest{
		DestinationPostalCode: "12240",
		WeightGrams:           1000,
	}

	rates, err := client.GetRates(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing origin postal code, got nil")
	}
	if len(rates) != 0 {
		t.Errorf("expected empty rates on error, got %d", len(rates))
	}
	if !strings.Contains(err.Error(), "app_kode_pos") {
		t.Errorf("expected error to mention app_kode_pos, got: %v", err)
	}
}

func TestBiteshipClient_GetRates_APIError(t *testing.T) {
	mockRepo := &mockRepository{
		systemConfigRows: []repository.SystemConfigRow{
			{
				Key:   "app_kode_pos",
				Value: "12440",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal Server Error"})
	}))
	defer ts.Close()

	client := NewBiteshipClient(
		mockRepo,
		"test-api-key",
		ts.URL,
		http.DefaultClient,
	)

	req := service.ShippingQuoteRequest{
		DestinationPostalCode: "12240",
		WeightGrams:           1000,
	}

	rates, err := client.GetRates(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}
	if len(rates) != 0 {
		t.Errorf("expected empty rates on error, got %d", len(rates))
	}
}

func TestBiteshipClient_GetRates_InvalidJSON(t *testing.T) {
	mockRepo := &mockRepository{
		systemConfigRows: []repository.SystemConfigRow{
			{
				Key:   "app_kode_pos",
				Value: "12440",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	client := NewBiteshipClient(
		mockRepo,
		"test-api-key",
		ts.URL,
		http.DefaultClient,
	)

	req := service.ShippingQuoteRequest{
		DestinationPostalCode: "12240",
		WeightGrams:           1000,
	}

	rates, err := client.GetRates(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if len(rates) != 0 {
		t.Errorf("expected empty rates on error, got %d", len(rates))
	}
}

func TestBiteshipClient_GetRates_OriginUnset(t *testing.T) {
	mockRepo := &mockRepository{
		systemConfigRows: []repository.SystemConfigRow{
			// No app_kode_pos configured
		},
	}

	client := NewBiteshipClient(
		mockRepo,
		"test-api-key",
		"https://api.biteship.com",
		http.DefaultClient,
	)

	req := service.ShippingQuoteRequest{
		DestinationPostalCode: "12240",
		WeightGrams:           1000,
	}

	_, err := client.GetRates(context.Background(), req)
	if !errors.Is(err, service.ErrShippingOriginUnset) {
		t.Fatalf("expected errors.Is(err, service.ErrShippingOriginUnset), got: %v", err)
	}
}

func TestBiteshipClient_GetRates_AuthRejected(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			mockRepo := &mockRepository{
				systemConfigRows: []repository.SystemConfigRow{
					{Key: "app_kode_pos", Value: "12440"},
				},
			}

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte("unauthorized: invalid api key"))
			}))
			defer ts.Close()

			client := NewBiteshipClient(
				mockRepo,
				"bad-api-key",
				ts.URL,
				http.DefaultClient,
			)

			req := service.ShippingQuoteRequest{
				DestinationPostalCode: "12240",
				WeightGrams:           1000,
			}

			_, err := client.GetRates(context.Background(), req)
			if !errors.Is(err, service.ErrShippingAuthRejected) {
				t.Fatalf("expected errors.Is(err, service.ErrShippingAuthRejected), got: %v", err)
			}
			if !strings.Contains(err.Error(), "unauthorized: invalid api key") {
				t.Errorf("expected error message to carry response body, got: %v", err)
			}
		})
	}
}

func TestBiteshipClient_GetRates_ItemValue(t *testing.T) {
	tests := []struct {
		name          string
		itemValue     int64
		expectedValue float64
	}{
		{"nonzero ItemValue carries through", 250000, 250000},
		{"zero ItemValue falls back to 1", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockRepository{
				systemConfigRows: []repository.SystemConfigRow{
					{Key: "app_kode_pos", Value: "12440"},
				},
			}

			var capturedBody map[string]interface{}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if err := json.Unmarshal(body, &capturedBody); err != nil {
					t.Fatalf("failed to unmarshal request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"pricing": []map[string]interface{}{}})
			}))
			defer ts.Close()

			client := NewBiteshipClient(
				mockRepo,
				"test-api-key",
				ts.URL,
				http.DefaultClient,
			)

			req := service.ShippingQuoteRequest{
				DestinationPostalCode: "12240",
				WeightGrams:           1000,
				ItemValue:             tt.itemValue,
			}

			if _, err := client.GetRates(context.Background(), req); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			items, ok := capturedBody["items"].([]interface{})
			if !ok || len(items) != 1 {
				t.Fatalf("expected 1 item in request body, got %v", capturedBody["items"])
			}
			item, ok := items[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected item to be an object, got %v", items[0])
			}
			if item["value"] != tt.expectedValue {
				t.Errorf("expected items[0].value=%v, got %v", tt.expectedValue, item["value"])
			}
		})
	}
}
