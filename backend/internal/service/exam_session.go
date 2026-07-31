package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// ---------- Session result types ----------

type CheckInResult struct {
	RegistrationID uuid.UUID  `json:"registration_id"`
	ExamTitle      string     `json:"exam_title"`
	ScheduledAt    *time.Time `json:"scheduled_at"`
}

type SessionTestPayload struct {
	ID        uuid.UUID         `json:"id"`
	Title     string            `json:"title"`
	Subject   string            `json:"subject"`
	Questions []SessionQuestion `json:"questions"`
	// Sectioned-exam fields (FR-7/FR-16). omitempty keeps standard-mode JSON
	// byte-compatible: these are only populated when exam.Mode is utbk|ielts.
	SectionType      *string `json:"section_type,omitempty"`
	DurationMinutes  *int    `json:"duration_minutes,omitempty"`
	AudioURL         *string `json:"audio_url,omitempty"`
	AudioPlayLimit   *int    `json:"audio_play_limit,omitempty"`
	Status           string  `json:"status,omitempty"`
	RemainingSeconds int64   `json:"remaining_seconds,omitempty"`
}

type SessionStartPayload struct {
	SessionID        uuid.UUID            `json:"session_id"`
	RemainingSeconds int64                `json:"remaining_seconds"`
	TimerMode        string               `json:"timer_mode"`
	DurationMinutes  *int                 `json:"duration_minutes"`
	Tests            []SessionTestPayload `json:"tests"`
	// Sectioned-exam fields (FR-7). omitempty keeps standard-mode JSON byte-compatible.
	Mode         string     `json:"mode,omitempty"`
	ActiveTestID *uuid.UUID `json:"active_test_id,omitempty"`
}

type SessionQuestion struct {
	ID         uuid.UUID          `json:"id"`
	TestID     uuid.UUID          `json:"test_id"`
	Format     string             `json:"format"`
	Body       string             `json:"body"`
	Options    []SessionOption    `json:"options"`
	AudioURL   *string            `json:"audio_url,omitempty"`
	Blanks     []int              `json:"blanks,omitempty"`
	Statements []SessionStatement `json:"statements,omitempty"`
	SortOrder  int                `json:"sort_order"`
}

type SessionOption struct {
	Key       string  `json:"key"`
	Text      string  `json:"text"`
	ImageURL  *string `json:"image_url"`
	SortOrder int     `json:"sort_order"`
}

// SessionStatement is the student-facing statement shape for a true_false
// question: index and body only. IsTrue (the answer key) must never appear
// here (NFR-5).
type SessionStatement struct {
	Index int    `json:"index"`
	Body  string `json:"body"`
}

type SessionStatePayload struct {
	SessionID        uuid.UUID                 `json:"session_id"`
	Status           string                    `json:"status"`
	RemainingSeconds int64                     `json:"remaining_seconds"`
	TimerMode        string                    `json:"timer_mode"`
	DurationMinutes  *int                      `json:"duration_minutes"`
	Tests            []SessionTestPayload      `json:"tests"`
	Answers          []model.ExamSessionAnswer `json:"answers"`
	// CurrentPosition is the last server-persisted question index (FR-35/FR-36);
	// nil when the session has never carried a saved position.
	CurrentPosition *int `json:"current_position"`
	// Sectioned-exam fields (FR-16). omitempty keeps standard-mode JSON byte-compatible.
	Mode         string     `json:"mode,omitempty"`
	ActiveTestID *uuid.UUID `json:"active_test_id,omitempty"`
}

// AdvanceSectionResult is the response for POST /sessions/:id/sections/:testId/advance
// (FR-10/FR-12). It carries the updated per-section timing block (same shape as the
// state payload's tests[]) plus the new active_test_id and a completed flag.
type AdvanceSectionResult struct {
	Mode         string               `json:"mode,omitempty"`
	ActiveTestID *uuid.UUID           `json:"active_test_id,omitempty"`
	Completed    bool                 `json:"completed"`
	Tests        []SessionTestPayload `json:"tests"`
}

