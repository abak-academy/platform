package repository

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const externalRoundCols = `r.id, r.exam_id, r.label, r.label_en, r.source_key, r.published_at, r.updated_at,
	(SELECT COUNT(*) FROM exam_external_result x WHERE x.round_id = r.id)`

func scanExternalRound(row pgx.Row) (*model.ExternalRound, error) {
	var rd model.ExternalRound
	if err := row.Scan(&rd.ID, &rd.ExamID, &rd.Label, &rd.LabelEN, &rd.SourceKey, &rd.PublishedAt, &rd.UpdatedAt, &rd.RowCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rd, nil
}

// ExternalRegistration is a registration as seen by the participant-number match.
type ExternalRegistration struct {
	ID      uuid.UUID
	Revoked bool
}

// GetExternalMatchData returns the exam's exam_number and its registrations by
// participant_number (revoked ones included, flagged); ErrNotFound for no exam.
func (r *Repository) GetExternalMatchData(ctx context.Context, examID uuid.UUID) (*int, map[int]ExternalRegistration, error) {
	var examNumber *int
	if err := r.pool.QueryRow(ctx, `SELECT exam_number FROM exam WHERE id = $1`, examID).Scan(&examNumber); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT participant_number, id, status = 'revoked' FROM exam_registration
		WHERE exam_id = $1 AND participant_number IS NOT NULL`, examID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	regs := map[int]ExternalRegistration{}
	for rows.Next() {
		var n int
		var reg ExternalRegistration
		if err := rows.Scan(&n, &reg.ID, &reg.Revoked); err != nil {
			return nil, nil, err
		}
		regs[n] = reg
	}
	return examNumber, regs, rows.Err()
}

func (r *Repository) GetExternalRoundByLabel(ctx context.Context, examID uuid.UUID, label string) (*model.ExternalRound, error) {
	return scanExternalRound(r.pool.QueryRow(ctx,
		`SELECT `+externalRoundCols+` FROM exam_external_round r WHERE r.exam_id = $1 AND r.label = $2`, examID, label))
}

func (r *Repository) GetExternalRound(ctx context.Context, roundID uuid.UUID) (*model.ExternalRound, error) {
	return scanExternalRound(r.pool.QueryRow(ctx,
		`SELECT `+externalRoundCols+` FROM exam_external_round r WHERE r.id = $1`, roundID))
}

func (r *Repository) ListExternalRounds(ctx context.Context, examID uuid.UUID) ([]model.ExternalRound, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+externalRoundCols+` FROM exam_external_round r WHERE r.exam_id = $1 ORDER BY r.created_at, r.label`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ExternalRound{}
	for rows.Next() {
		var rd model.ExternalRound
		if err := rows.Scan(&rd.ID, &rd.ExamID, &rd.Label, &rd.LabelEN, &rd.SourceKey, &rd.PublishedAt, &rd.UpdatedAt, &rd.RowCount); err != nil {
			return nil, err
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

// ApplyExternalRound upserts the round by (exam_id, label), replaces all its
// rows, and audits, in one transaction. published_at is left untouched.
func (r *Repository) ApplyExternalRound(ctx context.Context, examID uuid.UUID, label, sourceKey, actorID string, inputs []model.ExternalResultInput) (*model.ExternalRound, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var roundID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO exam_external_round (exam_id, label, source_key, updated_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (exam_id, label) DO UPDATE
		SET source_key = EXCLUDED.source_key, updated_by = EXCLUDED.updated_by, updated_at = now()
		RETURNING id`, examID, label, sourceKey, actorID).Scan(&roundID); err != nil {
		return nil, err
	}
	del, err := tx.Exec(ctx, `DELETE FROM exam_external_result WHERE round_id = $1`, roundID)
	if err != nil {
		return nil, err
	}
	if len(inputs) > 0 {
		batch := &pgx.Batch{}
		for _, in := range inputs {
			batch.Queue(`INSERT INTO exam_external_result (round_id, registration_id, score, rank, award, certificate_url)
				VALUES ($1, $2, $3::numeric, $4, $5, $6)`,
				roundID, in.RegistrationID, in.Score, in.Rank, in.Award, in.CertificateURL)
		}
		br := tx.SendBatch(ctx, batch)
		for range inputs {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return nil, err
			}
		}
		if err := br.Close(); err != nil {
			return nil, err
		}
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "exam_external_round", roundID.String(), "apply", map[string]any{
		"exam_id": examID.String(), "label": label, "rows": len(inputs), "replaced_rows": del.RowsAffected(), "source_key": sourceKey,
	}); err != nil {
		return nil, err
	}
	rd, err := scanExternalRound(tx.QueryRow(ctx, `SELECT `+externalRoundCols+` FROM exam_external_round r WHERE r.id = $1`, roundID))
	if err != nil {
		return nil, err
	}
	return rd, tx.Commit(ctx)
}

