package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
)

// One Postgres container is shared by every test in the reporting suite
// (order_revenue_test, order_cursor_test, order_search_test,
// order_reporting_test), so seeded orders accumulate and `go test -shuffle=on`
// can run these in any order.
//
// GetRevenue and TopProducts aggregate over a date range with no per-student
// filter, so any test asserting an exact total must own its period outright.
// Reserved windows — take a free one rather than reusing:
//
//	2024-06/07  GetRevenue half-open boundary (seeds an order ON `to`)
//	2026-01     bucket counts          2026-02  top products (revenue order)
//	2026-03/04  date-range filter      2026-04  search
//	2026-05     cursor paging          2026-07  GetRevenue fan-out
//	2026-09     top products (qty order)        2026-11  bucket search filter
//	2025-05     shipped orders count as revenue
var (
	reportingPool     *pgxpool.Pool
	reportingPoolOnce sync.Once
	reportingPoolErr  error
)

// newReportingTestPool returns a Postgres container shared by every test in the
// reporting suite. One container per package run, not one per test — CI already
// starts hundreds of them.
func newReportingTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	reportingPoolOnce.Do(func() {
		ctx := context.Background()
		c, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			reportingPoolErr = err
			return
		}
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			reportingPoolErr = err
			return
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			reportingPoolErr = err
			return
		}
		reportingPool, reportingPoolErr = pgxpool.New(ctx, dsn)
	})
	require.NoError(t, reportingPoolErr)
	require.NotNil(t, reportingPool)
	return reportingPool
}

func seedStudent(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', $2) RETURNING id`,
		"rev-"+uuid.NewString()+"@test.local", name,
	).Scan(&id))
	return id
}

// Columns per migrations 0004 + 0009: `title` was renamed to `name`, and
// visibility is `status`, not an is_active flag. product.type is CHECK
// constrained to book | course | exam | merchandise | medal (0027).
func seedProduct(t *testing.T, pool *pgxpool.Pool, name, ptype string, price float64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO product (name, type, price, status)
		 VALUES ($1, $2, $3, 'published') RETURNING id`,
		name, ptype, price,
	).Scan(&id))
	return id
}

type seedItem struct {
	productID uuid.UUID
	name      string
	ptype     string
	unitPrice float64
	qty       int
}

// seedOrder inserts one order plus its items. total is set explicitly so a test
// can assert GetRevenue reports the charged total rather than a sum of lines.
func seedOrder(
	t *testing.T, pool *pgxpool.Pool, studentID uuid.UUID, status string,
	createdAt time.Time, subtotal, discount, shipping, total float64, items []seedItem,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var completedAt *time.Time
	if status == "completed" {
		completedAt = &createdAt
	}

	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, created_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		studentID, status, subtotal, discount, shipping, total, createdAt, completedAt,
	).Scan(&orderID))

	for _, it := range items {
		// weight_grams is nullable in the schema but OrderItem.WeightGrams is a
		// plain int, so fetchItems cannot scan a NULL back out.
		_, err := pool.Exec(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 0)`,
			orderID, it.productID, it.ptype, it.name, it.unitPrice, it.qty,
			it.unitPrice*float64(it.qty),
		)
		require.NoError(t, err)
	}
	return orderID
}

// A single Rp 500.000 order with three items across two product types.
// The previous query summed orders.total once per item row, reporting
// 1.500.000 and attributing the full order total to BOTH product types.
func TestGetRevenue_doesNotFanOutAcrossItems(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Rani Puspita")
	book := seedProduct(t, pool, "Buku TKA Saintek", "book", 150000)
	medal := seedProduct(t, pool, "Medali Emas", "medal", 100000)

	at := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	seedOrder(t, pool, student, "paid", at, 400000, 0, 100000, 500000, []seedItem{
		{book, "Buku TKA Saintek", "book", 150000, 2},
		{medal, "Medali Emas", "medal", 100000, 1},
	})

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)

	require.Equal(t, 500000.0, got["total"], "charged total counted once")
	require.Equal(t, 400000.0, got["product_revenue"], "sum of item lines")
	require.Equal(t, 100000.0, got["shipping_total"])
	require.Equal(t, 1, got["order_count"], "orders, not item rows")

	byType := got["by_type"].(map[string]interface{})
	bookBucket := byType["book"].(map[string]interface{})
	medalBucket := byType["medal"].(map[string]interface{})
	require.Equal(t, 300000.0, bookBucket["total"], "2 x 150.000")
	require.Equal(t, 100000.0, medalBucket["total"])
	require.Equal(t, 1, bookBucket["count"])
	require.Equal(t, 1, medalBucket["count"])
}

// BETWEEN is inclusive at both ends, so an order landing exactly on `to` was
// counted in this period and again in the next one.
func TestGetRevenue_rangeIsHalfOpen(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Boundary Buyer")
	book := seedProduct(t, pool, "Buku Batas", "book", 50000)

	// 2024, not 2026: this test seeds an order exactly ON `to`, which by
	// definition lands in the following period. Under -shuffle that order was
	// landing inside TestGetRevenue_doesNotFanOutAcrossItems' July 2026 window
	// and inflating its total. See the reservation table at the top of the file.
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", from, 50000, 0, 0, 50000,
		[]seedItem{{book, "Buku Batas", "book", 50000, 1}})
	seedOrder(t, pool, student, "paid", to, 50000, 0, 0, 50000,
		[]seedItem{{book, "Buku Batas", "book", 50000, 1}})

	got, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)

	require.Equal(t, 50000.0, got["total"], "`from` included, `to` excluded")
	require.Equal(t, 1, got["order_count"])
}

// A paid parcel in transit is money already taken. Leaving `shipped` out of the
// status set made an order vanish from revenue, order count, by-type totals and
// top products for the whole delivery window, then reappear on completion.
func TestGetRevenue_countsShippedOrders(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "In Transit Buyer")
	book := seedProduct(t, pool, "Buku Jalan", "book", 80000)
	at := time.Date(2025, 5, 10, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "shipped", at, 80000, 0, 20000, 100000,
		[]seedItem{{book, "Buku Jalan", "book", 80000, 1}})

	from := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.GetRevenue(ctx, from, to)
	require.NoError(t, err)
	require.Equal(t, 100000.0, got["total"], "an in-transit order is still revenue")
	require.Equal(t, 1, got["order_count"])

	byType := got["by_type"].(map[string]interface{})
	require.Contains(t, byType, "book")

	top, err := repo.TopProducts(ctx, from, to, "revenue", 10)
	require.NoError(t, err)
	require.Len(t, top, 1, "top products must use the same status set as revenue")
	require.Equal(t, 80000.0, top[0].ProductRevenue)
}
