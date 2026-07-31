package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"akademi-bimbel/internal/model"
)

func fp(v float64) *float64 { return &v }

// Per-item points (0050): an item's own worth wins; nil inherits point_correct.
func TestPerItemPoints_gradingAndMax(t *testing.T) {
	t.Run("multi_blank: blank 1 worth 2, blank 2 inherits 1", func(t *testing.T) {
		blanks := []model.QuestionBlank{
			{Index: 1, AcceptedAnswers: []string{"a"}, Points: fp(2)},
			{Index: 2, AcceptedAnswers: []string{"b"}},
		}
		ans := `["a","b"]`
		score, ok := gradeMultiBlank(&ans, blanks, 1, 0)
		assert.Equal(t, 3.0, score)
		assert.True(t, ok)
	})

	t.Run("true_false: statement 2 worth 5", func(t *testing.T) {
		sts := []model.QuestionStatement{
			{Index: 1, IsTrue: true},
			{Index: 2, IsTrue: false, Points: fp(5)},
		}
		ans := `["true","false"]`
		score, ok := gradeTrueFalse(&ans, sts, 1, 0)
		assert.Equal(t, 6.0, score)
		assert.True(t, ok)
	})

	t.Run("multi_answer: correct option b worth 3, wrong selection still costs point_wrong", func(t *testing.T) {
		opts := []model.QuestionOption{
			{Key: "a", IsCorrect: true},
			{Key: "b", IsCorrect: true, Points: fp(3)},
			{Key: "c", IsCorrect: false},
		}
		score, ok := gradeMultiAnswerPartial("a,b,c", opts, 1, 0.5)
		assert.Equal(t, 3.5, score) // 1 + 3 - 0.5
		assert.False(t, ok)
	})

	t.Run("questionMaxPoints sums effective per-item worth for all three formats", func(t *testing.T) {
		mb := model.QuestionWithOptions{
			Question: model.Question{Format: "multi_blank", PointCorrect: 1},
			Blanks:   []model.QuestionBlank{{Index: 1, Points: fp(2)}, {Index: 2}},
		}
		assert.Equal(t, 3.0, questionMaxPoints(mb))

		tf := model.QuestionWithOptions{
			Question: model.Question{Format: "true_false", PointCorrect: 1,
				Statements: []model.QuestionStatement{{Index: 1}, {Index: 2, Points: fp(5)}}},
		}
		assert.Equal(t, 6.0, questionMaxPoints(tf))

		ma := model.QuestionWithOptions{
			Question: model.Question{Format: "multi_answer", PointCorrect: 1},
			Options: []model.QuestionOption{
				{Key: "a", IsCorrect: true},
				{Key: "b", IsCorrect: true, Points: fp(3)},
				{Key: "c", IsCorrect: false, Points: fp(9)}, // wrong option's worth never counts
			},
		}
		assert.Equal(t, 4.0, questionMaxPoints(ma))
	})
}
