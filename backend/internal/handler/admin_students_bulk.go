package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminPresignStudentBulkUpload issues a presigned PUT URL to the private
// bucket for a student-bulk CSV upload.
//
// Object keys are student-bulk/{folder-uuid}/{file-uuid}-filename. For
// admin_school the folder is the JWT school. For super_admin the folder is a
// random UUID (storage namespace only — not a real school); row school comes
// from the CSV name column, same as enqueue.
func (h *Handler) AdminPresignStudentBulkUpload(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// super_admin with no school: folder UUID is a storage namespace, not a
	// real school. Enqueue UUID-parses the second path segment for the prefix
	// check only. Row school still comes from the CSV name column.
	if schoolID == "" {
		schoolID = uuid.New().String()
	}

	filename := c.QueryParam("filename")
	contentType := c.QueryParam("content_type")
	if filename == "" {
		return badRequest(c, "filename is required")
	}

	resp, err := h.svc.GeneratePresignedPrivateUploadURL(c.Request().Context(), schoolID, filename, contentType)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminBulkImportStudents enqueues an async student_bulk job from an
// already-uploaded CSV. Branches on role instead of using resolveSchoolScope:
// admin_school is pinned to their own school; super_admin has no school bound
// and derives the storage key prefix from the fileKey itself.
func (h *Handler) AdminBulkImportStudents(c echo.Context) error {
	claims := ClaimsFromContext(c)

	var schoolID string

	switch claims.Role {
	case "super_admin":
		// super_admin: school is determined per-row from the CSV; skip
		// resolveSchoolScope entirely. schoolID will be extracted from
		// fileKey below for storage validation.
	case "admin_school":
		if claims.SchoolID == nil {
			c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "missing school scope"})
			return nil
		}
		schoolID = *claims.SchoolID
	default:
		c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "insufficient permissions"})
		return nil
	}

	var req struct {
		FileKey string `json:"file_key"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.FileKey == "" {
		return badRequest(c, "file_key is required")
	}

	// For super_admin, extract the folder UUID from the fileKey prefix (the
	// presign endpoint already encoded it). This is only needed for
	// storage-level validation in EnqueueStudentBulkJob; it is not a school
	// id. Row school comes from the CSV name column.
	if claims.Role == "super_admin" {
		parts := strings.SplitN(req.FileKey, "/", 3)
		if len(parts) < 3 || parts[0] != "student-bulk" {
			return badRequest(c, "invalid file_key")
		}
		schoolID = parts[1]
		if _, err := uuid.Parse(schoolID); err != nil {
			return badRequest(c, "invalid file_key")
		}
	}

	jobID, err := h.svc.EnqueueStudentBulkJob(c.Request().Context(), schoolID, claims.Sub, req.FileKey)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"job_id": jobID})
}

// AdminBulkReissueCredentials reissues credentials for a batch of students
// (or the whole school) and returns the per-row report as a CSV attachment.
func (h *Handler) AdminBulkReissueCredentials(c echo.Context) error {
	claims := ClaimsFromContext(c)
	schoolID, err := h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if schoolID == "" {
		return badRequest(c, "school_id is required")
	}

	var req struct {
		StudentIDs []string `json:"student_ids"`
		All        bool     `json:"all"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.All == (len(req.StudentIDs) > 0) {
		return badRequest(c, "specify either student_ids or all, not both/neither")
	}

	csvBytes, err := h.svc.ReissueStudentCredentialsBulk(c.Request().Context(), schoolID, req.StudentIDs, req.All)
	if err != nil {
		return mapServiceError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="credentials.csv"`)
	return c.Blob(http.StatusOK, "text/csv", csvBytes)
}
