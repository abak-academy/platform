package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- fakeSessionRepo: leaderboard/analytics extensions ----------

func (f *fakeSessionRepo) ListExamLeaderboard(_ context.Context, examID uuid.UUID, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	if limit == 0 {
		limit = 20
	}
	// Gather fully-graded submitted sessions for this exam.
	type entry struct {
		sid     uuid.UUID
		student uuid.UUID
		name    string
		score   float64
	}
	var entries []entry
	for _, s := range f.sessions {
		if s.ExamID != examID || s.Status != "submitted" || s.Score == nil {
			continue
		}
		if !f.fullyGraded(s.ID) {
			continue
		}
		entries = append(entries, entry{sid: s.ID, student: s.StudentID, name: "", score: *s.Score})
	}
	// Sort by score desc, then id asc for tie-breaking.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return false
	})

	// Compute rank (1-based, ties share rank).
	type ranked struct {
		entry
		rank int
	}
	var rankedList []ranked
	for i, e := range entries {
		r := i + 1
		if i > 0 && entries[i].score == entries[i-1].score {
			r = rankedList[i-1].rank
		}
		rankedList = append(rankedList, ranked{entry: e, rank: r})
	}

	// Filter by cursor.
	if cursor != "" {
		var filtered []ranked
		for _, re := range rankedList {
			// Cursor is score,id — result must have strictly lower (score, id)
			// than cursor to come after it.
			cursorScore, cursorID, found := splitCursor(cursor)
			if !found {
				return nil, "", fmt.Errorf("invalid cursor")
			}
			if re.score < cursorScore || (re.score == cursorScore && re.sid.String() <= cursorID) {
				filtered = append(filtered, re)
			}
		}
		rankedList = filtered
	}

	// Paginate.
	var nextCursor string
	if len(rankedList) > limit {
		extra := rankedList[limit]
		nextCursor = fmt.Sprintf("%.2f,%s", extra.score, extra.sid.String())
		rankedList = rankedList[:limit]
	}

	result := make([]model.ExamLeaderboardEntry, len(rankedList))
	for i, re := range rankedList {
		result[i] = model.ExamLeaderboardEntry{
			Rank:        re.rank,
			StudentID:   re.student,
			StudentName: re.name,
			Score:       re.score,
		}
	}
	return result, nextCursor, nil
}

func (f *fakeSessionRepo) GetExamCompletionStats(_ context.Context, examID uuid.UUID) (int, int, error) {
	var total, submitted int
	for _, s := range f.sessions {
		if s.ExamID != examID {
			continue
		}
		total++
		if s.Status == "submitted" {
			submitted++
		}
	}
	return total, submitted, nil
}

func (f *fakeSessionRepo) GetFullyGradedScores(_ context.Context, examID uuid.UUID) ([]float64, error) {
	var scores []float64
	for _, s := range f.sessions {
		if s.ExamID != examID || s.Status != "submitted" || s.Score == nil {
			continue
		}
		if !f.fullyGraded(s.ID) {
			continue
		}
		scores = append(scores, *s.Score)
	}
	if scores == nil {
		scores = []float64{}
	}
	return scores, nil
}

// splitCursor splits "score,id" into (score, id).
func splitCursor(cursor string) (float64, string, bool) {
	for i := 0; i < len(cursor); i++ {
		if cursor[i] == ',' {
			return 0, cursor[i+1:], true
		}
	}
	return 0, "", false
}

// ---------- shimSessionService: leaderboard/analytics ----------

func (s *shimSessionService) AdminGetLeaderboard(ctx context.Context, examID uuid.UUID, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	return s.repo.ListExamLeaderboard(ctx, examID, cursor, limit)
}

