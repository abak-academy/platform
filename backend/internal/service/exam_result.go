package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- GetSessionResult ----------

// GetSessionResult loads a student's own session and applies the 3-gate result-visibility
// precedence (FR-S5-21): hidden -> grading -> locked -> result. Ownership is enforced via
// GetExamSessionForStudent (404 on mismatch, FR-S5-20).
func (s *Service) GetSessionResult(ctx context.Context, studentID, sessionID string) (model.SessionResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return model.SessionResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return model.SessionResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.SessionResult{}, ErrSessionNotFound
		}
		return model.SessionResult{}, err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return model.SessionResult{}, err
	}

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return model.SessionResult{}, err
	}
	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return model.SessionResult{}, err
	}

	var qs []model.QuestionWithOptions
	for _, td := range tests {
		qs = append(qs, td.Questions...)
	}

	// Resolve certificate URL (FR-5..8, FR-4). Must happen before the gate check so
	// certificate_url is present in any result-visibility state for submitted sessions.
	user, err := s.Me(ctx, studentID)
	studentName := ""
	if err == nil && user != nil {
		studentName = user.Name
	}
	// A certificate failure (MinIO down, PDF error) must not block the result view —
	// log it and degrade to a nil certificate_url.
	certURL, certErr := s.resolveCertificateURL(ctx, exam, sess, answers, studentName)
	if certErr != nil {
		slog.Error("resolve certificate url", "session_id", sessID, "err", certErr)
	}

	// Gates 1-3 (FR-S5-21): hidden -> grading -> locked. Short-circuits before the
	// rank aggregate query when the full result isn't visible yet.
	if gated, ok := resultGate(*exam, sess.Status == "submitted", isFullyGraded(qs, answers)); ok {
		gated.CertificateURL = certURL
		gated.EndScreenImageURL = exam.EndScreenImageURL
		gated.EndScreenPromoText = exam.EndScreenPromoText
		return gated, nil
	}

	// Gate 4: full result.
	score := 0.0
	if sess.Score != nil {
		score = *sess.Score
	}
	higherCount, err := s.storeRepo.CountHigherScores(ctx, sess.ExamID, score)
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
		result.Pembahasan = buildPembahasan(qs, answers)
	}

	result.CertificateURL = certURL
	result.EndScreenImageURL = exam.EndScreenImageURL
	result.EndScreenPromoText = exam.EndScreenPromoText
	return result, nil
}

// resultGate implements the 3-gate precedence (FR-S5-21): hidden -> grading -> locked.
// Returns (nonResultState, true) when a gate blocks the full result, or (zero, false)
// when all three gates pass and the caller should build the full result.
func resultGate(exam model.Exam, submitted, fullyGraded bool) (model.SessionResult, bool) {
	if exam.ResultConfig == "hidden" {
		return model.SessionResult{State: "hidden"}, true
	}
	// An unsubmitted session is trivially not fully graded.
	if !submitted || !fullyGraded {
		return model.SessionResult{State: "grading"}, true
	}
	if exam.ResultReleaseAt != nil && exam.ResultReleaseAt.After(time.Now()) {
		return model.SessionResult{State: "locked", ResultReleaseAt: exam.ResultReleaseAt}, true
	}
	return model.SessionResult{}, false
}

// isFullyGraded checks FR-S5-15: every essay question's answer has graded_at set.
func isFullyGraded(questions []model.QuestionWithOptions, answers []model.ExamSessionAnswer) bool {
	gradedAt := make(map[uuid.UUID]bool, len(answers))
	for _, a := range answers {
		gradedAt[a.QuestionID] = a.GradedAt != nil
	}
	for _, q := range questions {
		if q.Question.Format == "essay" && !gradedAt[q.Question.ID] {
			return false
		}
	}
	return true
}

// buildPembahasan builds one row per objective question for score_pembahasan (FR-S5-23);
// essay questions are excluded (out of scope for Slice 5).
func buildPembahasan(questions []model.QuestionWithOptions, answers []model.ExamSessionAnswer) []model.ResultPembahasanItem {
	answerByQuestion := make(map[uuid.UUID]model.ExamSessionAnswer, len(answers))
	for _, a := range answers {
		answerByQuestion[a.QuestionID] = a
	}

	items := make([]model.ResultPembahasanItem, 0, len(questions))
	for _, q := range questions {
		if q.Question.Format == "essay" {
			continue
		}
		a := answerByQuestion[q.Question.ID]
		items = append(items, model.ResultPembahasanItem{
			QuestionID:    q.Question.ID,
			Body:          q.Question.Body,
			Format:        q.Question.Format,
			YourAnswer:    a.Answer,
			CorrectAnswer: correctAnswerText(q.Question, q.Options),
			IsCorrect:     a.IsCorrect,
			Explanation:   q.Question.Explanation,
		})
	}
	return items
}

// correctAnswerText derives the displayable correct answer per format: the correct option
// key(s) for mcq/multi_answer, or the authored correct_answer for short/fill_blank.
func correctAnswerText(q model.Question, options []model.QuestionOption) *string {
	switch q.Format {
	case "mcq":
		for _, o := range options {
			if o.IsCorrect {
				key := o.Key
				return &key
			}
		}
		return nil
	case "multi_answer":
		var keys []string
		for _, o := range options {
			if o.IsCorrect {
				keys = append(keys, o.Key)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		sort.Strings(keys)
		joined := strings.Join(keys, ",")
		return &joined
	default:
		return q.CorrectAnswer
	}
}

// ---------- Admin grading ----------

// ListGradingSessions returns the grading queue for an exam (FR-S5-16).
func (s *Service) ListGradingSessions(ctx context.Context, examID uuid.UUID) ([]model.GradingSessionItem, error) {
	return s.storeRepo.ListSessionsNeedingGrading(ctx, examID)
}

// GetSessionEssays returns the essay answers of one session for admin grading (FR-S5-17).
func (s *Service) GetSessionEssays(ctx context.Context, sessionID uuid.UUID) ([]model.GradingEssayItem, error) {
	return s.storeRepo.GetSessionEssayAnswers(ctx, sessionID)
}

// validateGrade enforces FR-S5-13: score must be in [0, pointCorrect]. The grade may be
// fractional — point_correct is NUMERIC since 0049, so an essay worth 2.5 points must be
// awardable 2.5. Requiring a whole number here made full marks unreachable on any
// fractional-point essay.
func validateGrade(score float64, pointCorrect float64) error {
	if score < 0 || score > pointCorrect {
		return ErrGradeOutOfRange
	}
	return nil
}

// GradeEssayAnswer validates and persists an admin's grade for one essay answer, recomputes
// the session total, and returns it (FR-S5-12..14). Opens its own transaction; the answer
// write and the session-total write commit together.
func (s *Service) GradeEssayAnswer(ctx context.Context, sessionID, questionID uuid.UUID, score float64, comment *string, gradedBy uuid.UUID) (float64, error) {
	essays, err := s.storeRepo.GetSessionEssayAnswers(ctx, sessionID)
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

	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	for i := range answers {
		if answers[i].QuestionID == questionID {
			answers[i].Score = &score
		}
	}
	total := computeSessionTotal(answers)

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.GradeEssayAnswerTx(ctx, tx, sessionID, questionID, score, comment, gradedBy); err != nil {
		return 0, err
	}
	if err := s.storeRepo.UpdateSessionScoreTx(ctx, tx, sessionID, total); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return total, nil
}
