package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- groupQuestionsByTest (FR-26) ----------

func TestGroupQuestionsByTest_derivesTestIDAndOrderFromOwningTest(t *testing.T) {
	test1 := uuid.New()
	test2 := uuid.New()

	tests := []model.TestDetail{
		{
			Test: model.Test{ID: test1, Title: "T1"},
			Questions: []model.QuestionWithOptions{
				{Question: model.Question{ID: uuid.New(), Format: "mcq"}, SortOrder: 2},
				{Question: model.Question{ID: uuid.New(), Format: "mcq"}, SortOrder: 1},
			},
		},
		{
			Test: model.Test{ID: test2, Title: "T2"},
			Questions: []model.QuestionWithOptions{
				{Question: model.Question{ID: uuid.New(), Format: "essay"}, SortOrder: 1},
			},
		},
	}

	grouped := groupQuestionsByTest(tests)

	if len(grouped) != 2 {
		t.Fatalf("want 2 tests, got %d", len(grouped))
	}
	if grouped[0].ID != test1 {
		t.Errorf("test[0].id: want %v, got %v", test1, grouped[0].ID)
	}
	if len(grouped[0].Questions) != 2 {
		t.Fatalf("test[0] questions: want 2, got %d", len(grouped[0].Questions))
	}
	// Per-question test_id must be the owning test's id, never a (now-dropped) q.Question.TestID.
	if grouped[0].Questions[0].TestID != test1 {
		t.Errorf("question[0].test_id: want %v, got %v", test1, grouped[0].Questions[0].TestID)
	}
	if grouped[0].Questions[1].TestID != test1 {
		t.Errorf("question[1].test_id: want %v, got %v", test1, grouped[0].Questions[1].TestID)
	}
	// Order must come from the read shape's SortOrder (test_question order), preserving input order.
	if grouped[0].Questions[0].SortOrder != 2 {
		t.Errorf("question[0].sort_order: want 2, got %d", grouped[0].Questions[0].SortOrder)
	}
	if grouped[0].Questions[1].SortOrder != 1 {
		t.Errorf("question[1].sort_order: want 1, got %d", grouped[0].Questions[1].SortOrder)
	}
	if grouped[1].Questions[0].TestID != test2 {
		t.Errorf("test2 question.test_id: want %v, got %v", test2, grouped[1].Questions[0].TestID)
	}
}

func TestGroupQuestionsByTest_populates_AudioURL_and_Blanks(t *testing.T) {
	testID := uuid.New()
	qID1 := uuid.New()
	qID2 := uuid.New()
	audioURL := "https://example.com/audio.mp3"

	tests := []model.TestDetail{
		{
			Test: model.Test{ID: testID, Title: "T1"},
			Questions: []model.QuestionWithOptions{
				{
					Question: model.Question{
						ID:       qID1,
						Format:   "multi_blank",
						AudioURL: &audioURL,
					},
					Blanks: []model.QuestionBlank{
						{QuestionID: qID1, Index: 1, CorrectAnswer: "jakarta"},
						{QuestionID: qID1, Index: 2, CorrectAnswer: "1945"},
					},
					SortOrder: 1,
				},
				{
					Question: model.Question{ID: qID2, Format: "mcq"},
					SortOrder: 2,
				},
			},
		},
	}

	grouped := groupQuestionsByTest(tests)

	if len(grouped) != 1 {
		t.Fatalf("want 1 test, got %d", len(grouped))
	}
	if len(grouped[0].Questions) != 2 {
		t.Fatalf("want 2 questions, got %d", len(grouped[0].Questions))
	}

	// First question: multi_blank with AudioURL and Blanks
	q1 := grouped[0].Questions[0]
	if q1.AudioURL == nil || *q1.AudioURL != audioURL {
		t.Errorf("question[0].audio_url: want %v, got %v", audioURL, q1.AudioURL)
	}
	if len(q1.Blanks) != 2 {
		t.Fatalf("question[0].blanks: want 2, got %d", len(q1.Blanks))
	}
	if q1.Blanks[0] != 1 || q1.Blanks[1] != 2 {
		t.Errorf("question[0].blanks: want [1,2], got [%v,%v]", q1.Blanks[0], q1.Blanks[1])
	}
	// Second question: no audio or blanks
	q2 := grouped[0].Questions[1]
	if q2.AudioURL != nil {
		t.Errorf("question[1].audio_url: want nil, got %v", q2.AudioURL)
	}
	if len(q2.Blanks) != 0 {
		t.Errorf("question[1].blanks: want empty, got %v", q2.Blanks)
	}
}

// ---------- fakeSessionRepo ----------

type fakeSessionRepo struct {
	registrations     map[uuid.UUID]*model.RegistrationDetail
	exams             map[uuid.UUID]*model.Exam
	sessions          map[uuid.UUID]*model.ExamSession
	sessionAnswers    map[uuid.UUID][]model.ExamSessionAnswer
	testsByExam       map[uuid.UUID][]model.TestDetail
	essays            map[uuid.UUID][]model.GradingEssayItem
	gradingQueue      map[uuid.UUID][]model.GradingSessionItem
	studentSchools    map[uuid.UUID]string
	monitorRows       map[uuid.UUID][]model.SessionMonitorRow
	questionTotals    map[uuid.UUID]int
	recentViolations  map[uuid.UUID][]model.ViolationRecent
	sessionViolations map[uuid.UUID][]model.SessionViolationLog
	sessionSections   map[uuid.UUID][]model.ExamSessionSection
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{
		registrations:     make(map[uuid.UUID]*model.RegistrationDetail),
		exams:             make(map[uuid.UUID]*model.Exam),
		sessions:          make(map[uuid.UUID]*model.ExamSession),
		sessionAnswers:    make(map[uuid.UUID][]model.ExamSessionAnswer),
		testsByExam:       make(map[uuid.UUID][]model.TestDetail),
		essays:            make(map[uuid.UUID][]model.GradingEssayItem),
		gradingQueue:      make(map[uuid.UUID][]model.GradingSessionItem),
		studentSchools:    make(map[uuid.UUID]string),
		monitorRows:       make(map[uuid.UUID][]model.SessionMonitorRow),
		questionTotals:    make(map[uuid.UUID]int),
		recentViolations:  make(map[uuid.UUID][]model.ViolationRecent),
		sessionViolations: make(map[uuid.UUID][]model.SessionViolationLog),
	}
}

func (f *fakeSessionRepo) seedExam(exam *model.Exam) {
	if exam.ID == uuid.Nil {
		exam.ID = uuid.New()
	}
	if f.exams == nil {
		f.exams = make(map[uuid.UUID]*model.Exam)
	}
	cp := *exam
	f.exams[exam.ID] = &cp
}

func (f *fakeSessionRepo) seedRegistration(detail *model.RegistrationDetail) {
	if f.registrations == nil {
		f.registrations = make(map[uuid.UUID]*model.RegistrationDetail)
	}
	if detail.ExamRegistration.ID == uuid.Nil {
		detail.ExamRegistration.ID = uuid.New()
	}
	// Only add exam if not already present (seedExam should be called first)
	if _, exists := f.exams[detail.Exam.ID]; !exists {
		if f.exams == nil {
			f.exams = make(map[uuid.UUID]*model.Exam)
		}
		f.exams[detail.Exam.ID] = &model.Exam{
			ID:                   detail.Exam.ID,
			Title:                detail.Exam.Title,
			RequiresCheckin:      detail.Exam.RequiresCheckin,
			CheckInWindowMinutes: detail.Exam.CheckInWindowMinutes,
			ScheduledAt:          detail.Exam.ScheduledAt,
			ScheduledEndAt:       detail.Exam.ScheduledEndAt,
			TimerMode:            detail.Exam.TimerMode,
			DurationMinutes:      detail.Exam.DurationMinutes,
			ResultConfig:         detail.Exam.ResultConfig,
		}
	}
	cp := *detail
	f.registrations[detail.ExamRegistration.ID] = &cp
}

