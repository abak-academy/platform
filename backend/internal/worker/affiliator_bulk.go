package worker

import (
	"context"
	"fmt"
	"log/slog"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

// affiliatorBulkProcessor covers the row-processing step. The actor is the
// super admin who uploaded the file (the job's created_by), recorded in the audit log.
type affiliatorBulkProcessor interface {
	ProcessAffiliatorBulkRows(ctx context.Context, actorID string, rows []service.AffiliatorBulkRow, onProgress func(int)) ([]service.AffiliatorBulkResultRow, int, error)
}

// runAffiliatorBulkJob creates each affiliator through the single-create path,
// uploads the per-row report (the only place generated passwords appear) and
// finishes the job. The input CSV may hold plaintext passwords, so it is deleted
// whether the job succeeds or fails.
func (w *Worker) runAffiliatorBulkJob(ctx context.Context, job model.Job) {
	if job.InputURL != nil {
		defer func() {
			if err := w.objectStore.DeleteObject(context.WithoutCancel(ctx), w.privateBucket, *job.InputURL); err != nil {
				slog.Error("delete input", "job_id", job.ID, "err", err)
			}
		}()
	}
	if job.InputURL == nil {
		w.failAffiliatorBulkJob(ctx, job, "missing input_url")
		return
	}
	data, err := w.objectStore.GetObjectBytes(ctx, w.privateBucket, *job.InputURL)
	if err != nil {
		w.failAffiliatorBulkJob(ctx, job, fmt.Sprintf("download input: %v", err))
		return
	}
	rows, err := service.ParseAffiliatorBulkCSV(data)
	if err != nil {
		w.failAffiliatorBulkJob(ctx, job, fmt.Sprintf("parse csv: %v", err))
		return
	}

	onProgress := func(pct int) {
		if err := w.jobRepo.UpdateJobProgress(ctx, job.ID, pct); err != nil {
			slog.Error("update job progress", "job_id", job.ID, "err", err)
		}
	}
	results, successCount, err := w.svc.ProcessAffiliatorBulkRows(ctx, job.CreatedBy, rows, onProgress)
	if err != nil {
		w.failAffiliatorBulkJob(ctx, job, fmt.Sprintf("process rows: %v", err))
		return
	}

	resultKey := fmt.Sprintf("affiliator-bulk/results/%s.csv", job.ID)
	if err := w.objectStore.PutObjectBytes(ctx, w.privateBucket, resultKey, service.BuildAffiliatorBulkResultCSV(results), "text/csv"); err != nil {
		w.failAffiliatorBulkJob(ctx, job, fmt.Sprintf("upload result: %v", err))
		return
	}

	status := "succeeded"
	var errMsg *string
	if successCount == 0 {
		status = "failed"
		msg := fmt.Sprintf("affiliator_bulk job %s: all %d rows failed", job.ID, len(rows))
		errMsg = &msg
	}
	if err := w.jobRepo.FinishJob(ctx, job.ID, status, 100, &resultKey, errMsg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) failAffiliatorBulkJob(ctx context.Context, job model.Job, msg string) {
	if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}
