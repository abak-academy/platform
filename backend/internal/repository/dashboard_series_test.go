package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// This file shares the newGradingTestPool container (exam_grading_test.go)
// with 14+ other test files; rows are never reset between tests. Reserved
// windows, in Asia/Jakarta — take a free one rather than reusing one below.
// Follows the precedent set by newReportingTestPool's ledger in
// order_revenue_test.go.
//
//	2026-07-01 .. 2026-07-06  FillsEmptyDays               (day bucket, order on 07-03)
//	2026-08-01 .. 2026-08-03  BucketsInJakarta              (day bucket, order on 08-02)
//	2026-07-10 .. 2026-07-11  CountsOrderTotalOncePerOrder
//	2026-07-15 .. 2026-07-16  CountsDistinctStudents
//	2026-07-01 .. 2026-07-29  WeekBucket                    (structural only — bucket count and
//	                                                          first-bucket date; tolerant of other
//	                                                          tests' data landing in this range)
//	2026-09-01 .. 2026-09-02  DateSerializesInJakartaOffset (asserts only the JSON "date" shape,
//	                                                          seeds and reads no order/student data)
//	2026-10-05 .. 2026-10-05  EmptyWindowMarshalsToEmptyArray (from == to, single point, no seed data)

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

	// 1 Jul 2026 is a Wednesday. Weeks anchor to `from`, not to the ISO
	// Monday — date_trunc('week', ...) would instead produce a short first
	// bucket (29 Jun–1 Jul, only 3 days) and mislabel it 29 Jun.
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 7, 29, 0, 0, 0, 0, jkt) // 4 weeks

	pts, err := r.DashboardSeries(context.Background(), from, to, "week",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}
	if len(pts) != 4 {
		t.Fatalf("got %d week points, want exactly 4 (each a full 7 days, anchored to `from`)", len(pts))
	}
	if !pts[0].Date.Equal(from) {
		t.Errorf("first bucket = %v, want %v (anchored to `from`, not the preceding Monday)", pts[0].Date, from)
	}
}

// SeriesPoint.Date is scanned from a timezone-naive Postgres timestamp
// holding Jakarta wall-clock values; a bare pgx Scan leaves it with a UTC
// Location, so the default JSON encoding would emit a "...Z" suffix — a
// false claim that midnight Jakarta is midnight UTC. Task 3 serializes
// []SeriesPoint straight to JSON and the frontend does date arithmetic on
// it, so an incorrect offset here reproduces the "wrong day" bug one layer
// up, at serialization instead of at bucketing.
func TestDashboardSeriesDateSerializesInJakartaOffset(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 9, 2, 0, 0, 0, 0, jkt)

	pts, err := r.DashboardSeries(context.Background(), from, to, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("got %d points, want 1", len(pts))
	}

	raw, err := json.Marshal(pts[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	const want = "2026-09-01T00:00:00+07:00"
	if decoded.Date != want {
		t.Errorf("serialized date = %q, want %q (Jakarta offset, not a false UTC Z)", decoded.Date, want)
	}
}

// from == to means to_local (to minus 1 microsecond) falls before anchor
// (from), so generate_series produces zero rows — the window a custom period
// picker with no from/to bounds can trivially produce. `var out []SeriesPoint`
// stays nil in that case, and encoding/json marshals a nil slice as `null`,
// not `[]`; the frontend's DashboardSeriesPoint[] type isn't nullable, so
// `d.series.map(...)` throws before any JSX renders and the whole /admin page
// goes blank.
//
// A length check on the Go value can't catch this: nil and an empty slice
// both report len() == 0. Only the marshaled bytes tell them apart.
func TestDashboardSeriesEmptyWindowMarshalsToEmptyArray(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, jkt)

	pts, err := r.DashboardSeries(context.Background(), at, at, "day",
		[]string{"book", "merchandise", "medal"})
	if err != nil {
		t.Fatalf("DashboardSeries: %v", err)
	}

	raw, err := json.Marshal(pts)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(raw) != "[]" {
		t.Errorf("marshaled series = %s, want [] (a nil slice marshals as null)", raw)
	}
}