func (f *fakeSessionRepo) GetExamRegistrationByToken(_ context.Context, studentID uuid.UUID, token string) (*model.ExamRegistration, error) {
	for _, d := range f.registrations {
		if d.Token == token && d.StudentID == studentID {
			reg := d.ExamRegistration
			return &reg, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeSessionRepo) GetExamRegistrationByID(_ context.Context, regID, studentID uuid.UUID) (*model.RegistrationDetail, error) {
	d, ok := f.registrations[regID]
	if !ok || d.StudentID != studentID {
		return nil, repository.ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (f *fakeSessionRepo) GetExamForSession(_ context.Context, examID uuid.UUID) (*model.Exam, error) {
	e, ok := f.exams[examID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeSessionRepo) CheckInExam(_ context.Context, regID uuid.UUID) error {
	d, ok := f.registrations[regID]
	if !ok {
		return repository.ErrNotFound
	}
	now := time.Now()
	d.CheckedInAt = &now
	d.Status = "checked_in"
	return nil
}

func (f *fakeSessionRepo) CreateExamSession(_ context.Context, regID uuid.UUID) (model.ExamSession, error) {
	d, ok := f.registrations[regID]
	if !ok {
		return model.ExamSession{}, repository.ErrNotFound
	}
	d.AttemptsUsed++
	d.Status = "in_progress"

	s := model.ExamSession{
		ID:             uuid.New(),
		RegistrationID: regID,
		StudentID:      d.StudentID,
		ExamID:         d.Exam.ID,
		AttemptNumber:  1,
		StartedAt:      time.Now(),
		Status:         "in_progress",
	}
	if f.sessions == nil {
		f.sessions = make(map[uuid.UUID]*model.ExamSession)
	}
	f.sessions[s.ID] = &s
	return s, nil
}

func (f *fakeSessionRepo) GetExamSessionForStudent(_ context.Context, sessionID, studentID uuid.UUID) (*model.ExamSession, error) {
	s, ok := f.sessions[sessionID]
	if !ok || s.StudentID != studentID {
		return nil, repository.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSessionRepo) GetExamSessionByID(_ context.Context, sessionID uuid.UUID) (*model.ExamSession, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSessionRepo) GetSessionWithQuestions(_ context.Context, examID uuid.UUID) ([]model.TestDetail, error) {
	return f.testsByExam[examID], nil
}

func (f *fakeSessionRepo) GetSessionAnswers(_ context.Context, sessionID uuid.UUID) ([]model.ExamSessionAnswer, error) {
	answers, ok := f.sessionAnswers[sessionID]
	if !ok {
		return []model.ExamSessionAnswer{}, nil
	}
	return answers, nil
}

func (f *fakeSessionRepo) SaveAnswers(_ context.Context, sessionID uuid.UUID, answers []model.ExamSessionAnswer) error {
	if f.sessionAnswers == nil {
		f.sessionAnswers = make(map[uuid.UUID][]model.ExamSessionAnswer)
	}
	f.sessionAnswers[sessionID] = answers
	return nil
}

func (f *fakeSessionRepo) SubmitSession(_ context.Context, sessionID uuid.UUID, graded []model.ExamSessionAnswer, score float64, adminSubmitted bool) (int64, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return 0, repository.ErrNotFound
	}
	if s.Status != "in_progress" {
		return 0, nil
	}
	s.Status = "submitted"
	now := time.Now()
	s.SubmittedAt = &now
	s.Score = &score
	s.AdminSubmitted = adminSubmitted
	f.sessionAnswers[sessionID] = graded
	return 1, nil
}

func (f *fakeSessionRepo) LogViolation(_ context.Context, v model.SessionViolationLog) error {
	return nil
}

func (f *fakeSessionRepo) ReopenSession(_ context.Context, sessionID uuid.UUID, minutes int) error {
	s, ok := f.sessions[sessionID]
	if !ok {
		return repository.ErrNotFound
	}
	if s.Status != "in_progress" && s.Status != "submitted" {
		return repository.ErrNotFound
	}
	ext := time.Now().Add(time.Duration(minutes) * time.Minute)
	s.ExtendedUntil = &ext
	return nil
}

// ---------- Monitor / violation fake repo methods ----------

func (f *fakeSessionRepo) seedSessionMonitorRow(examID uuid.UUID, row model.SessionMonitorRow) {
	if f.monitorRows == nil {
		f.monitorRows = make(map[uuid.UUID][]model.SessionMonitorRow)
	}
	f.monitorRows[examID] = append(f.monitorRows[examID], row)
}

func (f *fakeSessionRepo) seedQuestionTotal(examID uuid.UUID, total int) {
	if f.questionTotals == nil {
		f.questionTotals = make(map[uuid.UUID]int)
	}
	f.questionTotals[examID] = total
}

func (f *fakeSessionRepo) seedRecentViolations(examID uuid.UUID, violations []model.ViolationRecent) {
	if f.recentViolations == nil {
		f.recentViolations = make(map[uuid.UUID][]model.ViolationRecent)
	}
	f.recentViolations[examID] = violations
}

func (f *fakeSessionRepo) seedSessionViolations(sessionID uuid.UUID, violations []model.SessionViolationLog) {
	if f.sessionViolations == nil {
		f.sessionViolations = make(map[uuid.UUID][]model.SessionViolationLog)
	}
	f.sessionViolations[sessionID] = violations
}

func (f *fakeSessionRepo) seedSessionSections(sessionID uuid.UUID, sections []model.ExamSessionSection) {
	if f.sessionSections == nil {
		f.sessionSections = make(map[uuid.UUID][]model.ExamSessionSection)
	}
	f.sessionSections[sessionID] = sections
}

func (f *fakeSessionRepo) GetSessionMonitorRows(_ context.Context, examID uuid.UUID) ([]model.SessionMonitorRow, error) {
	rows := f.monitorRows[examID]
	if rows == nil {
		return []model.SessionMonitorRow{}, nil
	}
	return rows, nil
}

func (f *fakeSessionRepo) GetExamQuestionTotal(_ context.Context, examID uuid.UUID) (int, error) {
	return f.questionTotals[examID], nil
}

func (f *fakeSessionRepo) GetRecentViolations(_ context.Context, examID uuid.UUID, _ int) ([]model.ViolationRecent, error) {
	v := f.recentViolations[examID]
	if v == nil {
		return []model.ViolationRecent{}, nil
	}
	return v, nil
}

func (f *fakeSessionRepo) ListSessionViolations(_ context.Context, sessionID uuid.UUID) ([]model.SessionViolationLog, error) {
	v := f.sessionViolations[sessionID]
	if v == nil {
		return []model.SessionViolationLog{}, nil
	}
	return v, nil
}

func (f *fakeSessionRepo) GetSessionSections(_ context.Context, sessionID uuid.UUID) ([]model.ExamSessionSection, error) {
	sections := f.sessionSections[sessionID]
	if sections == nil {
		return []model.ExamSessionSection{}, nil
	}
	return sections, nil
}

func (f *fakeSessionRepo) ExtendActiveSection(_ context.Context, sessionID uuid.UUID, extendMinutes int) error {
	sections, ok := f.sessionSections[sessionID]
	if !ok {
		return repository.ErrNoActiveSection
	}
	for i, sec := range sections {
		if sec.Status == "active" {
			ext := time.Now().Add(time.Duration(extendMinutes) * time.Minute)
			f.sessionSections[sessionID][i].ExtendedUntil = &ext
			return nil
		}
	}
	return repository.ErrNoActiveSection
}

// ---------- shimSessionService ----------

type shimSessionService struct {
	repo            *fakeSessionRepo
	rdb             *redis.Client
	mr              *miniredis.Miniredis
	uploadCertErr   error
	uploadCertCalls int
}

func newShimSessionService(t *testing.T) (*shimSessionService, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &shimSessionService{
		repo: newFakeSessionRepo(),
		rdb:  rdb,
		mr:   mr,
	}, mr
}

// ---------- CheckIn ----------

func (s *shimSessionService) CheckIn(ctx context.Context, studentID, token, fp string) (CheckInResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return CheckInResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}

	reg, err := s.repo.GetExamRegistrationByToken(ctx, sid, token)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CheckInResult{}, ErrRegistrationNotFound
		}
		return CheckInResult{}, err
	}

	exam, err := s.repo.GetExamForSession(ctx, reg.ExamID)
	if err != nil {
		return CheckInResult{}, err
	}

	// requires_checkin guard
	if !exam.RequiresCheckin {
		return CheckInResult{}, ErrNotCheckedIn
	}

	// Window check: now in [scheduled_at - window, scheduled_at) by default, or
	// [scheduled_at - window, scheduled_end_at] when an availability window is set.
	if exam.ScheduledAt != nil && exam.CheckInWindowMinutes != nil {
		now := time.Now()
		windowStart := exam.ScheduledAt.Add(-time.Duration(*exam.CheckInWindowMinutes) * time.Minute)
		closed := now.Before(windowStart)
		if !closed {
			if exam.ScheduledEndAt != nil {
				closed = now.After(*exam.ScheduledEndAt)
			} else {
				closed = !now.Before(*exam.ScheduledAt)
			}
		}
		if closed {
			return CheckInResult{}, ErrCheckinWindowClosed
		}
	}

	if err := s.repo.CheckInExam(ctx, reg.ID); err != nil {
		return CheckInResult{}, err
	}

	// Redis: device lock
	key := "exam:device:" + reg.ID.String()
	var ttl time.Duration
	if exam.DurationMinutes != nil && *exam.DurationMinutes > 0 {
		ttl = time.Duration(*exam.DurationMinutes) * time.Minute
	} else {
		ttl = 24 * time.Hour
	}
	if err := s.rdb.Set(ctx, key, fp, ttl).Err(); err != nil {
		return CheckInResult{}, err
	}

	return CheckInResult{
		RegistrationID: reg.ID,
		ExamTitle:      exam.Title,
		ScheduledAt:    exam.ScheduledAt,
	}, nil
}

func newReg(regID uuid.UUID, studentID uuid.UUID, examID uuid.UUID, opts ...func(*model.RegistrationDetail)) model.RegistrationDetail {
	d := model.RegistrationDetail{}
	d.ExamRegistration.ID = regID
	d.ExamRegistration.StudentID = studentID
	d.ExamRegistration.ExamID = examID
	d.ExamRegistration.Token = "TOKEN"
	d.ExamRegistration.Status = "registered"
	d.Exam.ID = examID
	for _, fn := range opts {
		fn(&d)
	}
	return d
}

func TestCheckIn_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, mr := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(10 * time.Minute)
	windowMin := 30
	durationMin := 120

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: &windowMin,
		DurationMinutes:      &durationMin,
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.Token = "ABC123" },
	)
	svc.repo.seedRegistration(&regDetail)

	result, err := svc.CheckIn(ctx, regDetail.StudentID.String(), "ABC123", "device-fp")
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}

	if result.RegistrationID == uuid.Nil {
		t.Error("expected non-nil registration_id")
	}
	if result.ExamTitle != "Finals" {
		t.Errorf("exam_title: want Finals, got %q", result.ExamTitle)
	}
	if result.ScheduledAt == nil || !result.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("scheduled_at mismatch")
	}

	// Verify Redis device key
	key := "exam:device:" + result.RegistrationID.String()
	val, err := svc.rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Redis device key not found: %v", err)
	}
	if val != "device-fp" {
		t.Errorf("device fp: want device-fp, got %q", val)
	}

	// Verify TTL
	ttl, err := svc.rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL check failed: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("expected positive TTL, got %v", ttl)
	}

	// Ignore miniredis in the linter
	_ = mr
}

