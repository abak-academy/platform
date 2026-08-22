package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- fakeSessionRepo: admin results extensions ----------

func studentNameFromID(id uuid.UUID) string {
	return "Student " + id.String()[:8]
}

func studentNISFromID(id uuid.UUID) *string {
	s := "NIS-" + id.String()[:8]
	return &s
}

func (f *fakeSessionRepo) ListSchoolResults(_ context.Context, examID uuid.UUID, schoolID string, filter repository.AdminResultFilter) ([]model.AdminResultRow, string, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}

	type entry struct {
		sessionID   uuid.UUID
		studentID   uuid.UUID
		score       *float64
		submittedAt *time.Time
	}

	var entries []entry
	for _, s := range f.sessions {
		if s.ExamID != examID || s.Status != "submitted" {
			continue
		}
		if !f.fullyGraded(s.ID) {
			continue
		}
		if f.studentSchools[s.StudentID] != schoolID {
			continue
		}
		entries = append(entries, entry{
			sessionID:   s.ID,
			studentID:   s.StudentID,
			score:       s.Score,
			submittedAt: s.SubmittedAt,
		})
	}

	// Sort by submitted_at DESC, id ASC.
	sort.SliceStable(entries, func(i, j int) bool {
		ti, tj := entries[i].submittedAt, entries[j].submittedAt
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.After(*tj)
		}
		if ti == nil && tj != nil {
			return false
		}
		if ti != nil && tj == nil {
			return true
		}
		return entries[i].sessionID.String() < entries[j].sessionID.String()
	})

	// Apply cursor filter.
	if filter.Cursor != "" {
		timeStr, idStr, found := strings.Cut(filter.Cursor, ",")
		if !found {
			return nil, "", fmt.Errorf("%w: %q", repository.ErrInvalidCursor, filter.Cursor)
		}
		cursorTime, err := time.Parse(time.RFC3339Nano, timeStr)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", repository.ErrInvalidCursor, err)
		}
		cursorID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", repository.ErrInvalidCursor, err)
		}
		var filtered []entry
		for _, e := range entries {
			if e.submittedAt != nil {
				if e.submittedAt.Before(cursorTime) || (e.submittedAt.Equal(cursorTime) && e.sessionID.String() > cursorID.String()) {
					filtered = append(filtered, e)
				}
			}
		}
		entries = filtered
	}

	// Paginate.
	var nextCursor string
	if len(entries) > filter.Limit {
		entries = entries[:filter.Limit]
		last := entries[filter.Limit-1]
		if last.submittedAt != nil {
			nextCursor = last.submittedAt.Format(time.RFC3339Nano) + "," + last.sessionID.String()
		}
	}

	result := make([]model.AdminResultRow, len(entries))
	for i, e := range entries {
		name := studentNameFromID(e.studentID)
		nis := studentNISFromID(e.studentID)
		result[i] = model.AdminResultRow{
			SessionID:   e.sessionID,
			StudentName: name,
			Username:    nis,
			Score:       e.score,
			SubmittedAt: e.submittedAt,
		}
	}
	return result, nextCursor, nil
}

func (f *fakeSessionRepo) GetSchoolResultSession(_ context.Context, sessionID uuid.UUID, schoolID string) (*model.AdminResultSession, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if f.studentSchools[s.StudentID] != schoolID {
		return nil, repository.ErrNotFound
	}
	name := studentNameFromID(s.StudentID)
	nis := studentNISFromID(s.StudentID)
	return &model.AdminResultSession{
		SessionID:   s.ID,
		ExamID:      s.ExamID,
		StudentID:   s.StudentID,
		StudentName: name,
		Username:    nis,
		Status:      s.Status,
		Score:       s.Score,
		SubmittedAt: s.SubmittedAt,
	}, nil
}

// ---------- shimSessionService: admin results extensions ----------

func (s *shimSessionService) ListSchoolResults(ctx context.Context, examID uuid.UUID, schoolID, q, cursor string, limit int) ([]model.AdminResultRow, string, error) {
	_, err := s.repo.GetExamForSession(ctx, examID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrExamNotFound
		}
		return nil, "", err
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	f := repository.AdminResultFilter{Q: q, Cursor: cursor, Limit: limit}
	rows, next, err := s.repo.ListSchoolResults(ctx, examID, schoolID, f)
	return rows, next, err
}

