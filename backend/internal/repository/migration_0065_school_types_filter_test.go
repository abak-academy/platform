package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0065_SchoolTypesFilter(t *testing.T) {
	ctx := t.Context()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0064_school_fields.up.sql")
	var schoolID, userID string
	types := []string{"SD", "SMP", "SMA"}
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types, category) VALUES ('Legacy Multi Level', 'migration-0065', $1, 'SD') RETURNING id`, types,
	).Scan(&schoolID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_id, jenjang) VALUES ('migration-0065@test.local', 'student', 'Migration Student', $1, 'SMP') RETURNING id`, schoolID,
	).Scan(&userID))

	applyMigrationFile(t, pool, "0065_school_types_filter.up.sql")
	requireColumnExists(t, pool, "school", "category", false)
	requireColumnExists(t, pool, "school", "provinsi_id", true)
	requireColumnExists(t, pool, "school", "kota_id", true)
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_kota_category_name_id", false)
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_kota_name_id")
	requireSchoolSearchIndexExists(t, pool, "idx_school_active_npsn")

	var savedTypes []string
	var linkedSchoolID, jenjang string
	require.NoError(t, pool.QueryRow(ctx, `SELECT school_types FROM school WHERE id = $1`, schoolID).Scan(&savedTypes))
	require.Equal(t, types, savedTypes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT school_id, jenjang FROM users WHERE id = $1`, userID).Scan(&linkedSchoolID, &jenjang))
	require.Equal(t, schoolID, linkedSchoolID)
	require.Equal(t, "SMP", jenjang)

	applyMigrationFile(t, pool, "0065_school_types_filter.down.sql")
	requireColumnExists(t, pool, "school", "category", true)
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_kota_category_name_id")
	requireSchoolSearchIndexExists(t, pool, "idx_school_provinsi_kota_name_id", false)
	require.NoError(t, pool.QueryRow(ctx, `SELECT school_types FROM school WHERE id = $1`, schoolID).Scan(&savedTypes))
	require.Equal(t, types, savedTypes)
}
