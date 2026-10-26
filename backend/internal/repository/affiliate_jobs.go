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

// AffiliateSmallRepeatRow is one affiliator's count of small repeat purchases
// in the flag window and how many came from bindings that were extended.
type AffiliateSmallRepeatRow struct {
	AffiliatorID uuid.UUID
	Total        int
	Extended     int
}

// AffiliateRefundRatioRow is one affiliator's commission total in the flag
// window and how many of those are cancelled by refund.
type AffiliateRefundRatioRow struct {
	AffiliatorID uuid.UUID
	Total        int
	Cancelled    int
}

// GetDuePendingCommissions returns pending commissions with wait_until at or
// before now, oldest wait first, with the order item name for the ledger note.
func (r *Repository) GetDuePendingCommissions(ctx context.Context, now time.Time, limit int) ([]AffiliateDueCommission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.affiliator_id, c.student_id, c.order_id, c.order_item_id, c.product_type,
		        c.base_amount, c.percentage, c.level_snapshot, c.amount, c.status, c.wait_until, c.paid_at,
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
			&c.BaseAmount, &c.Percentage, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt,
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

// ExpireAffiliateSanctions reactivates affiliators whose timed suspension has
// passed and clears expired freezes; permanent suspensions stay untouched.
// Repeat runs change nothing.
func (r *Repository) ExpireAffiliateSanctions(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE affiliators SET status = 'active', suspended_until = NULL, updated_at = now()
		 WHERE status = 'suspended' AND suspended_until IS NOT NULL AND suspended_until <= $1`,
		now,
	)
	if err != nil {
		return 0, err
	}
	changed := tag.RowsAffected()
	tag, err = r.pool.Exec(ctx,
		`UPDATE affiliators SET frozen_until = NULL, updated_at = now()
		 WHERE frozen_until IS NOT NULL AND frozen_until <= $1`,
		now,
	)
	if err != nil {
		return changed, err
	}
	return changed + tag.RowsAffected(), nil
}

// InsertAffiliateFlagOnce persists one rule flag row unless the affiliator
// already has an open flag for the same rule; returns whether a row was added.
func (r *Repository) InsertAffiliateFlagOnce(ctx context.Context, affiliatorID uuid.UUID, rule string, payload []byte) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO affiliate_flags (affiliator_id, rule, payload, status)
		 SELECT $1, $2, $3, 'open'
		 WHERE NOT EXISTS (
		     SELECT 1 FROM affiliate_flags
		     WHERE affiliator_id = $1 AND rule = $2 AND status = 'open')`,
		affiliatorID, rule, payload,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CountSmallRepeatCommissions groups the window's small commissions per
// affiliator and counts how many came from students whose active binding was
// extended (last_extended_at set), for the BR-PLG-08 boundary rule.
func (r *Repository) CountSmallRepeatCommissions(ctx context.Context, windowStart time.Time, minAmount, maxAmount float64, minCount int) ([]AffiliateSmallRepeatRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.affiliator_id, count(*) AS total,
		        count(*) FILTER (WHERE b.last_extended_at IS NOT NULL) AS extended
		 FROM affiliate_commissions c
		 LEFT JOIN affiliate_bindings b
		        ON b.student_id = c.student_id AND b.affiliator_id = c.affiliator_id AND b.status = 'active'
		 WHERE c.created_at >= $1 AND c.base_amount >= $2 AND c.base_amount <= $3
		 GROUP BY c.affiliator_id
		 HAVING count(*) >= $4`,
		windowStart, minAmount, maxAmount, minCount,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliateSmallRepeatRow
	for rows.Next() {
		var row AffiliateSmallRepeatRow
		if err := rows.Scan(&row.AffiliatorID, &row.Total, &row.Extended); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CountRefundCancelledCommissions groups the window's commissions per
// affiliator with their cancelled-by-refund counts, for BR-PLG-08.
func (r *Repository) CountRefundCancelledCommissions(ctx context.Context, windowStart time.Time, minTotal int) ([]AffiliateRefundRatioRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.affiliator_id, count(*) AS total,
		        count(*) FILTER (WHERE c.status = 'cancelled') AS cancelled
		 FROM affiliate_commissions c
		 WHERE c.created_at >= $1
		 GROUP BY c.affiliator_id
		 HAVING count(*) >= $2`,
		windowStart, minTotal,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliateRefundRatioRow
	for rows.Next() {
		var row AffiliateRefundRatioRow
		if err := rows.Scan(&row.AffiliatorID, &row.Total, &row.Cancelled); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListAffiliatorsForLevelEval returns every affiliator the quarterly level
// evaluation may consider: active or suspended.
func (r *Repository) ListAffiliatorsForLevelEval(ctx context.Context) ([]model.Affiliator, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, COALESCE(code, ''), status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE status IN ('active', 'suspended')
		 ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Affiliator
	for rows.Next() {
		var a model.Affiliator
		if err := rows.Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level, &a.LevelLockedUntil,
			&a.WarnQuarters, &a.SuspendedUntil, &a.FrozenUntil, &a.TermsVersion, &a.TermsAcceptedAt,
			&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// HasAffiliateLevelHistory reports whether the affiliator already has an auto
// history row for the quarter: the per-quarter idempotency guard.
func (r *Repository) HasAffiliateLevelHistory(ctx context.Context, affiliatorID uuid.UUID, quarter, source string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM affiliate_level_history
		     WHERE affiliator_id = $1 AND quarter = $2 AND source = $3)`,
		affiliatorID, quarter, source,
	).Scan(&exists)
	return exists, err
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
		 WHERE affiliator_id = $1 AND status IN ('available', 'paid')
		   AND COALESCE(wait_until, paid_at) >= $2 AND COALESCE(wait_until, paid_at) < $3`,
		affiliatorID, from, to,
	).Scan(&net)
	return net, err
}

// UpdateAffiliateLevelTx moves the affiliator's level and warning counter
// inside the caller's transaction.
func (r *Repository) UpdateAffiliateLevelTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, level string, warnQuarters int) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET level = $2, warn_quarters = $3, updated_at = now() WHERE id = $1`,
		id, level, warnQuarters,
	)
	return err
}

// SetAffiliateWarnQuarters rewrites the warning counter outside any
// transaction; the warn_quarters <> $2 guard keeps repeat writes no-ops.
func (r *Repository) SetAffiliateWarnQuarters(ctx context.Context, id uuid.UUID, warnQuarters int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE affiliators SET warn_quarters = $2, updated_at = now() WHERE id = $1 AND warn_quarters <> $2`,
		id, warnQuarters,
	)
	return err
}

// InsertAffiliateLevelHistoryTx appends one level-history row inside the
// caller's transaction and fills ID/CreatedAt.
func (r *Repository) InsertAffiliateLevelHistoryTx(ctx context.Context, tx pgx.Tx, h *model.AffiliateLevelHistory) error {
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_level_history (affiliator_id, quarter, from_level, to_level, net_revenue, source, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		h.AffiliatorID, h.Quarter, h.FromLevel, h.ToLevel, h.NetRevenue, h.Source, h.Reason,
	).Scan(&h.ID, &h.CreatedAt)
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
		 JOIN affiliators a ON a.id = b.affiliator_id AND a.status = 'active'
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