func (s *shimSessionService) GetSchoolResultDetail(ctx context.Context, sessionID uuid.UUID, schoolID string) (model.AdminResultDetail, error) {
	sess, err := s.repo.GetSchoolResultSession(ctx, sessionID, schoolID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AdminResultDetail{}, ErrSessionNotFound
		}
		return model.AdminResultDetail{}, err
	}

	exam, err := s.repo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return model.AdminResultDetail{}, err
	}

	tests, err := s.repo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return model.AdminResultDetail{}, err
	}
	answers, err := s.repo.GetSessionAnswers(ctx, sessionID)
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

	if exam.ResultConfig == "score_pembahasan" {
		detail.Breakdown = topicBreakdown(tests, answers)
		detail.Pembahasan = buildPembahasan(tests, answers, false)
	}

	return detail, nil
}

// ---------- tests: ListSchoolResults ----------

func TestAdminResultList_HiddenExam_ReturnsAdminResults(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "hidden"})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	rows, next, err := svc.ListSchoolResults(ctx, examID, schoolID, "", "", 20)
	if err != nil {
		t.Fatalf("ListSchoolResults: want nil error, got %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows: want 1, got %d", len(rows))
	}
	if next != "" {
		t.Errorf("next cursor: want empty, got %q", next)
	}
}

func TestAdminResultList_FutureRelease_ReturnsAdminResults(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	releaseAt := time.Now().Add(24 * time.Hour)
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only", ResultReleaseAt: &releaseAt})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	rows, next, err := svc.ListSchoolResults(ctx, examID, schoolID, "", "", 20)
	if err != nil {
		t.Fatalf("ListSchoolResults: want nil error, got %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows: want 1, got %d", len(rows))
	}
	if next != "" {
		t.Errorf("next cursor: want empty, got %q", next)
	}
}

func TestAdminResultList_ExamNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, _, err := svc.ListSchoolResults(ctx, uuid.New(), uuid.New().String(), "", "", 20)
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

func TestAdminResultList_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})
	svc.repo.seedTests(examID, nil) // no essays -> trivially fully graded

	now := time.Now()
	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80), SubmittedAt: &now,
	}

	rows, next, err := svc.ListSchoolResults(ctx, examID, schoolID, "", "", 20)
	if err != nil {
		t.Fatalf("ListSchoolResults: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].SessionID != sessID {
		t.Errorf("session id mismatch")
	}
	if rows[0].Score == nil || *rows[0].Score != 80 {
		t.Errorf("score: want 80, got %v", rows[0].Score)
	}
	if next != "" {
		t.Errorf("want empty cursor, got %q", next)
	}
}

func TestAdminResultList_CrossSchoolIsolation(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolA := uuid.New().String()
	schoolB := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})
	svc.repo.seedTests(examID, nil)

	studentA := uuid.New()
	svc.repo.studentSchools[studentA] = schoolA
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), StudentID: studentA, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	// School B queries -> should see no results.
	rows, next, err := svc.ListSchoolResults(ctx, examID, schoolB, "", "", 20)
	if err != nil {
		t.Fatalf("ListSchoolResults: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("want 0 rows for school B, got %d", len(rows))
	}
	if next != "" {
		t.Errorf("want empty cursor, got %q", next)
	}
}

// ---------- tests: GetSchoolResultDetail ----------

func TestAdminResultDetail_Hidden_ReturnsAdminResult(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "hidden"})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	detail, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if err != nil || detail.SessionID != sessID {
		t.Errorf("want admin detail for %s, got %+v, err %v", sessID, detail, err)
	}
}

func TestAdminResultDetail_Grading_NotSubmitted_GatesToNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "in_progress",
	}

	_, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestAdminResultDetail_Grading_NotFullyGraded_GatesToNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	testID := uuid.New()
	td, qID := essayTest(testID, 5)
	svc.repo.seedTests(examID, []model.TestDetail{td})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(0),
	}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("my essay"), GradedAt: nil},
	}

	_, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestAdminResultDetail_FutureRelease_ReturnsAdminResult(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	releaseAt := time.Now().Add(24 * time.Hour)
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only", ResultReleaseAt: &releaseAt})

	testID := uuid.New()
	svc.repo.seedTests(examID, []model.TestDetail{mcqTest(testID, examID)})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	detail, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if err != nil || detail.SessionID != sessID {
		t.Errorf("want admin detail for %s, got %+v, err %v", sessID, detail, err)
	}
}

