package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AdminUploadImage handles POST /admin/uploads/image. Validates that the
// content_type query parameter is prefixed with "image/", then generates a
// presigned PUT URL for the object.
func (h *Handler) AdminUploadImage(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "missing auth"})
	}

	filename := c.QueryParam("filename")
	contentType := c.QueryParam("content_type")
	if filename == "" {
		return badRequest(c, "filename is required")
	}

	if !strings.HasPrefix(contentType, "image/") {
		return badRequest(c, "content_type must be prefixed with image/")
	}

	resp, err := h.svc.GeneratePresignedUploadURL(c.Request().Context(), claims.Sub, "question", filename, contentType)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminUploadAudio handles POST /admin/uploads/audio. Validates that the
// content_type query parameter is prefixed with "audio/", then generates a
// presigned PUT URL for the object.
func (h *Handler) AdminUploadAudio(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "missing auth"})
	}

	filename := c.QueryParam("filename")
	contentType := c.QueryParam("content_type")
	if filename == "" {
		return badRequest(c, "filename is required")
	}

	if !strings.HasPrefix(contentType, "audio/") {
		return badRequest(c, "content_type must be prefixed with audio/")
	}

	resp, err := h.svc.GeneratePresignedUploadURL(c.Request().Context(), claims.Sub, "question", filename, contentType)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
