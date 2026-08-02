package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// AdminResultFilter carries optional filters for ListSchoolResults.
type AdminResultFilter struct {
	Q      string
	Cursor string
	Limit  int
}

// ListSchoolResults returns fully-graded submitted sessions for an exam, optionally
// scoped to a single school. An empty schoolID means "all schools" (the super_admin
// unscoped view). Cursor is keyset-encoded as "<RFC3339Nano submitted_at>,<session id>"
// ordered by submitted_at DESC, id ASC. Default limit 20, cap 100.
func (r *Repository) ListSchoolResults(ctx context.Context, examID uuid.UUID, schoolID string, filter AdminResultFilter) ([]model.AdminResultRow, string, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}

	query := `SELECT s.id, u.name, u.username, s.score, s.submitted_at, COALESCE(sc.name, u.unlisted_school_name)
		FROM exam_session s
		JOIN users u ON u.id = s.student_id AND u.role = 'student'
		LEFT JOIN school sc ON sc.id = u.school_id
		WHERE s.exam_id = $2 AND s.status = 'submitted' AND ($1::uuid IS NULL OR u.school_id = $1) AND ` + fullyGradedFilter
	var schoolArg *string
	if schoolID != "" {
		schoolArg = &schoolID
	}
	args := []any{schoolArg, examID}
	argIdx := 3

	if filter.Q != "" {
		query += fmt.Sprintf(` AND (u.name ILIKE $%d OR u.username ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+filter.Q+"%")
		argIdx++
	}

	if filter.Cursor != "" {
		timeStr, idStr, found := strings.Cut(filter.Cursor, ",")
		if !found {
			return nil, "", fmt.Errorf("%w: %q", ErrInvalidCursor, filter.Cursor)
		}
		cursorTime, err := time.Parse(time.RFC3339Nano, timeStr)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrInvalidCursor, err)
		}
		cursorID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrInvalidCursor, err)
		}
		query += fmt.Sprintf(` AND (s.submitted_at < $%d OR (s.submitted_at = $%d AND s.id > $%d))`, argIdx, argIdx, argIdx+1)
		args = append(args, cursorTime, cursorID)
		argIdx += 2
	}

	query += ` ORDER BY s.submitted_at DESC, s.id ASC LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	results := []model.AdminResultRow{}
	for rows.Next() {
		var row model.AdminResultRow
		if err := rows.Scan(&row.SessionID, &row.StudentName, &row.Username, &row.Score, &row.SubmittedAt, &row.SchoolName); err != nil {
			return nil, "", err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > filter.Limit {
		results = results[:filter.Limit]
		last := results[filter.Limit-1]
		if last.SubmittedAt != nil {
			nextCursor = last.SubmittedAt.Format(time.RFC3339Nano) + "," + last.SessionID.String()
		}
	}

	return results, nextCursor, nil
}

// GetSchoolResultSession returns a single session result, optionally scoped to a
// school. An empty schoolID means "all schools" (the super_admin unscoped view).
// No status=submitted filter — the service layer needs the actual status value
// to run resultGate / isFullyGraded. Returns ErrNotFound when the session
// doesn't exist or belongs to a different school (indistinguishable).
func (r *Repository) GetSchoolResultSession(ctx context.Context, sessionID uuid.UUID, schoolID string) (*model.AdminResultSession, error) {
	var s model.AdminResultSession
	var schoolArg *string
	if schoolID != "" {
		schoolArg = &schoolID
	}
	err := r.pool.QueryRow(ctx,
		`SELECT s.id, s.exam_id, s.student_id, u.name, u.username, s.status, s.score, s.submitted_at
		FROM exam_session s
		JOIN users u ON u.id = s.student_id AND u.role = 'student'
		WHERE s.id = $1 AND ($2::uuid IS NULL OR u.school_id = $2)`,
		sessionID, schoolArg,
	).Scan(
		&s.SessionID, &s.ExamID, &s.StudentID, &s.StudentName, &s.Username,
		&s.Status, &s.Score, &s.SubmittedAt,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}
