package worker

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

type fakeJobRepo struct {
	claimNextJobFn      func(ctx context.Context) (*model.Job, error)
	updateJobProgressFn func(ctx context.Context, id string, progress int) error
	finishJobFn         func(ctx context.Context, id, status string, progress int, resultURL, errMsg *string) error
	getUserByIDFn       func(ctx context.Context, id string) (*model.User, error)

	progressCalls []int
	finishCalls   []finishCall
	getUserCalls  []string
}

type finishCall struct {
	id        string
	status    string
	progress  int
	resultURL *string
	errMsg    *string
}

func (f *fakeJobRepo) ClaimNextJob(ctx context.Context) (*model.Job, error) {
	return f.claimNextJobFn(ctx)
}

func (f *fakeJobRepo) UpdateJobProgress(ctx context.Context, id string, progress int) error {
	f.progressCalls = append(f.progressCalls, progress)
	if f.updateJobProgressFn != nil {
		return f.updateJobProgressFn(ctx, id, progress)
	}
	return nil
}

func (f *fakeJobRepo) FinishJob(ctx context.Context, id, status string, progress int, resultURL, errMsg *string) error {
	f.finishCalls = append(f.finishCalls, finishCall{id, status, progress, resultURL, errMsg})
	if f.finishJobFn != nil {
		return f.finishJobFn(ctx, id, status, progress, resultURL, errMsg)
	}
	return nil
}

func (f *fakeJobRepo) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	f.getUserCalls = append(f.getUserCalls, id)
	return f.getUserByIDFn(ctx, id)
}

type fakeObjectStore struct {
	getObjectBytesFn func(ctx context.Context, bucket, key string) ([]byte, error)
	putObjectBytesFn func(ctx context.Context, bucket, key string, data []byte, contentType string) error
	deleteObjectFn   func(ctx context.Context, bucket, key string) error

	getCalls    []string
	putCalls    []putCall
	deleteCalls []string
}

type putCall struct {
	bucket, key, contentType string
	data                     []byte
}

func (f *fakeObjectStore) GetObjectBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	f.getCalls = append(f.getCalls, bucket+"/"+key)
	return f.getObjectBytesFn(ctx, bucket, key)
}

func (f *fakeObjectStore) PutObjectBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	f.putCalls = append(f.putCalls, putCall{bucket, key, contentType, data})
	if f.putObjectBytesFn != nil {
		return f.putObjectBytesFn(ctx, bucket, key, data, contentType)
	}
	return nil
}

func (f *fakeObjectStore) DeleteObject(ctx context.Context, bucket, key string) error {
	f.deleteCalls = append(f.deleteCalls, bucket+"/"+key)
	if f.deleteObjectFn != nil {
		return f.deleteObjectFn(ctx, bucket, key)
	}
	return nil
}

type fakeStudentBulkProcessor struct {
	processFn             func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error)
	processSchoolFn       func(ctx context.Context, rows []service.SchoolBulkRow, onProgress func(int)) ([]service.SchoolBulkResultRow, int, error)
	grantExamAccessBulkFn func(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamGrantBulkRowResult, error)
}

func (f *fakeStudentBulkProcessor) GrantExamAccessBulk(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamGrantBulkRowResult, error) {
	return f.grantExamAccessBulkFn(ctx, actorID, examID, usernames)
}

func (f *fakeStudentBulkProcessor) ProcessStudentBulkRows(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
	return f.processFn(ctx, schoolBound, actorRole, rows, onProgress)
}

func (f *fakeStudentBulkProcessor) ProcessSchoolBulkRows(ctx context.Context, rows []service.SchoolBulkRow, onProgress func(int)) ([]service.SchoolBulkResultRow, int, error) {
	return f.processSchoolFn(ctx, rows, onProgress)
}

func schoolIDPtr(s string) *string { return &s }

func TestPollJobsNoQueuedJobIsNoOp(t *testing.T) {
	ctx := context.Background()
	repo := &fakeJobRepo{
		claimNextJobFn: func(ctx context.Context) (*model.Job, error) { return nil, nil },
	}
	w := &Worker{jobRepo: repo}
	w.pollJobs(ctx)

	if len(repo.finishCalls) != 0 {
		t.Fatalf("expected no FinishJob calls, got %d", len(repo.finishCalls))
	}
}

func TestPollJobsUnknownTypeFinishesFailed(t *testing.T) {
	ctx := context.Background()
	repo := &fakeJobRepo{
		claimNextJobFn: func(ctx context.Context) (*model.Job, error) {
			return &model.Job{ID: "job-1", Type: "question_bulk", Status: "running", Progress: 0, CreatedBy: "u1"}, nil
		},
	}
	w := &Worker{jobRepo: repo}
	w.pollJobs(ctx)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	call := repo.finishCalls[0]
	if call.status != "failed" {
		t.Errorf("expected status failed, got %s", call.status)
	}
	if call.errMsg == nil || *call.errMsg != "unknown job type: question_bulk" {
		t.Errorf("expected unknown job type error message, got %v", call.errMsg)
	}
	if call.resultURL != nil {
		t.Errorf("expected nil resultURL, got %v", *call.resultURL)
	}
}

