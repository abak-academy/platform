package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

const validSchoolBulkCSV = "name,code,npsn,school_types,alamat\nSMA Satu,sma_satu,111,sma|smk,Jl Satu\nSMA Dua,sma_dua,222,sma,Jl Dua\n"

func TestPollJobsRoutesSchoolBulkToProcessor(t *testing.T) {
	ctx := context.Background()
	job := &model.Job{ID: "job-sb-1", Type: "school_bulk", Status: "running", CreatedBy: "u1", InputURL: strPtr("school-bulk/uuid-schools.csv")}

	repo := &fakeJobRepo{
		claimNextJobFn: func(ctx context.Context) (*model.Job, error) { return job, nil },
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validSchoolBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processSchoolFn: func(ctx context.Context, rows []service.SchoolBulkRow, onProgress func(int)) ([]service.SchoolBulkResultRow, int, error) {
			onProgress(100)
			return []service.SchoolBulkResultRow{
				{Name: "SMA Satu", Code: "sma_satu", Status: "success"},
				{Name: "SMA Dua", Code: "sma_dua", Status: "success"},
			}, 2, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.pollJobs(ctx)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.errMsg != nil && strings.Contains(*finish.errMsg, "unknown job type") {
		t.Fatalf("school_bulk was not routed to a handler: %s", *finish.errMsg)
	}
	if finish.status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", finish.status)
	}
	if finish.progress != 100 {
		t.Errorf("expected progress 100, got %d", finish.progress)
	}

	wantKey := "school-bulk/results/job-sb-1.csv"
	if len(store.putCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(store.putCalls))
	}
	if store.putCalls[0].key != wantKey {
		t.Errorf("expected result key %s, got %s", wantKey, store.putCalls[0].key)
	}
	if store.putCalls[0].bucket != "private-bucket" {
		t.Errorf("expected bucket private-bucket, got %s", store.putCalls[0].bucket)
	}
	if finish.resultURL == nil || *finish.resultURL != wantKey {
		t.Errorf("expected resultURL %s, got %v", wantKey, finish.resultURL)
	}
	if len(repo.progressCalls) == 0 {
		t.Error("expected UpdateJobProgress to be called via onProgress")
	}
	if len(repo.getUserCalls) != 0 {
		t.Errorf("school_bulk must not look up the owning user (schools are global), got %v", repo.getUserCalls)
	}
}

func TestRunSchoolBulkJobAllRowsFailedFinishesFailed(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-sb-2", Type: "school_bulk", CreatedBy: "u1", InputURL: strPtr("school-bulk/uuid-schools.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validSchoolBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processSchoolFn: func(ctx context.Context, rows []service.SchoolBulkRow, onProgress func(int)) ([]service.SchoolBulkResultRow, int, error) {
			return []service.SchoolBulkResultRow{
				{Name: "SMA Satu", Status: "failed", Error: "school code already taken"},
				{Name: "SMA Dua", Status: "failed", Error: "school code already taken"},
			}, 0, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runSchoolBulkJob(ctx, job)

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
	if finish.errMsg == nil || !strings.Contains(*finish.errMsg, "all 2 rows failed") {
		t.Errorf("expected an error message naming the row count, got %v", finish.errMsg)
	}
}

func TestRunSchoolBulkJobDownloadErrorFinishesFailedWithoutResultKey(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-sb-3", Type: "school_bulk", CreatedBy: "u1", Progress: 20, InputURL: strPtr("school-bulk/uuid-schools.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return nil, errors.New("connection refused")
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runSchoolBulkJob(ctx, job)

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

func TestRunSchoolBulkJobMissingInputURLFinishesFailed(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-sb-4", Type: "school_bulk", CreatedBy: "u1", Progress: 5}

	repo := &fakeJobRepo{}
	w := &Worker{jobRepo: repo, privateBucket: "private-bucket"}
	w.runSchoolBulkJob(ctx, job)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" || finish.resultURL != nil {
		t.Errorf("expected failed with no result key, got status=%s resultURL=%v", finish.status, finish.resultURL)
	}
}

func TestRunSchoolBulkJobParseErrorFinishesFailedWithoutResultKey(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-sb-5", Type: "school_bulk", CreatedBy: "u1", InputURL: strPtr("school-bulk/uuid-schools.csv")}

	repo := &fakeJobRepo{}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte("foo,bar\nx,y\n"), nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runSchoolBulkJob(ctx, job)

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