func TestAdminResultDetail_CrossSchool_GatesToNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolA := uuid.New().String()
	schoolB := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolA
	sessID := uuid.New()
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	_, err := svc.GetSchoolResultDetail(ctx, sessID, schoolB)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestAdminResultDetail_ScoreOnly_DoesNotExposePembahasan(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})

	testID := uuid.New()
	td := mcqTest(testID, examID)
	qID := td.Questions[0].Question.ID
	svc.repo.seedTests(examID, []model.TestDetail{td})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	trueVal := true
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(2),
	}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: floatPtr(2), GradedAt: timePtr(time.Now())},
	}

	detail, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if err != nil {
		t.Fatalf("GetSchoolResultDetail: %v", err)
	}
	if detail.Score != 2 {
		t.Errorf("score: want 2, got %v", detail.Score)
	}
	if detail.CorrectCount != 1 || detail.WrongCount != 0 || detail.EmptyCount != 0 {
		t.Errorf("counts: want 1/0/0, got %d/%d/%d", detail.CorrectCount, detail.WrongCount, detail.EmptyCount)
	}
	if len(detail.Breakdown) != 0 {
		t.Fatalf("score_only school-scoped detail should not include breakdown, got %d", len(detail.Breakdown))
	}
	if len(detail.Pembahasan) != 0 {
		t.Fatalf("score_only school-scoped detail should not include pembahasan, got %d", len(detail.Pembahasan))
	}
}

func TestAdminResultDetail_ScorePembahasan_HasBoth(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_pembahasan"})

	testID := uuid.New()
	td := mcqTest(testID, examID)
	qID := td.Questions[0].Question.ID
	svc.repo.seedTests(examID, []model.TestDetail{td})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	trueVal := true
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(2),
	}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: floatPtr(2), GradedAt: timePtr(time.Now())},
	}

	detail, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if err != nil {
		t.Fatalf("GetSchoolResultDetail: %v", err)
	}
	if len(detail.Breakdown) != 1 {
		t.Fatalf("want 1 breakdown row, got %d", len(detail.Breakdown))
	}
	if detail.Breakdown[0].Earned != 2 || detail.Breakdown[0].Max != 2 {
		t.Errorf("breakdown earned/max: want 2/2, got %v/%v", detail.Breakdown[0].Earned, detail.Breakdown[0].Max)
	}
	if len(detail.Pembahasan) != 1 {
		t.Fatalf("want 1 pembahasan item, got %d", len(detail.Pembahasan))
	}
	if detail.Pembahasan[0].CorrectAnswer == nil || *detail.Pembahasan[0].CorrectAnswer != "b" {
		t.Errorf("pembahasan correct_answer: want b, got %v", detail.Pembahasan[0].CorrectAnswer)
	}
}

// ---------- shimSessionService: admin results export ----------

func (s *shimSessionService) ExportSchoolResultsCSV(ctx context.Context, examID uuid.UUID, schoolID, q string) ([]byte, error) {
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

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "username", "score", "submitted_at"})
	for _, r := range rows {
		nis := ""
		if r.Username != nil {
			nis = *r.Username
		}
		scoreStr := ""
		if r.Score != nil {
			scoreStr = fmt.Sprintf("%v", *r.Score)
		}
		submittedAt := ""
		if r.SubmittedAt != nil {
			submittedAt = r.SubmittedAt.Format(time.RFC3339)
		}
		_ = w.Write([]string{r.StudentName, nis, scoreStr, submittedAt})
	}
	w.Flush()
	return buf.Bytes(), nil
}

// ---------- tests: ExportSchoolResultsCSV ----------

func TestExportSchoolResults_HiddenExam_IncludesAdminResults(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "hidden"})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	csvData, err := svc.ExportSchoolResultsCSV(ctx, examID, schoolID, "")
	if err != nil {
		t.Fatalf("ExportSchoolResultsCSV: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("want header and result, got %d records", len(records))
	}
	wantHeader := []string{"name", "username", "score", "submitted_at"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}
}

