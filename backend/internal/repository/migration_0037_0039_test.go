package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// These two tests run at the migration boundary: they seed rows that predate the
// migration, apply the real .up.sql, and assert on what the backfill produced.
// Tests that start from a fully-migrated schema cannot cover this — an insert
// after the fact exercises the column DEFAULT, not the backfill, so ordering,
// preservation and the sequence high-water mark all go unchecked (NFR-6).

// seedPre0037Registration inserts a registration with an explicit created_at so
// the backfill's ORDER BY created_at, id is exercised with a known answer.
func seedPre0037Registration(t *testing.T, pool *pgxpool.Pool, examID, studentID uuid.UUID, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO exam_registration (student_id, exam_id, token, created_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		studentID, examID, "seed-token-"+uuid.NewString(), createdAt,
	).Scan(&id))
	return id
}

func seedMigrationStudent(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'student', 'active', $2, '') RETURNING id`,
		name, "mig_"+uuid.NewString()[:8],
	).Scan(&id))
	return id
}

func TestMigration0037_ParticipantNumberBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0036_exam_schedule_end.up.sql")

	var examA, examB uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ('Exam A') RETURNING id`).Scan(&examA))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ('Exam B') RETURNING id`).Scan(&examB))

	base := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	// Inserted out of chronological order on purpose: the backfill must number by
	// created_at, not by insertion order.
	second := seedPre0037Registration(t, pool, examA, seedMigrationStudent(t, pool, "A Second"), base.Add(2*time.Hour))
	first := seedPre0037Registration(t, pool, examA, seedMigrationStudent(t, pool, "A First"), base)
	third := seedPre0037Registration(t, pool, examA, seedMigrationStudent(t, pool, "A Third"), base.Add(4*time.Hour))
	// A different exam must get its own sequence starting at 1 (PARTITION BY exam_id).
	otherExamOnly := seedPre0037Registration(t, pool, examB, seedMigrationStudent(t, pool, "B Only"), base.Add(time.Hour))

	applyMigrationFile(t, pool, "0037_participant_number.up.sql")

	participantNumber := func(regID uuid.UUID) int {
		t.Helper()
		var n *int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT participant_number FROM exam_registration WHERE id = $1`, regID,
		).Scan(&n))
		require.NotNil(t, n, "every pre-existing registration must be backfilled")
		return *n
	}

	require.Equal(t, 1, participantNumber(first), "earliest registration in exam A must be 1")
	require.Equal(t, 2, participantNumber(second))
	require.Equal(t, 3, participantNumber(third))
	require.Equal(t, 1, participantNumber(otherExamOnly), "numbering is per-exam, so exam B restarts at 1")

	// The unique index must actually reject a duplicate within one exam...
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number)
		 VALUES ($1, $2, $3, 1)`,
		seedMigrationStudent(t, pool, "A Duplicate"), examA, "dup-token-"+uuid.NewString(),
	)
	require.Error(t, err, "a duplicate participant_number within an exam must be rejected")

	// ...while the same number in a different exam stays legal.
	_, err = pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number)
		 VALUES ($1, $2, $3, 3)`,
		seedMigrationStudent(t, pool, "B Third"), examB, "ok-token-"+uuid.NewString(),
	)
	require.NoError(t, err, "the same participant_number in another exam must be allowed")

	applyMigrationFile(t, pool, "0037_participant_number.down.sql")
	var exists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'exam_registration' AND column_name = 'participant_number')`,
	).Scan(&exists))
	require.False(t, exists, "down must drop participant_number")
}

func TestMigration0039_ExamNumberBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0038_product_availability.up.sql")

	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	seedExamAt := func(title string, createdAt time.Time) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO exam (title, created_at) VALUES ($1, $2) RETURNING id`, title, createdAt,
		).Scan(&id))
		return id
	}

	// Again inserted out of order, so numbering by created_at is a real assertion.
	middle := seedExamAt("Middle Exam", base.Add(2*time.Hour))
	oldest := seedExamAt("Oldest Exam", base)
	newest := seedExamAt("Newest Exam", base.Add(5*time.Hour))

	applyMigrationFile(t, pool, "0039_exam_number.up.sql")

	examNumber := func(id uuid.UUID) int {
		t.Helper()
		var n *int
		require.NoError(t, pool.QueryRow(ctx, `SELECT exam_number FROM exam WHERE id = $1`, id).Scan(&n))
		require.NotNil(t, n, "every pre-existing exam must be backfilled")
		return *n
	}

	require.Equal(t, 1, examNumber(oldest), "backfill must number in created_at order")
	require.Equal(t, 2, examNumber(middle))
	require.Equal(t, 3, examNumber(newest))

	// The sequence must resume past the backfilled rows, not collide with them —
	// this is what a post-migration-only test cannot see.
	var fresh *int
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title) VALUES ('Post-migration Exam') RETURNING exam_number`,
	).Scan(&fresh))
	require.NotNil(t, fresh, "the column DEFAULT must assign a number")
	require.Equal(t, 4, *fresh, "the sequence must continue after the highest backfilled number")

	// NOT NULL and uniqueness are both part of the migration's contract.
	_, err := pool.Exec(ctx, `INSERT INTO exam (title, exam_number) VALUES ('Dup Exam', 1)`)
	require.Error(t, err, "a duplicate exam_number must be rejected")
	_, err = pool.Exec(ctx, `UPDATE exam SET exam_number = NULL WHERE id = $1`, oldest)
	require.Error(t, err, "exam_number must be NOT NULL after the migration")

	applyMigrationFile(t, pool, "0039_exam_number.down.sql")
	var colExists, seqExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'exam' AND column_name = 'exam_number')`,
	).Scan(&colExists))
	require.False(t, colExists, "down must drop exam_number")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = 'exam_number_seq')`,
	).Scan(&seqExists))
	require.False(t, seqExists, "down must drop exam_number_seq")
}
