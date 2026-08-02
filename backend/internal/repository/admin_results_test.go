package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// insertSchool creates a school and returns its ID.
func insertSchool(t *testing.T, pool *pgxpool.Pool, name, code string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		name, code,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school: %v", err)
	}
	return id
}

// insertSchoolUser creates a user with the given school_id.
func insertSchoolUser(t *testing.T, pool *pgxpool.Pool, role, name string, schoolID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	email := fmt.Sprintf("%s-%s@test.local", role, uuid.NewString())
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		email, role, name, schoolID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school user: %v", err)
	}
	return id
}

// insertAdminResultsMCQQuestion inserts a non-essay question for tests where essays aren't needed.
func insertAdminResultsMCQQuestion(t *testing.T, pool *pgxpool.Pool, testID uuid.UUID, body string, pointCorrect, sortOrder int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('mcq', $1, $2, 0) RETURNING id`,
		body, pointCorrect,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert mcq question: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
		testID, id, sortOrder,
	); err != nil {
		t.Fatalf("insert test_question: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Compile-time signature check
// ---------------------------------------------------------------------------

var _ interface {
	ListSchoolResults(context.Context, uuid.UUID, string, AdminResultFilter) ([]model.AdminResultRow, string, error)
	GetSchoolResultSession(context.Context, uuid.UUID, string) (*model.AdminResultSession, error)
} = (*Repository)(nil)

// ---------------------------------------------------------------------------
// Test: ListSchoolResults
// ---------------------------------------------------------------------------

// TestAdminResults_ListSchoolResults exercises ListSchoolResults and
// GetSchoolResultSession against a real Postgres instance.
func TestAdminResults_ListSchoolResults(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	// Create two schools for cross-school isolation testing.
	schoolA := insertSchool(t, pool, "School A", "scha")
	schoolB := insertSchool(t, pool, "School B", "schb")

	// School A students.
	studentA := insertSchoolUser(t, pool, "student", "Student A", schoolA)
	studentB := insertSchoolUser(t, pool, "student", "Student B", schoolA)
	studentC := insertSchoolUser(t, pool, "student", "Student C", schoolA)
	studentD := insertSchoolUser(t, pool, "student", "Student D", schoolA)
	// Student with username set for free-text search testing (Q filter matches name OR username).
	studentF := insertSchoolUser(t, pool, "student", "Student F", schoolA)
	pool.Exec(ctx, `UPDATE users SET username = 'usrf-1234' WHERE id = $1`, studentF)

	// School B student (cross-school exclusion check).
	studentE := insertSchoolUser(t, pool, "student", "Student E", schoolB)
	pool.Exec(ctx, `UPDATE users SET username = 'usre-1234' WHERE id = $1`, studentE)

	// Admin/grader user.
	admin := insertSchoolUser(t, pool, "admin_exam", "Grader Admin", schoolA)

	// Create exam with an essay question to test fullyGradedFilter.
	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain gravity", 10, 1)
	examID := insertGradingExam(t, pool, testID)

	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	muchEarlier := now.Add(-2 * time.Hour)

	// Session A (student A, school A): submitted, fully graded → should appear.
	sessionA := insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(90))
	insertGradingAnswer(t, pool, sessionA, essayQID, strPtrG("a answer"), f64PtrG(10), &admin, timePtrG(now))

	// Session B (student B, school A): submitted, fully graded → should appear.
	sessionB := insertGradingSession(t, pool, studentB, examID, "submitted", &earlier, f64PtrG(80))
	insertGradingAnswer(t, pool, sessionB, essayQID, strPtrG("b answer"), f64PtrG(10), &admin, timePtrG(earlier))

	// Session C (student C, school A): submitted, essay UNGRADED → excluded by fullyGradedFilter.
	sessionC := insertGradingSession(t, pool, studentC, examID, "submitted", &muchEarlier, f64PtrG(0))
	insertGradingAnswer(t, pool, sessionC, essayQID, strPtrG("c answer"), nil, nil, nil)

	// Session D (student D, school A): in_progress → excluded by status filter.
	sessionD := insertGradingSession(t, pool, studentD, examID, "in_progress", nil, nil)

	// Session E (student E, school B): submitted, fully graded → excluded by school scope.
	sessionE := insertGradingSession(t, pool, studentE, examID, "submitted", &now, f64PtrG(95))
	insertGradingAnswer(t, pool, sessionE, essayQID, strPtrG("e answer"), f64PtrG(10), &admin, timePtrG(now))

	// Session F (student F, school A): submitted, fully graded → should appear (username searchable).
	sessionF := insertGradingSession(t, pool, studentF, examID, "submitted", &now, f64PtrG(85))
	insertGradingAnswer(t, pool, sessionF, essayQID, strPtrG("f answer"), f64PtrG(10), &admin, timePtrG(now))

	_ = sessionC
	_ = sessionD
	_ = sessionE

	t.Run("excludes other schools, non-submitted sessions, and ungraded essays", func(t *testing.T) {
		rows, nextCursor, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		// Sessions A, B, F should appear (3). C (ungraded), D (in_progress), E (wrong school) excluded.
		if len(rows) != 3 {
			t.Fatalf("want 3 rows (A, B, F), got %d: %+v", len(rows), rows)
		}
		if nextCursor != "" {
			t.Errorf("expected empty cursor (all rows returned), got %q", nextCursor)
		}
		// Verify correct order: newest first (submitted_at DESC).
		if rows[0].SessionID != sessionA && rows[0].SessionID != sessionF {
			t.Errorf("first row should be session A or F (newest), got %v", rows[0].SessionID)
		}
		// Verify StudentName is populated.
		if rows[0].StudentName == "" {
			t.Errorf("StudentName should be populated, got empty")
		}
		// Verify score is populated for fully-graded sessions.
		if rows[0].Score == nil {
			t.Errorf("Score should be non-nil for fully-graded session")
		}
		// Verify submitted_at is populated.
		if rows[0].SubmittedAt == nil {
			t.Errorf("SubmittedAt should be non-nil for submitted session")
		}
	})

	t.Run("empty school filter returns rows from every school", func(t *testing.T) {
		rows, _, err := repo.ListSchoolResults(ctx, examID, "", AdminResultFilter{Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		// A, B, F (school A) + E (school B) — C (ungraded) and D (in_progress) excluded.
		if len(rows) != 4 {
			t.Fatalf("want 4 rows across both schools, got %d: %+v", len(rows), rows)
		}
		seenSchools := map[string]bool{}
		for _, r := range rows {
			if r.SchoolName == nil {
				t.Errorf("row %v: SchoolName should be populated, got nil", r.SessionID)
				continue
			}
			seenSchools[*r.SchoolName] = true
		}
		if len(seenSchools) != 2 {
			t.Errorf("want rows from 2 distinct schools, got %d: %v", len(seenSchools), seenSchools)
		}
		if !seenSchools["School A"] || !seenSchools["School B"] {
			t.Errorf("want School A and School B both present, got %v", seenSchools)
		}
	})

	t.Run("school B sees only its own sessions", func(t *testing.T) {
		rows, cursor, err := repo.ListSchoolResults(ctx, examID, schoolB, AdminResultFilter{Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		// Session E belongs to school B — should appear. Sessions A/B/C/F belong to school A.
		if len(rows) != 1 {
			t.Fatalf("want 1 row (session E) for school B, got %d: %+v", len(rows), rows)
		}
		if rows[0].SessionID != sessionE {
			t.Errorf("expected session E, got %v", rows[0].SessionID)
		}
		if cursor != "" {
			t.Errorf("expected empty cursor for 1 row, got %q", cursor)
		}
	})

	t.Run("free-text search filters by name ILIKE", func(t *testing.T) {
		rows, _, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{Q: "Student A", Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults with Q: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row for Q='Student A', got %d: %+v", len(rows), rows)
		}
		if rows[0].SessionID != sessionA {
			t.Errorf("expected session A, got %v", rows[0].SessionID)
		}
	})

	t.Run("free-text search filters by username ILIKE", func(t *testing.T) {
		rows, _, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{Q: "usrf-1234", Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults with Q='usrf-1234': %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row for Q='usrf-1234', got %d: %+v", len(rows), rows)
		}
		if rows[0].SessionID != sessionF {
			t.Errorf("expected session F, got %v", rows[0].SessionID)
		}
	})

	t.Run("malformed cursor returns ErrInvalidCursor", func(t *testing.T) {
		for _, bad := range []string{"nocomma", "badtime,00000000-0000-0000-0000-000000000001", "2026-notatime,uuid"} {
			_, _, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{Cursor: bad, Limit: 20})
			if err == nil {
				t.Errorf("cursor %q: expected error, got nil", bad)
				continue
			}
			if !errors.Is(err, ErrInvalidCursor) {
				t.Errorf("cursor %q: want ErrInvalidCursor, got %v", bad, err)
			}
		}
	})

	t.Run("default and capping of limit", func(t *testing.T) {
		// Zero limit should default to 20 (and fetch at most 21 rows).
		rows, _, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{})
		if err != nil {
			t.Fatalf("ListSchoolResults with zero limit: %v", err)
		}
		if len(rows) == 0 {
			t.Error("expected at least some rows with default limit")
		}
	})
}

// ---------------------------------------------------------------------------
// Test: NULL school_id students (unlisted_school_name) are not silently dropped
// ---------------------------------------------------------------------------

// TestAdminResults_ListSchoolResults_NullSchoolID_UsesUnlistedName is the regression
// test for the inner-join bug: a student with school_id NULL and a free-text
// unlisted_school_name must still appear in the all-schools view, with that name
// as their school. A school-scoped filter must NOT surface them (they belong to
// no tenant), so RBAC for school-scoped admins is unaffected.
func TestAdminResults_ListSchoolResults_NullSchoolID_UsesUnlistedName(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertSchool(t, pool, "Unlisted Test School A", "unla")

	// Student with no school_id, but a free-text unlisted_school_name.
	unlistedName := "SMA Swasta Merdeka"
	unlistedStudent := insertSchoolUser(t, pool, "student", "Unlisted Student", schoolA)
	// Clear the school_id set by the helper and set the free-text name instead.
	if _, err := pool.Exec(ctx,
		`UPDATE users SET school_id = NULL, unlisted_school_name = $2 WHERE id = $1`,
		unlistedStudent, unlistedName,
	); err != nil {
		t.Fatalf("clear school_id / set unlisted_school_name: %v", err)
	}

	testID := insertGradingTest(t, pool)
	insertAdminResultsMCQQuestion(t, pool, testID, "Q1", 5, 1)
	examID := insertGradingExam(t, pool, testID)

	now := time.Now()
	session := insertGradingSession(t, pool, unlistedStudent, examID, "submitted", &now, f64PtrG(100))

	t.Run("appears in the all-schools view with the unlisted name", func(t *testing.T) {
		rows, _, err := repo.ListSchoolResults(ctx, examID, "", AdminResultFilter{Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
		}
		if rows[0].SessionID != session {
			t.Errorf("SessionID = %v, want %v", rows[0].SessionID, session)
		}
		if rows[0].SchoolName == nil || *rows[0].SchoolName != unlistedName {
			t.Errorf("SchoolName = %v, want %q", rows[0].SchoolName, unlistedName)
		}
	})

	t.Run("does not appear under an unrelated school's scoped filter", func(t *testing.T) {
		rows, _, err := repo.ListSchoolResults(ctx, examID, schoolA, AdminResultFilter{Limit: 20})
		if err != nil {
			t.Fatalf("ListSchoolResults: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("want 0 rows for schoolA filter (student has no school_id), got %d: %+v", len(rows), rows)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: keyset pagination with identical submitted_at
// ---------------------------------------------------------------------------

// TestAdminResults_SameTimestampPagination proves that two sessions sharing an
// identical submitted_at are both reachable via limit=1 cursor walk (no skipping,
// no duplication). This is the regression test for the tie-breaker (FR-SCHOOL-08-09).
func TestAdminResults_SameTimestampPagination(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolID := insertSchool(t, pool, "Pagination School", "pagsch")
	student1 := insertSchoolUser(t, pool, "student", "Paginate 1", schoolID)
	student2 := insertSchoolUser(t, pool, "student", "Paginate 2", schoolID)

	// Exam with only MCQ questions — fullyGradedFilter always passes.
	testID := insertGradingTest(t, pool)
	insertAdminResultsMCQQuestion(t, pool, testID, "Q1", 5, 1)
	examID := insertGradingExam(t, pool, testID)

	// Both sessions share the exact same submitted_at timestamp.
	sameTime := time.Now().Truncate(time.Microsecond)
	session1 := insertGradingSession(t, pool, student1, examID, "submitted", &sameTime, f64PtrG(80))
	session2 := insertGradingSession(t, pool, student2, examID, "submitted", &sameTime, f64PtrG(90))

	// Walk with limit=1, verifying every row is visited exactly once.
	seenSessions := map[uuid.UUID]int{}
	cursor := ""
	for page := 0; page < 10; page++ {
		rows, next, err := repo.ListSchoolResults(ctx, examID, schoolID, AdminResultFilter{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("walk page %d: %v", page, err)
		}
		for _, r := range rows {
			seenSessions[r.SessionID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}

	if len(seenSessions) != 2 {
		t.Errorf("walk should visit 2 distinct sessions, got %d: %v", len(seenSessions), seenSessions)
	}
	if seenSessions[session1] != 1 {
		t.Errorf("session1 should appear exactly once, got %d", seenSessions[session1])
	}
	if seenSessions[session2] != 1 {
		t.Errorf("session2 should appear exactly once, got %d", seenSessions[session2])
	}
}

// ---------------------------------------------------------------------------
// Test: GetSchoolResultSession
// ---------------------------------------------------------------------------

// TestAdminResults_GetSchoolResultSession verifies the detail lookup is properly
// school-scoped and returns ErrNotFound for cross-school / nonexistent sessions.
func TestAdminResults_GetSchoolResultSession(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertSchool(t, pool, "Detail School A", "deta")
	schoolB := insertSchool(t, pool, "Detail School B", "detb")

	studentA := insertSchoolUser(t, pool, "student", "Detail Student A", schoolA)
	studentB := insertSchoolUser(t, pool, "student", "Detail Student B", schoolB)

	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	now := time.Now()

	// Session A belongs to school A.
	sessionA := insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(85))

	// Session B belongs to school B.
	sessionB := insertGradingSession(t, pool, studentB, examID, "submitted", &now, f64PtrG(90))
	_ = sessionB

	t.Run("returns session when school matches", func(t *testing.T) {
		s, err := repo.GetSchoolResultSession(ctx, sessionA, schoolA)
		if err != nil {
			t.Fatalf("GetSchoolResultSession: %v", err)
		}
		if s.SessionID != sessionA {
			t.Errorf("SessionID = %v, want %v", s.SessionID, sessionA)
		}
		if s.StudentName != "Detail Student A" {
			t.Errorf("StudentName = %q, want %q", s.StudentName, "Detail Student A")
		}
		if s.Status != "submitted" {
			t.Errorf("Status = %q, want %q", s.Status, "submitted")
		}
		if s.Score == nil || *s.Score != 85 {
			t.Errorf("Score = %v, want 85", s.Score)
		}
	})

	t.Run("ErrNotFound for session from a different school", func(t *testing.T) {
		_, err := repo.GetSchoolResultSession(ctx, sessionA, schoolB)
		if err != ErrNotFound {
			t.Fatalf("want ErrNotFound for cross-school session, got %v", err)
		}
	})

	t.Run("ErrNotFound for non-existent session", func(t *testing.T) {
		_, err := repo.GetSchoolResultSession(ctx, uuid.New(), schoolA)
		if err != ErrNotFound {
			t.Fatalf("want ErrNotFound for non-existent session, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: EXPLAIN ANALYZE confirms idx_examsession_monitor
// ---------------------------------------------------------------------------

// TestAdminResults_ExplainAnalyze verifies the query planner uses
// idx_examsession_monitor (exam_id, status) rather than a sequential scan.
func TestAdminResults_ExplainAnalyze(t *testing.T) {
	pool := newGradingTestPool(t)
	ctx := context.Background()

	// Seed a school, some students, and an exam with enough sessions so the
	// planner has meaningful options.
	schoolID := insertSchool(t, pool, "Explain School", "expln")

	// Create a single MCQ question to avoid duplicate sort_order (uq_question_order).
	tID := insertGradingTest(t, pool)
	qID := insertAdminResultsMCQQuestion(t, pool, tID, "Q1", 5, 1)
	examID := insertGradingExam(t, pool, tID)
	now := time.Now()

	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("Explain Student %d", i)
		student := insertSchoolUser(t, pool, "student", name, schoolID)
		session := insertGradingSession(t, pool, student, examID, "submitted", &now, f64PtrG(float64(70+i)))
		// Insert a dummy answer so the session has content (references the single question).
		_, _ = pool.Exec(ctx,
			`INSERT INTO exam_session_answer (session_id, question_id, answer, is_correct, score, saved_at)
			VALUES ($1, $2, 'x', true, 5, now())`,
			session, qID,
		)
	}

	// Run EXPLAIN ANALYZE on the core query shape (same JOIN/filter structure
	// as ListSchoolResults, with fullyGradedFilter relaxed for MCQ simplicity).
	query := `SELECT s.id FROM exam_session s
		JOIN users u ON u.id = s.student_id AND u.school_id = $1 AND u.role = 'student'
		WHERE s.exam_id = $2 AND s.status = 'submitted'`

	// enable_seqscan=off because the planner is *right* to seq-scan a table this small:
	// once real statistics exist it sees one page and picks the cheaper plan. This test
	// used to pass only because a private container had never been ANALYZEd, so Postgres
	// assumed the table was large. What it means to assert is that the index covers this
	// predicate and the planner can use it — which is what discouraging seq scan shows,
	// at any table size.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}
	defer func() { _, _ = conn.Exec(ctx, `SET enable_seqscan = on`) }()

	rows, err := conn.Query(ctx, `EXPLAIN ANALYZE `+query, schoolID, examID)
	if err != nil {
		t.Fatalf("EXPLAIN ANALYZE: %v", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan line: %v", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	plan := strings.Join(planLines, "\n")
	t.Logf("EXPLAIN ANALYZE plan:\n%s", plan)

	// Name the index, and only that index. The previous "... || Index Scan" escape hatch
	// accepted any index at all — including the primary key — so the assertion survived
	// a query whose predicate idx_examsession_monitor cannot serve.
	if !strings.Contains(plan, "idx_examsession_monitor") {
		t.Errorf("expected idx_examsession_monitor in the plan for (exam_id, status), got:\n%s", plan)
	}
}
