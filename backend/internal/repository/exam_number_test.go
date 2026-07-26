package repository

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
)

// TestCreateExam_AssignsIncreasingExamNumber verifies a new exam gets a non-nil
// exam_number from exam_number_seq (FR-23), and that successive creates increase.
func TestCreateExam_AssignsIncreasingExamNumber(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	e1 := model.Exam{Title: "Exam One", ResultConfig: "hidden"}
	if err := r.CreateExam(ctx, &e1); err != nil {
		t.Fatalf("create exam 1: %v", err)
	}
	if e1.ExamNumber == nil {
		t.Fatal("exam_number is nil after create, want non-nil")
	}

	e2 := model.Exam{Title: "Exam Two", ResultConfig: "hidden"}
	if err := r.CreateExam(ctx, &e2); err != nil {
		t.Fatalf("create exam 2: %v", err)
	}
	if e2.ExamNumber == nil {
		t.Fatal("exam_number is nil after create, want non-nil")
	}

	if *e2.ExamNumber <= *e1.ExamNumber {
		t.Errorf("exam_number not increasing: e1=%d e2=%d", *e1.ExamNumber, *e2.ExamNumber)
	}
}

// TestExamNumber_RawInsert_AssignedByColumnDefault verifies an insert that names
// no exam_number still lands with one, via the column DEFAULT.
//
// This runs against a fully-migrated schema, so it says nothing about the 0039
// backfill itself — ordering, preservation and the sequence high-water mark are
// covered at the migration boundary by TestMigration0039_ExamNumberBackfill.
func TestExamNumber_RawInsert_AssignedByColumnDefault(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)

	var examNumber *int
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title) VALUES ('Raw Insert Exam') RETURNING exam_number`,
	).Scan(&examNumber)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	if examNumber == nil {
		t.Fatal("exam_number is nil for a raw insert, want the DEFAULT to assign one")
	}
}