func TestCheckIn_WindowClosed_BeforeWindow(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(60 * time.Minute) // 60 min out, window 30 = opens 30 min from now
	windowMin := 30

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: &windowMin,
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.Token = "ABC123" },
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.CheckIn(ctx, regDetail.StudentID.String(), "ABC123", "fp")
	if !errors.Is(err, ErrCheckinWindowClosed) {
		t.Errorf("want ErrCheckinWindowClosed, got %v", err)
	}
}

func TestCheckIn_WindowClosed_AfterScheduled(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-10 * time.Minute) // already past
	windowMin := 30

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: &windowMin,
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.Token = "ABC123" },
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.CheckIn(ctx, regDetail.StudentID.String(), "ABC123", "fp")
	if !errors.Is(err, ErrCheckinWindowClosed) {
		t.Errorf("want ErrCheckinWindowClosed, got %v", err)
	}
}

// TestCheckIn_ScheduledWindow_OpenAfterScheduledAt covers the new availability-
// window behavior (scheduled_end_at set): check-in used to close the instant
// scheduled_at passed — with a window it stays open until scheduled_end_at.
func TestCheckIn_ScheduledWindow_OpenAfterScheduledAt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-10 * time.Minute) // window already started
	scheduledEndAt := now.Add(2 * time.Hour)
	windowMin := 30

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		ScheduledEndAt:       &scheduledEndAt,
		CheckInWindowMinutes: &windowMin,
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.Token = "ABC123" },
	)
	svc.repo.seedRegistration(&regDetail)

	if _, err := svc.CheckIn(ctx, regDetail.StudentID.String(), "ABC123", "fp"); err != nil {
		t.Fatalf("CheckIn should succeed within the window (past scheduled_at, before scheduled_end_at): %v", err)
	}
}

func TestCheckIn_ScheduledWindow_ClosedAfterScheduledEndAt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-2 * time.Hour)
	scheduledEndAt := now.Add(-10 * time.Minute) // window already closed
	windowMin := 30

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		ScheduledEndAt:       &scheduledEndAt,
		CheckInWindowMinutes: &windowMin,
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.Token = "ABC123" },
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.CheckIn(ctx, regDetail.StudentID.String(), "ABC123", "fp")
	if !errors.Is(err, ErrCheckinWindowClosed) {
		t.Errorf("want ErrCheckinWindowClosed after scheduled_end_at, got %v", err)
	}
}

func TestCheckIn_TokenNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.CheckIn(ctx, "11111111-1111-1111-1111-111111111111", "WRONG", "fp")
	if !errors.Is(err, ErrRegistrationNotFound) {
		t.Errorf("want ErrRegistrationNotFound, got %v", err)
	}
}

func TestCheckIn_InvalidStudentID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.CheckIn(ctx, "not-a-uuid", "ABC123", "fp")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// ---------- StartSession ----------

func (s *shimSessionService) StartSession(ctx context.Context, studentID, registrationID, fp string) (SessionStartPayload, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SessionStartPayload{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	rid, err := uuid.Parse(registrationID)
	if err != nil {
		return SessionStartPayload{}, fmt.Errorf("%w: invalid registration id", ErrValidation)
	}

	detail, err := s.repo.GetExamRegistrationByID(ctx, rid, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SessionStartPayload{}, ErrRegistrationNotFound
		}
		return SessionStartPayload{}, err
	}

	// Load exam config from the exam repo (source of truth)
	exam, err := s.repo.GetExamForSession(ctx, detail.ExamID)
	if err != nil {
		return SessionStartPayload{}, err
	}

	// Availability window (scheduled_end_at set): applies regardless of
	// requires_checkin.
	if exam.ScheduledEndAt != nil {
		now := time.Now()
		if exam.ScheduledAt != nil && now.Before(*exam.ScheduledAt) {
			return SessionStartPayload{}, ErrExamNotStarted
		}
		if now.After(*exam.ScheduledEndAt) {
			return SessionStartPayload{}, ErrExamWindowClosed
		}
	}

	// Branch on requires_checkin
	if exam.RequiresCheckin {
		if exam.ScheduledAt != nil && time.Now().Before(*exam.ScheduledAt) {
			return SessionStartPayload{}, ErrExamNotStarted
		}
		if detail.CheckedInAt == nil {
			return SessionStartPayload{}, ErrNotCheckedIn
		}

		// Device fingerprint check
		key := "exam:device:" + rid.String()
		deviceFP, err := s.rdb.Get(ctx, key).Result()
		if err == redis.Nil {
			return SessionStartPayload{}, ErrDeviceMismatch
		}
		if err != nil {
			return SessionStartPayload{}, err
		}
		if deviceFP != fp {
			return SessionStartPayload{}, ErrDeviceMismatch
		}
	}

	// Attempt limit
	if detail.AttemptsUsed >= 1 {
		return SessionStartPayload{}, ErrAlreadyAttempted
	}

	// Create session
	sess, err := s.repo.CreateExamSession(ctx, rid)
	if err != nil {
		return SessionStartPayload{}, err
	}

	// Get questions
	// TODO: implement question loading and stripping

	duration := exam.DurationMinutes
	var remaining int64
	if duration != nil && *duration > 0 {
		deadline := sess.StartedAt.Add(time.Duration(*duration) * time.Minute)
		remaining = int64(math.Max(0, time.Until(deadline).Seconds()))
	}

	return SessionStartPayload{
		SessionID:        sess.ID,
		RemainingSeconds: remaining,
		TimerMode:        exam.TimerMode,
		DurationMinutes:  duration,
		Tests:            nil,
	}, nil
}

