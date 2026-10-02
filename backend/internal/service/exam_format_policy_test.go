package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestExamFormatPolicy_sanitizesMaterialHTML(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, tc := range []struct{ name, input, want string }{
		{"script", "<p>intro<script>alert(1)</script></p>", "<p>intro</p>"},
		{"event handler", `<img src="/x.png" onerror="alert(2)">`, `<img src="/x.png">`},
		{"iframe", `<b>read</b><iframe src="evil"></iframe>`, "<b>read</b>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.CreateTest(ctx, model.Test{
				Title: "sanitize " + uniqueSuffix(), Subject: "english", Topic: "reading", DurationMinutes: 10,
				SharedMaterialHTML: &tc.input, InstructionsHTML: &tc.input,
			})
			require.NoError(t, err)
			require.Equal(t, &tc.want, created.SharedMaterialHTML)
			require.Equal(t, &tc.want, created.InstructionsHTML)
			stored, err := repo.GetTestByID(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, &tc.want, stored.SharedMaterialHTML)
			require.Equal(t, &tc.want, stored.InstructionsHTML)
			created.SharedMaterialHTML = &tc.input
			created.InstructionsHTML = nil
			updated, err := svc.UpdateTest(ctx, created.ID, created)
			require.NoError(t, err)
			require.Equal(t, &tc.want, updated.SharedMaterialHTML)
			require.Nil(t, updated.InstructionsHTML)
			stored, err = repo.GetTestByID(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, &tc.want, stored.SharedMaterialHTML)
			require.Nil(t, stored.InstructionsHTML)
			q, err := svc.CreateBankQuestion(ctx, model.Question{
				Format: "essay", Body: "Explain the stem " + uniqueSuffix(), PointCorrect: 1, MaterialHTML: &tc.input,
			}, nil, nil, nil)
			require.NoError(t, err)
			require.Equal(t, &tc.want, q.Question.MaterialHTML)
			storedQuestion, err := repo.GetQuestionByID(ctx, q.Question.ID)
			require.NoError(t, err)
			require.Equal(t, &tc.want, storedQuestion.MaterialHTML)
		})
	}
}

func TestExamFormatPolicy_navigationPolicyDefaultedOnCreate(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, tc := range []struct{ name, policy, want string }{
		{"omitted defaults to free", "", "free"},
		{"explicit sequential", "sequential", "sequential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exam, err := svc.CreateExam(ctx, model.Exam{Title: "nav " + uniqueSuffix(), NavigationPolicy: tc.policy})
			require.NoError(t, err)
			require.Equal(t, 1, exam.FormatVersion)
			require.Equal(t, tc.want, exam.NavigationPolicy)
			stored, err := repo.GetExamByID(ctx, exam.ID)
			require.NoError(t, err)
			require.Equal(t, 1, stored.FormatVersion)
			require.Equal(t, tc.want, stored.NavigationPolicy)
		})
	}
}

func intPtr(v int) *int { return &v }

func v2Section(testID uuid.UUID, title string, durationMinutes *int, presentation string) model.ExamTestEntry {
	entry := model.ExamTestEntry{}
	entry.TestID = testID
	entry.Test.Title = title
	entry.Test.DurationMinutes = durationMinutes
	entry.QuestionPresentation = presentation
	return entry
}

// TestExamFormatPolicy_hasQuestionMaterial pins the machine-differentiation of
// question material from body: only material_html/image_url/audio_url count, and
// blank or whitespace-only material_html does not.
func TestExamFormatPolicy_hasQuestionMaterial(t *testing.T) {
	blank := ""
	spaces := "   "
	material := "<p>passage</p>"
	image := "https://cdn.example/x.png"
	audio := "https://cdn.example/x.mp3"

	tests := []struct {
		name string
		q    model.Question
		want bool
	}{
		{"nothing set", model.Question{}, false},
		{"body alone is not material", model.Question{Body: "explain the passage"}, false},
		{"empty material_html", model.Question{MaterialHTML: &blank}, false},
		{"whitespace-only material_html", model.Question{MaterialHTML: &spaces}, false},
		{"material_html", model.Question{MaterialHTML: &material}, true},
		{"image_url", model.Question{ImageURL: &image}, true},
		{"audio_url", model.Question{AudioURL: &audio}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasQuestionMaterial(tt.q))
		})
	}
}