type SubmitResult struct {
	Status string   `json:"status"`
	Score  *float64 `json:"score"`
}

type AnswerInput struct {
	QuestionID       uuid.UUID `json:"question_id"`
	Answer           *string   `json:"answer"`
	FlaggedForReview bool      `json:"flagged_for_review"`
}

// fingerprint derives a device fingerprint from IP and User-Agent.
func fingerprint(ip, ua string) string {
	h := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(h[:])
}

// validViolationTypes is the set of allowed violation_type values.
var validViolationTypes = map[string]bool{
	"fullscreen_exit": true,
	"tab_switch":      true,
	"copy_attempt":    true,
}

// ---------- Shared helpers ----------

// groupQuestionsByTest groups questions by their parent test and strips
// correct_answer / is_correct from the student-facing payload. Per-question
// test_id and sort_order come from the owning test context, not from the bank
// question (FR-26). Populates audio_url from the question and blanks (ordered
// indices only, never correct_answer) for multi_blank questions.
func groupQuestionsByTest(tests []model.TestDetail) []SessionTestPayload {
	var out []SessionTestPayload
	for _, td := range tests {
		st := SessionTestPayload{
			ID:        td.Test.ID,
			Title:     td.Test.Title,
			Subject:   td.Test.Subject,
			Questions: make([]SessionQuestion, 0, len(td.Questions)),
		}
		for _, q := range td.Questions {
			sq := SessionQuestion{
				ID:        q.Question.ID,
				TestID:    td.Test.ID,
				Format:    q.Question.Format,
				Body:      q.Question.Body,
				AudioURL:  q.Question.AudioURL,
				SortOrder: q.SortOrder,
			}
			for _, o := range q.Options {
				sq.Options = append(sq.Options, SessionOption{
					Key:       o.Key,
					Text:      o.Text,
					ImageURL:  o.ImageURL,
					SortOrder: o.SortOrder,
				})
			}
			// Populate blanks (ordered indices) for multi_blank questions
			if q.Question.Format == "multi_blank" {
				for _, b := range q.Blanks {
					sq.Blanks = append(sq.Blanks, b.Index)
				}
			}
			// Populate statements (index + body, never is_true) for true_false
			// questions (FR-31, NFR-5).
			if q.Question.Format == "true_false" {
				for _, stmt := range q.Question.Statements {
					sq.Statements = append(sq.Statements, SessionStatement{Index: stmt.Index, Body: stmt.Body})
				}
			}
			st.Questions = append(st.Questions, sq)
		}
		out = append(out, st)
	}
	return out
}

// enrichSectionedTests populates the per-section fields (FR-7/FR-16) on a grouped
// tests payload from the ordered test details and section rows. grouped and tests
// are both ordered by exam_test.sort_order, so they align by index. Returns the
// active section's test_id (nil when no section is active, e.g. all submitted).
func enrichSectionedTests(grouped []SessionTestPayload, tests []model.TestDetail, sections []model.ExamSessionSection) *uuid.UUID {
	sectionByTest := make(map[uuid.UUID]model.ExamSessionSection, len(sections))
	for _, s := range sections {
		sectionByTest[s.TestID] = s
	}
	var activeID *uuid.UUID
	for i := range grouped {
		td := tests[i].Test
		sec := sectionByTest[grouped[i].ID]
		grouped[i].SectionType = td.SectionType
		dur := sec.DurationMinutes
		grouped[i].DurationMinutes = &dur
		grouped[i].AudioURL = td.AudioURL
		grouped[i].AudioPlayLimit = td.AudioPlayLimit
		grouped[i].Status = sec.Status
		// FR-7: remaining_seconds is the section's own remaining; 0 for non-active sections.
		if sec.Status == "active" {
			grouped[i].RemainingSeconds = computeSectionRemaining(sec)
			id := grouped[i].ID
			activeID = &id
		} else {
			grouped[i].RemainingSeconds = 0
		}
	}
	return activeID
}

