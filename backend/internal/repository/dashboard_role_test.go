package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// seedCountsStudentForSchool mirrors seedCountsStudentWithStatus but binds the
// student to a school, needed to pin SchoolDashboardCounts' scoping.
func seedCountsStudentForSchool(t *testing.T, r *Repository, schoolID string, createdAt time.Time) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, school_id, jenjang, grade, otp_enabled, created_at)
		 VALUES ($1, $2, 'student', 'active', $3, 'sma', 10, false, $4)
		 RETURNING id`,
		"Dashboard Student "+uuid.NewString()[:12], "dash_"+uuid.NewString()[:12], schoolID, createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed student for school: %v", err)
	}
	return id
}

// TestSchoolDashboardCountsExcludesOtherSchools pins the scope. A count that
// silently spans every school is the failure mode this endpoint exists to
// avoid, and it looks identical to a correct one on a single-school fixture.
func TestSchoolDashboardCountsExcludesOtherSchools(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	schoolA := seedCountsSchool(t, r, "School A "+uuid.NewString()[:8])
	schoolB := seedCountsSchool(t, r, "School B "+uuid.NewString()[:8])

	for i := 0; i < 2; i++ {
		seedCountsStudentForSchool(t, r, schoolA, now)
	}
	for i := 0; i < 3; i++ {
		seedCountsStudentForSchool(t, r, schoolB, now)
	}

	scoped, err := r.SchoolDashboardCounts(ctx, &schoolA, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("SchoolDashboardCounts(schoolA): %v", err)
	}
	if scoped.Students != 2 {
		t.Errorf("schoolA Students = %d, want 2 (not 5 — must not span schoolB)", scoped.Students)
	}

	platform, err := r.SchoolDashboardCounts(ctx, nil, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("SchoolDashboardCounts(nil): %v", err)
	}
	if platform.Students != 5 {
		t.Errorf("platform-wide Students = %d, want 5 — the super_admin, platform-wide view", platform.Students)
	}
}

// TestSchoolDashboardCountsNewStudentsMonthWindowed pins the COUNT(*) FILTER:
// a student created outside the given month window must not count toward
// new-student intake even though they still count in Students.
func TestSchoolDashboardCountsNewStudentsMonthWindowed(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	monthStart := time.Date(2026, 9, 1, 0, 0, 0, 0, jkt)
	monthEnd := monthStart.AddDate(0, 1, 0)

	school := seedCountsSchool(t, r, "School C "+uuid.NewString()[:8])

	seedCountsStudentForSchool(t, r, school, time.Date(2026, 9, 15, 0, 0, 0, 0, jkt))
	seedCountsStudentForSchool(t, r, school, time.Date(2026, 8, 20, 0, 0, 0, 0, jkt))

	counts, err := r.SchoolDashboardCounts(ctx, &school, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("SchoolDashboardCounts: %v", err)
	}
	if counts.Students != 2 {
		t.Errorf("Students = %d, want 2", counts.Students)
	}
	if counts.NewStudentsMonth != 1 {
		t.Errorf("NewStudentsMonth = %d, want 1 — the August student must be excluded", counts.NewStudentsMonth)
	}
}