func (s *shimSessionService) StudentGetSessionLeaderboard(ctx context.Context, studentID, sessionID string, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrSessionNotFound
		}
		return nil, "", err
	}

	exam, err := s.repo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return nil, "", err
	}

	if !exam.AllowLeaderboard {
		return nil, "", ErrLeaderboardNotAvailable
	}

	tests, err := s.repo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return nil, "", err
	}
	answers, err := s.repo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return nil, "", err
	}

	var qs []model.QuestionWithOptions
	for _, td := range tests {
		qs = append(qs, td.Questions...)
	}

	if _, ok := resultGate(*exam, sess.Status == "submitted", isFullyGraded(qs, answers)); ok {
		return nil, "", ErrLeaderboardNotAvailable
	}

	return s.repo.ListExamLeaderboard(ctx, sess.ExamID, cursor, limit)
}

func (s *shimSessionService) GetExamAnalytics(ctx context.Context, examID uuid.UUID) (model.ExamAnalytics, error) {
	total, submitted, err := s.repo.GetExamCompletionStats(ctx, examID)
	if err != nil {
		return model.ExamAnalytics{}, err
	}

	completionRate := 0.0
	if total > 0 {
		completionRate = float64(submitted) / float64(total)
	}

	scores, err := s.repo.GetFullyGradedScores(ctx, examID)
	if err != nil {
		return model.ExamAnalytics{}, err
	}

	averageScore := 0.0
	if len(scores) > 0 {
		var sum float64
		for _, sc := range scores {
			sum += sc
		}
		averageScore = sum / float64(len(scores))
	}

	tests, err := s.repo.GetSessionWithQuestions(ctx, examID)
	if err != nil {
		return model.ExamAnalytics{}, err
	}

	maxPossible := 0.0
	for _, td := range tests {
		for _, q := range td.Questions {
			maxPossible += q.Question.PointCorrect
		}
	}

	distribution := []model.ScoreBucket{
		{Label: "0-20", Count: 0},
		{Label: "21-40", Count: 0},
		{Label: "41-60", Count: 0},
		{Label: "61-80", Count: 0},
		{Label: "81-100", Count: 0},
	}

	if maxPossible > 0 {
		maxF := maxPossible
		for _, sc := range scores {
			pct := (sc / maxF) * 100
			switch {
			case pct <= 20:
				distribution[0].Count++
			case pct <= 40:
				distribution[1].Count++
			case pct <= 60:
				distribution[2].Count++
			case pct <= 80:
				distribution[3].Count++
			default:
				distribution[4].Count++
			}
		}
	}

	return model.ExamAnalytics{
		AverageScore:   averageScore,
		CompletionRate: completionRate,
		Distribution:   distribution,
	}, nil
}

// ---------- tests: AdminGetLeaderboard ----------

func TestAdminGetLeaderboard_Delegates(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, Title: "Test Exam"})

	// Create 3 submitted sessions with scores.
	studentID := uuid.New()
	for i := 0; i < 3; i++ {
		sID := uuid.New()
		score := float64(100 - i*10)
		svc.repo.sessions[sID] = &model.ExamSession{
			ID: sID, StudentID: uuid.New(), ExamID: examID,
			Status: "submitted", Score: &score,
		}
	}
	_ = studentID

	entries, nextCursor, err := svc.AdminGetLeaderboard(ctx, examID, "", 10)
	if err != nil {
		t.Fatalf("AdminGetLeaderboard: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("want 3 entries, got %d", len(entries))
	}
	if nextCursor != "" {
		t.Errorf("want empty cursor, got %q", nextCursor)
	}
}

// ---------- tests: StudentGetSessionLeaderboard ----------

func TestStudentGetLeaderboard_InvalidStudentID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, "not-a-uuid", uuid.New().String(), "", 20)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestStudentGetLeaderboard_InvalidSessionID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, uuid.New().String(), "not-a-uuid", "", 20)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestStudentGetLeaderboard_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	sessionID := uuid.New()

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessionID.String(), "", 20)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestStudentGetLeaderboard_AllowLeaderboardFalse(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, AllowLeaderboard: false, ResultConfig: "score_only"})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessID.String(), "", 20)
	if !errors.Is(err, ErrLeaderboardNotAvailable) {
		t.Errorf("want ErrLeaderboardNotAvailable, got %v", err)
	}
}

