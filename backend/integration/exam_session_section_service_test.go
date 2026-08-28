package integration_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

type saveAnswersQueryTracer struct {
	mu      sync.Mutex
	queries []string
}

func (t *saveAnswersQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	t.mu.Lock()
	t.queries = append(t.queries, data.SQL)
	t.mu.Unlock()
	return ctx
}

func (t *saveAnswersQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *saveAnswersQueryTracer) reset() {
	t.mu.Lock()
	t.queries = nil
	t.mu.Unlock()
}

func (t *saveAnswersQueryTracer) countMatching(text string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, query := range t.queries {
		if strings.Contains(query, text) {
			count++
		}
	}
	return count
}

func (t *saveAnswersQueryTracer) total() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.queries)
}

func tracedSaveAnswersService(t *testing.T, env *testEnv) (*service.Service, *saveAnswersQueryTracer) {
	t.Helper()
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(env.pool.Config().ConnString())
	require.NoError(t, err)
	tracer := &saveAnswersQueryTracer{}
	poolConfig.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	repo := repository.New(pool)
	return service.NewWithStore(
		repo, repo, env.rdb, nil,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, nil, nil,
	), tracer
}

// seedSectionedExamForService seeds a student, a sectioned exam (mode), N tests
// (each with one mcq question), exam_test links ordered by sort_order, and a
// registration with attempts_used=0 ready for svc.StartSession. Returns the
// studentID and registrationID (as strings for the service call).
func seedSectionedExamForService(t *testing.T, env *testEnv, mode string, sectionDurations []int) (studentID, registrationID string, testIDs []string) {
	t.Helper()
	ctx := context.Background()

	studentID = seedUser(t, env, "student", "active", false)

	var examID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO exam (title, is_free, requires_checkin, timer_mode, result_config, mode)
		 VALUES ($1, true, false, 'per_test', 'hidden', $2) RETURNING id`,
		"Sectioned Exam "+uuid.NewString()[:8], mode,
	).Scan(&examID)
	require.NoError(t, err)

	for i, dur := range sectionDurations {
		var stType *string
		if mode == "ielts" {
			types := []string{"listening", "reading", "writing"}
			st := types[i%len(types)]
			stType = &st
		}
		var testID string
		err = env.pool.QueryRow(ctx,
			`INSERT INTO test (title, subject, topic, duration_minutes, section_type)
			 VALUES ($1, 'umum', 'penalaran', $2, $3) RETURNING id`,
			"Section "+uuid.NewString()[:8], dur, stType,
		).Scan(&testID)
		require.NoError(t, err)
		testIDs = append(testIDs, testID)

		// One mcq question per test so SaveAnswers has a target.
		var qID string
		err = env.pool.QueryRow(ctx,
			`INSERT INTO question (format, body, point_correct, point_wrong)
			 VALUES ('mcq', $1, 1, 0) RETURNING id`,
			"Q body "+uuid.NewString()[:8],
		).Scan(&qID)
		require.NoError(t, err)

		_, err = env.pool.Exec(ctx,
			`INSERT INTO question_option (question_id, key, text, is_correct, sort_order)
			 VALUES ($1, 'a', 'opt a', true, 0), ($1, 'b', 'opt b', false, 1)`,
			qID,
		)
		require.NoError(t, err)

		_, err = env.pool.Exec(ctx,
			`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 0)`,
			testID, qID,
		)
		require.NoError(t, err)

		_, err = env.pool.Exec(ctx,
			`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
			examID, testID, i,
		)
		require.NoError(t, err)
	}

	token := "tok-" + uuid.NewString()[:8]
	err = env.pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		 VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, token,
	).Scan(&registrationID)
	require.NoError(t, err)
	return studentID, registrationID, testIDs
}

