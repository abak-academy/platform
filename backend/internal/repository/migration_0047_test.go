package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0047_OrderIsEstimate proves 0047 adds orders.is_estimate as
// BOOLEAN NOT NULL DEFAULT false and backfills true for every pre-existing
// row whose selected_courier is the current flat-rate label ('Ongkir Flat')
// or the legacy one ('Flat') — FR-B-1 — while a real-carrier row stays false.
// The down migration must drop only that column (FR-B-2).
func TestMigration0047_OrderIsEstimate(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0046_papua_2022_regions.up.sql")

	assertColumnExists := func(want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'orders' AND column_name = 'is_estimate')`,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}
	assertColumnExists(false, "is_estimate must not exist before 0047")

	var studentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"migration-0047@test.local", "student", "Migration 0047 Test",
	).Scan(&studentID))

	insertOrder := func(courier string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, selected_courier) VALUES ($1, 'paid', $2) RETURNING id`,
			studentID, courier,
		).Scan(&id))
		return id
	}

	ongkirFlatID := insertOrder("Ongkir Flat")
	legacyFlatID := insertOrder("Flat")
	jneID := insertOrder("JNE")

	applyMigrationFile(t, pool, "0047_order_is_estimate.up.sql")

	assertColumnExists(true, "is_estimate must exist after 0047")

	readIsEstimate := func(id uuid.UUID) bool {
		var isEstimate bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_estimate FROM orders WHERE id = $1`, id,
		).Scan(&isEstimate))
		return isEstimate
	}
	require.True(t, readIsEstimate(ongkirFlatID), "'Ongkir Flat' must backfill to true")
	require.True(t, readIsEstimate(legacyFlatID), "legacy 'Flat' must backfill to true")
	require.False(t, readIsEstimate(jneID), "a real-carrier order must stay false")

	applyMigrationFile(t, pool, "0047_order_is_estimate.down.sql")

	assertColumnExists(false, "is_estimate must be dropped by down")

	var otherColumnsIntact int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE id = $1 AND selected_courier = 'Ongkir Flat'`, ongkirFlatID,
	).Scan(&otherColumnsIntact))
	require.Equal(t, 1, otherColumnsIntact, "down must not touch any other order column")
}