// TestExamFormatPolicy_V2PublishMatrix covers every accepted/rejected combination
// of the format_version=2 publish matrix, plus the regression that legacy exams
// keep the mode-driven FR-19 rules.
func TestExamFormatPolicy_V2PublishMatrix(t *testing.T) {
	testA := uuid.New()
	testB := uuid.New()
	question := model.Question{ID: uuid.New(), Format: "essay", Body: "body", PointCorrect: 1}
	material := "<p>own material</p>"
	questionWithMaterial := model.Question{ID: uuid.New(), Format: "essay", Body: "body", PointCorrect: 1, MaterialHTML: &material}

	v2Exam := func(nav, timer string, duration *int) model.Exam {
		return model.Exam{FormatVersion: 2, Mode: "standard", NavigationPolicy: nav, TimerMode: timer, DurationMinutes: duration}
	}

	tests := []struct {
		name      string
		exam      model.Exam
		sections  []model.ExamTestEntry
		questions map[uuid.UUID][]model.Question
		wantErr   bool
		wantMsg   string
	}{
		{
			name:      "free overall with positive duration accepted",
			exam:      v2Exam("free", "overall", intPtr(60)),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}},
		},
		{
			name:      "sequential per_test with positive section durations accepted",
			exam:      v2Exam("sequential", "per_test", nil),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "one_per_page"), v2Section(testB, "Section B", intPtr(45), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}, testB: {question}},
		},
		{
			name:      "all_on_page without question material accepted",
			exam:      v2Exam("free", "overall", intPtr(60)),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "all_on_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}},
		},
		{
			name:     "sequential without any test rejected",
			exam:     v2Exam("sequential", "overall", intPtr(60)),
			sections: nil,
			wantErr:  true,
		},
		{
			name:      "overall without positive exam duration rejected",
			exam:      v2Exam("free", "overall", nil),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}},
			wantErr:   true,
		},
		{
			name:      "per_test with non-positive section duration rejected and names the section",
			exam:      v2Exam("sequential", "per_test", nil),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(0), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}},
			wantErr:   true,
			wantMsg:   "Section A",
		},
		{
			name:      "free with per_test rejected",
			exam:      v2Exam("free", "per_test", nil),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {question}},
			wantErr:   true,
		},
		{
			name:      "attached test without questions rejected and names the section",
			exam:      v2Exam("free", "overall", intPtr(60)),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "one_per_page")},
			questions: map[uuid.UUID][]model.Question{testA: {}},
			wantErr:   true,
			wantMsg:   "Section A",
		},
		{
			name:      "all_on_page with question material rejected and names the question",
			exam:      v2Exam("free", "overall", intPtr(60)),
			sections:  []model.ExamTestEntry{v2Section(testA, "Section A", intPtr(30), "all_on_page")},
			questions: map[uuid.UUID][]model.Question{testA: {questionWithMaterial}},
			wantErr:   true,
			wantMsg:   questionWithMaterial.ID.String(),
		},
		{
			name:     "legacy v1 utbk without tests still rejected",
			exam:     model.Exam{FormatVersion: 1, Mode: "utbk"},
			sections: nil,
			wantErr:  true,
		},
		{
			name:     "legacy v1 ielts untyped section still rejected",
			exam:     model.Exam{FormatVersion: 1, Mode: "ielts"},
			sections: []model.ExamTestEntry{entryTitled("Untyped Section", nil)},
			wantErr:  true,
			wantMsg:  "Untyped Section",
		},
		{
			name:     "legacy v1 standard skips the gate",
			exam:     model.Exam{FormatVersion: 1, Mode: "standard"},
			sections: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublishSections(tt.exam, tt.sections, tt.questions)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrValidation)
			if tt.wantMsg != "" {
				require.Contains(t, err.Error(), tt.wantMsg)
			}
		})
	}
}

func createV2Exam(t *testing.T, repo *repository.Repository, nav string) uuid.UUID {
	t.Helper()
	exam := &model.Exam{
		Title:            "V2 Edit " + uniqueSuffix(),
		ResultConfig:     "hidden",
		Status:           "draft",
		FormatVersion:    2,
		NavigationPolicy: nav,
	}
	require.NoError(t, repo.CreateExam(context.Background(), exam))
	return exam.ID
}

