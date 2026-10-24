package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// affiliateClickRequest is the body only the public click endpoint reads.
type affiliateClickRequest struct {
	Code      string `json:"code"`
	UserAgent string `json:"user_agent"`
}

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

// AffiliateRecordClick backs POST /api/v1/affiliates/clicks — the public click
// statistics recorder for /r/{code}.
func (h *Handler) AffiliateRecordClick(c echo.Context) error {
	var req affiliateClickRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	recorded, err := h.svc.RecordAffiliateClick(c.Request().Context(), req.Code, c.RealIP(), req.UserAgent)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"recorded": recorded})
}

// AffiliateClickRateLimiter mirrors LoginRateLimiter for the public affiliate
// surfaces; its constants stay matched to LoginRateLimiter's.
func AffiliateClickRateLimiter() echo.MiddlewareFunc {
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
