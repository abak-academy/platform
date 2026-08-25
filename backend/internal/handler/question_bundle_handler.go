package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

// CreateQuestionBundleRequest is the POST body for creating a question bundle.
type CreateQuestionBundleRequest struct {
	IncludeAnswerKey bool `json:"include_answer_key"`
}

// QuestionBundleResponse is the API response for question bundle operations.
type QuestionBundleResponse struct {
	ID          string  `json:"id"`
	ScopeType   string  `json:"scope_type"`
	ScopeID     string  `json:"scope_id"`
	Variant     string  `json:"variant"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	GeneratedAt *string `json:"generated_at,omitempty"`
	Error       *string `json:"error,omitempty"`
}

// AdminCreateTestQuestionBundle creates a question bundle for a test. POST /admin/tests/:id/question-bundle
func (h *Handler) AdminCreateTestQuestionBundle(c echo.Context) error {
	testID := c.Param("id")
	if testID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "test_id required"})
	}

	tid, err := uuid.Parse(testID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid test_id"})
	}

	var req CreateQuestionBundleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing auth"})
	}

	actor, err := uuid.Parse(claims.Sub)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid actor"})
	}

	bundle, err := h.svc.EnqueueQuestionBundle(c.Request().Context(), actor, claims.Role, nil, &tid, req.IncludeAnswerKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "test not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		if errors.Is(err, service.ErrValidation) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	return c.JSON(http.StatusAccepted, bundleToResponse(bundle))
}

// AdminCreateExamQuestionBundle creates a question bundle for an exam. POST /admin/exams/:id/question-bundle
func (h *Handler) AdminCreateExamQuestionBundle(c echo.Context) error {
	examID := c.Param("id")
	if examID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "exam_id required"})
	}

	eid, err := uuid.Parse(examID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid exam_id"})
	}

	var req CreateQuestionBundleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing auth"})
	}

	actor, err := uuid.Parse(claims.Sub)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid actor"})
	}

	bundle, err := h.svc.EnqueueQuestionBundle(c.Request().Context(), actor, claims.Role, &eid, nil, req.IncludeAnswerKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "exam not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		if errors.Is(err, service.ErrValidation) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	return c.JSON(http.StatusAccepted, bundleToResponse(bundle))
}

// AdminGetQuestionBundleStatus gets bundle status. GET /admin/question-bundles/:id
func (h *Handler) AdminGetQuestionBundleStatus(c echo.Context) error {
	bundleID := c.Param("id")
	if bundleID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bundle_id required"})
	}

	bid, err := uuid.Parse(bundleID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid bundle_id"})
	}

	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing auth"})
	}

	actor, err := uuid.Parse(claims.Sub)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid actor"})
	}

	bundle, err := h.svc.GetQuestionBundle(c.Request().Context(), bid, actor, claims.Role)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "bundle not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	return c.JSON(http.StatusOK, bundleToResponse(bundle))
}

// AdminDownloadQuestionBundle gets a download URL. GET /admin/question-bundles/:id/download
func (h *Handler) AdminDownloadQuestionBundle(c echo.Context) error {
	bundleID := c.Param("id")
	if bundleID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bundle_id required"})
	}

	bid, err := uuid.Parse(bundleID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid bundle_id"})
	}

	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing auth"})
	}

	actor, err := uuid.Parse(claims.Sub)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid actor"})
	}

	url, err := h.svc.GetQuestionBundleDownloadURL(c.Request().Context(), bid, actor, claims.Role)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "bundle not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, service.ErrValidation) {
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"url": url,
	})
}

func bundleToResponse(b *model.QuestionBundle) QuestionBundleResponse {
	resp := QuestionBundleResponse{
		ID:        b.ID.String(),
		Variant:   b.Variant,
		Status:    b.Status,
		CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: b.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if b.GeneratedAt != nil {
		t := b.GeneratedAt.UTC().Format(time.RFC3339)
		resp.GeneratedAt = &t
	}

	if b.Error != nil {
		resp.Error = b.Error
	}

	if b.ExamID != nil {
		resp.ScopeType = "exam"
		resp.ScopeID = b.ExamID.String()
	} else if b.TestID != nil {
		resp.ScopeType = "test"
		resp.ScopeID = b.TestID.String()
	}

	return resp
}
