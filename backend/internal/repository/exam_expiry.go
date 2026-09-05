package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

func (r *Repository) ListDueExamExpiryCandidates(ctx context.Context, now time.Time, limit int) ([]model.ExamExpiryCandidate, error) {
	return r.ListDueExamExpiryCandidatesPage(ctx, now, limit, 0)
}

func (r *Repository) ListDueExamExpiryCandidatesPage(ctx context.Context, now time.Time, limit, offset int) ([]model.ExamExpiryCandidate, error) {
	if limit <= 0 {
		return []model.ExamExpiryCandidate{}, nil
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx,
		`WITH candidates AS (
			SELECT
				s.id AS session_id,
				NULL::uuid AS test_id,
				GREATEST(
					s.started_at + e.duration_minutes * interval '1 minute',
					COALESCE(s.extended_until, '-infinity'::timestamptz)
				) + COALESCE(e.grace_window_minutes, 0) * interval '1 minute' AS due_at
			FROM exam_session s
			JOIN exam e ON e.id = s.exam_id
			WHERE s.status = 'in_progress'
			  AND e.mode = 'standard'
			  AND e.duration_minutes > 0

			UNION ALL

			SELECT
				s.id AS session_id,
				ss.test_id,
				GREATEST(
					ss.started_at + ss.duration_minutes * interval '1 minute',
					COALESCE(ss.extended_until, '-infinity'::timestamptz)
				) + COALESCE(e.grace_window_minutes, 0) * interval '1 minute' AS due_at
			FROM exam_session s
			JOIN exam e ON e.id = s.exam_id
			JOIN exam_session_section ss ON ss.session_id = s.id
			WHERE s.status = 'in_progress'
			  AND e.mode IN ('utbk', 'ielts')
			  AND ss.status = 'active'
			  AND ss.started_at IS NOT NULL
			  AND ss.duration_minutes > 0
		)
		SELECT session_id, test_id, due_at
		FROM candidates
		WHERE due_at <= $1
		ORDER BY due_at ASC, session_id ASC, test_id ASC NULLS FIRST
		LIMIT $2 OFFSET $3`,
		now, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ExamExpiryCandidate, 0)
	for rows.Next() {
		var c model.ExamExpiryCandidate
		var testID *uuid.UUID
		if err := rows.Scan(&c.SessionID, &testID, &c.DueAt); err != nil {
			return nil, err
		}
		c.TestID = testID
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetExamByIDTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Exam, error) {
	out := &model.Exam{}
	err := scanExam(tx.QueryRow(ctx,
		`SELECT id, title, is_free, scheduled_at, scheduled_end_at, requires_checkin, allow_leaderboard,
			cdn_bundle, bundle_url, bundle_generated_at, check_in_window_minutes, grace_window_minutes,
			max_attempts, timer_mode, duration_minutes, randomize, result_config, result_release_at,
			status, created_at, mode,
			certificate_design, certificate_design_updated_at, exam_number, certificate_enabled, certificate_template_html,
			end_screen_image_url, end_screen_promo_text, card_enabled, card_notes
		FROM exam
		WHERE id = $1`,
		id,
	), out)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetActiveSessionSectionForUpdateTx(ctx context.Context, tx pgx.Tx, sessionID, testID uuid.UUID) (*model.ExamSessionSection, error) {
	var section model.ExamSessionSection
	err := scanExamSessionSection(tx.QueryRow(ctx,
		`SELECT session_id, test_id, sort_order, duration_minutes, status, started_at, submitted_at, extended_until
		FROM exam_session_section
		WHERE session_id = $1 AND test_id = $2 AND status = 'active'
		FOR UPDATE`,
		sessionID, testID,
	), &section)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNoActiveSection
		}
		return nil, err
	}
	return &section, nil
}
