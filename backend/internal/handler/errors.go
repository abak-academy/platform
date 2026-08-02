package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func mapServiceError(c echo.Context, err error) error {
	var status int
	var apiErr APIError
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		status, apiErr = http.StatusConflict, APIError{Code: "email_taken", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidCredentials):
		status, apiErr = http.StatusUnauthorized, APIError{Code: "invalid_credentials", Message: err.Error()}
	case errors.Is(err, service.ErrOTPRateLimit):
		c.Response().Header().Set("Retry-After", "60")
		status, apiErr = http.StatusTooManyRequests, APIError{Code: "rate_limited", Message: err.Error()}
	case errors.Is(err, service.ErrOTPExpired):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "otp_invalid", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidOTP):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "otp_invalid", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidPendingToken):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "otp_invalid", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidRefreshToken):
		status, apiErr = http.StatusUnauthorized, APIError{Code: "invalid_refresh_token", Message: err.Error()}
	case errors.Is(err, service.ErrUserNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "user_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidResetToken):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "reset_invalid", Message: err.Error()}
	case errors.Is(err, service.ErrAccountDeactivated):
		status, apiErr = http.StatusForbidden, APIError{Code: "account_deactivated", Message: err.Error()}
	case errors.Is(err, service.ErrVerificationPending):
		status, apiErr = http.StatusForbidden, APIError{Code: "verification_pending", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidToken):
		status, apiErr = http.StatusUnauthorized, APIError{Code: "invalid_google_token", Message: "invalid or expired Google token"}
	case errors.Is(err, service.ErrInvalidUUID):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrWeakPassword):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error(), Details: "password must be at least 8 characters"}
	case errors.Is(err, service.ErrInvalidSpecs):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_specs", Message: err.Error()}
	case errors.Is(err, service.ErrProductNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "product_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrCourseNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "course_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrForbidden):
		status, apiErr = http.StatusForbidden, APIError{Code: "forbidden", Message: err.Error()}
	case errors.Is(err, service.ErrOutOfStock):
		status, apiErr = http.StatusConflict, APIError{Code: "out_of_stock", Message: err.Error()}
	case errors.Is(err, service.ErrInsufficientStock):
		status, apiErr = http.StatusConflict, APIError{Code: "insufficient_stock", Message: err.Error()}
	case errors.Is(err, service.ErrOrderNotEditable):
		status, apiErr = http.StatusConflict, APIError{Code: "order_not_editable", Message: err.Error()}
	case errors.Is(err, service.ErrOrderNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "order_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrTestNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "test_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrQuestionNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "question_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrQuestionInLiveExam):
		status, apiErr = http.StatusConflict, APIError{Code: "question_in_published_exam", Message: err.Error()}
	case errors.Is(err, service.ErrQuestionFormatLocked):
		status, apiErr = http.StatusConflict, APIError{Code: "question_format_locked", Message: err.Error()}
	case errors.Is(err, service.ErrExamNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "exam_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrCertificateDisabled):
		status, apiErr = http.StatusNotFound, APIError{Code: "certificate_disabled", Message: err.Error()}
	case errors.Is(err, service.ErrTopicNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "topic_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrRegistrationNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "registration_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrValidation):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "validation_failed", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidPromo):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_promo", Message: err.Error()}
	case errors.Is(err, service.ErrPromoMinOrder):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "promo_min_order", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidSignature):
		status, apiErr = http.StatusUnauthorized, APIError{Code: "invalid_signature", Message: err.Error()}
	case errors.Is(err, service.ErrCourseLinkRequired):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "course_required", Message: err.Error()}
	case errors.Is(err, service.ErrNoCourseAccess):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "no_course_access", Message: err.Error()}
	case errors.Is(err, service.ErrUnknownConfigKey):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrConfigEncryption):
		status, apiErr = http.StatusInternalServerError, APIError{Code: "internal_error", Message: "config encryption failed"}
	case errors.Is(err, service.ErrCannotDeactivateSelf):
		status, apiErr = http.StatusForbidden, APIError{Code: "cannot_deactivate_self", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidAdminRole):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidRoleFilter):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidStatusFilter):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrAccountNoEmail):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "reset_invalid", Message: err.Error()}
	case errors.Is(err, service.ErrMissingField):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidDateFormat):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrAnnouncementNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "announcement_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrAnnouncementImmutable):
		status, apiErr = http.StatusConflict, APIError{Code: "announcement_immutable", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidAnnouncementField):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrSchoolNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "school_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrSchoolCodeTaken):
		status, apiErr = http.StatusConflict, APIError{Code: "school_code_taken", Message: err.Error()}
	case errors.Is(err, service.ErrSchoolRequired):
		status, apiErr = http.StatusBadRequest, APIError{Code: "school_required", Message: err.Error()}
	case errors.Is(err, service.ErrSchoolNotAllowed):
		status, apiErr = http.StatusBadRequest, APIError{Code: "school_not_allowed", Message: err.Error()}
	case errors.Is(err, service.ErrSchoolDeactivated):
		status, apiErr = http.StatusConflict, APIError{Code: "school_deactivated", Message: err.Error()}
	case errors.Is(err, service.ErrStudentNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "student_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidJenjang):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_jenjang", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidGender):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_gender", Message: err.Error()}
	case errors.Is(err, service.ErrIncompleteAddress):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "incomplete_address", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidProvinsi):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_provinsi", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidKota):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_kota", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidKecamatan):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_kecamatan", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidKodePos):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_kode_pos", Message: err.Error()}
	case errors.Is(err, service.ErrUsernameGenerationExhausted):
		status, apiErr = http.StatusServiceUnavailable, APIError{Code: "username_generation_exhausted", Message: err.Error()}
	case errors.Is(err, service.ErrUploadNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "upload_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidCSV):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_csv", Message: err.Error()}
	case errors.Is(err, service.ErrMissingCSVHeader):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_csv_headers", Message: err.Error()}
	case errors.Is(err, service.ErrRowLimitExceeded):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "row_limit_exceeded", Message: err.Error()}
	case errors.Is(err, service.ErrJobNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "job_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrAlreadyAttempted):
		status, apiErr = http.StatusConflict, APIError{Code: "already_attempted", Message: err.Error()}
	case errors.Is(err, service.ErrExamNotStarted):
		status, apiErr = http.StatusConflict, APIError{Code: "exam_not_started", Message: err.Error()}
	case errors.Is(err, service.ErrDeviceMismatch):
		status, apiErr = http.StatusForbidden, APIError{Code: "device_mismatch", Message: err.Error()}
	case errors.Is(err, service.ErrCheckinWindowClosed):
		status, apiErr = http.StatusConflict, APIError{Code: "checkin_window_closed", Message: err.Error()}
	case errors.Is(err, service.ErrExamWindowClosed):
		status, apiErr = http.StatusConflict, APIError{Code: "exam_window_closed", Message: err.Error()}
	case errors.Is(err, service.ErrNotCheckedIn):
		status, apiErr = http.StatusConflict, APIError{Code: "not_checked_in", Message: err.Error()}
	case errors.Is(err, service.ErrAlreadySubmitted):
		status, apiErr = http.StatusConflict, APIError{Code: "already_submitted", Message: err.Error()}
	case errors.Is(err, service.ErrSessionNotFound):
		status, apiErr = http.StatusNotFound, APIError{Code: "session_not_found", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidViolationType):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_violation_type", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidPosition):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_position", Message: err.Error()}
	case errors.Is(err, service.ErrGradeOutOfRange):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "grade_out_of_range", Message: err.Error()}
	case errors.Is(err, service.ErrNotEssayQuestion):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "not_essay_question", Message: err.Error()}
	case errors.Is(err, service.ErrResultHidden):
		status, apiErr = http.StatusForbidden, APIError{Code: "result_hidden", Message: err.Error()}
	case errors.Is(err, service.ErrResultNotReleased):
		status, apiErr = http.StatusConflict, APIError{Code: "result_not_released", Message: err.Error()}
	case errors.Is(err, service.ErrSessionNotGraded):
		status, apiErr = http.StatusConflict, APIError{Code: "session_not_graded", Message: err.Error()}
	case errors.Is(err, service.ErrLeaderboardNotAvailable):
		status, apiErr = http.StatusForbidden, APIError{Code: "leaderboard_not_available", Message: err.Error()}
	case errors.Is(err, service.ErrSectionLocked):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "section_locked", Message: err.Error()}
	case errors.Is(err, service.ErrSectionNotActive):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "section_not_active", Message: err.Error()}
	case errors.Is(err, service.ErrCrossSchoolStudent):
		status, apiErr = http.StatusForbidden, APIError{Code: "cross_school_student", Message: err.Error()}
	case errors.Is(err, service.ErrDuplicateParticipant):
		status, apiErr = http.StatusBadRequest, APIError{Code: "duplicate_participant", Message: err.Error()}
	case errors.Is(err, service.ErrEmptySelector):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrZeroNetNewParticipants):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "zero_net_new", Message: err.Error()}
	case errors.Is(err, service.ErrExamNotOrderable):
		status, apiErr = http.StatusNotFound, APIError{Code: "exam_not_orderable", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidGrantStudent):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_grant_student", Message: err.Error()}
	case errors.Is(err, service.ErrShippingRequired):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidCourierSelection):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "invalid_courier_selection", Message: err.Error()}
	case errors.Is(err, service.ErrBiodataIncomplete):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "biodata_incomplete", Message: err.Error()}
		var bioErr *service.BiodataIncompleteError
		if errors.As(err, &bioErr) {
			apiErr.Details = map[string]any{"missing_fields": bioErr.Missing}
		}
	case errors.Is(err, service.ErrShippingUnavailable):
		status, apiErr = http.StatusServiceUnavailable, APIError{Code: "shipping_unavailable", Message: "shipping is not available right now"}
	case errors.Is(err, service.ErrDigitalQtyLimit):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_qty", Message: err.Error()}
	case errors.Is(err, service.ErrInvalidQty):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_qty", Message: err.Error()}
	case errors.Is(err, service.ErrNoCarrierCode):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "no_carrier_code", Message: err.Error()}
	case errors.Is(err, service.ErrOrderNotShippable):
		status, apiErr = http.StatusConflict, APIError{Code: "order_not_shippable", Message: err.Error()}
	case errors.Is(err, service.ErrOrderNotRefundable):
		status, apiErr = http.StatusConflict, APIError{Code: "order_not_refundable", Message: err.Error()}
	case errors.Is(err, service.ErrNoTrackingNumber):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "no_tracking_number", Message: err.Error()}
	case errors.Is(err, service.ErrMissingGatewayRef):
		status, apiErr = http.StatusUnprocessableEntity, APIError{Code: "missing_gateway_ref", Message: err.Error()}
	case errors.Is(err, repository.ErrInvalidCursor):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_cursor", Message: err.Error()}
	default:
		slog.Error("unhandled service error", "method", c.Request().Method, "uri", c.Request().RequestURI, "err", err)
		status, apiErr = http.StatusInternalServerError, APIError{Code: "internal_error", Message: "internal server error"}
	}
	return c.JSON(status, apiErr)
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, APIError{Code: "invalid_request", Message: msg})
}