// computeRemainingSeconds calculates remaining time based on effective deadline.
func computeRemainingSeconds(startedAt time.Time, durationMinutes *int, extendedUntil *time.Time) int64 {
	if durationMinutes == nil || *durationMinutes <= 0 {
		return 0
	}
	deadline := startedAt.Add(time.Duration(*durationMinutes) * time.Minute)
	if extendedUntil != nil && extendedUntil.After(deadline) {
		deadline = *extendedUntil
	}
	return int64(math.Max(0, time.Until(deadline).Seconds()))
}

// computeSectionRemaining computes the remaining seconds for a sectioned-exam
// section per FR-8. The effective deadline is started_at + duration_minutes·60s,
// pushed forward by extended_until when it is later. This is a dedicated path —
// it MUST NOT route through the flat computeRemainingSeconds (which takes the
// exam-level durationMinutes and previously produced an instant-0 auto-submit
// regression — PR#25).
func computeSectionRemaining(section model.ExamSessionSection) int64 {
	if section.StartedAt == nil || section.DurationMinutes <= 0 {
		return 0
	}
	deadline := section.StartedAt.Add(time.Duration(section.DurationMinutes) * time.Minute)
	if section.ExtendedUntil != nil && section.ExtendedUntil.After(deadline) {
		deadline = *section.ExtendedUntil
	}
	return int64(math.Max(0, time.Until(deadline).Seconds()))
}

// ---------- CheckIn ----------