func TestStudentGetLeaderboard_Hidden_Gated(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, AllowLeaderboard: true, ResultConfig: "hidden"})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessID.String(), "", 20)
	if !errors.Is(err, ErrLeaderboardNotAvailable) {
		t.Errorf("want ErrLeaderboardNotAvailable, got %v", err)
	}
}

func TestStudentGetLeaderboard_NotSubmitted_Gated(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, AllowLeaderboard: true, ResultConfig: "score_only"})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "in_progress",
	}

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessID.String(), "", 20)
	if !errors.Is(err, ErrLeaderboardNotAvailable) {
		t.Errorf("want ErrLeaderboardNotAvailable, got %v", err)
	}
}

func TestStudentGetLeaderboard_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, AllowLeaderboard: true, ResultConfig: "score_only"})

	testID := uuid.New()
	svc.repo.seedTests(examID, []model.TestDetail{mcqTest(testID, examID)})

	sessID := uuid.New()
	score := 80.0
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: &score,
	}
	qID := mcqTest(testID, examID).Questions[0].Question.ID
	trueVal := true
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: &score, GradedAt: timePtr(time.Now())},
	}

	entries, nextCursor, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessID.String(), "", 20)
	if err != nil {
		t.Fatalf("StudentGetSessionLeaderboard: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Rank != 1 {
		t.Errorf("want rank 1, got %d", entries[0].Rank)
	}
	if nextCursor != "" {
		t.Errorf("want empty cursor, got %q", nextCursor)
	}
}

// ---------- tests: GetExamAnalytics ----------

func TestGetExamAnalytics_NoSessions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}
	if analytics.AverageScore != 0 {
		t.Errorf("want average 0, got %v", analytics.AverageScore)
	}
	if analytics.CompletionRate != 0 {
		t.Errorf("want completion rate 0, got %v", analytics.CompletionRate)
	}
	if len(analytics.Distribution) != 5 {
		t.Fatalf("want 5 distribution buckets, got %d", len(analytics.Distribution))
	}
	for _, b := range analytics.Distribution {
		if b.Count != 0 {
			t.Errorf("bucket %q: want count 0, got %d", b.Label, b.Count)
		}
	}
}

func TestGetExamAnalytics_WithSessions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	// Create 4 sessions: 1 in_progress, 3 submitted.
	scores := []float64{90, 50, 30}
	for i, sc := range scores {
		sID := uuid.New()
		status := "submitted"
		svc.repo.sessions[sID] = &model.ExamSession{
			ID: sID, StudentID: uuid.New(), ExamID: examID,
			Status: status, Score: &sc,
		}
		_ = i
	}
	// One unsubmitted session.
	sID := uuid.New()
	svc.repo.sessions[sID] = &model.ExamSession{
		ID: sID, StudentID: uuid.New(), ExamID: examID,
		Status: "in_progress",
	}

	// Seed tests so maxPossible > 0.
	testID := uuid.New()
	svc.repo.seedTests(examID, []model.TestDetail{mcqTest(testID, examID)})

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}

	// CompletionRate: 3 submitted / 4 total = 0.75
	if analytics.CompletionRate != 0.75 {
		t.Errorf("want completion rate 0.75, got %v", analytics.CompletionRate)
	}

	// AverageScore: (90 + 50 + 30) / 3 = 56.666...
	wantAvg := (90.0 + 50.0 + 30.0) / 3.0
	if analytics.AverageScore != wantAvg {
		t.Errorf("want average %v, got %v", wantAvg, analytics.AverageScore)
	}

	// Distribution: maxPossible = 2 (from mcqTest). Score 90 → 90/2*100=4500% → last bucket.
	// Score 50 → 2500% → last bucket. Score 30 → 1500% → last bucket.
	if len(analytics.Distribution) != 5 {
		t.Fatalf("want 5 distribution buckets, got %d", len(analytics.Distribution))
	}
	// All scores are well above 100% of maxPossible, so all in last bucket.
	if analytics.Distribution[4].Count != 3 {
		t.Errorf("want 3 entries in 81-100 bucket, got %d", analytics.Distribution[4].Count)
	}
}

