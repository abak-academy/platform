package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// TestStartSession_RealDB_MaxAttempts exercises the production Service.StartSession
// end to end against a real Postgres instance. storeRepo is a concrete
// *repository.Repository (see realdb_test.go), so the atomic CreateExamSessionTx path
// cannot be exercised through a fake — this is the only place FB-26's full flow (both
// guards + the DB predicate) is proven together. max_attempts=2 allows two starts
// after the first sitting is submitted, and the third exhausts to ErrAlreadyAttempted.
func TestStartSession_RealDB_MaxAttempts(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Attempts Student "+uniqueSuffix(), "att_"+uniqueSuffix(),
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	maxAttempts := 2
	exam := &model.Exam{Title: "Retake Exam " + uniqueSuffix(), MaxAttempts: &maxAttempts, ResultConfig: "hidden"}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	var regID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, exam.ID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}

	first, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("first StartSession: %v", err)
	}
	if first.SessionID == uuid.Nil {
		t.Error("expected non-nil session_id on first start")
	}

	resumed, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("resume StartSession: %v", err)
	}
	if resumed.SessionID != first.SessionID {
		t.Errorf("resume: want same session_id %s, got %s", first.SessionID, resumed.SessionID)
	}

	if _, err := svc.SubmitSession(ctx, studentID.String(), first.SessionID.String()); err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	second, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("second StartSession: %v", err)
	}
	if second.SessionID == first.SessionID {
		t.Error("second start returned the same session id as the first")
	}

	if _, err := svc.SubmitSession(ctx, studentID.String(), second.SessionID.String()); err != nil {
		t.Fatalf("SubmitSession second: %v", err)
	}

	_, err = svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if !errors.Is(err, ErrAlreadyAttempted) {
		t.Errorf("third StartSession: want ErrAlreadyAttempted, got %v", err)
	}
}

func TestStartSession_RealDB_NilMaxAttemptsUnlimited(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Unlimited Student "+uniqueSuffix(), "unl_"+uniqueSuffix(),
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	exam := &model.Exam{Title: "Unlimited Exam " + uniqueSuffix(), ResultConfig: "hidden"}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	var regID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, exam.ID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}

	first, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("first StartSession: %v", err)
	}
	if _, err := svc.SubmitSession(ctx, studentID.String(), first.SessionID.String()); err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}
	second, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("second StartSession: want unlimited retake, got %v", err)
	}
	if second.SessionID == first.SessionID {
		t.Error("retake returned the first session id")
	}
}

func TestSubmitSession_RealDB_GradesAnswerCommittedBeforeSessionLock(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	var studentID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Locked Snapshot Student "+suffix, "snap_"+suffix,
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	var testID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'math', 'atomic', 60) RETURNING id`,
		"Locked Snapshot Test "+suffix,
	).Scan(&testID); err != nil {
		t.Fatalf("insert test: %v", err)
	}
	var questionID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('essay', $1, 5, 0) RETURNING id`,
		"Locked snapshot essay "+suffix,
	).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`,
		testID, questionID,
	); err != nil {
		t.Fatalf("insert test_question: %v", err)
	}

	var examID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, result_config) VALUES ($1, 'hidden') RETURNING id`,
		"Locked Snapshot Exam "+suffix,
	).Scan(&examID); err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		examID, testID,
	); err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}

	var regID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert registration: %v", err)
	}
	var sessionID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status)
		VALUES ($1, $2, $3, now(), 'in_progress') RETURNING id`,
		regID, studentID, examID,
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	oldAnswer := "old"
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_session_answer (session_id, question_id, answer, saved_at)
		VALUES ($1, $2, $3, now())`,
		sessionID, questionID, oldAnswer,
	); err != nil {
		t.Fatalf("insert answer: %v", err)
	}

	lockTx, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer lockTx.Rollback(ctx)
	if _, err := lockTx.Exec(ctx, `SELECT id FROM exam_session WHERE id = $1 FOR UPDATE`, sessionID); err != nil {
		t.Fatalf("lock session: %v", err)
	}
	lateAnswer := "late committed before finalizer lock"
	if _, err := lockTx.Exec(ctx,
		`UPDATE exam_session_answer SET answer = $1, saved_at = now() WHERE session_id = $2 AND question_id = $3`,
		lateAnswer, sessionID, questionID,
	); err != nil {
		t.Fatalf("update locked answer: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := svc.SubmitSession(ctx, studentID.String(), sessionID.String())
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	var got *string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT answer FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
		sessionID, questionID,
	).Scan(&got); err != nil {
		t.Fatalf("query final answer: %v", err)
	}
	if got == nil || *got != lateAnswer {
		t.Fatalf("final graded answer = %v, want %q", got, lateAnswer)
	}
}
