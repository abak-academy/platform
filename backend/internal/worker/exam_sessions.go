package worker

import (
	"context"
	"log/slog"
	"time"

	"akademi-bimbel/internal/metrics"
)

// examSessionPollInterval trades gauge freshness against query cost. The
// count is one partial-index scan per 30s — idx_examsession_active narrows
// the rows to in-progress, unsubmitted sessions, so the poll never seq-scans
// the whole table — and is noise next to the exam-day write load, while
// being fresh enough to watch the "N students working" curve during an event.
const examSessionPollInterval = 30 * time.Second

// pollActiveExamSessions keeps §4 of issue #98 alive: the number of students
// currently sitting an exam. "Currently" is the live window — from the exam's
// scheduled start until duration+grace past its end — NOT "submitted_at IS
// NULL", which would accumulate every abandoned session forever (there is no
// server-side auto-submit sweeper; a student who closes the tab keeps
// submitted_at NULL). Exams without a schedule (free practice) fall back to
// "started within the last 3 hours", comfortably above the ~2h max exam
// duration plus grace.
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
	err := w.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM exam_session s
		JOIN exam e ON e.id = s.exam_id
		WHERE s.status = 'in_progress'
		  AND s.submitted_at IS NULL
		  AND (
		        (e.scheduled_at IS NOT NULL
		           AND now() >= e.scheduled_at
		           AND now() <= COALESCE(e.scheduled_end_at, e.scheduled_at)
		                        + COALESCE(e.duration_minutes, 0) * interval '1 minute'
		                        + COALESCE(e.grace_window_minutes, 0) * interval '1 minute')
		        OR
		        (e.scheduled_at IS NULL
		           AND s.started_at > now() - interval '3 hours')
		      )`,
	).Scan(&n)
	if err != nil {
		slog.Error("count active exam sessions", "err", err)
		return
	}
	metrics.ExamSessionsActive.Set(float64(n))
}
