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
// The nil-scope check is a before/after delta, not an absolute count, since
// the shared pool's rows persist across the package's other tests.
func TestSchoolDashboardCountsExcludesOtherSchools(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	before, err := r.SchoolDashboardCounts(ctx, nil, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("SchoolDashboardCounts(nil) baseline: %v", err)
	}

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

	after, err := r.SchoolDashboardCounts(ctx, nil, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("SchoolDashboardCounts(nil): %v", err)
	}
	if after.Students-before.Students != 5 {
		t.Errorf("platform-wide Students delta = %d, want 5 — the super_admin, platform-wide view",
			after.Students-before.Students)
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

// seedBulkOrderBuyer's fresh gen_random_uuid() id keeps it isolated from other tests' orders.
func seedBulkOrderBuyer(t *testing.T, r *Repository) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'admin_school', 'active', false)
		 RETURNING id`,
		"Dashboard Buyer "+uuid.NewString()[:12], "buyer_"+uuid.NewString()[:12],
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed bulk order buyer: %v", err)
	}
	return id
}

// seedOrderForBuyer lets created_at and checked_out_at diverge, so tests can desync mint from checkout.
func seedOrderForBuyer(
	t *testing.T, r *Repository, buyerID, status string, total float64, createdAt time.Time, checkedOutAt *time.Time,
) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO orders (student_id, status, subtotal, total, created_at, checked_out_at)
		 VALUES ($1, $2, $3, $3, $4, $5)
		 RETURNING id`,
		buyerID, status, total, createdAt, checkedOutAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return id
}

func seedOrderParticipant(t *testing.T, r *Repository, orderID, studentID string) {
	t.Helper()
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`,
		orderID, studentID,
	)
	if err != nil {
		t.Fatalf("seed order_participant: %v", err)
	}
}

// Order A mints first but checks out last; a regression to bare created_at would return B instead of A.
func TestLatestBulkExamOrderOrdersByCheckoutNotCreation(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	buyer := seedBulkOrderBuyer(t, r)
	participant := seedDashboardStudent(t, r, time.Now())

	mintA := time.Now().Add(-30 * 24 * time.Hour)
	checkoutA := time.Now().Add(-1 * time.Hour)
	orderA := seedOrderForBuyer(t, r, buyer, "paid", 100000, mintA, &checkoutA)
	seedOrderParticipant(t, r, orderA, participant)

	mintB := time.Now().Add(-2 * time.Hour)
	checkoutB := time.Now().Add(-20 * 24 * time.Hour)
	orderB := seedOrderForBuyer(t, r, buyer, "paid", 200000, mintB, &checkoutB)
	seedOrderParticipant(t, r, orderB, participant)

	latest, err := r.LatestBulkExamOrder(ctx, buyer)
	if err != nil {
		t.Fatalf("LatestBulkExamOrder: %v", err)
	}
	if latest == nil {
		t.Fatalf("LatestBulkExamOrder = nil, want order A (%s)", orderA)
	}
	if latest.ID.String() != orderA {
		t.Errorf("LatestBulkExamOrder = %s, want %s — A checked out most recently despite minting first", latest.ID, orderA)
	}
}

// A checked-out order with no order_participant rows is a personal purchase, not a bulk order.
func TestLatestBulkExamOrderIgnoresPersonalPurchases(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	buyer := seedBulkOrderBuyer(t, r)
	checkoutAt := time.Now().Add(-1 * time.Hour)
	seedOrderForBuyer(t, r, buyer, "paid", 50000, time.Now().Add(-2*time.Hour), &checkoutAt)

	latest, err := r.LatestBulkExamOrder(ctx, buyer)
	if err != nil {
		t.Fatalf("LatestBulkExamOrder: %v", err)
	}
	if latest != nil {
		t.Errorf("LatestBulkExamOrder = %+v, want nil — a personal purchase is not a bulk order", latest)
	}
}

// seedSubmittedSession's score is nullable — pass nil for an ungraded session.
func seedSubmittedSession(t *testing.T, r *Repository, studentID string, submittedAt time.Time, score *float64) string {
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
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at, score, status)
		 VALUES ($1, $2, $3, $4, $4, $5, 'submitted') RETURNING id`,
		regID, studentID, examID, submittedAt, score,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("seed submitted session: %v", err)
	}
	return sessionID
}

// A submitted session with score IS NULL must decode as a nil pointer, not 0.
func TestRecentSchoolResultsNullScoreStaysNil(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	school := seedCountsSchool(t, r, "School D "+uuid.NewString()[:8])
	student := seedCountsStudentForSchool(t, r, school, time.Now())
	seedSubmittedSession(t, r, student, time.Now(), nil)

	results, err := r.RecentSchoolResults(ctx, &school, 5)
	if err != nil {
		t.Fatalf("RecentSchoolResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(results), results)
	}
	if results[0].Score != nil {
		t.Errorf("Score = %v, want nil — ungraded must not read as a real 0", *results[0].Score)
	}
}
