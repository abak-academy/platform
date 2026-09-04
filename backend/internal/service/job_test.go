package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestEnqueueStudentBulkJobFromData_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	reg, err := svc.RegisterStudent(ctx, schoolID, "Job Creator", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	createdBy := reg.ID
	fileKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-students.csv"

	t.Run("valid csv creates a queued job pointing at the file key", func(t *testing.T) {
		csv := []byte("name,school,jenjang\nBudi,SchoolName,sma\n")
		jobID, err := svc.enqueueStudentBulkJobFromData(ctx, schoolID, createdBy, fileKey, "", csv)
		if err != nil {
			t.Fatalf("enqueueStudentBulkJobFromData: %v", err)
		}
		if jobID == "" {
			t.Fatal("want non-empty job id")
		}

		job, err := svc.storeRepo.GetJobByID(ctx, jobID)
		if err != nil {
			t.Fatalf("GetJobByID: %v", err)
		}
		if job == nil {
			t.Fatal("job not found after creation")
		}
		if job.Type != "student_bulk" {
			t.Errorf("Type: want student_bulk, got %s", job.Type)
		}
		if job.Status != "queued" {
			t.Errorf("Status: want queued, got %s", job.Status)
		}
		if job.InputURL == nil || *job.InputURL != fileKey {
			t.Errorf("InputURL: want %s, got %v", fileKey, job.InputURL)
		}
		if job.CreatedBy != createdBy {
			t.Errorf("CreatedBy: want %s, got %s", createdBy, job.CreatedBy)
		}
	})

	t.Run("csv missing required headers propagates ErrMissingCSVHeader, no job created", func(t *testing.T) {
		csv := []byte("foo,bar\nx,y\n")
		_, err := svc.enqueueStudentBulkJobFromData(ctx, schoolID, createdBy, fileKey, "", csv)
		if !errors.Is(err, ErrMissingCSVHeader) {
			t.Errorf("want ErrMissingCSVHeader, got %v", err)
		}
	})

	t.Run("over row-limit csv propagates ErrRowLimitExceeded", func(t *testing.T) {
		csv := "name,school,jenjang\n"
		for i := 0; i < maxBulkRows+1; i++ {
			csv += "Student,School,sma\n"
		}
		_, err := svc.enqueueStudentBulkJobFromData(ctx, schoolID, createdBy, fileKey, "", []byte(csv))
		if !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})
}

func TestEnqueueStudentBulkJob_RejectsFileKeyOutsideCallersSchool_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	schoolA := createTestSchool(t, svc)
	schoolB := createTestSchool(t, svc)
	reg, err := svc.RegisterStudent(ctx, schoolA, "Job Creator", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	otherSchoolsFileKey := "student-bulk/" + schoolB + "/" + uniqueSuffix() + "-students.csv"
	_, err = svc.EnqueueStudentBulkJob(ctx, schoolA, reg.ID, otherSchoolsFileKey, "")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Errorf("want ErrUploadNotFound for a file_key outside the caller's own school prefix, got %v", err)
	}
}

// TestEnqueueStudentBulkJob_RequestPasswordKey_Integration pins the Redis
// one-shot transport: a request-level temp password is stored under
// bulkpass:{jobID} for the worker to consume, and absent when not supplied.
func TestEnqueueStudentBulkJob_RequestPasswordKey_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	if realDBRedis == nil {
		t.Fatal("newRealDBService must provide miniredis")
	}
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	reg, err := svc.RegisterStudent(ctx, schoolID, "Bulk Pass Enqueuer", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	fileKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-students.csv"
	csv := []byte("name,school,jenjang\nBudi,SchoolName,sma\n")

	t.Run("stores the request-level password under bulkpass:{jobID}", func(t *testing.T) {
		jobID, err := svc.enqueueStudentBulkJobFromData(ctx, schoolID, reg.ID, fileKey, "welcomePass123", csv)
		if err != nil {
			t.Fatalf("enqueueStudentBulkJobFromData: %v", err)
		}
		val, err := realDBRedis.Get("bulkpass:" + jobID)
		if err != nil {
			t.Fatalf("bulkpass key missing: %v", err)
		}
		if val != "welcomePass123" {
			t.Errorf("bulkpass value: want welcomePass123, got %s", val)
		}
	})

	t.Run("no request-level password stores no key", func(t *testing.T) {
		jobID, err := svc.enqueueStudentBulkJobFromData(ctx, schoolID, reg.ID, fileKey, "", csv)
		if err != nil {
			t.Fatalf("enqueueStudentBulkJobFromData: %v", err)
		}
		if realDBRedis.Exists("bulkpass:" + jobID) {
			t.Error("bulkpass key must be absent when no temp password supplied")
		}
	})
}