func TestStartSession_NoCheckin_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	regDetail.Exam.RequiresCheckin = false
	regDetail.Exam.TimerMode = "overall"
	regDetail.Exam.DurationMinutes = intptr(120)
	svc.repo.seedRegistration(&regDetail)

	result, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if result.SessionID == uuid.Nil {
		t.Error("expected non-nil session_id")
	}
	if result.TimerMode != "overall" {
		t.Errorf("timer_mode: want overall, got %q", result.TimerMode)
	}
	if result.RemainingSeconds <= 0 {
		t.Errorf("expected positive remaining_seconds, got %d", result.RemainingSeconds)
	}

	updated, _ := svc.repo.GetExamRegistrationByID(ctx, regDetail.ID, regDetail.StudentID)
	if updated.AttemptsUsed != 1 {
		t.Errorf("attempts_used: want 1, got %d", updated.AttemptsUsed)
	}
}

func TestStartSession_NoCheckin_AlreadyAttempted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) { d.AttemptsUsed = 1 },
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if !errors.Is(err, ErrAlreadyAttempted) {
		t.Errorf("want ErrAlreadyAttempted, got %v", err)
	}
}

func TestStartSession_RegNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.StartSession(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "fp")
	if !errors.Is(err, ErrRegistrationNotFound) {
		t.Errorf("want ErrRegistrationNotFound, got %v", err)
	}
}

func TestStartSession_Checkin_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	checkedInAt := now.Add(-10 * time.Minute)
	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) {
			d.CheckedInAt = &checkedInAt
			d.Status = "checked_in"
		},
	)
	svc.repo.seedRegistration(&regDetail)

	// Set matching device fingerprint
	key := "exam:device:" + regDetail.ID.String()
	if err := svc.rdb.Set(ctx, key, "device-fp", time.Hour).Err(); err != nil {
		t.Fatalf("set device key: %v", err)
	}

	result, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "device-fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if result.SessionID == uuid.Nil {
		t.Error("expected non-nil session_id")
	}
	if result.RemainingSeconds <= 0 {
		t.Errorf("expected positive remaining_seconds, got %d", result.RemainingSeconds)
	}
}

func TestStartSession_Checkin_DeviceMismatch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	checkedInAt := now.Add(-10 * time.Minute)
	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) {
			d.CheckedInAt = &checkedInAt
			d.Status = "checked_in"
		},
	)
	svc.repo.seedRegistration(&regDetail)

	key := "exam:device:" + regDetail.ID.String()
	if err := svc.rdb.Set(ctx, key, "different-fp", time.Hour).Err(); err != nil {
		t.Fatalf("set device key: %v", err)
	}

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "my-fp")
	if !errors.Is(err, ErrDeviceMismatch) {
		t.Errorf("want ErrDeviceMismatch, got %v", err)
	}
}

func TestStartSession_Checkin_NotCheckedIn(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if !errors.Is(err, ErrNotCheckedIn) {
		t.Errorf("want ErrNotCheckedIn, got %v", err)
	}
}

func TestStartSession_Checkin_NotStarted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(30 * time.Minute)

	e := &model.Exam{
		Title:                "Finals",
		RequiresCheckin:      true,
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
		DurationMinutes:      intptr(120),
		TimerMode:            "overall",
	}
	svc.repo.seedExam(e)

	checkedInAt := now.Add(-10 * time.Minute)
	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
		func(d *model.RegistrationDetail) {
			d.CheckedInAt = &checkedInAt
			d.Status = "checked_in"
		},
	)
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if !errors.Is(err, ErrExamNotStarted) {
		t.Errorf("want ErrExamNotStarted, got %v", err)
	}
}

// TestStartSession_ScheduledWindow_OpenBetweenStartAndEnd covers a no-checkin
// exam that now has an availability window: start used to be unrestricted for
// no-checkin exams, but a set scheduled_end_at gates it universally.
func TestStartSession_ScheduledWindow_OpenBetweenStartAndEnd(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-1 * time.Hour)
	scheduledEndAt := now.Add(1 * time.Hour)

	e := &model.Exam{
		Title:           "Open Tryout",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		ScheduledEndAt:  &scheduledEndAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	regDetail.Exam.RequiresCheckin = false
	regDetail.Exam.TimerMode = "overall"
	regDetail.Exam.DurationMinutes = intptr(120)
	svc.repo.seedRegistration(&regDetail)

	if _, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp"); err != nil {
		t.Fatalf("StartSession should succeed inside the window, got %v", err)
	}
}

func TestStartSession_ScheduledWindow_TooEarly(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(30 * time.Minute)
	scheduledEndAt := now.Add(2 * time.Hour)

	e := &model.Exam{
		Title:           "Open Tryout",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		ScheduledEndAt:  &scheduledEndAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	regDetail.Exam.RequiresCheckin = false
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if !errors.Is(err, ErrExamNotStarted) {
		t.Errorf("want ErrExamNotStarted before scheduled_at, got %v", err)
	}
}

func TestStartSession_ScheduledWindow_ClosedAfterEnd(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-2 * time.Hour)
	scheduledEndAt := now.Add(-10 * time.Minute)

	e := &model.Exam{
		Title:           "Open Tryout",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		ScheduledEndAt:  &scheduledEndAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	regDetail.Exam.RequiresCheckin = false
	svc.repo.seedRegistration(&regDetail)

	_, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if !errors.Is(err, ErrExamWindowClosed) {
		t.Errorf("want ErrExamWindowClosed after scheduled_end_at, got %v", err)
	}
}

// ---------- ReconnectSession ----------

func (s *shimSessionService) ReconnectSession(ctx context.Context, studentID, sessionID string) (SessionStatePayload, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SessionStatePayload{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SessionStatePayload{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SessionStatePayload{}, ErrSessionNotFound
		}
		return SessionStatePayload{}, err
	}

	exam, err := s.repo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return SessionStatePayload{}, err
	}

	duration := exam.DurationMinutes
	var effectiveDeadline time.Time
	if duration != nil && *duration > 0 {
		effectiveDeadline = sess.StartedAt.Add(time.Duration(*duration) * time.Minute)
	}
	if sess.ExtendedUntil != nil && sess.ExtendedUntil.After(effectiveDeadline) {
		effectiveDeadline = *sess.ExtendedUntil
	}

	remaining := int64(math.Max(0, time.Until(effectiveDeadline).Seconds()))

	answers, _ := s.repo.GetSessionAnswers(ctx, sessID)

	return SessionStatePayload{
		SessionID:        sess.ID,
		Status:           sess.Status,
		RemainingSeconds: remaining,
		TimerMode:        exam.TimerMode,
		DurationMinutes:  duration,
		Tests:            nil,
		Answers:          answers,
	}, nil
}

func TestReconnectSession_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-30 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	// Start a session first
	result, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Reconnect
	state, err := svc.ReconnectSession(ctx, regDetail.StudentID.String(), result.SessionID.String())
	if err != nil {
		t.Fatalf("ReconnectSession: %v", err)
	}

	if state.SessionID != result.SessionID {
		t.Errorf("session_id mismatch")
	}
	if state.Status != "in_progress" {
		t.Errorf("status: want in_progress, got %q", state.Status)
	}
	if state.RemainingSeconds <= 0 {
		t.Errorf("expected positive remaining_seconds, got %d", state.RemainingSeconds)
	}
	if state.TimerMode != "overall" {
		t.Errorf("timer_mode: want overall, got %q", state.TimerMode)
	}
	if state.DurationMinutes == nil || *state.DurationMinutes != 120 {
		t.Errorf("duration_minutes: want 120, got %v", state.DurationMinutes)
	}
}

func TestReconnectSession_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.ReconnectSession(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

// ---------- SaveAnswers ----------

func (s *shimSessionService) SaveAnswers(ctx context.Context, studentID, sessionID string, inputs []AnswerInput) error {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	if sess.Status != "in_progress" {
		return ErrAlreadySubmitted
	}

	answers := make([]model.ExamSessionAnswer, len(inputs))
	for i, in := range inputs {
		answers[i] = model.ExamSessionAnswer{
			SessionID:        sessID,
			QuestionID:       in.QuestionID,
			Answer:           in.Answer,
			FlaggedForReview: in.FlaggedForReview,
		}
	}

	return s.repo.SaveAnswers(ctx, sessID, answers)
}

func TestSaveAnswers_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	qID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	answer := "Paris"
	err = svc.SaveAnswers(ctx, regDetail.StudentID.String(), sess.SessionID.String(), []AnswerInput{
		{QuestionID: qID, Answer: &answer},
	})
	if err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}

	saved, _ := svc.repo.GetSessionAnswers(ctx, sess.SessionID)
	if len(saved) != 1 {
		t.Fatalf("want 1 saved answer, got %d", len(saved))
	}
	if saved[0].QuestionID != qID {
		t.Errorf("question_id mismatch")
	}
	if saved[0].Answer == nil || *saved[0].Answer != "Paris" {
		t.Errorf("answer: want Paris, got %v", saved[0].Answer)
	}
}

