package service

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

type ExamExpiryResult struct {
	Candidate model.ExamExpiryCandidate
	Processed bool
	Err       error
}

func (s *Service) ExpireExamSessions(ctx context.Context, now time.Time, limit int) ([]ExamExpiryResult, error) {
	candidates, err := s.storeRepo.ListDueExamExpiryCandidates(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	results := make([]ExamExpiryResult, 0, len(candidates))
	for _, candidate := range candidates {
		processed, err := s.expireExamCandidate(ctx, now, candidate)
		results = append(results, ExamExpiryResult{
			Candidate: candidate,
			Processed: processed,
			Err:       err,
		})
	}
	return results, nil
}

func (s *Service) expireExamCandidate(ctx context.Context, now time.Time, candidate model.ExamExpiryCandidate) (bool, error) {
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	sess, err := s.storeRepo.GetExamSessionByIDForUpdateTx(ctx, tx, candidate.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if sess.Status != "in_progress" {
		return false, nil
	}
	exam, err := s.storeRepo.GetExamByIDTx(ctx, tx, sess.ExamID)
	if err != nil {
		return false, err
	}

	if candidate.TestID == nil {
		if exam.Mode != "standard" || !standardSessionDue(*sess, *exam, now) {
			return false, nil
		}
		if _, err := s.finalizeLockedSessionTx(ctx, tx, sess.ID, sess, false); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	if exam.Mode != "utbk" && exam.Mode != "ielts" {
		return false, nil
	}
	section, err := s.storeRepo.GetActiveSessionSectionForUpdateTx(ctx, tx, sess.ID, *candidate.TestID)
	if err != nil {
		if errors.Is(err, repository.ErrNoActiveSection) || errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !sectionDue(*section, exam.GraceWindowMinutes, now) {
		return false, nil
	}
	nextTestID, err := s.storeRepo.AdvanceSessionSectionTx(ctx, tx, sess.ID, *candidate.TestID)
	if err != nil {
		if errors.Is(err, repository.ErrNoActiveSection) {
			return false, nil
		}
		return false, err
	}
	if nextTestID == nil {
		if _, err := s.finalizeLockedSessionTx(ctx, tx, sess.ID, sess, false); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func standardSessionDue(sess model.ExamSession, exam model.Exam, now time.Time) bool {
	if exam.DurationMinutes == nil || *exam.DurationMinutes <= 0 {
		return false
	}
	return !now.Before(expiryDeadline(sess.StartedAt, *exam.DurationMinutes, sess.ExtendedUntil, exam.GraceWindowMinutes))
}

func sectionDue(section model.ExamSessionSection, graceMinutes *int, now time.Time) bool {
	if section.StartedAt == nil || section.DurationMinutes <= 0 {
		return false
	}
	return !now.Before(expiryDeadline(*section.StartedAt, section.DurationMinutes, section.ExtendedUntil, graceMinutes))
}

func expiryDeadline(startedAt time.Time, durationMinutes int, extendedUntil *time.Time, graceMinutes *int) time.Time {
	deadline := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
	if extendedUntil != nil && extendedUntil.After(deadline) {
		deadline = *extendedUntil
	}
	if graceMinutes != nil {
		deadline = deadline.Add(time.Duration(*graceMinutes) * time.Minute)
	}
	return deadline
}
