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

// ---------------------------------------------------------------------------
// card_notes round-trip (PR #87 review, P1)
//
// The frontend test only asserted the outgoing payload, so it could not see
// that examPatchRequest had no card_notes field: the PATCH 200'd, the UI closed,
// and the stored notes never changed.
// ---------------------------------------------------------------------------

func TestAdminUpdateExam_CardNotes_PersistsToDatabase(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Card Notes")
	examID := seedExam(t, env.pool, "Card Notes Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"card_notes": []string{"Bawa kartu identitas.", "  ", "Datang lebih awal."}},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var stored []string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_notes FROM exam WHERE id = $1`, examID,
	).Scan(&stored); err != nil {
		t.Fatalf("query card_notes: %v", err)
	}
	want := []string{"Bawa kartu identitas.", "Datang lebih awal."}
	if len(stored) != len(want) {
		t.Fatalf("persisted card_notes = %q, want %q (blank entry dropped)", stored, want)
	}
	for i := range want {
		if stored[i] != want[i] {
			t.Errorf("note %d = %q, want %q", i, stored[i], want[i])
		}
	}
}

func TestAdminUpdateExam_CardNotes_OmittedPreservesExisting(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Keep Card Notes")
	examID := seedExam(t, env.pool, "Keep Card Notes Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET card_notes = $1 WHERE id = $2`, `["Keep me."]`, examID,
	); err != nil {
		t.Fatalf("seed card_notes: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var stored []string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_notes FROM exam WHERE id = $1`, examID,
	).Scan(&stored); err != nil {
		t.Fatalf("query card_notes: %v", err)
	}
	if len(stored) != 1 || stored[0] != "Keep me." {
		t.Errorf("card_notes: want preserved [\"Keep me.\"], got %q", stored)
	}
}

// TestAdminUpdateExam_CardNotes_EmptyArrayClears proves the *[]string choice:
// an explicit [] is distinguishable from an absent key and resets the exam to
// the built-in defaults.
func TestAdminUpdateExam_CardNotes_EmptyArrayClears(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Clear Card Notes")
	examID := seedExam(t, env.pool, "Clear Card Notes Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET card_notes = $1 WHERE id = $2`, `["Remove me."]`, examID,
	); err != nil {
		t.Fatalf("seed card_notes: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"card_notes": []string{}},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var stored []string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_notes FROM exam WHERE id = $1`, examID,
	).Scan(&stored); err != nil {
		t.Fatalf("query card_notes: %v", err)
	}
	if len(stored) != 0 {
		t.Errorf("card_notes: want cleared, got %q", stored)
	}
}

// TestAdminUpdateExam_CardNotesChange_InvalidatesCachedCard covers the second
// P1: GetExamCard presigns an existing card_key without re-rendering, so an
// edit had to drop the cache or it would never reach an already-downloaded card.
func TestAdminUpdateExam_CardNotesChange_InvalidatesCachedCard(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Invalidate Card")
	student := seedUser(t, env.pool, "student", "Card Holder")
	examID := seedExam(t, env.pool, "Invalidate Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, "cards/cached.pdf", regID,
	); err != nil {
		t.Fatalf("seed card_key: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"card_notes": []string{"Aturan baru."}},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var cardKey *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_key FROM exam_registration WHERE id = $1`, regID,
	).Scan(&cardKey); err != nil {
		t.Fatalf("query card_key: %v", err)
	}
	if cardKey != nil {
		t.Errorf("card_key: want cleared so the next download re-renders, got %q", *cardKey)
	}
}

// An unrelated PATCH must NOT throw away every cached card.
func TestAdminUpdateExam_UnrelatedEdit_KeepsCachedCard(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Keep Cached Card")
	student := seedUser(t, env.pool, "student", "Card Holder Two")
	examID := seedExam(t, env.pool, "Keep Cached Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, "cards/keep.pdf", regID,
	); err != nil {
		t.Fatalf("seed card_key: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"title": "Renamed Exam"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var cardKey *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_key FROM exam_registration WHERE id = $1`, regID,
	).Scan(&cardKey); err != nil {
		t.Fatalf("query card_key: %v", err)
	}
	if cardKey == nil || *cardKey != "cards/keep.pdf" {
		t.Errorf("card_key: want the cached key preserved, got %v", cardKey)
	}
}
