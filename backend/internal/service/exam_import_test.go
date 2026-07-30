package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuestionImportResultRow_marshalsSnakeCase(t *testing.T) {
	qid := uuid.New()
	row := QuestionImportResultRow{RowNumber: 2, Status: "error", QuestionID: &qid, Error: "bad format"}

	data, err := json.Marshal(row)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, float64(2), decoded["row_number"])
	assert.Equal(t, "error", decoded["status"])
	assert.Equal(t, qid.String(), decoded["question_id"])
	assert.Equal(t, "bad format", decoded["error"])

	// omitempty fields absent (not null) when zero-valued.
	inserted := QuestionImportResultRow{RowNumber: 1, Status: "inserted"}
	data, err = json.Marshal(inserted)
	require.NoError(t, err)
	decoded = map[string]any{}
	require.NoError(t, json.Unmarshal(data, &decoded))
	_, hasQuestionID := decoded["question_id"]
	_, hasError := decoded["error"]
	assert.False(t, hasQuestionID)
	assert.False(t, hasError)
}

func TestParseQuestionImportCSV_acceptsFlexibleOptions(t *testing.T) {
	csv := []byte(`format,body,subject,topic,difficulty,point_correct,point_wrong,correct_answer,option_a,option_b,option_c
mcq,2+2,Math,Arithmetic,medium,2,0,a,4,5,6`)

	rows, err := ParseQuestionImportCSV(csv)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	r := rows[0]
	assert.Equal(t, "mcq", r.Format)
	assert.Equal(t, "2+2", r.Body)
	assert.Equal(t, "Math", r.Subject)
	assert.Equal(t, "Arithmetic", r.Topic)
	require.NotNil(t, r.Difficulty)
	assert.Equal(t, "medium", *r.Difficulty)
	assert.Equal(t, 2.0, r.PointCorrect)
	assert.Equal(t, 0.0, r.PointWrong)
	require.NotNil(t, r.CorrectAnswer)
	assert.Equal(t, "a", *r.CorrectAnswer)
	require.Len(t, r.Options, 3)
	assert.Equal(t, "a", r.Options[0].Key)
	assert.Equal(t, "4", r.Options[0].Text)
	assert.True(t, r.Options[0].IsCorrect)
	assert.False(t, r.Options[1].IsCorrect)
}

func TestParseQuestionImportCSV_acceptsFractionalPoints(t *testing.T) {
	csv := []byte(`format,body,subject,topic,point_correct,point_wrong
essay,explain gravity,Physics,Mechanics,2.5,0.5`)

	rows, err := ParseQuestionImportCSV(csv)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Equal(t, 2.5, rows[0].PointCorrect)
	assert.Equal(t, 0.5, rows[0].PointWrong)
	assert.Empty(t, rows[0].Error)
}

func TestParseQuestionImportCSV_rejectsMissingHeader(t *testing.T) {
	csv := []byte(`format,body,subject,topic,difficulty,point_wrong,correct_answer,option_a
mcq,2+2,Math,Arithmetic,medium,0,a,4`)

	_, err := ParseQuestionImportCSV(csv)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "point_correct"), "expected error about missing point_correct header, got %v", err)
}

func TestProcessQuestionImportRows_mixedRowsWithTopicResolution(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	mathTopic := model.ExamTopic{Name: "Arithmetic " + suffix, Subject: "Math " + suffix}
	require.NoError(t, repo.CreateTopic(ctx, &mathTopic))
	defer func() {
		_ = repo.DeleteTopic(ctx, mathTopic.ID)
	}()

	csvRows := []QuestionImportRow{
		{
			Format:       "mcq",
			Body:         "2+2",
			Subject:      mathTopic.Subject,
			Topic:        mathTopic.Name,
			PointCorrect: 2,
			PointWrong:   0,
			CorrectAnswer: strPtr("a"),
			Options: []model.QuestionOption{
				{Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
				{Key: "b", Text: "5", IsCorrect: false, SortOrder: 2},
			},
		},
		{
			Format:       "essay",
			Body:         "explain gravity",
			Subject:      mathTopic.Subject,
			Topic:        mathTopic.Name,
			PointCorrect: 5,
			PointWrong:   0,
		},
		{
			Format:       "mcq",
			Body:         "bad: no options",
			Subject:      mathTopic.Subject,
			Topic:        mathTopic.Name,
			PointCorrect: 1,
			PointWrong:   0,
			CorrectAnswer: strPtr("a"),
		},
		{
			Format:       "mcq",
			Body:         "unknown topic",
			Subject:      "Missing",
			Topic:        "Missing",
			PointCorrect: 1,
			PointWrong:   0,
			CorrectAnswer: strPtr("a"),
			Options: []model.QuestionOption{
				{Key: "a", Text: "x", IsCorrect: true, SortOrder: 1},
				{Key: "b", Text: "y", IsCorrect: false, SortOrder: 2},
			},
		},
	}

	result, err := svc.ProcessQuestionImportRows(ctx, csvRows)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Inserted)
	require.Len(t, result.Rows, 4)

	assert.Equal(t, "inserted", result.Rows[0].Status)
	require.NotNil(t, result.Rows[0].QuestionID)

	assert.Equal(t, "inserted", result.Rows[1].Status)
	require.NotNil(t, result.Rows[1].QuestionID)

	assert.Equal(t, "error", result.Rows[2].Status)
	assert.True(t, strings.Contains(result.Rows[2].Error, "at least 2 options"), "expected options error, got %q", result.Rows[2].Error)

	assert.Equal(t, "error", result.Rows[3].Status)
	assert.True(t, strings.Contains(result.Rows[3].Error, "topic not found"), "expected topic not found error, got %q", result.Rows[3].Error)

	t.Cleanup(func() {
		for _, r := range result.Rows {
			if r.QuestionID != nil {
				_ = repo.DeleteQuestion(ctx, *r.QuestionID)
			}
		}
	})

	// Valid rows should have been persisted despite the bad rows. Scope by this
	// test's own topic — an unscoped list can push these rows past a fixed
	// Limit once other tests in the shared DB have inserted enough bank
	// questions, especially under -shuffle=on.
	bank, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{TopicID: mathTopic.ID.String(), Limit: 10})
	require.NoError(t, err)
	var found int
	for _, q := range bank {
		if result.Rows[0].QuestionID != nil && q.Question.ID == *result.Rows[0].QuestionID {
			found++
		}
		if result.Rows[1].QuestionID != nil && q.Question.ID == *result.Rows[1].QuestionID {
			found++
		}
	}
	assert.Equal(t, 2, found)
}

