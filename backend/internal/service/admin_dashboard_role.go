package service

import (
	"context"
	"time"

	"akademi-bimbel/internal/repository"
)

const (
	roleDashboardExamHorizonDays = 14
	roleDashboardExamLimit       = 5
	roleDashboardViolationLimit  = 5
)

type ExamDashboardResponse struct {
	ActiveSessions   int                            `json:"active_sessions"`
	UpcomingExams    []repository.UpcomingExam      `json:"upcoming_exams"`
	Counts           repository.ExamDashboardCounts `json:"counts"`
	RecentViolations []repository.RecentViolation   `json:"recent_violations"`
}

func (s *Service) ExamDashboard(ctx context.Context) (ExamDashboardResponse, error) {
	var out ExamDashboardResponse

	active, err := s.storeRepo.CountActiveExamSessions(ctx)
	if err != nil {
		return out, err
	}

	now := time.Now()
	upcoming, err := s.storeRepo.UpcomingExams(
		ctx, now, now.AddDate(0, 0, roleDashboardExamHorizonDays), roleDashboardExamLimit,
	)
	if err != nil {
		return out, err
	}

	counts, err := s.storeRepo.ExamDashboardCounts(ctx)
	if err != nil {
		return out, err
	}

	violations, err := s.storeRepo.RecentViolationsGlobal(ctx, roleDashboardViolationLimit)
	if err != nil {
		return out, err
	}

	return ExamDashboardResponse{
		ActiveSessions:   active,
		UpcomingExams:    upcoming,
		Counts:           counts,
		RecentViolations: violations,
	}, nil
}
