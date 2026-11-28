package repository

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const competitionResultCols = `r.id, r.exam_id, r.source_key, r.published_at, r.updated_at,
	(SELECT COUNT(*) FROM competition_result_row x WHERE x.result_id = r.id)`

func scanCompetitionResult(row pgx.Row) (*model.CompetitionResult, error) {
	var res model.CompetitionResult
	if err := row.Scan(&res.ID, &res.ExamID, &res.SourceKey, &res.PublishedAt, &res.UpdatedAt, &res.RowCount); err != nil {
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

// GetCompetitionMatchData returns the exam's exam_number and its registrations
// by participant_number (revoked ones included, flagged); ErrNotFound for no exam.
func (r *Repository) GetCompetitionMatchData(ctx context.Context, examID uuid.UUID) (int, map[int]CompetitionRegistration, error) {
	var examNo *int
	if err := r.pool.QueryRow(ctx, `SELECT exam_number FROM exam WHERE id = $1`, examID).Scan(&examNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrNotFound
		}
		return 0, nil, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT participant_number, id, status = 'revoked'
		FROM exam_registration WHERE exam_id = $1 AND participant_number IS NOT NULL`, examID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	out := map[int]CompetitionRegistration{}
	for rows.Next() {
		var n int
		var reg CompetitionRegistration
		if err := rows.Scan(&n, &reg.ID, &reg.Revoked); err != nil {
			return 0, nil, err
		}
		out[n] = reg
	}
	if examNo == nil {
		return 0, out, rows.Err()
	}
	return *examNo, out, rows.Err()
}

func (r *Repository) GetCompetitionResult(ctx context.Context, examID uuid.UUID) (*model.CompetitionResult, error) {
	return scanCompetitionResult(r.pool.QueryRow(ctx,
		`SELECT `+competitionResultCols+` FROM competition_result r WHERE r.exam_id = $1`, examID))
}

// ApplyCompetitionResult upserts the exam's result set, replaces all its
// rows, and audits, in one transaction. published_at is left untouched.
func (r *Repository) ApplyCompetitionResult(ctx context.Context, examID uuid.UUID, sourceKey, actorID string, inputs []model.CompetitionResultInput) (*model.CompetitionResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var resultID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO competition_result (exam_id, source_key, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (exam_id) DO UPDATE
		SET source_key = EXCLUDED.source_key, updated_by = EXCLUDED.updated_by, updated_at = now()
		RETURNING id`, examID, sourceKey, actorID).Scan(&resultID); err != nil {
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
		"exam_id": examID.String(), "rows": len(inputs), "replaced_rows": del.RowsAffected(), "source_key": sourceKey,
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
// unpublishes the exam's result set and audits; ErrNotFound for none.
func (r *Repository) SetCompetitionResultPublished(ctx context.Context, examID uuid.UUID, publish bool, actorID string) (*model.CompetitionResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res, err := scanCompetitionResult(tx.QueryRow(ctx,
		`UPDATE competition_result r SET
			published_at = CASE WHEN $2 THEN COALESCE(r.published_at, now()) END,
			updated_by = $3, updated_at = now()
		WHERE r.exam_id = $1
		RETURNING `+competitionResultCols, examID, publish, actorID))
	if err != nil {
		return nil, err
	}
	action := "unpublish"
	if publish {
		action = "publish"
	}
	if err := r.InsertAuditLogMeta(ctx, tx, &actorID, "competition_result", res.ID.String(), action, map[string]any{
		"exam_id": examID.String(), "rows": res.RowCount,
	}); err != nil {
		return nil, err
	}
	return res, tx.Commit(ctx)
}

func (r *Repository) DeleteCompetitionResult(ctx context.Context, examID uuid.UUID, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	res, err := scanCompetitionResult(tx.QueryRow(ctx,
		`SELECT `+competitionResultCols+` FROM competition_result r WHERE r.exam_id = $1 FOR UPDATE`, examID))
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
		"exam_id": examID.String(), "rows": res.RowCount, "source_key": source,
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
// registrations, read via each registration's exam, in one query.
func (r *Repository) ListPublishedCompetitionResults(ctx context.Context, registrationIDs []uuid.UUID) ([]model.StudentCompetitionResult, error) {
	if len(registrationIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT x.registration_id, x.score::float8, x.rank, x.award, x.certificate_url, r.published_at
		FROM exam_registration reg
		JOIN competition_result r ON r.exam_id = reg.exam_id AND r.published_at IS NOT NULL
		JOIN competition_result_row x ON x.result_id = r.id AND x.registration_id = reg.id
		WHERE reg.id = ANY($1)`, registrationIDs)
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
