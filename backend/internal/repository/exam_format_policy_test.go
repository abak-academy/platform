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

	// Explicit policy survives; a zero FormatVersion resolves to the column default (1).
	sequential := &model.Exam{Title: "E " + uuid.NewString(), ResultConfig: "hidden", Status: "draft", NavigationPolicy: "sequential"}
	require.NoError(t, repo.CreateExam(ctx, sequential))
	require.Equal(t, 1, sequential.FormatVersion)
	require.Equal(t, "sequential", sequential.NavigationPolicy)

	got, err := repo.GetExamByID(ctx, sequential.ID)
	require.NoError(t, err)
	require.Equal(t, 1, got.FormatVersion)
	require.Equal(t, "sequential", got.NavigationPolicy)

	// Empty policy resolves to the default 'free'.
	defaulted := &model.Exam{Title: "E " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
	require.NoError(t, repo.CreateExam(ctx, defaulted))
	require.Equal(t, 1, defaulted.FormatVersion)
	require.Equal(t, "free", defaulted.NavigationPolicy)

	// UpdateExamPATCH may change navigation_policy but must never touch format_version.
	onUpdate := sequential
	onUpdate.FormatVersion = 99
	onUpdate.NavigationPolicy = "free"
	require.NoError(t, repo.UpdateExam(ctx, sequential.ID, onUpdate))

	afterUpdate, err := repo.GetExamByID(ctx, sequential.ID)
	require.NoError(t, err)
	require.Equal(t, 1, afterUpdate.FormatVersion, "format_version must not be mutable via UpdateExam")
	require.Equal(t, "free", afterUpdate.NavigationPolicy)
}

func TestExamFormatPolicy_ExamTestPresentationRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	test := &model.Test{Title: "P " + uuid.NewString(), Subject: "english", Topic: "listening", DurationMinutes: 15}
	require.NoError(t, repo.CreateTest(ctx, test))
	exam := &model.Exam{Title: "PE " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
	require.NoError(t, repo.CreateExam(ctx, exam))

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceExamTestsTx(ctx, tx, exam.ID, []model.ExamTest{
		{ExamID: exam.ID, TestID: test.ID, SortOrder: 0, QuestionPresentation: "all_on_page"},
	}))
	require.NoError(t, tx.Commit(ctx))

	detail, err := repo.GetExamDetail(ctx, exam.ID)
	require.NoError(t, err)
	require.Len(t, detail.Tests, 1)
	require.Equal(t, "all_on_page", detail.Tests[0].QuestionPresentation)

	var stored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT question_presentation FROM exam_test WHERE exam_id = $1`, exam.ID).Scan(&stored))
	require.Equal(t, "all_on_page", stored)

	// An empty presentation falls back to the one_per_page column default.
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceExamTestsTx(ctx, tx2, exam.ID, []model.ExamTest{
		{ExamID: exam.ID, TestID: test.ID, SortOrder: 0},
	}))
	require.NoError(t, tx2.Commit(ctx))

	require.NoError(t, pool.QueryRow(ctx, `SELECT question_presentation FROM exam_test WHERE exam_id = $1`, exam.ID).Scan(&stored))
	require.Equal(t, "one_per_page", stored)
}

func TestExamFormatPolicy_InvalidValuesRejected(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO exam (title, navigation_policy) VALUES ('bad-nav', 'bogus')`)
	assertCheckViolation(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO exam (title, format_version) VALUES ('bad-format', 0)`)
	assertCheckViolation(t, err)

	exam := &model.Exam{Title: "CV " + uuid.NewString(), ResultConfig: "hidden", Status: "draft"}
	require.NoError(t, repo.CreateExam(ctx, exam))
	test := &model.Test{Title: "CV " + uuid.NewString(), Subject: "s", Topic: "t", DurationMinutes: 5}
	require.NoError(t, repo.CreateTest(ctx, test))

	_, err = pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order, question_presentation) VALUES ($1, $2, 0, 'bogus')`,
		exam.ID, test.ID)
	assertCheckViolation(t, err)
}
