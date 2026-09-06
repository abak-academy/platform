package service

import "errors"

var (
	// ErrStorageNotConfigured replaces eight separate inline allocations of the
	// same message across certificate/exam/job/student, so callers can match it
	// with errors.Is instead of comparing strings.
	ErrStorageNotConfigured = errors.New("storage not configured")
	ErrIncompleteSchedule   = errors.New("delivery_date and delivery_time must be given together")

	ErrAlreadyAttempted     = errors.New("already attempted")
	ErrExamNotStarted       = errors.New("exam not started")
	ErrDeviceMismatch       = errors.New("device mismatch")
	ErrCheckinWindowClosed  = errors.New("check-in window closed")
	ErrExamWindowClosed     = errors.New("exam availability window closed")
	ErrNotCheckedIn         = errors.New("not checked in")
	ErrAlreadySubmitted     = errors.New("already submitted")
	ErrSessionNotFound      = errors.New("session not found")
	ErrInvalidViolationType = errors.New("invalid violation type")

	// Result gating (FR-S5-21) and essay grading (FR-S5-13) sentinels.
	// The gating non-result states (hidden/grading/locked) are returned as data on
	// SessionResult, not as errors — these are for the grading write path and any
	// hard-fail result cases.
	ErrResultHidden      = errors.New("result hidden")
	ErrResultNotReleased = errors.New("result not released")
	ErrSessionNotGraded  = errors.New("session not fully graded")
	ErrGradeOutOfRange   = errors.New("grade out of range")
	ErrNotEssayQuestion  = errors.New("question is not an essay")

	ErrLeaderboardNotAvailable = errors.New("leaderboard not available")

	// Sectioned-exam errors (FR-11/FR-14).
	ErrSectionLocked    = errors.New("section locked")
	ErrSectionNotActive = errors.New("section not active")
)

// --- from admin_students.go ---
var (
	ErrSchoolDeactivated = errors.New("school is deactivated")
	ErrStudentNotFound   = errors.New("student not found")
	ErrInvalidJenjang    = errors.New("invalid jenjang for school")
	ErrIncompleteAddress = errors.New("incomplete address: all or none of provinsi/kota/kecamatan required")
	ErrInvalidProvinsi   = errors.New("invalid provinsi")
	ErrInvalidKota       = errors.New("invalid kota")
	ErrInvalidKecamatan  = errors.New("invalid kecamatan")
	ErrInvalidKodePos    = errors.New("invalid kode pos: digits only")
	ErrInvalidGender     = errors.New("invalid gender: expected male or female")
)

// --- from admin_users.go ---
var (
	ErrCannotDeactivateSelf = errors.New("cannot deactivate your own account")
	ErrInvalidAdminRole     = errors.New("role must be an admin role")
	// ErrGoogleAccountNoPassword is returned when promoting/moving an account
	// that signs in via Google and has never set a password — doing so would
	// leave the account with neither door open (spec FR-9). Indonesian,
	// user-facing copy per spec C-4.
	ErrGoogleAccountNoPassword = errors.New("akun ini masuk menggunakan Google dan belum memiliki kata sandi. Super admin harus mengatur kata sandi melalui aksi reset password terlebih dahulu.")
	ErrInvalidRoleFilter       = errors.New("invalid role filter")
	ErrInvalidStatusFilter     = errors.New("invalid status filter")
	ErrAccountNoEmail          = errors.New("account has no email for reset")
	ErrMissingField            = errors.New("missing required field")
	ErrSchoolRequired          = errors.New("school_id is required for admin_school role")
	ErrSchoolNotAllowed        = errors.New("school_id must be empty for this role")
)

// --- from announcement.go ---
var (
	// ErrAnnouncementImmutable is returned when attempting to edit/delete/send
	// an announcement that has already been sent. Maps to 409 conflict.
	ErrAnnouncementImmutable = errors.New("announcement is immutable: already sent")
	// ErrAnnouncementNotFound is returned when the announcement does not exist.
	// Maps to 404 not found.
	ErrAnnouncementNotFound = errors.New("announcement not found")
	// ErrInvalidAnnouncementField is returned when a field value is invalid
	// (e.g. unknown type, recipients, or status). Maps to 400 invalid_request.
	ErrInvalidAnnouncementField = errors.New("invalid announcement field")
)

// --- from audit_read.go ---
var (
	ErrInvalidDateFormat = errors.New("invalid date format")
)

