package handler

import (
	"net/http"

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

	err := h.svc.AdminConfirmOrder(c.Request().Context(), actorID, orderID, key)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order confirmed",
	})
}

func (h *Handler) AdminShipOrder(c echo.Context) error {
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

	// AdminShipOrder now auto-books via Biteship; this route stays on the manual escape hatch until Task 12's HTTP surface split.
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
	orderID := c.Param("id")
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	err := h.svc.AdminReconcileOrder(c.Request().Context(), orderID, key)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order reconciled",
	})
}
