package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestAllocateCertificateNumber proves FR-25: a certificate number is composed in Go
// as ABK/YYYY/<exam_number(pad4)>/<participant_number(pad6)> — YYYY from the exam's
// scheduled_at in WIB, exam_number and participant_number joined in from exam and
// exam_registration — minted once on first call and reused unchanged thereafter.
func TestAllocateCertificateNumber(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	studentA := insertGradingUser(t, pool, "student", "Student CertNum A")
	studentB := insertGradingUser(t, pool, "student", "Student CertNum B")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)

	scheduledAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx,
		`UPDATE exam SET exam_number = 42, scheduled_at = $1 WHERE id = $2`,
		scheduledAt, examID,
	)
	require.NoError(t, err)

	sessionA := insertCertNumSession(t, pool, studentA, examID, 5)
	sessionB := insertCertNumSession(t, pool, studentB, examID, 9)

	t.Run("first call composes ABK/YYYY/exam_number/participant_number", func(t *testing.T) {
		number, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)
		require.Equal(t, "ABK/2026/0042/000005", number)

		sess, err := repo.GetExamSessionByID(ctx, sessionA)
		require.NoError(t, err)
		require.NotNil(t, sess.CertificateNumber)
		require.Equal(t, number, *sess.CertificateNumber)
	})

	t.Run("repeated calls are idempotent", func(t *testing.T) {
		first, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)

		second, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)
		require.Equal(t, first, second)

		third, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)
		require.Equal(t, first, third)
	})

	t.Run("distinct sessions get distinct numbers", func(t *testing.T) {
		numberA, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)
		numberB, err := repo.AllocateCertificateNumber(ctx, sessionB)
		require.NoError(t, err)
		require.NotEqual(t, numberA, numberB)
		require.Equal(t, "ABK/2026/0042/000009", numberB)
	})

	t.Run("unknown session returns ErrNotFound", func(t *testing.T) {
		_, err := repo.AllocateCertificateNumber(ctx, uuid.New())
		require.ErrorIs(t, err, ErrNotFound)
	})
}

// insertCertNumSession seeds an exam_registration (with an explicit participant_number)
// + submitted exam_session pair for a student, returning the session ID.
func insertCertNumSession(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID, participantNumber int) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var regID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number) VALUES ($1, $2, $3, $4) RETURNING id`,
		studentID, examID, uuid.NewString(), participantNumber,
	).Scan(&regID)
	require.NoError(t, err)

	var sessionID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status)
		VALUES ($1, $2, $3, now(), 'submitted') RETURNING id`,
		regID, studentID, examID,
	).Scan(&sessionID)
	require.NoError(t, err)
	return sessionID
}

// Compile-time check: *Repository implements AllocateCertificateNumber.
var _ interface {
	AllocateCertificateNumber(context.Context, uuid.UUID) (string, error)
} = (*Repository)(nil)
