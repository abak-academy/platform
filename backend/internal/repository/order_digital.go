package repository

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// digitalQueryer runs on the caller's transaction when there is one, so a
// checkout reads links and claims in the same snapshot it writes them in.
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

// FindUnavailableDigitalTargets reports which of targets the student already
// has live access to (an active course_session, a non-revoked
// exam_registration) or has reserved through another live claim. Access wins
// over a claim when both hold. A claim held by exceptOrderID does not count,
// so an order rechecking its own targets is not blocked by its own claims;
// uuid.Nil excepts nothing.
func (r *Repository) FindUnavailableDigitalTargets(ctx context.Context, tx pgx.Tx, studentID uuid.UUID, targets []model.DigitalTarget, exceptOrderID uuid.UUID) (map[model.DigitalTargetKey]string, error) {
	unavailable := map[model.DigitalTargetKey]string{}
	for _, kind := range []string{model.DigitalTargetCourse, model.DigitalTargetExam} {
		ids := []string{}
		for _, t := range targets {
			if t.Kind == kind {
				ids = append(ids, t.TargetID.String())
			}
		}
		if len(ids) == 0 {
			continue
		}
		accessQuery := `SELECT course_id, 'owned' FROM course_session
			 WHERE student_id = $1 AND status = 'active' AND course_id = ANY($2::uuid[])`
		if kind == model.DigitalTargetExam {
			accessQuery = `SELECT exam_id, 'owned' FROM exam_registration
			 WHERE student_id = $1 AND status <> 'revoked' AND exam_id = ANY($2::uuid[])`
		}
		rows, err := r.digitalQueryer(tx).Query(ctx,
			accessQuery+`
			 UNION ALL
			 SELECT target_id, 'claimed' FROM order_digital_target
			 WHERE student_id = $1 AND target_kind = $3 AND released_at IS NULL AND target_id = ANY($2::uuid[])
			   AND order_id <> $4`,
			studentID, ids, kind, exceptOrderID,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uuid.UUID
			var reason string
			if err := rows.Scan(&id, &reason); err != nil {
				rows.Close()
				return nil, err
			}
			key := model.DigitalTargetKey{Kind: kind, TargetID: id}
			if unavailable[key] != model.DigitalConflictOwned {
				unavailable[key] = reason
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return unavailable, nil
}

// ClaimDigitalTargetsTx reserves targets for the order and stamps the order as
// snapshotted, inside the caller's transaction. Targets already held by a live
// claim of the same student are returned and left unclaimed; the caller must
// roll back when any come back. Rows are inserted in a fixed order so two
// checkouts contending for overlapping targets wait on each other rather than
// deadlock.
func (r *Repository) ClaimDigitalTargetsTx(ctx context.Context, tx pgx.Tx, orderID, studentID uuid.UUID, targets []model.DigitalTarget) ([]model.DigitalTarget, error) {
	ordered := append([]model.DigitalTarget(nil), targets...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Kind != ordered[j].Kind {
			return ordered[i].Kind < ordered[j].Kind
		}
		return ordered[i].TargetID.String() < ordered[j].TargetID.String()
	})

	var conflicted []model.DigitalTarget
	for _, t := range ordered {
		tag, err := tx.Exec(ctx,
			`INSERT INTO order_digital_target (order_id, order_item_id, student_id, target_kind, target_id)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (student_id, target_kind, target_id) WHERE released_at IS NULL DO NOTHING`,
			orderID, t.OrderItemID, studentID, t.Kind, t.TargetID,
		)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			conflicted = append(conflicted, t)
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET digital_targets_snapshot_at = now() WHERE id = $1`, orderID,
	); err != nil {
		return nil, err
	}
	return conflicted, nil
}

// GetOrderDigitalSnapshot returns the targets checkout reserved for the order.
// Taken is false for orders placed before snapshots existed.
func (r *Repository) GetOrderDigitalSnapshot(ctx context.Context, orderID uuid.UUID) (model.OrderDigitalSnapshot, error) {
	var snapshot model.OrderDigitalSnapshot
	if err := r.pool.QueryRow(ctx,
		`SELECT digital_targets_snapshot_at IS NOT NULL FROM orders WHERE id = $1`, orderID,
	).Scan(&snapshot.Taken); err != nil {
		if isNotFound(err) {
			return model.OrderDigitalSnapshot{}, nil
		}
		return model.OrderDigitalSnapshot{}, err
	}
	if !snapshot.Taken {
		return snapshot, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT t.order_item_id, oi.product_id, t.target_kind, t.target_id
		 FROM order_digital_target t
		 JOIN order_item oi ON oi.id = t.order_item_id
		 WHERE t.order_id = $1
		 ORDER BY t.target_kind, t.target_id`,
		orderID,
	)
	if err != nil {
		return model.OrderDigitalSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var t model.DigitalTarget
		if err := rows.Scan(&t.OrderItemID, &t.ProductID, &t.Kind, &t.TargetID); err != nil {
			return model.OrderDigitalSnapshot{}, err
		}
		snapshot.Targets = append(snapshot.Targets, t)
	}
	return snapshot, rows.Err()
}

// ReleaseDigitalTargetsTx frees the order's live claims so the student can
// buy those targets again.
func (r *Repository) ReleaseDigitalTargetsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reason string) error {
	_, err := tx.Exec(ctx,
		`UPDATE order_digital_target SET released_at = now(), release_reason = $2
		 WHERE order_id = $1 AND released_at IS NULL`,
		orderID, reason,
	)
	return err
}

// ReleaseExamClaimsAfterRevokeTx frees the purchase claims of students whose
// registration for the exam was just revoked, so the exam can be bought again.
// Only claims of fulfilled orders count: a payment_pending or paid-but-unfulfilled
// order still reserves its target, and revoking other access must not undo that.
// A claim stays while the student holds any other non-revoked registration.
func (r *Repository) ReleaseExamClaimsAfterRevokeTx(ctx context.Context, tx pgx.Tx, examID uuid.UUID, studentIDs []uuid.UUID) error {
	if len(studentIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx,
		`UPDATE order_digital_target t SET released_at = now(), release_reason = 'revoked'
		 FROM orders o
		 WHERE o.id = t.order_id AND o.status IN ('processing', 'shipped', 'completed')
		   AND t.released_at IS NULL AND t.target_kind = 'exam' AND t.target_id = $1
		   AND t.student_id = ANY($2)
		   AND NOT EXISTS (
		       SELECT 1 FROM exam_registration r
		       WHERE r.exam_id = t.target_id AND r.student_id = t.student_id AND r.status <> 'revoked')`,
		examID, studentIDs,
	)
	return err
}
