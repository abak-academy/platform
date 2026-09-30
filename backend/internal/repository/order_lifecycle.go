package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
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

// RecordLatePaymentTx durably flags a payment that the gateway reports for an
// order that can no longer be fulfilled (expired, cancelled, refunded), for an
// operator to reconcile or refund by hand: webhook_log rows of event_type
// 'payment_late', keyed by the order id in gateway_ref. One flag exists per
// order and payment reference, so redelivered webhooks and repeated reconciles
// do not pile up rows; the caller's row lock on the order serializes concurrent
// writers. It reports whether a new flag was written. audit_log cannot carry
// the flag because it requires an actor and a webhook has none.
func (r *Repository) RecordLatePaymentTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reference string, detail map[string]any) (bool, error) {
	detail["reference"] = reference
	payload, err := json.Marshal(detail)
	if err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO webhook_log (event_type, payload, gateway_ref)
		 SELECT 'payment_late', $1::jsonb, $2
		 WHERE NOT EXISTS (
		     SELECT 1 FROM webhook_log
		     WHERE event_type = 'payment_late' AND gateway_ref = $2 AND payload->>'reference' = $3)`,
		payload, orderID.String(), reference,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
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
// payment_pending one. False means the order moved on (expired, paid,
// cancelled) and the payment must not be attached or offered.
func (r *Repository) SetPaymentRefTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, ref string, expiresAt time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE orders SET gateway_ref = $1, payment_expires_at = $2, checked_out_at = now(), updated_at = now()
		 WHERE id = $3 AND status = 'payment_pending'
		   AND (payment_expires_at IS NULL OR payment_expires_at > now())`,
		ref, expiresAt, orderID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListOrderDigitalTargetsTx returns every target the order claimed at
// checkout, released or not, under the product line that reached it.
func (r *Repository) ListOrderDigitalTargetsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) ([]model.DigitalTarget, error) {
	rows, err := tx.Query(ctx,
		`SELECT t.order_item_id, oi.product_id, t.target_kind, t.target_id
		 FROM order_digital_target t
		 JOIN order_item oi ON oi.id = t.order_item_id
		 WHERE t.order_id = $1
		 ORDER BY t.target_kind, t.target_id`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []model.DigitalTarget
	for rows.Next() {
		var t model.DigitalTarget
		if err := rows.Scan(&t.OrderItemID, &t.ProductID, &t.Kind, &t.TargetID); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}
