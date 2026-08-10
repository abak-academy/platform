package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// This file shares the newGradingTestPool container (exam_grading_test.go) with
// 14+ other test files; rows are never reset between tests. Reserved windows,
// in Asia/Jakarta — take a free one rather than reusing one below. Only this
// file writes exam.scheduled_at (confirmed by grep; exam_certificate_design_test.go
// is the one other writer, pinned to 2026-03-15 UTC), so these windows only need
// to stay clear of each other, not of dashboard_series_test.go's orders/users windows.
//
//	2026-10-03 .. 2026-10-17  UpcomingExamsExcludesPastAndUnscheduled
//	2026-11-03 .. 2026-11-17  UpcomingExamsOrdersBySoonestFirst
//	2026-12-01                UpcomingExamsScheduledAtInJakartaOffset (single point)
//
// CountActiveExamSessions and CountStudentsAndSchools count rows globally and
// assert on before/after deltas instead, so they claim no window.

func ptrTime(t time.Time) *time.Time {
	return &t
}

// seedCountsSchool inserts a school row into the shared grading pool. Distinct
// from seedSchool (order_buyer_test.go), which writes into the separate
// newReportingTestPool container — a different database, invisible to r here.
func seedCountsSchool(t *testing.T, r *Repository, name string) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		name, "SCH-"+uuid.NewString()[:8],
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed school: %v", err)
	}
	return id
}

// seedCountsStudentWithStatus mirrors seedDashboardStudent (dashboard_series_test.go)
// but parameterizes status instead of hardcoding 'active'.
func seedCountsStudentWithStatus(t *testing.T, r *Repository, createdAt time.Time, status string) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, jenjang, grade, otp_enabled, created_at)
		 VALUES ($1, $2, 'student', $3, 'sma', 10, false, $4)
		 RETURNING id`,
		"Dashboard Student "+uuid.NewString()[:12], "dash_"+uuid.NewString()[:12], status, createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed student with status: %v", err)
	}
	return id
}

// seedCountsExam inserts an exam row with an explicit title and optional
// scheduled_at (nil leaves it NULL) — unlike seedDashboardExam, which never
// sets scheduled_at.
func seedCountsExam(t *testing.T, r *Repository, title string, scheduledAt *time.Time) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO exam (title, scheduled_at) VALUES ($1, $2) RETURNING id`,
		title, scheduledAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	return id
}

// seedCountsExamRegistration registers a fresh student for the given exam.
// UpcomingExams counts registrations, not students, so each call needs its
// own student to avoid the (student_id, exam_id) uniqueness constraint.
func seedCountsExamRegistration(t *testing.T, r *Repository, examID string) string {
	t.Helper()
	studentID := seedDashboardStudent(t, r, time.Now())
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed exam_registration: %v", err)
	}
	return id
}

// seedCountsExamSessionWithStatus mirrors seedExamSession (dashboard_series_test.go)
// but parameterizes status instead of hardcoding 'submitted'.
func seedCountsExamSessionWithStatus(t *testing.T, r *Repository, studentID string, startedAt time.Time, status string) string {
	t.Helper()
	ctx := context.Background()
	examID := seedDashboardExam(t, r)

	var regID string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("seed exam_registration: %v", err)
	}

	var sessionID string
	err = r.pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		regID, studentID, examID, startedAt, status,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("seed exam_session: %v", err)
	}
	return sessionID
}

