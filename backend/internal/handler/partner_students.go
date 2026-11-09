package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// PartnerListStudents backs GET /partners/me/students: the students of the
// Mitra's allowed schools plus the ones it registered.
func (h *Handler) PartnerListStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)
	states := partnerStatesFromContext(c)
	status := c.QueryParam("status")
	if status != "" && status != "active" && status != "deactivated" {
		return badRequest(c, "status must be active or deactivated")
	}
	registeredByMe := false
	if raw := c.QueryParam("registered_by_me"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return badRequest(c, "registered_by_me must be true or false")
		}
		registeredByMe = parsed
	}
	page, err := h.svc.ListPartnerStudents(c.Request().Context(), claims.Sub, states.PartnerSchools, service.PartnerStudentListFilter{
		SchoolID:       strings.ToLower(c.QueryParam("school_id")),
		Status:         status,
		Q:              c.QueryParam("q"),
		RegisteredByMe: registeredByMe,
		Cursor:         c.QueryParam("cursor"),
		Limit:          parseLimit(c.QueryParam("limit"), 20, 100),
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

type partnerRegisterStudentRequest struct {
	SchoolID       string  `json:"school_id"`
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
	Password       *string `json:"password"`
}

// PartnerRegisterStudent backs POST /partners/me/students. A Mitra never
// chooses the password: the temporary one comes back once in the response.
func (h *Handler) PartnerRegisterStudent(c echo.Context) error {
	claims := ClaimsFromContext(c)
	var req partnerRegisterStudentRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Password != nil {
		return badRequest(c, "password cannot be set; a temporary password is generated")
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
	resp, err := h.svc.PartnerRegisterStudent(c.Request().Context(), claims.Sub, service.PartnerRegisterStudentInput{
		SchoolID:       strings.ToLower(req.SchoolID),
		Name:           req.Name,
		Jenjang:        req.Jenjang,
		Email:          req.Email,
		DOB:            dob,
		Gender:         req.Gender,
		Grade:          req.Grade,
		AlamatDomisili: req.AlamatDomisili,
		TargetExam:     req.TargetExam,
		ProvinsiID:     req.ProvinsiID,
		KotaID:         req.KotaID,
		KecamatanID:    req.KecamatanID,
		KodePos:        req.KodePos,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// PartnerReissueStudentCredentials backs POST /partners/me/students/:id/credentials.
func (h *Handler) PartnerReissueStudentCredentials(c echo.Context) error {
	claims := ClaimsFromContext(c)
	resp, err := h.svc.PartnerReissueStudentCredentials(c.Request().Context(), claims.Sub, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// PartnerPresignStudentBulkUpload backs POST /partners/me/students/bulk/presign.
// The upload folder is the Mitra's own id, so a Mitra can only start a job from
// a file it uploaded itself.
func (h *Handler) PartnerPresignStudentBulkUpload(c echo.Context) error {
	claims := ClaimsFromContext(c)
	filename := c.QueryParam("filename")
	if filename == "" {
		return badRequest(c, "filename is required")
	}
	resp, err := h.svc.GeneratePresignedPrivateUploadURL(c.Request().Context(), claims.Sub, filename, c.QueryParam("content_type"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// PartnerBulkImportStudents backs POST /partners/me/students/bulk: it queues the
// import of an uploaded CSV as an async job.
func (h *Handler) PartnerBulkImportStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)
	var req struct {
		FileKey string `json:"file_key"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.FileKey == "" {
		return badRequest(c, "file_key is required")
	}
	jobID, err := h.svc.EnqueueStudentBulkJob(c.Request().Context(), claims.Sub, claims.Sub, claims.Role, req.FileKey)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"job_id": jobID})
}

// PartnerGetJob backs GET /partners/me/jobs/:id: only a job the Mitra created.
func (h *Handler) PartnerGetJob(c echo.Context) error {
	claims := ClaimsFromContext(c)
	resp, err := h.svc.PartnerJobStatus(c.Request().Context(), claims.Sub, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// PartnerListProducts backs GET /partners/me/products: what the admin opened to this Mitra.
func (h *Handler) PartnerListProducts(c echo.Context) error {
	claims := ClaimsFromContext(c)
	products, err := h.svc.ListPartnerProducts(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": products})
}

// PartnerListVouchers backs GET /partners/me/vouchers: the Mitra vouchers this
// Mitra can use now. The promos Affiliators share with students are not here.
func (h *Handler) PartnerListVouchers(c echo.Context) error {
	claims := ClaimsFromContext(c)
	vouchers, err := h.svc.ListPartnerVouchers(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": vouchers})
}
