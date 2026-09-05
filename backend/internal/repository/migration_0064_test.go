package repository

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	dbmigrations "akademi-bimbel/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMigration0064_SchoolNameValidation(t *testing.T) {
	t.Run("fails visibly when an existing invalid school drifts in", func(t *testing.T) {
		ctx := context.Background()
		pool := newMigration0025Pool(t)
		applyMigrationsUpTo(t, pool, "0063_exam_session_active_index.up.sql")

		_, err := pool.Exec(ctx,
			`INSERT INTO school (name, code) VALUES ($1, $2)`,
			"...", "MIGRATION-0064-INVALID",
		)
		require.NoError(t, err)

		err = applyMigrationFileErr(t, pool, "0064_school_name_validation.up.sql")
		requireCheckViolation(t, err, "school_name_meaningful_check")

		var name string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT name FROM school WHERE code = $1`,
			"MIGRATION-0064-INVALID",
		).Scan(&name))
		require.Equal(t, "...", name, "migration must not rewrite invalid drift")
		requireConstraintExists(t, pool, "school_name_meaningful_check", false)
	})

	t.Run("enforces meaningful names and preserves existing school links", func(t *testing.T) {
		ctx := context.Background()
		pool := newMigration0025Pool(t)
		applyMigrationsUpTo(t, pool, "0063_exam_session_active_index.up.sql")

		var schoolID, userID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO school (name, code, npsn, school_types, alamat, status)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			"Al-Ma'ruf", "MIGRATION-0064-VALID", "12345", []string{"sma"}, "Jl. Valid", "active",
		).Scan(&schoolID))
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (email, role, name, school_id)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			"migration-0064@test.local", "student", "Migration Student", schoolID,
		).Scan(&userID))

		applyMigrationFile(t, pool, "0064_school_name_validation.up.sql")
		requireConstraintExists(t, pool, "school_name_meaningful_check", true)

		var schoolName, code, npsn, alamat, status string
		var schoolTypes []string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT name, code, npsn, school_types, alamat, status FROM school WHERE id = $1`,
			schoolID,
		).Scan(&schoolName, &code, &npsn, &schoolTypes, &alamat, &status))
		require.Equal(t, "Al-Ma'ruf", schoolName)
		require.Equal(t, "MIGRATION-0064-VALID", code)
		require.Equal(t, "12345", npsn)
		require.Equal(t, []string{"sma"}, schoolTypes)
		require.Equal(t, "Jl. Valid", alamat)
		require.Equal(t, "active", status)

		var linkedSchoolID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT school_id FROM users WHERE id = $1`,
			userID,
		).Scan(&linkedSchoolID))
		require.Equal(t, schoolID, linkedSchoolID, "migration must not relink users")

		_, err := pool.Exec(ctx,
			`INSERT INTO school (name, code) VALUES ($1, $2)`,
			"-----", "MIGRATION-0064-PUNCT-INSERT",
		)
		requireCheckViolation(t, err, "school_name_meaningful_check")

		_, err = pool.Exec(ctx,
			`UPDATE school SET name = $1 WHERE id = $2`,
			"...", schoolID,
		)
		requireCheckViolation(t, err, "school_name_meaningful_check")

		_, err = pool.Exec(ctx,
			`INSERT INTO school (name, code) VALUES ($1, $2)`,
			"SMA Harapan-1", "MIGRATION-0064-PUNCT-VALID",
		)
		require.NoError(t, err)

		_, err = pool.Exec(ctx,
			`INSERT INTO school (name, code) VALUES ($1, $2)`,
			"１", "MIGRATION-0064-UNICODE-VALID",
		)
		require.NoError(t, err)

		applyMigrationFile(t, pool, "0064_school_name_validation.down.sql")
		requireConstraintExists(t, pool, "school_name_meaningful_check", false)

		var userStillLinked bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND school_id = $2)`,
			userID, schoolID,
		).Scan(&userStillLinked))
		require.True(t, userStillLinked, "down migration must not touch linked users")

		_, err = pool.Exec(ctx,
			`INSERT INTO school (name, code) VALUES ($1, $2)`,
			"...", "MIGRATION-0064-PUNCT-AFTER-DOWN",
		)
		require.NoError(t, err, "down migration must remove only this validation boundary")
	})
}

func applyMigrationFileErr(t *testing.T, pool *pgxpool.Pool, filename string) error {
	t.Helper()
	ctx := context.Background()
	sql, err := fs.ReadFile(dbmigrations.FS, "migrations/"+filename)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	return err
}

func requireConstraintExists(t *testing.T, pool *pgxpool.Pool, name string, want bool) {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = $1)`,
		name,
	).Scan(&exists))
	require.Equal(t, want, exists)
}

func requireCheckViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "want PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, "23514", pgErr.Code, "want check_violation, got %s: %v", pgErr.Code, err)
	require.Equal(t, constraint, pgErr.ConstraintName)
}
