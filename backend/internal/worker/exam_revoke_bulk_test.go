package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

const validExamRevokeBulkCSV = "username\nandi123\nbudi456\n"

func TestPollJobsRoutesExamRevokeBulkToProcessor(t *testing.T) {
	ctx := context.Background()
	job := &model.Job{ID: "job-er-1", Type: "exam_revoke_bulk", Status: "running", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{
		claimNextJobFn: func(ctx context.Context) (*model.Job, error) { return job, nil },
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validExamRevokeBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		revokeExamAccessBulkFn: func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error) {
			return []service.ExamRevokeBulkRowResult{
				{Username: "andi123", Status: "revoked"},
				{Username: "budi456", Status: "skipped", Message: "already revoked"},
			}, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.pollJobs(ctx)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.errMsg != nil && strings.Contains(*finish.errMsg, "unknown job type") {
		t.Fatalf("exam_revoke_bulk was not routed to a handler: %s", *finish.errMsg)
	}
	if finish.status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", finish.status)
	}
	if len(repo.getUserCalls) != 0 {
		t.Errorf("exam_revoke_bulk must not look up the owning user, got %v", repo.getUserCalls)
	}
}

func TestRunExamRevokeBulkJobSucceedsUploadsReportAndFinishesSucceeded(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-2", Type: "exam_revoke_bulk", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validExamRevokeBulkCSV), nil
		},
	}
	var sawActorID, sawExamID string
	var sawUsernames []string
	svc := &fakeStudentBulkProcessor{
		revokeExamAccessBulkFn: func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error) {
			sawActorID = actorID
			sawExamID = examID
			sawUsernames = usernames
			return []service.ExamRevokeBulkRowResult{
				{Username: "andi123", Status: "revoked"},
				{Username: "budi456", Status: "skipped", Message: "already revoked"},
			}, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if sawActorID != "actor-1" {
		t.Errorf("expected actorID actor-1, got %s", sawActorID)
	}
	if sawExamID != "exam-1" {
		t.Errorf("expected examID exam-1 resolved from input_url, got %s", sawExamID)
	}
	if len(sawUsernames) != 2 || sawUsernames[0] != "andi123" || sawUsernames[1] != "budi456" {
		t.Errorf("expected usernames [andi123 budi456], got %v", sawUsernames)
	}

	if len(store.getCalls) != 1 || store.getCalls[0] != "private-bucket/exam-revoke-bulk/exam-1/uuid-revokes.csv" {
		t.Fatalf("expected download from private-bucket/exam-revoke-bulk/exam-1/uuid-revokes.csv, got %v", store.getCalls)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(store.putCalls))
	}
	put := store.putCalls[0]
	if put.bucket != "private-bucket" {
		t.Errorf("expected upload bucket private-bucket, got %s", put.bucket)
	}
	wantKey := "exam-revoke-bulk/exam-1/results/job-er-2.csv"
	if put.key != wantKey {
		t.Errorf("expected upload key %s, got %s", wantKey, put.key)
	}
	if !strings.Contains(string(put.data), "username,status,message") {
		t.Errorf("expected result CSV header username,status,message, got %s", string(put.data))
	}
	if !strings.Contains(string(put.data), "andi123,revoked") {
		t.Errorf("expected result CSV to contain revoked row, got %s", string(put.data))
	}

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", finish.status)
	}
	if finish.progress != 100 {
		t.Errorf("expected progress 100, got %d", finish.progress)
	}
	if finish.resultURL == nil || *finish.resultURL != wantKey {
		t.Errorf("expected resultURL %s, got %v", wantKey, finish.resultURL)
	}
	if finish.errMsg != nil {
		t.Errorf("expected nil errMsg, got %v", *finish.errMsg)
	}
}

