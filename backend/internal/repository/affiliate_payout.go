package repository

import (
	"context"
	"strconv"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

// AffiliatePayoutQueueRow is one payout queue row joined with the owning
// affiliator's code and account name, for the admin list and CSV export.
type AffiliatePayoutQueueRow struct {
	model.AffiliatePayout
	AffiliatorName string `json:"affiliator_name"`
	AffiliatorCode string `json:"affiliator_code"`
}

// ListAffiliateAdjustments returns one affiliator's adjustments newest first
// with the total row count for pagination.
func (r *Repository) ListAffiliateAdjustments(ctx context.Context, affiliatorID uuid.UUID, limit, offset int) ([]model.AffiliateAdjustment, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_transactions WHERE type = 'adjustment' AND affiliator_id = $1`, affiliatorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliateAdjustmentSelect("a")+`
		 FROM affiliate_transactions a
		 WHERE a.type = 'adjustment' AND a.affiliator_id = $1
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []model.AffiliateAdjustment{}
	for rows.Next() {
		var a model.AffiliateAdjustment
		if err := rows.Scan(affiliateAdjustmentDest(&a)...); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ListAffiliatePayoutQueue returns payouts in the given statuses joined with
// the owning affiliator's name and code, plus the total row count. Undecided
// payouts come first (oldest request first), then the decided ones newest first.
// q.Q matches the affiliator's name, code or the bank account holder; q.From/To
// bound the request date.
func (r *Repository) ListAffiliatePayoutQueue(ctx context.Context, statuses []string, q AffiliateListQuery, limit, offset int) ([]AffiliatePayoutQueueRow, int, error) {
	p := newAffPredicate(2)
	p.dates("p.created_at", q.From, q.To)
	p.search(q.Q, "u.name", "a.referral_code", "p.detail->'bank_account'->>'account_holder'")
	args := append([]interface{}{statuses}, p.args...)
	from := `FROM affiliate_transactions p
		 JOIN affiliators a ON a.user_id = p.affiliator_id
		 JOIN users u ON u.id = a.user_id
		 WHERE p.type = 'payout' AND p.status = ANY($1)` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliatePayoutSelect("p")+`, u.name, COALESCE(a.referral_code, '')
		 `+from+`
		 ORDER BY (p.status IN ('requested', 'processing')) DESC,
		          CASE WHEN p.status IN ('requested', 'processing') THEN p.created_at END ASC,
		          p.created_at DESC, p.id
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliatePayoutQueueRow{}
	for rows.Next() {
		var row AffiliatePayoutQueueRow
		if err := rows.Scan(append(affiliatePayoutDest(&row.AffiliatePayout), &row.AffiliatorName, &row.AffiliatorCode)...); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