// TestService_SectionedStart_SeedsSections verifies FR-5/FR-7/FR-9: a sectioned
// (utbk) start creates N exam_session_section rows, the first is active with
// remaining_seconds ≈ duration·60 > 0, the rest are pending with 0 remaining.
func TestService_SectionedStart_SeedsSections(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repo := repository.New(env.pool)

	studentID, regID, testIDs := seedSectionedExamForService(t, env, "utbk", []int{30, 45})
	require.Len(t, testIDs, 2)

	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	// FR-7: payload carries mode + active_test_id.
	assert.Equal(t, "utbk", start.Mode)
	require.NotNil(t, start.ActiveTestID, "active_test_id must be set for sectioned start")
	assert.Equal(t, testIDs[0], start.ActiveTestID.String())

	// FR-5: exactly N section rows.
	sections, err := repo.GetSessionSections(ctx, start.SessionID)
	require.NoError(t, err)
	require.Len(t, sections, 2)
	assert.Equal(t, "active", sections[0].Status)
	assert.NotNil(t, sections[0].StartedAt)
	assert.Equal(t, "pending", sections[1].Status)
	assert.Nil(t, sections[1].StartedAt)

	// FR-9: first section remaining ≈ 30·60 = 1800s, strictly > 0.
	require.Len(t, start.Tests, 2)
	assert.Equal(t, "active", start.Tests[0].Status)
	assert.Greater(t, start.Tests[0].RemainingSeconds, int64(0), "FR-9: first active remaining must be > 0")
	assert.LessOrEqual(t, start.Tests[0].RemainingSeconds, int64(30*60), "remaining must not exceed full duration")
	assert.Equal(t, "pending", start.Tests[1].Status)
	assert.Equal(t, int64(0), start.Tests[1].RemainingSeconds)

	// duration_minutes is the section's own, not the exam-level one.
	require.NotNil(t, start.Tests[0].DurationMinutes)
	assert.Equal(t, 30, *start.Tests[0].DurationMinutes)
	require.NotNil(t, start.Tests[1].DurationMinutes)
	assert.Equal(t, 45, *start.Tests[1].DurationMinutes)
}

// TestService_AdvanceSection_PromotesAndIdempotent verifies FR-10/FR-11/FR-12:
// advance closes the active section, promotes the next with started_at stamped
// and remaining > 0; advance on an already-submitted section is a 200 no-op;
// advance on a pending section returns ErrSectionNotActive; advancing the last
// section returns completed=true with active_test_id=nil.
func TestService_AdvanceSection_PromotesAndIdempotent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repo := repository.New(env.pool)

	studentID, regID, testIDs := seedSectionedExamForService(t, env, "utbk", []int{30, 45, 20})
	require.Len(t, testIDs, 3)

	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	// Advance a pending section (testIDs[1]) -> ErrSectionNotActive (FR-11).
	_, err = env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[1])
	assert.ErrorIs(t, err, service.ErrSectionNotActive)

	// Advance the active section (testIDs[0]) -> promotes testIDs[1].
	adv1, err := env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[0])
	require.NoError(t, err)
	assert.Equal(t, "utbk", adv1.Mode)
	require.NotNil(t, adv1.ActiveTestID)
	assert.Equal(t, testIDs[1], adv1.ActiveTestID.String())
	assert.False(t, adv1.Completed, "not completed when a next section was promoted")
	require.Len(t, adv1.Tests, 3)
	// Promoted section is active with remaining > 0 (FR-9 after advance).
	found1 := false
	for _, tp := range adv1.Tests {
		if tp.ID.String() == testIDs[1] {
			assert.Equal(t, "active", tp.Status)
			assert.Greater(t, tp.RemainingSeconds, int64(0), "promoted section remaining must be > 0")
			assert.LessOrEqual(t, tp.RemainingSeconds, int64(45*60))
			found1 = true
		}
	}
	require.True(t, found1, "promoted section must appear in tests[]")

	// Verify persisted: section 0 submitted, section 1 active with started_at.
	sections, err := repo.GetSessionSections(ctx, start.SessionID)
	require.NoError(t, err)
	require.Len(t, sections, 3)
	assert.Equal(t, "submitted", sections[0].Status)
	assert.NotNil(t, sections[0].SubmittedAt)
	assert.Equal(t, "active", sections[1].Status)
	assert.NotNil(t, sections[1].StartedAt)
	assert.Equal(t, "pending", sections[2].Status)

	// Advance on already-submitted (testIDs[0]) -> 200 no-op (FR-11 idempotent).
	advNoop, err := env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[0])
	require.NoError(t, err, "double-advance on submitted must be a 200 no-op")
	assert.NotNil(t, advNoop.ActiveTestID, "no-op must still return current active_test_id")
	assert.Equal(t, testIDs[1], advNoop.ActiveTestID.String())

	// Advance testIDs[1] (active) -> promotes testIDs[2].
	adv2, err := env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[1])
	require.NoError(t, err)
	require.NotNil(t, adv2.ActiveTestID)
	assert.Equal(t, testIDs[2], adv2.ActiveTestID.String())
	assert.False(t, adv2.Completed)

	// Advance the last section (testIDs[2]) -> completed=true, active_test_id=nil (FR-12).
	advLast, err := env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[2])
	require.NoError(t, err)
	assert.Nil(t, advLast.ActiveTestID, "active_test_id must be nil when last section completes")
	assert.True(t, advLast.Completed, "completed must be true when last section closes")
}

