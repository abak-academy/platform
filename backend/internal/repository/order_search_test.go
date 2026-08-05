package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListOrders_searchMatchesOrderNumberSuffix(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Suffix Buyer")
	book := seedProduct(t, pool, "Buku Suffix", "book", 10000)
	at := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)

	target := seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Suffix", "book", 10000, 1}})
	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Suffix", "book", 10000, 1}})

	// The UI shows the last 8 characters of the uuid, so that is what an admin
	// reads off a screen and types back in.
	full := target.String()
	short := full[len(full)-8:]

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50, Search: short,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, target, got[0].ID)
}

func TestListOrders_searchMatchesBuyerNamePartialCaseInsensitive(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	// The pool is shared across this package's tests, and the search below is
	// deliberately unscoped, so the name has to be one no other test seeds.
	zulaikha := seedStudent(t, pool, "Zulaikha Renggani")
	bagus := seedStudent(t, pool, "Bagus Wicaksono")
	book := seedProduct(t, pool, "Buku Nama", "book", 10000)
	at := time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)

	want := seedOrder(t, pool, zulaikha, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Nama", "book", 10000, 1}})
	seedOrder(t, pool, bagus, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Nama", "book", 10000, 1}})

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		ExcludeCart: true, Limit: 50, Search: "renggani",
	})
	require.NoError(t, err)

	ids := map[uuid.UUID]bool{}
	for _, o := range got {
		ids[o.ID] = true
	}
	require.True(t, ids[want], "partial, case-insensitive name match")
	require.Len(t, ids, 1)
}

func TestListOrders_dateRangeIsHalfOpen(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Range Buyer")
	book := seedProduct(t, pool, "Buku Range", "book", 10000)

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	inRange := seedOrder(t, pool, student, "paid", from, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Range", "book", 10000, 1}})
	seedOrder(t, pool, student, "paid", to, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Range", "book", 10000, 1}})

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50,
		CreatedFrom: &from, CreatedTo: &to,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, inRange, got[0].ID)
}

// A student whose name contains digits would otherwise be dragged in by a
// numeric order-number search.
func TestListOrders_digitsOnlySearchSkipsNameBranch(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	numeric := seedStudent(t, pool, "Siswa 12345678")
	book := seedProduct(t, pool, "Buku Digit", "book", 10000)
	at := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, numeric, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Digit", "book", 10000, 1}})

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		ExcludeCart: true, Limit: 50, Search: "12345678",
	})
	require.NoError(t, err)
	require.Empty(t, got, "digits match order numbers only, never names")
}
