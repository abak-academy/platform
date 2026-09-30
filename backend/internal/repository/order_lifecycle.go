package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ExpirePaymentOrderTx moves one unpaid order to payment_expired and, in the
// same transaction, gives back the physical stock checkout took. The UPDATE
// guards status and deadline itself, so it takes the row lock and re-evaluates
// against whatever a concurrent webhook, refund or sweeper committed: only one
// caller ever sees true, which is what keeps stock from being restored twice
// and a paid order from being expired.
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
	return true, nil
}

// PaymentDeadlineElapsedTx judges the deadline on the database clock, the same
// one the sweeper's guarded UPDATE uses, so a retry and the sweeper cannot
// disagree about whether an order is still payable. An order with no deadline
// (gateway creation never succeeded) has not elapsed.
func (r *Repository) PaymentDeadlineElapsedTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (bool, error) {
	var elapsed bool
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(payment_expires_at <= now(), false) FROM orders WHERE id = $1`, orderID,
	).Scan(&elapsed)
	return elapsed, err
}

// SetPaymentRefTx records a freshly created gateway payment inside the
// caller's transaction, but only while the order is still an unexpired
// payment_pending one, and returns the order's deadline. The deadline is ABAK's
// and is never moved by a gateway response or a retry; an order that has none
// (placed before checkout set one) gets now()+window once. Not attached means
// the order moved on (expired, paid, cancelled) and the payment must not be
// attached or offered.
func (r *Repository) SetPaymentRefTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, ref string, window time.Duration) (time.Time, bool, error) {
	var deadline time.Time
	err := tx.QueryRow(ctx,
		`UPDATE orders SET gateway_ref = $1,
		     payment_expires_at = COALESCE(payment_expires_at, now() + make_interval(secs => $2)),
		     checked_out_at = now(), updated_at = now()
		 WHERE id = $3 AND status = 'payment_pending'
		   AND (payment_expires_at IS NULL OR payment_expires_at > now())
		 RETURNING payment_expires_at`,
		ref, window.Seconds(), orderID,
	).Scan(&deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return deadline, true, nil
}
