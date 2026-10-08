package service

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"akademi-bimbel/internal/model"
)

func floatPtr(f float64) *float64 { return &f }

func TestGrading_mcq_caseInsensitive(t *testing.T) {
	options := []model.QuestionOption{
		{Key: "a", Text: "Option A", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "Option B", SortOrder: 2},
	}

	tests := []struct {
		name   string
		answer *string
		want   bool
	}{
		{"exact match", strPtr("a"), true},
		{"case-insensitive", strPtr("A"), true},
		{"wrong answer", strPtr("b"), false},
		{"nil answer", nil, false},
		{"empty answer", strPtr(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeAnswer("mcq", tt.answer, nil, options)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGrading_mcq_trim(t *testing.T) {
	options := []model.QuestionOption{
		{Key: "a", Text: "Option A", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "Option B", SortOrder: 2},
	}

	got := gradeAnswer("mcq", strPtr("  a  "), nil, options)
	assert.True(t, got, "mcq should trim whitespace")
}

func TestGrading_mcq_noCorrectOption(t *testing.T) {
	got := gradeAnswer("mcq", strPtr("a"), nil, nil)
	assert.False(t, got, "mcq with no options should return false")
}

func TestGrading_multiAnswer_setEquality(t *testing.T) {
	options := []model.QuestionOption{
		{Key: "a", Text: "A", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "B", IsCorrect: true, SortOrder: 2},
		{Key: "c", Text: "C", IsCorrect: true, SortOrder: 3},
		{Key: "d", Text: "D", SortOrder: 4},
	}

	tests := []struct {
		name   string
		answer *string
		want   bool
	}{
		{"all correct order a,b,c", strPtr("a,b,c"), true},
		{"all correct order c,b,a", strPtr("c,b,a"), true},
		{"all correct with spaces", strPtr("  a , b , c  "), true},
		{"missing one option", strPtr("a,b"), false},
		{"extra option", strPtr("a,b,c,d"), false},
		{"empty answer", strPtr(""), false},
		{"nil answer", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeAnswer("multi_answer", tt.answer, nil, options)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGrading_short_trimLower(t *testing.T) {
	tests := []struct {
		name     string
		answer   *string
		accepted []string
		want     bool
	}{
		{"exact match", strPtr("hello"), []string{"hello"}, true},
		{"case-insensitive", strPtr("Hello"), []string{"hello"}, true},
		{"trimmed", strPtr("  hello  "), []string{"hello"}, true},
		{"wrong answer", strPtr("world"), []string{"hello"}, false},
		{"nil answer", nil, []string{"hello"}, false},
		{"empty answer", strPtr(""), []string{"hello"}, false},
		{"empty accepted set", strPtr("hello"), nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeAnswer("short", tt.answer, tt.accepted, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

// FR-22: a set of accepted answers, not a single scalar.
func TestGrading_short_multipleAcceptedAnswers(t *testing.T) {
	accepted := []string{"2", "dua"}

	assert.True(t, gradeAnswer("short", strPtr("2"), accepted, nil))
	assert.True(t, gradeAnswer("short", strPtr("dua"), accepted, nil))
	assert.False(t, gradeAnswer("short", strPtr("two"), accepted, nil))
}

func TestGrading_fillBlank_trimLower(t *testing.T) {
	tests := []struct {
		name     string
		answer   *string
		accepted []string
		want     bool
	}{
		{"exact match", strPtr("jakarta"), []string{"jakarta"}, true},
		{"case-insensitive", strPtr("Jakarta"), []string{"jakarta"}, true},
		{"wrong answer", strPtr("bandung"), []string{"jakarta"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeAnswer("fill_blank", tt.answer, tt.accepted, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGrading_essay_returnsFalse(t *testing.T) {
	got := gradeAnswer("essay", strPtr("some long essay"), nil, nil)
	assert.False(t, got, "essay should not be auto-graded")
}

func TestGrading_unknownFormat(t *testing.T) {
	got := gradeAnswer("unknown_format", strPtr("anything"), nil, nil)
	assert.False(t, got, "unknown format should return false")
}

// ---- gradeObjective: points model (FR-S5-06..10) ----

func mcqQuestion(id uuid.UUID, pointCorrect, pointWrong float64) model.QuestionWithOptions {
	return model.QuestionWithOptions{
		Question: model.Question{ID: id, Format: "mcq", Body: "2+2", PointCorrect: pointCorrect, PointWrong: pointWrong},
		Options: []model.QuestionOption{
			{Key: "a", Text: "3", SortOrder: 1},
			{Key: "b", Text: "4", IsCorrect: true, SortOrder: 2},
		},
	}
}

func TestGradingObjective_correct_earnsPointCorrect(t *testing.T) {
	q := mcqQuestion(uuid.New(), 2, 1)
	answers := map[uuid.UUID]*string{q.Question.ID: strPtr("b")}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 2.0, score)
	assert.Len(t, graded, 1)
	assert.True(t, *graded[0].IsCorrect)
	assert.Equal(t, 2.0, *graded[0].Score)
	assert.NotNil(t, graded[0].GradedAt, "objective answer should be stamped graded_at")
}

func TestGradingObjective_wrong_subtractsPointWrong(t *testing.T) {
	q := mcqQuestion(uuid.New(), 2, 1)
	answers := map[uuid.UUID]*string{q.Question.ID: strPtr("a")} // wrong option

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score, "session total is clamped even though the per-answer score is negative")
	assert.Len(t, graded, 1)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, -1.0, *graded[0].Score, "per-answer score stores the raw (unclamped) penalty")
	assert.NotNil(t, graded[0].GradedAt)
}

// FR-20/FR-21: fractional points score exactly, and the session total is still
// floored at 0 even though the per-answer penalty is negative.
func TestGradingObjective_fractionalPoints(t *testing.T) {
	correctQ := mcqQuestion(uuid.New(), 2.5, 0.5)
	correctAnswers := map[uuid.UUID]*string{correctQ.Question.ID: strPtr("b")}

	graded, score := gradeObjective([]model.QuestionWithOptions{correctQ}, correctAnswers)
	assert.Equal(t, 2.5, score)
	assert.Equal(t, 2.5, *graded[0].Score)

	wrongQ := mcqQuestion(uuid.New(), 2.5, 0.5)
	wrongAnswers := map[uuid.UUID]*string{wrongQ.Question.ID: strPtr("a")}

	graded, score = gradeObjective([]model.QuestionWithOptions{wrongQ}, wrongAnswers)
	assert.Equal(t, 0.0, score, "session total is floored at 0")
	assert.Equal(t, -0.5, *graded[0].Score, "per-answer score stores the raw penalty")
}

func TestGradingObjective_wrong_pointWrongZero_noPenalty(t *testing.T) {
	q := mcqQuestion(uuid.New(), 1, 0)
	answers := map[uuid.UUID]*string{q.Question.ID: strPtr("a")} // wrong option

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, 0.0, *graded[0].Score)
}

func TestGradingObjective_empty_scoresZero_neverPenalized(t *testing.T) {
	q := mcqQuestion(uuid.New(), 2, 1)
	answers := map[uuid.UUID]*string{} // unanswered

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score)
	assert.Len(t, graded, 1)
	assert.False(t, *graded[0].IsCorrect, "empty answer is not correct")
	assert.Equal(t, 0.0, *graded[0].Score)
	assert.NotNil(t, graded[0].GradedAt, "empty objective answer is still auto-graded now")
}

func TestGradingObjective_essay_leftUngraded(t *testing.T) {
	qEssay := model.Question{ID: uuid.New(), Format: "essay", Body: "Write about X"}
	questions := []model.QuestionWithOptions{{Question: qEssay}}
	answers := map[uuid.UUID]*string{qEssay.ID: strPtr("My essay")}

	graded, score := gradeObjective(questions, answers)

	assert.Equal(t, 0.0, score)
	assert.Len(t, graded, 1)
	assert.Nil(t, graded[0].IsCorrect, "essay IsCorrect should be nil")
	assert.Nil(t, graded[0].Score, "essay Score should be nil")
	assert.Nil(t, graded[0].GradedAt, "essay GradedAt should be nil until manually graded")
	assert.Equal(t, qEssay.ID, graded[0].QuestionID)
	assert.NotNil(t, graded[0].Answer)
}

func TestGradingObjective_worked_threeCorrect(t *testing.T) {
	// spec worked example: 3 correct, 0 wrong, 0 empty (pc=2, pw=1) -> score 6
	var questions []model.QuestionWithOptions
	answers := map[uuid.UUID]*string{}
	for i := 0; i < 3; i++ {
		q := mcqQuestion(uuid.New(), 2, 1)
		questions = append(questions, q)
		answers[q.Question.ID] = strPtr("b")
	}

	_, score := gradeObjective(questions, answers)

	assert.Equal(t, 6.0, score)
}

func TestGradingObjective_worked_clampedAtZero(t *testing.T) {
	// spec worked example: 1 correct, 4 wrong (pc=2, pw=1) -> sum -2 -> clamp 0
	var questions []model.QuestionWithOptions
	answers := map[uuid.UUID]*string{}

	correctQ := mcqQuestion(uuid.New(), 2, 1)
	questions = append(questions, correctQ)
	answers[correctQ.Question.ID] = strPtr("b")

	for i := 0; i < 4; i++ {
		q := mcqQuestion(uuid.New(), 2, 1)
		questions = append(questions, q)
		answers[q.Question.ID] = strPtr("a") // wrong
	}

	_, score := gradeObjective(questions, answers)

	assert.Equal(t, 0.0, score)
}

func TestGradingObjective_worked_allEmpty(t *testing.T) {
	// spec worked example: 0 correct, 0 wrong, 5 empty -> score 0
	var questions []model.QuestionWithOptions
	for i := 0; i < 5; i++ {
		questions = append(questions, mcqQuestion(uuid.New(), 2, 1))
	}

	_, score := gradeObjective(questions, map[uuid.UUID]*string{})

	assert.Equal(t, 0.0, score)
}

func TestGradingObjective_worked_pointWrongZero(t *testing.T) {
	// spec worked example: 2 correct (pc=1), 1 wrong (pw=0) -> score 2
	var questions []model.QuestionWithOptions
	answers := map[uuid.UUID]*string{}

	for i := 0; i < 2; i++ {
		q := mcqQuestion(uuid.New(), 1, 0)
		questions = append(questions, q)
		answers[q.Question.ID] = strPtr("b")
	}
	wrongQ := mcqQuestion(uuid.New(), 1, 0)
	questions = append(questions, wrongQ)
	answers[wrongQ.Question.ID] = strPtr("a")

	_, score := gradeObjective(questions, answers)

	assert.Equal(t, 2.0, score)
}

func TestGradingObjective_multiAnswerCorrect(t *testing.T) {
	qID := uuid.New()
	questions := []model.QuestionWithOptions{
		{
			Question: model.Question{ID: qID, Format: "multi_answer", Body: "Select A B C", PointCorrect: 1, PointWrong: 0},
			Options: []model.QuestionOption{
				{Key: "a", Text: "A", IsCorrect: true, SortOrder: 1},
				{Key: "b", Text: "B", IsCorrect: true, SortOrder: 2},
				{Key: "c", Text: "C", IsCorrect: true, SortOrder: 3},
				{Key: "d", Text: "D", SortOrder: 4},
			},
		},
	}
	answers := map[uuid.UUID]*string{
		qID: strPtr("c,a,b"),
	}

	graded, score := gradeObjective(questions, answers)

	// Partial credit per selected option (2026-07-31): three correct
	// selections at point_correct=1 score 3, no longer all-or-nothing 1.
	assert.Equal(t, 3.0, score)
	assert.Len(t, graded, 1)
	assert.True(t, *graded[0].IsCorrect)
}

func TestGradeMultiAnswerPartial_pinsTheRule(t *testing.T) {
	options := []model.QuestionOption{
		{Key: "a", IsCorrect: true},
		{Key: "b", IsCorrect: true},
		{Key: "c", IsCorrect: false},
		{Key: "d", IsCorrect: false},
	}

	cases := []struct {
		name      string
		answer    string
		wantScore float64
		wantOK    bool
	}{
		{"one of two correct earns one point, still counts correct", "a", 1, true},
		{"both correct earns both points", "a,b", 2, true},
		{"a wrong selection costs point_wrong and kills is_correct", "a,c", 0.5, false},
		{"selecting everything cannot farm points", "a,b,c,d", 1, false},
		{"only wrong selections go negative (session floor handles it)", "c,d", -1, false},
		{"duplicate keys are counted once", "a,a,a", 1, true},
		{"unknown key is a wrong selection (tampered payload)", "a,zz", 0.5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, ok := gradeMultiAnswerPartial(tc.answer, options, 1, 0.5)
			assert.Equal(t, tc.wantScore, score)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestQuestionMaxPoints_multiAnswerCountsCorrectOptions(t *testing.T) {
	q := model.QuestionWithOptions{
		Question: model.Question{Format: "multi_answer", PointCorrect: 2},
		Options: []model.QuestionOption{
			{Key: "a", IsCorrect: true},
			{Key: "b", IsCorrect: true},
			{Key: "c", IsCorrect: false},
		},
	}
	assert.Equal(t, 4.0, questionMaxPoints(q))
}

func TestGradingObjective_noQuestions_scoreZero(t *testing.T) {
	graded, score := gradeObjective([]model.QuestionWithOptions{}, map[uuid.UUID]*string{})

	assert.Equal(t, 0.0, score)
	assert.Empty(t, graded)
}

// ---- clampScore (FR-S5-10) ----

func TestClampScore_positive_unchanged(t *testing.T) {
	assert.Equal(t, 6.0, clampScore(6.0))
}

func TestClampScore_negative_clampedToZero(t *testing.T) {
	assert.Equal(t, 0.0, clampScore(-2.0))
}

func TestClampScore_zero_unchanged(t *testing.T) {
	assert.Equal(t, 0.0, clampScore(0.0))
}

// ---- computeSessionTotal (FR-S5-14) ----

func TestComputeSessionTotal_foldsPersistedScores(t *testing.T) {
	answers := []model.ExamSessionAnswer{
		{QuestionID: uuid.New(), Score: floatPtr(2)},
		{QuestionID: uuid.New(), Score: floatPtr(3)},
		{QuestionID: uuid.New(), Score: nil}, // ungraded essay
	}

	assert.Equal(t, 5.0, computeSessionTotal(answers))
}

func TestComputeSessionTotal_clampsNegativeSum(t *testing.T) {
	answers := []model.ExamSessionAnswer{
		{QuestionID: uuid.New(), Score: floatPtr(1)},
		{QuestionID: uuid.New(), Score: floatPtr(-5)},
	}

	assert.Equal(t, 0.0, computeSessionTotal(answers))
}

func TestComputeSessionTotal_idempotent(t *testing.T) {
	answers := []model.ExamSessionAnswer{
		{QuestionID: uuid.New(), Score: floatPtr(4)},
		{QuestionID: uuid.New(), Score: floatPtr(1)},
	}

	first := computeSessionTotal(answers)
	second := computeSessionTotal(answers)

	assert.Equal(t, first, second)
}

// ---- computeRank (FR-S5-18) ----

func TestComputeRank_zeroHigher_rankOne(t *testing.T) {
	assert.Equal(t, 1, computeRank(0))
}

func TestComputeRank_higherCount_addsOne(t *testing.T) {
	assert.Equal(t, 4, computeRank(3))
}

// ---- topicBreakdown (FR-S5-19) ----

func TestTopicBreakdown_onePerTest_earnedAndMax(t *testing.T) {
	testID := uuid.New()
	q1 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 2, PointWrong: 1}
	q2 := model.Question{ID: uuid.New(), Format: "essay", PointCorrect: 5, PointWrong: 0}
	tests := []model.TestDetail{
		{
			Test: model.Test{ID: testID, Title: "Math", Subject: "Math", Topic: "Algebra"},
			Questions: []model.QuestionWithOptions{
				{Question: q1, SortOrder: 1},
				{Question: q2, SortOrder: 2},
			},
		},
	}
	answers := []model.ExamSessionAnswer{
		{QuestionID: q1.ID, Score: floatPtr(2)},
		{QuestionID: q2.ID, Score: floatPtr(3)},
	}

	rows := topicBreakdown(tests, answers)

	assert.Len(t, rows, 1)
	assert.Equal(t, testID, rows[0].TestID)
	assert.Equal(t, "Math", rows[0].Title)
	assert.Equal(t, "Math", rows[0].Subject)
	assert.Equal(t, "Algebra", rows[0].Topic)
	assert.Equal(t, 5.0, rows[0].Earned, "earned sums answer scores within the test")
	assert.Equal(t, 7.0, rows[0].Max, "max sums point_correct across objective + essay")
}

func TestTopicBreakdown_ungradedEssay_earnedZeroContribution(t *testing.T) {
	testID := uuid.New()
	qEssay := model.Question{ID: uuid.New(), Format: "essay", PointCorrect: 5, PointWrong: 0}
	tests := []model.TestDetail{
		{
			Test:      model.Test{ID: testID, Title: "Essay Test", Subject: "Bahasa", Topic: "Writing"},
			Questions: []model.QuestionWithOptions{{Question: qEssay, SortOrder: 1}},
		},
	}
	answers := []model.ExamSessionAnswer{
		{QuestionID: qEssay.ID, Score: nil}, // not yet graded
	}

	rows := topicBreakdown(tests, answers)

	assert.Len(t, rows, 1)
	assert.Equal(t, 0.0, rows[0].Earned)
	assert.Equal(t, 5.0, rows[0].Max)
}

func TestTopicBreakdown_sectionType_populatedForIelts(t *testing.T) {
	listening := "listening"
	reading := "reading"
	writing := "writing"

	test1 := uuid.New()
	test2 := uuid.New()
	test3 := uuid.New()
	q1 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 1, PointWrong: 0}
	q2 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 1, PointWrong: 0}
	q3 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 1, PointWrong: 0}
	tests := []model.TestDetail{
		{Test: model.Test{ID: test1, Title: "Listening", Subject: "EN", Topic: "Listening", SectionType: &listening}, Questions: []model.QuestionWithOptions{{Question: q1, SortOrder: 1}}},
		{Test: model.Test{ID: test2, Title: "Reading", Subject: "EN", Topic: "Reading", SectionType: &reading}, Questions: []model.QuestionWithOptions{{Question: q2, SortOrder: 1}}},
		{Test: model.Test{ID: test3, Title: "Writing", Subject: "EN", Topic: "Writing", SectionType: &writing}, Questions: []model.QuestionWithOptions{{Question: q3, SortOrder: 1}}},
	}
	answers := []model.ExamSessionAnswer{
		{QuestionID: q1.ID, Score: floatPtr(1)},
		{QuestionID: q2.ID, Score: floatPtr(0)},
		{QuestionID: q3.ID, Score: floatPtr(1)},
	}

	rows := topicBreakdown(tests, answers)

	assert.Len(t, rows, 3)
	assert.NotNil(t, rows[0].SectionType, "section_type should be populated, not nil")
	assert.Equal(t, listening, *rows[0].SectionType)
	assert.Equal(t, reading, *rows[1].SectionType)
	assert.Equal(t, writing, *rows[2].SectionType)
}

func TestTopicBreakdown_multipleTests_oneRowEach(t *testing.T) {
	test1 := uuid.New()
	test2 := uuid.New()
	q1 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 1, PointWrong: 0}
	q2 := model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 1, PointWrong: 0}
	tests := []model.TestDetail{
		{Test: model.Test{ID: test1, Title: "T1", Subject: "S1", Topic: "Top1"}, Questions: []model.QuestionWithOptions{{Question: q1, SortOrder: 1}}},
		{Test: model.Test{ID: test2, Title: "T2", Subject: "S2", Topic: "Top2"}, Questions: []model.QuestionWithOptions{{Question: q2, SortOrder: 1}}},
	}
	answers := []model.ExamSessionAnswer{
		{QuestionID: q1.ID, Score: floatPtr(1)},
		{QuestionID: q2.ID, Score: floatPtr(0)},
	}

	rows := topicBreakdown(tests, answers)

	assert.Len(t, rows, 2)
}

// ---- objectiveCounts (FR-S5-24) ----

func TestObjectiveCounts_correctWrongEmpty(t *testing.T) {
	qCorrect := model.Question{ID: uuid.New(), Format: "mcq"}
	qWrong := model.Question{ID: uuid.New(), Format: "mcq"}
	qEmpty := model.Question{ID: uuid.New(), Format: "mcq"}
	qEssay := model.Question{ID: uuid.New(), Format: "essay"}
	questions := []model.QuestionWithOptions{
		{Question: qCorrect}, {Question: qWrong}, {Question: qEmpty}, {Question: qEssay},
	}

	trueVal := true
	falseVal := false
	answers := []model.ExamSessionAnswer{
		{QuestionID: qCorrect.ID, Answer: strPtr("b"), IsCorrect: &trueVal},
		{QuestionID: qWrong.ID, Answer: strPtr("a"), IsCorrect: &falseVal},
		{QuestionID: qEmpty.ID, Answer: nil, IsCorrect: &falseVal},
		{QuestionID: qEssay.ID, Answer: strPtr("essay text")}, // essay excluded from counts
	}

	correct, wrong, empty := objectiveCounts(questions, answers)

	assert.Equal(t, 1, correct)
	assert.Equal(t, 1, wrong)
	assert.Equal(t, 1, empty)
}

func TestObjectiveCounts_emptyStringAnswer_countsAsEmpty(t *testing.T) {
	q := model.Question{ID: uuid.New(), Format: "short"}
	questions := []model.QuestionWithOptions{{Question: q}}
	falseVal := false
	answers := []model.ExamSessionAnswer{
		{QuestionID: q.ID, Answer: strPtr(""), IsCorrect: &falseVal},
	}

	correct, wrong, empty := objectiveCounts(questions, answers)

	assert.Equal(t, 0, correct)
	assert.Equal(t, 0, wrong)
	assert.Equal(t, 1, empty)
}

func TestObjectiveCounts_missingAnswerRow_countsAsEmpty(t *testing.T) {
	q := model.Question{ID: uuid.New(), Format: "mcq"}
	questions := []model.QuestionWithOptions{{Question: q}}

	correct, wrong, empty := objectiveCounts(questions, []model.ExamSessionAnswer{})

	assert.Equal(t, 0, correct)
	assert.Equal(t, 0, wrong)
	assert.Equal(t, 1, empty)
}

// ---- gradeMultiBlank (FR-20/21/22/24) ----

func multiBlankQuestion(id uuid.UUID, pointCorrect, pointWrong float64, correctAnswers []string) model.QuestionWithOptions {
	blanks := make([]model.QuestionBlank, len(correctAnswers))
	for i, ans := range correctAnswers {
		blanks[i] = model.QuestionBlank{
			QuestionID:      id,
			Index:           i + 1,
			CorrectAnswer:   ans,
			AcceptedAnswers: []string{ans},
		}
	}
	return model.QuestionWithOptions{
		Question: model.Question{ID: id, Format: "multi_blank", Body: "Stem", PointCorrect: pointCorrect, PointWrong: pointWrong},
		Blanks:   blanks,
	}
}

func TestGradingMultiBlank_correctAndEmpty_FR20(t *testing.T) {
	// FR-20: blank 1 correct, blank 2 empty -> +point_correct + 0 = 2
	q := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	answer := `["jakarta"]` // only blank 1 answered correctly
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 2.0, score)
	assert.Len(t, graded, 1)
	assert.True(t, *graded[0].IsCorrect)
	assert.Equal(t, 2.0, *graded[0].Score)
}

func TestGradingMultiBlank_correctAndWrong_FR21(t *testing.T) {
	// FR-21: blank 1 correct, blank 2 wrong -> +point_correct - point_wrong = 2-1 = 1
	q := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	answer := `["jakarta","1946"]` // blank 1 correct, blank 2 wrong
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 1.0, score)
	assert.Len(t, graded, 1)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, 1.0, *graded[0].Score)
}

func TestGradingMultiBlank_allWrong_unclamped_FR22(t *testing.T) {
	// FR-22: both blanks wrong -> -1 - 1 = -2, not clamped per-question, only session-level
	q := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	answer := `["bandung","1946"]` // both wrong
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score, "session total is clamped to 0")
	assert.Len(t, graded, 1)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, -2.0, *graded[0].Score, "per-question score is -2 (unclamped)")
}

func TestGradingMultiBlank_malformedJSON_FR24(t *testing.T) {
	// FR-24: malformed JSON -> treat all blanks as empty -> score 0
	q := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	answer := `invalid-json`
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score)
	assert.Len(t, graded, 1)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, 0.0, *graded[0].Score)
}

func TestGradingMultiBlank_caseInsensitiveTrim(t *testing.T) {
	// Same comparator as fill_blank: case-insensitive and trimmed
	q := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	answer := `["  JAKARTA  ","  1945  "]`
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 4.0, score)
	assert.True(t, *graded[0].IsCorrect)
}

