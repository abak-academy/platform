package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// AffiliateValidateCode backs GET /api/v1/affiliates/public/validate — the
// register/checkout UX check. Unknown or inactive codes answer valid:false
// with 200 so the endpoint cannot be used to enumerate accounts.
func (h *Handler) AffiliateValidateCode(c echo.Context) error {
	res, err := h.svc.ValidateAffiliateCode(c.Request().Context(), c.QueryParam("code"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

// AffiliateProgramStatus backs GET /api/v1/affiliates/public/status: whether
// sign-up and referral links are currently open, so the web can hide them.
func (h *Handler) AffiliateProgramStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"enabled": h.svc.AffiliateProgramEnabled(c.Request().Context())})
}

// AffiliateRateLimiter mirrors LoginRateLimiter for the public affiliate
// surfaces; its constants stay matched to LoginRateLimiter's.
func AffiliateRateLimiter() echo.MiddlewareFunc {
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Skipper: echomw.DefaultSkipper,
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(
			echomw.RateLimiterMemoryStoreConfig{
				Rate:      10,
				Burst:     10,
				ExpiresIn: 1 * time.Minute,
			},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"code":    "rate_limited",
				"message": "too many requests",
			})
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			c.Response().Header().Set("Retry-After", "1")
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"code":    "rate_limited",
				"message": "too many requests",
			})
		},
	})
}
