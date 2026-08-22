package service

import (
	"context"
	"errors"
	"testing"

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

	if _, err := svc.SubmitSession(ctx, studentID.String(), first.SessionID.String(), ""); err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	second, err := svc.StartSession(ctx, studentID.String(), regID.String(), "fp")
	if err != nil {
		t.Fatalf("second StartSession: %v", err)
	}
	if second.SessionID == first.SessionID {
		t.Error("second start returned the same session id as the first")
	}

	if _, err := svc.SubmitSession(ctx, studentID.String(), second.SessionID.String(), ""); err != nil {
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
	if _, err := svc.SubmitSession(ctx, studentID.String(), first.SessionID.String(), ""); err != nil {
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
