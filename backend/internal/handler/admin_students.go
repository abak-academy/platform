package handler

import (
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminListStudents returns cursor-paginated students scoped to the caller's school.
// Supports optional grade and jenjang query filters.
func (h *Handler) AdminListStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)
	isAffiliator := claims.Role == service.RoleAffiliator
	var schoolID string
	if !isAffiliator {
		var err error
		schoolID, err = h.resolveStudentSchoolScope(c, claims)
		if scopeHandled(err) {
			return nil
		}
		if err != nil {
			return err
		}
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

	var (
		students   []service.StudentResponse
		nextCursor string
		counts     repository.StudentAdminCounts
		err        error
	)
	if isAffiliator {
		students, nextCursor, counts, err = h.svc.ListAffiliatorStudents(c.Request().Context(), claims.Sub, strings.ToLower(c.QueryParam("school_id")), c.QueryParam("registered_by_me") == "true", statusFilter, q, limit, cursor, grade, jenjang)
	} else {
		students, nextCursor, counts, err = h.svc.ListStudents(c.Request().Context(), schoolID, statusFilter, q, limit, cursor, grade, jenjang, examID)
	}
	if err != nil {
		return mapServiceError(c, err)
	}

	// total/active/deactivated are filter-aware counts of the whole scoped
	// set (ignoring cursor/limit), mirroring GET /admin/schools — the stat
	// cards must not be derived from the single page loaded on the client.
	return c.JSON(http.StatusOK, map[string]any{
		"data":        students,
		"next_cursor": nextCursor,
		"total":       counts.Total,
		"active":      counts.Active,
		"deactivated": counts.Deactivated,
	})
}

// AdminRegisterStudent creates a new student under the caller's school.
func (h *Handler) AdminRegisterStudent(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveStudentSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var req struct {
		Name               string  `json:"name"`
		Jenjang            string  `json:"jenjang"`
		Email              *string `json:"email"`
		DOB                *string `json:"dob"`
		Gender             *string `json:"gender"`
		Grade              *int    `json:"grade"`
		AlamatDomisili     *string `json:"alamat_domisili"`
		TargetExam         *string `json:"target_exam"`
		ProvinsiID         *string `json:"provinsi_id"`
		KotaID             *string `json:"kota_id"`
		KecamatanID        *string `json:"kecamatan_id"`
		KodePos            *string `json:"kode_pos"`
		Password           *string `json:"password"`
		UnlistedSchoolName *string `json:"unlisted_school_name"`
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

	var unlistedSchoolName *string
	if req.UnlistedSchoolName != nil {
		if !service.CanManageAllStudents(claims.Role) {
			return mapServiceError(c, service.ErrForbidden)
		}
		name := strings.TrimSpace(*req.UnlistedSchoolName)
		if name == "" {
			return badRequest(c, "unlisted_school_name is required")
		}
		if schoolID != "" {
			return badRequest(c, "choose either school_id or unlisted_school_name")
		}
		unlistedSchoolName = &name
	}

	var resp *service.StudentRegistrationResponse
	if req.Password != nil && *req.Password != "" {
		if unlistedSchoolName != nil {
			resp, err = h.svc.RegisterStudentUnlistedWithPassword(c.Request().Context(), claims.Sub, claims.Role, *unlistedSchoolName, req.Name, req.Jenjang, req.Email, dob, req.Gender, req.Grade, req.AlamatDomisili, req.TargetExam, req.ProvinsiID, req.KotaID, req.KecamatanID, req.KodePos, *req.Password)
		} else {
			resp, err = h.svc.RegisterStudentWithPassword(c.Request().Context(), claims.Sub, claims.Role, schoolID, req.Name, req.Jenjang, req.Email, dob, req.Gender, req.Grade, req.AlamatDomisili, req.TargetExam, req.ProvinsiID, req.KotaID, req.KecamatanID, req.KodePos, *req.Password)
		}
	} else {
		if unlistedSchoolName != nil {
			resp, err = h.svc.RegisterStudentUnlisted(c.Request().Context(), claims.Sub, claims.Role, *unlistedSchoolName, req.Name, req.Jenjang, req.Email, dob, req.Gender, req.Grade, req.AlamatDomisili, req.TargetExam, req.ProvinsiID, req.KotaID, req.KecamatanID, req.KodePos)
		} else {
			resp, err = h.svc.RegisterStudent(c.Request().Context(), claims.Sub, schoolID, req.Name, req.Jenjang, req.Email, dob, req.Gender, req.Grade, req.AlamatDomisili, req.TargetExam, req.ProvinsiID, req.KotaID, req.KecamatanID, req.KodePos)
		}
	}
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) AdminSetStudentPassword(c echo.Context) error {
	claims := ClaimsFromContext(c)
	var req struct {
		NewPassword *string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.NewPassword == nil || *req.NewPassword == "" {
		return badRequest(c, "new_password is required")
	}
	if err := h.svc.SetStudentPassword(c.Request().Context(), claims.Sub, c.Param("id"), *req.NewPassword); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password updated"})
}

// AdminChangeStudentStatus toggles a student's active/deactivated status.
// super_admin and admin_exam may omit ?school_id=; the student id is the scope.
func (h *Handler) AdminChangeStudentStatus(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveStudentSchoolScope(c, claims)
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

	if err := h.svc.ChangeStudentStatus(c.Request().Context(), claims.Sub, schoolID, id, req.Status); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "status updated"})
}

// AdminGetStudentCredentials resets and reissues a student's credentials.
// super_admin and admin_exam may omit ?school_id=; the student id is the scope.
func (h *Handler) AdminGetStudentCredentials(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveStudentSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}

	id := c.Param("id")

	resp, err := h.svc.ReissueStudentCredentials(c.Request().Context(), claims.Sub, schoolID, id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
