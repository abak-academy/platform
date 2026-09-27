package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// TestStartSession_RealDB_ExamWithoutQuestions exercises the ErrExamNotReady
// gate on the production Service.StartSession: a NEW start on an exam with no
// attached questions is rejected before any session row is created, so
// attempts_used stays 0. Once a test with a question is attached, the same
// registration starts normally.
func TestStartSession_RealDB_ExamWithoutQuestions(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Gate Student "+uniqueSuffix(), "gate_"+uniqueSuffix(),
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	exam := &model.Exam{Title: "Empty Exam " + uniqueSuffix(), ResultConfig: "hidden"}
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

	if _, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp"); !errors.Is(err, ErrExamNotReady) {
		t.Fatalf("StartSession without questions: want ErrExamNotReady, got %v", err)
	}

	var attemptsUsed int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT attempts_used FROM exam_registration WHERE id = $1`,
		regID,
	).Scan(&attemptsUsed); err != nil {
		t.Fatalf("select attempts_used: %v", err)
	}
	if attemptsUsed != 0 {
		t.Errorf("attempts_used after rejected start: want 0, got %d", attemptsUsed)
	}

	var sessionCount int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE registration_id = $1`,
		regID,
	).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Errorf("session rows after rejected start: want 0, got %d", sessionCount)
	}

	// Attach one test with one mcq question, then the start succeeds.
	test := &model.Test{Title: "Gate Test " + uniqueSuffix(), Subject: "math", Topic: "gate", DurationMinutes: 20}
	if err := repo.CreateTest(ctx, test); err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	var questionID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, correct_answer, point_correct, point_wrong)
		VALUES ('mcq', 'gate question', 'a', 1, 0)
		RETURNING id`,
	).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO question_option (question_id, key, text, is_correct, sort_order)
		VALUES ($1, 'a', 'A', true, 1), ($1, 'b', 'B', false, 2)`,
		questionID,
	); err != nil {
		t.Fatalf("insert question options: %v", err)
	}
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`,
		test.ID, questionID,
	); err != nil {
		t.Fatalf("insert test_question: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		exam.ID, test.ID,
	); err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	start, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession after attaching test: %v", err)
	}
	if start.SessionID == uuid.Nil {
		t.Error("expected non-nil session_id after questions attached")
	}
}