// FR-24: a multi_blank blank with its own accepted-answer set awards
// +point_correct for an alternate accepted spelling and -point_wrong for a
// wrong one.
func TestGradingMultiBlank_perBlankAcceptedAnswerSet_FR24(t *testing.T) {
	id := uuid.New()
	blanks := []model.QuestionBlank{
		{QuestionID: id, Index: 1, CorrectAnswer: "4", AcceptedAnswers: []string{"4", "empat"}},
		{QuestionID: id, Index: 2, CorrectAnswer: "4", AcceptedAnswers: []string{"4", "empat"}},
	}
	q := model.QuestionWithOptions{
		Question: model.Question{ID: id, Format: "multi_blank", Body: "Stem", PointCorrect: 1, PointWrong: 0.5},
		Blanks:   blanks,
	}

	answer := `["empat","four"]` // blank 1 alternate accepted spelling, blank 2 wrong
	answers := map[uuid.UUID]*string{id: &answer}

	graded, _ := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Len(t, graded, 1)
	assert.Equal(t, 0.5, *graded[0].Score, "empat (+1) minus four (-0.5) = 0.5")
	assert.False(t, *graded[0].IsCorrect, "one blank was wrong")
}

func TestGradingObjective_multiBlankMixed_withOtherFormats(t *testing.T) {
	// Full-session test: multi_blank + mcq + short to verify sum and session clamping
	multiBlankQ := multiBlankQuestion(uuid.New(), 2, 1, []string{"jakarta", "1945"})
	mcqQ := mcqQuestion(uuid.New(), 2, 1)
	shortQ := model.QuestionWithOptions{
		Question: model.Question{ID: uuid.New(), Format: "short", Body: "Q3", PointCorrect: 2, PointWrong: 1, CorrectAnswer: strPtr("answer"), AcceptedAnswers: []string{"answer"}},
	}

	answers := map[uuid.UUID]*string{
		multiBlankQ.Question.ID: strPtr(`["jakarta","1945"]`), // both correct: +2+2 = 4
		mcqQ.Question.ID:        strPtr("b"),                   // correct: +2
		shortQ.Question.ID:      strPtr("answer"),              // correct: +2
	}

	graded, score := gradeObjective(
		[]model.QuestionWithOptions{multiBlankQ, mcqQ, shortQ},
		answers,
	)

	assert.Equal(t, 8.0, score)
	assert.Len(t, graded, 3)
	assert.True(t, *graded[0].IsCorrect)  // multi_blank both correct
	assert.Equal(t, 4.0, *graded[0].Score)
	assert.True(t, *graded[1].IsCorrect)  // mcq correct
	assert.Equal(t, 2.0, *graded[1].Score)
	assert.True(t, *graded[2].IsCorrect)  // short correct
	assert.Equal(t, 2.0, *graded[2].Score)
}

