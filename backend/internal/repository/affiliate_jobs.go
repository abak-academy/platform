package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAffiliateLedgerEntryExists reports that this commission already carries a
// ledger entry of the given type (uq_affiliate_ledger_commission): the job
// already released it, not a failure.
var ErrAffiliateLedgerEntryExists = errors.New("affiliate ledger entry already exists for commission")

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
		`SELECT c.id, c.affiliator_id, c.student_id, c.order_id, c.order_item_id, c.product_type,
		        c.base_amount, c.commission_type, c.commission_value, c.level_snapshot, c.amount, c.status, c.wait_until, c.paid_at,
		        c.created_at, c.updated_at, oi.name
		 FROM affiliate_commissions c
		 JOIN order_item oi ON oi.id = c.order_item_id
		 WHERE c.status = 'pending' AND c.wait_until IS NOT NULL AND c.wait_until <= $1
		 ORDER BY c.wait_until
		 LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliateDueCommission
	for rows.Next() {
		var c model.AffiliateCommission
		var name string
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType,
			&c.BaseAmount, &c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt,
			&c.CreatedAt, &c.UpdatedAt, &name); err != nil {
			return nil, err
		}
		out = append(out, AffiliateDueCommission{Commission: c, ItemName: name})
	}
	return out, rows.Err()
}

// ReleaseAffiliateCommissionTx performs the pending→available transition and
// the commission ledger insert atomically for one commission, locking the row
// first so a refund settling the same commission wins or loses cleanly. It
// returns false when the row is gone, no longer pending, or the ledger entry
// already exists (released by an earlier run).
func (r *Repository) ReleaseAffiliateCommissionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, note string) (bool, error) {
	var status string
	err := tx.QueryRow(ctx,
		`SELECT status FROM affiliate_commissions WHERE id = $1 FOR UPDATE`, id,
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
	tag, err := tx.Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id, note)
		 SELECT affiliator_id, 'commission', amount, id, $2 FROM affiliate_commissions WHERE id = $1
		 ON CONFLICT (commission_id, entry_type) WHERE commission_id IS NOT NULL DO NOTHING`,
		id, note,
	)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, ErrAffiliateLedgerEntryExists
	}
	tag, err = tx.Exec(ctx,
		`UPDATE affiliate_commissions SET status = 'available', updated_at = now()
		 WHERE id = $1 AND status = 'pending'`, id,
	)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, fmt.Errorf("commission %s left pending while row-locked", id)
	}
	return true, nil
}

// GetPendingPhysicalCommissions returns pending physical-product commissions
// with no wait_until whose order has been completed by an admin.
func (r *Repository) GetPendingPhysicalCommissions(ctx context.Context, limit int) ([]AffiliatePendingPhysical, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.level_snapshot, o.completed_at
		 FROM affiliate_commissions c
		 JOIN orders o ON o.id = c.order_id
		 WHERE c.status = 'pending' AND c.wait_until IS NULL
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
		`UPDATE affiliate_commissions SET wait_until = $2, updated_at = now()
		 WHERE id = $1 AND status = 'pending' AND wait_until IS NULL`,
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
// whose wait_until (paid_at when there is none) falls in it. A commission
// refunded after release is cancelled, so it drops out (BR-LVL-07); counting by
// the final date keeps a physical order that settles after the evaluation day
// in the next quarter instead of in none.
func (r *Repository) GetAffiliateQuarterNetRevenue(ctx context.Context, affiliatorID uuid.UUID, from, to time.Time) (float64, error) {
	var net float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(base_amount), 0) FROM affiliate_commissions
		 WHERE affiliator_id = $1 AND status = 'available'
		   AND COALESCE(wait_until, paid_at) >= $2 AND COALESCE(wait_until, paid_at) < $3`,
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
		         AND NOT EXISTS (SELECT 1 FROM affiliate_commissions c WHERE c.order_item_id = oi.id))
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