func TestExportSchoolResults_FutureRelease_IncludesAdminResults(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	releaseAt := time.Now().Add(24 * time.Hour)
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only", ResultReleaseAt: &releaseAt})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	svc.repo.sessions[uuid.New()] = &model.ExamSession{
		ID: uuid.New(), StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(80),
	}

	csvData, err := svc.ExportSchoolResultsCSV(ctx, examID, schoolID, "")
	if err != nil {
		t.Fatalf("ExportSchoolResultsCSV: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("want header and result, got %d records", len(records))
	}
	wantHeader := []string{"name", "username", "score", "submitted_at"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}
}

func TestExportSchoolResults_ExamNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.ExportSchoolResultsCSV(ctx, uuid.New(), uuid.New().String(), "")
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

func TestExportSchoolResults_PaginateThenCompare(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_only"})
	svc.repo.seedTests(examID, nil)

	now := time.Now()
	seedSessions := 5
	for i := 0; i < seedSessions; i++ {
		studentID := uuid.New()
		svc.repo.studentSchools[studentID] = schoolID
		svc.repo.sessions[uuid.New()] = &model.ExamSession{
			ID: uuid.New(), StudentID: studentID, ExamID: examID,
			Status: "submitted", Score: floatPtr(float64(70 + i)), SubmittedAt: &now,
		}
	}

	// Manually page through ListSchoolResults with limit=2 to exercise pagination.
	var accumulated []model.AdminResultRow
	cursor := ""
	for {
		page, next, err := svc.ListSchoolResults(ctx, examID, schoolID, "", cursor, 2)
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		accumulated = append(accumulated, page...)
		if next == "" {
			break
		}
		cursor = next
	}
	if len(accumulated) != seedSessions {
		t.Fatalf("accumulated rows: want %d, got %d", seedSessions, len(accumulated))
	}

	// Now call export.
	csvData, err := svc.ExportSchoolResultsCSV(ctx, examID, schoolID, "")
	if err != nil {
		t.Fatalf("ExportSchoolResultsCSV: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if len(records) != seedSessions+1 {
		t.Fatalf("want %d records (header + %d rows), got %d", seedSessions+1, seedSessions, len(records))
	}

	// Verify each accumulated row has a matching CSV row.
	for i, row := range accumulated {
		csvRow := records[i+1] // +1 for header
		if csvRow[0] != row.StudentName {
			t.Errorf("row %d name: want %s, got %s", i, row.StudentName, csvRow[0])
		}
		nis := ""
		if row.Username != nil {
			nis = *row.Username
		}
		if csvRow[1] != nis {
			t.Errorf("row %d nis: want %s, got %s", i, nis, csvRow[1])
		}
		scoreStr := ""
		if row.Score != nil {
			scoreStr = fmt.Sprintf("%v", *row.Score)
		}
		if csvRow[2] != scoreStr {
			t.Errorf("row %d score: want %s, got %s", i, scoreStr, csvRow[2])
		}
		submittedAt := ""
		if row.SubmittedAt != nil {
			submittedAt = row.SubmittedAt.Format(time.RFC3339)
		}
		if csvRow[3] != submittedAt {
			t.Errorf("row %d submitted_at: want %s, got %s", i, submittedAt, csvRow[3])
		}
	}
}

func TestAdminResultDetail_NoRankInJSON(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	schoolID := uuid.New().String()
	examID := uuid.New()
	svc.repo.seedExam(&model.Exam{ID: examID, ResultConfig: "score_pembahasan"})

	testID := uuid.New()
	td := mcqTest(testID, examID)
	qID := td.Questions[0].Question.ID
	svc.repo.seedTests(examID, []model.TestDetail{td})

	studentID := uuid.New()
	svc.repo.studentSchools[studentID] = schoolID
	sessID := uuid.New()
	trueVal := true
	svc.repo.sessions[sessID] = &model.ExamSession{
		ID: sessID, StudentID: studentID, ExamID: examID,
		Status: "submitted", Score: floatPtr(2),
	}
	svc.repo.sessionAnswers[sessID] = []model.ExamSessionAnswer{
		{QuestionID: qID, Answer: strPtr("b"), IsCorrect: &trueVal, Score: floatPtr(2), GradedAt: timePtr(time.Now())},
	}

	detail, err := svc.GetSchoolResultDetail(ctx, sessID, schoolID)
	if err != nil {
		t.Fatalf("GetSchoolResultDetail: %v", err)
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(data), `"rank"`) {
		t.Error("JSON body must not contain 'rank' (FR-SCHOOL-08-16)")
	}
}