// ---- gradeTrueFalse (FR-33/34) ----

func tfQuestion(id uuid.UUID, pointCorrect, pointWrong float64, truths []bool) model.QuestionWithOptions {
	statements := make([]model.QuestionStatement, len(truths))
	for i, isTrue := range truths {
		statements[i] = model.QuestionStatement{QuestionID: id, Index: i + 1, Body: fmt.Sprintf("statement %d", i+1), IsTrue: isTrue}
	}
	return model.QuestionWithOptions{
		Question: model.Question{ID: id, Format: "true_false", Body: "Stem", PointCorrect: pointCorrect, PointWrong: pointWrong, Statements: statements},
	}
}

// FR-33: partial per-statement scoring reproduces spec.md's worked example
// (point_correct=1, point_wrong=0.5 -> "1+1+1-0.5=2.5") for 3 correct statements
// and 1 wrong statement. Because point_correct=1 is a whole number, a .5
// fractional score is only reachable with at least one wrong (-0.5)
// contribution — which under FR-34 always makes is_correct false.
func TestGradingTrueFalse_partialScoring_threeCorrectOneWrong_FR33(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false, true, false})
	answer := `["true","false","true","true"]` // statement 4: truth false, answered true -> wrong
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 2.5, score)
	assert.Len(t, graded, 1)
	assert.Equal(t, 2.5, *graded[0].Score)
	assert.False(t, *graded[0].IsCorrect, "is_correct must be false when any statement is wrong")
}

