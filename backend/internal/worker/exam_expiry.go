package worker

import (
	"context"
	"log/slog"
	"time"

	"akademi-bimbel/internal/service"
)

const (
	examExpiryPageSize     = 50
	examExpiryPollInterval = 30 * time.Minute
	examExpirySweepTimeout = 5 * time.Minute
)

type expiryProcessor interface {
	ExpireExamSessions(ctx context.Context, now time.Time, pageSize int) ([]service.ExamExpiryResult, error)
}

func resolveExpiryProcessor(svc bulkProcessor) expiryProcessor {
	expirySvc, _ := svc.(expiryProcessor)
	return expirySvc
}

func (w *Worker) pollExamExpiryLoop(ctx context.Context) {
	if w.expirySvc == nil {
		return
	}
	ticker := time.NewTicker(examExpiryPollInterval)
	defer ticker.Stop()

	w.pollExamExpiry(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollExamExpiry(ctx)
		}
	}
}

func (w *Worker) pollExamExpiry(ctx context.Context) {
	if w.expirySvc == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	sweepCtx, cancel := context.WithTimeout(ctx, examExpirySweepTimeout)
	defer cancel()
	results, err := w.expirySvc.ExpireExamSessions(sweepCtx, time.Now(), examExpiryPageSize)
	if err != nil {
		slog.Error("expire exam sessions", "err", err)
		return
	}
	for _, result := range results {
		if result.Err != nil {
			slog.Error("expire exam session candidate", "session_id", result.Candidate.SessionID, "test_id", result.Candidate.TestID, "err", result.Err)
		}
	}
}