// CheckIn validates the registration token, the check-in window, and stamps
// checked_in_at in a transaction. A device-lock key is written to Redis.
// FR2–FR5.
func (s *Service) CheckIn(ctx context.Context, studentID, token, fp string) (CheckInResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return CheckInResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}

	reg, err := s.storeRepo.GetExamRegistrationByToken(ctx, sid, token)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CheckInResult{}, ErrRegistrationNotFound
		}
		return CheckInResult{}, err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, reg.ExamID)
	if err != nil {
		return CheckInResult{}, err
	}

	if !exam.RequiresCheckin {
		return CheckInResult{}, ErrNotCheckedIn
	}

	// Window check: now ∈ [scheduled_at − window, scheduled_at) by default, or
	// now ∈ [scheduled_at − window, scheduled_end_at] when the exam has an
	// availability window (ScheduledEndAt set) — check-in stays open through
	// the whole window instead of closing the instant the exam "starts".
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

	// Stamped checked_in_at in a transaction
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return CheckInResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CheckInExamTx(ctx, tx, reg.ID); err != nil {
		return CheckInResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CheckInResult{}, err
	}

	// Redis device lock
	key := "exam:device:" + reg.ID.String()
	ttl := 24 * time.Hour
	if exam.DurationMinutes != nil && *exam.DurationMinutes > 0 {
		ttl = time.Duration(*exam.DurationMinutes) * time.Minute
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

// ---------- StartSession ----------

// StartSession creates a new exam session. For requires_checkin=true exams it
// validates the window, check-in status, and device fingerprint. FR6–FR12.
func (s *Service) StartSession(ctx context.Context, studentID, registrationID, fp string) (SessionStartPayload, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SessionStartPayload{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	rid, err := uuid.Parse(registrationID)
	if err != nil {
		return SessionStartPayload{}, fmt.Errorf("%w: invalid registration id", ErrValidation)
	}

	detail, err := s.storeRepo.GetExamRegistrationByID(ctx, rid, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SessionStartPayload{}, ErrRegistrationNotFound
		}
		return SessionStartPayload{}, err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, detail.ExamID)
	if err != nil {
		return SessionStartPayload{}, err
	}

	// Availability window (ScheduledEndAt set): applies regardless of
	// requires_checkin — a window exam is only startable within
	// [scheduled_at, scheduled_end_at], same bound the CheckIn window check uses.
	if exam.ScheduledEndAt != nil {
		now := time.Now()
		if exam.ScheduledAt != nil && now.Before(*exam.ScheduledAt) {
			return SessionStartPayload{}, ErrExamNotStarted
		}
		if now.After(*exam.ScheduledEndAt) {
			return SessionStartPayload{}, ErrExamWindowClosed
		}
	}

	// requires_checkin branch
	if exam.RequiresCheckin {
		if exam.ScheduledAt != nil && time.Now().Before(*exam.ScheduledAt) {
			return SessionStartPayload{}, ErrExamNotStarted
		}
		if detail.CheckedInAt == nil {
			return SessionStartPayload{}, ErrNotCheckedIn
		}

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

	// max_attempts IS NULL or 0 means single-attempt (FR18) — every existing exam row
	// keeps today's behaviour unless an admin sets max_attempts >= 2. Mirrors the
	// WHERE clause's COALESCE(NULLIF(...,0),1) in CreateExamSessionTx; this is a cheap
	// pre-check only, the atomic predicate there stays authoritative (C5, Invariant 4).
	maxAttempts := 1
	if exam.MaxAttempts != nil && *exam.MaxAttempts > 0 {
		maxAttempts = *exam.MaxAttempts
	}
	if detail.AttemptsUsed >= maxAttempts {
		return SessionStartPayload{}, ErrAlreadyAttempted
	}

	isSectioned := exam.Mode == "utbk" || exam.Mode == "ielts"

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return SessionStartPayload{}, err
	}
	defer tx.Rollback(ctx)

	sess, err := s.storeRepo.CreateExamSessionTx(ctx, tx, detail.ExamRegistration, exam.MaxAttempts)
	if err != nil {
		if errors.Is(err, repository.ErrNoAttemptsLeft) {
			return SessionStartPayload{}, ErrAlreadyAttempted
		}
		return SessionStartPayload{}, err
	}

	// FR-5: sectioned start seeds N exam_session_section rows in the same tx as
	// the exam_session row; first (lowest sort_order) is 'active' with
	// started_at=now(), rest are 'pending'.
	if isSectioned {
		sectionTests, err := s.storeRepo.GetSessionWithQuestions(ctx, detail.ExamID)
		if err != nil {
			return SessionStartPayload{}, err
		}
		sections := make([]model.ExamSessionSection, 0, len(sectionTests))
		now := time.Now()
		for i, td := range sectionTests {
			st := "pending"
			var startedAt *time.Time
			if i == 0 {
				st = "active"
				sa := now
				startedAt = &sa
			}
			sections = append(sections, model.ExamSessionSection{
				TestID:          td.Test.ID,
				SortOrder:       i,
				DurationMinutes: td.Test.DurationMinutes,
				Status:          st,
				StartedAt:       startedAt,
			})
		}
		if err := s.storeRepo.CreateSessionSectionsTx(ctx, tx, sess.ID, sections); err != nil {
			return SessionStartPayload{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SessionStartPayload{}, err
	}

	// Load questions
	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, detail.ExamID)
	if err != nil {
		return SessionStartPayload{}, err
	}
	grouped := groupQuestionsByTest(tests)

	if isSectioned {
		sections, err := s.storeRepo.GetSessionSections(ctx, sess.ID)
		if err != nil {
			return SessionStartPayload{}, err
		}
		activeID := enrichSectionedTests(grouped, tests, sections)
		// Top-level remaining_seconds mirrors the active section's remaining so
		// the field stays meaningful for sectioned mode (it is not omitempty).
		var topRemaining int64
		if activeID != nil {
			for _, tp := range grouped {
				if tp.ID == *activeID {
					topRemaining = tp.RemainingSeconds
					break
				}
			}
		}
		return SessionStartPayload{
			SessionID:        sess.ID,
			RemainingSeconds: topRemaining,
			TimerMode:        exam.TimerMode,
			Tests:            grouped,
			Mode:             exam.Mode,
			ActiveTestID:     activeID,
		}, nil
	}

	remaining := computeRemainingSeconds(sess.StartedAt, exam.DurationMinutes, nil)

	return SessionStartPayload{
		SessionID:        sess.ID,
		RemainingSeconds: remaining,
		TimerMode:        exam.TimerMode,
		DurationMinutes:  exam.DurationMinutes,
		Tests:            grouped,
	}, nil
}

// ---------- ReconnectSession ----------

// ReconnectSession returns the current session state, questions, saved answers,
// and remaining time. FR13–FR14.
func (s *Service) ReconnectSession(ctx context.Context, studentID, sessionID string) (SessionStatePayload, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SessionStatePayload{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SessionStatePayload{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SessionStatePayload{}, ErrSessionNotFound
		}
		return SessionStatePayload{}, err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return SessionStatePayload{}, err
	}

	remaining := computeRemainingSeconds(sess.StartedAt, exam.DurationMinutes, sess.ExtendedUntil)

	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return SessionStatePayload{}, err
	}
	grouped := groupQuestionsByTest(tests)

	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return SessionStatePayload{}, err
	}

	if exam.Mode == "utbk" || exam.Mode == "ielts" {
		sections, err := s.storeRepo.GetSessionSections(ctx, sessID)
		if err != nil {
			return SessionStatePayload{}, err
		}
		activeID := enrichSectionedTests(grouped, tests, sections)
		var topRemaining int64
		if activeID != nil {
			for _, tp := range grouped {
				if tp.ID == *activeID {
					topRemaining = tp.RemainingSeconds
					break
				}
			}
		}
		return SessionStatePayload{
			SessionID:        sess.ID,
			Status:           sess.Status,
			RemainingSeconds: topRemaining,
			TimerMode:        exam.TimerMode,
			Tests:            grouped,
			Answers:          answers,
			CurrentPosition:  sess.CurrentPosition,
			Mode:             exam.Mode,
			ActiveTestID:     activeID,
		}, nil
	}

	return SessionStatePayload{
		SessionID:        sess.ID,
		Status:           sess.Status,
		RemainingSeconds: remaining,
		TimerMode:        exam.TimerMode,
		DurationMinutes:  exam.DurationMinutes,
		Tests:            grouped,
		Answers:          answers,
		CurrentPosition:  sess.CurrentPosition,
	}, nil
}