// TestService_SaveAnswers_SectionGuard verifies FR-14/FR-15: a save targeting a
// non-active section's question is rejected with ErrSectionLocked; a save to
// the active section succeeds; a standard-mode save skips the guard.
func TestService_SaveAnswers_SectionGuard(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	studentID, regID, testIDs := seedSectionedExamForService(t, env, "utbk", []int{30, 45})
	require.Len(t, testIDs, 2)

	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	// Question IDs come straight from the start payload (groupQuestionsByTest).
	require.Len(t, start.Tests, 2)
	require.Len(t, start.Tests[0].Questions, 1)
	require.Len(t, start.Tests[1].Questions, 1)
	qActive := start.Tests[0].Questions[0].ID
	qPending := start.Tests[1].Questions[0].ID

	// Save to the active section (testIDs[0]) -> succeeds.
	ans := "a"
	err = env.svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{
		{QuestionID: qActive, Answer: &ans},
	}, nil)
	require.NoError(t, err)

	// Save to the pending section (testIDs[1]) -> ErrSectionLocked.
	err = env.svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{
		{QuestionID: qPending, Answer: &ans},
	}, nil)
	assert.ErrorIs(t, err, service.ErrSectionLocked)

	// Mixed batch (active + pending) -> whole batch rejected.
	err = env.svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{
		{QuestionID: qActive, Answer: &ans},
		{QuestionID: qPending, Answer: &ans},
	}, nil)
	assert.ErrorIs(t, err, service.ErrSectionLocked)
}

