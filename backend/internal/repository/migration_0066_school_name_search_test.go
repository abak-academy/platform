package repository

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMigration0066_SchoolNameSearch(t *testing.T) {
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0065_school_types_filter.up.sql")
	_, err := pool.Exec(t.Context(), `INSERT INTO school (name, code) VALUES ('100% Bintang', 'search-index-test')`)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `CREATE EXTENSION IF NOT EXISTS pg_trgm;
 CREATE INDEX idx_school_active_name_trgm ON school USING gin (lower(name) gin_trgm_ops) WHERE status = 'active'`)
	require.NoError(t, err)
	applyMigrationFile(t, pool, "0066_school_name_search.up.sql")
	var definition string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_school_active_name_trgm'`).Scan(&definition))
	require.Contains(t, definition, "(name gin_trgm_ops)")
	requireSchoolSearchIndexExists(t, pool, "idx_school_active_name_trgm")
	var count int
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM school WHERE name ILIKE '%Bintang%' AND code = 'search-index-test'`).Scan(&count))
	require.Equal(t, 1, count)
	applyMigrationFile(t, pool, "0066_school_name_search.down.sql")
	requireSchoolSearchIndexExists(t, pool, "idx_school_active_name_trgm", false)
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM school WHERE code = 'search-index-test'`).Scan(&count))
	require.Equal(t, 1, count)
}