// ---------- SaveAnswers ----------

// SaveAnswers upserts the student's answers for a session. FR15–FR16.
// For sectioned exams (FR-14), a save targeting any question in a non-active
// section is rejected with ErrSectionLocked; standard mode skips the guard
// entirely (FR-15).
// position is optional (FR-35): when non-nil it must fall within
// [0, total_questions) for the exam or the whole request is rejected with
// ErrInvalidPosition before any answer or position is written (FR-36 relies on
// this field never holding a value the client couldn't have reached).
func (s *Service) SaveAnswers(ctx context.Context, studentID, sessionID string, inputs []AnswerInput, position *int) error {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	if sess.Status != "in_progress" {
		return ErrAlreadySubmitted
	}

	// FR-14/FR-15: sectioned-mode guard. Reject any answer whose question belongs
	// to a Test whose section is not 'active'. Standard mode skips the guard.
	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return err
	}
	if exam.Mode == "utbk" || exam.Mode == "ielts" {
		tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return err
		}
		sections, err := s.storeRepo.GetSessionSections(ctx, sessID)
		if err != nil {
			return err
		}
		sectionByTest := make(map[uuid.UUID]string, len(sections))
		for _, sec := range sections {
			sectionByTest[sec.TestID] = sec.Status
		}
		questionTest := make(map[uuid.UUID]uuid.UUID)
		for _, td := range tests {
			for _, q := range td.Questions {
				questionTest[q.Question.ID] = td.Test.ID
			}
		}
		for _, in := range inputs {
			tid, ok := questionTest[in.QuestionID]
			if !ok {
				return fmt.Errorf("%w: question not part of this exam", ErrValidation)
			}
			if sectionByTest[tid] != "active" {
				return ErrSectionLocked
			}
		}
	}

	if position != nil {
		total, err := s.storeRepo.GetExamQuestionTotal(ctx, sess.ExamID)
		if err != nil {
			return err
		}
		if *position < 0 || *position >= total {
			return fmt.Errorf("%w: position must be within [0, %d)", ErrInvalidPosition, total)
		}
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

	return s.storeRepo.SaveAnswersTx(ctx, sessID, answers, position)
}

