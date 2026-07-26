package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func TestPaymentWebhook_MissingTransactionID_ReturnsBadRequest(t *testing.T) {
	// Setup
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	svc := service.NewForTest(rdb)
	h := handler.New(svc)

	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/webhooks/payment",
		bytes.NewBufferString(`{"payment_ref": "pay_123"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Act
	err = h.HandlePaymentWebhook(c)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