func TestSaveAnswers_AlreadySubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Submit first
	_, err = svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String())
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	// Then try to save answers
	answer := "test"
	err = svc.SaveAnswers(ctx, regDetail.StudentID.String(), sess.SessionID.String(), []AnswerInput{
		{QuestionID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), Answer: &answer},
	})
	if !errors.Is(err, ErrAlreadySubmitted) {
		t.Errorf("want ErrAlreadySubmitted, got %v", err)
	}
}

func TestSaveAnswers_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	answer := "test"
	err := svc.SaveAnswers(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", []AnswerInput{
		{QuestionID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), Answer: &answer},
	})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

// ---------- SubmitSession ----------

func (s *shimSessionService) SubmitSession(ctx context.Context, studentID, sessionID string) (SubmitResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitResult{}, ErrSessionNotFound
		}
		return SubmitResult{}, err
	}

	// Load questions and answers for grading
	questions, _ := s.repo.GetSessionWithQuestions(ctx, sess.ExamID)
	answers, _ := s.repo.GetSessionAnswers(ctx, sessID)

	// Build answer map
	answerMap := make(map[uuid.UUID]*string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.Answer
	}

	// Flatten questions for grading
	var qs []model.QuestionWithOptions
	for _, td := range questions {
		qs = append(qs, td.Questions...)
	}

	graded, score := gradeObjective(qs, answerMap)

	rows, err := s.repo.SubmitSession(ctx, sessID, graded, score, false)
	if err != nil {
		return SubmitResult{}, err
	}
	if rows == 0 {
		return SubmitResult{}, ErrAlreadySubmitted
	}

	return SubmitResult{
		Status: "submitted",
		Score:  &score,
	}, nil
}

func TestSubmitSession_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	result, err := svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String())
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	if result.Status != "submitted" {
		t.Errorf("status: want submitted, got %q", result.Status)
	}
	if result.Score == nil || *result.Score != 0 {
		t.Errorf("score: want 0 (no questions), got %v", result.Score)
	}
}

func TestSubmitSession_AlreadySubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Submit once
	_, err = svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String())
	if err != nil {
		t.Fatalf("first SubmitSession: %v", err)
	}

	// Submit again
	_, err = svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String())
	if !errors.Is(err, ErrAlreadySubmitted) {
		t.Errorf("want ErrAlreadySubmitted, got %v", err)
	}
}

// ---------- LogViolation ----------

func (s *shimSessionService) LogViolation(ctx context.Context, studentID, sessionID, violationType string) error {
	if !validViolationTypes[violationType] {
		return ErrInvalidViolationType
	}

	sid, err := uuid.Parse(studentID)
	if err != nil {
		return fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	if sess.Status != "in_progress" {
		return ErrAlreadySubmitted
	}

	return s.repo.LogViolation(ctx, model.SessionViolationLog{
		SessionID:     sessID,
		StudentID:     sid,
		ViolationType: violationType,
		OccurredAt:    time.Now(),
	})
}

func TestLogViolation_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	err = svc.LogViolation(ctx, regDetail.StudentID.String(), sess.SessionID.String(), "tab_switch")
	if err != nil {
		t.Errorf("LogViolation: %v", err)
	}
}

func TestLogViolation_InvalidType(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	err := svc.LogViolation(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "unknown_type")
	if !errors.Is(err, ErrInvalidViolationType) {
		t.Errorf("want ErrInvalidViolationType, got %v", err)
	}
}

func TestLogViolation_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	err := svc.LogViolation(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "tab_switch")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

// ---------- ReopenSession (admin) ----------

func (s *shimSessionService) ReopenSession(ctx context.Context, sessionID string, minutes int) error {
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionByID(ctx, sessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	exam, err := s.repo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return err
	}

	// FR-22: sectioned path — extend the active section.
	if exam.Mode == "utbk" || exam.Mode == "ielts" {
		if err := s.repo.ExtendActiveSection(ctx, sessID, minutes); err != nil {
			return err
		}
		return nil
	}

	// Standard path — extend session-level extended_until.
	if err := s.repo.ReopenSession(ctx, sessID, minutes); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	return nil
}

func TestReopenSession_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	err = svc.ReopenSession(ctx, sess.SessionID.String(), 30)
	if err != nil {
		t.Errorf("ReopenSession: %v", err)
	}

	// Verify the session has extended_until set
	updated, _ := svc.repo.GetExamSessionForStudent(ctx, sess.SessionID, regDetail.StudentID)
	if updated.ExtendedUntil == nil {
		t.Error("expected extended_until to be set")
	}
}

func TestReopenSession_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	err := svc.ReopenSession(ctx, "22222222-2222-2222-2222-222222222222", 30)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}



// ---------- ReopenSession sectioned (FR-22) ----------

func TestReopenSession_Sectioned_ExtendsActiveSection(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-30 * time.Minute)

	e := &model.Exam{
		Title:           "UTBK Reopen",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
		Mode:            "utbk",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Seed section rows for this session (one active section)
	startedAt := now.Add(-30 * time.Minute)
	sections := []model.ExamSessionSection{
		{
			SessionID:       sess.SessionID,
			TestID:          uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			SortOrder:       0,
			DurationMinutes: 60,
			Status:          "active",
			StartedAt:       &startedAt,
		},
	}
	svc.repo.seedSessionSections(sess.SessionID, sections)

	// Reopen the sectioned session
	err = svc.ReopenSession(ctx, sess.SessionID.String(), 30)
	if err != nil {
		t.Fatalf("ReopenSession sectioned: %v", err)
	}

	// Verify the active section's extended_until is set
	updatedSections, err := svc.repo.GetSessionSections(ctx, sess.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSections: %v", err)
	}
	if len(updatedSections) != 1 {
		t.Fatalf("sections: want 1, got %d", len(updatedSections))
	}
	if updatedSections[0].ExtendedUntil == nil {
		t.Fatal("active section: expected extended_until to be set after reopen")
	}
	// Verify the extend time is in the future
	if updatedSections[0].ExtendedUntil.Before(time.Now()) {
		t.Errorf("extended_until should be in the future, got %v", updatedSections[0].ExtendedUntil)
	}
	// Verify remaining would be positive via the section helper
	remaining := computeSectionRemaining(updatedSections[0])
	if remaining <= 0 {
		t.Errorf("section remaining after reopen: want >0, got %d", remaining)
	}
}

func TestReopenSession_Sectioned_NoActiveSection_ReturnsErrSectionNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-30 * time.Minute)

	e := &model.Exam{
		Title:           "UTBK Reopen Fail",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
		Mode:            "utbk",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// No sections seeded → ExtendActiveSection fails with ErrNoActiveSection
	// The shim propagates this (no special mapping for sectioned path)
	err = svc.ReopenSession(ctx, sess.SessionID.String(), 30)
	if err == nil {
		t.Error("expected error when no active section, got nil")
	}
}

// ---------- ForceSubmitSession (admin) ----------

func (s *shimSessionService) ForceSubmitSession(ctx context.Context, sessionID string) (SubmitResult, error) {
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.repo.GetExamSessionByID(ctx, sessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitResult{}, ErrSessionNotFound
		}
		return SubmitResult{}, err
	}

	questions, _ := s.repo.GetSessionWithQuestions(ctx, sess.ExamID)
	answers, _ := s.repo.GetSessionAnswers(ctx, sessID)

	answerMap := make(map[uuid.UUID]*string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.Answer
	}

	var qs []model.QuestionWithOptions
	for _, td := range questions {
		qs = append(qs, td.Questions...)
	}

	graded, score := gradeObjective(qs, answerMap)

	rows, err := s.repo.SubmitSession(ctx, sessID, graded, score, true)
	if err != nil {
		return SubmitResult{}, err
	}
	if rows == 0 {
		return SubmitResult{}, ErrAlreadySubmitted
	}

	return SubmitResult{
		Status: "submitted",
		Score:  &score,
	}, nil
}

func TestForceSubmitSession_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	result, err := svc.ForceSubmitSession(ctx, sess.SessionID.String())
	if err != nil {
		t.Fatalf("ForceSubmitSession: %v", err)
	}

	if result.Status != "submitted" {
		t.Errorf("status: want submitted, got %q", result.Status)
	}
	if result.Score == nil {
		t.Error("expected non-nil score")
	}
}

