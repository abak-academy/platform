package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- fakeSessionRepo: grading/result extensions ----------

func (f *fakeSessionRepo) seedTests(examID uuid.UUID, tests []model.TestDetail) {
	f.testsByExam[examID] = tests
}

func (f *fakeSessionRepo) seedEssays(sessionID uuid.UUID, items []model.GradingEssayItem) {
	f.essays[sessionID] = items
}

func (f *fakeSessionRepo) GetSessionEssayAnswers(_ context.Context, sessionID uuid.UUID) ([]model.GradingEssayItem, error) {
	return f.essays[sessionID], nil
}

func (f *fakeSessionRepo) ListSessionsNeedingGrading(_ context.Context, examID uuid.UUID) ([]model.GradingSessionItem, error) {
	return f.gradingQueue[examID], nil
}

func (f *fakeSessionRepo) fullyGraded(sessionID uuid.UUID) bool {
	for _, e := range f.essays[sessionID] {
		if e.GradedAt == nil {
			return false
		}
	}
	return true
}

func (f *fakeSessionRepo) CountHigherScores(_ context.Context, examID uuid.UUID, score float64) (int, error) {
	count := 0
	for _, s := range f.sessions {
		if s.ExamID != examID || s.Status != "submitted" || s.Score == nil || *s.Score <= score {
			continue
		}
		if !f.fullyGraded(s.ID) {
			continue
		}
		count++
	}
	return count, nil
}

// GradeEssayAnswer is the fake's single-call, non-tx stand-in for the real
// GradeEssayAnswerTx + UpdateSessionScoreTx pair — mirrors the SubmitSession shim
// convention already used in this file (the atomic write is faked as one call).
func (f *fakeSessionRepo) GradeEssayAnswer(_ context.Context, sessionID, questionID uuid.UUID, score float64, comment *string, gradedBy uuid.UUID) (float64, error) {
	items, ok := f.essays[sessionID]
	if !ok {
		return 0, repository.ErrNotFound
	}
	found := false
	now := time.Now()
	for i := range items {
		if items[i].QuestionID == questionID {
			s := score
			items[i].Score = &s
			items[i].GraderComment = comment
			items[i].GradedAt = &now
			found = true
			break
		}
	}
	if !found {
		return 0, repository.ErrNotFound
	}
	f.essays[sessionID] = items

	answers := f.sessionAnswers[sessionID]
	for i := range answers {
		if answers[i].QuestionID == questionID {
			s := score
			answers[i].Score = &s
			answers[i].GradedAt = &now
			answers[i].GradedBy = &gradedBy
			answers[i].GraderComment = comment
		}
	}
	f.sessionAnswers[sessionID] = answers

	total := computeSessionTotal(answers)
	if sess, ok := f.sessions[sessionID]; ok {
		sess.Score = &total
	}
	return total, nil
}

// ---------- shimSessionService: GetSessionResult ----------

func (s *shimSessionService) GetSessionResult(ctx context.Context, studentID, sessionID string) (model.SessionResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return model.SessionResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return model.SessionResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.SessionResult{}, ErrSessionNotFound
		}
		return model.SessionResult{}, err
	}

	exam, err := s.repo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return model.SessionResult{}, err
	}

	tests, err := s.repo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return model.SessionResult{}, err
	}
	answers, err := s.repo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return model.SessionResult{}, err
	}

	var qs []model.QuestionWithOptions
	for _, td := range tests {
		qs = append(qs, td.Questions...)
	}

	if gated, ok := resultGate(*exam, sess.Status == "submitted", isFullyGraded(qs, answers)); ok {
		return gated, nil
	}

	score := 0.0
	if sess.Score != nil {
		score = *sess.Score
	}
	higherCount, err := s.repo.CountHigherScores(ctx, sess.ExamID, score)
	if err != nil {
		return model.SessionResult{}, err
	}
	correct, wrong, empty := objectiveCounts(qs, answers)

	result := model.SessionResult{
		State:        "result",
		ResultConfig: exam.ResultConfig,
		Score:        score,
		CorrectCount: correct,
		WrongCount:   wrong,
		EmptyCount:   empty,
		Rank:         computeRank(higherCount),
	}

	if exam.ResultConfig == "score_pembahasan" {
		result.Breakdown = topicBreakdown(tests, answers)
		result.Pembahasan = buildPembahasan(tests, answers)
	}

	return result, nil
}

