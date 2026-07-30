package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
)

// newGradingTestPool spins up an ephemeral Postgres container, applies the app's
// migrations, and returns a connected pool. Used only by this file's real-DB tests
// for the Slice 5 Task 3 grading/rank/result reads (no fake/mocked SQL at this layer).
func newGradingTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := infra.RunMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func insertGradingUser(t *testing.T, pool *pgxpool.Pool, role, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	email := fmt.Sprintf("%s-%s@test.local", role, uuid.NewString())
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		email, role, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertGradingTest(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ('T', 'math', 'algebra', 60) RETURNING id`,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test: %v", err)
	}
	return id
}

func insertGradingEssayQuestion(t *testing.T, pool *pgxpool.Pool, testID uuid.UUID, body string, pointCorrect, sortOrder int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('essay', $1, $2, 0) RETURNING id`,
		body, pointCorrect,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert essay question: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
		testID, id, sortOrder,
	); err != nil {
		t.Fatalf("insert test_question: %v", err)
	}
	return id
}

func insertGradingExam(t *testing.T, pool *pgxpool.Pool, testID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title) VALUES ('Essay Exam') RETURNING id`,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		examID, testID,
	); err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}
	return examID
}

// insertGradingSession seeds an exam_registration + exam_session pair for a student
// with the given status/submitted_at/score, returning the session ID.
func insertGradingSession(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID, status string, submittedAt *time.Time, score *float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var regID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}

	var sessionID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, submitted_at, score)
		VALUES ($1, $2, $3, now(), $4, $5, $6) RETURNING id`,
		regID, studentID, examID, status, submittedAt, score,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("insert exam_session: %v", err)
	}
	return sessionID
}

// insertGradingRegistration seeds a bare exam_registration (no session) for a student,
// used by tests that then attach multiple exam_session rows themselves (FB-26 retakes).
func insertGradingRegistration(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var regID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}
	return regID
}

// insertGradingSessionForRegistration seeds an exam_session row on an existing
// registration with an explicit attempt_number — used to simulate a second FB-26
// attempt on the same registration.
func insertGradingSessionForRegistration(t *testing.T, pool *pgxpool.Pool, regID, studentID, examID uuid.UUID, attemptNumber int, status string, submittedAt *time.Time, score *float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var sessionID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status, submitted_at, score)
		VALUES ($1, $2, $3, $4, now(), $5, $6, $7) RETURNING id`,
		regID, studentID, examID, attemptNumber, status, submittedAt, score,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("insert exam_session for registration: %v", err)
	}
	return sessionID
}

func insertGradingAnswer(t *testing.T, pool *pgxpool.Pool, sessionID, questionID uuid.UUID, answer *string, score *float64, gradedBy *uuid.UUID, gradedAt *time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_session_answer (session_id, question_id, answer, score, graded_by, graded_at, saved_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())`,
		sessionID, questionID, answer, score, gradedBy, gradedAt,
	)
	if err != nil {
		t.Fatalf("insert exam_session_answer: %v", err)
	}
}

func strPtrG(s string) *string    { return &s }
func f64PtrG(f float64) *float64  { return &f }
func timePtrG(t time.Time) *time.Time { return &t }

