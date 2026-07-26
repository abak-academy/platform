package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestMigration0041_DropCertificateNumberSeq proves 0041 drops the sequence
// certificate numbers no longer come from (they are composed in Go, FR-25), and
// that .down.sql restores it in a usable state.
//
// The down direction is the one with teeth: a rollback puts the pre-0041 code
// back, and that code allocates certificate numbers with nextval(). Recreating
// the sequence at its default start would re-issue numbers already sitting in
// exam_session.certificate_number, which idx_exam_session_certificate_number
// rejects — the rollback would leave certificate allocation permanently broken.
func TestMigration0041_DropCertificateNumberSeq(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Bring the DB to exactly the pre-0041 schema.
	applyMigrationsUpTo(t, pool, "0040_shipping_cost.up.sql")

	assertSequenceExists := func(want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = 'certificate_number_seq')`,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}

	assertSequenceExists(true, "certificate_number_seq must exist before 0041")

	// Seed history the way the pre-0041 code left it: numbers already allocated
	// off the sequence, the highest being 000007.
	examID := seedCertSeqExam(t, pool)
	for _, n := range []string{"ABK/2026/000003", "ABK/2026/000007", "ABK/2026/000001"} {
		seedCertSeqSession(t, pool, examID, n)
	}
	_, err := pool.Exec(ctx, `SELECT setval('certificate_number_seq', 7)`)
	require.NoError(t, err)

	// Apply 0041 up.
	applyMigrationFile(t, pool, "0041_drop_certificate_number_seq.up.sql")
	assertSequenceExists(false, "certificate_number_seq must be dropped by 0041")

	// Certificate numbers keep being composed in Go while the sequence is gone,
	// so history grows past the old high-water mark.
	seedCertSeqSession(t, pool, examID, "ABK/2026/000009")

	// Apply 0041 down.
	applyMigrationFile(t, pool, "0041_drop_certificate_number_seq.down.sql")
	assertSequenceExists(true, "certificate_number_seq must be restored by down")

	// The restored sequence must not hand back a number that already exists.
	var next int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT nextval('certificate_number_seq')`).Scan(&next))
	require.Greater(t, next, int64(9),
		"nextval must resume past the highest allocated certificate number, otherwise the rolled-back code re-issues a taken number")

	// Prove it end to end: allocating the way the pre-0041 code did must insert
	// cleanly against the live unique index.
	seedCertSeqSession(t, pool, examID, fmt.Sprintf("ABK/2026/%06d", next))
}

// seedCertSeqExam creates an exam plus the student the sessions below hang off.
func seedCertSeqExam(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO exam (title) VALUES ($1) RETURNING id`,
		"Cert Seq Exam "+uuid.NewString()[:8],
	).Scan(&id))
	return id
}

// newCertSeqRegistration creates a fresh student + registration, since
// exam_session requires both and uq_examregistration forbids reusing a student
// for the same exam.
func newCertSeqRegistration(t *testing.T, pool *pgxpool.Pool, examID uuid.UUID) (regID, studentID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Cert Seq Student', 'student', 'active', $1, '') RETURNING id`,
		"certseq_"+suffix,
	).Scan(&studentID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, "certseq-token-"+suffix,
	).Scan(&regID))
	return regID, studentID
}

func seedCertSeqSession(t *testing.T, pool *pgxpool.Pool, examID uuid.UUID, number string) {
	t.Helper()
	regID, studentID := newCertSeqRegistration(t, pool, examID)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, certificate_number)
		 VALUES ($1, $2, $3, now(), 'submitted', $4)`,
		regID, studentID, examID, number,
	)
	require.NoError(t, err)
}
