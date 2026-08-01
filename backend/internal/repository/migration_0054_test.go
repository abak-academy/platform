package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0054_PromoIsPublic proves 0054 adds promo_code.is_public as
// BOOLEAN NOT NULL DEFAULT false — FR-8: every pre-existing promo code reads
// back is_public = false immediately after the migration runs, so nothing
// leaks the moment it ships. The down migration drops only that column.
func TestMigration0054_PromoIsPublic(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0050_per_item_points.up.sql")

	assertColumnExists := func(want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'promo_code' AND column_name = 'is_public')`,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}
	assertColumnExists(false, "is_public must not exist before 0054")

	var preExistingID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO promo_code (code) VALUES ($1) RETURNING id`,
		"MIGRATION-0054-PRE",
	).Scan(&preExistingID))

	applyMigrationFile(t, pool, "0054_promo_is_public.up.sql")

	assertColumnExists(true, "is_public must exist after 0054")

	var isPublic bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT is_public FROM promo_code WHERE id = $1`, preExistingID,
	).Scan(&isPublic))
	require.False(t, isPublic, "every pre-existing promo code must read back is_public = false")

	applyMigrationFile(t, pool, "0054_promo_is_public.down.sql")

	assertColumnExists(false, "is_public must be dropped by down")

	var codeIntact string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT code FROM promo_code WHERE id = $1`, preExistingID,
	).Scan(&codeIntact))
	require.Equal(t, "MIGRATION-0054-PRE", codeIntact, "down must not touch any other promo_code column")
}