// TestGradingRepositoryMethods seeds one exam with an essay question and several
// sessions in different grading states, then exercises the five new repo methods
// added by Slice 5 Task 3 against a real Postgres instance.
func TestGradingRepositoryMethods(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	admin := insertGradingUser(t, pool, "admin_exam", "Grader Admin")
	studentA := insertGradingUser(t, pool, "student", "Student A")
	studentB := insertGradingUser(t, pool, "student", "Student B")
	studentC := insertGradingUser(t, pool, "student", "Student C")

	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain gravity", 5, 1)
	examID := insertGradingExam(t, pool, testID)

	now := time.Now()

	// Session A: submitted, essay still ungraded.
	sessionA := insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(0))
	insertGradingAnswer(t, pool, sessionA, essayQID, strPtrG("my essay answer"), nil, nil, nil)

	// Session B: submitted, essay graded, score 4 — fully graded.
	sessionB := insertGradingSession(t, pool, studentB, examID, "submitted", &now, f64PtrG(4))
	insertGradingAnswer(t, pool, sessionB, essayQID, strPtrG("b answer"), f64PtrG(4), &admin, timePtrG(now))

	// Session C: submitted, essay graded, score 1 — fully graded.
	sessionC := insertGradingSession(t, pool, studentC, examID, "submitted", &now, f64PtrG(1))
	insertGradingAnswer(t, pool, sessionC, essayQID, strPtrG("c answer"), f64PtrG(1), &admin, timePtrG(now))

	// Session D: still in_progress — must never surface in grading/rank reads.
	studentD := insertGradingUser(t, pool, "student", "Student D")
	sessionD := insertGradingSession(t, pool, studentD, examID, "in_progress", nil, nil)
	insertGradingAnswer(t, pool, sessionD, essayQID, strPtrG("draft"), nil, nil, nil)

	t.Run("ListSessionsNeedingGrading returns only the session with an ungraded essay", func(t *testing.T) {
		items, err := repo.ListSessionsNeedingGrading(ctx, examID)
		if err != nil {
			t.Fatalf("ListSessionsNeedingGrading: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("want 1 session needing grading, got %d: %+v", len(items), items)
		}
		got := items[0]
		if got.SessionID != sessionA {
			t.Errorf("SessionID = %v, want %v", got.SessionID, sessionA)
		}
		if got.StudentID != studentA {
			t.Errorf("StudentID = %v, want %v", got.StudentID, studentA)
		}
		if got.StudentName != "Student A" {
			t.Errorf("StudentName = %q, want %q", got.StudentName, "Student A")
		}
		if got.SubmittedAt == nil {
			t.Errorf("SubmittedAt = nil, want non-nil")
		}
		if got.UngradedEssayCount != 1 {
			t.Errorf("UngradedEssayCount = %d, want 1", got.UngradedEssayCount)
		}
	})

	t.Run("GetSessionEssayAnswers returns the essay row with question metadata", func(t *testing.T) {
		items, err := repo.GetSessionEssayAnswers(ctx, sessionA)
		if err != nil {
			t.Fatalf("GetSessionEssayAnswers: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("want 1 essay answer, got %d", len(items))
		}
		got := items[0]
		if got.QuestionID != essayQID {
			t.Errorf("QuestionID = %v, want %v", got.QuestionID, essayQID)
		}
		if got.Body != "Explain gravity" {
			t.Errorf("Body = %q, want %q", got.Body, "Explain gravity")
		}
		if got.Answer == nil || *got.Answer != "my essay answer" {
			t.Errorf("Answer = %v, want %q", got.Answer, "my essay answer")
		}
		if got.PointCorrect != 5 {
			t.Errorf("PointCorrect = %d, want 5", got.PointCorrect)
		}
		if got.Score != nil {
			t.Errorf("Score = %v, want nil (ungraded)", got.Score)
		}
		if got.GradedAt != nil {
			t.Errorf("GradedAt = %v, want nil (ungraded)", got.GradedAt)
		}
	})

	t.Run("CountFullyGradedSessions excludes ungraded and in-progress sessions", func(t *testing.T) {
		count, err := repo.CountFullyGradedSessions(ctx, examID)
		if err != nil {
			t.Fatalf("CountFullyGradedSessions: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2 (sessions B and C)", count)
		}
	})

	t.Run("CountHigherScores counts only fully-graded sessions strictly above the threshold", func(t *testing.T) {
		cases := []struct {
			name  string
			score float64
			want  int
		}{
			{"threshold 0 counts both graded sessions", 0, 2},
			{"threshold 2 counts only session B (score 4)", 2, 1},
			{"threshold 4 counts none (no strictly higher score)", 4, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				count, err := repo.CountHigherScores(ctx, examID, tc.score)
				if err != nil {
					t.Fatalf("CountHigherScores: %v", err)
				}
				if count != tc.want {
					t.Errorf("count = %d, want %d", count, tc.want)
				}
			})
		}
	})

	t.Run("GradeEssayAnswerTx persists the grade and clears the session from the queue", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		if err := repo.GradeEssayAnswerTx(ctx, tx, sessionA, essayQID, 3, strPtrG("Good job"), admin); err != nil {
			t.Fatalf("GradeEssayAnswerTx: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}

		var score float64
		var comment string
		var gradedBy uuid.UUID
		var gradedAt *time.Time
		err = pool.QueryRow(ctx,
			`SELECT score, grader_comment, graded_by, graded_at FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
			sessionA, essayQID,
		).Scan(&score, &comment, &gradedBy, &gradedAt)
		if err != nil {
			t.Fatalf("query graded answer: %v", err)
		}
		if score != 3 {
			t.Errorf("score = %v, want 3", score)
		}
		if comment != "Good job" {
			t.Errorf("comment = %q, want %q", comment, "Good job")
		}
		if gradedBy != admin {
			t.Errorf("graded_by = %v, want %v", gradedBy, admin)
		}
		if gradedAt == nil {
			t.Errorf("graded_at = nil, want stamped")
		}

		items, err := repo.ListSessionsNeedingGrading(ctx, examID)
		if err != nil {
			t.Fatalf("ListSessionsNeedingGrading after grading: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("want no sessions needing grading after grading session A, got %d: %+v", len(items), items)
		}
	})
}

// TestGradeEssayAnswerTx_notFound confirms the ErrNotFound sentinel surfaces when the
// (session_id, question_id) pair has no matching answer row.
func TestGradeEssayAnswerTx_notFound(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	admin := insertGradingUser(t, pool, "admin_exam", "Grader Admin")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	err = repo.GradeEssayAnswerTx(ctx, tx, uuid.New(), uuid.New(), 1, nil, admin)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestSubmitSessionTx_leavesEssayGradedAtNull reproduces PG note 1: SaveAnswers
// pre-creates the essay answer row (graded_at NULL) while the exam is in progress, so
// SubmitSessionTx's upsert hits the ON CONFLICT path. Before the fix, the ON CONFLICT
// clause stamped graded_at = now() unconditionally, wrongly marking untouched essays as
// graded the instant the session was submitted. The fix respects the per-answer
// graded_at passed in (EXCLUDED.graded_at), which is nil for essays (FR-S5-09).
func TestSubmitSessionTx_leavesEssayGradedAtNull(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student E")
	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain photosynthesis", 5, 1)
	examID := insertGradingExam(t, pool, testID)
	sessionID := insertGradingSession(t, pool, student, examID, "in_progress", nil, nil)

	// Simulate SaveAnswersTx having already inserted the essay answer row mid-exam.
	insertGradingAnswer(t, pool, sessionID, essayQID, strPtrG("draft answer"), nil, nil, nil)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	graded := []model.ExamSessionAnswer{
		{
			QuestionID: essayQID,
			Answer:     strPtrG("final answer"),
			IsCorrect:  nil,
			Score:      nil,
			GradedBy:   nil,
			GradedAt:   nil,
		},
	}
	affected, err := repo.SubmitSessionTx(ctx, tx, sessionID, graded, 0, false)
	if err != nil {
		t.Fatalf("SubmitSessionTx: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var answer string
	var gradedAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT answer, graded_at FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
		sessionID, essayQID,
	).Scan(&answer, &gradedAt)
	if err != nil {
		t.Fatalf("query answer: %v", err)
	}
	if answer != "final answer" {
		t.Errorf("answer = %q, want %q (upsert should have applied)", answer, "final answer")
	}
	if gradedAt != nil {
		t.Errorf("graded_at = %v, want nil — essay must stay ungraded after submit", *gradedAt)
	}
}

// TestUpdateSessionScoreTx verifies the essay-grading write path's score persistence step
// (Task 5, FR-S5-12/14): after GradeEssayAnswerTx, the caller recomputes and persists the
// session total via UpdateSessionScoreTx in the same transaction.
func TestUpdateSessionScoreTx(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student F")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	sessionID := insertGradingSession(t, pool, student, examID, "submitted", nil, f64PtrG(0))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := repo.UpdateSessionScoreTx(ctx, tx, sessionID, 7); err != nil {
		t.Fatalf("UpdateSessionScoreTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var score float64
	err = pool.QueryRow(ctx, `SELECT score FROM exam_session WHERE id = $1`, sessionID).Scan(&score)
	if err != nil {
		t.Fatalf("query score: %v", err)
	}
	if score != 7 {
		t.Errorf("score = %v, want 7", score)
	}
}

// TestGetSessionEssayAnswers_orderUsesTestQuestionSortOrder verifies that essay answers
// are returned in the order defined by test_question.sort_order for the session's exam,
// not by question insertion order or the dropped question.sort_order (FR-27).
func TestGetSessionEssayAnswers_orderUsesTestQuestionSortOrder(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student Order")
	testID := insertGradingTest(t, pool)

	// Insert two essay questions. qLate is created first but attached with a higher
	// sort_order; qEarly is created second but attached with a lower sort_order.
	qLate := insertGradingEssayQuestion(t, pool, testID, "Late question", 5, 2)
	qEarly := insertGradingEssayQuestion(t, pool, testID, "Early question", 5, 1)

	examID := insertGradingExam(t, pool, testID)
	now := time.Now()
	sessionID := insertGradingSession(t, pool, student, examID, "submitted", &now, f64PtrG(0))

	insertGradingAnswer(t, pool, sessionID, qLate, strPtrG("answer late"), nil, nil, nil)
	insertGradingAnswer(t, pool, sessionID, qEarly, strPtrG("answer early"), nil, nil, nil)

	items, err := repo.GetSessionEssayAnswers(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionEssayAnswers: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 essay answers, got %d", len(items))
	}

	if items[0].QuestionID != qEarly {
		t.Errorf("first question = %v, want %v (lower test_question.sort_order)", items[0].QuestionID, qEarly)
	}
	if items[1].QuestionID != qLate {
		t.Errorf("second question = %v, want %v (higher test_question.sort_order)", items[1].QuestionID, qLate)
	}
}

// TestRankReads_DedupeMultipleAttemptsPerRegistration covers the double-count Task 7
// found: FB-26 lets one registration have two submitted exam_session rows. Student R
// retakes and does WORSE (90 then 50) — the "latest attempt is authoritative" rule
// (not highest-score) means R must be counted once, represented by the 50, everywhere
// that collapses sessions to one row (CountHigherScores, CountFullyGradedSessions,
// ListExamLeaderboard).
func TestRankReads_DedupeMultipleAttemptsPerRegistration(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	now := time.Now()

	studentR := insertGradingUser(t, pool, "student", "Student R")
	regR := insertGradingRegistration(t, pool, studentR, examID)
	insertGradingSessionForRegistration(t, pool, regR, studentR, examID, 1, "submitted", &now, f64PtrG(90))
	insertGradingSessionForRegistration(t, pool, regR, studentR, examID, 2, "submitted", &now, f64PtrG(50))

	studentY := insertGradingUser(t, pool, "student", "Student Y")
	insertGradingSession(t, pool, studentY, examID, "submitted", &now, f64PtrG(70))

	t.Run("CountFullyGradedSessions counts R once, not twice", func(t *testing.T) {
		count, err := repo.CountFullyGradedSessions(ctx, examID)
		if err != nil {
			t.Fatalf("CountFullyGradedSessions: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2 (R deduped + Y)", count)
		}
	})

	t.Run("CountHigherScores does not double-count R's own two rows", func(t *testing.T) {
		// R's latest attempt is 50 (not > 60): R must not count here. Y's 70 > 60: counts.
		// A buggy, non-deduped query would also count R's stale 90 > 60, giving 2.
		count, err := repo.CountHigherScores(ctx, examID, 60)
		if err != nil {
			t.Fatalf("CountHigherScores: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1 (only Y; R's latest attempt is 50)", count)
		}
	})

	t.Run("ListExamLeaderboard shows R once, at the latest attempt's score", func(t *testing.T) {
		entries, _, err := repo.ListExamLeaderboard(ctx, examID, "", 20)
		if err != nil {
			t.Fatalf("ListExamLeaderboard: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("want 2 entries (R deduped + Y), got %d: %+v", len(entries), entries)
		}
		var rEntries int
		for _, e := range entries {
			if e.StudentID == studentR {
				rEntries++
				if e.Score != 50 {
					t.Errorf("R's score = %v, want 50 (latest attempt, not the higher 90)", e.Score)
				}
			}
		}
		if rEntries != 1 {
			t.Errorf("R appears %d times in the leaderboard, want 1", rEntries)
		}
	})
}