func TestRunExamRevokeBulkJobAllRowsFailedFinishesFailedButUploadsReport(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-3", Type: "exam_revoke_bulk", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validExamRevokeBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		revokeExamAccessBulkFn: func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error) {
			return []service.ExamRevokeBulkRowResult{
				{Username: "andi123", Status: "failed", Message: "not registered for this exam"},
				{Username: "budi456", Status: "failed", Message: "username not found"},
			}, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(store.putCalls) != 1 {
		t.Fatalf("expected the report to still be uploaded, got %d uploads", len(store.putCalls))
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed, got %s", finish.status)
	}
	if finish.resultURL == nil {
		t.Error("expected resultURL to still be set even though status is failed")
	}
	if finish.errMsg == nil || !strings.Contains(*finish.errMsg, "all 2 rows failed") {
		t.Errorf("expected an error message naming the row count, got %v", finish.errMsg)
	}
}

func TestRunExamRevokeBulkJobMissingInputURLFinishesFailed(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-4", Type: "exam_revoke_bulk", CreatedBy: "actor-1", Progress: 5}

	repo := &fakeJobRepo{}
	w := &Worker{jobRepo: repo, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" || finish.resultURL != nil {
		t.Errorf("expected failed with no result key, got status=%s resultURL=%v", finish.status, finish.resultURL)
	}
}

func TestRunExamRevokeBulkJobDownloadErrorFinishesFailedWithoutResultKey(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-5", Type: "exam_revoke_bulk", CreatedBy: "actor-1", Progress: 20, InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return nil, errors.New("connection refused")
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(store.putCalls) != 0 {
		t.Fatalf("expected no upload after a download failure, got %d", len(store.putCalls))
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed, got %s", finish.status)
	}
	if finish.resultURL != nil {
		t.Errorf("expected no result key, got %v", *finish.resultURL)
	}
	if finish.progress != 20 {
		t.Errorf("expected progress left unchanged at 20, got %d", finish.progress)
	}
	if finish.errMsg == nil || !strings.Contains(*finish.errMsg, "download input") {
		t.Errorf("expected a download-input error message, got %v", finish.errMsg)
	}
}

func TestRunExamRevokeBulkJobParseErrorFinishesFailedWithoutResultKey(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-6", Type: "exam_revoke_bulk", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte("foo\nbar\n"), nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(store.putCalls) != 0 {
		t.Fatalf("expected no upload after a parse failure, got %d", len(store.putCalls))
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" || finish.resultURL != nil {
		t.Errorf("expected failed with no result key, got status=%s resultURL=%v", finish.status, finish.resultURL)
	}
	if finish.errMsg == nil || !strings.Contains(*finish.errMsg, "parse csv") {
		t.Errorf("expected a parse-csv error message, got %v", finish.errMsg)
	}
}

func TestRunExamRevokeBulkJobUploadFailureFinishesFailedWithoutResultURL(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-7", Type: "exam_revoke_bulk", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validExamRevokeBulkCSV), nil
		},
		putObjectBytesFn: func(ctx context.Context, bucket, key string, data []byte, contentType string) error {
			return errors.New("upload failed")
		},
	}
	svc := &fakeStudentBulkProcessor{
		revokeExamAccessBulkFn: func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error) {
			return []service.ExamRevokeBulkRowResult{{Username: "andi123", Status: "revoked"}}, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed when upload fails, got %s", finish.status)
	}
	if finish.resultURL != nil {
		t.Error("expected nil resultURL when upload fails (no durable result)")
	}
}

func TestRunExamRevokeBulkJobProcessErrorFinishesFailedWithoutResultKey(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-er-8", Type: "exam_revoke_bulk", CreatedBy: "actor-1", InputURL: strPtr("exam-revoke-bulk/exam-1/uuid-revokes.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validExamRevokeBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		revokeExamAccessBulkFn: func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error) {
			return nil, errors.New("db down")
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runExamRevokeBulkJob(ctx, job)

	if len(store.putCalls) != 0 {
		t.Fatalf("expected no upload after a process failure, got %d", len(store.putCalls))
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" || finish.resultURL != nil {
		t.Errorf("expected failed with no result key, got status=%s resultURL=%v", finish.status, finish.resultURL)
	}
	if finish.errMsg == nil || !strings.Contains(*finish.errMsg, "process rows") {
		t.Errorf("expected a process-rows error message, got %v", finish.errMsg)
	}
}
