package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// markPaid stamps paid_at, which seedOrder leaves NULL. Nothing is recognised
// as revenue without it, so every revenue fixture has to call this.
func markPaid(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE orders SET paid_at = $1 WHERE id = $2`, at, id)
	require.NoError(t, err)
}

// refund reproduces what AdminRefundOrder writes: status cancelled, reason
// 'refunded', cancelled_at stamped. paid_at is deliberately left intact — the
// sale really did happen, and the ledger needs both events.
func refund(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE orders SET status = 'cancelled', cancellation_reason = 'refunded',
		        cancelled_at = $1 WHERE id = $2`, at, id)
	require.NoError(t, err)
}

// GetRevenue (order.go), TopProducts (order_reporting.go) and DashboardSeries
// all read the single revenueEventCTE ledger (dashboard_series.go) rather than
// each keeping its own copy of a status literal. Before the shared constant, a
// status added to one and not the others made /admin/revenue and /admin report
// different revenue for the same window with nothing failing.
//
// This seeds paid and unpaid orders and cross-checks all three aggregations
// against each other for the exact same window. If a future edit reintroduces
// an independent copy of the recognition rule anywhere and lets it drift, this
// fails.
//
// Window: 2026-10, reserved in order_revenue_test.go's ledger.
func TestRevenueSourcesAgree(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Konsistensi Pendapatan")
	book := seedProduct(t, pool, "Buku Konsistensi", "book", 50000)

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

	// Four orders carry money; the two unpaid ones must contribute nothing,
	// whatever their status says.
	paidStatuses := []string{"paid", "processing", "shipped", "completed"}
	for _, status := range paidStatuses {
		id := seedOrder(t, pool, student, status, at, 50000, 0, 0, 50000, []seedItem{
			{book, "Buku Konsistensi", "book", 50000, 1},
		})
		markPaid(t, pool, id, at)
	}
	for _, status := range []string{"payment_pending", "cancelled"} {
		seedOrder(t, pool, student, status, at, 50000, 0, 0, 50000, []seedItem{
			{book, "Buku Konsistensi", "book", 50000, 1},
		})
	}
	wantTotal := float64(len(paidStatuses)) * 50000

	revenue, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)
	require.Equal(t, wantTotal, revenue["total"].(float64),
		"sanity: only the four orders with paid_at count")

	products, err := repo.TopProducts(ctx, from, to, "revenue", 50)
	require.NoError(t, err)
	var productsTotal float64
	for _, p := range products {
		productsTotal += p.ProductRevenue
	}
	require.Equal(t, wantTotal, productsTotal,
		"TopProducts must recognise exactly the same money as GetRevenue")

	series, err := repo.DashboardSeries(ctx, from, to, "day", []string{"book", "merchandise", "medal"})
	require.NoError(t, err)
	var seriesRevenue, seriesItemRevenue float64
	for _, p := range series {
		seriesRevenue += p.Revenue
		seriesItemRevenue += p.RevenueDigital + p.RevenuePhysical
	}
	require.Equal(t, wantTotal, seriesRevenue,
		"DashboardSeries must recognise exactly the same money as GetRevenue")
	require.Equal(t, wantTotal, seriesItemRevenue,
		"DashboardSeries' item-line split must agree with TopProducts")
}

// An unpaid order carries no paid_at, so nothing about its status can pull it
// into revenue. This is the guarantee that replaces the old status allowlist.
//
// Window: 2025-06, reserved in order_revenue_test.go's ledger.
func TestGetRevenue_ignoresOrdersThatNeverPaid(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Belum Bayar")
	book := seedProduct(t, pool, "Buku Belum Bayar", "book", 90000)

	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2025, 6, 10, 8, 0, 0, 0, time.UTC)

	// 'paid' status but no paid_at — the shape a pre-migration-0009 row would
	// have. Recognising it would invent money nobody can point at. seedOrder
	// stamps paid_at for this status, so it has to be cleared deliberately.
	id := seedOrder(t, pool, student, "paid", at, 90000, 0, 0, 90000,
		[]seedItem{{book, "Buku Belum Bayar", "book", 90000, 1}})
	_, err := pool.Exec(ctx, `UPDATE orders SET paid_at = NULL WHERE id = $1`, id)
	require.NoError(t, err)

	got, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)
	require.Equal(t, 0.0, got["total"].(float64))
	require.Equal(t, 0, got["order_count"].(int))
}

// Revenue is recognised when the money arrived, not when the cart was minted.
//
// Window: 2024-02/03, reserved in order_revenue_test.go's ledger. 2025-08 would
// have read naturally here but is already reserved, and the pool is shared —
// asserting an exact total means owning the window outright.
func TestGetRevenue_recognisesOnPaidAtNotCartCreation(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Lintas Bulan")
	book := seedProduct(t, pool, "Buku Lintas Bulan", "book", 40000)

	minted := time.Date(2024, 2, 28, 20, 43, 0, 0, time.UTC)
	paid := time.Date(2024, 3, 2, 15, 9, 0, 0, time.UTC)

	id := seedOrder(t, pool, student, "shipped", minted, 40000, 0, 0, 40000,
		[]seedItem{{book, "Buku Lintas Bulan", "book", 40000, 1}})
	markPaid(t, pool, id, paid)

	mintMonth, err := repo.GetRevenue(ctx,
		time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, 0.0, mintMonth["total"].(float64), "the cart was minted here, the money was not")

	paidMonth, err := repo.GetRevenue(ctx,
		time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, 40000.0, paidMonth["total"].(float64))
}

