package worker

import (
	"context"
	"fmt"
	"log/slog"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

// schoolBulkProcessor covers just the row-processing step. Unlike
// studentBulkProcessor there is no schoolBound parameter — schools are a global
// registry, so there is no per-school boundary to enforce.
type schoolBulkProcessor interface {
	ProcessSchoolBulkRows(ctx context.Context, rows []service.SchoolBulkRow, onProgress func(int)) ([]service.SchoolBulkResultRow, int, error)
}

// bulkProcessor is the union of row-processing steps the worker needs;
// *service.Service satisfies it, so worker.New's call sites are unchanged.
type bulkProcessor interface {
	studentBulkProcessor
	schoolBulkProcessor
}

// runSchoolBulkJob downloads the job's input CSV, creates each school through
// the unmodified CreateSchool path, uploads the per-row report, and finishes
// the job. Any failure before the report is durably uploaded finishes the job
// as failed with the job's progress left unchanged. No owning-user lookup:
// there is no school to bind rows to.
func (w *Worker) runSchoolBulkJob(ctx context.Context, job model.Job) {
	if job.InputURL == nil {
		w.failSchoolBulkJob(ctx, job, "missing input_url")
		return
	}
	data, err := w.objectStore.GetObjectBytes(ctx, w.privateBucket, *job.InputURL)
	if err != nil {
		w.failSchoolBulkJob(ctx, job, fmt.Sprintf("download input: %v", err))
		return
	}

	rows, err := service.ParseSchoolBulkCSV(data)
	if err != nil {
		w.failSchoolBulkJob(ctx, job, fmt.Sprintf("parse csv: %v", err))
		return
	}

	onProgress := func(pct int) {
		if err := w.jobRepo.UpdateJobProgress(ctx, job.ID, pct); err != nil {
			slog.Error("update job progress", "job_id", job.ID, "err", err)
		}
	}

	results, successCount, err := w.svc.ProcessSchoolBulkRows(ctx, rows, onProgress)
	if err != nil {
		w.failSchoolBulkJob(ctx, job, fmt.Sprintf("process rows: %v", err))
		return
	}

	reportCSV := service.BuildSchoolBulkResultCSV(results)
	resultKey := fmt.Sprintf("school-bulk/results/%s.csv", job.ID)
	if err := w.objectStore.PutObjectBytes(ctx, w.privateBucket, resultKey, reportCSV, "text/csv"); err != nil {
		w.failSchoolBulkJob(ctx, job, fmt.Sprintf("upload result: %v", err))
		return
	}

	status := "succeeded"
	var errMsg *string
	if successCount == 0 {
		status = "failed"
		msg := fmt.Sprintf("school_bulk job %s: all %d rows failed", job.ID, len(rows))
		errMsg = &msg
	}
	if err := w.jobRepo.FinishJob(ctx, job.ID, status, 100, &resultKey, errMsg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) failSchoolBulkJob(ctx context.Context, job model.Job, msg string) {
	if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}
