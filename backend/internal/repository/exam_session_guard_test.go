package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
)

func seedGuardRegistration(t *testing.T, pool *pgxpool.Pool) (model.ExamRegistration, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	studentID := insertGradingUser(t, pool, "student", "Guard Student")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	questionID := insertGradingEssayQuestion(t, pool, testID, "Q1", 10, 1)

	var reg model.ExamRegistration
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token)
		VALUES ($1, $2, $3)
		RETURNING id, student_id, exam_id, token, attempts_used, status`,
		studentID, examID, uuid.NewString(),
	).Scan(&reg.ID, &reg.StudentID, &reg.ExamID, &reg.Token, &reg.AttemptsUsed, &reg.Status)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}
	return reg, questionID
}

// A second CreateExamSessionTx for the same registration must fail atomically at the
// SQL layer (NOT EXISTS in_progress), not rely on the service's read-then-act
// check — two concurrent starts would otherwise both pass the service guard and create
// two live sessions. maxAttempts=nil is unlimited, so this rejection is the live-session
// lock, not the attempt ceiling.
func TestCreateExamSessionTx_SecondCallRejected(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	reg, _ := seedGuardRegistration(t, pool)

	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	if _, err := repo.CreateExamSessionTx(ctx, tx1, reg, nil); err != nil {
		t.Fatalf("first CreateExamSessionTx: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}

	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	defer tx2.Rollback(ctx)
	_, err = repo.CreateExamSessionTx(ctx, tx2, reg, nil)
	if !errors.Is(err, ErrNoAttemptsLeft) {
		t.Fatalf("second CreateExamSessionTx: want ErrNoAttemptsLeft, got %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE registration_id = $1`, reg.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("sessions for registration: want 1, got %d", count)
	}
}

// intptr is a small helper for building *int max_attempts arguments in these tests.
func intptr(n int) *int { return &n }

func markSessionSubmitted(t *testing.T, pool *pgxpool.Pool, sessionID, regID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`UPDATE exam_session SET status = 'submitted', submitted_at = now() WHERE id = $1`,
		sessionID,
	); err != nil {
		t.Fatalf("mark session submitted: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE exam_registration SET status = 'submitted' WHERE id = $1`,
		regID,
	); err != nil {
		t.Fatalf("mark registration submitted: %v", err)
	}
}

