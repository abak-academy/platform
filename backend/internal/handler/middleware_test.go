package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestLoginRateLimiter_DenyHandler_RetryAfter(t *testing.T) {
	e := echo.New()
	e.Use(LoginRateLimiter())
	e.GET("/login", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	var rec *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.RemoteAddr = "203.0.113.5:12345"
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}

	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("want Retry-After %q, got %q", "1", got)
	}

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "rate_limited", body["code"])
	require.Equal(t, "too many login attempts", body["message"])
}
