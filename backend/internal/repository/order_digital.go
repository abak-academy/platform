package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// digitalQueryer runs on the caller's transaction when there is one, so a
// checkout reads links and live targets in the same snapshot it writes in.
func (r *Repository) digitalQueryer(tx pgx.Tx) queryer {
	if tx != nil {
		return tx
	}
	return r.pool
}

// ListProductDigitalTargets returns the course or exam ids a product is linked
// to today, in a stable order. Types other than course and exam link nothing.
func (r *Repository) ListProductDigitalTargets(ctx context.Context, tx pgx.Tx, productType string, productID uuid.UUID) ([]uuid.UUID, error) {
	var query string
	switch productType {
	case model.DigitalTargetCourse:
		query = `SELECT course_id FROM product_course WHERE product_id = $1 ORDER BY course_id`
	case model.DigitalTargetExam:
		query = `SELECT exam_id FROM product_exam WHERE product_id = $1 ORDER BY exam_id`
	default:
		return nil, nil
	}
	rows, err := r.digitalQueryer(tx).Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListLiveDigitalTargets returns the courses and exams the student already has
// live access to ('owned': an active course_session, a non-revoked
// exam_registration) or is already buying ('in_flight': a payment_pending, paid
// or processing order, other than excludeOrderID, whose products link them).
// Nothing is stored for this: it is derived from the order and access rows, so
// an expired, cancelled or refunded order, or a revoked access, stops counting
// by itself. A target can come back as both.
func (r *Repository) ListLiveDigitalTargets(ctx context.Context, tx pgx.Tx, studentID, excludeOrderID uuid.UUID) ([]model.LiveDigitalTarget, error) {
	rows, err := r.digitalQueryer(tx).Query(ctx, `
		WITH in_flight AS (
		    SELECT o.id AS order_id, 'course'::text AS kind, pc.course_id AS target_id
		      FROM orders o
		      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
		      JOIN product_course pc ON pc.product_id = oi.product_id
		     WHERE o.student_id = $1 AND o.id <> $2
		       AND o.status IN ('payment_pending', 'paid', 'processing')
		    UNION ALL
		    SELECT o.id, 'exam'::text, pe.exam_id
		      FROM orders o
		      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
		      JOIN product_exam pe ON pe.product_id = oi.product_id
		     WHERE o.student_id = $1 AND o.id <> $2
		       AND o.status IN ('payment_pending', 'paid', 'processing')
		)
		SELECT f.kind, f.target_id, 'in_flight' AS reason, (array_agg(f.order_id ORDER BY f.order_id))[1] AS order_id
		  FROM in_flight f GROUP BY f.kind, f.target_id
		UNION
		SELECT 'course', cs.course_id, 'owned', NULL::uuid FROM course_session cs
		 WHERE cs.student_id = $1 AND cs.status = 'active'
		UNION
		SELECT 'exam', er.exam_id, 'owned', NULL::uuid FROM exam_registration er
		 WHERE er.student_id = $1 AND er.status <> 'revoked'`,
		studentID, excludeOrderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var live []model.LiveDigitalTarget
	for rows.Next() {
		var t model.LiveDigitalTarget
		if err := rows.Scan(&t.Kind, &t.TargetID, &t.Reason, &t.OrderID); err != nil {
			return nil, err
		}
		live = append(live, t)
	}
	return live, rows.Err()
}
