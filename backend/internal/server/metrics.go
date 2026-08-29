package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/metrics"
)

// MetricsMiddleware records §2 of issue #98: request rate, duration and error
// rate per route template.
//
// The status fallback exists because Echo's global HTTPErrorHandler runs
// outside the middleware chain: when a handler returns an error, this
// middleware sees err != nil while Response().Status still holds its default
// 200. Deriving from the echo.HTTPError keeps the 5xx rate honest instead of
// hiding errors inside a "successful" series.
func MetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			status := c.Response().Status
			if err != nil && status == http.StatusOK {
				var he *echo.HTTPError
				if errors.As(err, &he) {
					status = he.Code
				} else {
					status = http.StatusInternalServerError
				}
			}

			route := c.Path()
			if route == "" {
				route = "unmatched"
			}
			method := c.Request().Method
			metrics.HTTPRequestsTotal.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
			return err
		}
	}
}
