package repository

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestQuestionBundleCreate_Transaction verifies atomic bundle + outbox + audit creation.
func TestQuestionBundleCreate_Transaction(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)
	actorID := insertGradingUser(t, pool, "admin_exam", "Question Bundle Admin")

	testID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO test (id, title, subject, topic, duration_minutes, created_at) VALUES ($1, $2, $3, $4, $5, now())`,
		testID, "Test Title", "Math", "Algebra", 60,
	); err != nil {
		t.Fatalf("insert test: %v", err)
	}

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	bundleID := uuid.New()
	bundle := &model.QuestionBundle{
		ID:        bundleID,
		TestID:    &testID,
		Variant:   "naskah",
		Status:    "queued",
		CreatedBy: actorID,
	}

	if err := repo.CreateQuestionBundleTx(ctx, tx, bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if bundle.Status != "queued" {
		t.Errorf("status: want 'queued', got %q", bundle.Status)
	}
	if err := repo.InsertOutboxEvent(ctx, tx, "question_bundle", bundleID, "QuestionBundleNeeded", map[string]interface{}{"bundle_id": bundleID.String()}); err != nil {
		t.Fatalf("insert outbox: %v", err)
	}

	actorStr := actorID.String()
	if err := repo.InsertAuditLogMeta(ctx, tx, &actorStr, "question_bundle", bundleID.String(), "question_bundle.request", map[string]any{"variant": "naskah", "test_id": testID.String()}); err != nil {
		t.Fatalf("insert audit: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	fetched, err := repo.GetQuestionBundleByID(ctx, bundleID)
	if err != nil {
		t.Fatalf("get bundle: %v", err)
	}
	if fetched.ID != bundleID {
		t.Errorf("bundle id: want %v, got %v", bundleID, fetched.ID)
	}
	if fetched.Status != "queued" {
		t.Errorf("status: want 'queued', got %q", fetched.Status)
	}
}

// TestQuestionBundleStateTransition verifies guarded status updates.
func TestQuestionBundleStateTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	bundleID := uuid.New()
	testID := uuid.New()
	actorID := insertGradingUser(t, pool, "admin_exam", "Question Bundle Admin")

	if _, err := pool.Exec(ctx,
		`INSERT INTO test (id, title, subject, topic, duration_minutes, created_at) VALUES ($1, $2, $3, $4, $5, now())`,
		testID, "Test", "Subject", "Topic", 60,
	); err != nil {
		t.Fatalf("insert test: %v", err)
	}

	bundle := &model.QuestionBundle{
		ID:        bundleID,
		TestID:    &testID,
		Variant:   "naskah",
		Status:    "queued",
		CreatedBy: actorID,
	}

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := repo.CreateQuestionBundleTx(ctx, tx, bundle); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	tx, err = repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := repo.UpdateQuestionBundleStatusTx(ctx, tx, bundleID, "queued", "processing"); err != nil {
		t.Fatalf("update to processing: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	fetched, err := repo.GetQuestionBundleByID(ctx, bundleID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Status != "processing" {
		t.Errorf("status: want 'processing', got %q", fetched.Status)
	}

	tx, err = repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	objectKey := "bundles/test-123-naskah.pdf"
	if err := repo.UpdateQuestionBundleReadyTx(ctx, tx, bundleID, objectKey); err != nil {
		t.Fatalf("update to ready: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	fetched, err = repo.GetQuestionBundleByID(ctx, bundleID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Status != "ready" {
		t.Errorf("status: want 'ready', got %q", fetched.Status)
	}
	if fetched.ObjectKey == nil || *fetched.ObjectKey != objectKey {
		t.Errorf("object_key: want %q, got %v", objectKey, fetched.ObjectKey)
	}
	if fetched.GeneratedAt == nil {
		t.Error("generated_at should be set")
	}
}

// TestQuestionBundleExactlyOneScope verifies migration constraint.
func TestQuestionBundleExactlyOneScope(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool := newGradingTestPool(t)

	testID := uuid.New()
	examID := uuid.New()
	actorID := insertGradingUser(t, pool, "admin_exam", "Question Bundle Admin")

	if _, err := pool.Exec(ctx,
		`INSERT INTO test (id, title, subject, topic, duration_minutes, created_at) VALUES ($1, $2, $3, $4, $5, now())`,
		testID, "Constraint Test", "Subject", "Topic", 60,
	); err != nil {
		t.Fatalf("insert test: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO exam (id, title, scheduled_at, duration_minutes, created_at) VALUES ($1, $2, now(), $3, now())`,
		examID, "Constraint Exam", 60,
	); err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO question_bundle (id, exam_id, test_id, variant, status, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now(), now())`,
		uuid.New(), examID, testID, "naskah", "queued", actorID,
	)
	if err == nil {
		t.Error("should reject bundle with both exam_id and test_id")
	} else if pgErr := (*pgconn.PgError)(nil); !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("expected exactly_one_scope check violation, got %T %v", err, err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO question_bundle (id, exam_id, test_id, variant, status, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now(), now())`,
		uuid.New(), nil, nil, "naskah", "queued", actorID,
	)
	if err == nil {
		t.Error("should reject bundle with neither exam_id nor test_id")
	}
}
