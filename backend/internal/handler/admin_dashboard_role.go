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