// ---------- AdvanceSection ----------

// AdvanceSection closes the active section and promotes the next pending one
// (FR-10/FR-11/FR-12). Idempotent on already-submitted sections (200 no-op);
// rejected with ErrSectionNotActive when the target section is pending. The
// repo's atomic WHERE status='active' guard (NFR-5) makes double-fire safe;
// the service disambiguates the 0-row result via the section's true status.
func (s *Service) AdvanceSection(ctx context.Context, studentID, sessionID, testID string) (AdvanceSectionResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return AdvanceSectionResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return AdvanceSectionResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}
	tid, err := uuid.Parse(testID)
	if err != nil {
		return AdvanceSectionResult{}, fmt.Errorf("%w: invalid test id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return AdvanceSectionResult{}, ErrSessionNotFound
		}
		return AdvanceSectionResult{}, err
	}
	if sess.Status != "in_progress" {
		return AdvanceSectionResult{}, ErrAlreadySubmitted
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return AdvanceSectionResult{}, err
	}
	if exam.Mode != "utbk" && exam.Mode != "ielts" {
		return AdvanceSectionResult{}, fmt.Errorf("%w: advance only applies to sectioned exams", ErrValidation)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return AdvanceSectionResult{}, err
	}
	defer tx.Rollback(ctx)

	_, err = s.storeRepo.AdvanceSessionSectionTx(ctx, tx, sessID, tid)
	if err != nil {
		if errors.Is(err, repository.ErrNoActiveSection) {
			// FR-11 disambiguation: 0 rows means either already-submitted
			// (idempotent 200 no-op) or still-pending (ErrSectionNotActive).
			// Read the section's true status via pool (the tx rolls back via defer).
			sections, rerr := s.storeRepo.GetSessionSections(ctx, sessID)
			if rerr != nil {
				return AdvanceSectionResult{}, rerr
			}
			for _, sec := range sections {
				if sec.TestID == tid {
					if sec.Status == "submitted" {
						return s.buildAdvanceResult(ctx, exam, sessID)
					}
					return AdvanceSectionResult{}, ErrSectionNotActive
				}
			}
			return AdvanceSectionResult{}, ErrSectionNotActive
		}
		return AdvanceSectionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdvanceSectionResult{}, err
	}

	return s.buildAdvanceResult(ctx, exam, sessID)
}

// buildAdvanceResult assembles the advance response from the current persisted
// section + test state. completed is true when no section is active (last
// section just closed — FR-12).
func (s *Service) buildAdvanceResult(ctx context.Context, exam *model.Exam, sessID uuid.UUID) (AdvanceSectionResult, error) {
	sections, err := s.storeRepo.GetSessionSections(ctx, sessID)
	if err != nil {
		return AdvanceSectionResult{}, err
	}
	tests, err := s.storeRepo.GetSessionWithQuestions(ctx, exam.ID)
	if err != nil {
		return AdvanceSectionResult{}, err
	}
	grouped := groupQuestionsByTest(tests)
	activeID := enrichSectionedTests(grouped, tests, sections)
	return AdvanceSectionResult{
		Mode:         exam.Mode,
		ActiveTestID: activeID,
		Completed:    activeID == nil,
		Tests:        grouped,
	}, nil
}

