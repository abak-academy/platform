package repository

import (
	"context"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AffiliateDueCommission is a pending commission whose wait_until has passed,
// with the order item name the release ledger note references.
type AffiliateDueCommission struct {
	Commission model.AffiliateCommission
	ItemName   string
}

// AffiliatePendingPhysical is a physical-product commission still waiting for
// the order's completed_at so the job can derive wait_until.
type AffiliatePendingPhysical struct {
	CommissionID  uuid.UUID
	LevelSnapshot string
	CompletedAt   time.Time
}

// GetDuePendingCommissions returns pending commissions with wait_until at or
// before now, oldest wait first, with the order item name for the ledger note.
func (r *Repository) GetDuePendingCommissions(ctx context.Context, now time.Time, limit int) ([]AffiliateDueCommission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliateCommissionSelect("c")+`, oi.name
		 FROM affiliate_transactions c
		 JOIN order_item oi ON oi.id = c.order_item_id
		 WHERE c.type = 'commission' AND c.status = 'pending' AND c.release_at IS NOT NULL AND c.release_at <= $1
		 ORDER BY c.release_at
		 LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliateDueCommission
	for rows.Next() {
		var c model.AffiliateCommission
		var name string
		if err := rows.Scan(append(affiliateCommissionDest(&c), &name)...); err != nil {
			return nil, err
		}
		out = append(out, AffiliateDueCommission{Commission: c, ItemName: name})
	}
	return out, rows.Err()
}

// ReleaseAffiliateCommissionTx performs the pending→available transition for
// one commission, locking the row first so a refund settling the same
// commission wins or loses cleanly. The commission takes effect on the balance
// now. It returns false when the row is gone or no longer pending.
func (r *Repository) ReleaseAffiliateCommissionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, note string) (bool, error) {
	var status string
	err := tx.QueryRow(ctx,
		`SELECT status FROM affiliate_transactions WHERE id = $1 AND type = 'commission' FOR UPDATE`, id,
	).Scan(&status)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "pending" {
		return false, nil
	}
	_, err = tx.Exec(ctx,
		`UPDATE affiliate_transactions
		 SET status = 'available', effective_at = now(), note = $2, updated_at = now()
		 WHERE id = $1`, id, note,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetPendingPhysicalCommissions returns pending physical-product commissions
// with no wait_until whose order has been completed by an admin.
func (r *Repository) GetPendingPhysicalCommissions(ctx context.Context, limit int) ([]AffiliatePendingPhysical, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.detail->>'level', o.completed_at
		 FROM affiliate_transactions c
		 JOIN orders o ON o.id = c.order_id
		 WHERE c.type = 'commission' AND c.status = 'pending' AND c.release_at IS NULL
		   AND c.product_type IN ('book', 'merchandise', 'medal')
		   AND o.completed_at IS NOT NULL
		 ORDER BY c.created_at
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliatePendingPhysical
	for rows.Next() {
		var p AffiliatePendingPhysical
		if err := rows.Scan(&p.CommissionID, &p.LevelSnapshot, &p.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetAffiliateCommissionWaitUntil stamps the derived wait on a pending
// physical commission; the wait_until IS NULL guard keeps repeat runs no-ops.
func (r *Repository) SetAffiliateCommissionWaitUntil(ctx context.Context, id uuid.UUID, waitUntil time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE affiliate_transactions SET release_at = $2, updated_at = now()
		 WHERE id = $1 AND type = 'commission' AND status = 'pending' AND release_at IS NULL`,
		id, waitUntil,
	)
	return err
}

// ListAffiliatorsForLevelEval returns every affiliator the quarterly level
// evaluation may consider: active or suspended.
func (r *Repository) ListAffiliatorsForLevelEval(ctx context.Context) ([]model.Affiliator, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+affiliatorColumns+` `+affiliatorFrom+`
		 WHERE a.program_status IN ('active', 'suspended')
		 ORDER BY u.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Affiliator
	for rows.Next() {
		a, err := scanAffiliator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetAffiliateQuarterNetRevenue sums the base amounts of the affiliator's
// available and paid commissions that became final inside the window, i.e.
// whose release_at (order_paid_at when there is none) falls in it. A commission
// refunded after release is cancelled, so it drops out (BR-LVL-07); counting by
// the final date keeps a physical order that settles after the evaluation day
// in the next quarter instead of in none.
func (r *Repository) GetAffiliateQuarterNetRevenue(ctx context.Context, affiliatorID uuid.UUID, from, to time.Time) (float64, error) {
	var net float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(base_amount), 0) FROM affiliate_transactions
		 WHERE type = 'commission' AND affiliator_id = $1 AND status = 'available'
		   AND COALESCE(release_at, order_paid_at) >= $2 AND COALESCE(release_at, order_paid_at) < $3`,
		affiliatorID, from, to,
	).Scan(&net)
	return net, err
}

// UpdateAffiliateLevelTx moves the affiliator's level and missed-quarter
// counter inside the caller's transaction.
func (r *Repository) UpdateAffiliateLevelTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, level string, missedQuarters int) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET level = $2, missed_quarters = $3 WHERE user_id = $1`,
		id, level, missedQuarters,
	)
	return err
}

// SettleAffiliateLevelEvaluationTx records one quarter's evaluation outcome
// (level, missed-quarter counter) and marks the quarter as evaluated, which is
// what keeps a rerun from counting the same quarter twice.
func (r *Repository) SettleAffiliateLevelEvaluationTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, level string, missedQuarters int, quarter string) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET level = $2, missed_quarters = $3, last_evaluated_quarter = $4 WHERE user_id = $1`,
		id, level, missedQuarters, quarter,
	)
	return err
}

// ListPaidOrdersMissingAffiliateCommissions returns paid orders (oldest first)
// of students bound to an active affiliator since the payment (a checkout-code
// binding is stamped a moment after the order's paid_at, hence the tolerance), that still
// have a commissionable item with no commission row. paid_before keeps it off
// orders whose OrderPaid event the worker may still be handling.
func (r *Repository) ListPaidOrdersMissingAffiliateCommissions(ctx context.Context, paidSince, paidBefore time.Time, limit int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id
		 FROM orders o
		 JOIN affiliate_bindings b ON b.student_id = o.student_id AND b.status = 'active'
		      AND b.started_at <= o.paid_at + interval '5 minutes'
		 JOIN affiliators a ON a.user_id = b.affiliator_id AND a.program_status = 'active'
		 WHERE o.status IN ('paid', 'processing', 'shipped', 'completed')
		   AND o.paid_at >= $1 AND o.paid_at < $2
		   AND EXISTS (
		       SELECT 1 FROM order_item oi
		       WHERE oi.order_id = o.id
		         AND oi.product_type IN ('exam', 'course', 'book', 'merchandise', 'medal')
		         AND NOT EXISTS (SELECT 1 FROM affiliate_transactions c WHERE c.type = 'commission' AND c.order_item_id = oi.id))
		 ORDER BY o.paid_at, o.id
		 LIMIT $3`,
		paidSince, paidBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
