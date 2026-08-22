package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// AssessmentFilter carries the optional q/school_id filters shared by the
// paginated row list and the unpaginated summary aggregate — both must see
// the same filtered cohort (Issue 124 spec: "Summary and table filters must
// match").
type AssessmentFilter struct {
	Q        string
	SchoolID *uuid.UUID
	Cursor   string
	Limit    int
}

// assessmentLatestSession is the authoritative status/latest-attempt CTE:
// newest by attempt_number DESC, tie-broken by started_at DESC then id DESC.
// References $1 = exam_id.
const assessmentLatestSession = `(
	SELECT DISTINCT ON (s.registration_id) s.registration_id, s.id AS session_id,
		s.attempt_number, s.status, s.submitted_at, s.score
	FROM exam_session s
	WHERE s.exam_id = $1
	ORDER BY s.registration_id, s.attempt_number DESC, s.started_at DESC, s.id DESC
)`

// assessmentFullyGraded mirrors the leaderboard fullyGradedFilter for a concrete
// exam_session alias.
const assessmentFullyGraded = `NOT EXISTS (
	SELECT 1 FROM exam_session_answer a
	JOIN question q ON q.id = a.question_id
	WHERE a.session_id = s.id AND q.format = 'essay' AND a.graded_at IS NULL
)`

// assessmentLatestScoredSession matches the leaderboard semantics: filter to
// submitted + scored + fully graded sessions first, then pick the newest scored
// attempt per registration. This keeps Assessment score/rank/average aligned
// with ListExamLeaderboard/GetFullyGradedScores even when a student starts a
// newer in-progress or ungraded retry.
const assessmentLatestScoredSession = `(
	SELECT DISTINCT ON (s.registration_id) s.registration_id, s.id AS session_id,
		s.attempt_number, s.submitted_at, s.score
	FROM exam_session s
	WHERE s.exam_id = $1 AND s.status = 'submitted' AND s.score IS NOT NULL AND ` + assessmentFullyGraded + `
	ORDER BY s.registration_id, s.attempt_number DESC, s.started_at DESC, s.id DESC
)`

func (f AssessmentFilter) whereRegistration(startArg int) (string, []any) {
	clause := ""
	args := []any{}
	arg := startArg
	if f.SchoolID != nil {
		clause += fmt.Sprintf(" AND u.school_id = $%d", arg)
		args = append(args, *f.SchoolID)
		arg++
	}
	if f.Q != "" {
		clause += fmt.Sprintf(" AND (u.name ILIKE $%d OR u.username ILIKE $%d)", arg, arg)
		args = append(args, "%"+f.Q+"%")
		arg++
	}
	return clause, args
}