// ---------- SubmitSession ----------

// SubmitSession grades objective answers and marks the session as submitted.
// FR17–FR20.
func (s *Service) SubmitSession(ctx context.Context, studentID, sessionID string) (SubmitResult, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitResult{}, ErrSessionNotFound
		}
		return SubmitResult{}, err
	}

	// A failed load must abort the submit: grading against a partial/empty question
	// set would CAS-submit the student's only attempt with a wrong score.
	questions, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return SubmitResult{}, err
	}
	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return SubmitResult{}, err
	}

	answerMap := make(map[uuid.UUID]*string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.Answer
	}

	var qs []model.QuestionWithOptions
	for _, td := range questions {
		qs = append(qs, td.Questions...)
	}

	graded, score := gradeObjective(qs, answerMap)

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return SubmitResult{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := s.storeRepo.SubmitSessionTx(ctx, tx, sessID, graded, score, false)
	if err != nil {
		return SubmitResult{}, err
	}
	if rows == 0 {
		return SubmitResult{}, ErrAlreadySubmitted
	}

	if err := tx.Commit(ctx); err != nil {
		return SubmitResult{}, err
	}

	return SubmitResult{
		Status: "submitted",
		Score:  &score,
	}, nil
}

// ---------- LogViolation ----------

// LogViolation records an integrity event. FR21–FR22.
func (s *Service) LogViolation(ctx context.Context, studentID, sessionID, violationType string) error {
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

	sess, err := s.storeRepo.GetExamSessionForStudent(ctx, sessID, sid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	if sess.Status != "in_progress" {
		return ErrAlreadySubmitted
	}

	return s.storeRepo.LogViolation(ctx, model.SessionViolationLog{
		SessionID:     sessID,
		StudentID:     sid,
		ViolationType: violationType,
		OccurredAt:    time.Now(),
	})
}

// ---------- ReopenSession (admin) ----------

// ReopenSession extends a session's deadline or the active section's deadline for
// sectioned exams. FR-22 / FR23.
func (s *Service) ReopenSession(ctx context.Context, sessionID string, minutes int) error {
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionByID(ctx, sessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return err
	}

	// FR-22: sectioned path — extend the active section, not the session-level deadline.
	if exam.Mode == "utbk" || exam.Mode == "ielts" {
		tx, err := s.storeRepo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		if err := s.storeRepo.ExtendActiveSectionTx(ctx, tx, sessID, minutes); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Standard path — extend session-level extended_until.
	if err := s.storeRepo.ReopenSession(ctx, sessID, minutes); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	return nil
}

// ---------- ForceSubmitSession (admin) ----------

// ForceSubmitSession grades and submits an in-progress session as admin. FR24.
func (s *Service) ForceSubmitSession(ctx context.Context, sessionID string) (SubmitResult, error) {
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: invalid session id", ErrValidation)
	}

	sess, err := s.storeRepo.GetExamSessionByID(ctx, sessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitResult{}, ErrSessionNotFound
		}
		return SubmitResult{}, err
	}

	// Same guard as SubmitSession: never grade against a failed load.
	questions, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
	if err != nil {
		return SubmitResult{}, err
	}
	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return SubmitResult{}, err
	}

	answerMap := make(map[uuid.UUID]*string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.Answer
	}

	var qs []model.QuestionWithOptions
	for _, td := range questions {
		qs = append(qs, td.Questions...)
	}

	graded, score := gradeObjective(qs, answerMap)

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return SubmitResult{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := s.storeRepo.SubmitSessionTx(ctx, tx, sessID, graded, score, true)
	if err != nil {
		return SubmitResult{}, err
	}
	if rows == 0 {
		return SubmitResult{}, ErrAlreadySubmitted
	}

	if err := tx.Commit(ctx); err != nil {
		return SubmitResult{}, err
	}

	return SubmitResult{
		Status: "submitted",
		Score:  &score,
	}, nil
}

