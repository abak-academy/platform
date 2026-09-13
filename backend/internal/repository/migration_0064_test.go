package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMigration0064_SchoolSearchMetadata(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0063_exam_session_active_index.up.sql")

	requireColumnExists(t, pool, "school", "category", false)
	requireColumnExists(t, pool, "school", "provinsi_id", false)
	requireColumnExists(t, pool, "school", "kota_id", false)

	var schoolID, userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"Metadata Migration School", "metadata-migration", "12345678", []string{"SMA"}, "Jl. Lama",
	).Scan(&schoolID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_id) VALUES ($1, 'student', $2, $3) RETURNING id`,
		"migration-0064@test.local", "Migration Student", schoolID,
	).Scan(&userID))

	applyMigrationFile(t, pool, "0064_school_search_metadata.up.sql")

	requireColumnExists(t, pool, "school", "category", true)
	requireColumnExists(t, pool, "school", "provinsi_id", true)
	requireColumnExists(t, pool, "school", "kota_id", true)
	requireSchoolSearchIndexExists(t, pool, "idx_school_active_name_trgm")
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_category_name_id")

	var category, provinsiID, kotaID *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT category, provinsi_id, kota_id FROM school WHERE id = $1`, schoolID,
	).Scan(&category, &provinsiID, &kotaID))
	require.Nil(t, category)
	require.Nil(t, provinsiID)
	require.Nil(t, kotaID)

	var linkedSchoolID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT school_id FROM users WHERE id = $1`, userID,
	).Scan(&linkedSchoolID))
	require.Equal(t, schoolID, linkedSchoolID)

	var validProvinsiID, validKotaID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT province_id, id FROM city ORDER BY id LIMIT 1`).Scan(&validProvinsiID, &validKotaID))
	_, err := pool.Exec(ctx,
		`UPDATE school SET category = $1, provinsi_id = $2, kota_id = $3 WHERE id = $4`,
		"SMA", validProvinsiID, validKotaID, schoolID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE school SET provinsi_id = $1 WHERE id = $2`,
		"invalid-province-id", schoolID,
	)
	require.Error(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE school SET kota_id = $1 WHERE id = $2`,
		"invalid-city-id", schoolID,
	)
	require.Error(t, err)

	applyMigrationFile(t, pool, "0064_school_search_metadata.down.sql")

	requireColumnExists(t, pool, "school", "category", false)
	requireColumnExists(t, pool, "school", "provinsi_id", false)
	requireColumnExists(t, pool, "school", "kota_id", false)
	requireSchoolSearchIndexExists(t, pool, "idx_school_active_name_trgm", false)
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_category_name_id", false)

	var pgTrgmExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')`,
	).Scan(&pgTrgmExists))
	require.True(t, pgTrgmExists, "down migration must not drop shared pg_trgm extension")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT school_id FROM users WHERE id = $1`, userID,
	).Scan(&linkedSchoolID))
	require.Equal(t, schoolID, linkedSchoolID)
}

func requireColumnExists(t *testing.T, pool *pgxpool.Pool, table, column string, want bool) {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		)`,
		table, column,
	).Scan(&exists))
	require.Equal(t, want, exists, "%s.%s existence", table, column)
}

func requireSchoolSearchIndexExists(t *testing.T, pool *pgxpool.Pool, indexName string, want ...bool) {
	t.Helper()
	expected := true
	if len(want) > 0 {
		expected = want[0]
	}
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1)`,
		indexName,
	).Scan(&exists))
	require.Equal(t, expected, exists, "index %s existence", indexName)
}
