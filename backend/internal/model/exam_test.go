package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Shared helpers
func mustField(t *testing.T, v reflect.Value, name string) reflect.StructField {
	t.Helper()
	f, ok := v.Type().FieldByName(name)
	if !ok {
		t.Fatalf("struct %s missing field %q", v.Type().Name(), name)
	}
	return f
}

func jsonTag(t *testing.T, v reflect.Value, name, want string) {
	t.Helper()
	f := mustField(t, v, name)
	got := f.Tag.Get("json")
	if got != want {
		t.Errorf("%s.%s json tag: got %q, want %q", v.Type().Name(), name, got, want)
	}
}

func fieldType(t *testing.T, v reflect.Value, name string, want reflect.Type) {
	t.Helper()
	f := mustField(t, v, name)
	if f.Type != want {
		t.Errorf("%s.%s type: got %s, want %s", v.Type().Name(), name, f.Type, want)
	}
}

func fieldKind(t *testing.T, v reflect.Value, name string, want reflect.Kind) {
	t.Helper()
	f := mustField(t, v, name)
	if f.Type.Kind() != want {
		t.Errorf("%s.%s kind: got %s, want %s", v.Type().Name(), name, f.Type.Kind(), want)
	}
}

func newModel(t reflect.Type) reflect.Value {
	v := reflect.New(t).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Ptr:
			// leave nil (zero value), valid for tests
		case reflect.Slice, reflect.Map:
			// leave nil (zero value), valid for tests
		}
	}
	return v
}

// ---- Test ----

func TestTestStruct(t *testing.T) {
	typ := reflect.TypeOf((*Test)(nil)).Elem()
	v := newModel(typ)

	if typ.NumField() != 10 {
		t.Fatalf("Test struct: got %d fields, want 10", typ.NumField())
	}

	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "Title", "title")
	jsonTag(t, v, "Subject", "subject")
	jsonTag(t, v, "Topic", "topic")
	jsonTag(t, v, "DurationMinutes", "duration_minutes")
	jsonTag(t, v, "AudioURL", "audio_url")
	jsonTag(t, v, "AudioPlayLimit", "audio_play_limit")
	jsonTag(t, v, "SectionType", "section_type,omitempty")
	jsonTag(t, v, "QuestionCount", "question_count")
	jsonTag(t, v, "CreatedAt", "created_at")

	fieldKind(t, v, "QuestionCount", reflect.Int)

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "Title", reflect.String)
	fieldKind(t, v, "DurationMinutes", reflect.Int)
	// AudioURL is nullable → pointer
	fieldKind(t, v, "AudioURL", reflect.Ptr)
	if mustField(t, v, "AudioURL").Type.Elem().Kind() != reflect.String {
		t.Errorf("Test.AudioURL pointer base type should be string")
	}
	// AudioPlayLimit is nullable → pointer to int
	fieldKind(t, v, "AudioPlayLimit", reflect.Ptr)
	if mustField(t, v, "AudioPlayLimit").Type.Elem().Kind() != reflect.Int {
		t.Errorf("Test.AudioPlayLimit pointer base type should be int")
	}
	fieldType(t, v, "CreatedAt", reflect.TypeOf(time.Time{}))
}

// ---- Question ----

