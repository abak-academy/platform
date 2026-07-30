package repository

import (
	"context"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// InsertShipmentEvent records one shipment status event for an order. The
// UNIQUE (order_id, status, occurred_at) constraint plus ON CONFLICT DO
// NOTHING is what makes a replayed webhook inert (FR-C-13): a second
// identical insert returns no error and creates no duplicate row.
func (r *Repository) InsertShipmentEvent(ctx context.Context, event model.OrderShipmentEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO order_shipment_events (order_id, status, courier_waybill_id, courier_driver_name, occurred_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (order_id, status, occurred_at) DO NOTHING`,
		event.OrderID, event.Status, event.CourierWaybillID, event.CourierDriverName, event.OccurredAt,
	)
	return err
}

// ListShipmentEvents returns every event for an order ordered by occurred_at
// (FR-C-16), oldest first.
func (r *Repository) ListShipmentEvents(ctx context.Context, orderID uuid.UUID) ([]model.OrderShipmentEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, order_id, status, courier_waybill_id, courier_driver_name, occurred_at, created_at
		 FROM order_shipment_events
		 WHERE order_id = $1
		 ORDER BY occurred_at`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []model.OrderShipmentEvent{}
	for rows.Next() {
		event := model.OrderShipmentEvent{}
		if err := rows.Scan(&event.ID, &event.OrderID, &event.Status,
			&event.CourierWaybillID, &event.CourierDriverName, &event.OccurredAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
