package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail any
	}{
		{name: "email taken", err: service.ErrEmailTaken, wantStatus: 409, wantCode: "email_taken"},
		{name: "invalid credentials", err: service.ErrInvalidCredentials, wantStatus: 401, wantCode: "invalid_credentials"},
		{name: "otp rate limit", err: service.ErrOTPRateLimit, wantStatus: 429, wantCode: "rate_limited"},
		{name: "otp expired", err: service.ErrOTPExpired, wantStatus: 422, wantCode: "otp_invalid"},
		{name: "invalid otp", err: service.ErrInvalidOTP, wantStatus: 422, wantCode: "otp_invalid"},
		{name: "invalid pending token", err: service.ErrInvalidPendingToken, wantStatus: 422, wantCode: "otp_invalid"},
		{name: "invalid refresh token", err: service.ErrInvalidRefreshToken, wantStatus: 401, wantCode: "invalid_refresh_token"},
		{name: "user not found", err: service.ErrUserNotFound, wantStatus: 404, wantCode: "user_not_found"},
		{name: "invalid reset token", err: service.ErrInvalidResetToken, wantStatus: 422, wantCode: "reset_invalid"},
		{name: "account deactivated", err: service.ErrAccountDeactivated, wantStatus: 403, wantCode: "account_deactivated"},
		{name: "invalid google token", err: service.ErrInvalidToken, wantStatus: 401, wantCode: "invalid_google_token"},
		{name: "invalid uuid", err: service.ErrInvalidUUID, wantStatus: 400, wantCode: "invalid_request"},
		{name: "weak password", err: service.ErrWeakPassword, wantStatus: 400, wantCode: "invalid_request", wantDetail: "password must be at least 8 characters"},
		{name: "product not found", err: service.ErrProductNotFound, wantStatus: 404, wantCode: "product_not_found"},
		{name: "course not found", err: service.ErrCourseNotFound, wantStatus: 404, wantCode: "course_not_found"},
		{name: "forbidden", err: service.ErrForbidden, wantStatus: 403, wantCode: "forbidden"},
		{name: "out of stock", err: service.ErrOutOfStock, wantStatus: 409, wantCode: "out_of_stock"},
		{name: "insufficient stock", err: service.ErrInsufficientStock, wantStatus: 409, wantCode: "insufficient_stock"},
		{name: "order not editable", err: service.ErrOrderNotEditable, wantStatus: 409, wantCode: "order_not_editable"},
		{name: "order not found", err: service.ErrOrderNotFound, wantStatus: 404, wantCode: "order_not_found"},
		{name: "invalid promo", err: service.ErrInvalidPromo, wantStatus: 422, wantCode: "invalid_promo"},
		{name: "promo min order", err: service.ErrPromoMinOrder, wantStatus: 422, wantCode: "promo_min_order"},
		{name: "invalid signature", err: service.ErrInvalidSignature, wantStatus: 401, wantCode: "invalid_signature"},
		{name: "course link required", err: service.ErrCourseLinkRequired, wantStatus: 422, wantCode: "course_required"},
		{name: "no course access", err: service.ErrNoCourseAccess, wantStatus: 422, wantCode: "no_course_access"},
		{name: "cannot deactivate self", err: service.ErrCannotDeactivateSelf, wantStatus: 403, wantCode: "cannot_deactivate_self"},
		{name: "invalid admin role", err: service.ErrInvalidAdminRole, wantStatus: 400, wantCode: "invalid_request"},
		{name: "invalid role filter", err: service.ErrInvalidRoleFilter, wantStatus: 400, wantCode: "invalid_request"},
		{name: "invalid status filter", err: service.ErrInvalidStatusFilter, wantStatus: 400, wantCode: "invalid_request"},
		{name: "unknown config key", err: service.ErrUnknownConfigKey, wantStatus: 400, wantCode: "invalid_request"},
		{name: "config encryption failed", err: service.ErrConfigEncryption, wantStatus: 500, wantCode: "internal_error"},
		{name: "missing field", err: service.ErrMissingField, wantStatus: 400, wantCode: "invalid_request"},
		{name: "account no email", err: service.ErrAccountNoEmail, wantStatus: 422, wantCode: "reset_invalid"},
		{name: "invalid date format", err: fmt.Errorf("%w: %s", service.ErrInvalidDateFormat, "bad-date"), wantStatus: 400, wantCode: "invalid_request"},
		{name: "announcement not found", err: service.ErrAnnouncementNotFound, wantStatus: 404, wantCode: "announcement_not_found"},
		{name: "announcement immutable", err: service.ErrAnnouncementImmutable, wantStatus: 409, wantCode: "announcement_immutable"},
		{name: "upload not found", err: service.ErrUploadNotFound, wantStatus: 404, wantCode: "upload_not_found"},
		{name: "invalid csv", err: service.ErrInvalidCSV, wantStatus: 422, wantCode: "invalid_csv"},
		{name: "missing csv header", err: service.ErrMissingCSVHeader, wantStatus: 422, wantCode: "invalid_csv_headers"},
		{name: "row limit exceeded", err: service.ErrRowLimitExceeded, wantStatus: 422, wantCode: "row_limit_exceeded"},
		{name: "invalid jenjang", err: service.ErrInvalidJenjang, wantStatus: 422, wantCode: "invalid_jenjang"},
		{name: "incomplete address", err: service.ErrIncompleteAddress, wantStatus: 422, wantCode: "incomplete_address"},
		{name: "invalid provinsi", err: service.ErrInvalidProvinsi, wantStatus: 422, wantCode: "invalid_provinsi"},
		{name: "invalid kota", err: service.ErrInvalidKota, wantStatus: 422, wantCode: "invalid_kota"},
		{name: "invalid kecamatan", err: service.ErrInvalidKecamatan, wantStatus: 422, wantCode: "invalid_kecamatan"},
		{name: "username generation exhausted", err: service.ErrUsernameGenerationExhausted, wantStatus: 503, wantCode: "username_generation_exhausted"},
		{name: "job not found", err: service.ErrJobNotFound, wantStatus: 404, wantCode: "job_not_found"},
		{name: "already attempted", err: service.ErrAlreadyAttempted, wantStatus: 409, wantCode: "already_attempted"},
		{name: "exam not started", err: service.ErrExamNotStarted, wantStatus: 409, wantCode: "exam_not_started"},
		{name: "device mismatch", err: service.ErrDeviceMismatch, wantStatus: 403, wantCode: "device_mismatch"},
		{name: "checkin window closed", err: service.ErrCheckinWindowClosed, wantStatus: 409, wantCode: "checkin_window_closed"},
		{name: "not checked in", err: service.ErrNotCheckedIn, wantStatus: 409, wantCode: "not_checked_in"},
		{name: "already submitted", err: service.ErrAlreadySubmitted, wantStatus: 409, wantCode: "already_submitted"},
		{name: "session not found", err: service.ErrSessionNotFound, wantStatus: 404, wantCode: "session_not_found"},
		{name: "invalid violation type", err: service.ErrInvalidViolationType, wantStatus: 400, wantCode: "invalid_violation_type"},
		{name: "question in live exam", err: service.ErrQuestionInLiveExam, wantStatus: 409, wantCode: "question_in_published_exam"},
		{name: "question format locked", err: service.ErrQuestionFormatLocked, wantStatus: 409, wantCode: "question_format_locked"},
		{name: "unknown error falls to 500", err: errors.New("something unexpected"), wantStatus: 500, wantCode: "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			_ = mapServiceError(c, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status: want %d, got %d", tt.wantStatus, rec.Code)
			}

			var apiErr APIError
			if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiErr.Code != tt.wantCode {
				t.Errorf("code: want %q, got %q", tt.wantCode, apiErr.Code)
			}
			if tt.wantDetail != nil {
				if apiErr.Details != tt.wantDetail {
					t.Errorf("details: want %v, got %v", tt.wantDetail, apiErr.Details)
				}
			}
		})
	}
}

