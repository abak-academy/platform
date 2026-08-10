package handler

import (
	"net/http"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminGetRevenue(c echo.Context) error {
	// Shares parseDayRange with the order list: dates are read in Asia/Jakarta
	// and `to` is advanced a day, because the revenue queries are half-open on
	// created_at. Parsed as bare UTC midnight, picking 5 Aug excluded every
	// order placed on 5 Aug.
	fromParam, toParam, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}

	// Midnight-aligned and exclusive, matching GET /admin/dashboard. This was
	// a bare now-30d..now instant window, so "30 hari" meant the last 720
	// hours and today counted only up to the moment the request landed — the
	// two pages reported different totals for the same preset label. `to` is
	// today+1 exclusive, so a window of exactly 30 days starts at today-29.
	now := time.Now().In(jakarta)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakarta)
	from := today.AddDate(0, 0, -29)
	if fromParam != nil {
		from = *fromParam
	}
	to := today.AddDate(0, 0, 1)
	if toParam != nil {
		to = *toParam
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
	// `to` is exclusive, so echo back the last day actually included — otherwise
	// the page's period label reads one day later than the data.
	// `to` is exclusive in both branches now, so one expression covers them —
	// it no longer needs to special-case a default that was a bare instant.
	revenue["from"] = from.Format("2006-01-02")
	revenue["to"] = to.AddDate(0, 0, -1).Format("2006-01-02")

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
