package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// Compile-time check: *Repository must implement all exam methods declared by Task 4.
var _ interface {
	CreateTest(context.Context, *model.Test) error
	GetTestByID(context.Context, uuid.UUID) (*model.Test, error)
	GetTestDetail(context.Context, uuid.UUID) (*model.TestDetail, error)
	ListTests(context.Context, TestFilter) ([]model.Test, string, error)
	UpdateTest(context.Context, uuid.UUID, *model.Test) error
	DeleteTest(context.Context, uuid.UUID) error
	ListQuestions(context.Context, uuid.UUID) ([]model.QuestionWithOptions, error)
	CreateQuestionTx(context.Context, pgx.Tx, *model.Question, []model.QuestionOption, []model.QuestionBlank) error
	UpdateQuestionTx(context.Context, pgx.Tx, *model.Question, []model.QuestionOption, []model.QuestionBlank) error
	DeleteQuestion(context.Context, uuid.UUID) error
	CountQuestionsByIDs(context.Context, []uuid.UUID) (int, error)
	ListAttachedQuestionIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	GetMaxSortOrderForTestTx(context.Context, pgx.Tx, uuid.UUID) (int, error)
	AttachQuestionsToTestTx(context.Context, pgx.Tx, uuid.UUID, []uuid.UUID) error
	DetachQuestionFromTest(context.Context, uuid.UUID, uuid.UUID) error
	ReorderTestQuestionsTx(context.Context, pgx.Tx, uuid.UUID, []uuid.UUID) error
} = (*Repository)(nil)

// Compile-time check: *Repository must implement all exam repository methods added
// by Task 2 of Slice 2.
var _ interface {
	CreateExam(context.Context, *model.Exam) error
	GetExamByID(context.Context, uuid.UUID) (*model.Exam, error)
	ListExams(context.Context, ExamFilter) ([]model.ExamListItem, string, error)
	GetExamDetail(context.Context, uuid.UUID) (*model.ExamDetail, error)
	UpdateExam(context.Context, uuid.UUID, *model.Exam) error
	ReplaceExamTestsTx(context.Context, pgx.Tx, uuid.UUID, []model.ExamTest) error
} = (*Repository)(nil)

