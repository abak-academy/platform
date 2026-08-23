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

	// School/admin_exam result detail remains bound to the exam's student-visible
	// result config. The super-admin results workspace has its own
	// results-workspace:read-gated detail path below for operational answer review.
	if exam.ResultConfig == "score_pembahasan" {
		detail.Breakdown = topicBreakdown(tests, answers)
		detail.Pembahasan = buildPembahasan(tests, answers, false)
	}

	return detail, nil
}

// AdminGetResultsWorkspaceResultDetail returns operational answer detail for the
// super-admin-only results workspace. Unlike GetSchoolResultDetail, this
// intentionally includes breakdown/pembahasan independent of student-visible
// result_config, but the route is gated by results-workspace:read.
func (s *Service) AdminGetResultsWorkspaceResultDetail(ctx context.Context, examID, sessionID uuid.UUID) (model.AdminResultDetail, error) {
	sess, err := s.storeRepo.GetSchoolResultSession(ctx, sessionID, "")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AdminResultDetail{}, ErrSessionNotFound
		}
		return model.AdminResultDetail{}, err
	}
	if sess.ExamID != examID {
		return model.AdminResultDetail{}, ErrSessionNotFound
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

	return model.AdminResultDetail{
		SessionID:    sess.SessionID,
		StudentName:  sess.StudentName,
		Username:     sess.Username,
		Score:        score,
		SubmittedAt:  sess.SubmittedAt,
		ResultConfig: exam.ResultConfig,
		CorrectCount: correct,
		WrongCount:   wrong,
		EmptyCount:   empty,
		Breakdown:    topicBreakdown(tests, answers),
		Pembahasan:   buildPembahasan(tests, answers, true),
	}, nil
}

// ExportSchoolResultsCSV builds the school/admin_exam summary export. It never
// includes per-question answers or points, so score_only exams cannot leak answer
// keys to school-scoped admins.
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

// ExportDetailedResultsCSV builds a detailed export with per-question answers,
// using latest-scored ranking semantics. Applies score_only policy: no correct
// answers or explanations leak (Issue 130). Header includes rank, student,
// school, score, counts, then per-question: answer + points.
func (s *Service) ExportDetailedResultsCSV(ctx context.Context, examID uuid.UUID, schoolID, q string) ([]byte, error) {
	if _, err := s.storeRepo.GetExamByID(ctx, examID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}

	rows, err := s.storeRepo.ListDetailedExportRows(ctx, examID, schoolID, q)
	if err != nil {
		return nil, err
	}

	// Fetch all questions to build header and ensure consistent ordering.
	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, examID)
	if err != nil {
		return nil, err
	}

	var questions []model.QuestionWithOptions
	for _, td := range tests {
		questions = append(questions, td.Questions...)
	}

	return BuildDetailedResultsCSV(rows, questions), nil
}

// BuildDetailedResultsCSV writes detailed results with per-question columns.
// score_only policy: IsCorrect, CorrectAnswer, Explanation never appear.
func BuildDetailedResultsCSV(rows []model.AdminExportRow, questions []model.QuestionWithOptions) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header: summary columns + per-question (Answer, Points).
	header := []string{"Rank", "Student Name", "Username", "School", "Score", "Correct", "Wrong", "Empty", "Started At", "Submitted At", "Duration Seconds"}
	for i := range questions {
		qNum := fmt.Sprintf("Q%d", i+1)
		header = append(header, qNum+" Answer", qNum+" Points")
	}
	_ = w.Write(header)

	for _, r := range rows {
		correctCount, wrongCount, emptyCount := objectiveCounts(questions, exportQuestionRowsAsAnswers(r.QuestionRows))
		questionRowsByID := make(map[uuid.UUID]model.AdminExportQuestionRow, len(r.QuestionRows))
		for _, qa := range r.QuestionRows {
			questionRowsByID[qa.QuestionID] = qa
		}

		// Summary row cells.
		rankStr := fmt.Sprintf("%d", r.Rank)
		username := ""
		if r.Username != nil {
			username = *r.Username
		}
		schoolName := ""
		if r.SchoolName != nil {
			schoolName = *r.SchoolName
		}
		scoreStr := ""
		if r.Score != nil {
			scoreStr = fmt.Sprintf("%v", *r.Score)
		}
		submittedAt := ""
		if r.SubmittedAt != nil {
			submittedAt = r.SubmittedAt.Format(time.RFC3339)
		}
		startedAt := ""
		if r.StartedAt != nil {
			startedAt = r.StartedAt.Format(time.RFC3339)
		}
		durationSeconds := ""
		if r.StartedAt != nil && r.SubmittedAt != nil {
			duration := r.SubmittedAt.Sub(*r.StartedAt).Seconds()
			if duration < 0 {
				duration = 0
			}
			durationSeconds = fmt.Sprintf("%.0f", duration)
		}

		row := []string{
			rankStr,
			csvSafeField(r.StudentName),
			csvSafeField(username),
			csvSafeField(schoolName),
			scoreStr,
			fmt.Sprintf("%d", correctCount),
			fmt.Sprintf("%d", wrongCount),
			fmt.Sprintf("%d", emptyCount),
			startedAt,
			submittedAt,
			durationSeconds,
		}

		// Per-question columns: answer + points (no is_correct, no correct_answer).
		for _, q := range questions {
			answer := ""
			points := ""

			if qa, ok := questionRowsByID[q.Question.ID]; ok {
				if qa.StudentAnswer != nil {
					answer = csvSafeField(*qa.StudentAnswer)
				}
				if qa.Points != nil {
					points = fmt.Sprintf("%v", *qa.Points)
				}
			}

			row = append(row, answer, points)
		}

		_ = w.Write(row)
	}

	w.Flush()
	return buf.Bytes()
}

func exportQuestionRowsAsAnswers(rows []model.AdminExportQuestionRow) []model.ExamSessionAnswer {
	answers := make([]model.ExamSessionAnswer, 0, len(rows))
	for _, r := range rows {
		answers = append(answers, model.ExamSessionAnswer{
			QuestionID: r.QuestionID,
			Answer:     r.StudentAnswer,
			Score:      r.Points,
			IsCorrect:  r.IsCorrect,
		})
	}
	return answers
}