// FR-33/34: statements 1 and 3 answered true, 2 answered false, 4 left blank
// (spec.md FR-32's encoding). All three non-empty statements match their
// is_true answer key, so the question scores 3 (3 x point_correct, the blank
// contributes 0) and is_correct is true (answered something, nothing wrong).
func TestGradingTrueFalse_allAnsweredCorrectAndOneEmpty_isCorrectTrue_FR34(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false, true, false})
	answer := `["true","false","true",""]`
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 3.0, score)
	assert.Len(t, graded, 1)
	assert.Equal(t, 3.0, *graded[0].Score)
	assert.True(t, *graded[0].IsCorrect)
}

func TestGradingTrueFalse_allEmpty_scoresZero_isCorrectFalse(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false})
	answer := `["",""]`
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, 0.0, *graded[0].Score)
}

func TestGradingTrueFalse_malformedJSON_treatsAllEmpty(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false})
	answer := `not-json`
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score)
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, 0.0, *graded[0].Score)
}

// Scoring integrity: SaveAnswers stores answer strings opaquely — it validates ids and
// section membership, never content — so a direct API submission can carry arbitrary
// tokens. Treating "anything that isn't true" as an explicit false handed full credit
// to garbage on every false statement. Only explicit true/false may score; every other
// token is unanswered, exactly like "".
func TestGradingTrueFalse_malformedTokens_scoreZero_notCredited(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{false, false})
	answer := `["garbage","garbage"]` // both statements are false — the exploitable case
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score, "malformed tokens must not earn credit")
	assert.Equal(t, 0.0, *graded[0].Score)
	assert.False(t, *graded[0].IsCorrect, "malformed tokens are unanswered, not correct")
}

