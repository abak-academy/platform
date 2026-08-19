package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminExamDashboard(c echo.Context) error {
	resp, err := h.svc.ExamDashboard(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) AdminSchoolDashboard(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	resp, err := h.svc.SchoolDashboard(c.Request().Context(), schoolID, claims.Role)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
