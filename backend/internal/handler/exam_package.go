package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *Handler) AdminListExams(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	filter := repository.ExamFilter{
		Cursor: c.QueryParam("cursor"),
		Limit:  limit,
	}

	exams, nextCursor, err := h.svc.ListExams(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        exams,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) AdminCreateExam(c echo.Context) error {
	var req model.Exam
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	exam, err := h.svc.CreateExam(c.Request().Context(), req)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, exam)
}

func (h *Handler) AdminGetExam(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	detail, err := h.svc.GetExam(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

// AdminListExamRegistrations returns the read-only admin participant roster
// for an exam (FR-32): registrations joined with student name/username and
// each row's FR-24 display participant number.
func (h *Handler) AdminListExamRegistrations(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	// admin_school sees only its own school's participants; super_admin and
	// admin_exam (global exam managers) see the full roster. Without this the
	// endpoint leaks other schools' student PII (name/username/id) for any
	// known exam id.
	var schoolFilter *string
	if claims.Role == "admin_school" {
		if claims.SchoolID == nil {
			return c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "missing school scope"})
		}
		schoolFilter = claims.SchoolID
	}
	rows, err := h.svc.AdminGetExamRoster(c.Request().Context(), id, schoolFilter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": rows})
}

// examPatchRequest is the PATCH body for AdminUpdateExam. Nullable[T] fields
// distinguish "absent — preserve" from "present and null — clear" from "present
// with a value" (a plain *T cannot: encoding/json leaves it nil for both absent
// and explicit null). Lifecycle/system-managed fields (status, bundle_url,
// bundle_generated_at) are deliberately not accepted: status flips via
// POST /:id/publish, bundle fields only via the (future) bundle generation flow.
// Mode is a plain string: absent (empty) preserves the stored value (FR-18).
type examPatchRequest struct {
	Title                string              `json:"title"`
	ScheduledAt          Nullable[time.Time] `json:"scheduled_at"`
	TimerMode            string              `json:"timer_mode"`
	DurationMinutes      Nullable[int]       `json:"duration_minutes"`
	ResultConfig         string              `json:"result_config"`
	ResultReleaseAt      Nullable[time.Time] `json:"result_release_at"`
	CheckInWindowMinutes Nullable[int]       `json:"check_in_window_minutes"`
	GraceWindowMinutes   Nullable[int]       `json:"grace_window_minutes"`
	MaxAttempts          Nullable[int]       `json:"max_attempts"`
	CertificateTemplate  string              `json:"certificate_template"`
	IsFree               *bool               `json:"is_free"`
	RequiresCheckin      *bool               `json:"requires_checkin"`
	AllowLeaderboard     *bool               `json:"allow_leaderboard"`
	CDNBundle            *bool               `json:"cdn_bundle"`
	Randomize            *bool               `json:"randomize"`
	Mode                 string              `json:"mode"`
}

// applyNullable overlays a Nullable[T] PATCH field onto a *T model field: absent
// (Set false) preserves the existing value; present-and-null (Set true, Valid
// false) clears it to nil; present-with-value sets it.
func applyNullable[T any](n Nullable[T], dst **T) {
	if !n.Set {
		return
	}
	if !n.Valid {
		*dst = nil
		return
	}
	v := n.Value
	*dst = &v
}

func (h *Handler) AdminUpdateExam(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	existing, err := h.svc.GetExam(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}

	var req examPatchRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	overlay := existing.Exam
	if req.Title != "" {
		overlay.Title = req.Title
	}
	applyNullable(req.ScheduledAt, &overlay.ScheduledAt)
	if req.TimerMode != "" {
		overlay.TimerMode = req.TimerMode
	}
	applyNullable(req.DurationMinutes, &overlay.DurationMinutes)
	if req.ResultConfig != "" {
		overlay.ResultConfig = req.ResultConfig
	}
	applyNullable(req.ResultReleaseAt, &overlay.ResultReleaseAt)
	applyNullable(req.CheckInWindowMinutes, &overlay.CheckInWindowMinutes)
	applyNullable(req.GraceWindowMinutes, &overlay.GraceWindowMinutes)
	applyNullable(req.MaxAttempts, &overlay.MaxAttempts)
	if req.CertificateTemplate != "" {
		raw, err := service.SetCertificateTemplate(overlay.CertificateDesign, req.CertificateTemplate)
		if err != nil {
			return badRequest(c, "invalid certificate design")
		}
		overlay.CertificateDesign = raw
	}
	if req.IsFree != nil {
		overlay.IsFree = *req.IsFree
	}
	if req.RequiresCheckin != nil {
		overlay.RequiresCheckin = *req.RequiresCheckin
	}
	if req.AllowLeaderboard != nil {
		overlay.AllowLeaderboard = *req.AllowLeaderboard
	}
	if req.CDNBundle != nil {
		overlay.CDNBundle = *req.CDNBundle
	}
	if req.Randomize != nil {
		overlay.Randomize = *req.Randomize
	}
	if req.Mode != "" {
		overlay.Mode = req.Mode
	}

	out, err := h.svc.UpdateExam(c.Request().Context(), id, overlay)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

type examTestReplaceItem struct {
	TestID    uuid.UUID `json:"test_id"`
	SortOrder *int      `json:"sort_order,omitempty"`
}

// AdminReplaceExamTests accepts either `[{test_id,sort_order},...]` or `[test_id,...]`
// and replaces the exam's attached-test list atomically. Service assigns sort_order
// from list position; sort_order in the body is accepted for forward-compat but ignored.
func (h *Handler) AdminReplaceExamTests(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return badRequest(c, "invalid request body")
	}

	var testIDs []uuid.UUID
	if err := json.Unmarshal(raw, &testIDs); err != nil {
		var rich []examTestReplaceItem
		if err2 := json.Unmarshal(raw, &rich); err2 != nil {
			return badRequest(c, "invalid request body")
		}
		for _, it := range rich {
			if it.TestID == uuid.Nil {
				return badRequest(c, "test_id required")
			}
			testIDs = append(testIDs, it.TestID)
		}
	}

	if err := h.svc.ReplaceExamTests(c.Request().Context(), id, testIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