// ---------- shimSessionService: admin grading ----------

func (s *shimSessionService) ListGradingSessions(ctx context.Context, examID uuid.UUID) ([]model.GradingSessionItem, error) {
	return s.repo.ListSessionsNeedingGrading(ctx, examID)
}

func (s *shimSessionService) GetSessionEssays(ctx context.Context, sessionID uuid.UUID) ([]model.GradingEssayItem, error) {
	return s.repo.GetSessionEssayAnswers(ctx, sessionID)
}

func (s *shimSessionService) GradeEssayAnswer(ctx context.Context, sessionID, questionID uuid.UUID, score float64, comment *string, gradedBy uuid.UUID) (float64, error) {
	essays, err := s.repo.GetSessionEssayAnswers(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	var target *model.GradingEssayItem
	for i := range essays {
		if essays[i].QuestionID == questionID {
			target = &essays[i]
			break
		}
	}
	if target == nil {
		return 0, ErrNotEssayQuestion
	}
	if err := validateGrade(score, target.PointCorrect); err != nil {
		return 0, err
	}

	return s.repo.GradeEssayAnswer(ctx, sessionID, questionID, score, comment, gradedBy)
}

// ---------- fixtures ----------

func mcqTest(testID, examID uuid.UUID) model.TestDetail {
	q := model.Question{ID: uuid.New(), Format: "mcq", Body: "2+2", PointCorrect: 2, PointWrong: 1}
	return model.TestDetail{
		Test: model.Test{ID: testID, Title: "Math", Subject: "Math", Topic: "Algebra"},
		Questions: []model.QuestionWithOptions{
			{
				Question:  q,
				SortOrder: 1,
				Options: []model.QuestionOption{
					{Key: "a", Text: "3", SortOrder: 1},
					{Key: "b", Text: "4", IsCorrect: true, SortOrder: 2},
				},
			},
		},
	}
}

func essayTest(testID uuid.UUID, pointCorrect float64) (model.TestDetail, uuid.UUID) {
	qID := uuid.New()
	q := model.Question{ID: qID, Format: "essay", Body: "Explain X", PointCorrect: pointCorrect, PointWrong: 0}
	return model.TestDetail{
		Test:      model.Test{ID: testID, Title: "Essay", Subject: "Bahasa", Topic: "Writing"},
		Questions: []model.QuestionWithOptions{{Question: q, SortOrder: 1}},
	}, qID
}

// ---------- GetSessionResult ----------

func TestGetSessionResult_Hidden(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "hidden"})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted"}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "hidden" {
		t.Errorf("state: want hidden, got %q", result.State)
	}
}

func TestGetSessionResult_Grading_UngradedEssay(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	testID := uuid.New()
	td, qID := essayTest(testID, 5)
	svc.repo.seedTests(examID, []model.TestDetail{td})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(0)}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my essay"), GradedAt: nil},
	}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "grading" {
		t.Errorf("state: want grading, got %q", result.State)
	}
}

func TestGetSessionResult_Grading_NotSubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "in_progress"}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "grading" {
		t.Errorf("state: want grading (not yet submitted), got %q", result.State)
	}
}

