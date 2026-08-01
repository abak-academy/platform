package handler

import (
	"bytes"
	"net/http"
	"strings"

	"akademi-bimbel/internal/repository"
	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminListOrders(c echo.Context) error {
	cursor := c.QueryParam("cursor")
	status := c.QueryParam("status")
	productType := c.QueryParam("type")
	limit := 20

	filter := repository.OrderFilter{
		Status:      status,
		ProductType: productType,
		Cursor:      cursor,
		Limit:       limit,
	}

	orders, nextCursor, err := h.svc.AdminListOrders(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        orders,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) AdminGetOrder(c echo.Context) error {
	orderID := c.Param("id")

	order, err := h.svc.AdminGetOrder(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, order)
}

func (h *Handler) AdminConfirmOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	var req struct {
		PaymentProofURL string `json:"payment_proof_url"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if !validPaymentProofKey(req.PaymentProofURL) {
		return badRequest(c, "payment_proof_url is required and must be a payment_proof/ upload key")
	}

	err := h.svc.AdminConfirmOrder(c.Request().Context(), actorID, orderID, key, req.PaymentProofURL)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order confirmed",
	})
}

// validPaymentProofKey enforces invariant 6 (FR-25/FR-26): a confirmed order
// must never exist without its evidence, and the evidence key must resolve
// inside the payment_proof/ upload prefix with no path-traversal segment.
func validPaymentProofKey(key string) bool {
	if key == "" {
		return false
	}
	if strings.Contains(key, "..") {
		return false
	}
	return strings.HasPrefix(key, "payment_proof/")
}

func (h *Handler) AdminShipOrder(c echo.Context) error {
	orderID := c.Param("id")

	err := h.svc.AdminShipOrder(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order shipped",
	})
}

// AdminGetShippingLabel streams a printable packing slip PDF for an order's
// waybill (FR-D-1..D-4) — a packing slip, not the scannable carrier label,
// which comes from Biteship's dashboard.
func (h *Handler) AdminGetShippingLabel(c echo.Context) error {
	orderID := c.Param("id")

	pdf, err := h.svc.GetShippingLabel(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	return c.Stream(http.StatusOK, "application/pdf", bytes.NewReader(pdf))
}

func (h *Handler) AdminShipOrderManual(c echo.Context) error {
	orderID := c.Param("id")

	var req struct {
		TrackingNumber string `json:"tracking_number"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.TrackingNumber == "" {
		return badRequest(c, "tracking_number is required")
	}

	err := h.svc.AdminShipOrderManual(c.Request().Context(), orderID, req.TrackingNumber)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order shipped",
	})
}

func (h *Handler) AdminCompleteOrder(c echo.Context) error {
	orderID := c.Param("id")

	err := h.svc.AdminCompleteOrder(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order completed",
	})
}

func (h *Handler) AdminRefundOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")

	err := h.svc.AdminRefundOrder(c.Request().Context(), actorID, orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order refunded",
	})
}

func (h *Handler) AdminReconcileOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	err := h.svc.AdminReconcileOrder(c.Request().Context(), actorID, orderID, key)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order reconciled",
	})
}