func TestPollJobsDispatchesStudentBulkJob(t *testing.T) {
	ctx := context.Background()
	repo := &fakeJobRepo{
		claimNextJobFn: func(ctx context.Context) (*model.Job, error) {
			return &model.Job{ID: "job-1", Type: "student_bulk", Status: "running", Progress: 0, CreatedBy: "u1"}, nil
		},
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return nil, errors.New("boom")
		},
	}
	w := &Worker{jobRepo: repo}
	w.pollJobs(ctx)

	if len(repo.getUserCalls) != 1 || repo.getUserCalls[0] != "u1" {
		t.Fatalf("expected student_bulk dispatch to look up owning user, got %v", repo.getUserCalls)
	}
}

const validBulkCSV = "name,school,jenjang\nAli,SchoolA,sma\nBudi,SchoolA,sma\n"

func TestRunStudentBulkJobSucceedsUploadsReportAndFinishesSucceeded(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-1", Type: "student_bulk", CreatedBy: "u1", InputURL: strPtr("student-bulk/s1/upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleAdminSchool, SchoolID: schoolIDPtr("s1")}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
		deleteObjectFn: func(ctx context.Context, bucket, key string) error {
			if len(repo.finishCalls) != 1 {
				t.Fatalf("input deletion must follow FinishJob, got %d finish calls", len(repo.finishCalls))
			}
			return nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processFn: func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
			if schoolBound == nil || *schoolBound != "s1" {
				t.Errorf("expected schoolBound s1, got %v", schoolBound)
			}
			if actorRole != service.RoleAdminSchool {
				t.Errorf("expected actorRole admin_school, got %s", actorRole)
			}
			if len(rows) != 2 {
				t.Fatalf("expected 2 parsed rows, got %d", len(rows))
			}
			onProgress(100)
			return []service.StudentBulkResultRow{
				{Name: "Ali", Status: "success", Username: "ali1", TempPassword: "temp1"},
				{Name: "Budi", Status: "success", Username: "budi1", TempPassword: "temp2"},
			}, 2, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if len(store.getCalls) != 1 || store.getCalls[0] != "private-bucket/student-bulk/s1/upload.csv" {
		t.Fatalf("expected download from private-bucket/student-bulk/s1/upload.csv, got %v", store.getCalls)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(store.putCalls))
	}
	put := store.putCalls[0]
	if put.bucket != "private-bucket" {
		t.Errorf("expected upload bucket private-bucket, got %s", put.bucket)
	}
	wantKey := "student-bulk/s1/results/job-1.csv"
	if put.key != wantKey {
		t.Errorf("expected upload key %s, got %s", wantKey, put.key)
	}
	if len(repo.progressCalls) == 0 {
		t.Error("expected UpdateJobProgress to be called via onProgress")
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
	if len(store.deleteCalls) != 1 || store.deleteCalls[0] != "private-bucket/student-bulk/s1/upload.csv" {
		t.Fatalf("expected input CSV deletion after completion, got %v", store.deleteCalls)
	}
}

