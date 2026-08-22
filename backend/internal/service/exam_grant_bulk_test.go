package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestGrantExamAccessBulk_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var actorID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"Bulk Grant Actor "+uniqueSuffix(), "bg_actor_"+uniqueSuffix()[:5],
	).Scan(&actorID)
	if err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	var examID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, timer_mode, result_config, mode)
		 VALUES ($1, 'active', 'manual', 'score_only', 'standard')
		 RETURNING id`,
		"Bulk Grant Exam "+uniqueSuffix(),
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	studentUsername := "bg_stu_" + uniqueSuffix()[:8]
	var studentID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'student', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Bulk Grant Student "+uniqueSuffix(), studentUsername,
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	nonStudentUsername := "bg_nonstu_" + uniqueSuffix()[:5]
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'admin_school', 'active', $2, '')`,
		"Bulk Grant Non Student "+uniqueSuffix(), nonStudentUsername,
	)
	if err != nil {
		t.Fatalf("insert non-student: %v", err)
	}

	alreadyRegisteredUsername := "bg_reg_" + uniqueSuffix()[:8]
	var alreadyRegisteredID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'student', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Bulk Grant Already Registered "+uniqueSuffix(), alreadyRegisteredUsername,
	).Scan(&alreadyRegisteredID)
	if err != nil {
		t.Fatalf("insert already-registered student: %v", err)
	}
	// Use a separate setup actor so the pre-grant call's own audit log entry
	// doesn't inflate the count asserted for the bulk-grant actor below.
	var setupActorID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"Bulk Grant Setup Actor "+uniqueSuffix(), "bg_setup_"+uniqueSuffix()[:5],
	).Scan(&setupActorID)
	if err != nil {
		t.Fatalf("insert setup actor: %v", err)
	}
	if _, err := svc.GrantExamAccess(ctx, setupActorID.String(), examID.String(), []uuid.UUID{alreadyRegisteredID}); err != nil {
		t.Fatalf("pre-grant already-registered student: %v", err)
	}

	usernames := []string{
		"",                          // blank -> failed
		studentUsername,             // ok -> granted
		studentUsername,             // duplicate row -> skipped
		"nonexistent_" + uniqueSuffix()[:8], // not found -> failed
		nonStudentUsername,          // wrong role -> failed
		alreadyRegisteredUsername,   // already registered -> skipped
	}

	results, err := svc.GrantExamAccessBulk(ctx, actorID.String(), examID.String(), usernames)
	if err != nil {
		t.Fatalf("GrantExamAccessBulk: %v", err)
	}
	if len(results) != len(usernames) {
		t.Fatalf("want %d results, got %d", len(usernames), len(results))
	}

	wantStatus := []string{"failed", "granted", "skipped", "failed", "failed", "skipped"}
	for i, want := range wantStatus {
		if results[i].Status != want {
			t.Errorf("row %d (%q): want status %q, got %q (message=%q)", i, usernames[i], want, results[i].Status, results[i].Message)
		}
	}

	// Verify the granted registration actually exists.
	var count int
	err = repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count registration: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 registration for granted student, got %d", count)
	}

	// Verify audit log entry was written for the bulk grant.
	var auditCount int
	err = repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE actor_id = $1 AND action = 'exam_grant.create'`,
		actorID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("count audit log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("want 1 audit log entry, got %d", auditCount)
	}
}

func TestGrantExamAccessBulk_AllRowsInvalid(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var actorID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"Bulk Grant Actor2 "+uniqueSuffix(), "bg_actor2_"+uniqueSuffix()[:5],
	).Scan(&actorID)
	if err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	var examID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, timer_mode, result_config, mode)
		 VALUES ($1, 'active', 'manual', 'score_only', 'standard')
		 RETURNING id`,
		"Bulk Grant Exam2 "+uniqueSuffix(),
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	results, err := svc.GrantExamAccessBulk(ctx, actorID.String(), examID.String(), []string{"", "  ", "nope_" + uniqueSuffix()[:8]})
	if err != nil {
		t.Fatalf("GrantExamAccessBulk: %v", err)
	}
	for i, r := range results {
		if r.Status != "failed" {
			t.Errorf("row %d: want failed, got %s", i, r.Status)
		}
	}

	var auditCount int
	err = repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE actor_id = $1 AND action = 'exam_grant.create'`,
		actorID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("count audit log: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("want 0 audit log entries when nothing granted, got %d", auditCount)
	}
}
