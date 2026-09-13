package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/service"
)

// safeInlineImageTypes are the only content types this proxy serves with their
// stored MIME type. The uploader controls the stored type, so anything else —
// text/html, image/svg+xml, application/* — is served as an opaque download
// instead, preventing a same-origin stored-XSS via an uploaded asset.
var safeInlineImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// safeServeContentType decides how a stored asset is served: a known raster
// image keeps its type and renders inline; anything else is forced to an opaque
// download so an uploaded text/html or image/svg+xml cannot execute on our origin.
func safeServeContentType(stored string) (served string, download bool) {
	if safeInlineImageTypes[stored] {
		return stored, false
	}
	return "application/octet-stream", true
}

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
	// Never let an uploaded asset's stored MIME type drive inline rendering:
	// non-image types become an opaque download, and nosniff stops the browser
	// second-guessing that. This closes the same-origin XSS regardless of who
	// uploaded or under which prefix.
	served, download := safeServeContentType(contentType)
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	if download {
		c.Response().Header().Set("Content-Disposition", "attachment")
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.Stream(http.StatusOK, served, obj)
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
		DOB                *string `json:"dob"`
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
	// Same wire shape the admin registration path already accepts, so a date
	// entered on the profile page and one entered by an admin parse identically.
	var dob *time.Time
	if req.DOB != nil && strings.TrimSpace(*req.DOB) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.DOB))
		if err != nil {
			return badRequest(c, "invalid dob format, expected YYYY-MM-DD")
		}
		dob = &parsed
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
		dob,
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
		// Anyone may upload their own avatar, but product images belong to
		// store management — gate them behind the same capability the admin
		// upload endpoints use, so a student can't seed the product namespace.
		if !service.HasCapability(claims.Role, "uploads:write") {
			return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "insufficient permissions"})
		}
		prefix = "product"
	case "payment_proof":
		// Payment proof is attached by whoever confirms the order manually —
		// gate it on orders:write, the same capability POST .../confirm needs.
		if !service.HasCapability(claims.Role, "orders:write") {
			return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "insufficient permissions"})
		}
		prefix = "payment_proof"
	case "refund_proof":
		// The transfer receipt evidencing a manual refund — same actor, same
		// capability as the refund action itself.
		if !service.HasCapability(claims.Role, "orders:write") {
			return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "insufficient permissions"})
		}
		prefix = "refund_proof"
	default:
		return badRequest(c, "kind must be avatar, product, payment_proof or refund_proof")
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
