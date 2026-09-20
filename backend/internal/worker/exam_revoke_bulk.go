package worker

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

// examRevokeBulkProcessor covers just the row-processing step so
// *service.Service (concrete, real-DB-backed) can be swapped for a fake at
// the worker-dispatch level.
type examRevokeBulkProcessor interface {
	RevokeExamAccessBulk(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamRevokeBulkRowResult, error)
}

// examRevokeBulkMaxRows mirrors examGrantBulkMaxRows — same 1,000-row cap.
const examRevokeBulkMaxRows = 1000

// parseExamRevokeBulkCSV reads the username-only bulk-revoke upload. Only a
// "username" header is required; rows are returned raw (untrimmed) — blank/
// duplicate/invalid resolution happens in RevokeExamAccessBulk.
func parseExamRevokeBulkCSV(data []byte) ([]string, error) {
	r := csv.NewReader(bytes.NewReader(data))

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, service.ErrMissingCSVHeader
		}
		return nil, service.ErrInvalidCSV
	}

	usernameIdx := -1
	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) == "username" {
			usernameIdx = i
		}
	}
	if usernameIdx == -1 {
		return nil, service.ErrMissingCSVHeader
	}

	var usernames []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, service.ErrInvalidCSV
		}
		if len(usernames)+1 > examRevokeBulkMaxRows {
			return nil, service.ErrRowLimitExceeded
		}
		usernames = append(usernames, record[usernameIdx])
	}

	return usernames, nil
}

// buildExamRevokeBulkResultCSV writes the per-row report as CSV bytes.
func buildExamRevokeBulkResultCSV(results []service.ExamRevokeBulkRowResult) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"username", "status", "message"})
	for _, r := range results {
		_ = w.Write([]string{r.Username, r.Status, r.Message})
	}
	w.Flush()
	return buf.Bytes()
}

// runExamRevokeBulkJob downloads the job's input CSV, revokes each row
// through RevokeExamAccessBulk, uploads the per-row report, and finishes the
// job. The exam ID is resolved from the input_url path
// (exam-revoke-bulk/{examID}/{uuid}-{filename}) since the job table has no
// dedicated exam_id column. Any failure before the report is durably
// uploaded finishes the job as failed with the job's progress left
// unchanged, mirroring runExamGrantBulkJob.
func (w *Worker) runExamRevokeBulkJob(ctx context.Context, job model.Job) {
	if job.InputURL == nil {
		w.failExamRevokeBulkJob(ctx, job, "missing input_url")
		return
	}

	inputParts := strings.SplitN(*job.InputURL, "/", 3)
	examID := inputParts[1]

	data, err := w.objectStore.GetObjectBytes(ctx, w.privateBucket, *job.InputURL)
	if err != nil {
		w.failExamRevokeBulkJob(ctx, job, fmt.Sprintf("download input: %v", err))
		return
	}

	usernames, err := parseExamRevokeBulkCSV(data)
	if err != nil {
		w.failExamRevokeBulkJob(ctx, job, fmt.Sprintf("parse csv: %v", err))
		return
	}

	results, err := w.svc.RevokeExamAccessBulk(ctx, job.CreatedBy, examID, usernames)
	if err != nil {
		w.failExamRevokeBulkJob(ctx, job, fmt.Sprintf("process rows: %v", err))
		return
	}

	reportCSV := buildExamRevokeBulkResultCSV(results)
	resultKey := fmt.Sprintf("exam-revoke-bulk/%s/results/%s.csv", examID, job.ID)
	if err := w.objectStore.PutObjectBytes(ctx, w.privateBucket, resultKey, reportCSV, "text/csv"); err != nil {
		w.failExamRevokeBulkJob(ctx, job, fmt.Sprintf("upload result: %v", err))
		return
	}

	successCount := 0
	for _, r := range results {
		if r.Status != "failed" {
			successCount++
		}
	}

	status := "succeeded"
	var errMsg *string
	if successCount == 0 {
		status = "failed"
		msg := fmt.Sprintf("exam_revoke_bulk job %s: all %d rows failed", job.ID, len(results))
		errMsg = &msg
	}
	if err := w.jobRepo.FinishJob(ctx, job.ID, status, 100, &resultKey, errMsg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) failExamRevokeBulkJob(ctx context.Context, job model.Job, msg string) {
	if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}
