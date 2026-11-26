package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/service"
)

func (h *Handler) AdminListTestimonials(c echo.Context) error {
	items, err := h.svc.AdminListTestimonials(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) AdminCreateTestimonial(c echo.Context) error {
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing actor"})
	}
	var req service.TestimonialPatch
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	t, err := h.svc.AdminCreateTestimonial(c.Request().Context(), actor, req)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, t)
}

func (h *Handler) AdminUpdateTestimonial(c echo.Context) error {
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing actor"})
	}
	var req service.TestimonialPatch
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	t, err := h.svc.AdminUpdateTestimonial(c.Request().Context(), actor, c.Param("id"), req)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

func (h *Handler) AdminDeleteTestimonial(c echo.Context) error {
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing actor"})
	}
	if err := h.svc.AdminDeleteTestimonial(c.Request().Context(), actor, c.Param("id")); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
