package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestGrantExamAccess_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	// Create an actor (super_admin user) for the audit log — actor_id is UUID.
	var actorID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"Grant Actor "+uniqueSuffix(), "g_actor_"+uniqueSuffix()[:5],
	).Scan(&actorID)
	if err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	// Create an exam
	var examID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, timer_mode, result_config, mode)
		 VALUES ($1, 'active', 'manual', 'score_only', 'standard')
		 RETURNING id`,
		"Grant Exam "+uniqueSuffix(),
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	schoolIDs := make([]string, 2)
	for i := range schoolIDs {
		var sid string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO school (name, code, status) VALUES ($1, $2, 'active') RETURNING id`,
			"Grant School "+uniqueSuffix(), "GS"+uniqueSuffix()[:5],
		).Scan(&sid)
		if err != nil {
			t.Fatalf("insert school %d: %v", i, err)
		}
		schoolIDs[i] = sid
	}

	// Create 3 students: 2 in school 0, 1 in school 1
	studentIDs := make([]uuid.UUID, 3)
	for i := 0; i < 2; i++ {
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
			 RETURNING id`,
			"Grant Student "+uniqueSuffix(), schoolIDs[0], "g_stu_"+uniqueSuffix()[:6],
		).Scan(&studentIDs[i])
		if err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
	}
	// Student in different school
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
		 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
		 RETURNING id`,
		"Grant Cross School Student "+uniqueSuffix(), schoolIDs[1], "g_cross_"+uniqueSuffix()[:5],
	).Scan(&studentIDs[2])
	if err != nil {
		t.Fatalf("insert cross-school student: %v", err)
	}

	// Create a non-student user (should fail)
	var nonStudentID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'admin_school', 'active', $2, '')
		 RETURNING id`,
		"Non Student "+uniqueSuffix(), "g_nonstu_"+uniqueSuffix()[:5],
	).Scan(&nonStudentID)
	if err != nil {
		t.Fatalf("insert admin user: %v", err)
	}

	actorStr := actorID.String()

	t.Run("happy path — cross-school grant succeeds", func(t *testing.T) {
		result, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), studentIDs)
		if err != nil {
			t.Fatalf("GrantExamAccess: %v", err)
		}
		if result.GrantedCount != 3 {
			t.Fatalf("want 3 granted students, got %d", result.GrantedCount)
		}
		if len(result.GrantedStudents) != 3 {
			t.Fatalf("want 3 GrantedStudents entries, got %d", len(result.GrantedStudents))
		}
		for _, gs := range result.GrantedStudents {
			if gs.ID == "" {
				t.Error("student ID should not be empty")
			}
			if gs.Name == "" {
				t.Error("student Name should not be empty")
			}
		}

		// Verify audit log entry
		var auditCount int
		err = repo.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_log
			 WHERE actor_id = $1 AND action = 'exam_grant.create'`,
			actorID,
		).Scan(&auditCount)
		if err != nil {
			t.Fatalf("count audit log: %v", err)
		}
		if auditCount != 1 {
			t.Errorf("want 1 audit log entry, got %d", auditCount)
		}
	})

	t.Run("granted registrations get a participant_number (FB-28)", func(t *testing.T) {
		// AllocateCertificateNumber (exam.go:1849-1860) scans reg.participant_number
		// into a non-nullable int; a grant-path registration left it NULL, so
		// resolveCertificateURL failed for every student who reached an exam via
		// grant rather than checkout — reproduced live, see task_11_finding.md.
		rows, err := repo.Pool().Query(ctx,
			`SELECT participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = ANY($2)`,
			examID, studentIDs,
		)
		if err != nil {
			t.Fatalf("query participant_number: %v", err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var pn *int
			if err := rows.Scan(&pn); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if pn == nil {
				t.Error("participant_number is NULL for a grant-path registration")
			}
			count++
		}
		if count != 3 {
			t.Fatalf("want 3 registrations, got %d", count)
		}
	})

	t.Run("non-existent student fails whole request", func(t *testing.T) {
		bogus := uuid.New()
		_, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentIDs[0], bogus})
		if err == nil {
			t.Fatal("expected error for non-existent student, got nil")
		}
	})

	t.Run("non-student user fails whole request", func(t *testing.T) {
		_, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{nonStudentID})
		if err == nil {
			t.Fatal("expected error for non-student user, got nil")
		}
	})

	t.Run("already-registered silently skipped", func(t *testing.T) {
		// Grant for studentIDs[0] again (already registered from the first test)
		result, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentIDs[0]})
		if err != nil {
			t.Fatalf("GrantExamAccess (repeat): %v", err)
		}
		// Should return no new registrations (ON CONFLICT DO NOTHING)
		if result.GrantedCount != 0 {
			t.Fatalf("want 0 new registrations (dedup), got %d", result.GrantedCount)
		}
		if len(result.GrantedStudents) != 0 {
			t.Fatalf("want 0 GrantedStudents entries (dedup), got %d", len(result.GrantedStudents))
		}
	})
}

// TestGetExamRegistrations_Integration covers the student-facing exam list
// endpoint (GET /exam/registrations) — specifically that is_free/
// requires_checkin/check_in_window_minutes/duration_minutes now come through
// from the joined exam row (added so the student Kompetisi page can render
// per-registration card state without a second round-trip per exam).
func TestGetExamRegistrations_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var actorID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"List Actor "+uniqueSuffix(), "l_actor_"+uniqueSuffix()[:5],
	).Scan(&actorID)
	if err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	windowMin := 30
	durationMin := 90
	var examID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, timer_mode, result_config, mode, is_free, requires_checkin, check_in_window_minutes, duration_minutes)
		 VALUES ($1, 'active', 'overall', 'score_only', 'standard', true, true, $2, $3)
		 RETURNING id`,
		"List Exam "+uniqueSuffix(), windowMin, durationMin,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	var studentID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'student', 'active', $2, '', 'sma')
		 RETURNING id`,
		"List Student "+uniqueSuffix(), "l_stu_"+uniqueSuffix()[:6],
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), []uuid.UUID{studentID}); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}

	items, err := svc.GetExamRegistrations(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetExamRegistrations: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 registration, got %d", len(items))
	}
	item := items[0]
	if !item.IsFree {
		t.Error("want IsFree=true")
	}
	if !item.RequiresCheckin {
		t.Error("want RequiresCheckin=true")
	}
	if item.CheckInWindowMinutes == nil || *item.CheckInWindowMinutes != windowMin {
		t.Errorf("want CheckInWindowMinutes=%d, got %v", windowMin, item.CheckInWindowMinutes)
	}
	if item.DurationMinutes == nil || *item.DurationMinutes != durationMin {
		t.Errorf("want DurationMinutes=%d, got %v", durationMin, item.DurationMinutes)
	}
}
