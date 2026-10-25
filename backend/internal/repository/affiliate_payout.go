package repository

import (
	"context"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AffiliatePayoutQueueRow is one payout queue row joined with the owning
// affiliator's code and account name, for the admin list and CSV export.
type AffiliatePayoutQueueRow struct {
	model.AffiliatePayout
	AffiliatorName string `json:"affiliator_name"`
	AffiliatorCode string `json:"affiliator_code"`
}

// GetAffiliatePayoutForUpdateTx returns one payout locked FOR UPDATE inside
// tx, or (nil, nil) when the id is unknown.
func (r *Repository) GetAffiliatePayoutForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.AffiliatePayout, error) {
	var p model.AffiliatePayout
	err := tx.QueryRow(ctx,
		`SELECT id, affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder,
		        proof_url, reject_reason, processed_by, processed_at, requested_at, created_at, updated_at
		 FROM affiliate_payouts
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(&p.ID, &p.AffiliatorID, &p.Amount, &p.Status, &p.BankName, &p.BankAccountNumber, &p.BankAccountHolder,
		&p.ProofURL, &p.RejectReason, &p.ProcessedBy, &p.ProcessedAt, &p.RequestedAt, &p.CreatedAt, &p.UpdatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateAffiliatePayoutDecisionTx moves one payout to a decided status with
// the proof or rejection reason and the acting admin, and returns the updated
// row.
func (r *Repository) UpdateAffiliatePayoutDecisionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string, proofURL, rejectReason *string, processedBy uuid.UUID) (*model.AffiliatePayout, error) {
	var p model.AffiliatePayout
	err := tx.QueryRow(ctx,
		`UPDATE affiliate_payouts
		 SET status = $2, proof_url = $3, reject_reason = $4, processed_by = $5,
		     processed_at = now(), updated_at = now()
		 WHERE id = $1
		 RETURNING id, affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder,
		           proof_url, reject_reason, processed_by, processed_at, requested_at, created_at, updated_at`,
		id, status, proofURL, rejectReason, processedBy,
	).Scan(&p.ID, &p.AffiliatorID, &p.Amount, &p.Status, &p.BankName, &p.BankAccountNumber, &p.BankAccountHolder,
		&p.ProofURL, &p.RejectReason, &p.ProcessedBy, &p.ProcessedAt, &p.RequestedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// HasAffiliateLedgerEntryTx reports whether this payout already carries a
// ledger row of the given entry type. uq_affiliate_ledger only constrains
// commission-linked rows, so payout rows need this check for idempotence.
func (r *Repository) HasAffiliateLedgerEntryTx(ctx context.Context, tx pgx.Tx, payoutID uuid.UUID, entryType string) (bool, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_ledger_entries WHERE payout_id = $1 AND entry_type = $2`,
		payoutID, entryType,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// InsertAffiliateLedgerEntryTx appends one signed ledger row inside tx.
// affiliate_ledger_entries is append-only (trigger 0070): INSERT only.
func (r *Repository) InsertAffiliateLedgerEntryTx(ctx context.Context, tx pgx.Tx, e *model.AffiliateLedgerEntry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id, payout_id, adjustment_id, note, actor_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.AffiliatorID, e.EntryType, e.Amount, e.CommissionID, e.PayoutID, e.AdjustmentID, e.Note, e.ActorID)
	return err
}

// InsertAffiliateAdjustmentTx persists one adjustment inside tx and fills
// ID/CreatedAt.
func (r *Repository) InsertAffiliateAdjustmentTx(ctx context.Context, tx pgx.Tx, a *model.AffiliateAdjustment) error {
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_adjustments
		     (affiliator_id, category, amount, reason, note, approval_status, approver_id, approved_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at`,
		a.AffiliatorID, a.Category, a.Amount, a.Reason, a.Note, a.ApprovalStatus, a.ApproverID, a.ApprovedAt, a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt)
}

// ListAffiliateAdjustments returns one affiliator's adjustments newest first
// with the total row count for pagination.
func (r *Repository) ListAffiliateAdjustments(ctx context.Context, affiliatorID uuid.UUID, limit, offset int) ([]model.AffiliateAdjustment, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_adjustments WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, affiliator_id, category, amount, reason, note, approval_status, approver_id, approved_at, created_by, created_at
		 FROM affiliate_adjustments
		 WHERE affiliator_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.AffiliateAdjustment
	for rows.Next() {
		var a model.AffiliateAdjustment
		if err := rows.Scan(&a.ID, &a.AffiliatorID, &a.Category, &a.Amount, &a.Reason, &a.Note,
			&a.ApprovalStatus, &a.ApproverID, &a.ApprovedAt, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ListAffiliatePayoutQueue returns payouts in the given statuses joined with
// the owning affiliator's name and code, oldest request first, plus the total
// row count in those statuses.
func (r *Repository) ListAffiliatePayoutQueue(ctx context.Context, statuses []string, limit, offset int) ([]AffiliatePayoutQueueRow, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_payouts WHERE status = ANY($1)`, statuses,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.affiliator_id, p.amount, p.status, p.bank_name, p.bank_account_number, p.bank_account_holder,
		        p.proof_url, p.reject_reason, p.processed_by, p.processed_at, p.requested_at, p.created_at, p.updated_at,
		        u.name, COALESCE(a.code, '')
		 FROM affiliate_payouts p
		 JOIN affiliators a ON a.id = p.affiliator_id
		 JOIN users u ON u.id = a.user_id
		 WHERE p.status = ANY($1)
		 ORDER BY p.requested_at, p.id
		 LIMIT $2 OFFSET $3`,
		statuses, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AffiliatePayoutQueueRow
	for rows.Next() {
		var row AffiliatePayoutQueueRow
		if err := rows.Scan(&row.ID, &row.AffiliatorID, &row.Amount, &row.Status, &row.BankName,
			&row.BankAccountNumber, &row.BankAccountHolder, &row.ProofURL, &row.RejectReason,
			&row.ProcessedBy, &row.ProcessedAt, &row.RequestedAt, &row.CreatedAt, &row.UpdatedAt,
			&row.AffiliatorName, &row.AffiliatorCode); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
