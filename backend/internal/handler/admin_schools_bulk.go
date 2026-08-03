package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// AdminPresignSchoolBulkUpload issues a presigned PUT URL to the private bucket
// for a school-bulk CSV upload. Deliberately no resolveSchoolScope: schools are
// a global registry, and the route's RBACMiddleware("schools:write") already
// restricts this to super_admin.
func (h *Handler) AdminPresignSchoolBulkUpload(c echo.Context) error {
	filename := c.QueryParam("filename")
	if filename == "" {
		return badRequest(c, "filename is required")
	}

	resp, err := h.svc.GeneratePresignedSchoolBulkUploadURL(c.Request().Context(), filename, c.QueryParam("content_type"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminBulkImportSchools enqueues an async school_bulk job from an
// already-uploaded CSV.
func (h *Handler) AdminBulkImportSchools(c echo.Context) error {
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

	jobID, err := h.svc.EnqueueSchoolBulkJob(c.Request().Context(), claims.Sub, req.FileKey)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"job_id": jobID})
}
