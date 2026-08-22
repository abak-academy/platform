package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminGetExamAssessment returns the participant-centric assessment workspace
// for an exam (Issue 124). Super-admin only — gated by the RBACMiddleware
// "assessment:read" capability on the route, not by an in-handler role check.
func (h *Handler) AdminGetExamAssessment(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}

	var schoolID *uuid.UUID
	if raw := c.QueryParam("school_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return badRequest(c, "invalid school_id")
		}
		schoolID = &id
	}

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}

	resp, err := h.svc.AdminGetExamAssessment(c.Request().Context(), examID, c.QueryParam("q"), schoolID, c.QueryParam("cursor"), limit)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, resp)
}

// AdminGetAssessmentAttempts returns the attempt history for a single
// registration within the assessment workspace drawer (Issue 124).
func (h *Handler) AdminGetAssessmentAttempts(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}
	registrationID, err := uuid.Parse(c.Param("registration_id"))
	if err != nil {
		return badRequest(c, "invalid registration id")
	}

	attempts, err := h.svc.AdminGetAssessmentAttempts(c.Request().Context(), examID, registrationID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{"data": attempts})
}
