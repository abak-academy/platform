package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCard_UpdateRegistrationCard verifies FR-30's card_key column round-trips through
// a real Postgres pool (migration 0043_rename_card_key) — mirrors
// TestCertificate_UpdateSessionCertificate's certificate_key coverage.
func TestCard_UpdateRegistrationCard(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student Card")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	regID := insertCardRegistration(t, pool, student, examID)

	t.Run("persists card_key", func(t *testing.T) {
		key := "cards/" + regID.String() + ".pdf"
		if err := repo.UpdateRegistrationCard(ctx, regID, key); err != nil {
			t.Fatalf("UpdateRegistrationCard: %v", err)
		}

		detail, err := repo.GetExamRegistrationByID(ctx, regID, student)
		if err != nil {
			t.Fatalf("GetExamRegistrationByID: %v", err)
		}
		if detail.CardKey == nil || *detail.CardKey != key {
			t.Errorf("CardKey = %v, want %q", detail.CardKey, key)
		}
	})

	t.Run("second call overwrites card_key", func(t *testing.T) {
		key2 := "cards/" + regID.String() + "-v2.pdf"
		if err := repo.UpdateRegistrationCard(ctx, regID, key2); err != nil {
			t.Fatalf("UpdateRegistrationCard (2nd): %v", err)
		}

		detail, err := repo.GetExamRegistrationByID(ctx, regID, student)
		if err != nil {
			t.Fatalf("GetExamRegistrationByID: %v", err)
		}
		if detail.CardKey == nil || *detail.CardKey != key2 {
			t.Errorf("CardKey = %v, want %q", detail.CardKey, key2)
		}
	})
}

// insertCardRegistration seeds a bare exam_registration row for a student, returning
// the registration ID (no exam_session needed for the card_key round-trip).
func insertCardRegistration(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var regID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}
	return regID
}

// Compile-time check: *Repository implements UpdateRegistrationCard.
var _ interface {
	UpdateRegistrationCard(context.Context, uuid.UUID, string) error
} = (*Repository)(nil)
