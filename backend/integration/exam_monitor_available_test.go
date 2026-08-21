package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// setExamSchedule directly updates an exam's scheduling columns — seedExam has no
// scheduling params, and going through the admin PATCH endpoint would drag in
// unrelated validation for a test that only cares about the monitor's window math.
func setExamSchedule(t *testing.T, env *testEnv, examID string, scheduledAt, scheduledEndAt *time.Time, checkInMinutes, durationMinutes, graceMinutes *int) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`UPDATE exam SET scheduled_at = $2, scheduled_end_at = $3, check_in_window_minutes = $4,
			duration_minutes = $5, grace_window_minutes = $6 WHERE id = $1`,
		examID, scheduledAt, scheduledEndAt, checkInMinutes, durationMinutes, graceMinutes,
	)
	require.NoError(t, err)
}

// checkInRegistration stamps checked_in_at directly, bypassing the check-in window's own
// HTTP validation — irrelevant to a test asserting on the monitor's registration counts.
func checkInRegistration(t *testing.T, env *testEnv, registrationID string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `UPDATE exam_registration SET checked_in_at = now() WHERE id = $1`, registrationID)
	require.NoError(t, err)
}

// seedInProgressSession inserts an exam_session row directly, so a registration counts as
// "active" in the monitor without driving the full start-session HTTP flow (which requires
// linked tests/questions unrelated to this test).
func seedInProgressSession(t *testing.T, env *testEnv, registrationID, studentID, examID string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status)
		 VALUES ($1, $2, $3, now(), 'in_progress')`,
		registrationID, studentID, examID,
	)
	require.NoError(t, err)
}

func findMonitorAvailableRow(out map[string]any, examID string) map[string]any {
	for _, raw := range out["data"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == examID {
			return row
		}
	}
	return nil
}

func TestExam_AdminListExamsForMonitor_excludesExamOutsideItsWindow(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	pastExam := seedExam(t, env, "Ujian Selesai Lama", "score_pembahasan", nil)
	longAgo := time.Now().Add(-72 * time.Hour)
	setExamSchedule(t, env, pastExam, &longAgo, nil, intp(30), intp(60), intp(10))

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/monitor/available", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	require.Nil(t, findMonitorAvailableRow(out, pastExam), "exam long past its window+buffer must not appear")
}

func TestExam_AdminListExamsForMonitor_includesGrantOnlyExamWithinWindow(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	// No product is ever linked to this exam — mirrors a client granted direct
	// access without a purchase, which has_published_product used to miss entirely.
	liveExam := seedExam(t, env, "Ujian Grant Langsung", "score_pembahasan", nil)
	startedAgo := time.Now().Add(-10 * time.Minute)
	setExamSchedule(t, env, liveExam, &startedAgo, nil, intp(30), intp(120), intp(10))

	notStartedStudent := seedUser(t, env, "student", "active", false)
	seedExamRegistration(t, env, notStartedStudent, liveExam)

	checkedInStudent := seedUser(t, env, "student", "active", false)
	checkedInReg := seedExamRegistration(t, env, checkedInStudent, liveExam)
	checkInRegistration(t, env, checkedInReg)

	inProgressStudent := seedUser(t, env, "student", "active", false)
	inProgressReg := seedExamRegistration(t, env, inProgressStudent, liveExam)
	checkInRegistration(t, env, inProgressReg)
	seedInProgressSession(t, env, inProgressReg, inProgressStudent, liveExam)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/monitor/available", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)

	row := findMonitorAvailableRow(out, liveExam)
	require.NotNil(t, row, "an exam with no linked product must still be available once inside its window")
	require.Equal(t, "live", row["state"])
	require.Equal(t, float64(3), row["total_registered"])
	require.Equal(t, float64(2), row["active_count"], "checked-in and in-progress both count as active")
	require.Equal(t, float64(1), row["not_started_count"])
}

func TestExam_AdminListExamsForMonitor_perTestExamUsesSectionsDuration(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	// seedExam defaults timer_mode='per_test' (duration_minutes stays NULL); two
	// 90-minute sections give the exam a real 180-minute run time that only shows
	// up via exam_test/test, never on the exam row itself.
	perTestExam := seedExam(t, env, "UTBK Per-Test", "score_pembahasan", nil)
	testA := seedTest(t, env, "Section A", "math", "algebra", 90)
	testB := seedTest(t, env, "Section B", "science", "biology", 90)
	linkExamTest(t, env, perTestExam, testA, 0)
	linkExamTest(t, env, perTestExam, testB, 1)

	startedAgo := time.Now().Add(-170 * time.Minute)
	setExamSchedule(t, env, perTestExam, &startedAgo, nil, intp(30), nil, intp(10))

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/monitor/available", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)

	row := findMonitorAvailableRow(out, perTestExam)
	require.NotNil(t, row, "a per_test exam 170min into a 180min run must still be live, not dropped after just the grace window")
	require.Equal(t, "live", row["state"])
}

func TestExam_AdminListExamsForMonitor_retakeSessionsCountOnceByRegistration(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	liveExam := seedExam(t, env, "Ujian Retake", "score_pembahasan", nil)
	startedAgo := time.Now().Add(-10 * time.Minute)
	setExamSchedule(t, env, liveExam, &startedAgo, nil, intp(30), intp(120), intp(10))

	student := seedUser(t, env, "student", "active", false)
	reg := seedExamRegistration(t, env, student, liveExam)
	checkInRegistration(t, env, reg)

	// A finished first attempt plus an active retake — both exam_session rows
	// belong to the same registration and must count as one registrant.
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at, status)
		 VALUES ($1, $2, $3, now() - interval '1 hour', now() - interval '30 minutes', 'submitted')`,
		reg, student, liveExam,
	)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status)
		 VALUES ($1, $2, $3, 2, now(), 'in_progress')`,
		reg, student, liveExam,
	)
	require.NoError(t, err)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/monitor/available", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)

	row := findMonitorAvailableRow(out, liveExam)
	require.NotNil(t, row)
	require.Equal(t, float64(1), row["total_registered"], "a retake must not double-count its registration")
	require.Equal(t, float64(1), row["active_count"])
	require.Equal(t, float64(0), row["not_started_count"])

	detailResp, detailOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/monitor?exam_id="+liveExam, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode, "body=%v", detailOut)
	detailRows := detailOut["rows"].([]any)
	require.Len(t, detailRows, 1, "the monitor detail must return one row per registration")
	require.Equal(t, "in_progress", detailRows[0].(map[string]any)["status"])
}

func intp(i int) *int { return &i }
