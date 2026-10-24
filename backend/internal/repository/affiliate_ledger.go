package repository

import (
	"context"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetCommissionsByOrderTx returns the order's affiliate commissions as read
// inside the caller's transaction, so the refund settles exactly the rows it
// mutates. An order without commissions returns an empty slice.
func (r *Repository) GetCommissionsByOrderTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) ([]model.AffiliateCommission, error) {
	rows, err := r.affiliateQueryer(tx).Query(ctx,
		`SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, percentage,
		        level_snapshot, amount, status, wait_until, paid_at
		 FROM affiliate_commissions
		 WHERE order_id = $1
		 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AffiliateCommission
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID,
			&c.ProductType, &c.BaseAmount, &c.Percentage, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateCommissionStatusTx moves one commission to a new status inside tx.
func (r *Repository) UpdateCommissionStatusTx(ctx context.Context, tx pgx.Tx, commissionID uuid.UUID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliate_commissions SET status = $2, updated_at = now() WHERE id = $1`,
		commissionID, status)
	return err
}

// InsertRefundDeductionTx appends one signed ledger row inside tx and leaves
// the entry alone when this commission already has one of this entry type.
// ON CONFLICT keeps a double refund attempt from crashing the refund; the
// refundableStatuses guard refuses the repeat before this code runs.
func (r *Repository) InsertRefundDeductionTx(ctx context.Context, tx pgx.Tx, e *model.AffiliateLedgerEntry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id, note, actor_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (commission_id, entry_type) WHERE commission_id IS NOT NULL DO NOTHING`,
		e.AffiliatorID, e.EntryType, e.Amount, e.CommissionID, e.Note, e.ActorID)
	return err
}