// ListAssessmentRows returns the paginated participant rows for the assessment
// workspace (Issue 124): one row per exam_registration, with latest-attempt
// status plus leaderboard-compatible score/rank from the newest submitted,
// fully graded, scored attempt. Rank is computed over the full filtered cohort
// (not just the current page) so page-2 ranks stay correct. Cursor is keyset-
// encoded as "<RFC3339Nano created_at>,<registration id>", default limit 20,
// cap 100.
func (r *Repository) ListAssessmentRows(ctx context.Context, examID uuid.UUID, filter AssessmentFilter) ([]model.AssessmentRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	regClause, regArgs := filter.whereRegistration(2)
	args := []any{examID}
	args = append(args, regArgs...)
	argIdx := len(args) + 1

	query := `WITH reg AS (
			SELECT reg.id, reg.student_id, reg.created_at, u.name AS student_name, u.username,
				u.school_id, COALESCE(sc.name, u.unlisted_school_name) AS school_name
			FROM exam_registration reg
			JOIN users u ON u.id = reg.student_id AND u.role = 'student'
			LEFT JOIN school sc ON sc.id = u.school_id
			WHERE reg.exam_id = $1` + regClause + `
		),
		latest AS ` + assessmentLatestSession + `,
		scored AS ` + assessmentLatestScoredSession + `,
		attempts_agg AS (
			SELECT registration_id, COUNT(*) AS attempts_count
			FROM exam_session
			WHERE exam_id = $1
			GROUP BY registration_id
		),
		latest_violations AS (
			SELECT l.session_id, COUNT(*) AS cnt
			FROM session_violation_log l
			JOIN exam_session s ON s.id = l.session_id
			WHERE s.exam_id = $1
			GROUP BY l.session_id
		),
		joined AS (
			SELECT reg.id AS registration_id, reg.student_id, reg.created_at, reg.student_name, reg.username,
				reg.school_id, reg.school_name,
				latest.session_id, latest.attempt_number, latest.status AS session_status,
				latest.submitted_at, scored.score,
				COALESCE(attempts_agg.attempts_count, 0) AS attempts_count,
				COALESCE(latest_violations.cnt, 0) AS latest_violations,
				(scored.session_id IS NOT NULL) AS eligible
			FROM reg
			LEFT JOIN latest ON latest.registration_id = reg.id
			LEFT JOIN scored ON scored.registration_id = reg.id
			LEFT JOIN attempts_agg ON attempts_agg.registration_id = reg.id
			LEFT JOIN latest_violations ON latest_violations.session_id = latest.session_id
		),
		ranked AS (
			SELECT joined.*,
				CASE WHEN eligible THEN RANK() OVER (PARTITION BY eligible ORDER BY score DESC) END AS rnk
			FROM joined
		)
		SELECT registration_id, student_id, created_at, student_name, username, school_id, school_name,
			rnk, CASE WHEN eligible THEN score END, session_status, session_id, attempt_number,
			submitted_at, attempts_count, latest_violations
		FROM ranked
		WHERE 1=1`

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
		query += fmt.Sprintf(` AND (created_at > $%d OR (created_at = $%d AND registration_id > $%d))`, argIdx, argIdx, argIdx+1)
		args = append(args, cursorTime, cursorID)
		argIdx += 2
	}

	query += ` ORDER BY created_at ASC, registration_id ASC LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var createdAts []time.Time
	results := []model.AssessmentRow{}
	for rows.Next() {
		var row model.AssessmentRow
		var createdAt time.Time
		var sessionStatus *string
		if err := rows.Scan(
			&row.RegistrationID, &row.StudentID, &createdAt, &row.StudentName, &row.Username, &row.SchoolID, &row.SchoolName,
			&row.Rank, &row.Score, &sessionStatus, &row.LatestSessionID, &row.LatestAttemptNumber,
			&row.LatestSubmittedAt, &row.AttemptsCount, &row.LatestViolations,
		); err != nil {
			return nil, "", err
		}
		switch {
		case row.LatestSessionID == nil:
			row.Status = "not_started"
		case sessionStatus != nil && *sessionStatus == "submitted":
			row.Status = "completed"
		default:
			row.Status = "in_progress"
		}
		createdAts = append(createdAts, createdAt)
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > filter.Limit {
		results = results[:filter.Limit]
		createdAts = createdAts[:filter.Limit]
		last := results[len(results)-1]
		nextCursor = createdAts[len(createdAts)-1].Format(time.RFC3339Nano) + "," + last.RegistrationID.String()
	}

	return results, nextCursor, nil
}

// GetAssessmentSummary computes the aggregate card block for the assessment
// workspace over the full filtered cohort (q/school_id), independent of
// pagination. AverageScore/Distribution cover only the latest-attempt,
// submitted, fully-graded, scored rows; ViolationAttempts/ViolationEvents
// count across every attempt of the filtered registrations, not just the
// latest (Issue 124 spec). Scores mirror leaderboard semantics: newest
// submitted + fully graded scored attempt per registration.
func (r *Repository) GetAssessmentSummary(ctx context.Context, examID uuid.UUID, filter AssessmentFilter) (total int, completed int, scores []float64, violationAttempts int, violationEvents int, err error) {
	regClause, regArgs := filter.whereRegistration(2)
	args := []any{examID}
	args = append(args, regArgs...)

	err = r.pool.QueryRow(ctx,
		`WITH reg AS (
			SELECT reg.id
			FROM exam_registration reg
			JOIN users u ON u.id = reg.student_id AND u.role = 'student'
			WHERE reg.exam_id = $1`+regClause+`
		),
		latest AS `+assessmentLatestSession+`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE latest.status = 'submitted')
		FROM reg
		LEFT JOIN latest ON latest.registration_id = reg.id`,
		args...,
	).Scan(&total, &completed)
	if err != nil {
		return 0, 0, nil, 0, 0, err
	}

	scoreRows, err := r.pool.Query(ctx,
		`WITH reg AS (
			SELECT reg.id
			FROM exam_registration reg
			JOIN users u ON u.id = reg.student_id AND u.role = 'student'
			WHERE reg.exam_id = $1`+regClause+`
		),
		scored AS `+assessmentLatestScoredSession+`
		SELECT latest.score
		FROM reg
		JOIN scored latest ON latest.registration_id = reg.id`,
		args...,
	)
	if err != nil {
		return 0, 0, nil, 0, 0, err
	}
	defer scoreRows.Close()
	for scoreRows.Next() {
		var score float64
		if err := scoreRows.Scan(&score); err != nil {
			return 0, 0, nil, 0, 0, err
		}
		scores = append(scores, score)
	}
	if err := scoreRows.Err(); err != nil {
		return 0, 0, nil, 0, 0, err
	}

	err = r.pool.QueryRow(ctx,
		`WITH reg AS (
			SELECT reg.id
			FROM exam_registration reg
			JOIN users u ON u.id = reg.student_id AND u.role = 'student'
			WHERE reg.exam_id = $1`+regClause+`
		)
		SELECT COUNT(DISTINCT s.id), COUNT(l.id)
		FROM reg
		JOIN exam_session s ON s.registration_id = reg.id
		JOIN session_violation_log l ON l.session_id = s.id`,
		args...,
	).Scan(&violationAttempts, &violationEvents)
	if err != nil {
		return 0, 0, nil, 0, 0, err
	}

	return total, completed, scores, violationAttempts, violationEvents, nil
}

// ListAssessmentAttempts returns every exam_session for a registration,
// newest-first, with a per-session violation count and IsLatest marking the
// same session ListAssessmentRows treats as authoritative for that
// registration. Returns ErrNotFound if the registration does not exist or
// does not belong to examID.
func (r *Repository) ListAssessmentAttempts(ctx context.Context, examID, registrationID uuid.UUID) ([]model.AssessmentAttempt, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_registration WHERE id = $1 AND exam_id = $2)`,
		registrationID, examID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.attempt_number, s.status, s.submitted_at, s.score,
			COALESCE((SELECT COUNT(*) FROM session_violation_log l WHERE l.session_id = s.id), 0),
			(s.status = 'submitted' AND s.score IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM exam_session_answer a
				JOIN question q ON q.id = a.question_id
				WHERE a.session_id = s.id AND q.format = 'essay' AND a.graded_at IS NULL
			)) AS result_available,
			s.id = (
				SELECT s2.id FROM exam_session s2
				WHERE s2.registration_id = $1
				ORDER BY s2.attempt_number DESC, s2.started_at DESC, s2.id DESC
				LIMIT 1
			) AS is_latest
		FROM exam_session s
		WHERE s.registration_id = $1
		ORDER BY s.attempt_number DESC`,
		registrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attempts := []model.AssessmentAttempt{}
	for rows.Next() {
		var a model.AssessmentAttempt
		if err := rows.Scan(&a.SessionID, &a.AttemptNumber, &a.Status, &a.SubmittedAt, &a.Score, &a.Violations, &a.ResultAvailable, &a.IsLatest); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attempts, nil
}
