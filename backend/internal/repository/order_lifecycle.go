package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ExpirePaymentOrderTx moves one unpaid order to payment_expired and, in the
// same transaction, gives back the physical stock checkout took and releases
// the order's digital claims. The UPDATE guards status and deadline itself, so
// it takes the row lock and re-evaluates against whatever a concurrent webhook,
// refund or sweeper committed: only one caller ever sees true, which is what
// keeps stock from being restored twice and a paid order from being expired.
func (r *Repository) ExpirePaymentOrderTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'payment_expired', updated_at = now()
		 WHERE id = $1 AND status = 'payment_pending'
		   AND payment_expires_at IS NOT NULL AND payment_expires_at <= now()`,
		orderID,
	)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(ctx,
		`UPDATE product p SET stock = p.stock + i.qty, updated_at = now()
		 FROM (SELECT product_id, sum(qty) AS qty FROM order_item
		       WHERE order_id = $1 AND product_type IN ('book', 'merchandise', 'medal')
		       GROUP BY product_id) i
		 WHERE p.id = i.product_id`,
		orderID,
	); err != nil {
		return false, err
	}
	if err := r.ReleaseDigitalTargetsTx(ctx, tx, orderID, "expired"); err != nil {
		return false, err
	}
	return true, nil
}
