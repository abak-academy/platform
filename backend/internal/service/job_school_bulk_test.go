package service

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"akademi-bimbel/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// schoolBulkKeyRe pins §D-5: no school segment, uuid then the original filename.
var schoolBulkKeyRe = regexp.MustCompile(`^school-bulk/[0-9a-f-]{36}-`)

// newPresignOnlyService wires a real minio client with Region set explicitly so
// PUT presigning is a pure local computation and never dials out (same trick as
// avatar_proxy_test.go / admin_uploads_test.go).
func newPresignOnlyService(t *testing.T) *Service {
	t.Helper()
	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return NewWithStore(nil, nil, nil, nil, nil, nil, nil, nil, client,
		&config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)
}

func TestGeneratePresignedSchoolBulkUploadURL_KeyHasNoSchoolSegment(t *testing.T) {
	svc := newPresignOnlyService(t)

	resp, err := svc.GeneratePresignedSchoolBulkUploadURL(context.Background(), "schools.csv", "text/csv")
	if err != nil {
		t.Fatalf("GeneratePresignedSchoolBulkUploadURL: %v", err)
	}
	if !schoolBulkKeyRe.MatchString(resp.Key) {
		t.Errorf("key %q does not match %s", resp.Key, schoolBulkKeyRe)
	}
	if !regexp.MustCompile(`-schools\.csv$`).MatchString(resp.Key) {
		t.Errorf("key %q does not end with the uploaded filename", resp.Key)
	}
	if resp.Method != "PUT" {
		t.Errorf("Method: want PUT, got %s", resp.Method)
	}
	if resp.URL == "" {
		t.Error("want a non-empty presigned URL")
	}
}

func TestGeneratePresignedSchoolBulkUploadURL_NoStorage_ErrStorageNotConfigured(t *testing.T) {
	svc := &Service{}
	if _, err := svc.GeneratePresignedSchoolBulkUploadURL(context.Background(), "schools.csv", "text/csv"); !errors.Is(err, ErrStorageNotConfigured) {
		t.Errorf("want ErrStorageNotConfigured, got %v", err)
	}
}

// TestEnqueueSchoolBulkJob_ForeignPrefix_ErrUploadNotFound proves the prefix
// guard runs before any storage call: this Service has storage == nil and cfg
// == nil, so reaching StatObject would panic rather than return.
func TestEnqueueSchoolBulkJob_ForeignPrefix_ErrUploadNotFound(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	for _, key := range []string{
		"student-bulk/11111111-1111-1111-1111-111111111111/x.csv",
		"avatars/admin/x.csv",
		"schools/x.csv",
		"",
	} {
		if _, err := svc.EnqueueSchoolBulkJob(ctx, "u1", key); !errors.Is(err, ErrUploadNotFound) {
			t.Errorf("file_key %q: want ErrUploadNotFound, got %v", key, err)
		}
	}
}

func TestEnqueueSchoolBulkJobFromData_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	reg, err := svc.RegisterStudent(ctx, schoolID, "School Bulk Creator", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	createdBy := reg.ID
	fileKey := "school-bulk/" + uniqueSuffix() + "-schools.csv"

	t.Run("valid csv creates a queued school_bulk job", func(t *testing.T) {
		jobID, err := svc.enqueueSchoolBulkJobFromData(ctx, createdBy, fileKey, []byte("name,code\nSMA Satu,sma_satu\n"))
		if err != nil {
			t.Fatalf("enqueueSchoolBulkJobFromData: %v", err)
		}
		job, err := svc.storeRepo.GetJobByID(ctx, jobID)
		if err != nil {
			t.Fatalf("GetJobByID: %v", err)
		}
		if job == nil {
			t.Fatal("job not found after creation")
		}
		if job.Type != "school_bulk" {
			t.Errorf("Type: want school_bulk, got %s", job.Type)
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

	t.Run("csv missing required headers is rejected at enqueue, no job created", func(t *testing.T) {
		before := countSchoolBulkJobs(t, svc, createdBy)

		_, err := svc.enqueueSchoolBulkJobFromData(ctx, createdBy, fileKey, []byte("foo,bar\nx,y\n"))
		if !errors.Is(err, ErrMissingSchoolCSVHeader) {
			t.Fatalf("want ErrMissingSchoolCSVHeader, got %v", err)
		}
		if after := countSchoolBulkJobs(t, svc, createdBy); after != before {
			t.Errorf("a job row was created for an invalid header: %d -> %d", before, after)
		}
	})

	t.Run("over row-limit csv propagates ErrRowLimitExceeded", func(t *testing.T) {
		csv := "name,code\n"
		for i := 0; i < maxBulkRows+1; i++ {
			csv += "SMA X,code_x\n"
		}
		if _, err := svc.enqueueSchoolBulkJobFromData(ctx, createdBy, fileKey, []byte(csv)); !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})
}

func countSchoolBulkJobs(t *testing.T, svc *Service, createdBy string) int {
	t.Helper()
	var n int
	if err := svc.storeRepo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM job WHERE type = 'school_bulk' AND created_by = $1`, createdBy).Scan(&n); err != nil {
		t.Fatalf("count school_bulk jobs: %v", err)
	}
	return n
}
