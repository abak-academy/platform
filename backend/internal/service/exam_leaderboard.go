package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// AdminGetLeaderboard returns a cursor-paginated ranked list of fully-graded submitted
// sessions for an exam (FR-10). No gating — RBAC middleware restricts this to admins.
func (s *Service) AdminGetLeaderboard(ctx context.Context, examID uuid.UUID, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	entries, next, err := s.storeRepo.ListExamLeaderboard(ctx, examID, cursor, limit)
	return entries, next, mapCursorErr(err)
}

// mapCursorErr surfaces a malformed pagination cursor as a validation error (422)
// instead of an opaque internal error (500).
func mapCursorErr(err error) error {
	if errors.Is(err, repository.ErrInvalidCursor) {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return err
}

// StudentGetSessionLeaderboard returns the exam leaderboard scoped to the calling
// student's own session (FR-11..14). Enforces AllowLeaderboard and the 3 result gates:
// hidden / grading / locked all block the leaderboard.
func (s *Service) StudentGetSessionLeaderboard(ctx context.Context, studentID, sessionID string, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrSessionNotFound
		}
		return nil, "", err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return nil, "", err
	}

	if !exam.AllowLeaderboard {
		return nil, "", ErrLeaderboardNotAvailable
	}

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return nil, "", err
	}
	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
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

	entries, next, err := s.storeRepo.ListExamLeaderboard(ctx, sess.ExamID, cursor, limit)
	return entries, next, mapCursorErr(err)
}

// GetExamAnalytics computes completion rate, average score, and score distribution
// for an exam (FR-15).
func (s *Service) GetExamAnalytics(ctx context.Context, examID uuid.UUID) (model.ExamAnalytics, error) {
	total, submitted, err := s.storeRepo.GetExamCompletionStats(ctx, examID)
	if err != nil {
		return model.ExamAnalytics{}, err
	}

	completionRate := 0.0
	if total > 0 {
		completionRate = float64(submitted) / float64(total)
	}

	scores, err := s.storeRepo.GetFullyGradedScores(ctx, examID)
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

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, examID)
	if err != nil {
		return model.ExamAnalytics{}, err
	}

	maxPossible := 0.0
	for _, td := range tests {
		for _, q := range td.Questions {
			maxPossible += questionMaxPoints(q)
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
