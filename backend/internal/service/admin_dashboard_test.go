package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPhysicalTypeValuesMatchIsPhysicalType(t *testing.T) {
	// The rule has one home. If a fourth physical type is added to
	// isPhysicalType and not here, the dashboard silently reclassifies it.
	for _, tp := range []string{"book", "merchandise", "medal"} {
		if !isPhysicalType(tp) {
			t.Errorf("%q should be physical", tp)
		}
	}
	for _, tp := range []string{"exam", "course"} {
		if isPhysicalType(tp) {
			t.Errorf("%q should be digital", tp)
		}
	}
	got := PhysicalTypeValues()
	if len(got) != 3 {
		t.Errorf("PhysicalTypeValues() = %v, want the three physical types", got)
	}
	for _, tp := range got {
		if !isPhysicalType(tp) {
			t.Errorf("PhysicalTypeValues() returned %q, which isPhysicalType rejects", tp)
		}
	}
}

func TestPreviousWindowIsEqualLengthImmediatelyBefore(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 8, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 8, 7, 0, 0, 0, 0, jkt) // 30 days

	pFrom, pTo := previousWindow(from, to)

	if !pTo.Equal(from) {
		t.Errorf("prev window should end where the current one starts: %v vs %v", pTo, from)
	}
	if to.Sub(from) != pTo.Sub(pFrom) {
		t.Errorf("prev window length %v != current %v", pTo.Sub(pFrom), to.Sub(from))
	}
}

func TestKPIOmitsPrevWhenPreviousWindowIsEmpty(t *testing.T) {
	// A delta against zero is noise, not information.
	k := makeKPI(126, 0, false)
	if k.Prev != nil {
		t.Errorf("prev = %v, want nil when the previous window had no data", *k.Prev)
	}

	k2 := makeKPI(126, 117, true)
	if k2.Prev == nil || *k2.Prev != 117 {
		t.Errorf("prev = %v, want 117", k2.Prev)
	}
}

func TestResolveBucketDefaultsByRangeLength(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, jkt)

	if got := resolveBucket("", from, from.AddDate(0, 0, 30)); got != "day" {
		t.Errorf("30 days = %q, want day", got)
	}
	if got := resolveBucket("", from, from.AddDate(0, 0, 31)); got != "day" {
		t.Errorf("31 days = %q, want day", got)
	}
	if got := resolveBucket("", from, from.AddDate(0, 0, 90)); got != "week" {
		t.Errorf("90 days = %q, want week", got)
	}
	if got := resolveBucket("day", from, from.AddDate(0, 0, 90)); got != "day" {
		t.Errorf("explicit bucket must win, got %q", got)
	}
}