func TestQuestionStruct(t *testing.T) {
	typ := reflect.TypeOf((*Question)(nil)).Elem()
	v := newModel(typ)

	// Question has NO created_at (migration doesn't define one). 12 fields: id,
	// format, body, correct_answer, explanation, difficulty, image_url, audio_url,
	// topic_id, topic, point_correct, point_wrong.
	if typ.NumField() != 12 {
		t.Fatalf("Question struct: got %d fields, want 12", typ.NumField())
	}
	if _, ok := typ.FieldByName("CreatedAt"); ok {
		t.Errorf("Question must NOT have CreatedAt — migration 0014_exam.up.sql does not define it")
	}

	jsonTag(t, v, "Format", "format")
	jsonTag(t, v, "CorrectAnswer", "correct_answer")
	jsonTag(t, v, "ImageURL", "image_url")
	jsonTag(t, v, "AudioURL", "audio_url")
	jsonTag(t, v, "TopicID", "topic_id")
	jsonTag(t, v, "Topic", "topic")
	jsonTag(t, v, "PointCorrect", "point_correct")
	jsonTag(t, v, "PointWrong", "point_wrong")

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "Format", reflect.String)
	fieldKind(t, v, "Body", reflect.String)
	fieldKind(t, v, "PointCorrect", reflect.Int)
	fieldKind(t, v, "PointWrong", reflect.Int)

	// Nullable pointers
	for _, name := range []string{"CorrectAnswer", "Explanation", "Difficulty", "ImageURL", "Topic"} {
		fieldKind(t, v, name, reflect.Ptr)
		if mustField(t, v, name).Type.Elem().Kind() != reflect.String {
			t.Errorf("Question.%s pointer base type should be string, got %s", name, mustField(t, v, name).Type.Elem().Kind())
		}
	}

	fieldKind(t, v, "TopicID", reflect.Ptr)
	if mustField(t, v, "TopicID").Type.Elem() != reflect.TypeOf(uuid.UUID{}) {
		t.Errorf("Question.TopicID pointer base type should be uuid.UUID, got %s", mustField(t, v, "TopicID").Type.Elem())
	}
}

// ---- QuestionOption ----

func TestQuestionOptionStruct(t *testing.T) {
	typ := reflect.TypeOf((*QuestionOption)(nil)).Elem()
	v := newModel(typ)

	// composite PK: question_id + key (no surrogate UUID primary key)
	if typ.NumField() != 6 {
		t.Fatalf("QuestionOption struct: got %d fields, want 6", typ.NumField())
	}

	jsonTag(t, v, "QuestionID", "question_id")
	jsonTag(t, v, "Key", "key")
	jsonTag(t, v, "Text", "text")
	jsonTag(t, v, "ImageURL", "image_url")
	jsonTag(t, v, "IsCorrect", "is_correct")
	jsonTag(t, v, "SortOrder", "sort_order")

	// Confirm NO UUID field exists with name like "ID"
	if _, ok := typ.FieldByName("ID"); ok {
		t.Errorf("QuestionOption must NOT have a surrogate ID field (composite PK)")
	}

	fieldType(t, v, "QuestionID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "Key", reflect.String)
	fieldKind(t, v, "Text", reflect.String)
	fieldKind(t, v, "IsCorrect", reflect.Bool)
	fieldKind(t, v, "SortOrder", reflect.Int)

	fieldKind(t, v, "ImageURL", reflect.Ptr)
	if mustField(t, v, "ImageURL").Type.Elem().Kind() != reflect.String {
		t.Errorf("QuestionOption.ImageURL pointer base type should be string")
	}

	// No CreatedAt per DBML
	if _, ok := typ.FieldByName("CreatedAt"); ok {
		t.Errorf("QuestionOption must NOT have CreatedAt per DBML")
	}
}

// ---- Exam ----

func TestExamStruct(t *testing.T) {
	typ := reflect.TypeOf((*Exam)(nil)).Elem()
	v := newModel(typ)

	// Check count later — we verify a representative subset of fields and tags.
	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "Title", "title")
	jsonTag(t, v, "IsFree", "is_free")
	jsonTag(t, v, "ScheduledAt", "scheduled_at")
	jsonTag(t, v, "RequiresCheckin", "requires_checkin")
	jsonTag(t, v, "AllowLeaderboard", "allow_leaderboard")
	jsonTag(t, v, "CDNBundle", "cdn_bundle")
	jsonTag(t, v, "BundleURL", "bundle_url")
	jsonTag(t, v, "BundleGeneratedAt", "bundle_generated_at")
	jsonTag(t, v, "CheckInWindowMinutes", "check_in_window_minutes")
	jsonTag(t, v, "GraceWindowMinutes", "grace_window_minutes")
	jsonTag(t, v, "MaxAttempts", "max_attempts")
	jsonTag(t, v, "TimerMode", "timer_mode")
	jsonTag(t, v, "DurationMinutes", "duration_minutes")
	jsonTag(t, v, "Randomize", "randomize")
	jsonTag(t, v, "ResultConfig", "result_config")
	jsonTag(t, v, "ResultReleaseAt", "result_release_at")
	jsonTag(t, v, "Status", "status")
	jsonTag(t, v, "CreatedAt", "created_at")
	jsonTag(t, v, "CertificateDesign", "certificate_design")
	jsonTag(t, v, "CertificateDesignUpdatedAt", "certificate_design_updated_at")

	fieldKind(t, v, "CertificateDesign", reflect.Ptr)
	if mustField(t, v, "CertificateDesign").Type != reflect.TypeOf((*json.RawMessage)(nil)) {
		t.Errorf("Exam.CertificateDesign should be *json.RawMessage, got %s", mustField(t, v, "CertificateDesign").Type)
	}
	fieldKind(t, v, "CertificateDesignUpdatedAt", reflect.Ptr)
	if mustField(t, v, "CertificateDesignUpdatedAt").Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Errorf("Exam.CertificateDesignUpdatedAt should be *time.Time, got %s", mustField(t, v, "CertificateDesignUpdatedAt").Type)
	}

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "IsFree", reflect.Bool)
	fieldKind(t, v, "Title", reflect.String)
	fieldKind(t, v, "TimerMode", reflect.String)
	fieldKind(t, v, "ResultConfig", reflect.String)
	fieldKind(t, v, "Status", reflect.String)
	fieldKind(t, v, "Randomize", reflect.Bool)

	// Nullable pointers
	for _, name := range []string{"ScheduledAt", "BundleGeneratedAt", "ResultReleaseAt"} {
		fieldKind(t, v, name, reflect.Ptr)
		if mustField(t, v, name).Type != reflect.TypeOf((*time.Time)(nil)) {
			t.Errorf("Exam.%s should be *time.Time, got %s", name, mustField(t, v, name).Type)
		}
	}
	for _, name := range []string{"CheckInWindowMinutes", "GraceWindowMinutes", "MaxAttempts", "DurationMinutes"} {
		fieldKind(t, v, name, reflect.Ptr)
		if mustField(t, v, name).Type.Elem().Kind() != reflect.Int {
			t.Errorf("Exam.%s pointer base type should be int, got %s", name, mustField(t, v, name).Type.Elem().Kind())
		}
	}
	fieldKind(t, v, "BundleURL", reflect.Ptr)
	if mustField(t, v, "BundleURL").Type.Elem().Kind() != reflect.String {
		t.Errorf("Exam.BundleURL pointer base type should be string")
	}

	fieldType(t, v, "CreatedAt", reflect.TypeOf(time.Time{}))
}

