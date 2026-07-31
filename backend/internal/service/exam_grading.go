package service

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// gradeAnswer determines correctness per format rules (FR20, R3).
// For mcq the correct key is derived from options (is_correct=true).
// For multi_answer the correct keys are derived from options.
// For short/fill_blank the answer must match any of acceptedAnswers under the
// confirmed FB-10 matching rule (FR-22/FR-23, see answer_match.go).
// For essay or unknown formats returns false (not auto-graded).
func gradeAnswer(format string, answer *string, acceptedAnswers []string, options []model.QuestionOption) bool {
	if answer == nil || *answer == "" {
		return false
	}

	switch format {
	case "mcq":
		return gradeMCQ(*answer, options)
	case "multi_answer":
		return gradeMultiAnswer(*answer, options)
	case "short", "fill_blank":
		return matchesAnyAccepted(*answer, acceptedAnswers)
	default:
		return false
	}
}

func gradeMCQ(answer string, options []model.QuestionOption) bool {
	var correctKey string
	for _, o := range options {
		if o.IsCorrect {
			correctKey = o.Key
			break
		}
	}
	if correctKey == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(correctKey))
}

func gradeMultiAnswer(answer string, options []model.QuestionOption) bool {
	selected := strings.Split(answer, ",")
	for i := range selected {
		selected[i] = strings.TrimSpace(selected[i])
	}
	sort.Strings(selected)

	var correct []string
	for _, o := range options {
		if o.IsCorrect {
			correct = append(correct, o.Key)
		}
	}
	sort.Strings(correct)

	if len(selected) != len(correct) {
		return false
	}
	for i := range selected {
		if selected[i] != correct[i] {
			return false
		}
	}
	return true
}

// gradeMultiBlank grades a multi_blank question by unmarshaling the answer
// string (JSON array of strings) and comparing each blank against its
// accepted-answer set under the confirmed FB-10 matching rule (FR-24, see
// answer_match.go).
// Per-blank scoring: empty -> 0, correct -> +point_correct, wrong -> -point_wrong.
// The question's score is the sum of per-blank scores (not independently clamped).
// On JSON unmarshal failure, returns 0 (all blanks treated as empty).
// IsCorrect is true only if all attempted blanks are correct (no wrong blanks).
func gradeMultiBlank(answer *string, blanks []model.QuestionBlank, pointCorrect, pointWrong float64) (float64, bool) {
	if answer == nil || *answer == "" {
		return 0, false
	}

	var answers []string
	if err := json.Unmarshal([]byte(*answer), &answers); err != nil {
		return 0, false
	}

	var score float64
	var hasAnswer bool
	var anyWrong bool
	for _, blank := range blanks {
		// Blank indices are 1-based; array positions are 0-based
		answerIdx := blank.Index - 1
		var blankAnswer string
		if answerIdx >= 0 && answerIdx < len(answers) {
			blankAnswer = strings.TrimSpace(answers[answerIdx])
		}

		if blankAnswer == "" {
			continue
		}

		hasAnswer = true
		if matchesAnyAccepted(blankAnswer, blank.AcceptedAnswers) {
			score += pointCorrect
		} else {
			score -= pointWrong
			anyWrong = true
		}
	}

	isCorrect := hasAnswer && !anyWrong
	return score, isCorrect
}

// gradeTrueFalse grades a true_false question by unmarshaling the answer string
// (JSON array of "true"/"false"/"" in statement index order) and comparing each
// statement's answered boolean against its is_true answer key — mirrors
// gradeMultiBlank exactly (FR-33/34).
// Per-statement scoring: empty -> 0, matches is_true -> +point_correct, mismatch
// -> -point_wrong. The question's score is the sum of per-statement scores (not
// independently clamped). On JSON unmarshal failure, returns 0 (all statements
// treated as empty). IsCorrect is true only if at least one statement was
// answered and none was wrong.
func gradeTrueFalse(answer *string, statements []model.QuestionStatement, pointCorrect, pointWrong float64) (float64, bool) {
	if answer == nil || *answer == "" {
		return 0, false
	}

	var answers []string
	if err := json.Unmarshal([]byte(*answer), &answers); err != nil {
		return 0, false
	}

	var score float64
	var hasAnswer bool
	var anyWrong bool
	for _, st := range statements {
		// Statement indices are 1-based; array positions are 0-based.
		idx := st.Index - 1
		var raw string
		if idx >= 0 && idx < len(answers) {
			raw = strings.TrimSpace(answers[idx])
		}

		// Only an explicit true/false counts. SaveAnswers stores these strings
		// opaquely, so a direct API call can submit any token; treating "not true"
		// as an explicit false credited garbage on every false statement.
		var answeredTrue bool
		switch {
		case strings.EqualFold(raw, "true"):
			answeredTrue = true
		case strings.EqualFold(raw, "false"):
			answeredTrue = false
		default:
			continue // "" and every malformed token are unanswered
		}

		hasAnswer = true
		if answeredTrue == st.IsTrue {
			score += pointCorrect
		} else {
			score -= pointWrong
			anyWrong = true
		}
	}

	isCorrect := hasAnswer && !anyWrong
	return score, isCorrect
}

// questionMaxPoints returns the maximum points a question can contribute: blank
// count x point_correct for multi_blank, statement count x point_correct for
// true_false, and point_correct otherwise (FR-35). Shared by topicBreakdown,
// certificate.go's maxScore loop and exam_leaderboard.go's maxPossible loop so
// the two multi-part formats cannot diverge.
func questionMaxPoints(q model.QuestionWithOptions) float64 {
	switch q.Question.Format {
	case "multi_blank":
		return float64(len(q.Blanks)) * q.Question.PointCorrect
	case "true_false":
		return float64(len(q.Question.Statements)) * q.Question.PointCorrect
	default:
		return q.Question.PointCorrect
	}
}