func TestGetSessionResult_Locked(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	releaseAt := time.Now().Add(24 * time.Hour)
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only", ResultReleaseAt: &releaseAt})

	testID := uuid.New()
	svc.repo.seedTests(examID, []model.TestDetail{mcqTest(testID, examID)})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(2)}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "locked" {
		t.Errorf("state: want locked, got %q", result.State)
	}
	if result.ResultReleaseAt == nil || !result.ResultReleaseAt.Equal(releaseAt) {
		t.Errorf("result_release_at mismatch: got %v", result.ResultReleaseAt)
	}
}

func TestGetSessionResult_ScoreOnly_NoBreakdown(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	testID := uuid.New()
	td := mcqTest(testID, examID)
	qID := td.Questions[0].Question.ID
	svc.repo.seedTests(examID, []model.TestDetail{td})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(2)}
	trueVal := true
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: floatPtr(2), GradedAt: timePtr(time.Now())},
	}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "result" {
		t.Fatalf("state: want result, got %q", result.State)
	}
	if result.Score != 2 {
		t.Errorf("score: want 2, got %v", result.Score)
	}
	if result.CorrectCount != 1 || result.WrongCount != 0 || result.EmptyCount != 0 {
		t.Errorf("counts: want 1/0/0, got %d/%d/%d", result.CorrectCount, result.WrongCount, result.EmptyCount)
	}
	if result.Rank != 1 {
		t.Errorf("rank: want 1, got %d", result.Rank)
	}
	if result.Breakdown != nil {
		t.Errorf("score_only should not include breakdown, got %v", result.Breakdown)
	}
	if result.Pembahasan != nil {
		t.Errorf("score_only should not include pembahasan, got %v", result.Pembahasan)
	}
}

func TestGetSessionResult_ScorePembahasan_IncludesBreakdownAndPembahasan(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_pembahasan"})

	testID := uuid.New()
	td := mcqTest(testID, examID)
	qID := td.Questions[0].Question.ID
	svc.repo.seedTests(examID, []model.TestDetail{td})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(2)}
	trueVal := true
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: floatPtr(2), GradedAt: timePtr(time.Now())},
	}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if len(result.Breakdown) != 1 {
		t.Fatalf("expected 1 breakdown row, got %d", len(result.Breakdown))
	}
	if result.Breakdown[0].Earned != 2 || result.Breakdown[0].Max != 2 {
		t.Errorf("breakdown earned/max: want 2/2, got %v/%v", result.Breakdown[0].Earned, result.Breakdown[0].Max)
	}
	if len(result.Pembahasan) != 1 {
		t.Fatalf("expected 1 pembahasan item, got %d", len(result.Pembahasan))
	}
	if result.Pembahasan[0].CorrectAnswer == nil || *result.Pembahasan[0].CorrectAnswer != "b" {
		t.Errorf("pembahasan correct_answer: want b, got %v", result.Pembahasan[0].CorrectAnswer)
	}
}