// SetExternalRoundPublished publishes (keeping an earlier published_at) or
// unpublishes the round and audits; ErrNotFound for no such round.
func (r *Repository) SetExternalRoundPublished(ctx context.Context, roundID uuid.UUID, publish bool, actorID string) (*model.ExternalRound, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rd, err := scanExternalRound(tx.QueryRow(ctx,
		`UPDATE exam_external_round r SET
			published_at = CASE WHEN $2 THEN COALESCE(r.published_at, now()) END,
			updated_by = $3, updated_at = now()
		WHERE r.id = $1
		RETURNING `+externalRoundCols, roundID, publish, actorID))
	if err != nil {
		return nil, err
	}
	action := "unpublish"
	if publish {
		action = "publish"
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "exam_external_round", roundID.String(), action, map[string]any{
		"exam_id": rd.ExamID.String(), "label": rd.Label, "rows": rd.RowCount,
	}); err != nil {
		return nil, err
	}
	return rd, tx.Commit(ctx)
}

func (r *Repository) DeleteExternalRound(ctx context.Context, roundID uuid.UUID, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rd, err := scanExternalRound(tx.QueryRow(ctx,
		`SELECT `+externalRoundCols+` FROM exam_external_round r WHERE r.id = $1 FOR UPDATE`, roundID))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM exam_external_round WHERE id = $1`, roundID); err != nil {
		return err
	}
	source := ""
	if rd.SourceKey != nil {
		source = *rd.SourceKey
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "exam_external_round", roundID.String(), "delete", map[string]any{
		"exam_id": rd.ExamID.String(), "label": rd.Label, "rows": rd.RowCount, "source_key": source,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListExternalResults(ctx context.Context, roundID uuid.UUID) ([]model.ExternalResultRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT x.registration_id, u.name, x.score::float8, x.rank, x.award, x.certificate_url,
			reg.participant_number, e.exam_number, e.scheduled_at, reg.created_at
		FROM exam_external_result x
		JOIN exam_registration reg ON reg.id = x.registration_id
		JOIN exam e ON e.id = reg.exam_id
		JOIN users u ON u.id = reg.student_id
		WHERE x.round_id = $1
		ORDER BY x.rank NULLS LAST, x.score DESC, reg.participant_number`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ExternalResultRow{}
	for rows.Next() {
		var row model.ExternalResultRow
		var createdAt time.Time
		if err := rows.Scan(&row.RegistrationID, &row.StudentName, &row.Score, &row.Rank, &row.Award, &row.CertificateURL,
			&row.ParticipantNumber, &row.ExamNumber, &row.ExamScheduledAt, &createdAt); err != nil {
			return nil, err
		}
		row.DisplayDate = createdAt
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListPublishedExternalResults returns published-round results for the given
// registrations in one query, ordered by published_at.
func (r *Repository) ListPublishedExternalResults(ctx context.Context, registrationIDs []uuid.UUID) ([]model.StudentExternalResult, error) {
	if len(registrationIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT x.registration_id, rd.id, rd.label, rd.label_en, x.score::float8, x.rank, x.award, x.certificate_url, rd.published_at
		FROM exam_external_result x
		JOIN exam_external_round rd ON rd.id = x.round_id
		WHERE x.registration_id = ANY($1) AND rd.published_at IS NOT NULL
		ORDER BY rd.published_at, rd.label`, registrationIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StudentExternalResult
	for rows.Next() {
		var x model.StudentExternalResult
		if err := rows.Scan(&x.RegistrationID, &x.RoundID, &x.RoundLabel, &x.LabelEN, &x.Score, &x.Rank, &x.Award, &x.CertificateURL, &x.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