// gradeObjective grades all objective questions per the points model (FR-S5-06..10).
// Essay questions are returned with is_correct=nil, score=nil, graded_at=nil (not
// auto-graded — awaits manual grading).
//
// Correct answers earn +q.PointCorrect; wrong (non-empty) answers subtract q.PointWrong
// (a positive magnitude authored per question — the engine applies the sign); empty
// answers score 0 and are never penalized. Objective answers are stamped graded_at=now()
// since they are auto-graded at submit time. The returned total is clamped at 0 (FR-S5-10).
// Multi_blank questions score the sum of per-blank points (unclamped per-question).
func gradeObjective(questions []model.QuestionWithOptions, answers map[uuid.UUID]*string) ([]model.ExamSessionAnswer, float64) {
	now := time.Now()
	graded := make([]model.ExamSessionAnswer, 0, len(questions))
	var sum float64

	for _, q := range questions {
		ans := answers[q.Question.ID]

		if q.Question.Format == "essay" {
			graded = append(graded, model.ExamSessionAnswer{
				QuestionID: q.Question.ID,
				Answer:     ans,
			})
			continue
		}

		if q.Question.Format == "multi_blank" {
			score, anyCorrect := gradeMultiBlank(ans, q.Blanks, q.Question.PointCorrect, q.Question.PointWrong)
			sum += score

			gradedAt := now
			graded = append(graded, model.ExamSessionAnswer{
				QuestionID: q.Question.ID,
				Answer:     ans,
				IsCorrect:  &anyCorrect,
				Score:      &score,
				GradedAt:   &gradedAt,
			})
			continue
		}

		if q.Question.Format == "true_false" {
			score, anyCorrect := gradeTrueFalse(ans, q.Question.Statements, q.Question.PointCorrect, q.Question.PointWrong)
			sum += score

			gradedAt := now
			graded = append(graded, model.ExamSessionAnswer{
				QuestionID: q.Question.ID,
				Answer:     ans,
				IsCorrect:  &anyCorrect,
				Score:      &score,
				GradedAt:   &gradedAt,
			})
			continue
		}

		correct := gradeAnswer(q.Question.Format, ans, q.Question.AcceptedAnswers, q.Options)
		empty := ans == nil || *ans == ""

		var score float64
		switch {
		case correct:
			score = q.Question.PointCorrect
		case empty:
			score = 0
		default:
			score = -q.Question.PointWrong
		}
		sum += score

		isCorrect := correct
		gradedAt := now
		graded = append(graded, model.ExamSessionAnswer{
			QuestionID: q.Question.ID,
			Answer:     ans,
			IsCorrect:  &isCorrect,
			Score:      &score,
			GradedAt:   &gradedAt,
		})
	}

	return graded, clampScore(sum)
}

// clampScore floors a raw point sum at 0 (FR-S5-10, FR-S5-14): a session/topic score is
// never negative, however many wrong answers exceed the correct ones.
func clampScore(sum float64) float64 {
	return max(0, sum)
}

// computeSessionTotal folds persisted answer scores (objective + already-graded essays)
// and re-clamps the total (FR-S5-14). Used to recompute a session's score after an essay
// is (re-)graded; ungraded answers (Score == nil) contribute 0.
func computeSessionTotal(answers []model.ExamSessionAnswer) float64 {
	var sum float64
	for _, a := range answers {
		if a.Score != nil {
			sum += *a.Score
		}
	}
	return clampScore(sum)
}

// computeRank derives a 1-based rank from the count of sessions with a strictly higher
// score (FR-S5-18); ties share a rank since only strictly-higher sessions are counted.
func computeRank(higherCount int) int {
	return 1 + higherCount
}

// topicBreakdown builds one row per attached Test (FR-S5-19): earned sums the persisted
// answer scores for questions in that test, max sums point_correct across the test's
// questions (objective + essay).
func topicBreakdown(tests []model.TestDetail, answers []model.ExamSessionAnswer) []model.ResultTopicRow {
	scoreByQuestion := make(map[uuid.UUID]float64, len(answers))
	for _, a := range answers {
		if a.Score != nil {
			scoreByQuestion[a.QuestionID] = *a.Score
		}
	}

	rows := make([]model.ResultTopicRow, 0, len(tests))
	for _, td := range tests {
		var earned float64
		var max float64
		for _, q := range td.Questions {
			max += questionMaxPoints(q)
			earned += scoreByQuestion[q.Question.ID]
		}
		rows = append(rows, model.ResultTopicRow{
			TestID:      td.Test.ID,
			Title:       td.Test.Title,
			Subject:     td.Test.Subject,
			Topic:       td.Test.Topic,
			SectionType: td.Test.SectionType,
			Earned:      earned,
			Max:         max,
		})
	}
	return rows
}

// objectiveCounts tallies correct/wrong/empty over objective questions only (FR-S5-24);
// essay questions are excluded (they are reflected in the session score, not these counts).
func objectiveCounts(questions []model.QuestionWithOptions, answers []model.ExamSessionAnswer) (correct, wrong, empty int) {
	answerByQuestion := make(map[uuid.UUID]model.ExamSessionAnswer, len(answers))
	for _, a := range answers {
		answerByQuestion[a.QuestionID] = a
	}

	for _, q := range questions {
		if q.Question.Format == "essay" {
			continue
		}
		a, ok := answerByQuestion[q.Question.ID]
		if !ok || a.Answer == nil || *a.Answer == "" {
			empty++
			continue
		}
		if a.IsCorrect != nil && *a.IsCorrect {
			correct++
		} else {
			wrong++
		}
	}
	return correct, wrong, empty
}
