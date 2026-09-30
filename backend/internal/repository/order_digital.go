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
// exam_registration) or is already buying ('in_flight': a live order of this
// student, other than excludeOrderID, that still owes or holds the target).
// Nothing is stored for this: it is derived from the order and access rows, so
// a cancelled or expired order, or a revoked access, stops counting by itself.
// A target can come back as both.
//
// Excluding statuses rather than listing them means a new status blocks instead of reopening a double purchase.
func (r *Repository) ListLiveDigitalTargets(ctx context.Context, tx pgx.Tx, studentID, excludeOrderID uuid.UUID) ([]model.LiveDigitalTarget, error) {
	rows, err := r.digitalQueryer(tx).Query(ctx, `
		WITH in_flight AS (
		    SELECT 'course'::text AS kind, pc.course_id AS target_id
		      FROM orders o
		      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
		      JOIN product_course pc ON pc.product_id = oi.product_id
		     WHERE o.student_id = $1 AND o.id <> $2
		       AND o.status NOT IN ('cart', 'cancelled', 'payment_expired')
		    UNION ALL
		    SELECT 'exam'::text, pe.exam_id
		      FROM orders o
		      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
		      JOIN product_exam pe ON pe.product_id = oi.product_id
		     WHERE o.student_id = $1 AND o.id <> $2
		       AND o.status NOT IN ('cart', 'cancelled', 'payment_expired')
		)
		SELECT f.kind, f.target_id, 'in_flight' AS reason FROM in_flight f
		UNION
		SELECT 'course', cs.course_id, 'owned' FROM course_session cs
		 WHERE cs.student_id = $1 AND cs.status = 'active'
		UNION
		SELECT 'exam', er.exam_id, 'owned' FROM exam_registration er
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
		if err := rows.Scan(&t.Kind, &t.TargetID, &t.Reason); err != nil {
			return nil, err
		}
		live = append(live, t)
	}
	return live, rows.Err()
}
