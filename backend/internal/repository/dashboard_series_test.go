package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// seedDashboardStudent inserts a student user with an explicit created_at,
// following the style of insertNullSchoolStudent (admin_students_test.go).
// Named distinctly from seedStudent (order_revenue_test.go): that helper
// takes a name instead of a timestamp and returns uuid.UUID, not a string id.
func seedDashboardStudent(t *testing.T, r *Repository, createdAt time.Time) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, jenjang, grade, otp_enabled, created_at)
		 VALUES ($1, $2, 'student', 'active', 'sma', 10, false, $3)
		 RETURNING id`,
		"Dashboard Student "+uuid.NewString()[:12], "dash_"+uuid.NewString()[:12], createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed dashboard student: %v", err)
	}
	return id
}

// seedPaidOrder inserts one paid order with the given total and no line
// items, for tests that only assert on aggregate revenue/order_count.
func seedPaidOrder(t *testing.T, r *Repository, createdAt time.Time, total float64) string {
	t.Helper()
	studentID := seedDashboardStudent(t, r, createdAt)
	var orderID string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO orders (student_id, status, subtotal, total, created_at)
		 VALUES ($1, 'paid', $2, $2, $3) RETURNING id`,
		studentID, total, createdAt,
	).Scan(&orderID)
	if err != nil {
		t.Fatalf("seed paid order: %v", err)
	}
	return orderID
}

// dashSeedItem describes one order_item row. Named distinctly from seedItem
// (order_revenue_test.go), which has different, unexported fields built for
// seedOrder rather than seedPaidOrderWithItems.
type dashSeedItem struct {
	ProductType string
	Amount      float64
	Qty         int
}

// seedPaidOrderWithItems inserts one paid order plus line items, each on its
// own freshly seeded product. jumlah is set to Amount*Qty per item so the
// digital/physical split sums item lines, independent of the order total.
func seedPaidOrderWithItems(t *testing.T, r *Repository, createdAt time.Time, total float64, items []dashSeedItem) string {
	t.Helper()
	ctx := context.Background()
	studentID := seedDashboardStudent(t, r, createdAt)

	var orderID string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total, created_at)
		 VALUES ($1, 'paid', $2, $2, $3) RETURNING id`,
		studentID, total, createdAt,
	).Scan(&orderID)
	if err != nil {
		t.Fatalf("seed paid order: %v", err)
	}

	for _, it := range items {
		productID := seedProduct(t, r.pool, "Dashboard Product "+uuid.NewString()[:12], it.ProductType, it.Amount)
		_, err := r.pool.Exec(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			orderID, productID, it.ProductType, "Dashboard Item", it.Amount, it.Qty, it.Amount*float64(it.Qty),
		)
		if err != nil {
			t.Fatalf("seed order item: %v", err)
		}
	}
	return orderID
}

// seedDashboardExam inserts a bare exam row with no test/questions attached
// — exam_session only needs a valid exam_id to reference.
func seedDashboardExam(t *testing.T, r *Repository) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO exam (title) VALUES ($1) RETURNING id`,
		"Dashboard Exam "+uuid.NewString()[:12],
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed dashboard exam: %v", err)
	}
	return id
}

