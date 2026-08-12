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
	roleDashboardResultLimit     = 5
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

type SchoolDashboardResponse struct {
	Counts             repository.SchoolDashboardCounts `json:"counts"`
	OrderableExamCount int                              `json:"orderable_exam_count"`
	LatestBulkOrder    *repository.LatestBulkOrder      `json:"latest_bulk_order"`
	RecentResults      []repository.RecentResult        `json:"recent_results"`
}

func (s *Service) SchoolDashboard(
	ctx context.Context, schoolID, adminID, role string,
) (SchoolDashboardResponse, error) {
	var out SchoolDashboardResponse

	var scope *string
	if schoolID != "" {
		scope = &schoolID
	}

	now := time.Now().In(jakarta)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, jakarta)
	monthEnd := monthStart.AddDate(0, 1, 0)

	counts, err := s.storeRepo.SchoolDashboardCounts(ctx, scope, monthStart, monthEnd)
	if err != nil {
		return out, err
	}

	exams, err := s.ListOrderableExams(ctx, role)
	if err != nil {
		return out, err
	}

	latest, err := s.storeRepo.LatestBulkExamOrder(ctx, adminID)
	if err != nil {
		return out, err
	}

	results, err := s.storeRepo.RecentSchoolResults(ctx, scope, roleDashboardResultLimit)
	if err != nil {
		return out, err
	}

	return SchoolDashboardResponse{
		Counts:             counts,
		OrderableExamCount: len(exams),
		LatestBulkOrder:    latest,
		RecentResults:      results,
	}, nil
}
