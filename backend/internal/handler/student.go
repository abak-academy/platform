package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ServeFile is an unauthenticated read-proxy for avatars, product images and
// question assets stored in the private object bucket. The service enforces
// an allowlist of prefixes, so certificates and private PII in the same
// bucket are never reachable here. The stored photo_url is
// <api-base>/files/<key>, which stays stable and browser-cacheable — unlike a
// presigned URL, which would expire.
func (h *Handler) ServeFile(c echo.Context) error {
	key := c.Param("*")
	obj, contentType, err := h.svc.OpenAvatar(c.Request().Context(), key)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	defer obj.Close()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.Stream(http.StatusOK, contentType, obj)
}

func (h *Handler) StudentDashboard(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	dashboard, err := h.svc.GetDashboard(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, dashboard)
}

func (h *Handler) ListSchools(c echo.Context) error {
	schools, err := h.svc.ListSchools(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, schools)
}

func (h *Handler) StudentProfile(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	user, err := h.svc.Me(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) StudentUpdateProfile(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req struct {
		Name               *string `json:"name"`
		Email              *string `json:"email"`
		Username           *string `json:"username"`
		Phone              *string `json:"phone"`
		Address            *string `json:"address"`
		TargetExam         *string `json:"target_exam"`
		Grade              *int    `json:"grade"`
		SchoolID           *string `json:"school_id"`
		UnlistedSchoolName *string `json:"unlisted_school_name"`
		Jenjang            *string `json:"jenjang"`
		ProvinsiID         *string `json:"provinsi_id"`
		KotaID             *string `json:"kota_id"`
		KecamatanID        *string `json:"kecamatan_id"`
		KodePos            *string `json:"kode_pos"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	user, err := h.svc.UpdateProfile(
		c.Request().Context(),
		claims.Sub,
		req.Name,
		req.Email,
		req.Username,
		req.Phone,
		req.Address,
		req.TargetExam,
		req.Grade,
		req.SchoolID,
		req.UnlistedSchoolName,
		req.Jenjang,
		req.ProvinsiID,
		req.KotaID,
		req.KecamatanID,
		req.KodePos,
	)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) GeneratePresignUploadURL(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	filename := c.QueryParam("filename")
	contentType := c.QueryParam("content_type")
	if filename == "" {
		return badRequest(c, "filename is required")
	}
	var prefix string
	switch c.QueryParam("kind") {
	case "", "avatar":
		prefix = "avatars"
	case "product":
		prefix = "product"
	default:
		return badRequest(c, "kind must be avatar or product")
	}
	resp, err := h.svc.GeneratePresignedUploadURL(c.Request().Context(), claims.Sub, prefix, filename, contentType)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdatePhoto(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req struct {
		PhotoURL string `json:"photo_url"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.PhotoURL == "" {
		return badRequest(c, "photo_url is required")
	}
	user, err := h.svc.UpdatePhoto(c.Request().Context(), claims.Sub, req.PhotoURL)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, user)
}