// seedExamSession inserts a fresh exam + registration + session for the
// given student at startedAt. Each call seeds its own exam, so calling it
// twice for the same student (simulating a retake) never collides with the
// exam_registration (student_id, exam_id) uniqueness constraint.
func seedExamSession(t *testing.T, r *Repository, studentID string, startedAt time.Time) string {
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
		 VALUES ($1, $2, $3, $4, 'submitted') RETURNING id`,
		regID, studentID, examID, startedAt,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("seed exam_session: %v", err)
	}
	return sessionID
}

func TestDashboardSeriesFillsEmptyDays(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 7, 6, 0, 0, 0, 0, jkt) // half-open: 1..5

	seedPaidOrder(t, r, time.Date(2026, 7, 3, 10, 0, 0, 0, jkt), 100000)

	pts, err := r.DashboardSeries(context.Background(), from, to, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}

	// A line chart that skips empty days misreports the shape of the trend.
	if len(pts) != 5 {
		t.Fatalf("got %d points, want 5 (one per day in range)", len(pts))
	}
	if pts[0].Revenue != 0 || pts[1].Revenue != 0 {
		t.Errorf("empty days should be zero, got %v and %v", pts[0].Revenue, pts[1].Revenue)
	}
	if pts[2].Revenue != 100000 {
		t.Errorf("day 3 revenue = %v, want 100000", pts[2].Revenue)
	}
}

func TestDashboardSeriesBucketsInJakarta(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	// A different month than TestDashboardSeriesFillsEmptyDays — the grading
	// pool is shared and never reset, so two tests both dating their data to
	// July 2026 would double-count each other's orders on overlapping days.
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 8, 3, 0, 0, 0, 0, jkt)

	// 08:00 WIB on the 1st is 01:00 UTC on the 1st — but 23:00 WIB on the 1st
	// is 16:00 UTC on the 1st, and 02:00 WIB on the 2nd is 19:00 UTC on the 1st.
	// The last of those is the one UTC bucketing gets wrong.
	seedPaidOrder(t, r, time.Date(2026, 8, 2, 2, 0, 0, 0, jkt), 50000)

	pts, err := r.DashboardSeries(context.Background(), from, to, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}

	if pts[0].Revenue != 0 {
		t.Errorf("1 Jul should be empty, got %v — bucketing is in UTC", pts[0].Revenue)
	}
	if pts[1].Revenue != 50000 {
		t.Errorf("2 Jul revenue = %v, want 50000", pts[1].Revenue)
	}
}

func TestDashboardSeriesCountsOrderTotalOncePerOrder(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	// A day not touched by any other test in this file — the grading pool is
	// shared and never reset between tests.
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	day := time.Date(2026, 7, 10, 10, 0, 0, 0, jkt)
	from := time.Date(2026, 7, 10, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 7, 11, 0, 0, 0, 0, jkt)

	// Three items on one order. Joining order_item and summing orders.total
	// counts it three times — the PR #80 fan-out bug.
	orderID := seedPaidOrderWithItems(t, r, day, 300000, []dashSeedItem{
		{ProductType: "exam", Amount: 100000, Qty: 1},
		{ProductType: "book", Amount: 100000, Qty: 1},
		{ProductType: "medal", Amount: 100000, Qty: 1},
	})
	_ = orderID

	pts, err := r.DashboardSeries(context.Background(), from, to, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}

	if pts[0].Revenue != 300000 {
		t.Errorf("revenue = %v, want 300000 (fan-out regression)", pts[0].Revenue)
	}
	if pts[0].OrderCount != 1 {
		t.Errorf("order_count = %d, want 1", pts[0].OrderCount)
	}
	if pts[0].RevenueDigital != 100000 {
		t.Errorf("digital = %v, want 100000", pts[0].RevenueDigital)
	}
	if pts[0].RevenuePhysical != 200000 {
		t.Errorf("physical = %v, want 200000", pts[0].RevenuePhysical)
	}
}

func TestDashboardSeriesCountsDistinctStudents(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	// A day not touched by any other test in this file — the grading pool is
	// shared and never reset between tests.
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	day := time.Date(2026, 7, 15, 10, 0, 0, 0, jkt)
	from := time.Date(2026, 7, 15, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 7, 16, 0, 0, 0, 0, jkt)

	studentID := seedDashboardStudent(t, r, day)
	seedExamSession(t, r, studentID, day)
	seedExamSession(t, r, studentID, day.Add(time.Hour)) // same student, second attempt

	pts, err := r.DashboardSeries(context.Background(), from, to, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}

	if pts[0].ExamStudents != 1 {
		t.Errorf("exam_students = %d, want 1 (distinct students, not sessions)", pts[0].ExamStudents)
	}
	if pts[0].NewStudents != 1 {
		t.Errorf("new_students = %d, want 1", pts[0].NewStudents)
	}
}

func TestDashboardSeriesWeekBucket(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 7, 29, 0, 0, 0, 0, jkt) // 4 weeks

	pts, err := r.DashboardSeries(context.Background(), from, to, "week",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}
	if len(pts) < 4 || len(pts) > 5 {
		t.Errorf("got %d week points, want 4 or 5", len(pts))
	}
}
