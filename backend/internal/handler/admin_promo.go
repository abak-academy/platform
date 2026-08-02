package handler

import (
	"net/http"
	"time"

	"akademi-bimbel/internal/model"
	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminListPromoCodes(c echo.Context) error {
	promoCodes, err := h.svc.AdminListPromoCodes(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": promoCodes,
	})
}

func (h *Handler) AdminCreatePromoCode(c echo.Context) error {
	var req struct {
		Code              string     `json:"code"`
		DiscountPercent   *float64   `json:"discount_percent"`
		DiscountAmount    *float64   `json:"discount_amount"`
		MaxDiscountAmount *float64   `json:"max_discount_amount"`
		MinOrderAmount    *float64   `json:"min_order_amount"`
		MaxUses           *int       `json:"max_uses"`
		ExpiresAt         *time.Time `json:"expires_at"`
		IsPublic          bool       `json:"is_public"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Code == "" {
		return badRequest(c, "code is required")
	}

	promo := model.PromoCode{
		Code:              req.Code,
		DiscountPercent:   req.DiscountPercent,
		DiscountAmount:    req.DiscountAmount,
		MaxDiscountAmount: req.MaxDiscountAmount,
		MinOrderAmount:    req.MinOrderAmount,
		MaxUses:           req.MaxUses,
		ExpiresAt:         req.ExpiresAt,
		IsPublic:          req.IsPublic,
	}

	created, err := h.svc.AdminCreatePromoCode(c.Request().Context(), promo)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusCreated, created)
}

func (h *Handler) AdminUpdatePromoCode(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		MaxUses   *int       `json:"max_uses"`
		ExpiresAt *time.Time `json:"expires_at"`
		IsPublic  bool       `json:"is_public"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	err := h.svc.AdminUpdatePromoCode(c.Request().Context(), id, req.MaxUses, req.ExpiresAt, req.IsPublic)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "promo code updated",
	})
}

func (h *Handler) AdminDeletePromoCode(c echo.Context) error {
	id := c.Param("id")

	err := h.svc.AdminDeletePromoCode(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}
