package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
)

func assertCheckViolation(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.Error(t, err)
	require.True(t, errors.As(err, &pgErr), "want PgError, got %v", err)
	require.Equal(t, "23514", pgErr.Code, "want CHECK constraint violation (23514), got %s: %s", pgErr.Code, pgErr.Message)
}

func TestExamFormatPolicy_TestMaterialRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	shared := "<p>reading passage " + uuid.NewString() + "</p>"
	instructions := "<b>read carefully</b>"
	created := &model.Test{
		Title:              "T " + uuid.NewString(),
		Subject:            "english",
		Topic:              "reading",
		DurationMinutes:    30,
		SharedMaterialHTML: &shared,
		InstructionsHTML:   &instructions,
	}
	require.NoError(t, repo.CreateTest(ctx, created))

	got, err := repo.GetTestByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.SharedMaterialHTML)
	require.Equal(t, shared, *got.SharedMaterialHTML)
	require.NotNil(t, got.InstructionsHTML)
	require.Equal(t, instructions, *got.InstructionsHTML)

	updatedShared := "<p>updated " + uuid.NewString() + "</p>"
	created.SharedMaterialHTML = &updatedShared
	created.InstructionsHTML = nil
	require.NoError(t, repo.UpdateTest(ctx, created.ID, created))

	gotAfter, err := repo.GetTestByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, gotAfter.SharedMaterialHTML)
	require.Equal(t, updatedShared, *gotAfter.SharedMaterialHTML)
	require.Nil(t, gotAfter.InstructionsHTML)

	list, _, err := repo.ListTests(ctx, TestFilter{Limit: 100})
	require.NoError(t, err)
	var found *model.Test
	for i := range list {
		if list[i].ID == created.ID {
			found = &list[i]
			break
		}
	}
	require.NotNil(t, found, "created test missing from ListTests page")
	require.NotNil(t, found.SharedMaterialHTML)
	require.Equal(t, updatedShared, *found.SharedMaterialHTML)
	require.Nil(t, found.InstructionsHTML)
}

func TestExamFormatPolicy_QuestionMaterialRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	test := &model.Test{Title: "Q " + uuid.NewString(), Subject: "english", Topic: "reading", DurationMinutes: 10}
	require.NoError(t, repo.CreateTest(ctx, test))

	material := "<p>listen to the clip " + uuid.NewString() + "</p>"
	q := &model.Question{Format: "essay", Body: "Explain the passage", PointCorrect: 1, MaterialHTML: &material}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateQuestionTx(ctx, tx, q, nil, nil, nil))
	require.NoError(t, tx.Commit(ctx))

	stored, err := repo.GetQuestionByID(ctx, q.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.MaterialHTML)
	require.Equal(t, material, *stored.MaterialHTML)

	_, err = pool.Exec(ctx, `INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`, test.ID, q.ID)
	require.NoError(t, err)
	questions, err := repo.ListQuestions(ctx, test.ID)
	require.NoError(t, err)
	require.Len(t, questions, 1)
	require.NotNil(t, questions[0].Question.MaterialHTML)
	require.Equal(t, material, *questions[0].Question.MaterialHTML)

	bank, _, err := repo.ListBankQuestions(ctx, QuestionFilter{Search: q.ID.String()})
	require.NoError(t, err)
	require.Len(t, bank, 1)
	require.NotNil(t, bank[0].Question.MaterialHTML)
	require.Equal(t, material, *bank[0].Question.MaterialHTML)

	updatedMaterial := "<p>revised " + uuid.NewString() + "</p>"
	q.MaterialHTML = &updatedMaterial
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateQuestionTx(ctx, tx2, q, nil, nil, nil))
	require.NoError(t, tx2.Commit(ctx))

	afterUpdate, err := repo.GetQuestionByID(ctx, q.ID)
	require.NoError(t, err)
	require.NotNil(t, afterUpdate.MaterialHTML)
	require.Equal(t, updatedMaterial, *afterUpdate.MaterialHTML)
}