func TestGetJobStatus_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	owner, err := svc.RegisterStudent(ctx, schoolID, "Job Owner", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	other, err := svc.RegisterStudent(ctx, schoolID, "Not Owner", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	fileKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-students.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: owner.ID}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	t.Run("owner can read status, no presign attempted when result_url is nil", func(t *testing.T) {
		resp, err := svc.GetJobStatus(ctx, job.ID, owner.ID)
		if err != nil {
			t.Fatalf("GetJobStatus: %v", err)
		}
		if resp.ID != job.ID {
			t.Errorf("ID: want %s, got %s", job.ID, resp.ID)
		}
		if resp.Type != "student_bulk" {
			t.Errorf("Type: want student_bulk, got %s", resp.Type)
		}
		if resp.Status != "queued" {
			t.Errorf("Status: want queued, got %s", resp.Status)
		}
		if resp.ResultURL != nil {
			t.Errorf("ResultURL: want nil, got %v", *resp.ResultURL)
		}
		if resp.CreatedAt == "" || resp.UpdatedAt == "" {
			t.Error("want non-empty CreatedAt/UpdatedAt")
		}
	})

	t.Run("different requester gets ErrJobNotFound", func(t *testing.T) {
		_, err := svc.GetJobStatus(ctx, job.ID, other.ID)
		if !errors.Is(err, ErrJobNotFound) {
			t.Errorf("want ErrJobNotFound, got %v", err)
		}
	})

	t.Run("nonexistent job id gets ErrJobNotFound", func(t *testing.T) {
		_, err := svc.GetJobStatus(ctx, "00000000-0000-0000-0000-000000000000", owner.ID)
		if !errors.Is(err, ErrJobNotFound) {
			t.Errorf("want ErrJobNotFound, got %v", err)
		}
	})
}

// TestGetJobStatus_SucceededWithResultURL_Integration drives a job to
// succeeded with a result_url key set via a direct FinishJob call, then reads
// it back through GetJobStatus. newRealDBService wires storage=nil and
// cfg=nil (see realdb_test.go), so this can't observe a real presigned URL
// (NFR-3 acknowledges MinIO wire calls stay untested). GetJobStatus guards
// `s.storage == nil` before minting a presigned result_url (matching
// GeneratePresignedUploadURL/GeneratePresignedPrivateUploadURL's existing
// pattern), so with storage unconfigured this asserts a graceful error
// rather than a nil-pointer panic.
func TestGetJobStatus_SucceededWithResultURL_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	owner, err := svc.RegisterStudent(ctx, schoolID, "Result Owner", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	fileKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-students.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: owner.ID}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	resultKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-report.csv"
	if err := repo.FinishJob(ctx, job.ID, "succeeded", 100, &resultKey, nil); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}

	_, err = svc.GetJobStatus(ctx, job.ID, owner.ID)
	if err == nil || err.Error() != "storage not configured" {
		t.Fatalf(`expected GetJobStatus to return a graceful "storage not configured" error with unconfigured storage/cfg, got %v`, err)
	}
}

// TestClaimNextJob_ConcurrentClaims_Integration proves ClaimNextJob's
// single-statement UPDATE...WHERE id=(SELECT...FOR UPDATE SKIP LOCKED) claim
// is safe under concurrent pollers: only one of many simultaneous callers can
// ever claim a given queued row. Run with -race.
func TestClaimNextJob_ConcurrentClaims_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	// This package's shared DB fixture is never reset between tests, and
	// earlier tests in this file leave jobs behind in 'queued' status. Drain
	// them first so the only queued row the race below can claim is the one
	// seeded here.
	for {
		leftover, err := repo.ClaimNextJob(ctx)
		if err != nil {
			t.Fatalf("draining pre-existing queued jobs: %v", err)
		}
		if leftover == nil {
			break
		}
	}

	schoolID := createTestSchool(t, svc)
	owner, err := svc.RegisterStudent(ctx, schoolID, "Claimer", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	fileKey := "student-bulk/" + schoolID + "/" + uniqueSuffix() + "-students.csv"
	seeded := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: owner.ID}
	if err := repo.CreateJob(ctx, seeded); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	const goroutines = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		claimed   int
		nilClaims int
		firstErr  error
	)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			job, err := repo.ClaimNextJob(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if job == nil {
				nilClaims++
				return
			}
			claimed++
		}()
	}
	wg.Wait()

	if firstErr != nil {
		t.Fatalf("ClaimNextJob returned an error: %v", firstErr)
	}
	if claimed != 1 {
		t.Errorf("want exactly 1 goroutine to claim the seeded job, got %d", claimed)
	}
	if nilClaims != goroutines-1 {
		t.Errorf("want %d goroutines to get (nil, nil), got %d", goroutines-1, nilClaims)
	}
}
