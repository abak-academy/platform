package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestSubmitSessionTx_advancesRegistrationStatus covers FR7 (spec.md): the student
// submit path (adminSubmitted=false) advances the owning exam_registration.status to
// 'submitted' in the same transaction as the CAS, so the row is reachable from
// GET /exam/registrations after logout (FB-27).
func TestSubmitSessionTx_advancesRegistrationStatus(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student Submit")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	sessionID := insertGradingSession(t, pool, student, examID, "in_progress", nil, nil)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	affected, err := repo.SubmitSessionTx(ctx, tx, sessionID, nil, 0, false)
	if err != nil {
		t.Fatalf("SubmitSessionTx: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var regStatus string
	err = pool.QueryRow(ctx,
		`SELECT reg.status FROM exam_registration reg
		JOIN exam_session s ON s.registration_id = reg.id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&regStatus)
	if err != nil {
		t.Fatalf("query registration status: %v", err)
	}
	if regStatus != "submitted" {
		t.Errorf("registration status = %q, want %q", regStatus, "submitted")
	}
}

// TestSubmitSessionTx_adminSubmitted_advancesRegistrationStatus covers FR8: the admin
// force-submit path (adminSubmitted=true) also advances exam_registration.status —
// C6 requires both call sites to land inside SubmitSessionTx, not just the student path.
func TestSubmitSessionTx_adminSubmitted_advancesRegistrationStatus(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student ForceSubmit")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	sessionID := insertGradingSession(t, pool, student, examID, "in_progress", nil, nil)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	affected, err := repo.SubmitSessionTx(ctx, tx, sessionID, nil, 0, true)
	if err != nil {
		t.Fatalf("SubmitSessionTx: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var regStatus string
	err = pool.QueryRow(ctx,
		`SELECT reg.status FROM exam_registration reg
		JOIN exam_session s ON s.registration_id = reg.id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&regStatus)
	if err != nil {
		t.Fatalf("query registration status: %v", err)
	}
	if regStatus != "submitted" {
		t.Errorf("registration status = %q, want %q", regStatus, "submitted")
	}
}

// TestSubmitSessionTx_zeroRowsCAS_leavesRegistrationUnchanged covers FR9: when the CAS
// affects 0 rows (session already submitted), the registration-status write must not run
// at all, not just idempotently reapply 'submitted'. The registration is pinned to a
// sentinel status that 'submitted' would never naturally be, so any unconditional write
// is caught.
func TestSubmitSessionTx_zeroRowsCAS_leavesRegistrationUnchanged(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student AlreadySubmitted")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	sessionID := insertGradingSession(t, pool, student, examID, "submitted", nil, nil)

	if _, err := pool.Exec(ctx,
		`UPDATE exam_registration SET status = 'checked_in'
		WHERE id = (SELECT registration_id FROM exam_session WHERE id = $1)`,
		sessionID,
	); err != nil {
		t.Fatalf("seed sentinel status: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	affected, err := repo.SubmitSessionTx(ctx, tx, sessionID, nil, 0, false)
	if err != nil {
		t.Fatalf("SubmitSessionTx: %v", err)
	}
	if affected != 0 {
		t.Fatalf("affected = %d, want 0 (session already submitted)", affected)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var regStatus string
	err = pool.QueryRow(ctx,
		`SELECT reg.status FROM exam_registration reg
		JOIN exam_session s ON s.registration_id = reg.id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&regStatus)
	if err != nil {
		t.Fatalf("query registration status: %v", err)
	}
	if regStatus != "checked_in" {
		t.Errorf("registration status = %q, want unchanged %q — a 0-row CAS must write nothing", regStatus, "checked_in")
	}
}

// TestGetExamRegistrationsByStudent_carriesSessionIDNoScore covers FR10 and C2/NF2: the
// registrations list carries the id of the submitted registration's session, and the
// marshalled response has no score field anywhere — reachability only, no result-gate
// bypass.
func TestGetExamRegistrationsByStudent_carriesSessionIDNoScore(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student List")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	score := 88.5
	sessionID := insertGradingSession(t, pool, student, examID, "submitted", nil, &score)

	if _, err := pool.Exec(ctx,
		`UPDATE exam_registration SET status = 'submitted'
		WHERE id = (SELECT registration_id FROM exam_session WHERE id = $1)`,
		sessionID,
	); err != nil {
		t.Fatalf("seed registration status: %v", err)
	}

	items, err := repo.GetExamRegistrationsByStudent(ctx, student)
	if err != nil {
		t.Fatalf("GetExamRegistrationsByStudent: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.SessionID == nil || *item.SessionID != sessionID {
		t.Fatalf("SessionID = %v, want %v", item.SessionID, sessionID)
	}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(strings.ToLower(string(b)), "score") {
		t.Errorf("marshalled RegistrationListItem contains %q, want no score field: %s", "score", b)
	}
}

// TestGetExamRegistrationsByStudent_carriesMaxAttempts covers FR20/FR21 (spec.md):
// the frontend's retake-offer predicate needs the exam's max_attempts alongside
// attempts_used, so the list join must surface it — nil when unset (FR18's
// single-attempt default), the configured value otherwise.
func TestGetExamRegistrationsByStudent_carriesMaxAttempts(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student MaxAttempts")
	testID := insertGradingTest(t, pool)

	examWithLimit := insertGradingExam(t, pool, testID)
	if _, err := pool.Exec(ctx, `UPDATE exam SET max_attempts = 3 WHERE id = $1`, examWithLimit); err != nil {
		t.Fatalf("seed max_attempts: %v", err)
	}
	insertGradingSession(t, pool, student, examWithLimit, "submitted", nil, nil)

	examNoLimit := insertGradingExam(t, pool, testID)
	insertGradingSession(t, pool, student, examNoLimit, "submitted", nil, nil)

	items, err := repo.GetExamRegistrationsByStudent(ctx, student)
	if err != nil {
		t.Fatalf("GetExamRegistrationsByStudent: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	var gotLimit, gotNoLimit bool
	for _, item := range items {
		switch item.ExamID {
		case examWithLimit:
			gotLimit = true
			if item.MaxAttempts == nil || *item.MaxAttempts != 3 {
				t.Errorf("examWithLimit MaxAttempts = %v, want 3", item.MaxAttempts)
			}
		case examNoLimit:
			gotNoLimit = true
			if item.MaxAttempts != nil {
				t.Errorf("examNoLimit MaxAttempts = %v, want nil", item.MaxAttempts)
			}
		}
	}
	if !gotLimit || !gotNoLimit {
		t.Fatalf("did not observe both exams in results: gotLimit=%v gotNoLimit=%v", gotLimit, gotNoLimit)
	}
}