func TestGetSessionResult_Rank_HigherScoreSessions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})
	svc.repo.seedTests(examID, nil) // no essay questions -> trivially fully graded

	lowSess := uuid.New()
	svc.repo.sessions[lowSess] = &model.ExamSession{ID: lowSess, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(5)}

	otherStudent := uuid.New()
	highSess := uuid.New()
	svc.repo.sessions[highSess] = &model.ExamSession{ID: highSess, StudentID: otherStudent, ExamID: examID, Status: "submitted", Score: floatPtr(10)}

	result, err := svc.GetSessionResult(ctx, studentID.String(), lowSess.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.Rank != 2 {
		t.Errorf("rank: want 2 (one higher score), got %d", result.Rank)
	}

	topResult, err := svc.GetSessionResult(ctx, otherStudent.String(), highSess.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if topResult.Rank != 1 {
		t.Errorf("rank: want 1 (no higher score), got %d", topResult.Rank)
	}
}

// TestGetSessionResult_Rank_TiedScores_ShareRank documents the tie semantic named in
// FR-S5-18 explicitly: sessions with equal (non-strictly-higher) scores share a rank,
// since CountHigherScores only counts strictly-higher scores.
func TestGetSessionResult_Rank_TiedScores_ShareRank(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})
	svc.repo.seedTests(examID, nil) // no essay questions -> trivially fully graded

	studentA := uuid.New()
	sessA := uuid.New()
	svc.repo.sessions[sessA] = &model.ExamSession{ID: sessA, StudentID: studentA, ExamID: examID, Status: "submitted", Score: floatPtr(5)}

	studentB := uuid.New()
	sessB := uuid.New()
	svc.repo.sessions[sessB] = &model.ExamSession{ID: sessB, StudentID: studentB, ExamID: examID, Status: "submitted", Score: floatPtr(5)}

	resultA, err := svc.GetSessionResult(ctx, studentA.String(), sessA.String())
	if err != nil {
		t.Fatalf("GetSessionResult A: %v", err)
	}
	resultB, err := svc.GetSessionResult(ctx, studentB.String(), sessB.String())
	if err != nil {
		t.Fatalf("GetSessionResult B: %v", err)
	}

	if resultA.Rank != 1 {
		t.Errorf("rank A: want 1 (tied, no strictly-higher score), got %d", resultA.Rank)
	}
	if resultB.Rank != 1 {
		t.Errorf("rank B: want 1 (tied, no strictly-higher score), got %d", resultB.Rank)
	}
}

// TestGetSessionResult_Result_AllEssaysGraded_PassesFullyGradedGate exercises the
// positive branch of isFullyGraded (FR-S5-15): a session with an essay question whose
// answer has graded_at set must reach the "result" state, not "grading". Existing
// coverage only exercised the negative branch (ungraded essay -> "grading"); the other
// "result" state tests use pure-mcq fixtures that never enter the essay loop at all.
func TestGetSessionResult_Result_AllEssaysGraded_PassesFullyGradedGate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	testID := uuid.New()
	td, qID := essayTest(testID, 5)
	svc.repo.seedTests(examID, []model.TestDetail{td})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, StudentID: studentID, ExamID: examID, Status: "submitted", Score: floatPtr(3)}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my essay"), Score: floatPtr(3), GradedAt: timePtr(time.Now())},
	}

	result, err := svc.GetSessionResult(ctx, studentID.String(), sessID.String())
	if err != nil {
		t.Fatalf("GetSessionResult: %v", err)
	}
	if result.State != "result" {
		t.Errorf("state: want result (all essays graded), got %q", result.State)
	}
}

func TestGetSessionResult_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.GetSessionResult(ctx, uuid.New().String(), uuid.New().String())
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

// ---------- Admin grading ----------

func TestListGradingSessions_Delegates(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	want := []model.GradingSessionItem{{SessionID: uuid.New(), StudentName: "Budi", UngradedEssayCount: 2}}
	svc.repo.gradingQueue[examID] = want

	got, err := svc.ListGradingSessions(ctx, examID)
	if err != nil {
		t.Fatalf("ListGradingSessions: %v", err)
	}
	if len(got) != 1 || got[0].StudentName != "Budi" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGetSessionEssays_Delegates(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	want := []model.GradingEssayItem{{QuestionID: uuid.New(), Body: "Explain X", PointCorrect: 5}}
	svc.repo.seedEssays(sessID, want)

	got, err := svc.GetSessionEssays(ctx, sessID)
	if err != nil {
		t.Fatalf("GetSessionEssays: %v", err)
	}
	if len(got) != 1 || got[0].PointCorrect != 5 {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGradeEssayAnswer_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	qID := uuid.New()
	otherQID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: qID, Body: "Explain X", PointCorrect: 5}})
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my answer")},
		{QuestionID: otherQID, Score: floatPtr(2), GradedAt: timePtr(time.Now())}, // already-graded objective
	}
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, Status: "submitted"}

	gradedBy := uuid.New()
	comment := "well done"
	total, err := svc.GradeEssayAnswer(ctx, sessID, qID, 3, &comment, gradedBy)
	if err != nil {
		t.Fatalf("GradeEssayAnswer: %v", err)
	}
	if total != 5 {
		t.Errorf("total: want 5 (2 objective + 3 essay), got %v", total)
	}

	essays, _ := svc.GetSessionEssays(ctx, sessID)
	if essays[0].Score == nil || *essays[0].Score != 3 {
		t.Errorf("essay score not persisted: got %v", essays[0].Score)
	}
	if essays[0].GradedAt == nil {
		t.Error("expected graded_at to be set")
	}
	if essays[0].GraderComment == nil || *essays[0].GraderComment != "well done" {
		t.Errorf("comment not persisted: got %v", essays[0].GraderComment)
	}
}

