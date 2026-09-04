package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListDueExamExpiryCandidates_SelectsDueTimedRowsDeterministically(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	dueFirst := seedExpiryStandardSession(t, pool, now, expiryStandardSeed{
		title:        "standard first " + uuid.NewString(),
		startedAt:    now.Add(-1000 * time.Hour),
		durationMins: intPtrExpiry(30),
		graceMins:    intPtrExpiry(5),
	})
	ignoredScheduledEnd := seedExpiryStandardSession(t, pool, now, expiryStandardSeed{
		title:          "standard scheduled end ignored " + uuid.NewString(),
		startedAt:      now.Add(-20 * time.Minute),
		durationMins:   intPtrExpiry(60),
		graceMins:      intPtrExpiry(5),
		scheduledEndAt: timePtrExpiry(now.Add(-10 * time.Minute)),
	})
	dueSecond := seedExpiryStandardSession(t, pool, now, expiryStandardSeed{
		title:         "standard extension " + uuid.NewString(),
		startedAt:     now.Add(-999 * time.Hour),
		durationMins:  intPtrExpiry(30),
		graceMins:     intPtrExpiry(10),
		extendedUntil: timePtrExpiry(now.Add(-998 * time.Hour)),
	})
	seedExpiryStandardSession(t, pool, now, expiryStandardSeed{
		title:        "standard untimed " + uuid.NewString(),
		startedAt:    now.Add(-24 * time.Hour),
		durationMins: nil,
		graceMins:    intPtrExpiry(5),
	})
	seedExpiryStandardSession(t, pool, now, expiryStandardSeed{
		title:        "standard terminal " + uuid.NewString(),
		startedAt:    now.Add(-24 * time.Hour),
		durationMins: intPtrExpiry(30),
		graceMins:    intPtrExpiry(5),
		status:       "submitted",
	})

	got, err := repo.ListDueExamExpiryCandidates(ctx, now, 2)
	if err != nil {
		t.Fatalf("ListDueExamExpiryCandidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("candidates: want 2 fixed-limit rows, got %d: %#v", len(got), got)
	}
	if got[0].SessionID != dueFirst || got[0].TestID != nil {
		t.Fatalf("first candidate = %#v, want standard session %s", got[0], dueFirst)
	}
	if got[1].SessionID != dueSecond || got[1].TestID != nil {
		t.Fatalf("second candidate = %#v, want standard session %s", got[1], dueSecond)
	}
	for _, cand := range got {
		if cand.SessionID == ignoredScheduledEnd {
			t.Fatalf("scheduled_end_at-only overdue session was selected: %#v", cand)
		}
	}
}

func TestListDueExamExpiryCandidates_SelectsDueActiveSections(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	dueSession, dueTest := seedExpirySectionedSession(t, pool, now, expirySectionSeed{
		title:        "ielts due section " + uuid.NewString(),
		mode:         "ielts",
		startedAt:    now.Add(-90 * time.Minute),
		durationMins: 45,
		graceMins:    intPtrExpiry(5),
	})
	seedExpirySectionedSession(t, pool, now, expirySectionSeed{
		title:         "utbk extended active " + uuid.NewString(),
		mode:          "utbk",
		startedAt:     now.Add(-90 * time.Minute),
		durationMins:  45,
		graceMins:     intPtrExpiry(5),
		extendedUntil: timePtrExpiry(now.Add(-4 * time.Minute)),
	})
	seedExpirySectionedSession(t, pool, now, expirySectionSeed{
		title:        "ielts no active " + uuid.NewString(),
		mode:         "ielts",
		startedAt:    now.Add(-90 * time.Minute),
		durationMins: 45,
		graceMins:    intPtrExpiry(5),
		noActive:     true,
	})

	got, err := repo.ListDueExamExpiryCandidates(ctx, now, 10)
	if err != nil {
		t.Fatalf("ListDueExamExpiryCandidates: %v", err)
	}

	var found bool
	for _, cand := range got {
		if cand.SessionID == dueSession {
			found = true
			if cand.TestID == nil || *cand.TestID != dueTest {
				t.Fatalf("due section test_id = %v, want %s", cand.TestID, dueTest)
			}
		}
	}
	if !found {
		t.Fatalf("due active section session %s was not selected: %#v", dueSession, got)
	}
}

type expiryStandardSeed struct {
	title          string
	startedAt      time.Time
	durationMins   *int
	graceMins      *int
	extendedUntil  *time.Time
	scheduledEndAt *time.Time
	status         string
}

func seedExpiryStandardSession(t *testing.T, pool *pgxpool.Pool, now time.Time, seed expiryStandardSeed) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if seed.status == "" {
		seed.status = "in_progress"
	}
	studentID := insertGradingUser(t, pool, "student", "Expiry Student")
	testID := insertGradingTest(t, pool)
	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, mode, timer_mode, duration_minutes, grace_window_minutes, scheduled_end_at)
		VALUES ($1, 'standard', 'overall', $2, $3, $4)
		RETURNING id`,
		seed.title, seed.durationMins, seed.graceMins, seed.scheduledEndAt,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert expiry exam: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		examID, testID,
	); err != nil {
		t.Fatalf("insert expiry exam_test: %v", err)
	}
	return insertExpirySession(t, pool, studentID, examID, seed.startedAt, seed.extendedUntil, seed.status)
}

type expirySectionSeed struct {
	title         string
	mode          string
	startedAt     time.Time
	durationMins  int
	graceMins     *int
	extendedUntil *time.Time
	noActive      bool
}

func seedExpirySectionedSession(t *testing.T, pool *pgxpool.Pool, now time.Time, seed expirySectionSeed) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	studentID := insertGradingUser(t, pool, "student", "Expiry Section Student")
	activeTestID := insertGradingTest(t, pool)
	pendingTestID := insertGradingTest(t, pool)
	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, mode, timer_mode, duration_minutes, grace_window_minutes)
		VALUES ($1, $2, 'per_test', NULL, $3)
		RETURNING id`,
		seed.title, seed.mode, seed.graceMins,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert sectioned expiry exam: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1), ($1, $3, 2)`,
		examID, activeTestID, pendingTestID,
	); err != nil {
		t.Fatalf("insert sectioned expiry exam_test: %v", err)
	}
	sessionID := insertExpirySession(t, pool, studentID, examID, now.Add(-5*time.Minute), nil, "in_progress")
	status := "active"
	startedAt := &seed.startedAt
	if seed.noActive {
		status = "pending"
		startedAt = nil
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO exam_session_section
			(session_id, test_id, sort_order, duration_minutes, status, started_at, extended_until)
		VALUES ($1, $2, 1, $3, $4, $5, $6),
		       ($1, $7, 2, $3, 'pending', NULL, NULL)`,
		sessionID, activeTestID, seed.durationMins, status, startedAt, seed.extendedUntil, pendingTestID,
	); err != nil {
		t.Fatalf("insert session sections: %v", err)
	}
	return sessionID, activeTestID
}

func insertExpirySession(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID, startedAt time.Time, extendedUntil *time.Time, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var regID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		VALUES ($1, $2, $3, 'in_progress')
		RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert expiry registration: %v", err)
	}
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, extended_until, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		regID, studentID, examID, startedAt, extendedUntil, status,
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert expiry session: %v", err)
	}
	return sessionID
}

func intPtrExpiry(v int) *int { return &v }

func timePtrExpiry(v time.Time) *time.Time { return &v }