func TestStudentGetLeaderboard_NotFullyGraded(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	studentID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, AllowLeaderboard: true, ResultConfig: "score_only"})

	testID := uuid.New()
	td, qID := essayTest(testID, 5)
	svc.repo.seedTests(examID, []model.TestDetail{td})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(0),
	}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my essay"), GradedAt: nil},
	}

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, studentID.String(), sessID.String(), "", 20)
	if !errors.Is(err, ErrLeaderboardNotAvailable) {
		t.Errorf("want ErrLeaderboardNotAvailable for ungraded essay, got %v", err)
	}
}

func TestStudentGetLeaderboard_NotOwnedByCaller(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	ownerID := uuid.New()
	callerID := uuid.New()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: ownerID, ExamID: examID,
		Status: "submitted",
	}

	_, _, err := svc.StudentGetSessionLeaderboard(ctx, callerID.String(), sessID.String(), "", 20)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound for session owned by another student, got %v", err)
	}
}

func TestGetExamAnalytics_NoQualifyingScores_ZeroAverage(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	// Submitted session with an ungraded essay → not fully graded → excluded from scores.
	testID := uuid.New()
	td, qID := essayTest(testID, 5)
	svc.repo.seedTests(examID, []model.TestDetail{td})

	s1 := uuid.New()
	svc.repo.sessions[s1] = &model.ExamSession{
		ID: s1, ExamID: examID, Status: "submitted", Score: floatPtr(50),
	}
	svc.repo.seedEssays(s1, []model.GradingEssayItem{
		{QuestionID: qID, Body: "Explain X", PointCorrect: 5, GradedAt: nil},
	})
	svc.repo.sessionAnswers[s1] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my essay"), GradedAt: nil},
	}

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}
	// total=1, submitted=1 → completion_rate = 1.0
	if analytics.CompletionRate != 1.0 {
		t.Errorf("completion_rate: want 1.0, got %v", analytics.CompletionRate)
	}
	// 0 fully-graded submitted sessions → average = 0
	if analytics.AverageScore != 0 {
		t.Errorf("average_score: want 0, got %v", analytics.AverageScore)
	}
}

func TestGetExamAnalytics_BucketBoundaries(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	// Single question with PointCorrect=100 → maxPossible=100.
	testID := uuid.New()
	td := model.TestDetail{
		Test: model.Test{ID: testID, Title: "Math", Subject: "Math", Topic: "Algebra"},
		Questions: []model.QuestionWithOptions{
			{Question: model.Question{ID: uuid.New(), Format: "mcq", PointCorrect: 100}, SortOrder: 1},
		},
	}
	svc.repo.seedTests(examID, []model.TestDetail{td})

	// Score 60 → 60% → bucket "41-60" (index 2)
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), ExamID: examID, Status: "submitted", Score: floatPtr(60),
	}
	// Score 80 → 80% → bucket "61-80" (index 3)
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), ExamID: examID, Status: "submitted", Score: floatPtr(80),
	}
	// Score 0 → 0-20 (index 0)
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), ExamID: examID, Status: "submitted", Score: floatPtr(0),
	}
	// Score 40 → 21-40 (index 1)
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), ExamID: examID, Status: "submitted", Score: floatPtr(40),
	}
	// Score 100 → 81-100 (index 4)
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), ExamID: examID, Status: "submitted", Score: floatPtr(100),
	}

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}

	if len(analytics.Distribution) != 5 {
		t.Fatalf("want 5 buckets, got %d", len(analytics.Distribution))
	}
	checks := []struct {
		idx   int
		label string
		want  int
	}{
		{0, "0-20", 1},    // score 0
		{1, "21-40", 1},   // score 40
		{2, "41-60", 1},   // score 60
		{3, "61-80", 1},   // score 80
		{4, "81-100", 1},  // score 100
	}
	for _, c := range checks {
		if analytics.Distribution[c.idx].Count != c.want {
			t.Errorf("bucket %q: want %d, got %d", c.label, c.want, analytics.Distribution[c.idx].Count)
		}
	}
}

