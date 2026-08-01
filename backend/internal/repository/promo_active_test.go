package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestListActivePublicPromos_FiltersByPublicAndValidity is FR-10: the
// listing returns exactly the is_public codes that are still valid under
// the same rules ValidatePromo applies — an is_public code that is expired
// or exhausted must not appear, and a private (non-public) code must never
// appear regardless of validity.
func TestListActivePublicPromos_FiltersByPublicAndValidity(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "9999_sentinel_all_migrations.up.sql")
	repo := New(pool)

	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)
	maxUsesTwo := 2

	// public + valid — must appear.
	_, err := pool.Exec(ctx,
		`INSERT INTO promo_code (code, is_public, expires_at, max_uses, used_count) VALUES ($1, true, $2, $3, 0)`,
		"PUBOK", future, maxUsesTwo,
	)
	require.NoError(t, err)

	// public + expired — must not appear.
	_, err = pool.Exec(ctx,
		`INSERT INTO promo_code (code, is_public, expires_at) VALUES ($1, true, $2)`,
		"PUBEXPIRED", past,
	)
	require.NoError(t, err)

	// public + exhausted (used_count == max_uses) — must not appear.
	_, err = pool.Exec(ctx,
		`INSERT INTO promo_code (code, is_public, max_uses, used_count) VALUES ($1, true, $2, $2)`,
		"PUBEXHAUSTED", maxUsesTwo,
	)
	require.NoError(t, err)

	// private + valid — must not appear.
	_, err = pool.Exec(ctx,
		`INSERT INTO promo_code (code, is_public, expires_at, max_uses, used_count) VALUES ($1, false, $2, $3, 0)`,
		"PRIVATEVALID", future, maxUsesTwo,
	)
	require.NoError(t, err)

	result, err := repo.ListActivePublicPromos(ctx)
	require.NoError(t, err)

	codes := make(map[string]bool, len(result))
	for _, p := range result {
		codes[p.Code] = true
	}

	require.True(t, codes["PUBOK"], "a public, valid, non-exhausted code must appear")
	require.False(t, codes["PUBEXPIRED"], "an is_public but expired code must not appear")
	require.False(t, codes["PUBEXHAUSTED"], "an is_public but exhausted code must not appear")
	require.False(t, codes["PRIVATEVALID"], "a private (non-public) code must never appear, however valid")
}