// Behaviour change: a fractional grade within range is now accepted end to end.
// It was rejected while point_correct was an integer column; since 0049 made it
// NUMERIC, refusing decimals made full marks unreachable on a fractional-point essay.
func TestGradeEssayAnswer_Fractional_Accepted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	qID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: qID, PointCorrect: 5}})
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{{QuestionID: qID, Answer: strPtr("my answer")}}
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, Status: "submitted"}

	total, err := svc.GradeEssayAnswer(ctx, sessID, qID, 2.5, nil, uuid.New())
	if err != nil {
		t.Fatalf("want nil for a fractional grade in range, got %v", err)
	}
	if total != 2.5 {
		t.Errorf("session total: got %v, want 2.5", total)
	}
}

func TestGradeEssayAnswer_FullMarksOnFractionalPointCorrect(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	qID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: qID, PointCorrect: 2.5}})
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{{QuestionID: qID, Answer: strPtr("my answer")}}
	svc.repo.sessions[sessID] = &model.ExamSession{ID: sessID, Status: "submitted"}

	total, err := svc.GradeEssayAnswer(ctx, sessID, qID, 2.5, nil, uuid.New())
	if err != nil {
		t.Fatalf("want nil awarding full marks on a 2.5-point essay, got %v", err)
	}
	if total != 2.5 {
		t.Errorf("session total: got %v, want 2.5", total)
	}
}

func TestGradeEssayAnswer_Negative(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	qID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: qID, PointCorrect: 5}})

	_, err := svc.GradeEssayAnswer(ctx, sessID, qID, -1, nil, uuid.New())
	if !errors.Is(err, ErrGradeOutOfRange) {
		t.Errorf("want ErrGradeOutOfRange, got %v", err)
	}
}

func TestGradeEssayAnswer_ExceedsPointCorrect(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	qID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: qID, PointCorrect: 5}})

	_, err := svc.GradeEssayAnswer(ctx, sessID, qID, 6, nil, uuid.New())
	if !errors.Is(err, ErrGradeOutOfRange) {
		t.Errorf("want ErrGradeOutOfRange, got %v", err)
	}
}

