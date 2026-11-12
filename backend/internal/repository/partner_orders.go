package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

// PartnerOrderFilter narrows a Mitra's order list. Q matches a product on the
// order, a student it covers, or the end of the order id; From and To bound the
// day the order was made, To exclusive.
type PartnerOrderFilter struct {
	Status string
	Q      string
	From   *time.Time
	To     *time.Time
}

// ListPartnerOrdersPage returns one page of the orders the Mitra bought, newest
// first, with their items, and the total the filters match.
func (r *Repository) ListPartnerOrdersPage(ctx context.Context, partnerID uuid.UUID, filter PartnerOrderFilter, page, perPage int) ([]model.Order, int, error) {
	where := ` WHERE o.student_id = $1`
	args := []any{partnerID}
	argNum := 2
	if filter.Status != "" {
		where += fmt.Sprintf(` AND o.status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.From != nil {
		where += fmt.Sprintf(` AND o.created_at >= $%d`, argNum)
		args = append(args, *filter.From)
		argNum++
	}
	if filter.To != nil {
		where += fmt.Sprintf(` AND o.created_at < $%d`, argNum)
		args = append(args, *filter.To)
		argNum++
	}
	if filter.Q != "" {
		where += fmt.Sprintf(` AND (o.id::text ILIKE $%d
			OR EXISTS (SELECT 1 FROM order_item oi WHERE oi.order_id = o.id AND oi.name ILIKE $%d)
			OR EXISTS (SELECT 1 FROM order_participant op JOIN users u ON u.id = op.student_id
				WHERE op.order_id = o.id AND u.name ILIKE $%d))`, argNum, argNum, argNum)
		args = append(args, "%"+filter.Q+"%")
		argNum++
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM orders o`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT o.id FROM orders o`+where+fmt.Sprintf(` ORDER BY o.created_at DESC, o.id DESC LIMIT $%d OFFSET $%d`, argNum, argNum+1),
		append(args, perPage, (page-1)*perPage)...,
	)
	if err != nil {
		return nil, 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	orders := make([]model.Order, 0, len(ids))
	for _, id := range ids {
		order, err := r.GetOrderByID(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		items, err := r.fetchItems(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		order.Items = items
		orders = append(orders, order)
	}
	return orders, total, nil
}