func TestImportQuestionsFromCSV_endToEnd(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	mathTopic := model.ExamTopic{Name: "Algebra " + suffix, Subject: "Math " + suffix}
	require.NoError(t, repo.CreateTopic(ctx, &mathTopic))
	defer func() {
		_ = repo.DeleteTopic(ctx, mathTopic.ID)
	}()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"format", "body", "subject", "topic", "difficulty", "point_correct", "point_wrong", "correct_answer", "option_a", "option_b", "option_c", "option_d"})
	_ = w.Write([]string{"mcq", "2+2", mathTopic.Subject, mathTopic.Name, "easy", "2", "0", "a", "4", "5", "6", "7"})
	_ = w.Write([]string{"multi_answer", "primes", mathTopic.Subject, mathTopic.Name, "medium", "3", "0", "a,b", "2", "3", "4", "5"})
	_ = w.Write([]string{"short", "capital of france", mathTopic.Subject, mathTopic.Name, "easy", "1", "0", "Paris", "", "", "", ""})
	_ = w.Write([]string{"essay", "explain gravity", mathTopic.Subject, mathTopic.Name, "hard", "5", "0", "", "", "", "", ""})
	_ = w.Write([]string{"mcq", "bad row", mathTopic.Subject, mathTopic.Name, "easy", "1", "0", "a", "only one", "", "", ""})
	w.Flush()
	require.NoError(t, w.Error())

	result, err := svc.ImportQuestionsFromCSV(ctx, buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, 4, result.Inserted)
	require.Len(t, result.Rows, 5)

	assert.Equal(t, "inserted", result.Rows[0].Status)
	assert.Equal(t, "inserted", result.Rows[1].Status)
	assert.Equal(t, "inserted", result.Rows[2].Status)
	assert.Equal(t, "inserted", result.Rows[3].Status)
	assert.Equal(t, "error", result.Rows[4].Status)

	t.Cleanup(func() {
		for _, r := range result.Rows {
			if r.QuestionID != nil {
				_ = repo.DeleteQuestion(ctx, *r.QuestionID)
			}
		}
	})
}

func TestProcessQuestionImportRows_sanitizesScriptPayloadInBody(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	mathTopic := model.ExamTopic{Name: "Arithmetic " + suffix, Subject: "Math " + suffix}
	require.NoError(t, repo.CreateTopic(ctx, &mathTopic))
	t.Cleanup(func() { _ = repo.DeleteTopic(ctx, mathTopic.ID) })

	csvRows := []QuestionImportRow{
		{
			Format:       "mcq",
			Body:         `<script>alert(1)</script>What is 2+2?`,
			Subject:      mathTopic.Subject,
			Topic:        mathTopic.Name,
			PointCorrect: 2,
			PointWrong:   0,
			CorrectAnswer: strPtr("a"),
			Options: []model.QuestionOption{
				{Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
				{Key: "b", Text: "5", IsCorrect: false, SortOrder: 2},
			},
		},
	}

	result, err := svc.ProcessQuestionImportRows(ctx, csvRows)
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	require.Equal(t, "inserted", result.Rows[0].Status)
	require.NotNil(t, result.Rows[0].QuestionID)

	// Sanity: the on-disk body has no <script> surviving the sanitization
	// applied at the CSV write path. Mirrors the interactive create path
	// (TestSanitizeQuestionBody_stripsScriptTag) so both go through the
	// same policy.
	var stored string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT body FROM question WHERE id = $1`, *result.Rows[0].QuestionID,
	).Scan(&stored))
	assert.False(t, strings.Contains(stored, "<script>"),
		"stored body must not contain <script>, got %q", stored)
	assert.Contains(t, stored, "What is 2+2?")

	t.Cleanup(func() { _ = repo.DeleteQuestion(ctx, *result.Rows[0].QuestionID) })
}