func TestGradeEssayAnswer_NotEssayQuestion(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessID := uuid.New()
	svc.repo.seedEssays(sessID, []model.GradingEssayItem{{QuestionID: uuid.New(), PointCorrect: 5}})

	_, err := svc.GradeEssayAnswer(ctx, sessID, uuid.New(), 3, nil, uuid.New())
	if !errors.Is(err, ErrNotEssayQuestion) {
		t.Errorf("want ErrNotEssayQuestion, got %v", err)
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// ---- resultGate (FR-S5-21) — exercises the real function used by Service.GetSessionResult ----

func TestResultGate_Hidden_TakesPrecedenceOverEverything(t *testing.T) {
	exam := model.Exam{ResultConfig: "hidden"}
	result, ok := resultGate(exam, true, true)
	if !ok || result.State != "hidden" {
		t.Errorf("want hidden, got %+v ok=%v", result, ok)
	}
}

func TestResultGate_NotSubmitted_Grading(t *testing.T) {
	exam := model.Exam{ResultConfig: "score_only"}
	result, ok := resultGate(exam, false, true)
	if !ok || result.State != "grading" {
		t.Errorf("want grading, got %+v ok=%v", result, ok)
	}
}

func TestResultGate_NotFullyGraded_Grading(t *testing.T) {
	exam := model.Exam{ResultConfig: "score_only"}
	result, ok := resultGate(exam, true, false)
	if !ok || result.State != "grading" {
		t.Errorf("want grading, got %+v ok=%v", result, ok)
	}
}

func TestResultGate_Locked_ReleaseInFuture(t *testing.T) {
	releaseAt := time.Now().Add(time.Hour)
	exam := model.Exam{ResultConfig: "score_only", ResultReleaseAt: &releaseAt}
	result, ok := resultGate(exam, true, true)
	if !ok || result.State != "locked" || result.ResultReleaseAt != &releaseAt {
		t.Errorf("want locked with release_at, got %+v ok=%v", result, ok)
	}
}

func TestResultGate_ReleaseInPast_PassesThrough(t *testing.T) {
	releaseAt := time.Now().Add(-time.Hour)
	exam := model.Exam{ResultConfig: "score_only", ResultReleaseAt: &releaseAt}
	_, ok := resultGate(exam, true, true)
	if ok {
		t.Error("want gate to pass through (release_at in the past)")
	}
}

func TestResultGate_NoReleaseAt_PassesThrough(t *testing.T) {
	exam := model.Exam{ResultConfig: "score_pembahasan"}
	_, ok := resultGate(exam, true, true)
	if ok {
		t.Error("want gate to pass through (no release_at set)")
	}
}

func TestResultGate_HiddenBeatsNotFullyGraded(t *testing.T) {
	// Precedence check: hidden must win even when the session also isn't fully graded.
	exam := model.Exam{ResultConfig: "hidden"}
	result, ok := resultGate(exam, true, false)
	if !ok || result.State != "hidden" {
		t.Errorf("want hidden (gate 1 precedence), got %+v ok=%v", result, ok)
	}
}

// ---- validateGrade (FR-S5-13) — exercises the real function used by Service.GradeEssayAnswer ----

func TestValidateGrade_ValidInteger_OK(t *testing.T) {
	if err := validateGrade(3, 5); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidateGrade_Zero_OK(t *testing.T) {
	if err := validateGrade(0, 5); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidateGrade_EqualsMax_OK(t *testing.T) {
	if err := validateGrade(5, 5); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

// Behaviour change: fractional essay grades used to be rejected outright. Since
// point_correct became fractional, an essay authored at 2.5 points could never be
// awarded full marks — 2.5 was refused as non-integer and 3 exceeded the cap.
func TestValidateGrade_Fractional_OK(t *testing.T) {
	if err := validateGrade(2.5, 5); err != nil {
		t.Errorf("want nil for a fractional grade within range, got %v", err)
	}
}

func TestValidateGrade_FullMarksOnFractionalPointCorrect_OK(t *testing.T) {
	if err := validateGrade(2.5, 2.5); err != nil {
		t.Errorf("want nil awarding full marks on a 2.5-point essay, got %v", err)
	}
}

func TestValidateGrade_FractionalExceedingFractionalMax_Rejected(t *testing.T) {
	if err := validateGrade(2.75, 2.5); !errors.Is(err, ErrGradeOutOfRange) {
		t.Errorf("want ErrGradeOutOfRange, got %v", err)
	}
}

func TestValidateGrade_Negative_Rejected(t *testing.T) {
	if err := validateGrade(-1, 5); !errors.Is(err, ErrGradeOutOfRange) {
		t.Errorf("want ErrGradeOutOfRange, got %v", err)
	}
}

func TestValidateGrade_ExceedsMax_Rejected(t *testing.T) {
	if err := validateGrade(6, 5); !errors.Is(err, ErrGradeOutOfRange) {
		t.Errorf("want ErrGradeOutOfRange, got %v", err)
	}
}
