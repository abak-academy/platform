package repository

import (
	"context"
	"strconv"

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
		`SELECT id, user_id, COALESCE(code, ''), status, rejection_reason, level,
		        warn_quarters, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level,
		&a.WarnQuarters, &a.TermsVersion, &a.TermsAcceptedAt,
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
// request first with the total row count for pagination. Status and the
// request-date range are filterable.
func (r *Repository) ListAffiliatePayoutsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery, limit, offset int) ([]model.AffiliatePayout, int, error) {
	p := newAffPredicate(2)
	p.eq("status", q.Status)
	p.dates("requested_at", q.From, q.To)
	args := append([]interface{}{affiliatorID}, p.args...)

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_payouts WHERE affiliator_id = $1`+p.sql, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliatePayoutColumns+`
		 FROM affiliate_payouts
		 WHERE affiliator_id = $1`+p.sql+`
		 ORDER BY requested_at DESC, id DESC
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []model.AffiliatePayout{}
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
// newest first, each joined with whether its ledger entry exists. Type filters
// the category; the text search covers the reason and the note.
func (r *Repository) ListAffiliateAdjustmentsWithApplied(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery, limit, offset int) ([]AffiliateAdjustmentWithApplied, int, error) {
	p := newAffPredicate(2)
	p.eq("a.category", q.Type)
	p.dates("a.created_at", q.From, q.To)
	p.search(q.Q, "a.reason", "a.note")
	args := append([]interface{}{affiliatorID}, p.args...)

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_adjustments a WHERE a.affiliator_id = $1`+p.sql, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.affiliator_id, a.category, a.amount, a.reason, a.note, a.approval_status,
		        a.approver_id, a.approved_at, a.created_by, a.created_at,
		        EXISTS (SELECT 1 FROM affiliate_ledger_entries l WHERE l.adjustment_id = a.id)
		 FROM affiliate_adjustments a
		 WHERE a.affiliator_id = $1`+p.sql+`
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateAdjustmentWithApplied{}
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
