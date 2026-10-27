package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

const validAffiliatorBulkCSV = "name,email,password,school_code\nAndi,andi@example.com,,SMA1\nBela,bela@example.com,secret123,SMA2\n"

func TestPollJobsRoutesAffiliatorBulkToProcessor(t *testing.T) {
	ctx := context.Background()
	job := &model.Job{ID: "job-ab-1", Type: "affiliator_bulk", Status: "running", CreatedBy: "super-1", InputURL: strPtr("affiliator-bulk/uuid-a.csv")}

	repo := &fakeJobRepo{claimNextJobFn: func(ctx context.Context) (*model.Job, error) { return job, nil }}
	store := &fakeObjectStore{getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
		return []byte(validAffiliatorBulkCSV), nil
	}}
	var gotActor string
	var gotRows []service.AffiliatorBulkRow
	svc := &fakeStudentBulkProcessor{processAffiliatorFn: func(ctx context.Context, actorID string, rows []service.AffiliatorBulkRow, onProgress func(int)) ([]service.AffiliatorBulkResultRow, int, error) {
		gotActor, gotRows = actorID, rows
		onProgress(100)
		return []service.AffiliatorBulkResultRow{
			{Row: 2, Name: "Andi", Email: "andi@example.com", Status: "success", TempPassword: "Gen12345"},
			{Row: 3, Name: "Bela", Email: "bela@example.com", Status: "success"},
		}, 2, nil
	}}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.pollJobs(ctx)

	if gotActor != "super-1" || len(gotRows) != 2 || gotRows[0].Email != "andi@example.com" || gotRows[1].Password != "secret123" {
		t.Errorf("processor got actor=%q rows=%+v, want the uploader and both parsed rows", gotActor, gotRows)
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	wantKey := "affiliator-bulk/results/job-ab-1.csv"
	if finish.status != "succeeded" || finish.progress != 100 || finish.resultURL == nil || *finish.resultURL != wantKey {
		t.Errorf("finish = %+v, want succeeded at 100 with result %s", finish, wantKey)
	}
	if len(store.putCalls) != 1 || store.putCalls[0].key != wantKey || store.putCalls[0].bucket != "private-bucket" {
		t.Fatalf("uploads = %+v, want the result CSV at %s", store.putCalls, wantKey)
	}
	if !strings.Contains(string(store.putCalls[0].data), "Gen12345") {
		t.Error("the result CSV must carry the generated password")
	}
	if len(store.deleteCalls) != 1 || store.deleteCalls[0] != "private-bucket/affiliator-bulk/uuid-a.csv" {
		t.Errorf("deletions = %v, want the input CSV removed because it may hold passwords", store.deleteCalls)
	}
}

func TestRunAffiliatorBulkJobFailures(t *testing.T) {
	ctx := context.Background()
	input := strPtr("affiliator-bulk/uuid-a.csv")

	t.Run("all rows failed finishes failed but keeps the report", func(t *testing.T) {
		repo, store := &fakeJobRepo{}, &fakeObjectStore{getObjectBytesFn: func(context.Context, string, string) ([]byte, error) {
			return []byte(validAffiliatorBulkCSV), nil
		}}
		svc := &fakeStudentBulkProcessor{processAffiliatorFn: func(ctx context.Context, _ string, rows []service.AffiliatorBulkRow, _ func(int)) ([]service.AffiliatorBulkResultRow, int, error) {
			return []service.AffiliatorBulkResultRow{{Row: 2, Status: "failed", Error: "x"}, {Row: 3, Status: "failed", Error: "y"}}, 0, nil
		}}
		w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "b"}
		w.runAffiliatorBulkJob(ctx, model.Job{ID: "j1", CreatedBy: "u", InputURL: input})

		f := repo.finishCalls[0]
		if f.status != "failed" || f.errMsg == nil || !strings.Contains(*f.errMsg, "all 2 rows failed") || f.resultURL == nil {
			t.Errorf("finish = %+v, want failed with the report still attached", f)
		}
		if len(store.deleteCalls) != 1 {
			t.Errorf("deletions = %v, want the input removed", store.deleteCalls)
		}
	})

	t.Run("a download error fails without a result and still removes the input", func(t *testing.T) {
		repo, store := &fakeJobRepo{}, &fakeObjectStore{getObjectBytesFn: func(context.Context, string, string) ([]byte, error) {
			return nil, errors.New("boom")
		}}
		w := &Worker{jobRepo: repo, objectStore: store, svc: &fakeStudentBulkProcessor{}, privateBucket: "b"}
		w.runAffiliatorBulkJob(ctx, model.Job{ID: "j2", CreatedBy: "u", InputURL: input})

		f := repo.finishCalls[0]
		if f.status != "failed" || f.resultURL != nil || f.errMsg == nil || !strings.Contains(*f.errMsg, "download input") {
			t.Errorf("finish = %+v, want failed without a result key", f)
		}
		if len(store.deleteCalls) != 1 {
			t.Errorf("deletions = %v, want the input removed", store.deleteCalls)
		}
	})

	t.Run("a bad header fails the job", func(t *testing.T) {
		repo, store := &fakeJobRepo{}, &fakeObjectStore{getObjectBytesFn: func(context.Context, string, string) ([]byte, error) {
			return []byte("name,email\nA,a@example.com\n"), nil
		}}
		w := &Worker{jobRepo: repo, objectStore: store, svc: &fakeStudentBulkProcessor{}, privateBucket: "b"}
		w.runAffiliatorBulkJob(ctx, model.Job{ID: "j3", CreatedBy: "u", InputURL: input})

		f := repo.finishCalls[0]
		if f.status != "failed" || f.errMsg == nil || !strings.Contains(*f.errMsg, "parse csv") || len(store.putCalls) != 0 {
			t.Errorf("finish = %+v puts=%v, want a parse failure and no report", f, store.putCalls)
		}
	})

	t.Run("a missing input_url fails the job", func(t *testing.T) {
		repo := &fakeJobRepo{}
		w := &Worker{jobRepo: repo, objectStore: &fakeObjectStore{}, svc: &fakeStudentBulkProcessor{}, privateBucket: "b"}
		w.runAffiliatorBulkJob(ctx, model.Job{ID: "j4", CreatedBy: "u"})

		if f := repo.finishCalls[0]; f.status != "failed" || f.errMsg == nil || *f.errMsg != "missing input_url" {
			t.Errorf("finish = %+v, want missing input_url", f)
		}
	})
}
