package handler

import (
	"net/http"
	"slices"

	"akademi-bimbel/internal/repository"

	"github.com/labstack/echo/v4"
)

type partnerOrderRequest struct {
	ProductIDs []string `json:"product_ids"`
	StudentIDs []string `json:"student_ids"`
	PromoCode  string   `json:"promo_code"`
}

func bindPartnerOrder(c echo.Context) (partnerOrderRequest, error) {
	var req partnerOrderRequest
	if err := c.Bind(&req); err != nil {
		return req, badRequest(c, "invalid request body")
	}
	if len(req.ProductIDs) == 0 || len(req.StudentIDs) == 0 {
		return req, badRequest(c, "product_ids and student_ids are required")
	}
	return req, nil
}

// PartnerPreviewOrder backs POST /partners/me/orders/preview.
func (h *Handler) PartnerPreviewOrder(c echo.Context) error {
	req, err := bindPartnerOrder(c)
	if err != nil || len(req.ProductIDs) == 0 || len(req.StudentIDs) == 0 {
		return err
	}
	claims := ClaimsFromContext(c)
	preview, err := h.svc.PreviewPartnerOrder(c.Request().Context(), claims.Sub, req.ProductIDs, req.StudentIDs, req.PromoCode)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, preview)
}

// PartnerCreateOrder backs POST /partners/me/orders.
func (h *Handler) PartnerCreateOrder(c echo.Context) error {
	req, err := bindPartnerOrder(c)
	if err != nil || len(req.ProductIDs) == 0 || len(req.StudentIDs) == 0 {
		return err
	}
	claims := ClaimsFromContext(c)
	order, err := h.svc.CreatePartnerOrder(c.Request().Context(), claims.Sub, req.ProductIDs, req.StudentIDs, req.PromoCode)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, order)
}

var partnerOrderStatuses = []string{"cart", "payment_pending", "paid", "processing", "shipped", "completed", "payment_expired", "cancelled"}

// PartnerListOrders backs GET /partners/me/orders.
func (h *Handler) PartnerListOrders(c echo.Context) error {
	claims := ClaimsFromContext(c)
	status := c.QueryParam("status")
	if status != "" && !slices.Contains(partnerOrderStatuses, status) {
		return badRequest(c, "unknown order status")
	}
	page, perPage, err := adminQueuePaging(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}
	orders, total, err := h.svc.ListPartnerOrders(c.Request().Context(), claims.Sub,
		repository.PartnerOrderFilter{Status: status, Q: c.QueryParam("q"), From: from, To: to}, page, perPage)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(orders, total, page, perPage))
}

// PartnerGetOrder backs GET /partners/me/orders/:id.
func (h *Handler) PartnerGetOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	order, err := h.svc.GetPartnerOrder(c.Request().Context(), claims.Sub, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, order)
}

// PartnerCheckoutOrder backs POST /partners/me/orders/:id/checkout.
func (h *Handler) PartnerCheckoutOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}
	result, err := h.svc.CheckoutPartnerOrder(c.Request().Context(), claims.Sub, c.Param("id"), key)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"gateway_ref":        result.GatewayRef,
		"snap_token":         result.SnapToken,
		"payment_url":        result.PaymentURL,
		"payment_expires_at": result.PaymentExpiresAt,
		"free":               result.Free,
	})
}

// PartnerRetryOrder backs POST /partners/me/orders/:id/retry.
func (h *Handler) PartnerRetryOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}
	result, err := h.svc.RetryPartnerOrder(c.Request().Context(), claims.Sub, c.Param("id"), key)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"gateway_ref":        result.GatewayRef,
		"snap_token":         result.SnapToken,
		"payment_url":        result.PaymentURL,
		"payment_expires_at": result.PaymentExpiresAt,
	})
}

// PartnerDashboard backs GET /partners/me/dashboard.
func (h *Handler) PartnerDashboard(c echo.Context) error {
	claims := ClaimsFromContext(c)
	dashboard, err := h.svc.GetPartnerDashboard(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, dashboard)
}
