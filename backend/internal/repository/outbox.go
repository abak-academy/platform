package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

func (r *Repository) InsertOutboxEvent(ctx context.Context, tx pgx.Tx, aggregateType string, aggregateID uuid.UUID, eventType string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) VALUES ($1, $2, $3, $4)`,
		aggregateType, aggregateID, eventType, b,
	)
	return err
}

func (r *Repository) ClaimOutboxEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at::text, attempts, last_error
		 FROM outbox
		 WHERE processed_at IS NULL
		 ORDER BY created_at
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload, &e.CreatedAt, &e.Attempts, &e.LastError); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *Repository) MarkOutboxProcessed(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE outbox SET processed_at = now() WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) HasPendingQuestionBundleEvent(ctx context.Context, testID uuid.UUID, variant string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM outbox
			WHERE aggregate_type = $1 AND aggregate_id = $2
			  AND event_type = 'QuestionBundleNeeded' AND processed_at IS NULL
			  AND payload->>'variant' = $3
		)`,
		"question_bundle_test", testID, variant,
	).Scan(&exists)
	return exists, err
}