// TestAdminDashboardCountsOrderRevenueOncePerOrder exercises the full
// AdminDashboard pipeline (not just DashboardSeries, which dashboard_series_test.go
// already covers at the repository level) against a real, seeded order with two
// line items. This repo shipped the PR #80 fan-out bug once already — joining
// order_item and summing orders.total counts the order once per line item — so
// an aggregation this central needs more than an empty-database smoke test.
//
// 2030 is untouched by every other test in this package: the real-DB container
// (newRealDBService) is shared and rows are never reset between tests.
func TestAdminDashboardCountsOrderRevenueOncePerOrder(t *testing.T) {
	svc, repo := newRealDBService(t)
	pool := repo.Pool()
	ctx := context.Background()

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	day := time.Date(2030, 1, 15, 10, 0, 0, 0, jkt)
	from := time.Date(2030, 1, 15, 0, 0, 0, 0, jkt)
	to := time.Date(2030, 1, 16, 0, 0, 0, 0, jkt)

	var studentID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, jenjang, grade, otp_enabled, created_at)
		 VALUES ($1, $2, 'student', 'active', 'sma', 10, false, $3) RETURNING id`,
		"Dashboard Pipeline Student "+uuid.NewString()[:12], "dashpipe_"+uuid.NewString()[:12], day,
	).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}

	var physicalProductID, digitalProductID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO product (name, type, price, status) VALUES ($1, 'book', 60000, 'published') RETURNING id`,
		"Dashboard Pipeline Book "+uuid.NewString()[:12],
	).Scan(&physicalProductID); err != nil {
		t.Fatalf("seed physical product: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO product (name, type, price, status) VALUES ($1, 'course', 40000, 'published') RETURNING id`,
		"Dashboard Pipeline Course "+uuid.NewString()[:12],
	).Scan(&digitalProductID); err != nil {
		t.Fatalf("seed digital product: %v", err)
	}

	// The order is charged once for 100000 (60000 book + 40000 course); the
	// fan-out bug would report this as 200000 (counted once per line item).
	const orderTotal = 100000.0
	var orderID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total, created_at)
		 VALUES ($1, 'paid', $2, $2, $3) RETURNING id`,
		studentID, orderTotal, day,
	).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	// weight_grams is set explicitly (0 is fine for both — the digital item has
	// none and the physical one isn't shipped in this test): AdminListOrders'
	// unfiltered ORDER BY created_at DESC picks up every order in the shared
	// real-DB fixture, this test's future-dated order included, and
	// fetchItems scans weight_grams into a plain int — a NULL there crashes an
	// unrelated test elsewhere in the package.
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
		 VALUES ($1, $2, 'book', 'Dashboard Pipeline Book', 60000, 1, 60000, 0)`,
		orderID, physicalProductID,
	); err != nil {
		t.Fatalf("seed physical item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
		 VALUES ($1, $2, 'course', 'Dashboard Pipeline Course', 40000, 1, 40000, 0)`,
		orderID, digitalProductID,
	); err != nil {
		t.Fatalf("seed digital item: %v", err)
	}

	resp, err := svc.AdminDashboard(ctx, from, to, "day")
	if err != nil {
		t.Fatalf("AdminDashboard: %v", err)
	}

	if resp.KPI["revenue"].Value != orderTotal {
		t.Errorf("kpi.revenue.value = %v, want %v exactly once (PR #80 fan-out regression)", resp.KPI["revenue"].Value, orderTotal)
	}

	var seriesRevenue, seriesDigital, seriesPhysical float64
	for _, p := range resp.Series {
		seriesRevenue += p.Revenue
		seriesDigital += p.RevenueDigital
		seriesPhysical += p.RevenuePhysical
	}
	if seriesRevenue != resp.KPI["revenue"].Value {
		t.Errorf("sum(series.revenue) = %v, want %v (kpi.revenue.value)", seriesRevenue, resp.KPI["revenue"].Value)
	}
	if seriesPhysical != 60000 {
		t.Errorf("sum(series.revenue_physical) = %v, want 60000 (the book line)", seriesPhysical)
	}
	if seriesDigital != 40000 {
		t.Errorf("sum(series.revenue_digital) = %v, want 40000 (the course line)", seriesDigital)
	}

	byProduct := map[string]DashboardTopProduct{}
	for _, p := range resp.TopProducts {
		byProduct[p.ProductID] = p
	}
	bookTop, ok := byProduct[physicalProductID.String()]
	if !ok {
		t.Fatalf("top_products missing the book line item: %+v", resp.TopProducts)
	}
	if bookTop.ProductRevenue != 60000 || bookTop.QtySold != 1 {
		t.Errorf("book top product = revenue %v qty %v, want 60000/1", bookTop.ProductRevenue, bookTop.QtySold)
	}
	if !bookTop.IsPhysical {
		t.Errorf("book top product IsPhysical = false, want true")
	}
	courseTop, ok := byProduct[digitalProductID.String()]
	if !ok {
		t.Fatalf("top_products missing the course line item: %+v", resp.TopProducts)
	}
	if courseTop.ProductRevenue != 40000 || courseTop.QtySold != 1 {
		t.Errorf("course top product = revenue %v qty %v, want 40000/1", courseTop.ProductRevenue, courseTop.QtySold)
	}
	if courseTop.IsPhysical {
		t.Errorf("course top product IsPhysical = true, want false")
	}
}