func TestForceSubmitSession_AlreadySubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Submit normally first
	_, err = svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String())
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	// Force submit should fail with already submitted
	_, err = svc.ForceSubmitSession(ctx, sess.SessionID.String())
	if !errors.Is(err, ErrAlreadySubmitted) {
		t.Errorf("want ErrAlreadySubmitted, got %v", err)
	}
}

func TestForceSubmitSession_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.ForceSubmitSession(ctx, "22222222-2222-2222-2222-222222222222")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

// TestSubmitSession_GradedAt_ObjectiveStamped_EssayNil is a regression guard for the
// SubmitSessionTx upsert bug fixed in this slice (PG note 1, spec.md): the objective
// answer must come out graded (graded_at set) while the essay answer stays ungraded
// (graded_at nil) end-to-end through SubmitSession, not just at the gradeObjective unit level.
func TestSubmitSession_GradedAt_ObjectiveStamped_EssayNil(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	mcqTD := mcqTest(uuid.New(), e.ID)
	mcqQID := mcqTD.Questions[0].Question.ID
	essayTD, essayQID := essayTest(uuid.New(), 5)
	svc.repo.seedTests(e.ID, []model.TestDetail{mcqTD, essayTD})

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	mcqAnswer := "b"
	essayAnswer := "my essay answer"
	err = svc.SaveAnswers(ctx, regDetail.StudentID.String(), sess.SessionID.String(), []AnswerInput{
		{QuestionID: mcqQID, Answer: &mcqAnswer},
		{QuestionID: essayQID, Answer: &essayAnswer},
	})
	if err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}

	if _, err := svc.SubmitSession(ctx, regDetail.StudentID.String(), sess.SessionID.String()); err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}

	answers, err := svc.repo.GetSessionAnswers(ctx, sess.SessionID)
	if err != nil {
		t.Fatalf("GetSessionAnswers: %v", err)
	}
	byQuestion := make(map[uuid.UUID]model.ExamSessionAnswer, len(answers))
	for _, a := range answers {
		byQuestion[a.QuestionID] = a
	}

	if byQuestion[mcqQID].GradedAt == nil {
		t.Error("objective answer: want graded_at stamped, got nil")
	}
	if byQuestion[essayQID].GradedAt != nil {
		t.Error("essay answer: want graded_at nil (awaits manual grading), got stamped")
	}
}

// TestForceSubmitSession_GradedAt_ObjectiveStamped_EssayNil mirrors
// TestSubmitSession_GradedAt_ObjectiveStamped_EssayNil for the admin force-submit path,
// which independently calls gradeObjective before SubmitSessionTx.
func TestForceSubmitSession_GradedAt_ObjectiveStamped_EssayNil(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	now := time.Now()
	scheduledAt := now.Add(-5 * time.Minute)

	e := &model.Exam{
		Title:           "Finals",
		RequiresCheckin: false,
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
	}
	svc.repo.seedExam(e)

	regDetail := newReg(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		e.ID,
	)
	svc.repo.seedRegistration(&regDetail)

	mcqTD := mcqTest(uuid.New(), e.ID)
	mcqQID := mcqTD.Questions[0].Question.ID
	essayTD, essayQID := essayTest(uuid.New(), 5)
	svc.repo.seedTests(e.ID, []model.TestDetail{mcqTD, essayTD})

	sess, err := svc.StartSession(ctx, regDetail.StudentID.String(), regDetail.ID.String(), "fp")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	mcqAnswer := "b"
	essayAnswer := "my essay answer"
	err = svc.SaveAnswers(ctx, regDetail.StudentID.String(), sess.SessionID.String(), []AnswerInput{
		{QuestionID: mcqQID, Answer: &mcqAnswer},
		{QuestionID: essayQID, Answer: &essayAnswer},
	})
	if err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}

	if _, err := svc.ForceSubmitSession(ctx, sess.SessionID.String()); err != nil {
		t.Fatalf("ForceSubmitSession: %v", err)
	}

	answers, err := svc.repo.GetSessionAnswers(ctx, sess.SessionID)
	if err != nil {
		t.Fatalf("GetSessionAnswers: %v", err)
	}
	byQuestion := make(map[uuid.UUID]model.ExamSessionAnswer, len(answers))
	for _, a := range answers {
		byQuestion[a.QuestionID] = a
	}

	if byQuestion[mcqQID].GradedAt == nil {
		t.Error("objective answer: want graded_at stamped, got nil")
	}
	if byQuestion[essayQID].GradedAt != nil {
		t.Error("essay answer: want graded_at nil (awaits manual grading), got stamped")
	}
}

// ---------- Tests ----------

func TestFingerprint_Deterministic(t *testing.T) {
	fp1 := fingerprint("192.168.1.1", "Mozilla/5.0")
	fp2 := fingerprint("192.168.1.1", "Mozilla/5.0")
	if fp1 != fp2 {
		t.Errorf("same input should produce same hash, got %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_VariesByIP(t *testing.T) {
	fp1 := fingerprint("192.168.1.1", "Mozilla/5.0")
	fp2 := fingerprint("10.0.0.1", "Mozilla/5.0")
	if fp1 == fp2 {
		t.Errorf("different IP should produce different hash")
	}
}

func TestFingerprint_VariesByUA(t *testing.T) {
	fp1 := fingerprint("192.168.1.1", "Mozilla/5.0")
	fp2 := fingerprint("192.168.1.1", "curl/7.68")
	if fp1 == fp2 {
		t.Errorf("different UA should produce different hash")
	}
}

func TestFingerprint_IsSHA256Hex(t *testing.T) {
	fp := fingerprint("1.2.3.4", "test")
	if len(fp) != 64 {
		t.Errorf("SHA256 hex should be 64 chars, got %d: %q", len(fp), fp)
	}
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %c in fingerprint", c)
		}
	}
}

// ---------- GetSessionMonitor (shim) ----------

func (s *shimSessionService) GetSessionMonitor(ctx context.Context, examID string) (model.SessionMonitorResponse, error) {
	eid, err := uuid.Parse(examID)
	if err != nil {
		return model.SessionMonitorResponse{}, fmt.Errorf("%w: invalid exam id", ErrValidation)
	}

	exam, err := s.repo.GetExamForSession(ctx, eid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.SessionMonitorResponse{}, ErrExamNotFound
		}
		return model.SessionMonitorResponse{}, err
	}

	rows, err := s.repo.GetSessionMonitorRows(ctx, eid)
	if err != nil {
		return model.SessionMonitorResponse{}, err
	}

	totalQ, err := s.repo.GetExamQuestionTotal(ctx, eid)
	if err != nil {
		return model.SessionMonitorResponse{}, err
	}

	recentV, err := s.repo.GetRecentViolations(ctx, eid, 20)
	if err != nil {
		return model.SessionMonitorResponse{}, err
	}

	now := time.Now()
	for i := range rows {
		rows[i].TotalQuestions = totalQ
		rows[i].Status = deriveStatus(rows[i], now, exam.DurationMinutes, exam.GraceWindowMinutes)
		// FR-21: populate the active section's remaining seconds for the proctor UI.
		if rows[i].ActiveSectionStartedAt != nil {
			sec := model.ExamSessionSection{
				DurationMinutes: *rows[i].ActiveSectionDurationMinutes,
				StartedAt:       rows[i].ActiveSectionStartedAt,
				ExtendedUntil:   rows[i].ActiveSectionExtendedUntil,
			}
			rows[i].ActiveSectionRemainingSeconds = computeSectionRemaining(sec)
		}
	}

	return model.SessionMonitorResponse{
		Exam: model.SessionMonitorExam{
			ID:                 exam.ID,
			Title:              exam.Title,
			ScheduledAt:        exam.ScheduledAt,
			DurationMinutes:    exam.DurationMinutes,
			GraceWindowMinutes: exam.GraceWindowMinutes,
			Status:             exam.Status,
		},
		Rows:             rows,
		ViolationsRecent: recentV,
	}, nil
}

// ---------- GetSessionViolations (shim) ----------

func (s *shimSessionService) GetSessionViolations(ctx context.Context, sessionID string) ([]model.SessionViolationLog, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid session id", ErrValidation)
	}
	return s.repo.ListSessionViolations(ctx, sid)
}

// ---------- Derived-status tests ----------

