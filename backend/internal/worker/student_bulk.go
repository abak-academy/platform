package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

// studentBulkProcessor covers just the row-processing step so *service.Service
// (concrete, real-DB-backed) can be swapped for a fake at the worker-dispatch level.
type studentBulkProcessor interface {
	ProcessStudentBulkRows(ctx context.Context, schoolBound *string, actorRole string, rows []service.StudentBulkRow, requestPassword *string, onProgress func(int)) ([]service.StudentBulkResultRow, int, error)
}

// runStudentBulkJob downloads the job's input CSV, processes each row through
// the unmodified RegisterStudent path, uploads the per-row report, and
// finishes the job. Any failure before a result CSV is durably uploaded
// finishes the job as failed with the job's progress left unchanged.
func (w *Worker) runStudentBulkJob(ctx context.Context, job model.Job) {
	if job.InputURL != nil {
		// Unlike school CSVs, student CSVs may hold plaintext passwords, so cleanup follows successful and failed FinishJob calls.
		defer func() {
			if err := w.objectStore.DeleteObject(context.WithoutCancel(ctx), w.privateBucket, *job.InputURL); err != nil {
				slog.Error("delete input", "job_id", job.ID, "err", err)
			}
		}()
	}

	user, err := w.jobRepo.GetUserByID(ctx, job.CreatedBy)
	if err != nil || user == nil {
		w.failStudentBulkJob(ctx, job, fmt.Sprintf("resolve owning user: %v", err))
		return
	}

	// schoolBound restricts row school resolution: non-nil = all rows must
	// match this school; nil = no restriction (super_admin cross-school).
	var schoolBound *string
	if user.SchoolID != nil {
		schoolBound = user.SchoolID
	}

	if job.InputURL == nil {
		w.failStudentBulkJob(ctx, job, "missing input_url")
		return
	}
	data, err := w.objectStore.GetObjectBytes(ctx, w.privateBucket, *job.InputURL)
	if err != nil {
		w.failStudentBulkJob(ctx, job, fmt.Sprintf("download input: %v", err))
		return
	}

	rows, err := service.ParseStudentBulkCSV(data)
	if err != nil {
		w.failStudentBulkJob(ctx, job, fmt.Sprintf("parse csv: %v", err))
		return
	}

	onProgress := func(pct int) {
		if err := w.jobRepo.UpdateJobProgress(ctx, job.ID, pct); err != nil {
			slog.Error("update job progress", "job_id", job.ID, "err", err)
		}
	}

	// Consume the request-level temp password (one-shot). A missing or
	// expired key degrades to per-row system-generated passwords.
	var requestPassword *string
	if w.rdb != nil {
		if pass, err := w.rdb.GetDel(ctx, "bulkpass:"+job.ID).Result(); err == nil && pass != "" {
			requestPassword = &pass
		}
	}

	results, successCount, err := w.svc.ProcessStudentBulkRows(ctx, schoolBound, user.Role, rows, requestPassword, onProgress)
	if err != nil {
		w.failStudentBulkJob(ctx, job, fmt.Sprintf("process rows: %v", err))
		return
	}

	reportCSV := service.BuildStudentBulkResultCSV(results)

	// Derive the result key prefix from the input URL path to keep results
	// co-located with the source CSV (handles super_admin where the creator
	// has no SchoolID). The second segment is a folder UUID — JWT school for
	// admin_school, random namespace for super_admin — not used as row scope.
	// InputURL format: "student-bulk/{folder-uuid}/uuid-filename"
	inputParts := strings.SplitN(*job.InputURL, "/", 3)
	resultSchoolID := inputParts[1] // schoolID from the key prefix
	resultKey := fmt.Sprintf("student-bulk/%s/results/%s.csv", resultSchoolID, job.ID)
	if err := w.objectStore.PutObjectBytes(ctx, w.privateBucket, resultKey, reportCSV, "text/csv"); err != nil {
		w.failStudentBulkJob(ctx, job, fmt.Sprintf("upload result: %v", err))
		return
	}

	status := "succeeded"
	var errMsg *string
	if successCount == 0 {
		status = "failed"
		msg := fmt.Sprintf("student_bulk job %s: all %d rows failed", job.ID, len(rows))
		errMsg = &msg
	}
	if err := w.jobRepo.FinishJob(ctx, job.ID, status, 100, &resultKey, errMsg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) failStudentBulkJob(ctx context.Context, job model.Job, msg string) {
	if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}
