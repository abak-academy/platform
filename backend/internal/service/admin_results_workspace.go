package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// AdminGetExamResultsWorkspace returns the participant-centric results workspace
// read model for an exam (Issue 124): a paginated per-registration row list
// plus a cohort summary computed over the same q/school_id filter. Super-admin
// only — enforced by the RBAC middleware, not here.
func (s *Service) AdminGetExamResultsWorkspace(ctx context.Context, examID uuid.UUID, q string, schoolID *uuid.UUID, cursor string, limit int) (model.ResultsWorkspaceResponse, error) {
	if _, err := s.storeRepo.GetExamByID(ctx, examID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ResultsWorkspaceResponse{}, ErrExamNotFound
		}
		return model.ResultsWorkspaceResponse{}, err
	}

	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	filter := repository.ResultsWorkspaceFilter{Q: q, SchoolID: schoolID, Cursor: cursor, Limit: limit}

	rows, nextCursor, err := s.storeRepo.ListResultsWorkspaceRows(ctx, examID, filter)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidCursor) {
			return model.ResultsWorkspaceResponse{}, ErrValidation
		}
		return model.ResultsWorkspaceResponse{}, err
	}

	total, completed, scores, violationAttempts, violationEvents, err := s.storeRepo.GetResultsWorkspaceSummary(ctx, examID, filter)
	if err != nil {
		return model.ResultsWorkspaceResponse{}, err
	}

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, examID)
	if err != nil {
		return model.ResultsWorkspaceResponse{}, err
	}
	maxPossible := 0.0
	for _, td := range tests {
		for _, question := range td.Questions {
			maxPossible += questionMaxPoints(question)
		}
	}

	completionRate := 0.0
	if total > 0 {
		completionRate = float64(completed) / float64(total)
	}

	averageScore := 0.0
	if len(scores) > 0 {
		var sum float64
		for _, sc := range scores {
			sum += sc
		}
		averageScore = sum / float64(len(scores))
	}

	summary := model.ResultsWorkspaceSummary{
		TotalRegistered:       total,
		CompletedParticipants: completed,
		CompletionRate:        completionRate,
		AverageScore:          averageScore,
		Distribution:          scoreDistribution(scores, maxPossible),
		ViolationAttempts:     violationAttempts,
		ViolationEvents:       violationEvents,
	}

	return model.ResultsWorkspaceResponse{Summary: summary, Data: rows, NextCursor: nextCursor}, nil
}

// AdminGetResultsWorkspaceAttempts returns every attempt for a single registration
// (Issue 124 attempt drawer). ErrRegistrationNotFound covers both "no such
// registration" and "registration belongs to a different exam" — the repo
// validates registration_id against exam_id together.
func (s *Service) AdminGetResultsWorkspaceAttempts(ctx context.Context, examID, registrationID uuid.UUID) ([]model.ResultsWorkspaceAttempt, error) {
	attempts, err := s.storeRepo.ListResultsWorkspaceAttempts(ctx, examID, registrationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRegistrationNotFound
		}
		return nil, err
	}
	return attempts, nil
}

// scoreDistribution buckets scores into the 5 fixed 20-point bands shared by
// GetExamAnalytics, normalizing by maxPossible the same way (percentage of
// max attainable, not raw score) so the two admin surfaces agree.
func scoreDistribution(scores []float64, maxPossible float64) []model.ScoreBucket {
	distribution := []model.ScoreBucket{
		{Label: "0-20", Count: 0},
		{Label: "21-40", Count: 0},
		{Label: "41-60", Count: 0},
		{Label: "61-80", Count: 0},
		{Label: "81-100", Count: 0},
	}
	if maxPossible <= 0 {
		return distribution
	}
	for _, sc := range scores {
		pct := (sc / maxPossible) * 100
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
	return distribution
}
