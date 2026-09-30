package repository

import (
	"context"
	"encoding/json"
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

// RecordLatePaymentTx durably flags a payment that the gateway reports for an
// order that can no longer be fulfilled (expired, cancelled, refunded), for an
// operator to reconcile or refund by hand: webhook_log rows of event_type
// 'payment_late', keyed by the order id in gateway_ref. One flag exists per
// order and payment, so redelivered webhooks and repeated reconciles do not
// pile up rows; the caller's row lock on the order serializes concurrent
// writers. A reference equal to detail["gateway_ref"] means the gateway's
// transaction id is unknown (reconcile never learns it): such a flag is
// skipped when the order already has any, and is attributed to the transaction
// id in place when a webhook later names it, on the assumption that a gateway
// order settles once. Distinct transaction ids stay distinct flags. It reports whether a new
// flag was written. audit_log cannot carry the flag because it requires an
// actor and a webhook has none.
func (r *Repository) RecordLatePaymentTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reference string, detail map[string]any) (bool, error) {
	// Copy before adding the reference: the caller's map is not ours to mutate.
	withReference := make(map[string]any, len(detail)+1)
	for k, v := range detail {
		withReference[k] = v
	}
	withReference["reference"] = reference
	payload, err := json.Marshal(withReference)
	if err != nil {
		return false, err
	}
	key := orderID.String()
	gatewayRef, _ := detail["gateway_ref"].(string)

	if reference == "" || reference == gatewayRef {
		tag, err := tx.Exec(ctx,
			`INSERT INTO webhook_log (event_type, payload, gateway_ref)
			 SELECT 'payment_late', $1::jsonb, $2
			 WHERE NOT EXISTS (SELECT 1 FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $2)`,
			payload, key,
		)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	}

	var known bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM webhook_log
		     WHERE event_type = 'payment_late' AND gateway_ref = $1 AND payload->>'reference' = $2)`,
		key, reference,
	).Scan(&known); err != nil || known {
		return false, err
	}
	if gatewayRef != "" {
		tag, err := tx.Exec(ctx,
			`UPDATE webhook_log SET payload = payload || $1::jsonb
			 WHERE event_type = 'payment_late' AND gateway_ref = $2 AND payload->>'reference' = $3`,
			payload, key, gatewayRef,
		)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() > 0 {
			return false, nil
		}
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO webhook_log (event_type, payload, gateway_ref) VALUES ('payment_late', $1::jsonb, $2)`,
		payload, key,
	)
	return err == nil, err
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
