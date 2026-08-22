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

// examGrantBulkProcessor covers just the row-processing step so
// *service.Service (concrete, real-DB-backed) can be swapped for a fake at
// the worker-dispatch level.
type examGrantBulkProcessor interface {
	GrantExamAccessBulk(ctx context.Context, actorID, examID string, usernames []string) ([]service.ExamGrantBulkRowResult, error)
}

// examGrantBulkMaxRows mirrors maxBulkRows in the service package (student/
// school bulk imports) — same 1,000-row cap for the exam-grant-bulk CSV.
const examGrantBulkMaxRows = 1000

// parseExamGrantBulkCSV reads the username-only bulk-grant upload. Only a
// "username" header is required; rows are returned raw (untrimmed) — blank/
// duplicate/invalid resolution happens in GrantExamAccessBulk.
func parseExamGrantBulkCSV(data []byte) ([]string, error) {
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
		if len(usernames)+1 > examGrantBulkMaxRows {
			return nil, service.ErrRowLimitExceeded
		}
		usernames = append(usernames, record[usernameIdx])
	}

	return usernames, nil
}

// buildExamGrantBulkResultCSV writes the per-row report as CSV bytes.
func buildExamGrantBulkResultCSV(results []service.ExamGrantBulkRowResult) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"username", "status", "message"})
	for _, r := range results {
		_ = w.Write([]string{r.Username, r.Status, r.Message})
	}
	w.Flush()
	return buf.Bytes()
}

// runExamGrantBulkJob downloads the job's input CSV, grants exam access to
// each row through GrantExamAccessBulk, uploads the per-row report, and
// finishes the job. The exam ID is resolved from the input_url path
// (exam-grant-bulk/{examID}/{uuid}-{filename}) since the job table has no
// dedicated exam_id column. Any failure before the report is durably
// uploaded finishes the job as failed with the job's progress left
// unchanged, mirroring runStudentBulkJob/runSchoolBulkJob.
func (w *Worker) runExamGrantBulkJob(ctx context.Context, job model.Job) {
	if job.InputURL == nil {
		w.failExamGrantBulkJob(ctx, job, "missing input_url")
		return
	}

	inputParts := strings.SplitN(*job.InputURL, "/", 3)
	examID := inputParts[1]

	data, err := w.objectStore.GetObjectBytes(ctx, w.privateBucket, *job.InputURL)
	if err != nil {
		w.failExamGrantBulkJob(ctx, job, fmt.Sprintf("download input: %v", err))
		return
	}

	usernames, err := parseExamGrantBulkCSV(data)
	if err != nil {
		w.failExamGrantBulkJob(ctx, job, fmt.Sprintf("parse csv: %v", err))
		return
	}

	results, err := w.svc.GrantExamAccessBulk(ctx, job.CreatedBy, examID, usernames)
	if err != nil {
		w.failExamGrantBulkJob(ctx, job, fmt.Sprintf("process rows: %v", err))
		return
	}

	reportCSV := buildExamGrantBulkResultCSV(results)
	resultKey := fmt.Sprintf("exam-grant-bulk/%s/results/%s.csv", examID, job.ID)
	if err := w.objectStore.PutObjectBytes(ctx, w.privateBucket, resultKey, reportCSV, "text/csv"); err != nil {
		w.failExamGrantBulkJob(ctx, job, fmt.Sprintf("upload result: %v", err))
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
		msg := fmt.Sprintf("exam_grant_bulk job %s: all %d rows failed", job.ID, len(results))
		errMsg = &msg
	}
	if err := w.jobRepo.FinishJob(ctx, job.ID, status, 100, &resultKey, errMsg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) failExamGrantBulkJob(ctx context.Context, job model.Job, msg string) {
	if err := w.jobRepo.FinishJob(ctx, job.ID, "failed", job.Progress, nil, &msg); err != nil {
		slog.Error("finish job", "job_id", job.ID, "err", err)
	}
}
