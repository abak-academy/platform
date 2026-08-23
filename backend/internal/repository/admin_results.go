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

// ListDetailedExportRows returns export rows with per-question details using
// latest-scored semantics. Applies school scope, search query, and ranking
// shared with the Results Workspace (Issue 130).
func (r *Repository) ListDetailedExportRows(ctx context.Context, examID uuid.UUID, schoolID, q string) ([]model.AdminExportRow, error) {
	// Use resultsWorkspaceLatestScoredSession CTE for latest-scored ranking alignment.
	query := `WITH
		scored AS (
			SELECT DISTINCT ON (s.registration_id) s.registration_id, s.id AS session_id,
				s.attempt_number, s.submitted_at, s.started_at, s.score
			FROM exam_session s
			WHERE s.exam_id = $1 AND s.status = 'submitted' AND s.score IS NOT NULL AND ` + resultsWorkspaceFullyGraded + `
			ORDER BY s.registration_id, s.attempt_number DESC, s.started_at DESC, s.id DESC
		),
		joined AS (
			SELECT reg.id AS registration_id, scored.session_id, u.name AS student_name, u.username,
				COALESCE(sc.name, u.unlisted_school_name) AS school_name,
				scored.submitted_at, scored.started_at, scored.score, reg.created_at
			FROM exam_registration reg
			JOIN users u ON u.id = reg.student_id AND u.role = 'student'
			LEFT JOIN school sc ON sc.id = u.school_id
			JOIN scored ON scored.registration_id = reg.id
			WHERE reg.exam_id = $1
				AND ($2::text = '' OR u.school_id = $2::uuid)
				AND ($3::text = '' OR u.name ILIKE '%' || $3 || '%' OR u.username ILIKE '%' || $3 || '%')
		),
		ranked AS (
			SELECT joined.*, RANK() OVER (ORDER BY score DESC) AS rnk
			FROM joined
		)
		SELECT registration_id, session_id, student_name, username, school_name, rnk, score,
			submitted_at, started_at
		FROM ranked
		ORDER BY rnk ASC, score DESC, registration_id ASC`

	rows, err := r.pool.Query(ctx, query, examID, schoolID, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []model.AdminExportRow{}
	sessionIDs := []uuid.UUID{}
	for rows.Next() {
		var row model.AdminExportRow
		if err := rows.Scan(&row.RegistrationID, &row.SessionID, &row.StudentName, &row.Username,
			&row.SchoolName, &row.Rank, &row.Score, &row.SubmittedAt, &row.StartedAt); err != nil {
			return nil, err
		}
		results = append(results, row)
		sessionIDs = append(sessionIDs, row.SessionID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if len(results) == 0 {
		return results, nil
	}

	// Fetch per-question answers in bounded session batches after the ranking rows
	// are closed. This avoids holding one pool connection while waiting for another
	// per participant during concurrent exports, without building an unbounded
	// sessions x questions result in one query.
	answersBySession := make(map[uuid.UUID][]model.AdminExportQuestionRow, len(results))
	const exportAnswerBatchSize = 100
	for start := 0; start < len(sessionIDs); start += exportAnswerBatchSize {
		end := start + exportAnswerBatchSize
		if end > len(sessionIDs) {
			end = len(sessionIDs)
		}

		qrows, err := r.pool.Query(ctx,
			`SELECT exported.session_id, q.id, q.question_number, q.format, a.answer, a.score, a.is_correct
			FROM exam_test et
			JOIN test_question tq ON tq.test_id = et.test_id
			JOIN question q ON q.id = tq.question_id
			CROSS JOIN unnest($2::uuid[]) AS exported(session_id)
			LEFT JOIN exam_session_answer a ON a.question_id = q.id AND a.session_id = exported.session_id
			WHERE et.exam_id = $1
			ORDER BY exported.session_id, et.sort_order ASC, tq.sort_order ASC, q.question_number ASC`,
			examID, sessionIDs[start:end])
		if err != nil {
			return nil, err
		}

		for qrows.Next() {
			var sessionID uuid.UUID
			var qa model.AdminExportQuestionRow
			if err := qrows.Scan(&sessionID, &qa.QuestionID, &qa.QuestionNum, &qa.Format, &qa.StudentAnswer, &qa.Points, &qa.IsCorrect); err != nil {
				qrows.Close()
				return nil, err
			}
			answersBySession[sessionID] = append(answersBySession[sessionID], qa)
		}
		if err := qrows.Err(); err != nil {
			qrows.Close()
			return nil, err
		}
		qrows.Close()
	}

	for i := range results {
		results[i].QuestionRows = answersBySession[results[i].SessionID]
	}

	return results, nil
}