func TestGetExamAnalytics_ZeroMaxPossible(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID})

	sID := uuid.New()
	score := 80.0
	svc.repo.sessions[sID] = &model.ExamSession{
		ID: sID, StudentID: uuid.New(), ExamID: examID,
		Status: "submitted", Score: &score,
	}
	// No tests seeded → maxPossible = 0.

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}
	for _, b := range analytics.Distribution {
		if b.Count != 0 {
			t.Errorf("bucket %q: want count 0 (maxPossible=0), got %d", b.Label, b.Count)
		}
	}
}

// TestGetExamAnalytics_TrueFalseMaxUsesStatementCount_FR35 exercises the real
// (*Service).GetExamAnalytics — not the shimSessionService duplicate used by the
// tests above — against a real Postgres (NFR-8), so the exam_leaderboard.go
// maxPossible loop's questionMaxPoints wiring is genuinely proven, not shimmed.
// A 4-statement true_false question at point_correct=1 has max=4: a session
// scoring 2 is 50% (bucket "41-60"). Had maxPossible stayed a flat point_correct
// (=1), the same score of 2 would read as 200% and land in "81-100" instead —
// the two computations are only distinguishable because the session's raw score
// exceeds a max of 1.
func TestGetExamAnalytics_TrueFalseMaxUsesStatementCount_FR35(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "TF analytics max "+uniqueSuffix(), "math", "algebra")
	q := model.Question{Format: "true_false", Body: "four statements " + uniqueSuffix(), PointCorrect: 1, PointWrong: 0}
	statements := []model.QuestionStatement{
		{Index: 1, Body: "s1", IsTrue: true},
		{Index: 2, Body: "s2", IsTrue: false},
		{Index: 3, Body: "s3", IsTrue: true},
		{Index: 4, Body: "s4", IsTrue: false},
	}
	created, err := svc.CreateBankQuestion(ctx, q, nil, nil, statements)
	if err != nil {
		t.Fatalf("CreateBankQuestion: %v", err)
	}
	attachQuestionDirect(t, ctx, repo, testID, created.Question.ID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)

	studentID := seedStudentDirect(t, ctx, repo, "TF Analytics Student "+uniqueSuffix(), "tfstudent"+uniqueSuffix())
	var regID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, "TOKEN"+uniqueSuffix(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}
	score := 2.0
	submittedAt := time.Now()
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status, submitted_at, score)
		VALUES ($1, $2, $3, 1, now(), 'submitted', $4, $5)`,
		regID, studentID, examID, submittedAt, score,
	); err != nil {
		t.Fatalf("insert exam_session: %v", err)
	}

	analytics, err := svc.GetExamAnalytics(ctx, examID)
	if err != nil {
		t.Fatalf("GetExamAnalytics: %v", err)
	}

	// Buckets are ["0-20","21-40","41-60","61-80","81-100"]; 50% falls in "41-60" (index 2).
	if analytics.Distribution[2].Count != 1 {
		t.Errorf("bucket 41-60: want 1 (score 2 / max 4 = 50%%), got %d", analytics.Distribution[2].Count)
	}
	if analytics.Distribution[4].Count != 0 {
		t.Errorf("bucket 81-100: want 0 — a flat point_correct=1 max would wrongly place the session here (200%%), got %d", analytics.Distribution[4].Count)
	}
}
