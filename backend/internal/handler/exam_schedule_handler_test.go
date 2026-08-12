package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

// ---------------------------------------------------------------------------
// AdminUpdateExam schedule-window round-trip tests
//
// scheduled_end_at was absent from examPatchRequest, so PATCH silently wrote
// the stored value back and the admin's edit never landed.
// ---------------------------------------------------------------------------

func TestAdminUpdateExam_ScheduledEndAt_RoundTrips(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Schedule")

	examID := seedExam(t, env.pool, "Schedule Window Exam", false, "hidden", "classic")

	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{
			"scheduled_at":     start.Format(time.RFC3339),
			"scheduled_end_at": end.Format(time.RFC3339),
		},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var gotStart, gotEnd *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT scheduled_at, scheduled_end_at FROM exam WHERE id = $1`, examID,
	).Scan(&gotStart, &gotEnd); err != nil {
		t.Fatalf("query schedule columns: %v", err)
	}
	if gotStart == nil || !gotStart.Equal(start) {
		t.Errorf("persisted scheduled_at: want %v, got %v", start, gotStart)
	}
	if gotEnd == nil || !gotEnd.Equal(end) {
		t.Errorf("persisted scheduled_end_at: want %v, got %v", end, gotEnd)
	}
}

// TestAdminUpdateExam_ScheduledEndAt_Moves is the reported bug: an exam that
// already has a window, whose end an admin then moves.
func TestAdminUpdateExam_ScheduledEndAt_Moves(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Move Schedule")

	examID := seedExam(t, env.pool, "Move Window Exam", false, "hidden", "classic")
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	oldEnd := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET scheduled_at = $1, scheduled_end_at = $2 WHERE id = $3`,
		start, oldEnd, examID,
	); err != nil {
		t.Fatalf("seed schedule columns: %v", err)
	}

	newEnd := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{
			"scheduled_at":     start.Format(time.RFC3339),
			"scheduled_end_at": newEnd.Format(time.RFC3339),
		},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var gotEnd *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT scheduled_end_at FROM exam WHERE id = $1`, examID,
	).Scan(&gotEnd); err != nil {
		t.Fatalf("query scheduled_end_at: %v", err)
	}
	if gotEnd == nil || !gotEnd.Equal(newEnd) {
		t.Errorf("scheduled_end_at: want moved to %v, got %v", newEnd, gotEnd)
	}
}

func TestAdminUpdateExam_ScheduledEndAt_ExplicitNullClears(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Clear Schedule")

	examID := seedExam(t, env.pool, "Clear Window Exam", false, "hidden", "classic")
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET scheduled_at = $1, scheduled_end_at = $2 WHERE id = $3`,
		start, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), examID,
	); err != nil {
		t.Fatalf("seed schedule columns: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"scheduled_end_at": nil},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var gotEnd *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT scheduled_end_at FROM exam WHERE id = $1`, examID,
	).Scan(&gotEnd); err != nil {
		t.Fatalf("query scheduled_end_at: %v", err)
	}
	if gotEnd != nil {
		t.Errorf("scheduled_end_at: want cleared (nil), got %v", *gotEnd)
	}
}

func TestAdminUpdateExam_ScheduledEndAt_OmittedPreservesExisting(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preserve Schedule")

	examID := seedExam(t, env.pool, "Preserve Window Exam", false, "hidden", "classic")
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET scheduled_at = $1, scheduled_end_at = $2 WHERE id = $3`,
		start, end, examID,
	); err != nil {
		t.Fatalf("seed schedule columns: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var gotEnd *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT scheduled_end_at FROM exam WHERE id = $1`, examID,
	).Scan(&gotEnd); err != nil {
		t.Fatalf("query scheduled_end_at: %v", err)
	}
	if gotEnd == nil || !gotEnd.Equal(end) {
		t.Errorf("scheduled_end_at: want preserved as %v, got %v", end, gotEnd)
	}
}