// --- from auth.go ---
var (
	ErrEmailTaken          = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrOTPRateLimit        = errors.New("otp requested too recently")
	ErrOTPExpired          = errors.New("otp expired")
	ErrInvalidOTP          = errors.New("invalid otp")
	ErrInvalidPendingToken = errors.New("invalid pending token")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidResetToken   = errors.New("invalid reset token")
	ErrAccountDeactivated  = errors.New("account deactivated")
	// ErrGoogleNotStudent is returned when a Google identity matching an
	// existing non-student account tries to sign in via /auth/google — Google
	// sign-in is student-only (spec FR-2/FR-3). Indonesian, user-facing copy
	// per spec C-4; deliberately does not name the role it holds.
	ErrGoogleNotStudent    = errors.New("akun ini bukan akun siswa. Silakan masuk melalui /admin/login.")
	ErrWeakPassword        = errors.New("password too weak")
	ErrInvalidToken        = errors.New("invalid token")
	ErrInvalidUUID         = errors.New("invalid uuid")
	ErrVerificationPending = errors.New("email verification pending")
)

// --- from bulk_exam_order.go ---
var (
	// ErrZeroNetNewParticipants is returned when all selected participants
	// are already registered for the exam (FR-BULK-05).
	ErrZeroNetNewParticipants = errors.New("all selected participants are already registered for this exam")
	// ErrExamNotOrderable is returned when the exam's product is not published
	// or not found (FR-BULK-08).
	ErrExamNotOrderable = errors.New("exam is not orderable")
)

// --- from course.go ---
var (
	ErrNoCourseAccess = errors.New("no active course access")
)

// --- from exam.go ---
var (
	ErrTestNotFound         = errors.New("test not found")
	ErrQuestionNotFound     = errors.New("question not found")
	ErrExamNotFound         = errors.New("exam not found")
	ErrRegistrationNotFound = errors.New("registration not found")
	ErrValidation           = errors.New("validation failed")
	// ErrQuestionInLiveExam is returned when deleting a question that is
	// attached (via test_question -> exam_test -> exam) to an exam that is
	// live: sold through a published product, or already has a session (FR-7).
	ErrQuestionInLiveExam = errors.New("question is in a live exam and cannot be deleted")
	// ErrQuestionFormatLocked is returned when an update changes a live-exam
	// question's format (FR-14). Other fields on the same question remain editable.
	ErrQuestionFormatLocked = errors.New("question format is locked while the question is in a live exam")
	// ErrInvalidPosition is returned when a SaveAnswers request carries a
	// current_position outside [0, total_questions) for the exam (FR-35). Maps
	// to 400; rejects the whole request so answers are never written either.
	ErrInvalidPosition = errors.New("invalid session position")
	// ErrCertificateDisabled is returned when the certificate feature is
	// disabled for an exam (certificate_enabled = false); the admin design
	// editor is unreachable until an admin explicitly enables it (FB-8/FR-9).
	ErrCertificateDisabled = errors.New("certificate disabled for this exam")
	// ErrCardDisabled is returned when the participant card is disabled for an
	// exam (card_enabled = false).
	ErrCardDisabled = errors.New("card disabled for this exam")
)

// --- from exam_grant.go ---
var (
	// ErrInvalidGrantStudent is returned when a student_id in the grant request
	// does not exist or is not a student (FR-GRANT-06).
	ErrInvalidGrantStudent = errors.New("one or more student IDs are invalid or not students")
)

// --- from exam_topic.go ---
var (
	// ErrTopicNotFound is returned when a topic lookup fails.
	ErrTopicNotFound = errors.New("topic not found")
)

// --- from item_qty.go ---
var (
	// ErrDigitalQtyLimit is returned when a caller tries to put more than one unit
	// of a digital product in an order.
	ErrDigitalQtyLimit = errors.New("digital products are limited to one per order")
	// ErrInvalidQty is returned for a non-positive quantity. There is no generic
	// validation sentinel in this package, so the rule owns its own.
	ErrInvalidQty = errors.New("invalid quantity")
)

// --- from job.go ---
var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrJobNotFound    = errors.New("job not found")
)

// --- from participants.go ---
var (
	// ErrCrossSchoolStudent is returned when a student_id in the selector
	// does not belong to the specified school (FR-BULK-03).
	ErrCrossSchoolStudent = errors.New("student does not belong to this school")
	// ErrEmptySelector is returned when no selection criteria are provided.
	ErrEmptySelector = errors.New("empty participant selector")
	// ErrDuplicateParticipant is returned when the same student_id appears more
	// than once in a participant selector (FR-BULK-03).
	ErrDuplicateParticipant = errors.New("duplicate student_id in participant selector")
)

// --- from product_specs.go ---
var (
	// ErrInvalidSpecs is returned when a product specification array is malformed.
	ErrInvalidSpecs = errors.New("invalid product specs")
)

