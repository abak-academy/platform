package adapter

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/midtrans/midtrans-go"
)

// testHTTPClient routes SDK requests to the provided httptest server by
// rewriting Midtrans production/sandbox host prefixes to the test server URL.
type testHTTPClient struct {
	base      *midtrans.HttpClientImplementation
	serverURL string
}

func (t *testHTTPClient) Call(method string, url string, apiKey *string, options *midtrans.ConfigOptions, body io.Reader, result interface{}) *midtrans.Error {
	url = rewriteTestURL(url, t.serverURL)
	return t.base.Call(method, url, apiKey, options, body, result)
}

func rewriteTestURL(url, serverURL string) string {
	for _, prefix := range []string{
		"https://app.sandbox.midtrans.com",
		"https://app.midtrans.com",
		"https://api.sandbox.midtrans.com",
		"https://api.midtrans.com",
	} {
		if strings.HasPrefix(url, prefix) {
			return serverURL + strings.TrimPrefix(url, prefix)
		}
	}
	return url
}

func newTestMidtransClient(serverKey string, ts *httptest.Server) *MidtransClient {
	c := NewMidtransClient(serverKey, "test-client-key", "sandbox")
	wrapper := &testHTTPClient{
		base:      midtrans.GetHttpClient(midtrans.Sandbox),
		serverURL: ts.URL,
	}
	c.snap.HttpClient = wrapper
	c.core.HttpClient = wrapper
	return c
}

func TestMidtransClient_CreatePayment(t *testing.T) {
	serverKey := "test-server-key"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/snap/v1/transactions" {
			t.Errorf("expected path /snap/v1/transactions, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":        "snap-token-123",
			"redirect_url": "https://pay.example.com/redirect",
		})
	}))
	defer ts.Close()

	client := newTestMidtransClient(serverKey, ts)
	req := service.PaymentRequest{
		OrderID:   "order-123",
		Reference: "ref-123",
		Amount:    100000,
		ExpiresIn: 30 * time.Minute,
	}

	resp, err := client.CreatePayment(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GatewayRef != req.OrderID {
		t.Errorf("GatewayRef: got %q, want %q", resp.GatewayRef, req.OrderID)
	}
	if resp.SnapToken != "snap-token-123" {
		t.Errorf("SnapToken: got %q, want %q", resp.SnapToken, "snap-token-123")
	}
	if resp.PaymentURL != "https://pay.example.com/redirect" {
		t.Errorf("PaymentURL: got %q, want %q", resp.PaymentURL, "https://pay.example.com/redirect")
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("expected non-zero ExpiresAt")
	}
	wantExpiresAt := time.Now().Add(req.ExpiresIn)
	if resp.ExpiresAt.Before(wantExpiresAt.Add(-5*time.Second)) || resp.ExpiresAt.After(wantExpiresAt.Add(5*time.Second)) {
		t.Errorf("ExpiresAt out of range: got %v, want around %v", resp.ExpiresAt, wantExpiresAt)
	}
}

func TestMidtransClient_CreatePayment_APIError(t *testing.T) {
	serverKey := "test-server-key"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	client := newTestMidtransClient(serverKey, ts)
	req := service.PaymentRequest{
		OrderID:   "order-456",
		Reference: "ref-456",
		Amount:    50000,
		ExpiresIn: 30 * time.Minute,
	}

	resp, err := client.CreatePayment(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != (service.PaymentResponse{}) {
		t.Errorf("expected empty PaymentResponse, got %+v", resp)
	}
}

func TestMidtransClient_VerifySignature(t *testing.T) {
	serverKey := "secret-server-key"
	orderID := "order-789"
	statusCode := "200"
	grossAmount := "100000.00"

	payload, err := json.Marshal(map[string]string{
		"order_id":     orderID,
		"status_code":  statusCode,
		"gross_amount": grossAmount,
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	client := NewMidtransClient(serverKey, "test-client-key", "sandbox")
	h := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	correctSig := hex.EncodeToString(h[:])
	tamperedSig := correctSig + "tampered"

	if !client.VerifySignature(payload, correctSig) {
		t.Error("expected VerifySignature to return true for correct signature")
	}
	if client.VerifySignature(payload, tamperedSig) {
		t.Error("expected VerifySignature to return false for tampered signature")
	}
}

func TestMidtransClient_QueryStatus(t *testing.T) {
	serverKey := "test-server-key"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v2/") || !strings.HasSuffix(r.URL.Path, "/status") {
			t.Errorf("expected path /v2/{order_id}/status, got %q", r.URL.Path)
		}

		status := strings.TrimPrefix(r.URL.Path, "/v2/")
		status = strings.TrimSuffix(status, "/status")

		var transactionStatus string
		switch status {
		case "order-settled":
			transactionStatus = "settlement"
		case "order-pending":
			transactionStatus = "pending"
		default:
			t.Fatalf("unexpected order id in status query: %q", status)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"transaction_status": transactionStatus,
		})
	}))
	defer ts.Close()

	client := newTestMidtransClient(serverKey, ts)

	settled, err := client.QueryStatus(context.Background(), "order-settled")
	if err != nil {
		t.Fatalf("expected no error for settlement, got %v", err)
	}
	if !settled.Paid {
		t.Error("expected Paid=true for settlement")
	}
	if settled.Reference != "order-settled" {
		t.Errorf("Reference: got %q, want %q", settled.Reference, "order-settled")
	}

	pending, err := client.QueryStatus(context.Background(), "order-pending")
	if err != nil {
		t.Fatalf("expected no error for pending, got %v", err)
	}
	if pending.Paid {
		t.Error("expected Paid=false for pending")
	}
	if pending.Reference != "order-pending" {
		t.Errorf("Reference: got %q, want %q", pending.Reference, "order-pending")
	}
}
