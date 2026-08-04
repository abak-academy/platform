package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// One order, five copies. qty_sold must be 5 while order_count is 1 — if the
// two ever collapse into the same number the aggregation is wrong.
func TestTopProducts_qtyAndOrderCountAreDifferentNumbers(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Bulk Buyer")
	book := seedProduct(t, pool, "Buku Borongan", "book", 20000)
	at := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", at, 100000, 0, 0, 100000,
		[]seedItem{{book, "Buku Borongan", "book", 20000, 5}})

	// February is used by no other test in this package; the pool is shared.
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.TopProducts(ctx, from, to, "revenue", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, book, got[0].ProductID)
	require.Equal(t, "Buku Borongan", got[0].Name)
	require.Equal(t, "book", got[0].ProductType)
	require.Equal(t, 5, got[0].QtySold)
	require.Equal(t, 1, got[0].OrderCount)
	require.Equal(t, 100000.0, got[0].ProductRevenue)
}

// orderBy is interpolated into the SQL, so it has to be an allowlist.
func TestTopProducts_orderByRejectsUnknownKey(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err := repo.TopProducts(context.Background(), from, to, "total; DROP TABLE orders", 10)
	require.ErrorIs(t, err, ErrInvalidOrderBy)
}

func TestTopProducts_ordersByQtyWhenAsked(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Ranking Buyer")
	cheap := seedProduct(t, pool, "Pin Murah", "merchandise", 1000)
	pricey := seedProduct(t, pool, "Medali Mahal", "medal", 500000)
	at := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

	// Many cheap units versus one expensive one: the two orderings disagree,
	// which is the whole point of having both.
	seedOrder(t, pool, student, "paid", at, 100000, 0, 0, 100000,
		[]seedItem{{cheap, "Pin Murah", "merchandise", 1000, 100}})
	seedOrder(t, pool, student, "paid", at, 500000, 0, 0, 500000,
		[]seedItem{{pricey, "Medali Mahal", "medal", 500000, 1}})

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	byRevenue, err := repo.TopProducts(ctx, from, to, "revenue", 10)
	require.NoError(t, err)
	require.Equal(t, pricey, byRevenue[0].ProductID)

	byQty, err := repo.TopProducts(ctx, from, to, "qty", 10)
	require.NoError(t, err)
	require.Equal(t, cheap, byQty[0].ProductID)
}

// The store dashboard used to count with .length of a page the API caps at 20.
func TestCountOrdersByBucket_countsBeyondOnePage(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Bucket Buyer")
	book := seedProduct(t, pool, "Buku Bucket", "book", 10000)
	at := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)

	for i := 0; i < 25; i++ {
		seedOrder(t, pool, student, "payment_pending", at, 10000, 0, 0, 10000,
			[]seedItem{{book, "Buku Bucket", "book", 10000, 1}})
	}
	seedOrder(t, pool, student, "completed", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Bucket", "book", 10000, 1}})

	monthStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)

	require.Equal(t, 25, got.NeedsConfirm, "counts are not capped by page size")
	require.Equal(t, 26, got.Total)
	require.Equal(t, 26, got.CreatedThisMonth)
	require.Equal(t, 1, got.CompletedThisMonth)
	require.Equal(t, 0, got.ShipmentFailed)
}

// The counts have to describe the same rows the table is showing, so they run
// through the same predicate builder as ListOrders.
func TestCountOrdersByBucket_honoursTheSearchFilter(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	wanted := seedStudent(t, pool, "Terpilih Sekali")
	other := seedStudent(t, pool, "Bukan Dia")
	book := seedProduct(t, pool, "Buku Filter", "book", 10000)
	at := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, wanted, "payment_pending", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Filter", "book", 10000, 1}})
	seedOrder(t, pool, other, "payment_pending", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Filter", "book", 10000, 1}})

	monthStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{ExcludeCart: true, Search: "terpilih sekali"},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)
	require.Equal(t, 1, got.NeedsConfirm, "search narrows the counts too")
}