func TestService_SaveAnswers_QuestionOutsideExamRejected(t *testing.T) {
	for _, mode := range []string{"utbk", "standard"} {
		t.Run(mode, func(t *testing.T) {
			env := newTestEnv(t)
			ctx := context.Background()

			studentID, regID, _ := seedSectionedExamForService(t, env, mode, []int{30})
			start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
			require.NoError(t, err)

			var outsideQuestionID uuid.UUID
			err = env.pool.QueryRow(ctx,
				`INSERT INTO question (format, body, point_correct, point_wrong)
				 VALUES ('mcq', $1, 1, 0) RETURNING id`,
				"Outside question "+uuid.NewString()[:8],
			).Scan(&outsideQuestionID)
			require.NoError(t, err)

			answer := "a"
			err = env.svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{{
				QuestionID: outsideQuestionID,
				Answer:     &answer,
			}}, nil)
			require.ErrorIs(t, err, service.ErrValidation)
			assert.ErrorContains(t, err, "question not part of this exam")

			var answerCount int
			err = env.pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM exam_session_answer WHERE session_id = $1`, start.SessionID,
			).Scan(&answerCount)
			require.NoError(t, err)
			assert.Zero(t, answerCount)
		})
	}
}

func TestService_SaveAnswers_PositionOnlySkipsSectionGuardQueries(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	studentID, regID, _ := seedSectionedExamForService(t, env, "utbk", []int{30})
	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	svc, tracer := tracedSaveAnswersService(t, env)
	position := 0
	tracer.reset()
	err = svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{}, &position)
	require.NoError(t, err)
	assert.Zero(t, tracer.countMatching("FROM exam_session_section"))
	assert.Zero(t, tracer.countMatching("SELECT tq.question_id, tq.test_id"))

	var currentPosition *int
	err = env.pool.QueryRow(ctx,
		`SELECT current_position FROM exam_session WHERE id = $1`, start.SessionID,
	).Scan(&currentPosition)
	require.NoError(t, err)
	require.NotNil(t, currentPosition)
	assert.Equal(t, position, *currentPosition)
}

func TestService_SaveAnswers_InvalidPositionRejectsWholeBatch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	studentID, regID, _ := seedSectionedExamForService(t, env, "utbk", []int{30})
	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)
	require.Len(t, start.Tests, 1)
	require.Len(t, start.Tests[0].Questions, 1)

	answer := "a"
	invalidPosition := 1
	err = env.svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{{
		QuestionID: start.Tests[0].Questions[0].ID,
		Answer:     &answer,
	}}, &invalidPosition)
	require.ErrorIs(t, err, service.ErrInvalidPosition)

	var currentPosition *int
	err = env.pool.QueryRow(ctx,
		`SELECT current_position FROM exam_session WHERE id = $1`, start.SessionID,
	).Scan(&currentPosition)
	require.NoError(t, err)
	assert.Nil(t, currentPosition)

	var answerCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session_answer WHERE session_id = $1`, start.SessionID,
	).Scan(&answerCount)
	require.NoError(t, err)
	assert.Zero(t, answerCount)
}

func TestService_SaveAnswers_StatementBudget(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       string
		sections   []int
		maxQueries int
	}{
		{name: "standard", mode: "standard", sections: []int{30}, maxQueries: 5},
		{name: "sectioned", mode: "utbk", sections: []int{30, 30, 30}, maxQueries: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			ctx := context.Background()
			studentID, regID, _ := seedSectionedExamForService(t, env, tc.mode, tc.sections)
			start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
			require.NoError(t, err)
			require.NotEmpty(t, start.Tests)
			require.Len(t, start.Tests[0].Questions, 1)

			svc, tracer := tracedSaveAnswersService(t, env)
			answer := "a"
			tracer.reset()
			err = svc.SaveAnswers(ctx, studentID, start.SessionID.String(), []service.AnswerInput{{
				QuestionID: start.Tests[0].Questions[0].ID,
				Answer:     &answer,
			}}, nil)
			require.NoError(t, err)
			assert.LessOrEqual(t, tracer.total(), tc.maxQueries)
		})
	}
}

