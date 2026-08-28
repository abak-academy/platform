package server

import (
	"akademi-bimbel/config"
	"log/slog"
	"strings"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func New(h *handler.Handler, svc *service.Service, jwtSigner *infra.JWTSigner, cfg config.Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Ordering contract: Echo runs first-registered middleware outermost, and a
	// panic unwinds past everything INNER to Recover(). MetricsMiddleware and
	// the request logger are therefore registered BEFORE Recover() so a panic
	// still lands in http_requests_total as a 500 and still produces its one
	// Info line — the runbook's status=~"5.." alert and decision #5 depend on
	// both. Recover() itself must stay inner to catch the panic and convert it
	// to a 500 response the outer middleware can record.
	e.Use(MetricsMiddleware())
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORSOrigins,
	}))
	// Structured request log, kept for every request (cheap in Loki, and the
	// status code per request is what correlates a slow exam window with its
	// cause). Enriched per the "invoked params, then errors only" convention:
	// route template, path params, latency and request id ride along so a log
	// line answers "which endpoint, which entity, how long" without grep.
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURI:       true,
		LogMethod:    true,
		LogLatency:   true,
		LogRoutePath: true,
		LogRequestID: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []any{
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.Int64("duration_ms", v.Latency.Milliseconds()),
				slog.String("route", v.RoutePath),
				slog.String("request_id", v.RequestID),
			}
			if params := routeParams(c); params != "" {
				attrs = append(attrs, slog.String("params", params))
			}
			slog.Info("request", attrs...)
			return nil
		},
	}))

	registerRoutes(e, h, svc, jwtSigner)
	return e
}

// routeParams renders path parameters as "name=value" pairs for the request
// log. Route params in this API are identifiers (session/exam/order ids), so
// they are safe to log and are exactly what turns "PATCH .../answers is slow"
// into "this session's answers are slow".
func routeParams(c echo.Context) string {
	names := c.ParamNames()
	values := c.ParamValues()
	pairs := make([]string, 0, len(names))
	for i := range names {
		pairs = append(pairs, names[i]+"="+values[i])
	}
	return strings.Join(pairs, ",")
}