// ---- ExamTest ----

func TestExamTestStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamTest)(nil)).Elem()
	v := newModel(typ)

	if typ.NumField() != 4 {
		t.Fatalf("ExamTest struct: got %d fields, want 4", typ.NumField())
	}

	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "ExamID", "exam_id")
	jsonTag(t, v, "TestID", "test_id")
	jsonTag(t, v, "SortOrder", "sort_order")

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "ExamID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "TestID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "SortOrder", reflect.Int)
}

// ---- ExamRegistration ----

func TestExamRegistrationStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamRegistration)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "StudentID", "student_id")
	jsonTag(t, v, "ExamID", "exam_id")
	jsonTag(t, v, "Token", "token")
	jsonTag(t, v, "CardKey", "card_key")
	jsonTag(t, v, "CheckedInAt", "checked_in_at")
	jsonTag(t, v, "AttemptsUsed", "attempts_used")
	jsonTag(t, v, "Status", "status")
	jsonTag(t, v, "CreatedAt", "created_at")

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "StudentID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "ExamID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "Token", reflect.String)
	fieldKind(t, v, "AttemptsUsed", reflect.Int)
	fieldKind(t, v, "Status", reflect.String)

	fieldKind(t, v, "CardKey", reflect.Ptr)
	if mustField(t, v, "CardKey").Type.Elem().Kind() != reflect.String {
		t.Errorf("ExamRegistration.CardKey pointer base type should be string")
	}
	fieldKind(t, v, "CheckedInAt", reflect.Ptr)
	if mustField(t, v, "CheckedInAt").Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Errorf("ExamRegistration.CheckedInAt should be *time.Time, got %s", mustField(t, v, "CheckedInAt").Type)
	}
	fieldType(t, v, "CreatedAt", reflect.TypeOf(time.Time{}))
}

