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

// GetAffiliatorForUpdateTx returns one affiliator row locked FOR UPDATE
// inside tx, or (nil, nil) when the id is unknown — the lock serializes
// concurrent payout requests for the same account (BR-PCR-05).
func (r *Repository) GetAffiliatorForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Affiliator, error) {
	a, err := scanAffiliator(tx.QueryRow(ctx,
		`SELECT `+affiliatorColumns+` `+affiliatorFrom+` WHERE a.user_id = $1 FOR UPDATE OF a`, id))
	if isNotFound(err) {
		return nil, nil
	}
	return a, err
}

// SumAffiliateLedgerTx is the saldo of the affiliator's transactions as read
// inside tx, so the payout request cannot overdraw against a concurrently
// released commission.
func (r *Repository) SumAffiliateLedgerTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (float64, error) {
	var saldo float64
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_transactions WHERE affiliator_id = $1 AND effective_at IS NOT NULL`,
		affiliatorID,
	).Scan(&saldo)
	return saldo, err
}

// HasAffiliatePayoutInFlightTx reports whether the affiliator already has a
// requested or processing payout (BR-PCR-05: one undecided request at a time).
func (r *Repository) HasAffiliatePayoutInFlightTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (bool, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_transactions
		 WHERE type = 'payout' AND affiliator_id = $1 AND status IN ('requested', 'processing')`,
		affiliatorID,
	).Scan(&n)
	return n > 0, err
}

// ListAffiliatePayoutsForAffiliator returns one affiliator's payouts newest
// request first with the total row count for pagination. Status and the
// request-date range are filterable.
func (r *Repository) ListAffiliatePayoutsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery, limit, offset int) ([]model.AffiliatePayout, int, error) {
	p := newAffPredicate(2)
	p.eq("p.status", q.Status)
	p.dates("p.created_at", q.From, q.To)
	args := append([]interface{}{affiliatorID}, p.args...)

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_transactions p WHERE p.type = 'payout' AND p.affiliator_id = $1`+p.sql, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliatePayoutSelect("p")+`
		 FROM affiliate_transactions p
		 WHERE p.type = 'payout' AND p.affiliator_id = $1`+p.sql+`
		 ORDER BY p.created_at DESC, p.id DESC
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []model.AffiliatePayout{}
	for rows.Next() {
		var item model.AffiliatePayout
		if err := rows.Scan(affiliatePayoutDest(&item)...); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// AffiliateAdjustmentWithApplied is one affiliator-facing adjustment row;
// every adjustment takes effect when posted, so Applied is always true.
type AffiliateAdjustmentWithApplied struct {
	model.AffiliateAdjustment
	Applied bool `json:"applied"`
}

// ListAffiliateAdjustmentsWithApplied returns one affiliator's adjustments
// newest first. Type filters the category; the text search covers the reason
// and the note.
func (r *Repository) ListAffiliateAdjustmentsWithApplied(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery, limit, offset int) ([]AffiliateAdjustmentWithApplied, int, error) {
	p := newAffPredicate(2)
	p.eq("a.detail->>'category'", q.Type)
	p.dates("a.created_at", q.From, q.To)
	p.search(q.Q, "a.note", "a.detail->>'note'")
	args := append([]interface{}{affiliatorID}, p.args...)

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_transactions a WHERE a.type = 'adjustment' AND a.affiliator_id = $1`+p.sql, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliateAdjustmentSelect("a")+`
		 FROM affiliate_transactions a
		 WHERE a.type = 'adjustment' AND a.affiliator_id = $1`+p.sql+`
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
		if err := rows.Scan(affiliateAdjustmentDest(&row.AffiliateAdjustment)...); err != nil {
			return nil, 0, err
		}
		row.Applied = true
		out = append(out, row)
	}
	return out, total, rows.Err()
}