func TestGradingTrueFalse_malformedTokenIsUnanswered_notWrong(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, true})
	answer := `["true","garbage"]` // statement 2 malformed -> unanswered, not a wrong answer
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 1.0, score, "no point_wrong deduction for an unanswered statement")
	assert.Equal(t, 1.0, *graded[0].Score)
	assert.True(t, *graded[0].IsCorrect, "one correct, one unanswered, nothing wrong")
}

func TestGradingTrueFalse_explicitFalseStillScores(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{false, false})
	answer := `["false","FALSE"]` // case-insensitive, both explicit
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 2.0, score)
	assert.True(t, *graded[0].IsCorrect)
}

func TestGradingTrueFalse_allWrong_unclamped(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false})
	answer := `["false","true"]` // both wrong
	answers := map[uuid.UUID]*string{q.Question.ID: &answer}

	graded, score := gradeObjective([]model.QuestionWithOptions{q}, answers)

	assert.Equal(t, 0.0, score, "session total is clamped to 0")
	assert.False(t, *graded[0].IsCorrect)
	assert.Equal(t, -1.0, *graded[0].Score, "per-question score is -1 (unclamped)")
}

// ---- questionMaxPoints (FR-35) ----

func TestQuestionMaxPoints_trueFalse_statementCountTimesPointCorrect(t *testing.T) {
	q := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false, true, false})
	assert.Equal(t, 4.0, questionMaxPoints(q))
}

