package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ListSchoolResults returns fully graded submissions independent of student result visibility.
func (s *Service) ListSchoolResults(ctx context.Context, examID uuid.UUID, schoolID, q, cursor string, limit int) ([]model.AdminResultRow, string, error) {
	_, err := s.storeRepo.GetExamByID(ctx, examID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrExamNotFound
		}
		return nil, "", err
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	filter := repository.AdminResultFilter{
		Q:      q,
		Cursor: cursor,
		Limit:  limit,
	}

	rows, next, err := s.storeRepo.ListSchoolResults(ctx, examID, schoolID, filter)
	return rows, next, err
}

// GetSchoolResultDetail returns the full detail of a single school-scoped session
// result (FR-SCHOOL-08-12..16). Gate violations (hidden, grading, locked) and
// cross-school access all surface as ErrSessionNotFound (FR-SCHOOL-08-13). Never
// calls CountHigherScores or computeRank — no rank field in the response
// (FR-SCHOOL-08-16).
func (s *Service) GetSchoolResultDetail(ctx context.Context, sessionID uuid.UUID, schoolID string) (model.AdminResultDetail, error) {
	sess, err := s.storeRepo.GetSchoolResultSession(ctx, sessionID, schoolID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AdminResultDetail{}, ErrSessionNotFound
		}
		return model.AdminResultDetail{}, err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return model.AdminResultDetail{}, err
	}

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return model.AdminResultDetail{}, err
	}
	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessionID)
	if err != nil {
		return model.AdminResultDetail{}, err
	}

	var qs []model.QuestionWithOptions
	for _, td := range tests {
		qs = append(qs, td.Questions...)
	}

	if sess.Status != "submitted" || !isFullyGraded(qs, answers) {
		return model.AdminResultDetail{}, ErrSessionNotFound
	}

	score := 0.0
	if sess.Score != nil {
		score = *sess.Score
	}
	correct, wrong, empty := objectiveCounts(qs, answers)

	detail := model.AdminResultDetail{
		SessionID:    sess.SessionID,
		StudentName:  sess.StudentName,
		Username:     sess.Username,
		Score:        score,
		SubmittedAt:  sess.SubmittedAt,
		ResultConfig: exam.ResultConfig,
		CorrectCount: correct,
		WrongCount:   wrong,
		EmptyCount:   empty,
	}

	// Admin result drill-down is an operational view, not the student-visible
	// result gate. Always include the per-topic and per-question detail so the
	// super-admin Results tab can expand a row into the exact submitted answers.
	detail.Breakdown = topicBreakdown(tests, answers)
	detail.Pembahasan = buildPembahasan(tests, answers)

	return detail, nil
}

// ExportSchoolResultsCSV builds a CSV of school-scoped results for an exam
// by looping ListSchoolResults page by page until exhausted (FR-SCHOOL-08-17).
// Uses encoding/csv + bytes.Buffer, matching BuildCredentialsResultCSV in
// bulk_credentials.go. Header rows are always written even when the result
// set is empty (hidden/locked exam -> header only, no error).
func (s *Service) ExportSchoolResultsCSV(ctx context.Context, examID uuid.UUID, schoolID, q string) ([]byte, error) {
	var rows []model.AdminResultRow
	cursor := ""
	for {
		page, next, err := s.ListSchoolResults(ctx, examID, schoolID, q, cursor, 100)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)
		if next == "" {
			break
		}
		cursor = next
	}

	return BuildSchoolResultsCSV(rows), nil
}

// BuildSchoolResultsCSV writes the results export as CSV bytes. Split out of
// ExportSchoolResultsCSV so the row encoding is reachable without a repository.
func BuildSchoolResultsCSV(rows []model.AdminResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "username", "score", "submitted_at"})
	for _, r := range rows {
		username := ""
		if r.Username != nil {
			username = *r.Username
		}
		scoreStr := ""
		if r.Score != nil {
			scoreStr = fmt.Sprintf("%v", *r.Score)
		}
		submittedAt := ""
		if r.SubmittedAt != nil {
			submittedAt = r.SubmittedAt.Format(time.RFC3339)
		}
		// score and submitted_at are machine-formatted, never attacker-supplied;
		// sanitising them would turn a negative score into text.
		_ = w.Write([]string{csvSafeField(r.StudentName), csvSafeField(username), scoreStr, submittedAt})
	}
	w.Flush()
	return buf.Bytes()
}