// Compile-time check: *Repository must implement all registration repository
// methods added by Task 2 of Slice 3.
var _ interface {
	GetExamsByProductID(context.Context, uuid.UUID) ([]model.Exam, error)
	CreateExamRegistration(context.Context, pgx.Tx, model.ExamRegistration) error
	StampOrderItemFulfilledAt(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error
	GetExamRegistrationsByStudent(context.Context, uuid.UUID) ([]model.RegistrationListItem, error)
	GetExamRegistrationByID(context.Context, uuid.UUID, uuid.UUID) (*model.RegistrationDetail, error)
} = (*Repository)(nil)

// Compile-time check: *Repository must implement all session repository
// methods added by Task 3.
var _ interface {
	GetExamRegistrationByToken(context.Context, uuid.UUID, string) (*model.ExamRegistration, error)
	CheckInExamTx(context.Context, pgx.Tx, uuid.UUID) error
	CreateExamSessionTx(context.Context, pgx.Tx, model.ExamRegistration) (model.ExamSession, error)
	GetExamSessionForStudent(context.Context, uuid.UUID, uuid.UUID) (*model.ExamSession, error)
	GetSessionWithQuestions(context.Context, uuid.UUID) ([]model.TestDetail, error)
	GetSessionAnswers(context.Context, uuid.UUID) ([]model.ExamSessionAnswer, error)
	SaveAnswersTx(context.Context, uuid.UUID, []model.ExamSessionAnswer) error
	SubmitSessionTx(context.Context, pgx.Tx, uuid.UUID, []model.ExamSessionAnswer, float64, bool) (int64, error)
	LogViolation(context.Context, model.SessionViolationLog) error
	ReopenSession(context.Context, uuid.UUID, int) error
	GetExamForSession(context.Context, uuid.UUID) (*model.Exam, error)
} = (*Repository)(nil)

// Compile-time check: *Repository must implement all grading/rank/result
// repository methods added by Slice 5 Task 3.
var _ interface {
	ListSessionsNeedingGrading(context.Context, uuid.UUID) ([]model.GradingSessionItem, error)
	GetSessionEssayAnswers(context.Context, uuid.UUID) ([]model.GradingEssayItem, error)
	CountHigherScores(context.Context, uuid.UUID, float64) (int, error)
	CountFullyGradedSessions(context.Context, uuid.UUID) (int, error)
	GradeEssayAnswerTx(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, float64, *string, uuid.UUID) error
} = (*Repository)(nil)

// Compile-time check: *Repository must implement all session monitor repository
// methods added by Slice 7 Task 1.
var _ interface {
	GetSessionMonitorRows(context.Context, uuid.UUID) ([]model.SessionMonitorRow, error)
	GetExamQuestionTotal(context.Context, uuid.UUID) (int, error)
	GetRecentViolations(context.Context, uuid.UUID, int) ([]model.ViolationRecent, error)
	ListSessionViolations(context.Context, uuid.UUID) ([]model.SessionViolationLog, error)
} = (*Repository)(nil)

// Sentinel error from the package: uq_question_order SQLSTATE 23505 — surfaced for service-layer mapping.
var _ error = ErrSortOrderConflict

func TestErrSortOrderConflict_isExported(t *testing.T) {
	if ErrSortOrderConflict == nil {
		t.Fatal("ErrSortOrderConflict must be a non-nil sentinel")
	}
	if ErrSortOrderConflict.Error() == "" {
		t.Error("ErrSortOrderConflict must have a non-empty message")
	}
}

func TestErrSortOrderConflict_isDistinctFromErrNotFound(t *testing.T) {
	if errors.Is(ErrSortOrderConflict, ErrNotFound) {
		t.Error("ErrSortOrderConflict must NOT be wrapped by/equal to ErrNotFound")
	}
}

func TestTestFilterShape(t *testing.T) {
	f := TestFilter{
		Subject: "math",
		Topic:   "algebra",
		Cursor:  uuid.NewString(),
		Limit:   10,
	}
	if f.Subject != "math" || f.Topic != "algebra" || f.Limit != 10 {
		t.Errorf("TestFilter fields not round-tripping: %+v", f)
	}
}

func TestTestDetailShape(t *testing.T) {
	now := time.Now()
	td := model.TestDetail{
		Test: model.Test{
			ID:              uuid.New(),
			Title:           "Sample",
			Subject:         "math",
			Topic:           "algebra",
			DurationMinutes: 60,
			CreatedAt:       now,
		},
		Questions: []model.QuestionWithOptions{
			{
				Question: model.Question{
					ID:     uuid.New(),
					Format: "mcq",
					Body:   "2+2",
				},
				SortOrder: 1,
				Options: []model.QuestionOption{
					{QuestionID: uuid.New(), Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
				},
			},
		},
	}
	if len(td.Questions) != 1 || len(td.Questions[0].Options) != 1 {
		t.Errorf("TestDetail not assembling as expected: %+v", td)
	}
}

type recordingScanner struct {
	dests []any
	err   error
}

func (r *recordingScanner) Scan(dest ...any) error {
	r.dests = append(r.dests, dest...)
	return r.err
}

func TestScanTest_passes_expected_destinations(t *testing.T) {
	var t0 model.Test
	t0.ID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanTest(rec, &t0); err != nil {
		t.Fatalf("scanTest returned error: %v", err)
	}

	if got := len(rec.dests); got != 9 {
		t.Fatalf("scanTest passed %d destinations, want 9 (id, title, subject, topic, duration_minutes, audio_url, audio_play_limit, section_type, created_at)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*string); !ok {
		t.Errorf("dest[1] = %T, want *string (title)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*string); !ok {
		t.Errorf("dest[2] = %T, want *string (subject)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(*string); !ok {
		t.Errorf("dest[3] = %T, want *string (topic)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(*int); !ok {
		t.Errorf("dest[4] = %T, want *int (duration_minutes)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(**string); !ok {
		t.Errorf("dest[5] = %T, want **string (audio_url, nullable local scanned into struct)", rec.dests[5])
	}
	if _, ok := rec.dests[6].(**int); !ok {
		t.Errorf("dest[6] = %T, want **int (audio_play_limit, nullable local scanned into struct)", rec.dests[6])
	}
	if _, ok := rec.dests[7].(**string); !ok {
		t.Errorf("dest[7] = %T, want **string (section_type, nullable pointer scanned into struct)", rec.dests[7])
	}
	if _, ok := rec.dests[8].(*time.Time); !ok {
		t.Errorf("dest[8] = %T, want *time.Time (created_at)", rec.dests[8])
	}
}

func TestScanQuestion_passes_expected_destinations(t *testing.T) {
	var q model.Question
	q.ID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanQuestion(rec, &q); err != nil {
		t.Fatalf("scanQuestion returned error: %v", err)
	}

	if got := len(rec.dests); got != 10 {
		t.Fatalf("scanQuestion passed %d destinations, want 10 (id, format, body, correct_answer, explanation, difficulty, image_url, topic_id, point_correct, point_wrong)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*string); !ok {
		t.Errorf("dest[1] = %T, want *string (format)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*string); !ok {
		t.Errorf("dest[2] = %T, want *string (body)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(**string); !ok {
		t.Errorf("dest[3] = %T, want **string (correct_answer, nullable local)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(**string); !ok {
		t.Errorf("dest[4] = %T, want **string (explanation, nullable local)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(**string); !ok {
		t.Errorf("dest[5] = %T, want **string (difficulty, nullable local)", rec.dests[5])
	}
	if _, ok := rec.dests[6].(**string); !ok {
		t.Errorf("dest[6] = %T, want **string (image_url, nullable local)", rec.dests[6])
	}
	if _, ok := rec.dests[7].(**uuid.UUID); !ok {
		t.Errorf("dest[7] = %T, want **uuid.UUID (topic_id, nullable local)", rec.dests[7])
	}
	if _, ok := rec.dests[8].(*int); !ok {
		t.Errorf("dest[8] = %T, want *int (point_correct)", rec.dests[8])
	}
	if _, ok := rec.dests[9].(*int); !ok {
		t.Errorf("dest[9] = %T, want *int (point_wrong)", rec.dests[9])
	}
}

func TestScanQuestionOption_passes_expected_destinations(t *testing.T) {
	var o model.QuestionOption
	o.QuestionID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanQuestionOption(rec, &o); err != nil {
		t.Fatalf("scanQuestionOption returned error: %v", err)
	}

	if got := len(rec.dests); got != 6 {
		t.Fatalf("scanQuestionOption passed %d destinations, want 6 (question_id, key, text, image_url, is_correct, sort_order)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (question_id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*string); !ok {
		t.Errorf("dest[1] = %T, want *string (key)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*string); !ok {
		t.Errorf("dest[2] = %T, want *string (text)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(**string); !ok {
		t.Errorf("dest[3] = %T, want **string (image_url, nullable local)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(*bool); !ok {
		t.Errorf("dest[4] = %T, want *bool (is_correct)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(*int); !ok {
		t.Errorf("dest[5] = %T, want *int (sort_order)", rec.dests[5])
	}
}

func TestExamFilterShape(t *testing.T) {
	f := ExamFilter{
		Cursor: uuid.NewString(),
		Limit:  15,
	}
	if f.Cursor == "" || f.Limit != 15 {
		t.Errorf("ExamFilter fields not round-tripping: %+v", f)
	}
}

func TestScanExam_passes_expected_destinations(t *testing.T) {
	var e model.Exam
	e.ID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanExam(rec, &e); err != nil {
		t.Fatalf("scanExam returned error: %v", err)
	}

	if got := len(rec.dests); got != 24 {
		t.Fatalf("scanExam passed %d destinations, want 24", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*string); !ok {
		t.Errorf("dest[1] = %T, want *string (title)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*bool); !ok {
		t.Errorf("dest[2] = %T, want *bool (is_free)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(**time.Time); !ok {
		t.Errorf("dest[3] = %T, want **time.Time (scheduled_at, nullable pointer field)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(**time.Time); !ok {
		t.Errorf("dest[4] = %T, want **time.Time (scheduled_end_at, nullable pointer field)", rec.dests[4])
	}
	if _, ok := rec.dests[8].(**string); !ok {
		t.Errorf("dest[8] = %T, want **string (bundle_url, nullable pointer field)", rec.dests[8])
	}
	if _, ok := rec.dests[14].(**int); !ok {
		t.Errorf("dest[14] = %T, want **int (duration_minutes, nullable pointer field)", rec.dests[14])
	}
	if _, ok := rec.dests[18].(*string); !ok {
		t.Errorf("dest[18] = %T, want *string (status, scalar)", rec.dests[18])
	}
	if _, ok := rec.dests[19].(*time.Time); !ok {
		t.Errorf("dest[19] = %T, want *time.Time (created_at)", rec.dests[19])
	}
	if _, ok := rec.dests[20].(*string); !ok {
		t.Errorf("dest[20] = %T, want *string (mode)", rec.dests[20])
	}
	if _, ok := rec.dests[21].(**json.RawMessage); !ok {
		t.Errorf("dest[21] = %T, want **json.RawMessage (certificate_design, nullable pointer field)", rec.dests[21])
	}
	if _, ok := rec.dests[22].(**time.Time); !ok {
		t.Errorf("dest[22] = %T, want **time.Time (certificate_design_updated_at, nullable pointer field)", rec.dests[22])
	}
	if _, ok := rec.dests[23].(**int); !ok {
		t.Errorf("dest[23] = %T, want **int (exam_number, nullable pointer field)", rec.dests[23])
	}
}

func TestRegistrationListItemShape(t *testing.T) {
	scheduled := time.Now()
	item := model.RegistrationListItem{
		ExamRegistration: model.ExamRegistration{
			ID:        uuid.New(),
			StudentID: uuid.New(),
			ExamID:    uuid.New(),
			Token:     "ABCD1234",
			Status:    "registered",
			CreatedAt: time.Now(),
		},
		ExamTitle:   "Tryout Matematika",
		ScheduledAt: &scheduled,
	}
	if item.ID == uuid.Nil || item.Token != "ABCD1234" || item.ExamTitle == "" {
		t.Errorf("RegistrationListItem fields not round-tripping: %+v", item)
	}
	if item.ScheduledAt == nil || !item.ScheduledAt.Equal(scheduled) {
		t.Errorf("RegistrationListItem.ScheduledAt pointer not preserved: %+v", item.ScheduledAt)
	}
}

func TestRegistrationDetailShape(t *testing.T) {
	scheduled := time.Now()
	checkInWindow := 15
	duration := 90
	detail := model.RegistrationDetail{
		ExamRegistration: model.ExamRegistration{
			ID:        uuid.New(),
			StudentID: uuid.New(),
			ExamID:    uuid.New(),
			Token:     "ABCD1234",
			Status:    "registered",
			CreatedAt: time.Now(),
		},
	}
	detail.Exam.ID = uuid.New()
	detail.Exam.Title = "Tryout Matematika"
	detail.Exam.ScheduledAt = &scheduled
	detail.Exam.RequiresCheckin = true
	detail.Exam.CheckInWindowMinutes = &checkInWindow
	detail.Exam.TimerMode = "overall"
	detail.Exam.DurationMinutes = &duration
	detail.Exam.ResultConfig = "hidden"

	if detail.Exam.Title == "" || !detail.Exam.RequiresCheckin {
		t.Errorf("RegistrationDetail.Exam fields not round-tripping: %+v", detail.Exam)
	}
	if detail.Exam.CheckInWindowMinutes == nil || *detail.Exam.CheckInWindowMinutes != 15 {
		t.Errorf("RegistrationDetail.Exam.CheckInWindowMinutes pointer not preserved: %+v", detail.Exam.CheckInWindowMinutes)
	}
}

func TestScanExamSession_passes_expected_destinations(t *testing.T) {
	var s model.ExamSession
	s.ID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanExamSession(rec, &s); err != nil {
		t.Fatalf("scanExamSession returned error: %v", err)
	}

	if got := len(rec.dests); got != 16 {
		t.Fatalf("scanExamSession passed %d destinations, want 16 (id, registration_id, student_id, exam_id, attempt_number, started_at, submitted_at, extended_until, admin_submitted, score, certificate_key, certificate_generated_at, certificate_number, last_saved_at, status, created_at)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*uuid.UUID); !ok {
		t.Errorf("dest[1] = %T, want *uuid.UUID (registration_id)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*uuid.UUID); !ok {
		t.Errorf("dest[2] = %T, want *uuid.UUID (student_id)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(*uuid.UUID); !ok {
		t.Errorf("dest[3] = %T, want *uuid.UUID (exam_id)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(*int); !ok {
		t.Errorf("dest[4] = %T, want *int (attempt_number)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(*time.Time); !ok {
		t.Errorf("dest[5] = %T, want *time.Time (started_at)", rec.dests[5])
	}
	if _, ok := rec.dests[6].(**time.Time); !ok {
		t.Errorf("dest[6] = %T, want **time.Time (submitted_at, nullable)", rec.dests[6])
	}
	if _, ok := rec.dests[7].(**time.Time); !ok {
		t.Errorf("dest[7] = %T, want **time.Time (extended_until, nullable)", rec.dests[7])
	}
	if _, ok := rec.dests[8].(*bool); !ok {
		t.Errorf("dest[8] = %T, want *bool (admin_submitted)", rec.dests[8])
	}
	if _, ok := rec.dests[9].(**float64); !ok {
		t.Errorf("dest[9] = %T, want **float64 (score, nullable)", rec.dests[9])
	}
	if _, ok := rec.dests[10].(**string); !ok {
		t.Errorf("dest[10] = %T, want **string (certificate_key, nullable)", rec.dests[10])
	}
	if _, ok := rec.dests[11].(**time.Time); !ok {
		t.Errorf("dest[11] = %T, want **time.Time (certificate_generated_at, nullable)", rec.dests[11])
	}
	if _, ok := rec.dests[12].(**string); !ok {
		t.Errorf("dest[12] = %T, want **string (certificate_number, nullable)", rec.dests[12])
	}
	if _, ok := rec.dests[13].(**time.Time); !ok {
		t.Errorf("dest[13] = %T, want **time.Time (last_saved_at, nullable)", rec.dests[13])
	}
	if _, ok := rec.dests[14].(*string); !ok {
		t.Errorf("dest[14] = %T, want *string (status)", rec.dests[14])
	}
	if _, ok := rec.dests[15].(*time.Time); !ok {
		t.Errorf("dest[15] = %T, want *time.Time (created_at)", rec.dests[15])
	}
}

func TestScanExamSessionAnswer_passes_expected_destinations(t *testing.T) {
	var a model.ExamSessionAnswer
	a.SessionID = uuid.Nil
	a.QuestionID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanExamSessionAnswer(rec, &a); err != nil {
		t.Fatalf("scanExamSessionAnswer returned error: %v", err)
	}

	if got := len(rec.dests); got != 10 {
		t.Fatalf("scanExamSessionAnswer passed %d destinations, want 10 (session_id, question_id, answer, is_correct, score, graded_by, graded_at, grader_comment, flagged_for_review, saved_at)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (session_id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*uuid.UUID); !ok {
		t.Errorf("dest[1] = %T, want *uuid.UUID (question_id)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(**string); !ok {
		t.Errorf("dest[2] = %T, want **string (answer, nullable)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(**bool); !ok {
		t.Errorf("dest[3] = %T, want **bool (is_correct, nullable)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(**float64); !ok {
		t.Errorf("dest[4] = %T, want **float64 (score, nullable)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(**uuid.UUID); !ok {
		t.Errorf("dest[5] = %T, want **uuid.UUID (graded_by, nullable)", rec.dests[5])
	}
	if _, ok := rec.dests[6].(**time.Time); !ok {
		t.Errorf("dest[6] = %T, want **time.Time (graded_at, nullable)", rec.dests[6])
	}
	if _, ok := rec.dests[7].(**string); !ok {
		t.Errorf("dest[7] = %T, want **string (grader_comment, nullable)", rec.dests[7])
	}
	if _, ok := rec.dests[8].(*bool); !ok {
		t.Errorf("dest[8] = %T, want *bool (flagged_for_review)", rec.dests[8])
	}
	if _, ok := rec.dests[9].(*time.Time); !ok {
			t.Errorf("dest[9] = %T, want *time.Time (saved_at)", rec.dests[9])
		}
	}

func TestScanSessionMonitorRow_passes_expected_destinations(t *testing.T) {
	var row model.SessionMonitorRow
	row.RegistrationID = uuid.Nil

	rec := &recordingScanner{}
	if err := scanSessionMonitorRow(rec, &row); err != nil {
		t.Fatalf("scanSessionMonitorRow returned error: %v", err)
	}

	if got := len(rec.dests); got != 18 {
		t.Fatalf("scanSessionMonitorRow passed %d destinations, want 18 (registration_id, student_id, student_name, school_name, session_id, session_status, started_at, extended_until, admin_submitted, checked_in_at, last_saved_at, answers_saved, violation_count, active_section_test_id, active_section_title, active_section_started_at, active_section_duration_minutes, active_section_extended_until)", got)
	}

	if _, ok := rec.dests[0].(*uuid.UUID); !ok {
		t.Errorf("dest[0] = %T, want *uuid.UUID (registration_id)", rec.dests[0])
	}
	if _, ok := rec.dests[1].(*uuid.UUID); !ok {
		t.Errorf("dest[1] = %T, want *uuid.UUID (student_id)", rec.dests[1])
	}
	if _, ok := rec.dests[2].(*string); !ok {
		t.Errorf("dest[2] = %T, want *string (student_name)", rec.dests[2])
	}
	if _, ok := rec.dests[3].(**string); !ok {
		t.Errorf("dest[3] = %T, want **string (school_name, nullable)", rec.dests[3])
	}
	if _, ok := rec.dests[4].(**uuid.UUID); !ok {
		t.Errorf("dest[4] = %T, want **uuid.UUID (session_id, nullable)", rec.dests[4])
	}
	if _, ok := rec.dests[5].(**string); !ok {
		t.Errorf("dest[5] = %T, want **string (session_status, nullable)", rec.dests[5])
	}
	if _, ok := rec.dests[6].(**time.Time); !ok {
		t.Errorf("dest[6] = %T, want **time.Time (started_at, nullable)", rec.dests[6])
	}
	if _, ok := rec.dests[7].(**time.Time); !ok {
		t.Errorf("dest[7] = %T, want **time.Time (extended_until, nullable)", rec.dests[7])
	}
	if _, ok := rec.dests[8].(*bool); !ok {
		t.Errorf("dest[8] = %T, want *bool (admin_submitted)", rec.dests[8])
	}
	if _, ok := rec.dests[9].(**time.Time); !ok {
		t.Errorf("dest[9] = %T, want **time.Time (checked_in_at, nullable)", rec.dests[9])
	}
	if _, ok := rec.dests[10].(**time.Time); !ok {
		t.Errorf("dest[10] = %T, want **time.Time (last_saved_at, nullable)", rec.dests[10])
	}
	if _, ok := rec.dests[11].(*int); !ok {
		t.Errorf("dest[11] = %T, want *int (answers_saved)", rec.dests[11])
	}
	if _, ok := rec.dests[12].(*int); !ok {
		t.Errorf("dest[12] = %T, want *int (violation_count)", rec.dests[12])
	}
	if _, ok := rec.dests[13].(**uuid.UUID); !ok {
		t.Errorf("dest[13] = %T, want **uuid.UUID (active_section_test_id, nullable)", rec.dests[13])
	}
	if _, ok := rec.dests[14].(**string); !ok {
		t.Errorf("dest[14] = %T, want **string (active_section_title, nullable)", rec.dests[14])
	}
	if _, ok := rec.dests[15].(**time.Time); !ok {
		t.Errorf("dest[15] = %T, want **time.Time (active_section_started_at, nullable)", rec.dests[15])
	}
	if _, ok := rec.dests[16].(**int); !ok {
		t.Errorf("dest[16] = %T, want **int (active_section_duration_minutes, nullable)", rec.dests[16])
	}
	if _, ok := rec.dests[17].(**time.Time); !ok {
		t.Errorf("dest[17] = %T, want **time.Time (active_section_extended_until, nullable)", rec.dests[17])
	}
}

func TestSessionMonitorRowShape(t *testing.T) {
	now := time.Now()
	uid := uuid.New()
	schoolName := "SMA 1"
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID:  uuid.New(),
		StudentID:       uuid.New(),
		StudentName:     "Budi",
		SchoolName:      &schoolName,
		SessionID:       &uid,
		SessionStatus:   &sessionStatus,
		StartedAt:       &now,
		AdminSubmitted:  false,
		CheckedInAt:     &now,
		LastSavedAt:     &now,
		AnswersSaved:    5,
		ViolationCount:  0,
	}
	if row.RegistrationID == uuid.Nil || row.StudentName != "Budi" {
		t.Errorf("SessionMonitorRow fields not round-tripping: %+v", row)
	}
	if row.SchoolName == nil || *row.SchoolName != "SMA 1" {
		t.Error("SessionMonitorRow.SchoolName pointer not preserved")
	}
	if row.SessionStatus == nil || *row.SessionStatus != "in_progress" {
		t.Error("SessionMonitorRow.SessionStatus pointer not preserved")
	}
}

func TestSessionMonitorExamShape(t *testing.T) {
	now := time.Now()
	dur := 120
	gw := 5
	e := model.SessionMonitorExam{
		ID:                 uuid.New(),
		Title:              "Tryout",
		ScheduledAt:        &now,
		DurationMinutes:    &dur,
		GraceWindowMinutes: &gw,
		Status:             "published",
	}
	if e.ID == uuid.Nil || e.Title != "Tryout" {
		t.Errorf("SessionMonitorExam fields not round-tripping: %+v", e)
	}
	if e.DurationMinutes == nil || *e.DurationMinutes != 120 {
		t.Error("SessionMonitorExam.DurationMinutes pointer not preserved")
	}
	if e.GraceWindowMinutes == nil || *e.GraceWindowMinutes != 5 {
		t.Error("SessionMonitorExam.GraceWindowMinutes pointer not preserved")
	}
}

func TestViolationRecentShape(t *testing.T) {
	v := model.ViolationRecent{
		SessionID:        uuid.New(),
		StudentName:      "Budi",
		Count:            3,
		LatestType:       "tab_switch",
		LatestOccurredAt: time.Now(),
	}
	if v.SessionID == uuid.Nil || v.StudentName != "Budi" || v.LatestType == "" {
		t.Errorf("ViolationRecent fields not round-tripping: %+v", v)
	}
}

func TestSessionMonitorResponseShape(t *testing.T) {
	resp := model.SessionMonitorResponse{
		Exam: model.SessionMonitorExam{ID: uuid.New(), Title: "Tryout"},
		Rows: []model.SessionMonitorRow{
			{RegistrationID: uuid.New(), StudentID: uuid.New(), StudentName: "Budi"},
		},
		ViolationsRecent: []model.ViolationRecent{
			{SessionID: uuid.New(), StudentName: "Budi", Count: 1},
		},
	}
	if len(resp.Rows) != 1 || len(resp.ViolationsRecent) != 1 {
		t.Errorf("SessionMonitorResponse not assembling: %+v", resp)
	}
}