package service

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type PrivateUploadURL struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Key    string `json:"key"`
}

// GeneratePresignedPrivateUploadURL signs a PUT into the private bucket. The
// bucket is provisioned out of band and gets no public-read policy, unlike the
// avatar bucket in GeneratePresignedUploadURL.
func (s *Service) GeneratePresignedPrivateUploadURL(ctx context.Context, schoolID, filename, contentType string) (*PrivateUploadURL, error) {
	return s.presignPrivatePut(ctx, fmt.Sprintf("student-bulk/%s/%s-%s", schoolID, uuid.New().String(), filename))
}

// GeneratePresignedSchoolBulkUploadURL signs a PUT for a school-bulk CSV. The
// key carries no school segment (spec §D-5): schools are a global registry and
// the uploader is always super_admin, so there is no boundary to encode.
func (s *Service) GeneratePresignedSchoolBulkUploadURL(ctx context.Context, filename, contentType string) (*PrivateUploadURL, error) {
	return s.presignPrivatePut(ctx, fmt.Sprintf("school-bulk/%s-%s", uuid.New().String(), filename))
}

func (s *Service) presignPrivatePut(ctx context.Context, key string) (*PrivateUploadURL, error) {
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}

	// The private bucket is created at provisioning time, not per-request.
	presigned, err := s.presignStorage().PresignedPutObject(ctx, s.cfg.ObjectStoragePrivateBucketName, key, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	return &PrivateUploadURL{
		URL:    presigned.String(),
		Method: "PUT",
		Key:    key,
	}, nil
}

// fetchPrivateObject is a thin wrapper over the private-bucket GetObject wire
// call, kept separate so enqueueStudentBulkJobFromData is testable without
// MinIO.
func (s *Service) fetchPrivateObject(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.storage.GetObject(ctx, s.cfg.ObjectStoragePrivateBucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// EnqueueStudentBulkJob validates that fileKey lives under
// student-bulk/{schoolID}/ and exists in the private bucket, then delegates
// to enqueueStudentBulkJobFromData. For admin_school, schoolID is the JWT
// school. For super_admin it is the presign folder UUID (not a real school);
// row school is resolved later from the CSV.
func (s *Service) EnqueueStudentBulkJob(ctx context.Context, schoolID, createdBy, fileKey string) (string, error) {
	if !strings.HasPrefix(fileKey, fmt.Sprintf("student-bulk/%s/", schoolID)) {
		return "", ErrUploadNotFound
	}

	if _, err := s.storage.StatObject(ctx, s.cfg.ObjectStoragePrivateBucketName, fileKey, minio.StatObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return "", ErrUploadNotFound
		}
		return "", err
	}

	data, err := s.fetchPrivateObject(ctx, fileKey)
	if err != nil {
		return "", err
	}

	return s.enqueueStudentBulkJobFromData(ctx, schoolID, createdBy, fileKey, data)
}

// enqueueStudentBulkJobFromData validates the CSV and inserts the job row.
// schoolID is unused here — row-scoping is enforced by the caller passing
// claims.SchoolID, and a future job type might need it in this signature.
func (s *Service) enqueueStudentBulkJobFromData(ctx context.Context, schoolID, createdBy, fileKey string, data []byte) (string, error) {
	if _, err := ParseStudentBulkCSV(data); err != nil {
		return "", err
	}

	job := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: createdBy}
	if err := s.storeRepo.CreateJob(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

// EnqueueSchoolBulkJob validates that fileKey lives under the school-bulk
// prefix and exists in the private bucket, downloads it, then delegates to
// enqueueSchoolBulkJobFromData. There is no per-school prefix to check —
// schools are a global registry and the caller is always super_admin.
func (s *Service) EnqueueSchoolBulkJob(ctx context.Context, createdBy, fileKey string) (string, error) {
	if !strings.HasPrefix(fileKey, "school-bulk/") {
		return "", ErrUploadNotFound
	}

	if _, err := s.storage.StatObject(ctx, s.cfg.ObjectStoragePrivateBucketName, fileKey, minio.StatObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return "", ErrUploadNotFound
		}
		return "", err
	}

	data, err := s.fetchPrivateObject(ctx, fileKey)
	if err != nil {
		return "", err
	}

	return s.enqueueSchoolBulkJobFromData(ctx, createdBy, fileKey, data)
}

// enqueueSchoolBulkJobFromData validates the CSV eagerly so a bad header is a
// 4xx at enqueue time rather than a job that fails minutes later.
func (s *Service) enqueueSchoolBulkJobFromData(ctx context.Context, createdBy, fileKey string, data []byte) (string, error) {
	if _, err := ParseSchoolBulkCSV(data); err != nil {
		return "", err
	}

	job := &model.Job{Type: "school_bulk", InputURL: &fileKey, CreatedBy: createdBy}
	if err := s.storeRepo.CreateJob(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

type JobResponse struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Progress  int     `json:"progress"`
	ResultURL *string `json:"result_url"`
	Error     *string `json:"error"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func (s *Service) presignedPrivateGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presigned, err := s.presignStorage().PresignedGetObject(ctx, s.cfg.ObjectStoragePrivateBucketName, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return presigned.String(), nil
}

// GetJobStatus returns the job's status, substituting a freshly minted
// presigned GET for the stored result object key when present. Ownership
// mismatch is indistinguishable from non-existence.
func (s *Service) GetJobStatus(ctx context.Context, jobID, requesterID string) (*JobResponse, error) {
	job, err := s.storeRepo.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil || job.CreatedBy != requesterID {
		return nil, ErrJobNotFound
	}

	resp := &JobResponse{
		ID:        job.ID,
		Type:      job.Type,
		Status:    job.Status,
		Progress:  job.Progress,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.Format(time.RFC3339),
		UpdatedAt: job.UpdatedAt.Format(time.RFC3339),
	}

	if job.ResultURL != nil {
		if s.storage == nil {
			return nil, ErrStorageNotConfigured
		}
		presignedURL, err := s.presignedPrivateGetURL(ctx, *job.ResultURL, 15*time.Minute)
		if err != nil {
			return nil, err
		}
		resp.ResultURL = &presignedURL
	}

	return resp, nil
}