// TestCreateExamSessionTx_MaxAttemptsTwo asserts FB-26's multi-attempt ceiling: two
// starts succeed with attempt_number 1 then 2 after the first sitting is submitted,
// and a third returns ErrNoAttemptsLeft.
func TestCreateExamSessionTx_MaxAttemptsTwo(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	reg, _ := seedGuardRegistration(t, pool)
	ceiling := intptr(2)

	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	sess1, err := repo.CreateExamSessionTx(ctx, tx1, reg, ceiling)
	if err != nil {
		t.Fatalf("first CreateExamSessionTx: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}
	if sess1.AttemptNumber != 1 {
		t.Errorf("first attempt_number: want 1, got %d", sess1.AttemptNumber)
	}
	markSessionSubmitted(t, pool, sess1.ID, reg.ID)

	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	sess2, err := repo.CreateExamSessionTx(ctx, tx2, reg, ceiling)
	if err != nil {
		t.Fatalf("second CreateExamSessionTx: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit tx2: %v", err)
	}
	if sess2.AttemptNumber != 2 {
		t.Errorf("second attempt_number: want 2, got %d", sess2.AttemptNumber)
	}

	tx3, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx3: %v", err)
	}
	defer tx3.Rollback(ctx)
	_, err = repo.CreateExamSessionTx(ctx, tx3, reg, ceiling)
	if !errors.Is(err, ErrNoAttemptsLeft) {
		t.Fatalf("third CreateExamSessionTx: want ErrNoAttemptsLeft, got %v", err)
	}
}

// TestCreateExamSessionTx_MaxAttemptsZero asserts 0 means a single sitting.
func TestCreateExamSessionTx_MaxAttemptsZero(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	reg, _ := seedGuardRegistration(t, pool)

	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	sess, err := repo.CreateExamSessionTx(ctx, tx1, reg, intptr(0))
	if err != nil {
		t.Fatalf("first CreateExamSessionTx: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}
	markSessionSubmitted(t, pool, sess.ID, reg.ID)

	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	defer tx2.Rollback(ctx)
	_, err = repo.CreateExamSessionTx(ctx, tx2, reg, intptr(0))
	if !errors.Is(err, ErrNoAttemptsLeft) {
		t.Fatalf("second CreateExamSessionTx: want ErrNoAttemptsLeft, got %v", err)
	}
}

// TestCreateExamSessionTx_MaxAttemptsNilAllowsRetakeAfterSubmit asserts NULL is
// unlimited: a second start succeeds once the first sitting is submitted.
func TestCreateExamSessionTx_MaxAttemptsNilAllowsRetakeAfterSubmit(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	reg, _ := seedGuardRegistration(t, pool)

	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	sess1, err := repo.CreateExamSessionTx(ctx, tx1, reg, nil)
	if err != nil {
		t.Fatalf("first CreateExamSessionTx: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}
	markSessionSubmitted(t, pool, sess1.ID, reg.ID)

	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	sess2, err := repo.CreateExamSessionTx(ctx, tx2, reg, nil)
	if err != nil {
		t.Fatalf("second CreateExamSessionTx: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit tx2: %v", err)
	}
	if sess2.AttemptNumber != 2 {
		t.Errorf("second attempt_number: want 2, got %d", sess2.AttemptNumber)
	}
}

// TestCreateExamSessionTx_SecondStartWhileInProgressRejected covers both
// unlimited (nil) and single-sitting (0): a second create while the first
// session is still in_progress matches no row.
func TestCreateExamSessionTx_MaxAttemptsNilAndZero(t *testing.T) {
	cases := []struct {
		name        string
		maxAttempts *int
	}{
		{"nil", nil},
		{"zero", intptr(0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := newGradingTestPool(t)
			repo := New(pool)
			ctx := context.Background()

			reg, _ := seedGuardRegistration(t, pool)

			tx1, err := repo.BeginTx(ctx)
			if err != nil {
				t.Fatalf("begin tx1: %v", err)
			}
			if _, err := repo.CreateExamSessionTx(ctx, tx1, reg, tc.maxAttempts); err != nil {
				t.Fatalf("first CreateExamSessionTx: %v", err)
			}
			if err := tx1.Commit(ctx); err != nil {
				t.Fatalf("commit tx1: %v", err)
			}

			tx2, err := repo.BeginTx(ctx)
			if err != nil {
				t.Fatalf("begin tx2: %v", err)
			}
			defer tx2.Rollback(ctx)
			_, err = repo.CreateExamSessionTx(ctx, tx2, reg, tc.maxAttempts)
			if !errors.Is(err, ErrNoAttemptsLeft) {
				t.Fatalf("second CreateExamSessionTx: want ErrNoAttemptsLeft, got %v", err)
			}
		})
	}
}

// SaveAnswersTx racing a submit: once the session has left in_progress, a late
// autosave must not overwrite graded answer rows (is_correct/score/graded_at) —
// the status guard has to live inside the upsert statement, not only in the
// service's pre-check.
func TestSaveAnswersTx_AfterSubmit_NoOverwrite(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	reg, questionID := seedGuardRegistration(t, pool)

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	sess, err := repo.CreateExamSessionTx(ctx, tx, reg, nil)
	if err != nil {
		t.Fatalf("CreateExamSessionTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	gradedAt := time.Now()
	isCorrect := true
	answerText := "final answer"
	score := 10.0
	graded := []model.ExamSessionAnswer{{
		QuestionID: questionID,
		Answer:     &answerText,
		IsCorrect:  &isCorrect,
		Score:      &score,
		GradedAt:   &gradedAt,
	}}

	subTx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin submit tx: %v", err)
	}
	rows, err := repo.SubmitSessionTx(ctx, subTx, sess.ID, graded, score, false)
	if err != nil {
		t.Fatalf("SubmitSessionTx: %v", err)
	}
	if rows != 1 {
		t.Fatalf("SubmitSessionTx rows: want 1, got %d", rows)
	}
	if err := subTx.Commit(ctx); err != nil {
		t.Fatalf("commit submit: %v", err)
	}

	// Late autosave lands after the submit committed.
	stale := "stale autosave"
	late := []model.ExamSessionAnswer{{
		QuestionID: questionID,
		Answer:     &stale,
	}}
	if err := repo.SaveAnswersTx(ctx, sess.ID, late, nil); err != nil {
		t.Fatalf("SaveAnswersTx: %v", err)
	}

	var (
		gotAnswer  *string
		gotCorrect *bool
		gotScore   *float64
		gotGraded  *time.Time
	)
	err = pool.QueryRow(ctx,
		`SELECT answer, is_correct, score, graded_at FROM exam_session_answer
		WHERE session_id = $1 AND question_id = $2`,
		sess.ID, questionID,
	).Scan(&gotAnswer, &gotCorrect, &gotScore, &gotGraded)
	if err != nil {
		t.Fatalf("select answer row: %v", err)
	}

	if gotAnswer == nil || *gotAnswer != answerText {
		t.Errorf("answer: want %q preserved, got %v", answerText, gotAnswer)
	}
	if gotCorrect == nil || !*gotCorrect {
		t.Errorf("is_correct: want true preserved, got %v", gotCorrect)
	}
	if gotScore == nil || *gotScore != score {
		t.Errorf("score: want %v preserved, got %v", score, gotScore)
	}
	if gotGraded == nil {
		t.Errorf("graded_at: want preserved, got nil")
	}
}
