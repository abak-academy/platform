package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExamTopic is a curated (subject, name) pair used by reusable bank questions.
// QuestionCount is only populated by list-style reads.
type ExamTopic struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Subject       string    `json:"subject"`
	QuestionCount int       `json:"question_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Test is the top-level authoring unit (a set of questions). Nullable audio fields
// are pointer types so we can persist / return "not set" distinctly from empty strings.
type Test struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Subject         string    `json:"subject"`
	Topic           string    `json:"topic"`
	DurationMinutes int       `json:"duration_minutes"`
	AudioURL        *string   `json:"audio_url"`
	AudioPlayLimit  *int      `json:"audio_play_limit"`
	// SectionType identities an IELTS section (listening|reading|writing); NULL for
	// standard tests and UTBK subtests. Pointer so "not set" is distinct from "".
	SectionType *string `json:"section_type,omitempty"`
	// QuestionCount is only populated by list-style reads (e.g. ListTests LEFT JOIN).
	// It is zero on freshly created tests and on direct GetByID reads.
	QuestionCount int       `json:"question_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Question is a reusable bank item. `Format` is one of: mcq, multi_answer, short,
// fill_blank, multi_blank, essay. Options are stored separately on QuestionOption (composite PK)
// and surfaced via QuestionWithOptions for read paths. topic_id links to the curated
// exam_topic list; it is nullable for questions created before topics were assigned.
type Question struct {
	ID             uuid.UUID  `json:"id"`
	QuestionNumber int        `json:"question_number"`
	Format         string     `json:"format"`
	Body           string     `json:"body"`
	CorrectAnswer  *string    `json:"correct_answer"`
	Explanation    *string    `json:"explanation"`
	Difficulty     *string    `json:"difficulty"`
	ImageURL       *string    `json:"image_url"`
	AudioURL       *string    `json:"audio_url"`
	TopicID        *uuid.UUID `json:"topic_id"`
	Topic          *string    `json:"topic"`
	// PointCorrect and PointWrong are positive magnitudes (fractional allowed) authored
	// per question; the scoring engine (not the author) applies the sign for wrong answers.
	PointCorrect float64 `json:"point_correct"`
	PointWrong   float64 `json:"point_wrong"`
	// AcceptedAnswers is the question-level accepted-answer set (short/fill_blank only);
	// always a non-nil slice on read (falls back to []string{*CorrectAnswer} when no
	// question_accepted_answer rows exist, FR-27).
	AcceptedAnswers []string `json:"accepted_answers"`
	// Statements is the true_false statement set (admin payloads only); the
	// student session shape strips IsTrue. Lives here, not on the wrapper —
	// web/lib/types.ts declares it on Question.
	Statements []QuestionStatement `json:"statements"`
}

// QuestionOption has a composite PK (QuestionID, Key); no surrogate ID. `Key` is the
// letter shown to candidates (a, b, c, d…). `IsCorrect` is server-validated per format.
type QuestionOption struct {
	QuestionID uuid.UUID `json:"question_id"`
	Key        string    `json:"key"`
	Text       string    `json:"text"`
	ImageURL   *string   `json:"image_url"`
	IsCorrect  bool      `json:"is_correct"`
	SortOrder  int       `json:"sort_order"`
	// Points is this option's worth when selected correctly; nil falls back to
	// the question's PointCorrect (0050, per-item points).
	Points *float64 `json:"points,omitempty"`
}

// QuestionBlank has a composite PK (QuestionID, BlankIndex); no surrogate ID.
// Used for multi_blank questions to store per-blank correct answers.
type QuestionBlank struct {
	QuestionID    uuid.UUID `json:"question_id"`
	Index         int       `json:"index"`
	CorrectAnswer string    `json:"correct_answer"`
	// AcceptedAnswers is this blank's accepted-answer set; always a non-nil slice on
	// read (falls back to []string{CorrectAnswer} when no accepted-answer rows exist).
	AcceptedAnswers []string `json:"accepted_answers"`
	// Points is this blank's worth; nil falls back to the question's PointCorrect.
	Points *float64 `json:"points,omitempty"`
}

// QuestionStatement has a composite PK (QuestionID, Index); no surrogate ID.
// Used for true_false questions to store ordered statements. IsTrue is the
// answer key — it must never appear in a student-facing payload (NFR-5).
type QuestionStatement struct {
	QuestionID uuid.UUID `json:"question_id"`
	Index      int       `json:"index"`
	Body       string    `json:"body"`
	IsTrue     bool      `json:"is_true"`
	// Points is this statement's worth; nil falls back to the question's PointCorrect.
	Points *float64 `json:"points,omitempty"`
}

