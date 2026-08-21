package handler

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

// AdminPresignExamGrantBulkUpload handles POST /admin/exam-grants/bulk/presign.
// Issues a presigned PUT URL to the private bucket for a super_admin exam-grant
// CSV upload (RBAC already restricts this route to exam-grants:write, i.e.
// super_admin only — see routes.go).
func (h *Handler) AdminPresignExamGrantBulkUpload(c echo.Context) error {
	examID := c.QueryParam("exam_id")
	if examID == "" {
		return badRequest(c, "exam_id is required")
	}
	filename := c.QueryParam("filename")
	if filename == "" {
		return badRequest(c, "filename is required")
	}
	contentType := c.QueryParam("content_type")

	resp, err := h.svc.GeneratePresignedExamGrantBulkUploadURL(c.Request().Context(), examID, filename, contentType)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminEnqueueExamGrantBulk handles POST /admin/exam-grants/bulk. Enqueues an
// async exam_grant_bulk job from an already-uploaded CSV.
func (h *Handler) AdminEnqueueExamGrantBulk(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	var req struct {
		ExamID  string `json:"exam_id"`
		FileKey string `json:"file_key"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.ExamID == "" {
		return badRequest(c, "exam_id is required")
	}
	if req.FileKey == "" {
		return badRequest(c, "file_key is required")
	}

	jobID, err := h.svc.EnqueueExamGrantBulkJob(c.Request().Context(), req.ExamID, claims.Sub, req.FileKey)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"job_id": jobID})
}
