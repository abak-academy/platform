package repository

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const competitionResultCols = `r.id, r.product_id, r.source_key, r.published_at, r.updated_at,
	(SELECT COUNT(*) FROM competition_result_row x WHERE x.result_id = r.id)`

func scanCompetitionResult(row pgx.Row) (*model.CompetitionResult, error) {
	var res model.CompetitionResult
	if err := row.Scan(&res.ID, &res.ProductID, &res.SourceKey, &res.PublishedAt, &res.UpdatedAt, &res.RowCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &res, nil
}

// CompetitionRegistration is a registration as seen by the participant-number match.
type CompetitionRegistration struct {
	ID      uuid.UUID
	Revoked bool
}

// GetCompetitionMatchData returns, for every exam attached to the product, its
// registrations by participant_number (revoked ones included, flagged), keyed
// by exam_number then participant_number; ErrNotFound for no product.
func (r *Repository) GetCompetitionMatchData(ctx context.Context, productID uuid.UUID) (map[int]map[int]CompetitionRegistration, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM product WHERE id = $1)`, productID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := r.pool.Query(ctx,
		`SELECT e.exam_number, reg.participant_number, reg.id, COALESCE(reg.status = 'revoked', false)
		FROM product_exam pe
		JOIN exam e ON e.id = pe.exam_id
		LEFT JOIN exam_registration reg ON reg.exam_id = e.id AND reg.participant_number IS NOT NULL
		WHERE pe.product_id = $1`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]map[int]CompetitionRegistration{}
	for rows.Next() {
		var examNo int
		var n *int
		var id *uuid.UUID
		var revoked bool
		if err := rows.Scan(&examNo, &n, &id, &revoked); err != nil {
			return nil, err
		}
		if out[examNo] == nil {
			out[examNo] = map[int]CompetitionRegistration{}
		}
		if n != nil && id != nil {
			out[examNo][*n] = CompetitionRegistration{ID: *id, Revoked: revoked}
		}
	}
	return out, rows.Err()
}

func (r *Repository) GetCompetitionResult(ctx context.Context, productID uuid.UUID) (*model.CompetitionResult, error) {
	return scanCompetitionResult(r.pool.QueryRow(ctx,
		`SELECT `+competitionResultCols+` FROM competition_result r WHERE r.product_id = $1`, productID))
}

// ApplyCompetitionResult upserts the product's result set, replaces all its
// rows, and audits, in one transaction. published_at is left untouched.
func (r *Repository) ApplyCompetitionResult(ctx context.Context, productID uuid.UUID, sourceKey, actorID string, inputs []model.CompetitionResultInput) (*model.CompetitionResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var resultID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO competition_result (product_id, source_key, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (product_id) DO UPDATE
		SET source_key = EXCLUDED.source_key, updated_by = EXCLUDED.updated_by, updated_at = now()
		RETURNING id`, productID, sourceKey, actorID).Scan(&resultID); err != nil {
		return nil, err
	}
	del, err := tx.Exec(ctx, `DELETE FROM competition_result_row WHERE result_id = $1`, resultID)
	if err != nil {
		return nil, err
	}
	if len(inputs) > 0 {
		batch := &pgx.Batch{}
		for _, in := range inputs {
			batch.Queue(`INSERT INTO competition_result_row (result_id, registration_id, score, rank, award, certificate_url)
				VALUES ($1, $2, $3::numeric, $4, $5, $6)`,
				resultID, in.RegistrationID, in.Score, in.Rank, in.Award, in.CertificateURL)
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
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "competition_result", resultID.String(), "apply", map[string]any{
		"product_id": productID.String(), "rows": len(inputs), "replaced_rows": del.RowsAffected(), "source_key": sourceKey,
	}); err != nil {
		return nil, err
	}
	res, err := scanCompetitionResult(tx.QueryRow(ctx, `SELECT `+competitionResultCols+` FROM competition_result r WHERE r.id = $1`, resultID))
	if err != nil {
		return nil, err
	}
	return res, tx.Commit(ctx)
}

// SetCompetitionResultPublished publishes (keeping an earlier published_at) or
// unpublishes the product's result set and audits; ErrNotFound for none.
func (r *Repository) SetCompetitionResultPublished(ctx context.Context, productID uuid.UUID, publish bool, actorID string) (*model.CompetitionResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res, err := scanCompetitionResult(tx.QueryRow(ctx,
		`UPDATE competition_result r SET
			published_at = CASE WHEN $2 THEN COALESCE(r.published_at, now()) END,
			updated_by = $3, updated_at = now()
		WHERE r.product_id = $1
		RETURNING `+competitionResultCols, productID, publish, actorID))
	if err != nil {
		return nil, err
	}
	action := "unpublish"
	if publish {
		action = "publish"
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "competition_result", res.ID.String(), action, map[string]any{
		"product_id": productID.String(), "rows": res.RowCount,
	}); err != nil {
		return nil, err
	}
	return res, tx.Commit(ctx)
}

func (r *Repository) DeleteCompetitionResult(ctx context.Context, productID uuid.UUID, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	res, err := scanCompetitionResult(tx.QueryRow(ctx,
		`SELECT `+competitionResultCols+` FROM competition_result r WHERE r.product_id = $1 FOR UPDATE`, productID))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM competition_result WHERE id = $1`, res.ID); err != nil {
		return err
	}
	source := ""
	if res.SourceKey != nil {
		source = *res.SourceKey
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "competition_result", res.ID.String(), "delete", map[string]any{
		"product_id": productID.String(), "rows": res.RowCount, "source_key": source,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListCompetitionResultRows(ctx context.Context, resultID uuid.UUID) ([]model.CompetitionResultRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT x.registration_id, u.name, x.score::float8, x.rank, x.award, x.certificate_url,
			reg.participant_number, e.exam_number, e.scheduled_at, reg.created_at,
			`+examEventPrefixSQL("e.id")+`
		FROM competition_result_row x
		JOIN exam_registration reg ON reg.id = x.registration_id
		JOIN exam e ON e.id = reg.exam_id
		JOIN users u ON u.id = reg.student_id
		WHERE x.result_id = $1
		ORDER BY x.rank NULLS LAST, x.score DESC, reg.participant_number`, resultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CompetitionResultRow{}
	for rows.Next() {
		var row model.CompetitionResultRow
		var createdAt time.Time
		if err := rows.Scan(&row.RegistrationID, &row.StudentName, &row.Score, &row.Rank, &row.Award, &row.CertificateURL,
			&row.ParticipantNumber, &row.ExamNumber, &row.ExamScheduledAt, &createdAt, &row.ExamPrefix); err != nil {
			return nil, err
		}
		row.DisplayDate = createdAt
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListPublishedCompetitionResults returns the published results for the given
// registrations in one query.
func (r *Repository) ListPublishedCompetitionResults(ctx context.Context, registrationIDs []uuid.UUID) ([]model.StudentCompetitionResult, error) {
	if len(registrationIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT x.registration_id, x.score::float8, x.rank, x.award, x.certificate_url, r.published_at
		FROM competition_result_row x
		JOIN competition_result r ON r.id = x.result_id
		WHERE x.registration_id = ANY($1) AND r.published_at IS NOT NULL
		ORDER BY r.published_at`, registrationIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StudentCompetitionResult
	for rows.Next() {
		var x model.StudentCompetitionResult
		if err := rows.Scan(&x.RegistrationID, &x.Score, &x.Rank, &x.Award, &x.CertificateURL, &x.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
