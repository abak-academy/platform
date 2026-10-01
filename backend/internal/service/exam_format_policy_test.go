package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
)

// TestExamFormatPolicy_sanitizesMaterialHTML proves the section/question material
// write paths reuse the question-body sanitizer, so a stored value can never carry
// <script>/event handlers even when the caller submits them.
func TestExamFormatPolicy_sanitizesMaterialHTML(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	shared := `<p>intro<script>alert(1)</script><img src="/x.png" onerror="alert(2)"></p>`
	instructions := `<b>read</b><iframe src="evil"></iframe>`

	created, err := svc.CreateTest(ctx, model.Test{
		Title:              "sanitize " + uniqueSuffix(),
		Subject:            "english",
		Topic:              "reading",
		DurationMinutes:    10,
		SharedMaterialHTML: &shared,
		InstructionsHTML:   &instructions,
	})
	require.NoError(t, err)
	require.NotNil(t, created.SharedMaterialHTML)
	require.NotContains(t, *created.SharedMaterialHTML, "<script>")
	require.NotContains(t, *created.SharedMaterialHTML, "onerror")
	require.Contains(t, *created.SharedMaterialHTML, "intro")
	require.NotNil(t, created.InstructionsHTML)
	require.NotContains(t, *created.InstructionsHTML, "<iframe")

	storedTest, err := repo.GetTestByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, storedTest.SharedMaterialHTML)
	require.NotContains(t, *storedTest.SharedMaterialHTML, "<script>")
	require.NotNil(t, storedTest.InstructionsHTML)
	require.NotContains(t, *storedTest.InstructionsHTML, "<iframe")

	// UpdateTest sanitizes on the write path too, and clearing to null persists.
	updatedShared := `<p>updated<script>x</script></p>`
	created.SharedMaterialHTML = &updatedShared
	created.InstructionsHTML = nil
	updated, err := svc.UpdateTest(ctx, created.ID, created)
	require.NoError(t, err)
	require.NotNil(t, updated.SharedMaterialHTML)
	require.NotContains(t, *updated.SharedMaterialHTML, "<script>")
	require.Nil(t, updated.InstructionsHTML)

	storedAfter, err := repo.GetTestByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotContains(t, *storedAfter.SharedMaterialHTML, "<script>")
	require.Nil(t, storedAfter.InstructionsHTML)

	// Question material follows the same policy.
	questionMaterial := `<p>stem<script>alert(1)</script></p>`
	q, err := svc.CreateBankQuestion(ctx, model.Question{
		Format:       "essay",
		Body:         "Explain the stem " + uniqueSuffix(),
		PointCorrect: 1,
		MaterialHTML: &questionMaterial,
	}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, q.Question.MaterialHTML)
	require.NotContains(t, *q.Question.MaterialHTML, "<script>")
	require.Contains(t, *q.Question.MaterialHTML, "stem")

	storedQuestion, err := repo.GetQuestionByID(ctx, q.Question.ID)
	require.NoError(t, err)
	require.NotNil(t, storedQuestion.MaterialHTML)
	require.NotContains(t, *storedQuestion.MaterialHTML, "<script>")
}

// TestExamFormatPolicy_navigationPolicyDefaultedOnCreate pins the create-path
// default: an empty policy persists as 'free' and format_version resolves to 1.
func TestExamFormatPolicy_navigationPolicyDefaultedOnCreate(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "nav " + uniqueSuffix()})
	require.NoError(t, err)
	require.Equal(t, 1, exam.FormatVersion)
	require.Equal(t, "free", exam.NavigationPolicy)

	sequential := "sequential"
	exam2, err := svc.CreateExam(ctx, model.Exam{Title: "nav " + uniqueSuffix(), NavigationPolicy: sequential})
	require.NoError(t, err)
	require.Equal(t, "sequential", exam2.NavigationPolicy)
}
