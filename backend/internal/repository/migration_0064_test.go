package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestMigration0064SchoolNPSNUnique(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0063_exam_session_active_index.up.sql")

	for _, row := range []struct {
		name string
		code string
		npsn *string
	}{
		{name: "Legacy Blank One", code: "legacy_blank_1", npsn: stringPtr(" ")},
		{name: "Legacy Blank Two", code: "legacy_blank_2", npsn: stringPtr("\t")},
		{name: "Legacy Placeholder Dash One", code: "legacy_placeholder_dash_1", npsn: stringPtr("-")},
		{name: "Legacy Placeholder Dash Two", code: "legacy_placeholder_dash_2", npsn: stringPtr("-")},
		{name: "Legacy Placeholder One One", code: "legacy_placeholder_11111111_1", npsn: stringPtr("11111111")},
		{name: "Legacy Placeholder One Two", code: "legacy_placeholder_11111111_2", npsn: stringPtr("11111111")},
		{name: "Legacy Placeholder Two One", code: "legacy_placeholder_22222222_1", npsn: stringPtr("22222222")},
		{name: "Legacy Placeholder Two Two", code: "legacy_placeholder_22222222_2", npsn: stringPtr("22222222")},
		{name: "Legacy Placeholder Three One", code: "legacy_placeholder_33333333_1", npsn: stringPtr("33333333")},
		{name: "Legacy Placeholder Three Two", code: "legacy_placeholder_33333333_2", npsn: stringPtr("33333333")},
		{name: "Normalized Existing", code: "normalized_existing", npsn: stringPtr(" ab12cd34 ")},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO school (name, code, npsn) VALUES ($1, $2, $3)`, row.name, row.code, row.npsn)
		require.NoError(t, err)
	}

	applyMigrationFile(t, pool, "0064_school_npsn_unique.up.sql")

	var blankCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM school WHERE code LIKE 'legacy_blank_%' AND npsn IS NULL`).Scan(&blankCount))
	require.Equal(t, 2, blankCount)

	var placeholderCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM school WHERE code LIKE 'legacy_placeholder_%' AND npsn IS NULL`).Scan(&placeholderCount))
	require.Equal(t, 8, placeholderCount)

	var indexDef string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE tablename = 'school' AND indexname = 'uq_school_npsn_normalized'`,
	).Scan(&indexDef))
	require.Contains(t, indexDef, "UNIQUE")
	require.Contains(t, strings.ToUpper(indexDef), "UPPER(BTRIM(NPSN))")
	require.Contains(t, indexDef, "npsn IS NOT NULL")

	_, err := pool.Exec(ctx, `INSERT INTO school (name, code, npsn) VALUES ('Duplicate Normalized', 'duplicate_normalized', 'AB12CD34')`)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, "23505", pgErr.Code)
	require.Equal(t, "uq_school_npsn_normalized", pgErr.ConstraintName)

	require.NoError(t, insertSchoolWithoutNPSN(ctx, pool, "Null One", "null_one"))
	require.NoError(t, insertSchoolWithoutNPSN(ctx, pool, "Null Two", "null_two"))

	applyMigrationFile(t, pool, "0064_school_npsn_unique.down.sql")
	var indexExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename = 'school' AND indexname = 'uq_school_npsn_normalized')`,
	).Scan(&indexExists))
	require.False(t, indexExists)
}

func stringPtr(value string) *string { return &value }

func insertSchoolWithoutNPSN(ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, name, code string) error {
	_, err := pool.Exec(ctx, `INSERT INTO school (name, code, npsn) VALUES ($1, $2, NULL)`, name, code)
	return err
}
