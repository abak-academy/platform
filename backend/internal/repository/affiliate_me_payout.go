package repository

import (
	"context"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BR-PRV-01, same rule as affiliate_me.go: every query here is scoped to one
// affiliator_id the caller resolved from the JWT user_id.

const affiliatePayoutColumns = `id, affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder,
		proof_url, reject_reason, processed_by, processed_at, requested_at, created_at, updated_at`

func scanAffiliatePayout(row pgx.Row) (*model.AffiliatePayout, error) {
	var p model.AffiliatePayout
	if err := row.Scan(&p.ID, &p.AffiliatorID, &p.Amount, &p.Status, &p.BankName, &p.BankAccountNumber, &p.BankAccountHolder,
		&p.ProofURL, &p.RejectReason, &p.ProcessedBy, &p.ProcessedAt, &p.RequestedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetAffiliatorForUpdateTx returns one affiliator row locked FOR UPDATE
// inside tx, or (nil, nil) when the id is unknown — the lock serializes
// concurrent payout requests for the same account (BR-PCR-05).
func (r *Repository) GetAffiliatorForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Affiliator, error) {
	var a model.Affiliator
	err := tx.QueryRow(ctx,
		`SELECT id, user_id, code, status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level, &a.LevelLockedUntil,
		&a.WarnQuarters, &a.SuspendedUntil, &a.FrozenUntil, &a.TermsVersion, &a.TermsAcceptedAt,
		&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SumAffiliateLedgerTx is the saldo of the affiliator's ledger as read inside
// tx, so the payout request cannot overdraw against a concurrently released
// commission.
func (r *Repository) SumAffiliateLedgerTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (float64, error) {
	var saldo float64
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_ledger_entries WHERE affiliator_id = $1`,
		affiliatorID,
	).Scan(&saldo)
	return saldo, err
}

// InsertAffiliatePayoutTx persists one payout row inside tx and fills
// ID/RequestedAt/CreatedAt/UpdatedAt.
func (r *Repository) InsertAffiliatePayoutTx(ctx context.Context, tx pgx.Tx, p *model.AffiliatePayout) error {
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_payouts (affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+affiliatePayoutColumns,
		p.AffiliatorID, p.Amount, p.Status, p.BankName, p.BankAccountNumber, p.BankAccountHolder,
	).Scan(&p.ID, &p.AffiliatorID, &p.Amount, &p.Status, &p.BankName, &p.BankAccountNumber, &p.BankAccountHolder,
		&p.ProofURL, &p.RejectReason, &p.ProcessedBy, &p.ProcessedAt, &p.RequestedAt, &p.CreatedAt, &p.UpdatedAt)
}

// HasAffiliatePayoutInFlightTx reports whether the affiliator already has a
// requested or processing payout (BR-PCR-05: one undecided request at a time).
func (r *Repository) HasAffiliatePayoutInFlightTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (bool, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_payouts WHERE affiliator_id = $1 AND status IN ('requested', 'processing')`,
		affiliatorID,
	).Scan(&n)
	return n > 0, err
}

// ListAffiliatePayoutsForAffiliator returns one affiliator's payouts newest
// request first with the total row count for pagination.
func (r *Repository) ListAffiliatePayoutsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, limit, offset int) ([]model.AffiliatePayout, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_payouts WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliatePayoutColumns+`
		 FROM affiliate_payouts
		 WHERE affiliator_id = $1
		 ORDER BY requested_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.AffiliatePayout
	for rows.Next() {
		p, err := scanAffiliatePayout(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// AffiliateAdjustmentWithApplied is one affiliator-facing adjustment row with
// the applied flag derived from the adjustment's ledger entry (a
// threshold-pending row has none).
type AffiliateAdjustmentWithApplied struct {
	model.AffiliateAdjustment
	Applied bool `json:"applied"`
}

// ListAffiliateAdjustmentsWithApplied returns one affiliator's adjustments
// newest first, each joined with whether its ledger entry exists.
func (r *Repository) ListAffiliateAdjustmentsWithApplied(ctx context.Context, affiliatorID uuid.UUID, limit, offset int) ([]AffiliateAdjustmentWithApplied, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_adjustments WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.affiliator_id, a.category, a.amount, a.reason, a.note, a.approval_status,
		        a.approver_id, a.approved_at, a.created_by, a.created_at,
		        EXISTS (SELECT 1 FROM affiliate_ledger_entries l WHERE l.adjustment_id = a.id)
		 FROM affiliate_adjustments a
		 WHERE a.affiliator_id = $1
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AffiliateAdjustmentWithApplied
	for rows.Next() {
		var row AffiliateAdjustmentWithApplied
		if err := rows.Scan(&row.ID, &row.AffiliatorID, &row.Category, &row.Amount, &row.Reason, &row.Note,
			&row.ApprovalStatus, &row.ApproverID, &row.ApprovedAt, &row.CreatedBy, &row.CreatedAt,
			&row.Applied); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// ListAffiliateViolationsForAffiliator returns one affiliator's violations
// newest first with the total row count for pagination.
func (r *Repository) ListAffiliateViolationsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, limit, offset int) ([]model.AffiliateViolation, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_violations WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, affiliator_id, category, description, sanction, suspension_until, status, decided_by, decided_at,
		        appeal_text, appeal_status, appeal_decided_by, appeal_decided_at, created_at
		 FROM affiliate_violations
		 WHERE affiliator_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.AffiliateViolation
	for rows.Next() {
		var v model.AffiliateViolation
		if err := rows.Scan(&v.ID, &v.AffiliatorID, &v.Category, &v.Description, &v.Sanction, &v.SuspensionUntil,
			&v.Status, &v.DecidedBy, &v.DecidedAt, &v.AppealText, &v.AppealStatus, &v.AppealDecidedBy,
			&v.AppealDecidedAt, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// GetAffiliateViolationForUpdateTx returns one violation locked FOR UPDATE
// inside tx, or (nil, nil) when the id is unknown.
func (r *Repository) GetAffiliateViolationForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.AffiliateViolation, error) {
	var v model.AffiliateViolation
	err := tx.QueryRow(ctx,
		`SELECT id, affiliator_id, category, description, sanction, suspension_until, status, decided_by, decided_at,
		        appeal_text, appeal_status, appeal_decided_by, appeal_decided_at, created_at
		 FROM affiliate_violations
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(&v.ID, &v.AffiliatorID, &v.Category, &v.Description, &v.Sanction, &v.SuspensionUntil,
		&v.Status, &v.DecidedBy, &v.DecidedAt, &v.AppealText, &v.AppealStatus, &v.AppealDecidedBy,
		&v.AppealDecidedAt, &v.CreatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdateAffiliateViolationAppealTx records an affiliator's appeal on one
// violation: only the appeal text and the pending status move; deciding the
// appeal stays an admin action.
func (r *Repository) UpdateAffiliateViolationAppealTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, text string) (*model.AffiliateViolation, error) {
	var v model.AffiliateViolation
	err := tx.QueryRow(ctx,
		`UPDATE affiliate_violations
		 SET appeal_text = $2, appeal_status = 'pending'
		 WHERE id = $1
		 RETURNING id, affiliator_id, category, description, sanction, suspension_until, status, decided_by, decided_at,
		           appeal_text, appeal_status, appeal_decided_by, appeal_decided_at, created_at`,
		id, text,
	).Scan(&v.ID, &v.AffiliatorID, &v.Category, &v.Description, &v.Sanction, &v.SuspensionUntil,
		&v.Status, &v.DecidedBy, &v.DecidedAt, &v.AppealText, &v.AppealStatus, &v.AppealDecidedBy,
		&v.AppealDecidedAt, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
