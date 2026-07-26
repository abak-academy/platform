package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// TestCreateExamRegistration_AssignsPerExamSequence verifies participant numbers
// are assigned as a per-exam sequence (FUP-1), independent across exams, and that
// the ON CONFLICT dedup does not consume a number.
func TestCreateExamRegistration_AssignsPerExamSequence(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	testID := insertGradingTest(t, pool)
	examA := insertGradingExam(t, pool, testID)
	examB := insertGradingExam(t, pool, testID)
	s1 := insertGradingUser(t, pool, "student", "S1")
	s2 := insertGradingUser(t, pool, "student", "S2")
	s3 := insertGradingUser(t, pool, "student", "S3")
	s4 := insertGradingUser(t, pool, "student", "S4")

	register := func(exam, student uuid.UUID) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback(ctx)
		if err := r.CreateExamRegistration(ctx, tx, model.ExamRegistration{
			StudentID: student, ExamID: exam, Token: GenerateExamToken(), Status: "registered",
		}); err != nil {
			t.Fatalf("create registration: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	number := func(exam, student uuid.UUID) *int {
		t.Helper()
		var n *int
		if err := pool.QueryRow(ctx,
			`SELECT participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
			exam, student,
		).Scan(&n); err != nil {
			t.Fatalf("read participant_number: %v", err)
		}
		return n
	}
	wantNum := func(label string, got *int, want int) {
		t.Helper()
		if got == nil {
			t.Fatalf("%s: participant_number is nil, want %d", label, want)
		}
		if *got != want {
			t.Errorf("%s: participant_number = %d, want %d", label, *got, want)
		}
	}

	register(examA, s1)
	register(examA, s2)
	register(examB, s3) // different exam restarts at 1
	register(examA, s3)
	register(examA, s1) // duplicate — ON CONFLICT DO NOTHING, must not consume a number
	register(examA, s4)

	wantNum("examA/s1", number(examA, s1), 1)
	wantNum("examA/s2", number(examA, s2), 2)
	wantNum("examA/s3", number(examA, s3), 3)
	wantNum("examA/s4", number(examA, s4), 4) // dup did not consume number 4
	wantNum("examB/s3", number(examB, s3), 1)
}