func TestRunStudentBulkJobCleanupIgnoresCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	job := model.Job{ID: "job-cancelled", Type: "student_bulk", CreatedBy: "u1", InputURL: strPtr("student-bulk/s1/upload.csv")}
	repo := &fakeJobRepo{
		getUserByIDFn: func(context.Context, string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleAdminSchool, SchoolID: schoolIDPtr("s1")}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(context.Context, string, string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
		deleteObjectFn: func(deleteCtx context.Context, _, _ string) error {
			if err := deleteCtx.Err(); err != nil {
				t.Errorf("cleanup context is cancelled: %v", err)
			}
			return nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processFn: func(context.Context, *string, string, []service.StudentBulkRow, func(int)) ([]service.StudentBulkResultRow, int, error) {
			cancel()
			return []service.StudentBulkResultRow{{Name: "Ali", Status: "success"}}, 1, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if len(store.deleteCalls) != 1 {
		t.Fatalf("expected one cleanup call, got %v", store.deleteCalls)
	}
}

func TestRunStudentBulkJob_SuperAdmin_NamespaceFolder_CoLocatesResult(t *testing.T) {
	ctx := context.Background()
	folder := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	job := model.Job{ID: "job-ns-1", Type: "student_bulk", CreatedBy: "sa1", InputURL: strPtr("student-bulk/" + folder + "/upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "sa1", Role: service.RoleSuperAdmin, SchoolID: nil}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processFn: func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
			if schoolBound != nil {
				t.Errorf("super_admin must not bind rows to a school, got %v", schoolBound)
			}
			if actorRole != service.RoleSuperAdmin {
				t.Errorf("expected actorRole super_admin, got %s", actorRole)
			}
			onProgress(100)
			return []service.StudentBulkResultRow{
				{Name: "Ali", Status: "success", Username: "ali1", TempPassword: "temp1"},
			}, 1, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	wantKey := "student-bulk/" + folder + "/results/job-ns-1.csv"
	if len(store.putCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(store.putCalls))
	}
	if store.putCalls[0].key != wantKey {
		t.Errorf("expected upload key %s, got %s", wantKey, store.putCalls[0].key)
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.resultURL == nil || *finish.resultURL != wantKey {
		t.Errorf("expected resultURL %s, got %v", wantKey, finish.resultURL)
	}
}

func TestRunStudentBulkJobZeroSuccessesFinishesFailedButKeepsResultURL(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-2", Type: "student_bulk", CreatedBy: "u1", InputURL: strPtr("student-bulk/s1/upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleAdminSchool, SchoolID: schoolIDPtr("s1")}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
	}
	svc := &fakeStudentBulkProcessor{
		processFn: func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
			return []service.StudentBulkResultRow{
				{Name: "Ali", Status: "failed", Error: "some error"},
				{Name: "Budi", Status: "failed", Error: "some error"},
			}, 0, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if len(store.putCalls) != 1 {
		t.Fatalf("expected report to still be uploaded, got %d uploads", len(store.putCalls))
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed on zero successes, got %s", finish.status)
	}
	if finish.progress != 100 {
		t.Errorf("expected progress 100, got %d", finish.progress)
	}
	if finish.resultURL == nil {
		t.Error("expected resultURL to still be set even though status is failed")
	}
	if finish.errMsg == nil || *finish.errMsg == "" {
		t.Error("expected a summary error message for zero successes")
	}
}

func TestRunStudentBulkJobFailsWhenUserLookupFails(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-3", Type: "student_bulk", CreatedBy: "u1", Progress: 0, InputURL: strPtr("upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return nil, errors.New("db down")
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			t.Fatal("expected no download attempt when user lookup fails")
			return nil, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed, got %s", finish.status)
	}
	if finish.progress != 0 {
		t.Errorf("expected unchanged progress 0, got %d", finish.progress)
	}
	if finish.resultURL != nil {
		t.Error("expected nil resultURL on early failure")
	}
	if finish.errMsg == nil {
		t.Error("expected an error message")
	}
}

func TestRunStudentBulkJobNilSchoolIDProceedsUnrestricted(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-6", Type: "student_bulk", CreatedBy: "u1", Progress: 0, InputURL: strPtr("student-bulk/s1/upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleSuperAdmin, SchoolID: nil}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
	}
	var sawBound *string
	svc := &fakeStudentBulkProcessor{
		processFn: func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
			sawBound = schoolBound
			if actorRole != service.RoleSuperAdmin {
				t.Errorf("expected actorRole super_admin, got %s", actorRole)
			}
			return []service.StudentBulkResultRow{
				{Name: "Ali", Status: "success", Username: "ali1", TempPassword: "temp1"},
			}, 1, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if sawBound != nil {
		t.Errorf("expected nil schoolBound for user with nil SchoolID, got %v", sawBound)
	}
	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", finish.status)
	}
}

func TestRunStudentBulkJobFailsWhenDownloadFails(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-4", Type: "student_bulk", CreatedBy: "u1", InputURL: strPtr("upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleAdminSchool, SchoolID: schoolIDPtr("s1")}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return nil, errors.New("not found")
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

	if len(repo.finishCalls) != 1 {
		t.Fatalf("expected 1 FinishJob call, got %d", len(repo.finishCalls))
	}
	finish := repo.finishCalls[0]
	if finish.status != "failed" {
		t.Errorf("expected status failed, got %s", finish.status)
	}
	if finish.resultURL != nil {
		t.Error("expected nil resultURL when download fails")
	}
}

func TestRunStudentBulkJobFailsWhenUploadFails(t *testing.T) {
	ctx := context.Background()
	job := model.Job{ID: "job-5", Type: "student_bulk", CreatedBy: "u1", InputURL: strPtr("student-bulk/s1/upload.csv")}

	repo := &fakeJobRepo{
		getUserByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{ID: "u1", Role: service.RoleAdminSchool, SchoolID: schoolIDPtr("s1")}, nil
		},
	}
	store := &fakeObjectStore{
		getObjectBytesFn: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return []byte(validBulkCSV), nil
		},
		putObjectBytesFn: func(ctx context.Context, bucket, key string, data []byte, contentType string) error {
			return errors.New("upload failed")
		},
	}
	svc := &fakeStudentBulkProcessor{
		processFn: func(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, onProgress func(int)) ([]service.StudentBulkResultRow, int, error) {
			return []service.StudentBulkResultRow{{Name: "Ali", Status: "success"}}, 1, nil
		},
	}

	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, job)

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

func strPtr(s string) *string { return &s }