func TestExamFormatPolicy_ExamNavigationRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	tests := []struct {
		name             string
		navigationPolicy string
		want             string
	}{
		{"explicit policy", "sequential", "sequential"},
		{"omitted policy defaults to free", "", "free"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exam := &model.Exam{Title: "E " + uuid.NewString(), ResultConfig: "hidden", Status: "draft", NavigationPolicy: tt.navigationPolicy}
			require.NoError(t, repo.CreateExam(ctx, exam))
			require.Equal(t, 1, exam.FormatVersion)
			require.Equal(t, tt.want, exam.NavigationPolicy)

			got, err := repo.GetExamByID(ctx, exam.ID)
			require.NoError(t, err)
			require.Equal(t, 1, got.FormatVersion)
			require.Equal(t, tt.want, got.NavigationPolicy)

			exam.FormatVersion = 99
			exam.NavigationPolicy = "free"
			require.NoError(t, repo.UpdateExam(ctx, exam.ID, exam))

			afterUpdate, err := repo.GetExamByID(ctx, exam.ID)
			require.NoError(t, err)
			require.Equal(t, 1, afterUpdate.FormatVersion, "format_version must not be mutable via UpdateExam")
			require.Equal(t, "free", afterUpdate.NavigationPolicy)
		})
	}
}

func TestExamFormatPolicy_ExamTestPresentationRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	tests := []struct {
		name         string
		initial      string
		presentation string
		want         string
	}{
		{"explicit presentation", "one_per_page", "all_on_page", "all_on_page"},
		{"omitted presentation defaults to one per page", "all_on_page", "", "one_per_page"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test := &model.Test{Title: "P " + uuid.NewString(), Subject: "english", Topic: "listening", DurationMinutes: 15}
			require.NoError(t, repo.CreateTest(ctx, test))
			exam := &model.Exam{Title: "PE " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
			require.NoError(t, repo.CreateExam(ctx, exam))

			for _, presentation := range []string{tt.initial, tt.presentation} {
				tx, err := pool.Begin(ctx)
				require.NoError(t, err)
				defer tx.Rollback(ctx)
				require.NoError(t, repo.ReplaceExamTestsTx(ctx, tx, exam.ID, []model.ExamTest{
					{ExamID: exam.ID, TestID: test.ID, SortOrder: 0, QuestionPresentation: presentation},
				}))
				require.NoError(t, tx.Commit(ctx))
			}

			detail, err := repo.GetExamDetail(ctx, exam.ID)
			require.NoError(t, err)
			require.Len(t, detail.Tests, 1)
			require.Equal(t, tt.want, detail.Tests[0].QuestionPresentation)

			var stored string
			require.NoError(t, pool.QueryRow(ctx, `SELECT question_presentation FROM exam_test WHERE exam_id = $1`, exam.ID).Scan(&stored))
			require.Equal(t, tt.want, stored)
		})
	}
}

func TestExamFormatPolicy_InvalidValuesRejected(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	exam := &model.Exam{Title: "CV " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
	require.NoError(t, repo.CreateExam(ctx, exam))
	test := &model.Test{Title: "CV " + uuid.NewString(), Subject: "s", Topic: "t", DurationMinutes: 5}
	require.NoError(t, repo.CreateTest(ctx, test))

	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{"invalid navigation policy", `INSERT INTO exam (title, navigation_policy) VALUES ('bad-nav', 'bogus')`, nil},
		{"non-positive format version", `INSERT INTO exam (title, format_version) VALUES ('bad-format', 0)`, nil},
		{"invalid question presentation", `INSERT INTO exam_test (exam_id, test_id, sort_order, question_presentation) VALUES ($1, $2, 0, 'bogus')`, []any{exam.ID, test.ID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.query, tt.args...)
			assertCheckViolation(t, err)
		})
	}
}

func TestExamFormatPolicy_ExamHasAnySession(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	tests := []struct {
		name            string
		hasRegistration bool
		hasSession      bool
		want            bool
	}{
		{"no registration or session", false, false, false},
		{"registration without session", true, false, false},
		{"registration with session", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exam := &model.Exam{Title: "HAS " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
			require.NoError(t, repo.CreateExam(ctx, exam))

			if tt.hasRegistration {
				var studentID uuid.UUID
				require.NoError(t, pool.QueryRow(ctx,
					`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
					 VALUES ($1, $2, 'sd', 'student', 'active', 'password') RETURNING id`,
					"HAS Student "+uuid.NewString(), "has_"+uuid.NewString()).Scan(&studentID))

				var regID uuid.UUID
				require.NoError(t, pool.QueryRow(ctx,
					`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
					studentID, exam.ID, uuid.NewString()).Scan(&regID))

				if tt.hasSession {
					_, err := pool.Exec(ctx,
						`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at) VALUES ($1, $2, $3, now())`,
						regID, studentID, exam.ID)
					require.NoError(t, err)
				}
			}

			has, err := repo.ExamHasAnySession(ctx, exam.ID)
			require.NoError(t, err)
			require.Equal(t, tt.want, has)
		})
	}
}