// Exam is a scheduled test offering. It bundles one or more Tests via ExamTest and may
// be sold via product — M:N through product_exam (mirrors Course/product_course), so a
// Product can attach more than one Exam and an Exam has no direct product reference.
type Exam struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	IsFree      bool       `json:"is_free"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	// ScheduledEndAt, when set, turns ScheduledAt into the start of an
	// availability window rather than a single fixed instant — students may
	// check in/start any time in [ScheduledAt, ScheduledEndAt].
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
	RequiresCheckin      bool       `json:"requires_checkin"`
	AllowLeaderboard     bool       `json:"allow_leaderboard"`
	CDNBundle            bool       `json:"cdn_bundle"`
	BundleURL            *string    `json:"bundle_url"`
	BundleGeneratedAt    *time.Time `json:"bundle_generated_at"`
	CheckInWindowMinutes *int       `json:"check_in_window_minutes"`
	GraceWindowMinutes   *int       `json:"grace_window_minutes"`
	MaxAttempts          *int       `json:"max_attempts"`
	TimerMode            string     `json:"timer_mode"`
	DurationMinutes      *int       `json:"duration_minutes"`
	Randomize            bool       `json:"randomize"`
	ResultConfig         string     `json:"result_config"`
	ResultReleaseAt      *time.Time `json:"result_release_at"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	// Mode discriminates standard vs sectioned (utbk|ielts) exams. NOT NULL DEFAULT
	// 'standard' in the DB; omitempty no-ops since 'standard' is non-empty — admin
	// payloads gain the key, student-facing payloads are assembled in the service.
	Mode string `json:"mode,omitempty"`
	// CertificateDesign is the single persisted JSON blob for the certificate editor:
	// template, background (kind/ref plus the private-bucket object key for a custom
	// upload — never a raw or presigned URL), signature_key, and fields (FR-26/FR-27).
	// Nil until an admin has saved a design; consolidates what were previously three
	// separate columns (certificate_template, certificate_background_key,
	// certificate_layout — folded by migration 0042).
	CertificateDesign *json.RawMessage `json:"certificate_design"`
	// CertificateDesignUpdatedAt is bumped whenever template, background key, or layout
	// changes (C3/FR-14) — the single staleness timestamp compared against a session's
	// certificate_generated_at.
	CertificateDesignUpdatedAt *time.Time `json:"certificate_design_updated_at"`
	// CertificateEnabled gates the whole certificate feature for this exam
	// (FB-8/FR-8..FR-12); DEFAULT false. Mutated only via the dedicated
	// enable/disable action, never through the general exam PATCH, so toggling
	// it never touches CertificateDesign or CertificateDesignUpdatedAt.
	CertificateEnabled bool `json:"certificate_enabled"`
	// CertificateTemplateHTML is the FE-serialized self-contained HTML for this
	// exam's certificate design, carrying {{token}} placeholders for every
	// verified value (score, student_name, certificate_number, image URLs...).
	// Nil until an admin has saved a design (migration 0056). The worker
	// substitutes it at generation time; nothing here is ever trusted as a
	// finished document on its own.
	CertificateTemplateHTML *string `json:"certificate_template_html,omitempty"`
	// CardEnabled gates the participant card for this exam; DEFAULT false, so new
	// exams are opt-in. Mutated only via the dedicated enable/disable action,
	// never through the general exam PATCH.
	CardEnabled bool `json:"card_enabled"`
	// CardNotes are the admin-authored "Perhatian" bullets printed on the card.
	// Empty falls back to the built-in defaults; the generated check-in bullet is
	// always appended after these.
	CardNotes []string `json:"card_notes"`
	// ExamNumber is a global human-friendly serial (FR-23) assigned from exam_number_seq,
	// distinct from the exam UUID. Non-nil after create; nil only pre-migration/pre-backfill.
	ExamNumber *int `json:"exam_number"`
	// EndScreenImageURL and EndScreenPromoText are the single admin-configured
	// image/promo block shown on the post-submit result screen (FR-38/FR-39);
	// both nil until an admin sets them. No templating — plain values only.
	EndScreenImageURL  *string `json:"end_screen_image_url"`
	EndScreenPromoText *string `json:"end_screen_promo_text"`
}

// ExamTest is the M:N join between Exam and Test with sort order.
type ExamTest struct {
	ID        uuid.UUID `json:"id"`
	ExamID    uuid.UUID `json:"exam_id"`
	TestID    uuid.UUID `json:"test_id"`
	SortOrder int       `json:"sort_order"`
}

