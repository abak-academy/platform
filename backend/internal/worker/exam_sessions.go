package worker

import (
	"context"
	"log/slog"
	"time"

	"akademi-bimbel/internal/metrics"
)

// examSessionPollInterval trades gauge freshness against query cost. The
// count is one indexed scan per 30s — noise next to the exam-day write load,
// and fresh enough to watch the "N students working" curve during an event.
const examSessionPollInterval = 30 * time.Second

// pollActiveExamSessions keeps §4 of issue #98 alive: the number of students
// currently sitting an exam, defined as sessions started but not submitted.
//
// Counted in SQL rather than tracked in-process on purpose: api and worker
// are separate processes (and a future second replica would lie), while the
// database is the only place where "active" has one authoritative answer.
func (w *Worker) pollActiveExamSessions(ctx context.Context) {
	ticker := time.NewTicker(examSessionPollInterval)
	defer ticker.Stop()

	w.refreshActiveExamSessions(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.refreshActiveExamSessions(ctx)
		}
	}
}

func (w *Worker) refreshActiveExamSessions(ctx context.Context) {
	var n int64
	err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM exam_session WHERE submitted_at IS NULL`,
	).Scan(&n)
	if err != nil {
		slog.Error("count active exam sessions", "err", err)
		return
	}
	metrics.ExamSessionsActive.Set(float64(n))
}
