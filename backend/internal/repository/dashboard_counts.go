package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UpcomingExam struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	RegistrantCount int       `json:"registrant_count"`
}

// CountActiveExamSessions counts in-progress sessions across every exam whose
// deadline has not yet passed. status alone is not enough: nothing flips it
// when a student disconnects and never submits, so a stale in_progress row
// would otherwise inflate this indefinitely.
//
// The deadline mirrors computeRemainingSeconds in internal/service/exam_session.go:
// started_at + exam.duration_minutes, pushed forward only when extended_until
// is later. A NULL duration_minutes means the exam has no timer (the per_test
// path), so such sessions never expire here. This is a whole-session
// approximation for the dashboard — it does not model per-section deadlines
// (computeSectionRemaining) or grace_window_minutes the way the session
// monitor's deriveStatus does; a coarser cut is the right granularity for a
// single count tile.
func (r *Repository) CountActiveExamSessions(ctx context.Context) (int, error) {
	var n int
	// Mirrors deriveStatus/effectiveDeadline in service/exam_session.go, which
	// are authoritative — nothing ever writes an "overdue" status, so counting
	// stored status alone reports disconnected sessions as active forever.
	//
	// Three rules, all of them load-bearing:
	//   - a session with an ACTIVE section uses that section's own clock and
	//     ignores the exam-level one entirely (FR-20). Without this a
	//     section-timed exam whose exam-level duration_minutes is NULL counts
	//     forever, which is the bug this query exists to prevent.
	//   - the exam-level path adds grace_window_minutes; a session still inside
	//     its grace window is in_progress, not overdue.
	//   - duration must be > 0, not merely non-NULL: effectiveDeadline treats 0
	//     as "no deadline" and leaves the session active.
	//
	// GREATEST ignores NULL operands and yields NULL only when every operand is
	// NULL, so "no deadline at all" falls through to COALESCE(..., true) —
	// matching effectiveDeadline returning a zero time and deriveStatus then
	// reporting in_progress.
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM exam_session es
		  JOIN exam e ON e.id = es.exam_id
		  LEFT JOIN exam_session_section ss
		         ON ss.session_id = es.id AND ss.status = 'active'
		 WHERE es.status = 'in_progress'
		   AND CASE
		         WHEN ss.started_at IS NOT NULL THEN
		           COALESCE(now() < GREATEST(
		             CASE WHEN ss.duration_minutes > 0
		                  THEN ss.started_at + ss.duration_minutes * interval '1 minute'
		             END,
		             ss.extended_until
		           ), true)
		         ELSE
		           COALESCE(now() < GREATEST(
		             CASE WHEN e.duration_minutes > 0
		                  THEN es.started_at
		                     + e.duration_minutes * interval '1 minute'
		                     + COALESCE(e.grace_window_minutes, 0) * interval '1 minute'
		             END,
		             es.extended_until
		           ), true)
		       END`,
	).Scan(&n)
	return n, err
}

// UpcomingExams returns scheduled exams in [from, to) soonest-first, with how
// many students are registered for each.
func (r *Repository) UpcomingExams(
	ctx context.Context, from, to time.Time, limit int,
) ([]UpcomingExam, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.title, e.scheduled_at,
		       COUNT(er.id) AS registrant_count
		  FROM exam e
		  LEFT JOIN exam_registration er ON er.exam_id = e.id
		 WHERE e.scheduled_at IS NOT NULL
		   AND e.scheduled_at >= $1
		   AND e.scheduled_at <  $2
		 GROUP BY e.id, e.title, e.scheduled_at
		 ORDER BY e.scheduled_at
		 LIMIT $3
	`, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UpcomingExam, 0, limit)
	for rows.Next() {
		var e UpcomingExam
		if err := rows.Scan(&e.ID, &e.Title, &e.ScheduledAt, &e.RegistrantCount); err != nil {
			return nil, err
		}
		// scheduled_at is a genuine timestamptz; pgx decodes it in UTC
		// location by default, which is a correct instant but would
		// serialize as a false "...Z". .In keeps the instant and just
		// changes the display zone, so JSON carries the honest +07:00.
		e.ScheduledAt = e.ScheduledAt.In(jakarta)
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountStudentsAndSchools returns running totals, not period-scoped figures.
func (r *Repository) CountStudentsAndSchools(ctx context.Context) (int, int, error) {
	var students, schools int
	err := r.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM users  WHERE role = 'student' AND status != 'deleted'),
		       (SELECT COUNT(*) FROM school WHERE status = 'active')
	`).Scan(&students, &schools)
	return students, schools, err
}
