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

// CountActiveExamSessions counts in-progress sessions across every exam. The
// session monitor is per-exam (it requires an exam_id), which is why the
// dashboard had no global number to show and hardcoded a dash.
func (r *Repository) CountActiveExamSessions(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE status = 'in_progress'`,
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
