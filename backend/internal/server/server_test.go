package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/metrics"
	"akademi-bimbel/internal/service"
)

// A panic in a handler must still land in http_requests_total as a 500 and
// still return 500 to the client. This holds only while MetricsMiddleware is
// registered OUTSIDE Recover() in New()'s middleware order — a regression
// that moves it inner again makes the runbook's status=~"5.." alert blind to
// the loudest failure class (reproduced during PR review).
func TestPanicRouteRecordedAs500ByMetrics(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	signer := infra.NewJWTSigner("test-secret", time.Hour)
	svc := service.NewForTest(rdb)

	e := New(handler.New(svc), svc, signer, config.Config{})
	e.GET("/panic", func(c echo.Context) error { panic("boom") })

	res := httptest.NewRecorder()
	e.ServeHTTP(res, httptest.NewRequest("GET", "/panic", nil))

	if res.Code != http.StatusInternalServerError {
		t.Errorf("panic route status = %d, want 500", res.Code)
	}
	got := metrics.HTTPRequestsTotal.WithLabelValues("/panic", "GET", "500")
	if n := testutil.ToFloat64(got); n != 1 {
		t.Errorf("http_requests_total for panic route = %v, want 1 (metrics middleware is inner to Recover)", n)
	}
}
