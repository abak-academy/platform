package repository

import (
	"context"
	"fmt"
	"sync"
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

	// Take the number from the sequence rather than pinning a literal: a literal is
	// both already-taken (uq_exam_exam_number) and never handed back (UPDATE does not
	// advance the sequence, so the DEFAULT reissues it later). Either way it is a
	// duplicate-key waiting for whichever test runs at the wrong moment.
	scheduledAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	var examNumber int
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE exam SET exam_number = nextval('exam_number_seq'), scheduled_at = $1
		WHERE id = $2 RETURNING exam_number`,
		scheduledAt, examID,
	).Scan(&examNumber))

	wantA := fmt.Sprintf("ABK/2026/%04d/%06d", examNumber, 5)
	wantB := fmt.Sprintf("ABK/2026/%04d/%06d", examNumber, 9)

	sessionA := insertCertNumSession(t, pool, studentA, examID, 5)
	sessionB := insertCertNumSession(t, pool, studentB, examID, 9)

	t.Run("first call composes ABK/YYYY/exam_number/participant_number", func(t *testing.T) {
		number, err := repo.AllocateCertificateNumber(ctx, sessionA)
		require.NoError(t, err)
		require.Equal(t, wantA, number)

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
		require.Equal(t, wantB, numberB)
	})

	t.Run("unknown session returns ErrNotFound", func(t *testing.T) {
		_, err := repo.AllocateCertificateNumber(ctx, uuid.New())
		require.ErrorIs(t, err, ErrNotFound)
	})
}

// TestAllocateCertificateNumber_RetakeSharesRegistrationNumber proves the 2026-08
// fix (docs/backlog/certificate-number-collision-23505.md): every session of one
// exam_registration — retakes included — resolves to the same certificate number.
// The first attempt composes ABK/...; a later attempt (new exam_session row, same
// registration) reuses the sibling's number instead of composing the identical
// string and deterministically violating idx_exam_session_certificate_number (23505).
func TestAllocateCertificateNumber_RetakeSharesRegistrationNumber(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student CertNum Retake")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)

	scheduledAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	var examNumber int
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE exam SET exam_number = nextval('exam_number_seq'), scheduled_at = $1
		WHERE id = $2 RETURNING exam_number`,
		scheduledAt, examID,
	).Scan(&examNumber))

	var regID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number) VALUES ($1, $2, $3, $4) RETURNING id`,
		student, examID, uuid.NewString(), 1,
	).Scan(&regID))

	firstID := insertCertNumRetakeSession(t, pool, regID, student, examID, 1)
	retakeID := insertCertNumRetakeSession(t, pool, regID, student, examID, 2)

	number, err := repo.AllocateCertificateNumber(ctx, firstID)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("ABK/2026/%04d/%06d", examNumber, 1), number)

	got, err := repo.AllocateCertificateNumber(ctx, retakeID)
	require.NoError(t, err, "retake must reuse the sibling's number, not collide with it")
	require.Equal(t, number, got, "retake session must share the registration's number")

	// The number was reused, not recomposed onto the sibling out of order: the
	// newest-numbered sibling wins the read-back, so allocate in reverse and the
	// answer is still the one shared value.
	reverse, err := repo.AllocateCertificateNumber(ctx, retakeID)
	require.NoError(t, err)
	require.Equal(t, number, reverse)
}

// TestAllocateCertificateNumber_ConcurrentRetakesConverge proves the race path:
// two sibling sessions allocating at the same moment both succeed with the same
// number — one composes, the loser's guarded UPDATE hits 23505 and falls back to
// sibling reuse — instead of the loser failing forever (the live error storm).
func TestAllocateCertificateNumber_ConcurrentRetakesConverge(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student CertNum Race")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)

	scheduledAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	var examNumber int
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE exam SET exam_number = nextval('exam_number_seq'), scheduled_at = $1
		WHERE id = $2 RETURNING exam_number`,
		scheduledAt, examID,
	).Scan(&examNumber))

	var regID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number) VALUES ($1, $2, $3, $4) RETURNING id`,
		student, examID, uuid.NewString(), 1,
	).Scan(&regID))

	a := insertCertNumRetakeSession(t, pool, regID, student, examID, 1)
	b := insertCertNumRetakeSession(t, pool, regID, student, examID, 2)

	numbers := make([]string, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, sid := range []uuid.UUID{a, b} {
		wg.Add(1)
		go func(i int, sid uuid.UUID) {
			defer wg.Done()
			numbers[i], errs[i] = repo.AllocateCertificateNumber(ctx, sid)
		}(i, sid)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, numbers[0], numbers[1], "concurrent siblings must converge on one number")
	require.Equal(t, fmt.Sprintf("ABK/2026/%04d/%06d", examNumber, 1), numbers[0])
}

// insertCertNumRetakeSession seeds a submitted exam_session on an existing
// registration with an explicit attempt_number, for retake-sharing tests.
func insertCertNumRetakeSession(t *testing.T, pool *pgxpool.Pool, regID, studentID, examID uuid.UUID, attemptNumber int) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var sessionID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, attempt_number)
		VALUES ($1, $2, $3, now(), 'submitted', $4) RETURNING id`,
		regID, studentID, examID, attemptNumber,
	).Scan(&sessionID)
	require.NoError(t, err)
	return sessionID
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

// TestUpdateExam_PersistsCertificateTemplateHTML proves UpdateExam actually
// writes certificate_template_html — a real re-read of the column, not a Go
// struct round-trip, since a round-trip through the same in-memory struct
// would pass even if the SET clause silently dropped the column (the bug
// this guards: certificate_template_html was read in every SELECT but
// missing from UpdateExam's SET list, so AdminUpdateExamCertificateDesign
// reported success while the template stayed NULL forever).
func TestUpdateExam_PersistsCertificateTemplateHTML(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)

	exam, err := repo.GetExamByID(ctx, examID)
	require.NoError(t, err)

	html := "<html><body>{{student_name}}</body></html>"
	exam.CertificateTemplateHTML = &html

	require.NoError(t, repo.UpdateExam(ctx, examID, exam))

	var persisted *string
	err = pool.QueryRow(ctx, `SELECT certificate_template_html FROM exam WHERE id = $1`, examID).Scan(&persisted)
	require.NoError(t, err)
	require.NotNil(t, persisted, "certificate_template_html was not persisted by UpdateExam")
	require.Equal(t, html, *persisted)
}
