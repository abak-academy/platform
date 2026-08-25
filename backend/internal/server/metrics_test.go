package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"akademi-bimbel/internal/metrics"
)

func TestMetricsMiddlewareRecordsRouteTemplate(t *testing.T) {
	e := echo.New()
	e.GET("/api/v1/exams/:id", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.Use(MetricsMiddleware())

	res := httptest.NewRecorder()
	e.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/exams/abc-123", nil))

	got := metrics.HTTPRequestsTotal.WithLabelValues("/api/v1/exams/:id", "GET", "200")
	if n := testutil.ToFloat64(got); n != 1 {
		t.Errorf("counter for route template = %v, want 1", n)
	}
}

// A handler returning an error is resolved through the global HTTPErrorHandler
// after this middleware unwinds — the fallback must not record such requests
// as 200, or the 5xx rate hides real failures.
func TestMetricsMiddlewareDerivesStatusFromReturnedError(t *testing.T) {
	e := echo.New()
	e.GET("/boom", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot)
	})
	e.Use(MetricsMiddleware())

	res := httptest.NewRecorder()
	e.ServeHTTP(res, httptest.NewRequest("GET", "/boom", nil))

	got := metrics.HTTPRequestsTotal.WithLabelValues("/boom", "GET", "418")
	if n := testutil.ToFloat64(got); n != 1 {
		t.Errorf("counter with derived status = %v, want 1", n)
	}
}

func TestMetricsMiddlewareLabelsUnmatchedRoutes(t *testing.T) {
	e := echo.New()
	e.Use(MetricsMiddleware())

	res := httptest.NewRecorder()
	e.ServeHTTP(res, httptest.NewRequest("GET", "/nowhere", nil))

	got := metrics.HTTPRequestsTotal.WithLabelValues("unmatched", "GET", "404")
	if n := testutil.ToFloat64(got); n != 1 {
		t.Errorf("unmatched counter = %v, want 1", n)
	}
}
