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

type SchoolDashboardCounts struct {
	Students         int `json:"students"`
	NewStudentsMonth int `json:"new_students_month"`
}

func (r *Repository) SchoolDashboardCounts(
	ctx context.Context, schoolID *string, monthStart, monthEnd time.Time,
) (SchoolDashboardCounts, error) {
	var c SchoolDashboardCounts
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3)
		  FROM users
		 WHERE role = 'student'
		   AND status != 'deleted'
		   -- nil schoolID must stay NULL::uuid here, not '' — casting '' to uuid errors.
		   AND ($1::uuid IS NULL OR school_id = $1::uuid)
	`, schoolID, monthStart, monthEnd).Scan(&c.Students, &c.NewStudentsMonth)
	return c, err
}

type LatestBulkOrder struct {
	ID               uuid.UUID `json:"id"`
	Status           string    `json:"status"`
	Total            float64   `json:"total"`
	ParticipantCount int       `json:"participant_count"`
	PlacedAt         time.Time `json:"placed_at"`
}

func (r *Repository) LatestBulkExamOrder(ctx context.Context, schoolID *string) (*LatestBulkOrder, error) {
	var o LatestBulkOrder
	err := r.pool.QueryRow(ctx, `
		SELECT o.id,
		       o.status,
		       COALESCE(o.total, 0),
		       (SELECT COUNT(*) FROM order_participant p WHERE p.order_id = o.id),
		       COALESCE(o.checked_out_at, o.created_at)
		  FROM orders o
		 WHERE o.status <> 'cart'
		   -- EXISTS keeps this a scalar test; a top-level JOIN would fan out the order row per participant.
		   AND EXISTS (
		         SELECT 1
		           FROM order_participant p
		           JOIN users u ON u.id = p.student_id
		          WHERE p.order_id = o.id
		            AND ($1::uuid IS NULL OR u.school_id = $1::uuid)
		       )
		 ORDER BY COALESCE(o.checked_out_at, o.created_at) DESC
		 LIMIT 1
	`, schoolID).Scan(&o.ID, &o.Status, &o.Total, &o.ParticipantCount, &o.PlacedAt)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	o.PlacedAt = o.PlacedAt.In(jakarta)
	return &o, nil
}

type RecentResult struct {
	SessionID   uuid.UUID `json:"session_id"`
	StudentName string    `json:"student_name"`
	ExamTitle   string    `json:"exam_title"`
	Score       *float64  `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

func (r *Repository) RecentSchoolResults(
	ctx context.Context, schoolID *string, limit int,
) ([]RecentResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT es.id, u.name, e.title, es.score, es.submitted_at
		  FROM exam_session es
		  JOIN users u ON u.id = es.student_id
		  JOIN exam  e ON e.id = es.exam_id
		 WHERE es.submitted_at IS NOT NULL
		   AND u.status != 'deleted'
		   AND ($1::uuid IS NULL OR u.school_id = $1::uuid)
		 ORDER BY es.submitted_at DESC
		 LIMIT $2
	`, schoolID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// A submitted-but-ungraded essay session must not read as a score of 0.
	out := make([]RecentResult, 0, limit)
	for rows.Next() {
		var res RecentResult
		if err := rows.Scan(
			&res.SessionID, &res.StudentName, &res.ExamTitle, &res.Score, &res.SubmittedAt,
		); err != nil {
			return nil, err
		}
		res.SubmittedAt = res.SubmittedAt.In(jakarta)
		out = append(out, res)
	}
	return out, rows.Err()
}