// ---- ExamSession ----

func TestExamSessionStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamSession)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "RegistrationID", "registration_id")
	jsonTag(t, v, "StudentID", "student_id")
	jsonTag(t, v, "ExamID", "exam_id")
	jsonTag(t, v, "AttemptNumber", "attempt_number")
	jsonTag(t, v, "StartedAt", "started_at")
	jsonTag(t, v, "SubmittedAt", "submitted_at")
	jsonTag(t, v, "ExtendedUntil", "extended_until")
	jsonTag(t, v, "AdminSubmitted", "admin_submitted")
	jsonTag(t, v, "Score", "score")
	jsonTag(t, v, "CertificateKey", "certificate_key")
	jsonTag(t, v, "CertificateGeneratedAt", "certificate_generated_at")
	jsonTag(t, v, "CertificateNumber", "certificate_number")
	jsonTag(t, v, "LastSavedAt", "last_saved_at")
	jsonTag(t, v, "Status", "status")
	jsonTag(t, v, "CreatedAt", "created_at")

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "RegistrationID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "StudentID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "ExamID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "AttemptNumber", reflect.Int)
	fieldKind(t, v, "AdminSubmitted", reflect.Bool)
	fieldKind(t, v, "Status", reflect.String)

	// started_at not null
	fieldType(t, v, "StartedAt", reflect.TypeOf(time.Time{}))
	fieldType(t, v, "CreatedAt", reflect.TypeOf(time.Time{}))

	// Nullable pointers
	for _, name := range []string{"SubmittedAt", "ExtendedUntil", "CertificateGeneratedAt", "LastSavedAt"} {
		fieldKind(t, v, name, reflect.Ptr)
		if mustField(t, v, name).Type != reflect.TypeOf((*time.Time)(nil)) {
			t.Errorf("ExamSession.%s should be *time.Time, got %s", name, mustField(t, v, name).Type)
		}
	}
	// Score is NUMERIC(6,2) nullable → *float64
	fieldKind(t, v, "Score", reflect.Ptr)
	if mustField(t, v, "Score").Type.Elem().Kind() != reflect.Float64 {
		t.Errorf("ExamSession.Score pointer base type should be float64")
	}
	fieldKind(t, v, "CertificateKey", reflect.Ptr)
	if mustField(t, v, "CertificateKey").Type.Elem().Kind() != reflect.String {
		t.Errorf("ExamSession.CertificateKey pointer base type should be string")
	}
	fieldKind(t, v, "CertificateNumber", reflect.Ptr)
	if mustField(t, v, "CertificateNumber").Type.Elem().Kind() != reflect.String {
		t.Errorf("ExamSession.CertificateNumber pointer base type should be string")
	}
}

