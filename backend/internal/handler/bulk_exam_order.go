package handler

import (
	"encoding/json"
	"net/http"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// AdminListOrderableExams returns published exam-type products (FR-BULK-01).
func (h *Handler) AdminListOrderableExams(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	products, err := h.svc.ListOrderableExams(c.Request().Context(), claims.Role)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": products})
}

type bulkExamOrderBody struct {
	ExamID     string   `json:"exam_id"`
	StudentIDs []string `json:"student_ids,omitempty"`
	Grade      *int     `json:"grade,omitempty"`
	All        bool     `json:"all,omitempty"`
}

// bindBulkExamOrderScope decodes the body then resolves school. Explicit
// student_ids are themselves the scope, so super_admin may omit ?school_id=.
// grade/all still need a school. handled means the response was already written.
func (h *Handler) bindBulkExamOrderScope(c echo.Context, claims *infra.Claims) (schoolID string, req bulkExamOrderBody, handled bool, err error) {
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return "", req, true, badRequest(c, "invalid request body")
	}
	if req.ExamID == "" {
		return "", req, true, badRequest(c, "exam_id is required")
	}

	schoolID, err = h.resolveSchoolScope(c, claims)
	if scopeHandled(err) {
		return "", req, true, nil
	}
	if err != nil {
		return "", req, true, mapServiceError(c, err)
	}
	// grade/all pick a whole school; explicit student_ids are themselves the scope.
	if schoolID == "" && len(req.StudentIDs) == 0 {
		return "", req, true, badRequest(c, "school_id is required")
	}
	return schoolID, req, false, nil
}

func (req bulkExamOrderBody) selector() service.ParticipantSelector {
	return service.ParticipantSelector{
		StudentIDs: req.StudentIDs,
		Grade:      req.Grade,
		All:        req.All,
	}
}

// AdminPreviewBulkOrder returns a preview of the bulk order (FR-BULK-02).
func (h *Handler) AdminPreviewBulkOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	schoolID, req, handled, err := h.bindBulkExamOrderScope(c, claims)
	if handled {
		return err
	}

	preview, err := h.svc.PreviewBulkExamOrder(c.Request().Context(), schoolID, req.ExamID, req.selector())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, preview)
}

// AdminCreateBulkOrder creates a bulk exam order (FR-BULK-04, FR-BULK-05).
func (h *Handler) AdminCreateBulkOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	schoolID, req, handled, err := h.bindBulkExamOrderScope(c, claims)
	if handled {
		return err
	}

	order, err := h.svc.CreateBulkExamOrder(c.Request().Context(), claims.Sub, schoolID, req.ExamID, req.selector())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, order)
}

// AdminCheckoutBulkOrder checkouts a bulk exam order (FR-BULK-06).
func (h *Handler) AdminCheckoutBulkOrder(c echo.Context) error {
	claims := ClaimsFromContext(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing or invalid token"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return badRequest(c, "order id is required")
	}

	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	result, err := h.svc.Checkout(c.Request().Context(), claims.Sub, orderID, key)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}