// A refund reduces the period it was ISSUED. The period that already reported
// the sale keeps reporting it — restating a closed month is the thing this
// design exists to prevent — and the two events net to zero all-time.
//
// Window: 2025-09/10, reserved in order_revenue_test.go's ledger.
func TestGetRevenue_refundDebitsTheRefundPeriodNotTheSalePeriod(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Refund Lintas Bulan")
	book := seedProduct(t, pool, "Buku Refund", "book", 60000)

	sept := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	oct := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	nov := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)

	id := seedOrder(t, pool, student, "completed", sept.Add(24*time.Hour),
		60000, 0, 0, 60000, []seedItem{{book, "Buku Refund", "book", 60000, 1}})
	markPaid(t, pool, id, sept.Add(48*time.Hour))
	refund(t, pool, id, oct.Add(48*time.Hour))

	sold, err := repo.GetRevenue(ctx, sept, oct)
	require.NoError(t, err)
	require.Equal(t, 60000.0, sold["total"].(float64), "September really did earn this")
	require.Equal(t, 1, sold["order_count"].(int))

	refunded, err := repo.GetRevenue(ctx, oct, nov)
	require.NoError(t, err)
	require.Equal(t, -60000.0, refunded["total"].(float64), "October gave it back")
	// Counts are a tally of sales, not a balance. A signed count would make
	// this -1, and the revenue page's total/order_count would then divide
	// -60000 by -1 and render a POSITIVE 60000 average order value for a month
	// that only refunded. Zero sales over negative revenue keeps the sign.
	require.Equal(t, 0, refunded["order_count"].(int), "no sale happened in October")

	allTime, err := repo.GetRevenue(ctx, sept, nov)
	require.NoError(t, err)
	require.Equal(t, 0.0, allTime["total"].(float64), "the ledger nets to zero")
	require.Equal(t, 1, allTime["order_count"].(int), "one sale happened, and it was refunded")
}

// The arithmetic the revenue page performs on these two numbers has to survive
// a refund. average order value = total / order_count must never come back
// positive for a period that only gave money back.
func TestGetRevenue_refundPeriodCannotFakeAPositiveAverage(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Rata-rata Refund")
	book := seedProduct(t, pool, "Buku Rata-rata", "book", 75000)

	dec := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	id := seedOrder(t, pool, student, "completed", dec.Add(24*time.Hour),
		75000, 0, 0, 75000, []seedItem{{book, "Buku Rata-rata", "book", 75000, 1}})
	markPaid(t, pool, id, dec.Add(24*time.Hour))
	refund(t, pool, id, dec.Add(240*time.Hour))

	// Sale and refund both land in December, so the month nets to zero on
	// every axis and the average is 0/1, not 0/0 or a sign-flipped number.
	got, err := repo.GetRevenue(ctx, dec, jan)
	require.NoError(t, err)
	total := got["total"].(float64)
	count := got["order_count"].(int)
	require.Equal(t, 0.0, total)
	require.Equal(t, 1, count)
	require.GreaterOrEqual(t, count, 0, "an order count must never go negative")
	require.Equal(t, 0.0, total/float64(count))
}

// The item-line reports have to debit the refund too, or the by-type breakdown
// and the product table drift away from the headline total they sit next to.
//
// Window: 2025-11, reserved in order_revenue_test.go's ledger.
func TestRefundDebitsItemLineReportsToo(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Refund Item Lines")
	book := seedProduct(t, pool, "Buku Refund Lines", "book", 25000)

	from := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2025, 11, 5, 8, 0, 0, 0, time.UTC)

	item := []seedItem{{book, "Buku Refund Lines", "book", 25000, 2}}

	kept := seedOrder(t, pool, student, "completed", at, 50000, 0, 0, 50000, item)
	markPaid(t, pool, kept, at)

	// Sold and refunded inside the same window: both events land here, so the
	// window must read as if it never happened.
	returned := seedOrder(t, pool, student, "completed", at, 50000, 0, 0, 50000, item)
	markPaid(t, pool, returned, at)
	refund(t, pool, returned, at.Add(72*time.Hour))

	revenue, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)
	require.Equal(t, 50000.0, revenue["total"].(float64))
	byType := revenue["by_type"].(map[string]interface{})
	bookBucket := byType["book"].(map[string]interface{})
	require.Equal(t, 50000.0, bookBucket["total"], "by_type debits the returned lines")
	// Two sales landed in this window; one of them was also refunded in it.
	// The money nets, the tally does not — see revenueEventCTE.
	require.Equal(t, 2, bookBucket["count"], "two sales happened, whatever became of them")

	products, err := repo.TopProducts(ctx, from, to, "revenue", 50)
	require.NoError(t, err)
	require.Len(t, products, 1)
	require.Equal(t, 50000.0, products[0].ProductRevenue)
	require.Equal(t, 2, products[0].QtySold, "4 sold minus 2 returned")
	require.Equal(t, 2, products[0].OrderCount)

	series, err := repo.DashboardSeries(ctx, from, to, "day", []string{"book", "merchandise", "medal"})
	require.NoError(t, err)
	var seriesRevenue, seriesSplit float64
	var seriesOrders int
	for _, p := range series {
		seriesRevenue += p.Revenue
		seriesSplit += p.RevenueDigital + p.RevenuePhysical
		seriesOrders += p.OrderCount
	}
	require.Equal(t, 50000.0, seriesRevenue)
	require.Equal(t, 50000.0, seriesSplit)
	require.Equal(t, 2, seriesOrders, "the series tallies sales the same way GetRevenue does")
}