func seedExamSession(t *testing.T, repo *repository.Repository, examID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var studentID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		 VALUES ($1, $2, 'sd', 'student', 'active', 'password') RETURNING id`,
		"Session Student "+uniqueSuffix(), "ss_"+uniqueSuffix()).Scan(&studentID))
	var regID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString()).Scan(&regID))
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at) VALUES ($1, $2, $3, now())`,
		regID, studentID, examID)
	require.NoError(t, err)
	return studentID
}

func seedExamRegistrationOnly(t *testing.T, repo *repository.Repository, examID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var studentID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		 VALUES ($1, $2, 'sd', 'student', 'active', 'password') RETURNING id`,
		"Registered Student "+uniqueSuffix(), "rs_"+uniqueSuffix()).Scan(&studentID))
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
		studentID, examID, uuid.NewString())
	require.NoError(t, err)
}

func seedTestWithQuestion(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var testID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'math', 'topic', 30) RETURNING id`,
		"Edit Test "+uniqueSuffix()).Scan(&testID))
	var questionID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('essay', 'q', 1, 0) RETURNING id`).Scan(&questionID))
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`, testID, questionID)
	require.NoError(t, err)
	return testID
}

func TestExamFormatPolicy_V2EditSafetyRejectsPolicyChangeWithSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		wantErr error
	}{
		{"navigation", ErrExamHasSessions},
		{"randomize", ErrExamHasSessions},
		{"title", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			examID := createV2Exam(t, repo, "free")
			seedExamSession(t, repo, examID)
			detail, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			overlay := detail.Exam
			switch tc.name {
			case "navigation":
				overlay.NavigationPolicy = "sequential"
			case "randomize":
				overlay.Randomize = !overlay.Randomize
			case "title":
				overlay.Title = "Renamed " + uniqueSuffix()
			}
			_, err = svc.UpdateExam(ctx, examID, overlay, uuid.NewString())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			stored, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			want := detail.Exam
			if tc.wantErr == nil {
				want = overlay
			}
			require.Equal(t, want.NavigationPolicy, stored.Exam.NavigationPolicy)
			require.Equal(t, want.Randomize, stored.Exam.Randomize)
			require.Equal(t, want.Title, stored.Exam.Title)
		})
	}
}

// TestExamFormatPolicy_V2EditSafetyRejectsTestListChangeWithSession proves the
// attached-test list and question_presentation freeze once an attempt exists.
func TestExamFormatPolicy_V2EditSafetyRejectsTestListChangeWithSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	examID := createV2Exam(t, repo, "free")
	testID := seedTestWithQuestion(t, repo)
	seedExamSession(t, repo, examID)

	err := svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: testID, QuestionPresentation: "one_per_page"}}, uuid.NewString())
	require.ErrorIs(t, err, ErrExamHasSessions)
}

func TestExamFormatPolicy_V2EditSafetyAllowsUnchangedTestListWithSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		changed bool
		wantErr error
	}{
		{"same list with omitted presentation", false, nil},
		{"different test", true, ErrExamHasSessions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			examID := createV2Exam(t, repo, "free")
			testID := seedTestWithQuestion(t, repo)
			require.NoError(t, svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: testID}}, uuid.NewString()))
			seedExamSession(t, repo, examID)
			replacement := testID
			if tc.changed {
				replacement = seedTestWithQuestion(t, repo)
			}
			err := svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: replacement}}, uuid.NewString())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			stored, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			require.Len(t, stored.Tests, 1)
			require.Equal(t, testID, stored.Tests[0].TestID)
			require.Equal(t, "one_per_page", stored.Tests[0].QuestionPresentation)
		})
	}
}