// ---------- Session monitor helpers ----------

// effectiveDeadline computes the overdue threshold for a session.
// When durationMinutes is nil (per_test), no duration-based deadline applies
// (FR-6a) — only extended_until can push the deadline forward. The zero
// time.Time return signals "no deadline" to the caller.
func effectiveDeadline(startedAt time.Time, durationMinutes *int, graceMinutes *int, extendedUntil *time.Time) time.Time {
	var deadline time.Time

	if durationMinutes != nil && *durationMinutes > 0 {
		deadline = startedAt.Add(time.Duration(*durationMinutes) * time.Minute)
		if graceMinutes != nil {
			deadline = deadline.Add(time.Duration(*graceMinutes) * time.Minute)
		}
	}

	if extendedUntil != nil && !extendedUntil.IsZero() {
		if deadline.IsZero() || extendedUntil.After(deadline) {
			deadline = *extendedUntil
		}
	}

	return deadline
}

// deriveStatus sets the monitor row's derived status per FR-3.
// When active-section data is present (FR-20), the deadline is computed from the
// section's own started_at + duration_minutes + extended_until, not the exam-level clock.
// Standard sessions (no active-section data) use the existing exam-level path unchanged.
func deriveStatus(row model.SessionMonitorRow, now time.Time, durationMinutes *int, graceMinutes *int) string {
	if row.SessionID == nil {
		if row.CheckedInAt != nil {
			return "checked_in"
		}
		return "registered"
	}

	if row.SessionStatus != nil && *row.SessionStatus == "submitted" {
		return "submitted"
	}

	// FR-20: sectioned path — use active section's deadline.
	if row.ActiveSectionStartedAt != nil {
		deadline := effectiveDeadline(*row.ActiveSectionStartedAt, row.ActiveSectionDurationMinutes, nil, row.ActiveSectionExtendedUntil)
		if deadline.IsZero() {
			return "in_progress"
		}
		if now.After(deadline) {
			return "overdue"
		}
		return "in_progress"
	}

	// Session is in_progress — check standard exam-level deadline.
	deadline := effectiveDeadline(*row.StartedAt, durationMinutes, graceMinutes, row.ExtendedUntil)
	if deadline.IsZero() {
		return "in_progress"
	}
	if now.After(deadline) {
		return "overdue"
	}
	return "in_progress"
}

// ---------- GetSessionMonitor ----------

// GetSessionMonitor returns the monitor payload for an exam: the exam summary,
// one row per registrant with derived status, and recent violations. FR-1–FR-7.
func (s *Service) GetSessionMonitor(ctx context.Context, examID string) (model.SessionMonitorResponse, error) {
	eid, err := uuid.Parse(examID)
	if err != nil {
		return model.SessionMonitorResponse{}, fmt.Errorf("%w: invalid exam id", ErrValidation)
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, eid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.SessionMonitorResponse{}, ErrExamNotFound
		}
		return model.SessionMonitorResponse{}, err
	}

	rows, err := s.storeRepo.GetSessionMonitorRows(ctx, eid)
	if err != nil {
		return model.SessionMonitorResponse{}, err
	}

	totalQ, err := s.storeRepo.GetExamQuestionTotal(ctx, eid)
	if err != nil {
		return model.SessionMonitorResponse{}, err
	}

	recentV, err := s.storeRepo.GetRecentViolations(ctx, eid, 20)
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

// ---------- GetSessionViolations ----------

// GetSessionViolations returns the violation log for a session, newest-first. FR-8.
func (s *Service) GetSessionViolations(ctx context.Context, sessionID string) ([]model.SessionViolationLog, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid session id", ErrValidation)
	}
	return s.storeRepo.ListSessionViolations(ctx, sid)
}
