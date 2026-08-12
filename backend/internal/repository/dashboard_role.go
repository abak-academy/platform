package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExamDashboardCounts struct {
	Questions int `json:"questions"`
	Tests     int `json:"tests"`
	Exams     int `json:"exams"`
	Courses   int `json:"courses"`
}

func (r *Repository) ExamDashboardCounts(ctx context.Context) (ExamDashboardCounts, error) {
	var c ExamDashboardCounts
	err := r.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM question),
		       (SELECT COUNT(*) FROM test),
		       (SELECT COUNT(*) FROM exam),
		       (SELECT COUNT(*) FROM course)
	`).Scan(&c.Questions, &c.Tests, &c.Exams, &c.Courses)
	return c, err
}

type RecentViolation struct {
	SessionID     uuid.UUID `json:"session_id"`
	ExamID        uuid.UUID `json:"exam_id"`
	ExamTitle     string    `json:"exam_title"`
	StudentName   string    `json:"student_name"`
	ViolationType string    `json:"violation_type"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// RecentViolationsGlobal is the cross-exam counterpart of GetRecentViolations,
// which takes an exam_id and so cannot answer "what just happened anywhere".
func (r *Repository) RecentViolationsGlobal(ctx context.Context, limit int) ([]RecentViolation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.session_id, e.id, e.title, u.name, v.violation_type, v.occurred_at
		  FROM session_violation_log v
		  JOIN exam_session es ON es.id = v.session_id
		  JOIN exam e          ON e.id  = es.exam_id
		  JOIN users u         ON u.id  = v.student_id
		 ORDER BY v.occurred_at DESC
		 LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RecentViolation, 0, limit)
	for rows.Next() {
		var v RecentViolation
		if err := rows.Scan(
			&v.SessionID, &v.ExamID, &v.ExamTitle, &v.StudentName, &v.ViolationType, &v.OccurredAt,
		); err != nil {
			return nil, err
		}
		v.OccurredAt = v.OccurredAt.In(jakarta)
		out = append(out, v)
	}
	return out, rows.Err()
}
