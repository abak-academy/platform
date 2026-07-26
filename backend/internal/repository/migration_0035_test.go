package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// TestMigration0035_CertificateDesign proves 0035 adds the certificate design columns
// on exam and the certificate_number column/index/sequence on exam_session, widens the
// certificate_template CHECK constraint to admit 'custom', and that .down.sql reverses
// all of it (including narrowing the CHECK constraint back).
func TestMigration0035_CertificateDesign(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Bring the DB to exactly the pre-0035 schema (0034 is the last migration before
	// the deliberate 0035 gap; 0036 already exists on main and is unaffected).
	applyMigrationsUpTo(t, pool, "0034_shipping_selected_service.up.sql")

	assertColumnExists := func(table, column string, want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
			table, column,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}

	// Pre-0035: none of the new columns exist.
	assertColumnExists("exam", "certificate_background_key", false, "certificate_background_key must not exist before 0035")
	assertColumnExists("exam", "certificate_layout", false, "certificate_layout must not exist before 0035")
	assertColumnExists("exam", "certificate_design_updated_at", false, "certificate_design_updated_at must not exist before 0035")
	assertColumnExists("exam_session", "certificate_number", false, "certificate_number must not exist before 0035")

	// Apply 0035 up.
	applyMigrationFile(t, pool, "0035_certificate_design.up.sql")

	// FR: all three exam columns exist.
	assertColumnExists("exam", "certificate_background_key", true, "certificate_background_key must exist after 0035")
	assertColumnExists("exam", "certificate_layout", true, "certificate_layout must exist after 0035")
	assertColumnExists("exam", "certificate_design_updated_at", true, "certificate_design_updated_at must exist after 0035")

	var dataType string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'exam' AND column_name = 'certificate_layout'`,
	).Scan(&dataType))
	require.Equal(t, "jsonb", dataType, "certificate_layout must be JSONB")

	// FR: exam_session.certificate_number exists.
	assertColumnExists("exam_session", "certificate_number", true, "certificate_number must exist after 0035")

	// FR-11: unique partial index on certificate_number exists.
	var indexExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename = 'exam_session' AND indexname = 'idx_exam_session_certificate_number')`,
	).Scan(&indexExists))
	require.True(t, indexExists, "idx_exam_session_certificate_number must exist after 0035")

	var indexDef string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE tablename = 'exam_session' AND indexname = 'idx_exam_session_certificate_number'`,
	).Scan(&indexDef))
	require.True(t, strings.Contains(indexDef, "UNIQUE"), "index must be UNIQUE, got: %s", indexDef)
	require.True(t, strings.Contains(indexDef, "certificate_number IS NOT NULL"), "index must be partial (WHERE certificate_number IS NOT NULL), got: %s", indexDef)

	// certificate_number_seq exists.
	var seqExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_sequences WHERE sequencename = 'certificate_number_seq')`,
	).Scan(&seqExists))
	require.True(t, seqExists, "certificate_number_seq must exist after 0035")

	// NFR-4: certificate_template CHECK constraint now admits 'custom'.
	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, certificate_template) VALUES ($1, 'custom') RETURNING id`,
		"Custom Template Exam",
	).Scan(&examID))

	// The unique partial index tolerates any number of NULLs but rejects duplicates
	// among non-NULL values. Two distinct students so (student_id, exam_id) stays
	// unique per uq_examregistration.
	var studentID1, studentID2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"cert-idx-0035-a@test.local", "student", "Cert Idx Test A",
	).Scan(&studentID1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"cert-idx-0035-b@test.local", "student", "Cert Idx Test B",
	).Scan(&studentID2))
	var regID1, regID2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID1, examID, uuid.NewString(),
	).Scan(&regID1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID2, examID, uuid.NewString(),
	).Scan(&regID2))
	var sessionID1, sessionID2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, certificate_number)
		VALUES ($1, $2, $3, now(), $4) RETURNING id`,
		regID1, studentID1, examID, "ABK/2026/000001",
	).Scan(&sessionID1))
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, certificate_number)
		VALUES ($1, $2, $3, now(), $4)`,
		regID2, studentID2, examID, "ABK/2026/000001",
	)
	require.Error(t, err, "duplicate certificate_number must violate the unique partial index")
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, "23505", pgErr.Code, "want unique_violation, got %s: %v", pgErr.Code, err)

	// Two NULL certificate_number rows are fine (partial index excludes NULLs).
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at)
		VALUES ($1, $2, $3, now()) RETURNING id`,
		regID2, studentID2, examID,
	).Scan(&sessionID2))
	_ = sessionID2

	// Apply 0035 down.
	applyMigrationFile(t, pool, "0035_certificate_design.down.sql")

	assertColumnExists("exam", "certificate_background_key", false, "certificate_background_key must be dropped by down")
	assertColumnExists("exam", "certificate_layout", false, "certificate_layout must be dropped by down")
	assertColumnExists("exam", "certificate_design_updated_at", false, "certificate_design_updated_at must be dropped by down")
	assertColumnExists("exam_session", "certificate_number", false, "certificate_number must be dropped by down")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename = 'exam_session' AND indexname = 'idx_exam_session_certificate_number')`,
	).Scan(&indexExists))
	require.False(t, indexExists, "idx_exam_session_certificate_number must be dropped by down")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_sequences WHERE sequencename = 'certificate_number_seq')`,
	).Scan(&seqExists))
	require.False(t, seqExists, "certificate_number_seq must be dropped by down")

	// The CHECK constraint is narrowed back to the original 3 values.
	_, err = pool.Exec(ctx,
		`INSERT INTO exam (title, certificate_template) VALUES ($1, 'custom')`,
		"Post-down Custom Exam",
	)
	require.Error(t, err, "'custom' must be rejected again after down narrows the CHECK constraint")
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, "23514", pgErr.Code, "want check_violation, got %s: %v", pgErr.Code, err)

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, certificate_template) VALUES ($1, 'classic') RETURNING id`,
		"Post-down Classic Exam",
	).Scan(&examID))
}
