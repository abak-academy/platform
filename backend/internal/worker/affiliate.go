package worker

import (
	"context"
	"log/slog"
	"time"
)

const (
	affiliatePollInterval = 15 * time.Minute
	affiliateSweepTimeout = 10 * time.Minute
)

// affiliateProcessor is the daily affiliate job entry point; *service.Service
// implements it, and a nil processor (unit-test workers) skips the loop.
type affiliateProcessor interface {
	RunAffiliateDaily(ctx context.Context, now time.Time) error
}

func resolveAffiliateProcessor(svc bulkProcessor) affiliateProcessor {
	affiliateSvc, _ := svc.(affiliateProcessor)
	return affiliateSvc
}

func (w *Worker) pollAffiliateLoop(ctx context.Context) {
	if w.affiliateSvc == nil {
		return
	}
	ticker := time.NewTicker(affiliatePollInterval)
	defer ticker.Stop()

	w.pollAffiliate(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAffiliate(ctx)
		}
	}
}

func (w *Worker) pollAffiliate(ctx context.Context) {
	if w.affiliateSvc == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	sweepCtx, cancel := context.WithTimeout(ctx, affiliateSweepTimeout)
	defer cancel()
	if err := w.affiliateSvc.RunAffiliateDaily(sweepCtx, time.Now()); err != nil {
		slog.Error("affiliate daily job", "err", err)
	}
}
