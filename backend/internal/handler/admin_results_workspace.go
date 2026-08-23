package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminGetExamResultsWorkspace returns the participant-centric results workspace
// for an exam (Issue 124). Super-admin only — gated by the RBACMiddleware
// "results-workspace:read" capability on the route, not by an in-handler role check.
func (h *Handler) AdminGetExamResultsWorkspace(c echo.Context) error {
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

	resp, err := h.svc.AdminGetExamResultsWorkspace(c.Request().Context(), examID, c.QueryParam("q"), schoolID, c.QueryParam("cursor"), limit)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, resp)
}

// AdminGetResultsWorkspaceAttempts returns the attempt history for a single
// registration within the results workspace drawer (Issue 124).
func (h *Handler) AdminGetResultsWorkspaceAttempts(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}
	registrationID, err := uuid.Parse(c.Param("registration_id"))
	if err != nil {
		return badRequest(c, "invalid registration id")
	}

	attempts, err := h.svc.AdminGetResultsWorkspaceAttempts(c.Request().Context(), examID, registrationID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{"data": attempts})
}

// AdminGetResultsWorkspaceResultDetail returns super-admin-only operational answer
// detail for a submitted, fully graded results workspace attempt.
func (h *Handler) AdminGetResultsWorkspaceResultDetail(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		return badRequest(c, "invalid session id")
	}

	detail, err := h.svc.AdminGetResultsWorkspaceResultDetail(c.Request().Context(), examID, sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, detail)
}