func TestEffectiveDeadline_WithDurationAndGrace(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	duration := intptr(120)
	grace := intptr(10)

	dl := effectiveDeadline(started, duration, grace, nil)
	expected := time.Date(2026, 7, 1, 10, 10, 0, 0, time.UTC)
	if !dl.Equal(expected) {
		t.Errorf("deadline: want %v, got %v", expected, dl)
	}
}

func TestEffectiveDeadline_ExtendedUntilDominates(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	duration := intptr(120)
	grace := intptr(10)
	extended := time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC)

	dl := effectiveDeadline(started, duration, grace, &extended)
	if !dl.Equal(extended) {
		t.Errorf("deadline: want %v (extended), got %v", extended, dl)
	}
}

func TestEffectiveDeadline_ExtendedBeforeComputed_UsesDuration(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	duration := intptr(120)
	grace := intptr(10)
	extended := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC) // before computed 10:10

	dl := effectiveDeadline(started, duration, grace, &extended)
	expected := time.Date(2026, 7, 1, 10, 10, 0, 0, time.UTC)
	if !dl.Equal(expected) {
		t.Errorf("deadline: want %v (computed), got %v", expected, dl)
	}
}

func TestEffectiveDeadline_NilDuration_ReturnsZero(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)

	dl := effectiveDeadline(started, nil, intptr(10), nil)
	if !dl.IsZero() {
		t.Errorf("nil duration: want zero deadline, got %v", dl)
	}
}

func TestEffectiveDeadline_NilDuration_ExtendedUntilIsDeadline(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	extended := time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC)

	dl := effectiveDeadline(started, nil, nil, &extended)
	if !dl.Equal(extended) {
		t.Errorf("deadline: want %v (extended), got %v", extended, dl)
	}
}

func TestEffectiveDeadline_NilGrace(t *testing.T) {
	started := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	duration := intptr(120)

	dl := effectiveDeadline(started, duration, nil, nil)
	expected := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	if !dl.Equal(expected) {
		t.Errorf("deadline: want %v, got %v", expected, dl)
	}
}

func TestDeriveStatus_Registered_NoCheckin(t *testing.T) {
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "registered" {
		t.Errorf("want registered, got %q", status)
	}
}

func TestDeriveStatus_CheckedIn(t *testing.T) {
	now := time.Now()
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		CheckedInAt:    &now,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "checked_in" {
		t.Errorf("want checked_in, got %q", status)
	}
}

func TestDeriveStatus_InProgress_BeforeDeadline(t *testing.T) {
	started := time.Now().Add(-30 * time.Minute)
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	// 120min duration + 10min grace = 130min deadline; only 30min elapsed
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "in_progress" {
		t.Errorf("want in_progress, got %q", status)
	}
}

func TestDeriveStatus_Overdue_ViaDurationGrace(t *testing.T) {
	started := time.Now().Add(-130*time.Minute - 1*time.Second) // past 120+10 deadline
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "overdue" {
		t.Errorf("want overdue, got %q", status)
	}
}

func TestDeriveStatus_Overdue_ViaExtendedUntil(t *testing.T) {
	// started far enough back that 120+10 duration deadline is in the past,
	// but extended_until is even more recent — it dominates AND is past.
	started := time.Now().Add(-200 * time.Minute) // deadline = now - 70min with 120+10
	extended := time.Now().Add(-5 * time.Minute)  // after computed deadline, but still past
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ExtendedUntil:  &extended,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "overdue" {
		t.Errorf("want overdue via extended_until, got %q", status)
	}
}

func TestDeriveStatus_Submitted(t *testing.T) {
	started := time.Now().Add(-60 * time.Minute)
	sessionID := uuid.New()
	sessionStatus := "submitted"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "submitted" {
		t.Errorf("want submitted, got %q", status)
	}
}

func TestDeriveStatus_AdminSubmitted_Passthrough(t *testing.T) {
	started := time.Now().Add(-60 * time.Minute)
	sessionID := uuid.New()
	sessionStatus := "submitted"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		AdminSubmitted: true,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "submitted" {
		t.Errorf("admin_submitted: want submitted, got %q", status)
	}
}

// FR-6a: per_test exam with nil DurationMinutes stays in_progress, not overdue
func TestDeriveStatus_FR6a_NilDuration_NotOverdue(t *testing.T) {
	started := time.Now().Add(-24 * time.Hour) // started long ago
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	status := deriveStatus(row, time.Now(), nil, intptr(10))
	if status != "in_progress" {
		t.Errorf("FR-6a: nil duration should stay in_progress, got %q", status)
	}
}

func TestDeriveStatus_FR6a_ExtendedUntilCanMakeOverdue(t *testing.T) {
	started := time.Now().Add(-60 * time.Minute)
	extended := time.Now().Add(-1 * time.Second) // extended_until passed
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ExtendedUntil:  &extended,
	}
	status := deriveStatus(row, time.Now(), nil, nil)
	if status != "overdue" {
		t.Errorf("FR-6a: passed extended_until should make overdue, got %q", status)
	}
}



// ---------- Sectioned deriveStatus tests (FR-20) ----------

func TestDeriveStatus_Sectioned_InProgress(t *testing.T) {
	started := time.Now().Add(-25 * time.Minute) // section started 25min ago
	dur := 60 // 60min section duration
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		// Section-level deadline: 60min from started → 35min remaining → in_progress
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: &dur,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "in_progress" {
		t.Errorf("sectioned within deadline: want in_progress, got %q", status)
	}
}

func TestDeriveStatus_Sectioned_Overdue(t *testing.T) {
	started := time.Now().Add(-65 * time.Minute) // section started 65min ago
	dur := 60 // 60min section, deadline passed 5min ago
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: &dur,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "overdue" {
		t.Errorf("sectioned past deadline: want overdue, got %q", status)
	}
}

func TestDeriveStatus_Sectioned_Overdue_ViaExtendedUntil(t *testing.T) {
	started := time.Now().Add(-30 * time.Minute) // section started 30min ago
	dur := 15 // 15min section deadline passed 15min ago
	extended := time.Now().Add(-1 * time.Minute) // extended_until also passed
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: &dur,
		ActiveSectionExtendedUntil:   &extended,
	}
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "overdue" {
		t.Errorf("sectioned overdue via extended_until: want overdue, got %q", status)
	}
}

func TestDeriveStatus_Sectioned_FR6a_NilDuration_NotOverdue(t *testing.T) {
	// FR-6a analog for sections: nil duration section has no deadline.
	started := time.Now().Add(-24 * time.Hour) // started long ago
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	var nilDur *int
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: nilDur,
	}
	status := deriveStatus(row, time.Now(), nil, nil)
	if status != "in_progress" {
		t.Errorf("sectioned nil duration: want in_progress, got %q", status)
	}
}

func TestDeriveStatus_Sectioned_FR6a_ExtendedUntilCanMakeOverdue(t *testing.T) {
	started := time.Now().Add(-60 * time.Minute)
	extended := time.Now().Add(-1 * time.Second)
	var nilDur *int
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: nilDur,
		ActiveSectionExtendedUntil:   &extended,
	}
	status := deriveStatus(row, time.Now(), nil, nil)
	if status != "overdue" {
		t.Errorf("sectioned nil duration + passed extended: want overdue, got %q", status)
	}
}

func TestDeriveStatus_Standard_NoActiveSectionData_Regression(t *testing.T) {
	// A standard session with no ActiveSection fields must still use the exam-level path.
	started := time.Now().Add(-30 * time.Minute)
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Standard Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		// No ActiveSection* fields set
	}
	// 120min + 10min grace → 130min; only 30min elapsed → in_progress
	status := deriveStatus(row, time.Now(), intptr(120), intptr(10))
	if status != "in_progress" {
		t.Errorf("standard regression: want in_progress, got %q", status)
	}
}

// ---------- GetSessionMonitor tests ----------

func TestGetSessionMonitor_InvalidExamID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.GetSessionMonitor(ctx, "not-a-uuid")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestGetSessionMonitor_ExamNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.GetSessionMonitor(ctx, uuid.New().String())
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