// --- from school.go ---
var (
	ErrSchoolNotFound    = errors.New("school not found")
	ErrSchoolCodeTaken   = errors.New("school code already taken")
	ErrInvalidSchoolName = errors.New("school name must contain at least one letter or digit")
)

// --- from shipping_rates.go ---
var (
	// ErrShippingUnavailable means no carrier quote could be obtained and no
	// flat-rate fallback is configured.
	ErrShippingUnavailable = errors.New("shipping is unavailable")
)

// --- from biteship.go ---
var (
	// ErrShippingOriginUnset means app_kode_pos is not configured in system_config.
	ErrShippingOriginUnset = errors.New("shipping origin postal code not configured")
	// ErrShippingAuthRejected means Biteship rejected the request as unauthorized (401/403).
	ErrShippingAuthRejected = errors.New("shipping provider rejected credentials")
)

// --- from store.go ---
var (
	ErrForbidden               = errors.New("forbidden")
	ErrProductNotFound         = errors.New("product not found")
	ErrCourseNotFound          = errors.New("course not found")
	ErrInvalidPromo            = errors.New("invalid or expired promo code")
	ErrPromoMinOrder           = errors.New("order subtotal below promo minimum")
	ErrOutOfStock              = errors.New("product out of stock")
	ErrInsufficientStock       = errors.New("insufficient stock")
	ErrOrderNotEditable        = errors.New("order not editable")
	ErrOrderChanged            = errors.New("order changed during update") // concurrent cart mutation; re-read and re-quote still lost the race
	ErrOrderNotFound           = errors.New("order not found")
	ErrMustShipBeforeComplete  = errors.New("order has physical items — must be shipped before completing")
	ErrInvalidSignature        = errors.New("invalid signature")
	ErrCourseLinkRequired      = errors.New("course product requires at least one linked course")
	ErrExamLinkRequired        = errors.New("exam product requires at least one linked exam")
	ErrShippingRequired        = errors.New("order requires a shipping selection before checkout")
	ErrInvalidCourierSelection = errors.New("selected courier is not available for this destination")
	ErrBiodataIncomplete       = errors.New("lengkapi biodata (sekolah, kelas, tanggal lahir) sebelum mendaftar ujian")
	ErrMissingGatewayRef       = errors.New("order has no gateway reference to reconcile")

	// ErrOrderNotShippable and ErrNoCarrierCode gate AdminShipOrder /
	// AdminShipOrderManual (shipping_order.go) before any Biteship call is made.
	ErrOrderNotShippable = errors.New("order not in shippable status")

	// ErrOrderNotRefundable gates AdminRefundOrder: a refund may only be
	// recorded against a status where money was actually taken. Maps to 422.
	ErrOrderNotRefundable = errors.New("order not in refundable status")
	ErrNoCarrierCode      = errors.New("order has no persisted courier code")
)

// --- from student_bulk.go ---
var (
	ErrInvalidCSV           = errors.New("invalid csv")
	ErrMissingCSVHeader     = errors.New("csv missing required name/jenjang/school header")
	ErrRowLimitExceeded     = errors.New("row limit exceeded")
	ErrSchoolNotFoundByName = errors.New("school not found by name")
	ErrCrossSchoolBound     = errors.New("school mismatch: row school differs from bound school")
	ErrInvalidDOBFormat     = errors.New("invalid dob format, expected YYYY-MM-DD")
	ErrInvalidGradeFormat   = errors.New("invalid grade, expected an integer")
)

// --- from school_bulk.go ---
var (
	// ErrMissingSchoolCSVHeader is distinct from ErrMissingCSVHeader (whose
	// message names name/jenjang/school) because the school-bulk header set is
	// name/code and that message would mislead here.
	ErrMissingSchoolCSVHeader = errors.New("csv missing required name/code header")
)

// --- from system_config.go ---
var (
	// ErrConfigEncryption is returned when AES-256-GCM encryption or decryption
	// fails (invalid key, tampered ciphertext, etc.). Maps to 500 internal_error.
	ErrConfigEncryption = errors.New("config encryption failed")
	// ErrUnknownConfigKey is returned when a provided config key is not in the
	// fixed catalog. Maps to 400 invalid_request.
	ErrUnknownConfigKey = errors.New("unknown config key")
)

// --- from username.go ---
var (
	// ErrUsernameGenerationExhausted is returned when 10 attempts to generate a
	// unique username all collide with existing usernames.
	ErrUsernameGenerationExhausted = errors.New("username generation exhausted after 10 attempts")
)