// TestExamFormatPolicy_V2RejectedEditLeavesNoAuditRow proves validation runs
// before the edit-safety check, so a request rejected for bad input cannot leave
// an exam.format_policy row behind.
func TestExamFormatPolicy_V2RejectedEditLeavesNoAuditRow(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	examID := createV2Exam(t, repo, "free")
	seedExamRegistrationOnly(t, repo, examID)

	testID := seedTestWithQuestion(t, repo)
	actor := uuid.NewString()
	err := svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: testID, QuestionPresentation: "side_by_side"}}, actor)
	require.ErrorIs(t, err, ErrValidation)

	var auditCount int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'exam.format_policy' AND target_id = $1 AND actor_id = $2`,
		examID.String(), actor).Scan(&auditCount))
	require.Zero(t, auditCount)
}

// TestExamFormatPolicy_V2EditSafetyAllowsChangeWithoutSessionAndAudits proves a
// registration without a session permits policy/section edits and audits them.
func TestExamFormatPolicy_V2EditSafetyAllowsChangeWithoutSessionAndAudits(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	examID := createV2Exam(t, repo, "free")
	seedExamRegistrationOnly(t, repo, examID)

	detail, err := svc.GetExam(ctx, examID)
	require.NoError(t, err)

	actor := uuid.NewString()
	navOverlay := detail.Exam
	navOverlay.NavigationPolicy = "sequential"
	_, err = svc.UpdateExam(ctx, examID, navOverlay, actor)
	require.NoError(t, err)

	var auditCount int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'exam.format_policy' AND target_type = 'exam' AND target_id = $1 AND actor_id = $2`,
		examID.String(), actor).Scan(&auditCount))
	require.Equal(t, 1, auditCount)

	testID := seedTestWithQuestion(t, repo)
	require.NoError(t, svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: testID, QuestionPresentation: "one_per_page"}}, actor))
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'exam.format_policy' AND target_id = $1 AND actor_id = $2`,
		examID.String(), actor).Scan(&auditCount))
	require.Equal(t, 2, auditCount)
}

// TestExamFormatPolicy_V2EditSafetyWithoutRegistrationIsNotAudited proves the
// audit trail is registration-scoped: policy edits on an unregistered exam stay silent.
func TestExamFormatPolicy_V2EditSafetyWithoutRegistrationIsNotAudited(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	examID := createV2Exam(t, repo, "free")

	detail, err := svc.GetExam(ctx, examID)
	require.NoError(t, err)

	navOverlay := detail.Exam
	navOverlay.NavigationPolicy = "sequential"
	_, err = svc.UpdateExam(ctx, examID, navOverlay, uuid.NewString())
	require.NoError(t, err)

	var auditCount int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'exam.format_policy' AND target_id = $1`,
		examID.String()).Scan(&auditCount))
	require.Zero(t, auditCount)
}

// TestExamFormatPolicy_V1EditSafetyAllowsChangeWithSession is the no-regression
// pin: legacy exams keep today's unrestricted edit behavior.
func TestExamFormatPolicy_V1EditSafetyAllowsChangeWithSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	exam := &model.Exam{Title: "V1 Edit " + uniqueSuffix(), ResultConfig: "hidden", Status: "draft", NavigationPolicy: "free"}
	require.NoError(t, repo.CreateExam(ctx, exam))
	seedExamSession(t, repo, exam.ID)

	detail, err := svc.GetExam(ctx, exam.ID)
	require.NoError(t, err)

	navOverlay := detail.Exam
	navOverlay.NavigationPolicy = "sequential"
	_, err = svc.UpdateExam(ctx, exam.ID, navOverlay, uuid.NewString())
	require.NoError(t, err)
}

func TestExamFormatPolicy_rejectsInvalidEnumValues(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, field, value string
		wantErr            error
	}{
		{"invalid navigation", "navigation", "bogus", ErrValidation},
		{"valid navigation", "navigation", "sequential", nil},
		{"invalid presentation", "presentation", "side_by_side", ErrValidation},
		{"valid presentation", "presentation", "all_on_page", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			examID := createV2Exam(t, repo, "free")
			detail, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			if tc.field == "navigation" {
				overlay := detail.Exam
				overlay.NavigationPolicy = tc.value
				_, err = svc.UpdateExam(ctx, examID, overlay, uuid.NewString())
			} else {
				testID := seedTestWithQuestion(t, repo)
				err = svc.ReplaceExamTests(ctx, examID, []model.ExamTest{{TestID: testID, QuestionPresentation: tc.value}}, uuid.NewString())
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			stored, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			if tc.field == "navigation" {
				want := "free"
				if tc.wantErr == nil {
					want = tc.value
				}
				require.Equal(t, want, stored.Exam.NavigationPolicy)
			} else if tc.wantErr != nil {
				require.Empty(t, stored.Tests)
			} else {
				require.Len(t, stored.Tests, 1)
				require.Equal(t, tc.value, stored.Tests[0].QuestionPresentation)
			}
		})
	}
}
