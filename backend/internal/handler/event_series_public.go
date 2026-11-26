package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const eventSeriesCacheControl = "public, max-age=300"

// ListEventSeries backs GET /api/v1/event-series.
func (h *Handler) ListEventSeries(c echo.Context) error {
	events, err := h.svc.ListPublicEventSeries(c.Request().Context(), c.QueryParam("lang"))
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Cache-Control", eventSeriesCacheControl)
	return c.JSON(http.StatusOK, map[string]any{"data": events})
}

// GetEventSeries backs GET /api/v1/event-series/:slug.
func (h *Handler) GetEventSeries(c echo.Context) error {
	event, err := h.svc.GetPublicEventSeries(c.Request().Context(), c.Param("slug"), c.QueryParam("lang"))
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Cache-Control", eventSeriesCacheControl)
	return c.JSON(http.StatusOK, event)
}