// seedCountsExamWithDuration inserts an exam with an explicit duration_minutes
// (nil leaves it NULL — "no timer", the per_test path).
func seedCountsExamWithDuration(t *testing.T, r *Repository, title string, durationMinutes *int) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO exam (title, duration_minutes) VALUES ($1, $2) RETURNING id`,
		title, durationMinutes,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed exam with duration: %v", err)
	}
	return id
}

// seedCountsExamSessionForExam mirrors seedCountsExamSessionWithStatus but
// takes an explicit examID (so the caller controls duration_minutes) and an
// optional extended_until, needed to exercise CountActiveExamSessions' deadline.
func seedCountsExamSessionForExam(
	t *testing.T, r *Repository, studentID, examID string, startedAt time.Time, status string, extendedUntil *time.Time,
) string {
	t.Helper()
	ctx := context.Background()

	var regID string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("seed exam_registration: %v", err)
	}

	var sessionID string
	err = r.pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, extended_until)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		regID, studentID, examID, startedAt, status, extendedUntil,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("seed exam_session: %v", err)
	}
	return sessionID
}

// TestCountActiveExamSessionsExcludesExpiredDeadline pins the fix for a
// disconnected student's session staying in_progress forever: only the
// deadline, not the stored status, decides whether it still counts.
//
//	timedExpired     started_at+duration in the past, no extension  -> excluded
//	timedLive        started_at+duration in the future              -> counted
//	timedExtended    nominal deadline passed, extended_until later  -> counted
//	timedShortExt    nominal deadline in the future, extended_until
//	                 EARLIER — must not shorten the deadline         -> counted
//	untimedStale     duration_minutes IS NULL, started long ago     -> counted
func TestCountActiveExamSessionsExcludesExpiredDeadline(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	sixty := 60
	examTimed := seedCountsExamWithDuration(t, r, "Timed Exam", &sixty)
	examUntimed := seedCountsExamWithDuration(t, r, "Untimed Exam", nil)

	before, err := r.CountActiveExamSessions(ctx)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	expiredStart := time.Now().Add(-3 * time.Hour)
	s1 := seedDashboardStudent(t, r, expiredStart)
	seedCountsExamSessionForExam(t, r, s1, examTimed, expiredStart, "in_progress", nil)

	liveStart := time.Now().Add(-5 * time.Minute)
	s2 := seedDashboardStudent(t, r, liveStart)
	seedCountsExamSessionForExam(t, r, s2, examTimed, liveStart, "in_progress", nil)

	extendedLater := time.Now().Add(1 * time.Hour)
	s3 := seedDashboardStudent(t, r, expiredStart)
	seedCountsExamSessionForExam(t, r, s3, examTimed, expiredStart, "in_progress", &extendedLater)

	extendedEarlier := time.Now().Add(-2 * time.Hour)
	s4 := seedDashboardStudent(t, r, liveStart)
	seedCountsExamSessionForExam(t, r, s4, examTimed, liveStart, "in_progress", &extendedEarlier)

	staleStart := time.Now().Add(-10 * time.Hour)
	s5 := seedDashboardStudent(t, r, staleStart)
	seedCountsExamSessionForExam(t, r, s5, examUntimed, staleStart, "in_progress", nil)

	after, err := r.CountActiveExamSessions(ctx)
	if err != nil {
		t.Fatalf("CountActiveExamSessions: %v", err)
	}
	if after-before != 4 {
		t.Errorf("delta = %d, want 4 (live, extended-later, extended-earlier-ignored, untimed-stale)", after-before)
	}
}

func TestCountActiveExamSessionsCountsOnlyInProgress(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, jkt)

	before, err := r.CountActiveExamSessions(ctx)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	s1 := seedDashboardStudent(t, r, now)
	s2 := seedDashboardStudent(t, r, now)
	seedCountsExamSessionWithStatus(t, r, s1, now, "in_progress")
	seedCountsExamSessionWithStatus(t, r, s2, now, "submitted")

	after, err := r.CountActiveExamSessions(ctx)
	if err != nil {
		t.Fatalf("CountActiveExamSessions: %v", err)
	}
	if after-before != 1 {
		t.Errorf("delta = %d, want 1 — only in_progress counts", after-before)
	}
}

func TestUpcomingExamsExcludesPastAndUnscheduled(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, jkt)
	horizon := now.AddDate(0, 0, 14)

	seedCountsExam(t, r, "Sudah lewat", ptrTime(now.AddDate(0, 0, -1)))
	seedCountsExam(t, r, "Belum dijadwalkan", nil)
	seedCountsExam(t, r, "Terlalu jauh", ptrTime(now.AddDate(0, 0, 30)))
	soon := seedCountsExam(t, r, "Try Out UTBK", ptrTime(now.AddDate(0, 0, 3)))
	seedCountsExamRegistration(t, r, soon)
	seedCountsExamRegistration(t, r, soon)

	exams, err := r.UpcomingExams(ctx, now, horizon, 10)
	if err != nil {
		t.Fatalf("UpcomingExams: %v", err)
	}
	if len(exams) != 1 {
		t.Fatalf("got %d exams, want 1: %+v", len(exams), exams)
	}
	if exams[0].Title != "Try Out UTBK" {
		t.Errorf("title = %q", exams[0].Title)
	}
	if exams[0].RegistrantCount != 2 {
		t.Errorf("registrant_count = %d, want 2", exams[0].RegistrantCount)
	}
}

func TestUpcomingExamsOrdersBySoonestFirst(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, jkt)

	seedCountsExam(t, r, "Nanti", ptrTime(now.AddDate(0, 0, 10)))
	seedCountsExam(t, r, "Segera", ptrTime(now.AddDate(0, 0, 2)))

	exams, err := r.UpcomingExams(ctx, now, now.AddDate(0, 0, 14), 10)
	if err != nil {
		t.Fatalf("UpcomingExams: %v", err)
	}
	if len(exams) != 2 || exams[0].Title != "Segera" {
		t.Errorf("want soonest first, got %+v", exams)
	}
}

// UpcomingExam.ScheduledAt must serialize with the Jakarta +07:00 offset, not
// a false UTC Z — Task 3 hands this straight to JSON and the frontend does
// date arithmetic on it. Unlike DashboardSeries' bucket dates, scheduled_at is
// a genuine timestamptz (not a naive value derived via AT TIME ZONE), so the
// correct fix is .In(jakarta) — reusing Task 1's time.Date reconstruction here
// would relabel the instant as Jakarta wall-clock and shift it by 7 hours.
func TestUpcomingExamsScheduledAtInJakartaOffset(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	scheduledAt := time.Date(2026, 12, 1, 9, 0, 0, 0, jkt)

	seedCountsExam(t, r, "Offset Check", ptrTime(scheduledAt))

	exams, err := r.UpcomingExams(ctx, scheduledAt.Add(-time.Hour), scheduledAt.Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("UpcomingExams: %v", err)
	}
	if len(exams) != 1 {
		t.Fatalf("got %d exams, want 1: %+v", len(exams), exams)
	}

	raw, err := json.Marshal(exams[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded struct {
		ScheduledAt string `json:"scheduled_at"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	const want = "2026-12-01T09:00:00+07:00"
	if decoded.ScheduledAt != want {
		t.Errorf("serialized scheduled_at = %q, want %q (Jakarta offset, not a false UTC Z)", decoded.ScheduledAt, want)
	}
}

func TestCountStudentsAndSchoolsExcludesDeleted(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 7, 3, 0, 0, 0, 0, jkt)

	beforeStudents, beforeSchools, err := r.CountStudentsAndSchools(ctx)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	seedDashboardStudent(t, r, now)
	seedCountsStudentWithStatus(t, r, now, "deleted")
	seedCountsSchool(t, r, "SMA 1")

	afterStudents, afterSchools, err := r.CountStudentsAndSchools(ctx)
	if err != nil {
		t.Fatalf("CountStudentsAndSchools: %v", err)
	}
	if afterStudents-beforeStudents != 1 {
		t.Errorf("students delta = %d, want 1 — deleted must not count", afterStudents-beforeStudents)
	}
	if afterSchools-beforeSchools != 1 {
		t.Errorf("schools delta = %d, want 1", afterSchools-beforeSchools)
	}
}
