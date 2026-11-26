package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ListTestimonials backs GET /api/v1/testimonials.
func (h *Handler) ListTestimonials(c echo.Context) error {
	items, err := h.svc.ListPublicTestimonials(c.Request().Context(), c.QueryParam("lang"))
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Cache-Control", eventSeriesCacheControl)
	return c.JSON(http.StatusOK, map[string]any{"data": items})
}