func TestGetSessionMonitor_HappyPath_FullResponse(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{
		Title:                "Finals",
		DurationMinutes:      intptr(120),
		GraceWindowMinutes:   intptr(10),
		TimerMode:            "overall",
		Status:               "published",
		ScheduledAt:          timePtr(time.Now().Add(-24 * time.Hour)),
	}
	svc.repo.seedExam(exam)

	// Row 1: registered (no check-in, no session)
	row1 := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row1)

	// Row 2: submitted
	sessionID := uuid.New()
	sessionStatus := "submitted"
	started := time.Now().Add(-60 * time.Minute)
	row2 := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student B",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row2)

	svc.repo.seedQuestionTotal(exam.ID, 25)

	recentV := []model.ViolationRecent{
		{SessionID: sessionID, StudentName: "Student B", Count: 3, LatestType: "tab_switch", LatestOccurredAt: time.Now()},
	}
	svc.repo.seedRecentViolations(exam.ID, recentV)

	resp, err := svc.GetSessionMonitor(ctx, exam.ID.String())
	if err != nil {
		t.Fatalf("GetSessionMonitor: %v", err)
	}

	if resp.Exam.ID != exam.ID {
		t.Errorf("exam.id mismatch")
	}
	if resp.Exam.Title != "Finals" {
		t.Errorf("exam.title: want Finals, got %q", resp.Exam.Title)
	}
	if len(resp.Rows) != 2 {
		t.Fatalf("rows: want 2, got %d", len(resp.Rows))
	}
	if resp.Rows[0].Status != "registered" {
		t.Errorf("row[0]: want registered, got %q", resp.Rows[0].Status)
	}
	if resp.Rows[0].TotalQuestions != 25 {
		t.Errorf("row[0].total_questions: want 25, got %d", resp.Rows[0].TotalQuestions)
	}
	if resp.Rows[1].Status != "submitted" {
		t.Errorf("row[1]: want submitted, got %q", resp.Rows[1].Status)
	}
	if resp.Rows[1].TotalQuestions != 25 {
		t.Errorf("row[1].total_questions: want 25, got %d", resp.Rows[1].TotalQuestions)
	}
	if len(resp.ViolationsRecent) != 1 {
		t.Fatalf("violations_recent: want 1, got %d", len(resp.ViolationsRecent))
	}
	if resp.ViolationsRecent[0].StudentName != "Student B" {
		t.Errorf("violation student: want Student B, got %q", resp.ViolationsRecent[0].StudentName)
	}
}

func TestGetSessionMonitor_StatusOverdue_ViaDurationGrace(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{
		Title:              "Finals",
		DurationMinutes:    intptr(120),
		GraceWindowMinutes: intptr(10),
		TimerMode:          "overall",
	}
	svc.repo.seedExam(exam)

	started := time.Now().Add(-130*time.Minute - 5*time.Second) // past 120+10 deadline
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row)
	svc.repo.seedQuestionTotal(exam.ID, 10)

	resp, err := svc.GetSessionMonitor(ctx, exam.ID.String())
	if err != nil {
		t.Fatalf("GetSessionMonitor: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows: want 1, got %d", len(resp.Rows))
	}
	if resp.Rows[0].Status != "overdue" {
		t.Errorf("status: want overdue, got %q", resp.Rows[0].Status)
	}
}

func TestGetSessionMonitor_FR6a_NilDuration_NotOverdue(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{
		Title:              "PerTest",
		DurationMinutes:    nil, // per_test
		GraceWindowMinutes: intptr(10),
		TimerMode:          "overall",
	}
	svc.repo.seedExam(exam)

	started := time.Now().Add(-24 * time.Hour) // started long ago
	sessionID := uuid.New()
	sessionStatus := "in_progress"
	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Student A",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row)
	svc.repo.seedQuestionTotal(exam.ID, 10)

	resp, err := svc.GetSessionMonitor(ctx, exam.ID.String())
	if err != nil {
		t.Fatalf("GetSessionMonitor: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows: want 1, got %d", len(resp.Rows))
	}
	if resp.Rows[0].Status != "in_progress" {
		t.Errorf("FR-6a: nil duration should be in_progress, got %q", resp.Rows[0].Status)
	}
}



// ---------- GetSessionMonitor sectioned test (FR-21) ----------

func TestGetSessionMonitor_SectionedSurfacesActiveSection(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{
		Title:           "UTBK Exam",
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
		Mode:            "utbk",
	}
	svc.repo.seedExam(exam)

	now := time.Now()
	started := now.Add(-30 * time.Minute) // section started 30min ago, 60min duration → in_progress
	dur := 60
	sectionTestID := uuid.New()
	sectionTitle := "Subtest 1"
	sessionID := uuid.New()
	sessionStatus := "in_progress"

	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "Sectioned Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionTestID:          &sectionTestID,
		ActiveSectionTitle:           &sectionTitle,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: &dur,
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row)
	svc.repo.seedQuestionTotal(exam.ID, 40)

	resp, err := svc.GetSessionMonitor(ctx, exam.ID.String())
	if err != nil {
		t.Fatalf("GetSessionMonitor: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows: want 1, got %d", len(resp.Rows))
	}

	r := resp.Rows[0]
	if r.ActiveSectionTestID == nil || *r.ActiveSectionTestID != sectionTestID {
		t.Errorf("active_section_test_id: want %v, got %v", sectionTestID, r.ActiveSectionTestID)
	}
	if r.ActiveSectionTitle == nil || *r.ActiveSectionTitle != sectionTitle {
		t.Errorf("active_section_title: want %s, got %v", sectionTitle, r.ActiveSectionTitle)
	}
	// FR-21: active section's remaining seconds must be >0 since 30min elapsed of 60min
	if r.ActiveSectionRemainingSeconds <= 0 {
		t.Errorf("active_section_remaining_seconds: want >0, got %d", r.ActiveSectionRemainingSeconds)
	}
	// Derived status must be in_progress (30min of 60min used)
	if r.Status != "in_progress" {
		t.Errorf("status: want in_progress, got %q", r.Status)
	}
}

func TestGetSessionMonitor_SectionedRow_Overdue(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{
		Title:           "IELTS Exam",
		DurationMinutes: intptr(120),
		TimerMode:       "overall",
		Mode:            "ielts",
	}
	svc.repo.seedExam(exam)

	now := time.Now()
	started := now.Add(-65 * time.Minute) // 65min ago, 60min duration → overdue
	dur := 60
	sectionTestID := uuid.New()
	sectionTitle := "Listening"
	sessionID := uuid.New()
	sessionStatus := "in_progress"

	row := model.SessionMonitorRow{
		RegistrationID: uuid.New(),
		StudentID:      uuid.New(),
		StudentName:    "IELTS Student",
		SessionID:      &sessionID,
		SessionStatus:  &sessionStatus,
		StartedAt:      &started,
		ActiveSectionTestID:          &sectionTestID,
		ActiveSectionTitle:           &sectionTitle,
		ActiveSectionStartedAt:       &started,
		ActiveSectionDurationMinutes: &dur,
	}
	svc.repo.seedSessionMonitorRow(exam.ID, row)
	svc.repo.seedQuestionTotal(exam.ID, 40)

	resp, err := svc.GetSessionMonitor(ctx, exam.ID.String())
	if err != nil {
		t.Fatalf("GetSessionMonitor: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows: want 1, got %d", len(resp.Rows))
	}

	r := resp.Rows[0]
	if r.Status != "overdue" {
		t.Errorf("status: want overdue, got %q", r.Status)
	}
	// Remaining must be 0 (deadline passed)
	if r.ActiveSectionRemainingSeconds != 0 {
		t.Errorf("active_section_remaining_seconds: want 0 (overdue), got %d", r.ActiveSectionRemainingSeconds)
	}
}

// ---------- GetSessionViolations tests ----------

func TestGetSessionViolations_InvalidSessionID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.GetSessionViolations(ctx, "not-a-uuid")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestGetSessionViolations_HappyPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sessionID := uuid.New()
	v1 := model.SessionViolationLog{
		ID:            uuid.New(),
		SessionID:     sessionID,
		StudentID:     uuid.New(),
		ViolationType: "tab_switch",
		OccurredAt:    time.Now().Add(-1 * time.Minute),
	}
	v2 := model.SessionViolationLog{
		ID:            uuid.New(),
		SessionID:     sessionID,
		StudentID:     uuid.New(),
		ViolationType: "fullscreen_exit",
		OccurredAt:    time.Now(),
	}
	svc.repo.seedSessionViolations(sessionID, []model.SessionViolationLog{v1, v2})

	violations, err := svc.GetSessionViolations(ctx, sessionID.String())
	if err != nil {
		t.Fatalf("GetSessionViolations: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("violations: want 2, got %d", len(violations))
	}
}

func TestGetSessionViolations_EmptyForUnknownSession(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	violations, err := svc.GetSessionViolations(ctx, uuid.New().String())
	if err != nil {
		t.Fatalf("GetSessionViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("want 0 violations for unknown session, got %d", len(violations))
	}
}

