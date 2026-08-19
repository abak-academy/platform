package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminListResults returns cursor-paginated school-scoped exam results.
func (h *Handler) AdminListResults(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	examIDStr := c.QueryParam("exam_id")
	if examIDStr == "" {
		return badRequest(c, "exam_id is required")
	}
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		return badRequest(c, "invalid exam_id")
	}

	q := c.QueryParam("q")
	cursor := c.QueryParam("cursor")

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}

	results, nextCursor, err := h.svc.ListSchoolResults(c.Request().Context(), examID, schoolID, q, cursor, limit)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":        results,
		"next_cursor": nextCursor,
	})
}

// AdminGetResultDetail returns the full detail of a single school-scoped session result.
func (h *Handler) AdminGetResultDetail(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		return badRequest(c, "invalid session_id")
	}

	detail, err := h.svc.GetSchoolResultDetail(c.Request().Context(), sessionID, schoolID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, detail)
}

// AdminExportResults returns a CSV file of school-scoped exam results.
func (h *Handler) AdminExportResults(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	examIDStr := c.QueryParam("exam_id")
	if examIDStr == "" {
		return badRequest(c, "exam_id is required")
	}
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		return badRequest(c, "invalid exam_id")
	}

	csvBytes, err := h.svc.ExportSchoolResultsCSV(c.Request().Context(), examID, schoolID)
	if err != nil {
		return mapServiceError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="results.csv"`)
	return c.Blob(http.StatusOK, "text/csv", csvBytes)
}
