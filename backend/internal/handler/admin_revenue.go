package handler

import (
	"net/http"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminGetRevenue(c echo.Context) error {
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	var from, to time.Time
	var err error

	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return badRequest(c, "invalid from date format (use YYYY-MM-DD)")
		}
	}

	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return badRequest(c, "invalid to date format (use YYYY-MM-DD)")
		}
	}

	now := time.Now().UTC()
	if from.IsZero() {
		from = now.AddDate(0, 0, -30)
	}
	if to.IsZero() {
		to = now
	}

	revenue, err := h.svc.AdminGetRevenue(c.Request().Context(), from, to)
	if err != nil {
		return mapServiceError(c, err)
	}

	products, err := h.svc.AdminTopProducts(c.Request().Context(), from, to, "revenue", 10)
	if err != nil {
		return mapServiceError(c, err)
	}
	revenue["top_products"] = products

	// Echoed back so the page can state the period it is actually showing. It
	// previously rendered this default 30-day window with no period at all,
	// which read as an all-time figure.
	revenue["from"] = from.Format("2006-01-02")
	revenue["to"] = to.Format("2006-01-02")

	return c.JSON(http.StatusOK, revenue)
}

func (h *Handler) AdminListNotifications(c echo.Context) error {
	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Role == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}

	cursor := c.QueryParam("cursor")
	notifType := c.QueryParam("type")
	unreadOnly := c.QueryParam("unread_only") == "true"
	limit := 20

	filter := service.NotifFilter{
		Type:       notifType,
		UnreadOnly: unreadOnly,
		Cursor:     cursor,
		Limit:      limit,
	}

	notifications, nextCursor, err := h.svc.ListNotifications(c.Request().Context(), claims.Role, filter)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        notifications,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) AdminMarkNotificationRead(c echo.Context) error {
	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Role == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}

	notifID := c.Param("id")

	err := h.svc.MarkNotificationRead(c.Request().Context(), claims.Role, notifID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "notification marked read",
	})
}
