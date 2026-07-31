package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0048_ShipmentTracking proves 0048 adds the five nullable
// shipment columns to orders and creates order_shipment_events with
// UNIQUE (order_id, status, occurred_at) so a replayed webhook insert is
// inert — no error, no duplicate row (FR-C-4). The down migration must drop
// exactly the table and the five columns and nothing else.
func TestMigration0048_ShipmentTracking(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0047_order_is_estimate.up.sql")

	assertOrderColumnExists := func(column string, want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'orders' AND column_name = $1)`,
			column,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}
	assertNullable := func(column string, msg string) {
		t.Helper()
		var nullable string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'orders' AND column_name = $1`,
			column,
		).Scan(&nullable))
		require.Equal(t, "YES", nullable, msg)
	}
	assertTableExists := func(want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'order_shipment_events')`,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}

	newColumns := []string{"biteship_order_id", "shipment_status", "waybill_source", "courier_code", "courier_service_code"}
	for _, col := range newColumns {
		assertOrderColumnExists(col, false, col+" must not exist before 0048")
	}
	assertTableExists(false, "order_shipment_events must not exist before 0048")

	var studentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"migration-0048@test.local", "student", "Migration 0048 Test",
	).Scan(&studentID))

	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status) VALUES ($1, 'paid') RETURNING id`,
		studentID,
	).Scan(&orderID))

	applyMigrationFile(t, pool, "0048_shipment_tracking.up.sql")

	for _, col := range newColumns {
		assertOrderColumnExists(col, true, col+" must exist after 0048")
		assertNullable(col, col+" must be nullable")
	}
	assertTableExists(true, "order_shipment_events must exist after 0048")

	// FR-C-4 / the point of the table: a replayed webhook insert with the
	// same (order_id, status, occurred_at) must be inert — no error, no
	// second row — via ON CONFLICT DO NOTHING at the repository layer, but
	// the constraint itself must exist and reject a plain duplicate INSERT
	// unless ON CONFLICT is used. Prove the constraint is present by relying
	// on ON CONFLICT DO NOTHING here directly.
	occurredAt := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	insertEvent := func() {
		_, err := pool.Exec(ctx,
			`INSERT INTO order_shipment_events (order_id, status, courier_waybill_id, courier_driver_name, occurred_at)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (order_id, status, occurred_at) DO NOTHING`,
			orderID, "confirmed", "WB123", "Budi Test", occurredAt,
		)
		require.NoError(t, err)
	}
	insertEvent()
	insertEvent()

	var eventCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM order_shipment_events WHERE order_id = $1`, orderID,
	).Scan(&eventCount))
	require.Equal(t, 1, eventCount, "a replayed identical event must not create a second row")

	// A plain duplicate INSERT without ON CONFLICT must fail on the
	// constraint — proving the UNIQUE constraint itself exists, not just
	// that our ON CONFLICT clause happens to work.
	_, err := pool.Exec(ctx,
		`INSERT INTO order_shipment_events (order_id, status, courier_waybill_id, courier_driver_name, occurred_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		orderID, "confirmed", "WB123", "Budi Test", occurredAt,
	)
	require.Error(t, err, "a duplicate (order_id, status, occurred_at) INSERT without ON CONFLICT must violate the UNIQUE constraint")

	applyMigrationFile(t, pool, "0048_shipment_tracking.down.sql")

	for _, col := range newColumns {
		assertOrderColumnExists(col, false, col+" must be dropped by down")
	}
	assertTableExists(false, "order_shipment_events must be dropped by down")

	var otherColumnsIntact int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE id = $1 AND status = 'paid'`, orderID,
	).Scan(&otherColumnsIntact))
	require.Equal(t, 1, otherColumnsIntact, "down must not touch any other order column")
}