// TestExamCertificateDesignJSONRoundTrip proves CertificateDesign serializes as raw
// JSON (not base64, as a plain []byte would) and round-trips through marshal/unmarshal,
// per memory go-json-tags-verification-blindspot: assert on marshalled bytes, not by
// eyeballing the struct.
func TestExamCertificateDesignJSONRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"template":"classic","fields":[{"id":"student_name","x_mm":10,"y_mm":20}]}`)
	e := Exam{CertificateDesign: &raw}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal envelope: %v", err)
	}
	design, ok := decoded["certificate_design"].(map[string]any)
	if !ok {
		t.Fatalf("certificate_design is not a nested JSON object (want raw pass-through, got %T: %v)", decoded["certificate_design"], decoded["certificate_design"])
	}
	if design["template"] != "classic" {
		t.Errorf("certificate_design missing template key: %v", design)
	}

	// nil design marshals as JSON null, never omitted or base64.
	var e2 Exam
	b2, err := json.Marshal(e2)
	if err != nil {
		t.Fatalf("Marshal nil design: %v", err)
	}
	var decoded2 map[string]any
	if err := json.Unmarshal(b2, &decoded2); err != nil {
		t.Fatalf("Unmarshal nil-design envelope: %v", err)
	}
	if v, ok := decoded2["certificate_design"]; !ok || v != nil {
		t.Errorf("nil CertificateDesign should marshal to JSON null, got %v (present=%v)", v, ok)
	}
}

// ---- ExamSessionAnswer ----

func TestExamSessionAnswerStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamSessionAnswer)(nil)).Elem()
	v := newModel(typ)

	// composite PK (session_id, question_id) — no surrogate ID
	if _, ok := typ.FieldByName("ID"); ok {
		t.Errorf("ExamSessionAnswer must NOT have a surrogate ID field (composite PK)")
	}

	jsonTag(t, v, "SessionID", "session_id")
	jsonTag(t, v, "QuestionID", "question_id")
	jsonTag(t, v, "Answer", "answer")
	jsonTag(t, v, "IsCorrect", "is_correct")
	jsonTag(t, v, "Score", "score")
	jsonTag(t, v, "GradedBy", "graded_by")
	jsonTag(t, v, "GradedAt", "graded_at")
	jsonTag(t, v, "GraderComment", "grader_comment")
	jsonTag(t, v, "FlaggedForReview", "flagged_for_review")
	jsonTag(t, v, "SavedAt", "saved_at")

	fieldType(t, v, "SessionID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "QuestionID", reflect.TypeOf(uuid.UUID{}))

	// Nullable pointers
	fieldKind(t, v, "Answer", reflect.Ptr)
	if mustField(t, v, "Answer").Type.Elem().Kind() != reflect.String {
		t.Errorf("ExamSessionAnswer.Answer pointer base type should be string")
	}
	fieldKind(t, v, "IsCorrect", reflect.Ptr)
	if mustField(t, v, "IsCorrect").Type.Elem().Kind() != reflect.Bool {
		t.Errorf("ExamSessionAnswer.IsCorrect pointer base type should be bool")
	}
	fieldKind(t, v, "Score", reflect.Ptr)
	if mustField(t, v, "Score").Type.Elem().Kind() != reflect.Float64 {
		t.Errorf("ExamSessionAnswer.Score pointer base type should be float64 (NUMERIC)")
	}
	fieldKind(t, v, "GradedBy", reflect.Ptr)
	if mustField(t, v, "GradedBy").Type != reflect.TypeOf((*uuid.UUID)(nil)) {
		t.Errorf("ExamSessionAnswer.GradedBy should be *uuid.UUID, got %s", mustField(t, v, "GradedBy").Type)
	}
	fieldKind(t, v, "GradedAt", reflect.Ptr)
	if mustField(t, v, "GradedAt").Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Errorf("ExamSessionAnswer.GradedAt should be *time.Time, got %s", mustField(t, v, "GradedAt").Type)
	}
	fieldKind(t, v, "GraderComment", reflect.Ptr)
	if mustField(t, v, "GraderComment").Type.Elem().Kind() != reflect.String {
		t.Errorf("ExamSessionAnswer.GraderComment pointer base type should be string")
	}
	fieldKind(t, v, "FlaggedForReview", reflect.Bool)
	fieldType(t, v, "SavedAt", reflect.TypeOf(time.Time{}))
}

// ---- SessionViolationLog ----

func TestSessionViolationLogStruct(t *testing.T) {
	typ := reflect.TypeOf((*SessionViolationLog)(nil)).Elem()
	v := newModel(typ)

	if typ.NumField() != 5 {
		t.Fatalf("SessionViolationLog struct: got %d fields, want 5", typ.NumField())
	}

	jsonTag(t, v, "ID", "id")
	jsonTag(t, v, "SessionID", "session_id")
	jsonTag(t, v, "StudentID", "student_id")
	jsonTag(t, v, "ViolationType", "violation_type")
	jsonTag(t, v, "OccurredAt", "occurred_at")

	fieldType(t, v, "ID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "SessionID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "StudentID", reflect.TypeOf(uuid.UUID{}))
	fieldKind(t, v, "ViolationType", reflect.String)
	fieldType(t, v, "OccurredAt", reflect.TypeOf(time.Time{}))
}

// ---- Composite read shapes ----

func TestTestDetailStruct(t *testing.T) {
	typ := reflect.TypeOf((*TestDetail)(nil)).Elem()
	v := newModel(typ)

	// TestDetail must embed Test and have Questions slice
	fieldType(t, v, "Test", reflect.TypeOf(Test{}))
	qs, ok := typ.FieldByName("Questions")
	if !ok {
		t.Fatal("TestDetail missing Questions field")
	}
	if qs.Type.Kind() != reflect.Slice {
		t.Errorf("TestDetail.Questions should be a slice, got %s", qs.Type.Kind())
	}
	if qs.Type.Elem() != reflect.TypeOf(QuestionWithOptions{}) {
		t.Errorf("TestDetail.Questions element should be QuestionWithOptions, got %s", qs.Type.Elem())
	}
}

func TestQuestionWithOptionsStruct(t *testing.T) {
	typ := reflect.TypeOf((*QuestionWithOptions)(nil)).Elem()
	v := newModel(typ)

	fieldType(t, v, "Question", reflect.TypeOf(Question{}))
	jsonTag(t, v, "SortOrder", "sort_order")
	fieldKind(t, v, "SortOrder", reflect.Int)
	opts, ok := typ.FieldByName("Options")
	if !ok {
		t.Fatal("QuestionWithOptions missing Options field")
	}
	if opts.Type.Kind() != reflect.Slice {
		t.Errorf("QuestionWithOptions.Options should be a slice, got %s", opts.Type.Kind())
	}
	if opts.Type.Elem() != reflect.TypeOf(QuestionOption{}) {
		t.Errorf("QuestionWithOptions.Options element should be QuestionOption, got %s", opts.Type.Elem())
	}
}

// ---- SessionResult ----

func TestSessionResultStruct(t *testing.T) {
	typ := reflect.TypeOf((*SessionResult)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "State", "state")
	jsonTag(t, v, "ResultConfig", "result_config,omitempty")
	jsonTag(t, v, "ResultReleaseAt", "result_release_at,omitempty")
	jsonTag(t, v, "Score", "score")
	jsonTag(t, v, "CorrectCount", "correct_count")
	jsonTag(t, v, "WrongCount", "wrong_count")
	jsonTag(t, v, "EmptyCount", "empty_count")
	jsonTag(t, v, "Rank", "rank")
	jsonTag(t, v, "Breakdown", "breakdown,omitempty")
	jsonTag(t, v, "Pembahasan", "pembahasan,omitempty")
	jsonTag(t, v, "CertificateURL", "certificate_url,omitempty")

	fieldKind(t, v, "State", reflect.String)
	fieldKind(t, v, "ResultConfig", reflect.String)
	fieldKind(t, v, "Score", reflect.Float64)
	fieldKind(t, v, "CorrectCount", reflect.Int)
	fieldKind(t, v, "WrongCount", reflect.Int)
	fieldKind(t, v, "EmptyCount", reflect.Int)
	fieldKind(t, v, "Rank", reflect.Int)

	fieldKind(t, v, "ResultReleaseAt", reflect.Ptr)
	if mustField(t, v, "ResultReleaseAt").Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Errorf("SessionResult.ResultReleaseAt should be *time.Time, got %s", mustField(t, v, "ResultReleaseAt").Type)
	}
	fieldKind(t, v, "CertificateURL", reflect.Ptr)
	if mustField(t, v, "CertificateURL").Type.Elem().Kind() != reflect.String {
		t.Errorf("SessionResult.CertificateURL pointer base type should be string")
	}

	brk, ok := typ.FieldByName("Breakdown")
	if !ok {
		t.Fatal("SessionResult missing Breakdown field")
	}
	if brk.Type.Kind() != reflect.Slice {
		t.Errorf("SessionResult.Breakdown should be a slice, got %s", brk.Type.Kind())
	}
	if brk.Type.Elem() != reflect.TypeOf(ResultTopicRow{}) {
		t.Errorf("SessionResult.Breakdown element should be ResultTopicRow, got %s", brk.Type.Elem())
	}

	pem, ok := typ.FieldByName("Pembahasan")
	if !ok {
		t.Fatal("SessionResult missing Pembahasan field")
	}
	if pem.Type.Kind() != reflect.Slice {
		t.Errorf("SessionResult.Pembahasan should be a slice, got %s", pem.Type.Kind())
	}
	if pem.Type.Elem() != reflect.TypeOf(ResultPembahasanItem{}) {
		t.Errorf("SessionResult.Pembahasan element should be ResultPembahasanItem, got %s", pem.Type.Elem())
	}
}

// ---- ExamLeaderboardEntry ----

func TestExamLeaderboardEntryStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamLeaderboardEntry)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "Rank", "rank")
	jsonTag(t, v, "SessionID", "session_id")
	jsonTag(t, v, "StudentID", "student_id")
	jsonTag(t, v, "StudentName", "student_name")
	jsonTag(t, v, "Score", "score")

	fieldKind(t, v, "Rank", reflect.Int)
	fieldKind(t, v, "StudentName", reflect.String)
	fieldKind(t, v, "Score", reflect.Float64)
	fieldType(t, v, "SessionID", reflect.TypeOf(uuid.UUID{}))
	fieldType(t, v, "StudentID", reflect.TypeOf(uuid.UUID{}))
}

// ---- ScoreBucket ----

func TestScoreBucketStruct(t *testing.T) {
	typ := reflect.TypeOf((*ScoreBucket)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "Label", "label")
	jsonTag(t, v, "Count", "count")

	fieldKind(t, v, "Label", reflect.String)
	fieldKind(t, v, "Count", reflect.Int)
}

// ---- ExamAnalytics ----

func TestExamAnalyticsStruct(t *testing.T) {
	typ := reflect.TypeOf((*ExamAnalytics)(nil)).Elem()
	v := newModel(typ)

	jsonTag(t, v, "AverageScore", "average_score")
	jsonTag(t, v, "CompletionRate", "completion_rate")
	jsonTag(t, v, "Distribution", "distribution")

	fieldKind(t, v, "AverageScore", reflect.Float64)
	fieldKind(t, v, "CompletionRate", reflect.Float64)

	dist, ok := typ.FieldByName("Distribution")
	if !ok {
		t.Fatal("ExamAnalytics missing Distribution field")
	}
	if dist.Type.Kind() != reflect.Slice {
		t.Errorf("ExamAnalytics.Distribution should be a slice, got %s", dist.Type.Kind())
	}
	if dist.Type.Elem() != reflect.TypeOf(ScoreBucket{}) {
		t.Errorf("ExamAnalytics.Distribution element should be ScoreBucket, got %s", dist.Type.Elem())
	}
}

// All main + composite structs must be reachable
func TestExamTypesRegistered(t *testing.T) {
	names := []string{
		"Test", "Question", "QuestionOption", "Exam", "ExamTest",
		"ExamRegistration", "ExamSession", "ExamSessionAnswer", "SessionViolationLog",
		"SessionResult", "TestDetail", "QuestionWithOptions",
		"ExamLeaderboardEntry", "ScoreBucket", "ExamAnalytics",
	}
	for _, n := range names {
		if _, ok := typesByName[n]; !ok {
			t.Errorf("model.%s not registered", n)
		}
	}
}

var typesByName = func() map[string]reflect.Type {
	m := map[string]reflect.Type{}
	typ := reflect.TypeOf((*Test)(nil)).Elem()
	m[typ.Name()] = typ
	for _, t := range []reflect.Type{
		reflect.TypeOf((*Question)(nil)).Elem(),
		reflect.TypeOf((*QuestionOption)(nil)).Elem(),
		reflect.TypeOf((*Exam)(nil)).Elem(),
		reflect.TypeOf((*ExamTest)(nil)).Elem(),
		reflect.TypeOf((*ExamRegistration)(nil)).Elem(),
		reflect.TypeOf((*ExamSession)(nil)).Elem(),
		reflect.TypeOf((*ExamSessionAnswer)(nil)).Elem(),
		reflect.TypeOf((*SessionViolationLog)(nil)).Elem(),
			reflect.TypeOf((*TestDetail)(nil)).Elem(),
			reflect.TypeOf((*SessionResult)(nil)).Elem(),
			reflect.TypeOf((*ExamLeaderboardEntry)(nil)).Elem(),
			reflect.TypeOf((*ScoreBucket)(nil)).Elem(),
			reflect.TypeOf((*ExamAnalytics)(nil)).Elem(),
			reflect.TypeOf((*QuestionWithOptions)(nil)).Elem(),
	} {
		m[t.Name()] = t
	}
	return m
}()
