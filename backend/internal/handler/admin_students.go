package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminListStudents returns cursor-paginated students scoped to the caller's school.
// Supports optional grade and jenjang query filters.
func (h *Handler) AdminListStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	statusFilter := c.QueryParam("status")
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

	students, nextCursor, err := h.svc.ListStudents(c.Request().Context(), schoolID, statusFilter, q, limit, cursor, grade, jenjang, examID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":        students,
		"next_cursor": nextCursor,
	})
}

// AdminRegisterStudent creates a new student under the caller's school.
func (h *Handler) AdminRegisterStudent(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var req struct {
		Name           string  `json:"name"`
		Jenjang        string  `json:"jenjang"`
		Email          *string `json:"email"`
		DOB            *string `json:"dob"`
		Gender         *string `json:"gender"`
		Grade          *int    `json:"grade"`
		AlamatDomisili *string `json:"alamat_domisili"`
		TargetExam     *string `json:"target_exam"`
		ProvinsiID     *string `json:"provinsi_id"`
		KotaID         *string `json:"kota_id"`
		KecamatanID    *string `json:"kecamatan_id"`
		KodePos        *string `json:"kode_pos"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Name == "" || req.Jenjang == "" {
		return badRequest(c, "name and jenjang are required")
	}

	var dob *time.Time
	if req.DOB != nil && *req.DOB != "" {
		parsed, err := time.Parse("2006-01-02", *req.DOB)
		if err != nil {
			return badRequest(c, "invalid dob format, expected YYYY-MM-DD")
		}
		dob = &parsed
	}

	resp, err := h.svc.RegisterStudent(c.Request().Context(), schoolID, req.Name, req.Jenjang, req.Email, dob, req.Gender, req.Grade, req.AlamatDomisili, req.TargetExam, req.ProvinsiID, req.KotaID, req.KecamatanID, req.KodePos)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// AdminChangeStudentStatus toggles a student's active/deactivated status.
// super_admin may omit ?school_id=; the student id is the scope.
func (h *Handler) AdminChangeStudentStatus(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	id := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Status != "active" && req.Status != "deactivated" {
		return badRequest(c, "status must be active or deactivated")
	}

	if err := h.svc.ChangeStudentStatus(c.Request().Context(), schoolID, id, req.Status); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "status updated"})
}

// AdminGetStudentCredentials resets and reissues a student's credentials.
// super_admin may omit ?school_id=; the student id is the scope.
func (h *Handler) AdminGetStudentCredentials(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	id := c.Param("id")

	resp, err := h.svc.ReissueStudentCredentials(c.Request().Context(), schoolID, id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
