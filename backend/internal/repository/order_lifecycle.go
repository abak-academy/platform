package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The UPDATE carries status and deadline itself, so only one caller among sweeper, webhook and reconcile sees true and stock is restored once.
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
		 FROM (SELECT oi.product_id, sum(oi.qty) AS qty FROM order_item oi
		       WHERE oi.order_id = $1
		         AND (oi.product_type IN ('book', 'merchandise', 'medal')
		              OR (oi.product_type = 'exam' AND NOT EXISTS (
		                  SELECT 1 FROM order_participant op WHERE op.order_id = oi.order_id)))
		       GROUP BY oi.product_id) i
		 WHERE p.id = i.product_id AND p.stock IS NOT NULL`,
		orderID,
	); err != nil {
		return false, err
	}
	return true, nil
}

// RecordLatePaymentTx writes one payment_late flag per order and payment; a reference equal to detail["gateway_ref"] is an unknown transaction id (see the spec).
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
	err = r.InsertWebhookLog(ctx, tx, "payment_late", payload, key)
	return err == nil, err
}

// PaymentDeadlineElapsedTx judges the deadline on the database clock, like the sweeper; NULL (a legacy order) has not elapsed.
func (r *Repository) PaymentDeadlineElapsedTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (bool, error) {
	var elapsed bool
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(payment_expires_at <= now(), false) FROM orders WHERE id = $1`, orderID,
	).Scan(&elapsed)
	return elapsed, err
}

// The deadline is ABAK's and never moves (only a legacy order with none gets one); not attached means the order moved on and no payment may be offered.
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
