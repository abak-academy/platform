package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// GetRevenue (order.go), TopProducts (order_reporting.go), and
// DashboardSeries all filter orders by the same "paid" status set, now all
// built from the single paidStatusList constant (dashboard_series.go). Before
// that extraction, each site kept its own copy of the literal, and a status
// added to one and not the others would make /admin/revenue and /admin
// silently report different revenue for the same window with no test
// failing.
//
// This seeds one order per order status — the four that should count as
// paid, and two that should not — and cross-checks all three aggregations
// against each other for the exact same window. If a future edit reintroduces
// a fifth, independent copy of the status list anywhere and lets it drift,
// this fails; extraction alone wouldn't have caught someone bypassing the
// shared constant.
//
// Window: 2026-10, reserved in order_revenue_test.go's ledger.
func TestPaidStatusListsAgree(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Konsistensi Pendapatan")
	book := seedProduct(t, pool, "Buku Konsistensi", "book", 50000)

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

	// paid, processing, shipped, completed must count; payment_pending and
	// cancelled must not (orders_status_check, migration 0013).
	countedStatuses := []string{"paid", "processing", "shipped", "completed"}
	excludedStatuses := []string{"payment_pending", "cancelled"}
	for _, status := range append(append([]string{}, countedStatuses...), excludedStatuses...) {
		seedOrder(t, pool, student, status, at, 50000, 0, 0, 50000, []seedItem{
			{book, "Buku Konsistensi", "book", 50000, 1},
		})
	}
	wantTotal := float64(len(countedStatuses)) * 50000

	revenue, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)
	revenueTotal := revenue["total"].(float64)
	require.Equal(t, wantTotal, revenueTotal, "sanity: only the four paid-like orders count")

	products, err := repo.TopProducts(ctx, from, to, "revenue", 50)
	require.NoError(t, err)
	var productsTotal float64
	for _, p := range products {
		productsTotal += p.ProductRevenue
	}
	require.Equal(t, wantTotal, productsTotal,
		"TopProducts must count exactly the same orders as GetRevenue")

	series, err := repo.DashboardSeries(ctx, from, to, "day", []string{"book", "merchandise", "medal"})
	require.NoError(t, err)
	var seriesRevenue, seriesItemRevenue float64
	for _, p := range series {
		seriesRevenue += p.Revenue
		seriesItemRevenue += p.RevenueDigital + p.RevenuePhysical
	}
	require.Equal(t, wantTotal, seriesRevenue,
		"DashboardSeries must count exactly the same orders as GetRevenue")
	require.Equal(t, wantTotal, seriesItemRevenue,
		"DashboardSeries' item-line split must count exactly the same orders as TopProducts")
}