// TestService_StandardMode_Regression verifies FR-6/FR-15: a standard-mode
// start/save/reconnect is byte-for-byte unchanged — no mode/active_test_id
// fields in the JSON, no exam_session_section rows, remaining computed by the
// flat path.
func TestService_StandardMode_Regression(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repo := repository.New(env.pool)

	studentID, regID, _ := seedSectionedExamForService(t, env, "standard", []int{30})
	// Override the exam to standard timer_mode overall + duration_minutes at exam level.
	// (seedSectionedExamForService set timer_mode='per_test'; for a pure standard
	// regression we want the classic overall-timer shape. We re-seed manually below.)

	// Build a classic standard exam: timer_mode=overall, duration_minutes=60, no sections.
	student2 := seedUser(t, env, "student", "active", false)
	var examID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO exam (title, is_free, requires_checkin, timer_mode, duration_minutes, result_config, mode)
		 VALUES ($1, true, false, 'overall', 60, 'hidden', 'standard') RETURNING id`,
		"Standard Exam "+uuid.NewString()[:8],
	).Scan(&examID)
	require.NoError(t, err)

	var testID string
	err = env.pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'umum', 'penalaran', 30) RETURNING id`,
		"Std Test "+uuid.NewString()[:8],
	).Scan(&testID)
	require.NoError(t, err)

	var qID string
	err = env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		 VALUES ('mcq', 'q', 1, 0) RETURNING id`,
	).Scan(&qID)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO question_option (question_id, key, text, is_correct, sort_order) VALUES ($1,'a','a',true,0),($1,'b','b',false,1)`,
		qID,
	)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 0)`,
		testID, qID,
	)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 0)`,
		examID, testID,
	)
	require.NoError(t, err)

	token := "tok-" + uuid.NewString()[:8]
	var regID2 string
	err = env.pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		 VALUES ($1, $2, $3, 'registered') RETURNING id`,
		student2, examID, token,
	).Scan(&regID2)
	require.NoError(t, err)

	start, err := env.svc.StartSession(ctx, student2, regID2, "fp")
	require.NoError(t, err)

	// FR-6: no exam_session_section rows for standard.
	sections, err := repo.GetSessionSections(ctx, start.SessionID)
	require.NoError(t, err)
	assert.Empty(t, sections, "standard mode must not create section rows")

	// FR-6: remaining computed by the flat path (> 0, under 60·60).
	assert.Greater(t, start.RemainingSeconds, int64(0))
	assert.LessOrEqual(t, start.RemainingSeconds, int64(60*60))

	// FR-6/NFR-4: mode and active_test_id must be absent from JSON (omitempty).
	// We assert by checking the Go zero values — Mode is "", ActiveTestID is nil.
	assert.Empty(t, start.Mode, "standard start must not set mode")
	assert.Nil(t, start.ActiveTestID, "standard start must not set active_test_id")

	// Reconnect: same regression — no mode/active_test_id, flat remaining.
	state, err := env.svc.ReconnectSession(ctx, student2, start.SessionID.String())
	require.NoError(t, err)
	assert.Empty(t, state.Mode)
	assert.Nil(t, state.ActiveTestID)
	assert.Greater(t, state.RemainingSeconds, int64(0))

	// SaveAnswers: standard skips the section guard (FR-15).
	quid, err := uuid.Parse(qID)
	require.NoError(t, err)
	ans := "a"
	err = env.svc.SaveAnswers(ctx, student2, start.SessionID.String(), []service.AnswerInput{
		{QuestionID: quid, Answer: &ans},
	}, nil)
	require.NoError(t, err)

	// Suppress unused-var from the first seed helper (keeps the compiler happy
	// when the standard-only assertions above are the real subject).
	_ = studentID
	_ = regID
}

// TestService_ReconnectSectioned_ReflectsSectionTruth verifies FR-16: reconnect
// reports the current active_test_id and per-section status with remaining
// computed from the active section's server-side started_at (FR-8), and never
// reopens a submitted section.
func TestService_ReconnectSectioned_ReflectsSectionTruth(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	studentID, regID, testIDs := seedSectionedExamForService(t, env, "utbk", []int{30, 45})
	require.Len(t, testIDs, 2)

	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	// Advance to the second section before reconnecting.
	_, err = env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[0])
	require.NoError(t, err)

	state, err := env.svc.ReconnectSession(ctx, studentID, start.SessionID.String())
	require.NoError(t, err)

	assert.Equal(t, "utbk", state.Mode)
	require.NotNil(t, state.ActiveTestID)
	assert.Equal(t, testIDs[1], state.ActiveTestID.String())

	require.Len(t, state.Tests, 2)
	// First section submitted, 0 remaining; second active, remaining > 0.
	assert.Equal(t, "submitted", state.Tests[0].Status)
	assert.Equal(t, int64(0), state.Tests[0].RemainingSeconds)
	assert.Equal(t, "active", state.Tests[1].Status)
	assert.Greater(t, state.Tests[1].RemainingSeconds, int64(0), "active section remaining must be > 0 on reconnect")
	assert.LessOrEqual(t, state.Tests[1].RemainingSeconds, int64(45*60))
}

// TestService_AdvanceSection_StandardMode_Rejected verifies advance on a
// standard-mode session is rejected (the endpoint only applies to sectioned
// exams).
func TestService_AdvanceSection_StandardMode_Rejected(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	studentID, regID, testIDs := seedSectionedExamForService(t, env, "standard", []int{30})
	require.Len(t, testIDs, 1)

	start, err := env.svc.StartSession(ctx, studentID, regID, "fp")
	require.NoError(t, err)

	_, err = env.svc.AdvanceSection(ctx, studentID, start.SessionID.String(), testIDs[0])
	assert.Error(t, err, "advance on a standard-mode session must be rejected")
}

// keep imports used
var _ = time.Now
