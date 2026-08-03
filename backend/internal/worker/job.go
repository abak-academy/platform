package worker

import (
	"bytes"
	"context"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"

	"akademi-bimbel/internal/model"
)

// jobRepository defines the methods pollJobs needs from the repository to
// claim, progress, and finish rows in the generic job table.
type jobRepository interface {
	ClaimNextJob(ctx context.Context) (*model.Job, error)
	UpdateJobProgress(ctx context.Context, id string, progress int) error
	FinishJob(ctx context.Context, id, status string, progress int, resultURL, errMsg *string) error
	GetUserByID(ctx context.Context, id string) (*model.User, error)
}

// objectStore is the narrow whole-file-in-memory interface job handlers use
// to read/write the private bucket — a natural fit given the 1,000-row cap.
type objectStore interface {
	GetObjectBytes(ctx context.Context, bucket, key string) ([]byte, error)
	PutObjectBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

type minioObjectStore struct {
	client *minio.Client
}

// NewMinioObjectStore adapts a *minio.Client to the objectStore interface.
func NewMinioObjectStore(client *minio.Client) *minioObjectStore {
	return &minioObjectStore{client: client}
}

func (m *minioObjectStore) GetObjectBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (m *minioObjectStore) PutObjectBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := m.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

// pollJobs claims one queued job of any type and routes it to a handler by
// job.Type. An unrecognized type finishes the job as failed rather than
// crashing the poll loop.
func (w *Worker) pollJobs(ctx context.Context) {
	job, err := w.jobRepo.ClaimNextJob(ctx)
	if err != nil {
		slog.Error("claim next job", "err", err)
		return
	}
	if job == nil {
		return
	}

	switch job.Type {
	case "student_bulk":
		w.runStudentBulkJob(ctx, *job)
	case "school_bulk":
		w.runSchoolBulkJob(ctx, *job)
	default:
		msg := "unknown job type: " + job.Type
		if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
			slog.Error("finish job", "job_id", job.ID, "err", err)
		}
	}
}
