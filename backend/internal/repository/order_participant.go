package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InsertOrderItemParticipantsTx bulk-inserts order_participant rows inside the
// caller's transaction. Each row links one participant student to one order
// item, so a multi-item order can serve different students per item. Caller
// must ensure the transaction commits; no-op for empty studentIDs.
func (r *Repository) InsertOrderItemParticipantsTx(ctx context.Context, tx pgx.Tx, orderID, orderItemID uuid.UUID, studentIDs []uuid.UUID) error {
	if len(studentIDs) == 0 {
		return nil
	}

	// Single-statement VALUES bulk insert — one round-trip, no loop.
	// pgx doesn't support rows-from-values with a parameter array for the
	// UUID list, so we build the value placeholders dynamically.
	args := make([]interface{}, 0, 3*len(studentIDs))
	placeholders := ""
	for i, sid := range studentIDs {
		if i > 0 {
			placeholders += ", "
		}
		base := i * 3
		placeholders += fmt.Sprintf("($%d, $%d, $%d)", base+1, base+2, base+3)
		args = append(args, orderID, orderItemID, sid)
	}

	query := `INSERT INTO order_participant (order_id, order_item_id, student_id) VALUES ` + placeholders +
		` ON CONFLICT (order_item_id, student_id) DO NOTHING`

	_, err := tx.Exec(ctx, query, args...)
	return err
}

// GetOrderParticipants returns the distinct student_ids that have an
// order_participant row anywhere on the given order (across every item).
// Returns an empty slice (not nil) when no participants exist — this
// empty-vs-nonempty distinction is the fan-out discriminator used outside the
// per-item fulfilment path (e.g. the order-detail student list, revocation).
func (r *Repository) GetOrderParticipants(ctx context.Context, orderID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT student_id FROM order_participant WHERE order_id = $1 ORDER BY student_id`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Pre-allocate with an empty slice so zero rows returns [] not nil.
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// GetOrderItemParticipants returns the student_ids participating in one
// specific order item, so a multi-item order can be fulfilled (or
// revalidated) per item instead of applying one flat list to every item.
func (r *Repository) GetOrderItemParticipants(ctx context.Context, orderItemID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT student_id FROM order_participant WHERE order_item_id = $1 ORDER BY student_id`,
		orderItemID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// FilterAlreadyRegistered returns the subset of studentIDs that already have
// a live (non-revoked) exam_registration row for the given examID. Revoked
// registrations don't count as access — re-ordering an exam whose access was
// revoked must re-grant it, mirroring the grant-search NOT EXISTS filter.
// Returns an empty slice when none are registered.
func (r *Repository) FilterAlreadyRegistered(ctx context.Context, examID uuid.UUID, studentIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(studentIDs) == 0 {
		return []uuid.UUID{}, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT student_id FROM exam_registration WHERE exam_id = $1 AND student_id = ANY($2) AND status <> 'revoked'`,
		examID, studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
