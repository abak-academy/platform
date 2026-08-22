package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminSearchGrantStudents handles GET /admin/exam-grants/students/search.
// Returns cross-school student results for the super_admin grant participant
// picker (FR-SEARCH-01/02/03). Deliberately does NOT call resolveSchoolScope.
func (h *Handler) AdminSearchGrantStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	q := c.QueryParam("q")

	var schoolID *string
	var noSchool bool
	if sid := c.QueryParam("school_id"); sid != "" {
		switch {
		case sid == "none":
			noSchool = true
		default:
			if _, err := uuid.Parse(sid); err != nil {
				return c.JSON(http.StatusBadRequest, APIError{Code: "invalid_request", Message: "school_id must be a valid UUID"})
			}
			schoolID = &sid
		}
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
	cursor := c.QueryParam("cursor")

	var grade *int
	if g := c.QueryParam("grade"); g != "" {
		if n, err := strconv.Atoi(g); err == nil {
			grade = &n
		}
	}
	jenjang := c.QueryParam("jenjang")
	examID := c.QueryParam("exam_id")
	if examID != "" {
		if _, err := uuid.Parse(examID); err != nil {
			return c.JSON(http.StatusBadRequest, APIError{Code: "invalid_request", Message: "exam_id must be a valid UUID"})
		}
	}

	students, nextCursor, err := h.svc.SearchStudentsAcrossSchools(c.Request().Context(), q, schoolID, noSchool, grade, jenjang, limit, cursor, examID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":        students,
		"next_cursor": nextCursor,
	})
}

// AdminGrantExamAccess handles POST /admin/exam-grants.
// Creates direct exam registrations for the given students, bypassing the
// order pipeline (FR-GRANT-01/02/07). No school_id in the request —
// super_admin's grant is not school-scoped.
func (h *Handler) AdminGrantExamAccess(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	var req struct {
		ExamID     string      `json:"exam_id"`
		StudentIDs []uuid.UUID `json:"student_ids"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.ExamID == "" {
		return badRequest(c, "exam_id is required")
	}
	if len(req.StudentIDs) == 0 {
		return badRequest(c, "student_ids is required")
	}

	result, err := h.svc.GrantExamAccess(c.Request().Context(), claims.Sub, req.ExamID, req.StudentIDs)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusCreated, result)
}