// TestMapServiceError_BiodataIncomplete_BodyCarriesMissingFields checks the
// raw response bytes (not a decoded APIError struct) so the assertion pins the
// actual wire shape a frontend has to parse: a "details.missing_fields" array
// naming only the fields that are actually missing.
func TestMapServiceError_BiodataIncomplete_BodyCarriesMissingFields(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = mapServiceError(c, &service.BiodataIncompleteError{Missing: []string{"grade"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: want 422, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "biodata_incomplete" {
		t.Errorf("code: want biodata_incomplete, got %v", body["code"])
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("want a non-empty message for a client that only reads message")
	}
	if !strings.Contains(msg, "kelas") {
		t.Errorf("message: want it to name grade (kelas), got %q", msg)
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("details: want an object, got %#v", body["details"])
	}
	fields, ok := details["missing_fields"].([]any)
	if !ok {
		t.Fatalf("details.missing_fields: want an array, got %#v", details["missing_fields"])
	}
	if len(fields) != 1 || fields[0] != "grade" {
		t.Errorf("details.missing_fields: want [grade], got %v", fields)
	}
}

func TestMapServiceError_OTPRateLimit_SetsRetryAfter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = mapServiceError(c, service.ErrOTPRateLimit)

	if h := rec.Header().Get("Retry-After"); h != "60" {
		t.Errorf("Retry-After header: want 60, got %q", h)
	}
}

func TestBadRequest(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = badRequest(c, "something is wrong")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}

	var apiErr APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "invalid_request" {
		t.Errorf("code: want %q, got %q", "invalid_request", apiErr.Code)
	}
	if apiErr.Message != "something is wrong" {
		t.Errorf("message: want %q, got %q", "something is wrong", apiErr.Message)
	}
}
