package repository

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertAssessmentSchool(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		name, "as_"+uuid.NewString()[:8],
	).Scan(&id); err != nil {
		t.Fatalf("insert school: %v", err)
	}
	return id
}

func insertAssessmentStudent(t *testing.T, pool *pgxpool.Pool, name, username string, schoolID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, username, school_id)
		VALUES ($1, 'student', $2, $3, $4) RETURNING id`,
		username+"-"+uuid.NewString()[:8]+"@test.local", name, username, schoolID,
	).Scan(&id); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

func insertAssessmentViolation(t *testing.T, pool *pgxpool.Pool, sessionID, studentID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO session_violation_log (session_id, student_id, violation_type, occurred_at)
		VALUES ($1, $2, 'tab_blur', now())`,
		sessionID, studentID,
	); err != nil {
		t.Fatalf("insert violation: %v", err)
	}
}

func assertFloatClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %.4f, want %.4f", got, want)
	}
}

func TestAssessmentRepository_DBBackedSemantics(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertAssessmentSchool(t, pool, "Assessment School A")
	schoolB := insertAssessmentSchool(t, pool, "Assessment School B")
	grader := insertGradingUser(t, pool, "admin_exam", "Assessment Grader")
	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain assessment", 100, 1)
	examID := insertGradingExam(t, pool, testID)

	budi := insertAssessmentStudent(t, pool, "Budi Assessment", "budi-assessment", schoolA)
	cici := insertAssessmentStudent(t, pool, "Cici Retry", "cici-retry", schoolA)
	dodi := insertAssessmentStudent(t, pool, "Dodi Ungraded", "dodi-ungraded", schoolB)
	evi := insertAssessmentStudent(t, pool, "Evi Empty", "evi-empty", schoolA)
	fani := insertAssessmentStudent(t, pool, "Fani Tie", "fani-tie", schoolA)

	base := time.Now().Add(-2 * time.Hour)
	gradedAt := base.Add(time.Hour)
	answer := "answer"

	regBudi := insertGradingRegistration(t, pool, budi, examID)
	budiOld := insertGradingSessionForRegistration(t, pool, regBudi, budi, examID, 1, "submitted", timePtrG(base), f64PtrG(60))
	insertGradingAnswer(t, pool, budiOld, essayQID, &answer, f64PtrG(60), &grader, &gradedAt)
	budiLatest := insertGradingSessionForRegistration(t, pool, regBudi, budi, examID, 2, "submitted", timePtrG(base.Add(30*time.Minute)), f64PtrG(90))
	insertGradingAnswer(t, pool, budiLatest, essayQID, &answer, f64PtrG(90), &grader, &gradedAt)
	insertAssessmentViolation(t, pool, budiLatest, budi)
	insertAssessmentViolation(t, pool, budiLatest, budi)

	regCici := insertGradingRegistration(t, pool, cici, examID)
	ciciOld := insertGradingSessionForRegistration(t, pool, regCici, cici, examID, 1, "submitted", timePtrG(base), f64PtrG(100))
	insertGradingAnswer(t, pool, ciciOld, essayQID, &answer, f64PtrG(100), &grader, &gradedAt)
	insertAssessmentViolation(t, pool, ciciOld, cici)
	insertGradingSessionForRegistration(t, pool, regCici, cici, examID, 2, "in_progress", nil, nil)

	regDodi := insertGradingRegistration(t, pool, dodi, examID)
	dodiSession := insertGradingSessionForRegistration(t, pool, regDodi, dodi, examID, 1, "submitted", timePtrG(base), f64PtrG(70))
	insertGradingAnswer(t, pool, dodiSession, essayQID, &answer, nil, nil, nil)
	insertAssessmentViolation(t, pool, dodiSession, dodi)

	insertGradingRegistration(t, pool, evi, examID)
	regFani := insertGradingRegistration(t, pool, fani, examID)
	faniSession := insertGradingSessionForRegistration(t, pool, regFani, fani, examID, 1, "submitted", timePtrG(base), f64PtrG(90))
	insertGradingAnswer(t, pool, faniSession, essayQID, &answer, f64PtrG(90), &grader, &gradedAt)

	filterSchoolA := AssessmentFilter{SchoolID: &schoolA, Limit: 10}
	rows, next, err := repo.ListAssessmentRows(ctx, examID, filterSchoolA)
	if err != nil {
		t.Fatalf("ListAssessmentRows school A: %v", err)
	}
	if next != "" {
		t.Fatalf("next cursor = %q, want empty", next)
	}
	if len(rows) != 4 {
		t.Fatalf("school A row count = %d, want 4: %+v", len(rows), rows)
	}
	byName := map[string]struct {
		status       string
		score        *float64
		rank         *int
		attempts     int
		latestVios   int
		latestSessID *uuid.UUID
	}{}
	for _, row := range rows {
		byName[row.StudentName] = struct {
			status       string
			score        *float64
			rank         *int
			attempts     int
			latestVios   int
			latestSessID *uuid.UUID
		}{row.Status, row.Score, row.Rank, row.AttemptsCount, row.LatestViolations, row.LatestSessionID}
	}
	if got := byName["Budi Assessment"]; got.status != "completed" || got.score == nil || *got.score != 90 || got.rank == nil || *got.rank != 2 || got.attempts != 2 || got.latestVios != 2 || got.latestSessID == nil || *got.latestSessID != budiLatest {
		t.Fatalf("Budi row mismatch: %+v", got)
	}
	if got := byName["Cici Retry"]; got.status != "in_progress" || got.score == nil || *got.score != 100 || got.rank == nil || *got.rank != 1 || got.attempts != 2 {
		t.Fatalf("Cici row should keep latest status while scoring latest fully graded submission, got %+v", got)
	}
	if got := byName["Evi Empty"]; got.status != "not_started" || got.latestSessID != nil || got.attempts != 0 {
		t.Fatalf("Evi row should be not_started, got %+v", got)
	}
	if got := byName["Fani Tie"]; got.status != "completed" || got.rank == nil || *got.rank != 2 {
		t.Fatalf("Fani should share rank 2 tie, got %+v", got)
	}

	total, completed, scores, violationAttempts, violationEvents, err := repo.GetAssessmentSummary(ctx, examID, filterSchoolA)
	if err != nil {
		t.Fatalf("GetAssessmentSummary school A: %v", err)
	}
	if total != 4 || completed != 2 || len(scores) != 3 || violationAttempts != 2 || violationEvents != 3 {
		t.Fatalf("summary mismatch total=%d completed=%d scores=%v violationAttempts=%d violationEvents=%d", total, completed, scores, violationAttempts, violationEvents)
	}
	assertFloatClose(t, scores[0]+scores[1]+scores[2], 280)

	q := AssessmentFilter{Q: "budi-assessment", Limit: 10}
	total, completed, scores, violationAttempts, violationEvents, err = repo.GetAssessmentSummary(ctx, examID, q)
	if err != nil {
		t.Fatalf("GetAssessmentSummary q: %v", err)
	}
	if total != 1 || completed != 1 || len(scores) != 1 || scores[0] != 90 || violationAttempts != 1 || violationEvents != 2 {
		t.Fatalf("q-filter summary mismatch total=%d completed=%d scores=%v violationAttempts=%d violationEvents=%d", total, completed, scores, violationAttempts, violationEvents)
	}

	firstPage, cursor, err := repo.ListAssessmentRows(ctx, examID, AssessmentFilter{SchoolID: &schoolA, Limit: 1})
	if err != nil {
		t.Fatalf("ListAssessmentRows page1: %v", err)
	}
	if len(firstPage) != 1 || cursor == "" {
		t.Fatalf("page1 len=%d cursor=%q", len(firstPage), cursor)
	}
	secondPage, _, err := repo.ListAssessmentRows(ctx, examID, AssessmentFilter{SchoolID: &schoolA, Cursor: cursor, Limit: 1})
	if err != nil {
		t.Fatalf("ListAssessmentRows page2: %v", err)
	}
	if len(secondPage) != 1 || secondPage[0].RegistrationID == firstPage[0].RegistrationID {
		t.Fatalf("page2 did not advance: page1=%+v page2=%+v", firstPage, secondPage)
	}

	attempts, err := repo.ListAssessmentAttempts(ctx, examID, regBudi)
	if err != nil {
		t.Fatalf("ListAssessmentAttempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempt count = %d, want 2: %+v", len(attempts), attempts)
	}
	if attempts[0].SessionID != budiLatest || !attempts[0].IsLatest || !attempts[0].ResultAvailable || attempts[0].Violations != 2 {
		t.Fatalf("latest attempt mismatch: %+v", attempts[0])
	}
	if attempts[1].SessionID != budiOld || attempts[1].IsLatest {
		t.Fatalf("old attempt mismatch: %+v", attempts[1])
	}

	if _, err := repo.ListAssessmentAttempts(ctx, examID, uuid.New()); err != ErrNotFound {
		t.Fatalf("wrong registration err = %v, want ErrNotFound", err)
	}
	if _, _, err := repo.ListAssessmentRows(ctx, examID, AssessmentFilter{Cursor: "not-a-cursor"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid cursor err = %v, want ErrInvalidCursor", err)
	}
}