func TestQuestionMaxPoints_multiBlank_blankCountTimesPointCorrect(t *testing.T) {
	q := multiBlankQuestion(uuid.New(), 1, 0.5, []string{"a", "b", "c"})
	assert.Equal(t, 3.0, questionMaxPoints(q))
}

func TestQuestionMaxPoints_otherFormats_isPointCorrect(t *testing.T) {
	q := model.QuestionWithOptions{Question: model.Question{Format: "mcq", PointCorrect: 2}}
	assert.Equal(t, 2.0, questionMaxPoints(q))
}

// FR-35: a true_false question's contribution to the result breakdown max is its
// statement count x point_correct, not a flat point_correct — proven through the
// same topicBreakdown function used by GetSessionResult, alongside a multi_blank
// question so the two multi-part formats are shown not to diverge.
func TestTopicBreakdown_trueFalseAndMultiBlank_maxUsesSharedHelper_FR35(t *testing.T) {
	testID := uuid.New()
	tfQ := tfQuestion(uuid.New(), 1, 0.5, []bool{true, false, true, false})
	mbQ := multiBlankQuestion(uuid.New(), 1, 0.5, []string{"a", "b", "c"})
	tests := []model.TestDetail{
		{
			Test: model.Test{ID: testID, Title: "Mixed", Subject: "Math", Topic: "Mixed"},
			Questions: []model.QuestionWithOptions{
				{Question: tfQ.Question, SortOrder: 1},
				{Question: mbQ.Question, Blanks: mbQ.Blanks, SortOrder: 2},
			},
		},
	}
	answers := []model.ExamSessionAnswer{}

	rows := topicBreakdown(tests, answers)

	assert.Len(t, rows, 1)
	assert.Equal(t, 7.0, rows[0].Max, "true_false (4 statements) + multi_blank (3 blanks) at point_correct=1 each")
}