// ExamRegistration enrolls a student in an exam; (student_id, exam_id) is unique.
type ExamRegistration struct {
	ID           uuid.UUID  `json:"id"`
	StudentID    uuid.UUID  `json:"student_id"`
	ExamID       uuid.UUID  `json:"exam_id"`
	Token        string     `json:"token"`
	CardKey      *string    `json:"card_key"`
	CheckedInAt  *time.Time `json:"checked_in_at"`
	AttemptsUsed int        `json:"attempts_used"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	// ParticipantNumber is a per-exam sequence assigned at registration (nil for
	// rows predating the column until backfilled). Displayed as ParticipantNo.
	ParticipantNumber *int `json:"participant_number"`
}

// ExamSession is one in-flight attempt by a student; multiple sessions per registration
// are numbered by AttemptNumber.
type ExamSession struct {
	ID                     uuid.UUID  `json:"id"`
	RegistrationID         uuid.UUID  `json:"registration_id"`
	StudentID              uuid.UUID  `json:"student_id"`
	ExamID                 uuid.UUID  `json:"exam_id"`
	AttemptNumber          int        `json:"attempt_number"`
	StartedAt              time.Time  `json:"started_at"`
	SubmittedAt            *time.Time `json:"submitted_at"`
	ExtendedUntil          *time.Time `json:"extended_until"`
	AdminSubmitted         bool       `json:"admin_submitted"`
	Score                  *float64   `json:"score"`
	CertificateKey         *string    `json:"certificate_key"`
	CertificateGeneratedAt *time.Time `json:"certificate_generated_at"`
	// CertificateNumber is allocated once (ABK/YYYY/NNNNNN) on first certificate
	// generation and reused thereafter; nil until allocated (FR-9/FR-10).
	CertificateNumber *string    `json:"certificate_number"`
	LastSavedAt       *time.Time `json:"last_saved_at"`
	// CurrentPosition is the student's last-known question index (FR-35/FR-36);
	// nil until the first save that carries a position.
	CurrentPosition *int      `json:"current_position"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// ExamSessionAnswer is one answer record per (session, question) — composite PK.
// Score is NUMERIC(6,2) → float64; nullable because essay answers are graded later.
type ExamSessionAnswer struct {
	SessionID        uuid.UUID  `json:"session_id"`
	QuestionID       uuid.UUID  `json:"question_id"`
	Answer           *string    `json:"answer"`
	IsCorrect        *bool      `json:"is_correct"`
	Score            *float64   `json:"score"`
	GradedBy         *uuid.UUID `json:"graded_by"`
	GradedAt         *time.Time `json:"graded_at"`
	GraderComment    *string    `json:"grader_comment"`
	FlaggedForReview bool       `json:"flagged_for_review"`
	SavedAt          time.Time  `json:"saved_at"`
}

// SessionViolationLog records integrity events (tab-switch, copy-paste, etc.) for a session.
type SessionViolationLog struct {
	ID            uuid.UUID `json:"id"`
	SessionID     uuid.UUID `json:"session_id"`
	StudentID     uuid.UUID `json:"student_id"`
	ViolationType string    `json:"violation_type"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// TestDetail is a composite read shape used by the authoring API for a single test page:
// the parent Test and its full ordered question list with inline options.
type TestDetail struct {
	Test      Test                  `json:"test"`
	Questions []QuestionWithOptions `json:"questions"`
}

// QuestionWithOptions is a composite read shape: a Question plus its inline option list and blanks.
// SortOrder carries the per-test order from test_question for authoring/ session reads.
// Options are empty for non-option formats (short / fill_blank / essay).
// Blanks are empty for non-multi_blank formats, never nil (consistent with options).
type QuestionWithOptions struct {
	Question  Question         `json:"question"`
	Options   []QuestionOption `json:"options"`
	Blanks    []QuestionBlank  `json:"blanks"`
	SortOrder int              `json:"sort_order"`
}

// BankQuestionListItem is one row of GET /admin/questions — a bank question with
// its inline options, blanks, topic name, and the count of tests it is currently attached
// to (Used-in). Nested (not embedded) to match the {question, options, blanks, ...} shape
// the admin bank page and QuestionWithOptions both expect.
type BankQuestionListItem struct {
	Question      Question         `json:"question"`
	Options       []QuestionOption `json:"options"`
	Blanks        []QuestionBlank  `json:"blanks"`
	AttachedCount int              `json:"attached_count"`
	// InLiveExam mirrors Service.IsQuestionInLiveExam so the admin bank page can
	// disable delete/format controls without a second round trip (FR-7/FR-14).
	InLiveExam bool `json:"in_live_exam"`
}

// ExamListItem is the read shape returned by GET /admin/exams. Cursor pagination
// assembles a slice of these. Price/status now live on the attached Product(s) — see
// GET /admin/products?type=exam, since a single Exam can be attached to more than one.
// HasPublishedProduct is a computed flag (true if any attached product is published)
// used by admin surfaces (e.g. the session monitor) that only care about exams
// currently on sale, without needing full product detail.
type ExamListItem struct {
	Exam                `json:",inline"`
	HasPublishedProduct bool `json:"has_published_product"`
	RegistrationCount   int  `json:"registration_count"`
}

// ExamTestEntry is the read shape for an exam_test row plus the inline Test metadata
// (title, subject, topic, duration_minutes, question_count) needed by the admin detail
// page without a second round-trip.
type ExamTestEntry struct {
	ExamTest `json:",inline"`
	Test     struct {
		ID              uuid.UUID `json:"id"`
		Title           string    `json:"title"`
		Subject         string    `json:"subject"`
		Topic           *string   `json:"topic"`
		DurationMinutes *int      `json:"duration_minutes"`
		SectionType     *string   `json:"section_type,omitempty"`
		QuestionCount   int       `json:"question_count"`
	} `json:"test"`
}

// ExamDetail is the read shape returned by GET /admin/exams/:id — full Exam config
// plus an ordered list of attached tests. Price/status live on the attached Product(s).
type ExamDetail struct {
	Exam  `json:",inline"`
	Tests []ExamTestEntry `json:"tests"`
}

// RegistrationListItem is the read shape returned by GET /api/v1/exam/registrations:
// an ExamRegistration joined with exam.title and exam.scheduled_at.
type RegistrationListItem struct {
	ExamRegistration     `json:",inline"`
	ExamTitle            string     `json:"exam_title"`
	ScheduledAt          *time.Time `json:"scheduled_at"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
	IsFree               bool       `json:"is_free"`
	RequiresCheckin      bool       `json:"requires_checkin"`
	CheckInWindowMinutes *int       `json:"check_in_window_minutes"`
	DurationMinutes      *int       `json:"duration_minutes"`
	SessionID            *uuid.UUID `json:"session_id"`
	// MaxAttempts is the exam's raw max_attempts column: nil means unlimited
	// retake, 0 or 1 means a single sitting. Carried so the card can compute
	// whether a submitted registration still has attempts left (FR20/FR21).
	MaxAttempts *int `json:"max_attempts"`
}

// RegistrationDetail is the read shape returned by GET /api/v1/exam/registrations/:id:
// an ExamRegistration joined with the nested exam config needed by the student detail page.
type RegistrationDetail struct {
	ExamRegistration `json:",inline"`
	// ParticipantNo is the display form of ParticipantNumber, "YYMMDD-<exam_number(pad4)>-NNNNNN"
	// where YYMMDD is the exam's scheduled start date (falls back to the
	// registration date if the exam is not yet scheduled). Empty if unassigned.
	ParticipantNo string `json:"participant_no"`
	// Subject is the aggregated subject(s) of the exam's attached test(s)
	// (Paket/Mapel on the card). Platform is the single, system-config-sourced
	// exam platform (Platform/Ruang on the card).
	Subject  string `json:"subject"`
	Platform string `json:"platform"`
	// FooterNote and Contact are the card's generated check-in bullet and its
	// system-config-sourced footer bar, supplied here so the on-screen card
	// renders exactly what the generated PDF does — a student cannot read
	// system_config directly.
	FooterNote string `json:"footer_note"`
	TenantName string `json:"tenant_name"`
	Contact    struct {
		Phone        string `json:"phone"`
		Email        string `json:"email"`
		HelpURL      string `json:"help_url"`
		SocialHandle string `json:"social_handle"`
	} `json:"contact"`
	Exam struct {
		ID                   uuid.UUID  `json:"id"`
		Title                string     `json:"title"`
		ScheduledAt          *time.Time `json:"scheduled_at"`
		ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
		RequiresCheckin      bool       `json:"requires_checkin"`
		CheckInWindowMinutes *int       `json:"check_in_window_minutes"`
		TimerMode            string     `json:"timer_mode"`
		DurationMinutes      *int       `json:"duration_minutes"`
		ResultConfig         string     `json:"result_config"`
		// ExamNumber is joined in for the participant-number display format (FR-24, Task 5).
		ExamNumber  *int     `json:"exam_number"`
		CardEnabled bool     `json:"card_enabled"`
		CardNotes   []string `json:"card_notes"`
	} `json:"exam"`
}

// ExamRosterEntry is one row of the admin participant roster for an exam
// (GET /admin/exams/:id/registrations, FR-32): a registration joined with the
// student's name/username. ParticipantNumber may be nil for rows predating
// FR-24 (participant_number backfill) — ParticipantNo stays "" in that case;
// the service composes it (formatParticipantNo) from the raw ExamScheduledAt/
// ExamNumber/RegisteredAt ingredients, which are join inputs only and not
// surfaced on the wire (json:"-").
// Token (FR-47/FR-48, NFR-S7) is the exam check-in credential; it must never
// be added to the CSV export or logged, and only ever served on this
// read/write-RBAC-gated admin endpoint (see routes.go's adminExamsRead group).
type ExamRosterEntry struct {
	RegistrationID    uuid.UUID  `json:"registration_id"`
	StudentID         uuid.UUID  `json:"student_id"`
	StudentName       string     `json:"student_name"`
	StudentUsername   *string    `json:"student_username"`
	ParticipantNumber *int       `json:"participant_number"`
	ParticipantNo     string     `json:"participant_no"`
	Status            string     `json:"status"`
	CheckedInAt       *time.Time `json:"checked_in_at"`
	Token             string     `json:"token"`
	RegisteredAt      time.Time  `json:"-"`
	ExamScheduledAt   *time.Time `json:"-"`
	ExamNumber        *int       `json:"-"`
}

type ExamRosterFilter struct {
	Cursor string
	Limit  int
	Sort   string
}

// SessionResult is the read shape for GET /api/v1/exam/sessions/:id/result. State is the
// gate discriminator ("hidden" | "grading" | "locked" | "result"); the remaining fields are
// populated per state (score/counts/rank always on "result"; breakdown/pembahasan only on
// "score_pembahasan"; ResultReleaseAt only on "locked").
type SessionResult struct {
	State           string                 `json:"state"`
	ResultConfig    string                 `json:"result_config,omitempty"`
	ResultReleaseAt *time.Time             `json:"result_release_at,omitempty"`
	Score           float64                `json:"score"`
	CorrectCount    int                    `json:"correct_count"`
	WrongCount      int                    `json:"wrong_count"`
	EmptyCount      int                    `json:"empty_count"`
	Rank            int                    `json:"rank"`
	Breakdown       []ResultTopicRow       `json:"breakdown,omitempty"`
	Pembahasan      []ResultPembahasanItem `json:"pembahasan,omitempty"`
	// CertificateURL has no omitempty (NFR-R3, FR-5): the key must appear on
	// every gated result state, carrying null when the gate denies or a
	// certificate render fails — omitempty would silently drop the key
	// instead of serialising null.
	CertificateURL *string `json:"certificate_url"`
	// EndScreenImageURL/EndScreenPromoText mirror the exam's configured
	// post-submit content (FR-38/FR-39); present on every gate state (like
	// CertificateURL) since they're shown regardless of result visibility.
	// omitempty so an unconfigured exam's response matches today's shape.
	EndScreenImageURL  *string `json:"end_screen_image_url,omitempty"`
	EndScreenPromoText *string `json:"end_screen_promo_text,omitempty"`
}

// ResultTopicRow is one per-Test row of the score_pembahasan breakdown (FR-S5-19).
// Max is the sum of point_correct across the test's questions (objective + essay).
type ResultTopicRow struct {
	TestID      uuid.UUID `json:"test_id"`
	Title       string    `json:"title"`
	Subject     string    `json:"subject"`
	Topic       string    `json:"topic"`
	SectionType *string   `json:"section_type,omitempty"`
	Earned      float64   `json:"earned"`
	Max         float64   `json:"max"`
}

// ResultPembahasanItem is one objective-question row of the score_pembahasan pembahasan
// list (FR-S5-23). Essay pembahasan is out of scope for Slice 5.
type ResultPembahasanItem struct {
	QuestionID    uuid.UUID  `json:"question_id"`
	TestID        *uuid.UUID `json:"test_id,omitempty"`
	TestTitle     *string    `json:"test_title,omitempty"`
	Body          string     `json:"body"`
	Format        string     `json:"format"`
	YourAnswer    *string    `json:"your_answer"`
	CorrectAnswer *string    `json:"correct_answer"`
	IsCorrect     *bool      `json:"is_correct"`
	Explanation   *string    `json:"explanation"`
}

// GradingSessionItem is one row of the admin grading queue (FR-S5-16): a submitted
// session that still has at least one ungraded essay answer.
type GradingSessionItem struct {
	SessionID          uuid.UUID  `json:"session_id"`
	StudentID          uuid.UUID  `json:"student_id"`
	StudentName        string     `json:"student_name"`
	SubmittedAt        *time.Time `json:"submitted_at"`
	UngradedEssayCount int        `json:"ungraded_essay_count"`
}

// GradingEssayItem is one essay answer row of the per-session grading read (FR-S5-17).
type GradingEssayItem struct {
	QuestionID    uuid.UUID  `json:"question_id"`
	Body          string     `json:"body"`
	Answer        *string    `json:"answer"`
	PointCorrect  float64    `json:"point_correct"`
	Score         *float64   `json:"score"`
	GraderComment *string    `json:"grader_comment"`
	GradedAt      *time.Time `json:"graded_at"`
}

// ExamLeaderboardEntry is one row of the exam leaderboard — rank, student, score.
// SessionID identifies the row (a student can hold several sessions when retakes are allowed).
type ExamLeaderboardEntry struct {
	Rank        int       `json:"rank"`
	SessionID   uuid.UUID `json:"session_id"`
	StudentID   uuid.UUID `json:"student_id"`
	StudentName string    `json:"student_name"`
	Score       float64   `json:"score"`
}

// AdminResultRow is one row of the school-scoped results list (FR-SCHOOL-08-07).
// SessionID is the opaque identifier for drill-down to the detail endpoint.
// SchoolName is resolved from the student's linked school, falling back to
// their free-text unlisted_school_name when school_id is NULL.
type AdminResultRow struct {
	SessionID   uuid.UUID  `json:"session_id"`
	StudentName string     `json:"student_name"`
	SchoolName  *string    `json:"school_name"`
	Username    *string    `json:"username"`
	Score       *float64   `json:"score"`
	SubmittedAt *time.Time `json:"submitted_at"`
	Violations  int        `json:"violations"`
}

// AdminExportRow is the export-specific row shape. Latest export emits one row
// per registration with latest-scored attempt; all-attempt export emits one row
// per scored attempt, including per-question answers and points.
type AdminExportRow struct {
	RegistrationID uuid.UUID                `json:"registration_id"`
	SessionID      uuid.UUID                `json:"session_id"`
	StudentName    string                   `json:"student_name"`
	Username       *string                  `json:"username"`
	SchoolName     *string                  `json:"school_name"`
	AttemptNumber  int                      `json:"attempt_number"`
	AttemptStatus  string                   `json:"attempt_status"`
	Rank           int                      `json:"rank"`
	Score          *float64                 `json:"score"`
	CorrectCount   int                      `json:"correct_count"`
	WrongCount     int                      `json:"wrong_count"`
	EmptyCount     int                      `json:"empty_count"`
	Violations     int                      `json:"violations"`
	SubmittedAt    *time.Time               `json:"submitted_at"`
	StartedAt      *time.Time               `json:"started_at"`
	QuestionRows   []AdminExportQuestionRow `json:"question_rows"`
}

// AdminExportQuestionRow is one question's answer detail in an export row.
type AdminExportQuestionRow struct {
	QuestionID    uuid.UUID `json:"question_id"`
	QuestionNum   int       `json:"question_num"`
	Format        string    `json:"format"`
	StudentAnswer *string   `json:"student_answer"`
	Points        *float64  `json:"points"`
	IsCorrect     *bool     `json:"is_correct"`
}

// AdminResultSession is the detail read shape for a school-scoped session result
// (FR-SCHOOL-08-13/14/15). It carries the fields resultGate and isFullyGraded need
// (status, score, etc.) plus the joined student name/nis, without a rank field.
type AdminResultSession struct {
	SessionID   uuid.UUID  `json:"session_id"`
	ExamID      uuid.UUID  `json:"exam_id"`
	StudentID   uuid.UUID  `json:"student_id"`
	StudentName string     `json:"student_name"`
	Username    *string    `json:"username"`
	Status      string     `json:"status"`
	Score       *float64   `json:"score"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

// AdminResultDetail is the detail response for a school-scoped session result
// (FR-SCHOOL-08-13/14/15/16). It does NOT embed SessionResult (which carries
// a non-omitempty Rank field). No rank, no certificate_url.
type AdminResultDetail struct {
	SessionID    uuid.UUID              `json:"session_id"`
	StudentName  string                 `json:"student_name"`
	Username     *string                `json:"username"`
	Score        float64                `json:"score"`
	SubmittedAt  *time.Time             `json:"submitted_at"`
	ResultConfig string                 `json:"result_config"`
	CorrectCount int                    `json:"correct_count"`
	WrongCount   int                    `json:"wrong_count"`
	EmptyCount   int                    `json:"empty_count"`
	Breakdown    []ResultTopicRow       `json:"breakdown,omitempty"`
	Pembahasan   []ResultPembahasanItem `json:"pembahasan,omitempty"`
}

// ScoreBucket is one band of the exam analytics score distribution.
type ScoreBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ExamAnalytics is the read shape for GET /admin/exams/:id/analytics.
type ExamAnalytics struct {
	AverageScore   float64       `json:"average_score"`
	CompletionRate float64       `json:"completion_rate"`
	Distribution   []ScoreBucket `json:"distribution"`
}

// SessionMonitorRow is one registrant row in the session monitor dashboard.
// Status is populated by the service layer, not by the repo -- defaults to empty.
// The Active* fields are populated only for sectioned (utbk|ielts) sessions where a
// section is currently active; all are nil for standard-mode sessions.
type SessionMonitorRow struct {
	RegistrationID                uuid.UUID  `json:"registration_id"`
	StudentID                     uuid.UUID  `json:"student_id"`
	StudentName                   string     `json:"student_name"`
	SchoolName                    *string    `json:"school_name"`
	SessionID                     *uuid.UUID `json:"session_id"`
	SessionStatus                 *string    `json:"session_status"`
	StartedAt                     *time.Time `json:"started_at"`
	ExtendedUntil                 *time.Time `json:"extended_until"`
	AdminSubmitted                bool       `json:"admin_submitted"`
	CheckedInAt                   *time.Time `json:"checked_in_at"`
	LastSavedAt                   *time.Time `json:"last_saved_at"`
	AnswersSaved                  int        `json:"answers_saved"`
	TotalQuestions                int        `json:"total_questions"`
	ViolationCount                int        `json:"violation_count"`
	Status                        string     `json:"status"`
	ActiveSectionTestID           *uuid.UUID `json:"active_section_test_id,omitempty"`
	ActiveSectionTitle            *string    `json:"active_section_title,omitempty"`
	ActiveSectionStartedAt        *time.Time `json:"active_section_started_at,omitempty"`
	ActiveSectionDurationMinutes  *int       `json:"active_section_duration_minutes,omitempty"`
	ActiveSectionExtendedUntil    *time.Time `json:"active_section_extended_until,omitempty"`
	ActiveSectionRemainingSeconds int64      `json:"active_section_remaining_seconds,omitempty"`
}

// ExamSessionSection is one per-section timing row for a sectioned (utbk|ielts) exam
// session (FR-3). (session_id, test_id) is the composite PK; sort_order and
// duration_minutes are snapshots taken at session start. status is pending|active|submitted.
type ExamSessionSection struct {
	SessionID       uuid.UUID  `json:"session_id"`
	TestID          uuid.UUID  `json:"test_id"`
	SortOrder       int        `json:"sort_order"`
	DurationMinutes int        `json:"duration_minutes"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
	ExtendedUntil   *time.Time `json:"extended_until,omitempty"`
}

// SessionMonitorExam is the exam summary block in the monitor response.
type SessionMonitorExam struct {
	ID                 uuid.UUID  `json:"id"`
	Title              string     `json:"title"`
	ScheduledAt        *time.Time `json:"scheduled_at"`
	DurationMinutes    *int       `json:"duration_minutes"`
	GraceWindowMinutes *int       `json:"grace_window_minutes"`
	Status             string     `json:"status"`
}

// ViolationRecent is a per-session aggregate row in the recent-violations sidebar.
type ViolationRecent struct {
	SessionID        uuid.UUID `json:"session_id"`
	StudentName      string    `json:"student_name"`
	Count            int       `json:"count"`
	LatestType       string    `json:"latest_type"`
	LatestOccurredAt time.Time `json:"latest_occurred_at"`
}

// SessionMonitorResponse is the top-level response for the session monitor endpoint.
type SessionMonitorResponse struct {
	Exam             SessionMonitorExam  `json:"exam"`
	Rows             []SessionMonitorRow `json:"rows"`
	ViolationsRecent []ViolationRecent   `json:"violations_recent"`
}

// ExamMonitorCandidate is the scheduling-relevant subset of an Exam used to decide
// whether it belongs on the Session Monitor's available-exams list. Exams are
// granted directly (no Product) as often as through a purchase, so availability
// here is judged by the exam's own schedule, not by a linked product's status.
type ExamMonitorCandidate struct {
	ID                   uuid.UUID
	Title                string
	ScheduledAt          *time.Time
	ScheduledEndAt       *time.Time
	CheckInWindowMinutes *int
	// DurationMinutes is nil for timer_mode=per_test exams (UTBK/IELTS), which
	// have no single exam-level clock — SectionsDurationMinutes stands in for it.
	DurationMinutes         *int
	GraceWindowMinutes      *int
	SectionsDurationMinutes int
}

// ExamMonitorAvailable is one row of GET /admin/sessions/monitor/available: an
// exam currently inside its scheduled window (or recently ended), with live
// registration counts for the picker list.
type ExamMonitorAvailable struct {
	ID              uuid.UUID  `json:"id"`
	Title           string     `json:"title"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	ScheduledEndAt  *time.Time `json:"scheduled_end_at"`
	State           string     `json:"state"` // "live" | "ended"
	TotalRegistered int        `json:"total_registered"`
	ActiveCount     int        `json:"active_count"`
	NotStartedCount int        `json:"not_started_count"`
}

// ResultsWorkspaceSummary is the aggregate card block of GET /admin/exams/:id/results-workspace
// (Issue 124). CompletedParticipants counts latest-attempt submitted registrations
// regardless of grading; AverageScore/Distribution cover only the latest-attempt,
// submitted, fully-graded, scored cohort. ViolationAttempts/ViolationEvents count
// across every attempt for the filtered registrations, not just the latest.
type ResultsWorkspaceSummary struct {
	TotalRegistered       int           `json:"total_registered"`
	CompletedParticipants int           `json:"completed_participants"`
	CompletionRate        float64       `json:"completion_rate"`
	AverageScore          float64       `json:"average_score"`
	MaxPossibleScore      float64       `json:"max_possible_score"`
	Distribution          []ScoreBucket `json:"distribution"`
	ViolationAttempts     int           `json:"violation_attempts"`
	ViolationEvents       int           `json:"violation_events"`
}

// ResultsWorkspaceRow is one ranked result row of GET /admin/exams/:id/results-workspace — one
// exam_registration with a submitted, fully graded, scored attempt. The Issue 124
// workspace is intentionally a Hasil/leaderboard view, not a full roster/status
// table; registrations without result rows are summarized but not listed.
type ResultsWorkspaceRow struct {
	RegistrationID      uuid.UUID  `json:"registration_id"`
	StudentID           uuid.UUID  `json:"student_id"`
	StudentName         string     `json:"student_name"`
	Username            *string    `json:"username"`
	SchoolID            *uuid.UUID `json:"school_id"`
	SchoolName          *string    `json:"school_name"`
	Rank                *int       `json:"rank"`
	Score               *float64   `json:"score"`
	AttemptsCount       int        `json:"attempts_count"`
	LatestSessionID     *uuid.UUID `json:"latest_session_id"`
	LatestAttemptNumber *int       `json:"latest_attempt_number"`
	LatestSubmittedAt   *time.Time `json:"latest_submitted_at"`
	LatestViolations    int        `json:"latest_violations"`
}

// ResultsWorkspaceResponse is the top-level response for GET /admin/exams/:id/results-workspace.
type ResultsWorkspaceResponse struct {
	Summary    ResultsWorkspaceSummary `json:"summary"`
	Data       []ResultsWorkspaceRow   `json:"data"`
	NextCursor string                  `json:"next_cursor"`
}

// ResultsWorkspaceAttempt is one row of GET /admin/exams/:id/results-workspace/:registration_id/attempts,
// newest-first. IsLatest marks exactly the same session ListResultsWorkspaceRows treats as
// authoritative for that registration. Score/Violations are raw per-attempt facts,
// not gated by grading completeness (unlike ResultsWorkspaceRow.Score).
type ResultsWorkspaceAttempt struct {
	SessionID       uuid.UUID  `json:"session_id"`
	AttemptNumber   int        `json:"attempt_number"`
	Status          string     `json:"status"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	Score           *float64   `json:"score"`
	Violations      int        `json:"violations"`
	ResultAvailable bool       `json:"result_available"`
	IsLatest        bool       `json:"is_latest"`
}

type QuestionBundleOwner struct {
	ObjectKey   *string
	GeneratedAt *time.Time
	Revision    int64
}

type QuestionBundleState struct {
	TestID      uuid.UUID  `json:"test_id"`
	Variant     string     `json:"variant"`
	Status      string     `json:"status"`
	GeneratedAt *time.Time `json:"generated_at,omitempty"`
}
