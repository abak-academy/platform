package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// checkOut backdates an order's checkout to at. seedOrder leaves checked_out_at
// NULL, which is the pre-migration-0009 shape — every test that wants the two
// timestamps to disagree has to set it explicitly.
func checkOut(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE orders SET checked_out_at = $1 WHERE id = $2`, at, id)
	require.NoError(t, err)
}

// created_at is stamped by MintCart, so a cart that sat idle outranks a cart
// minted later and checked out sooner. Ordering by it puts the list in an order
// no admin can explain.
func TestListOrders_sortsByCheckoutNotCartCreation(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Sort Buyer")
	book := seedProduct(t, pool, "Buku Sort", "book", 10000)
	item := []seedItem{{book, "Buku Sort", "book", 10000, 1}}

	// Minted first, checked out last — must sort first under DESC.
	staleCart := seedOrder(t, pool, student, "paid",
		time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC), 10000, 0, 0, 10000, item)
	checkOut(t, pool, staleCart, time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC))

	freshCart := seedOrder(t, pool, student, "paid",
		time.Date(2025, 1, 5, 8, 0, 0, 0, time.UTC), 10000, 0, 0, 10000, item)
	checkOut(t, pool, freshCart, time.Date(2025, 1, 6, 8, 0, 0, 0, time.UTC))

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, staleCart, got[0].ID, "checked out 10 Jan, must outrank the 6 Jan checkout")
	require.Equal(t, freshCart, got[1].ID)
}

// An order minted on the 4th and checked out on the 12th belongs to the 12th:
// that is the date the buyer saw and the date the admin filters by.
func TestListOrders_dateFilterMatchesCheckoutNotCartCreation(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Filter Buyer")
	book := seedProduct(t, pool, "Buku Filter", "book", 10000)
	item := []seedItem{{book, "Buku Filter", "book", 10000, 1}}

	minted := time.Date(2025, 2, 4, 2, 39, 0, 0, time.UTC)
	placed := time.Date(2025, 2, 12, 7, 23, 0, 0, time.UTC)

	id := seedOrder(t, pool, student, "payment_pending", minted, 20000, 0, 0, 20000, item)
	checkOut(t, pool, id, placed)

	checkoutDay := time.Date(2025, 2, 12, 0, 0, 0, 0, time.UTC)
	got, _, err := repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50,
		CreatedFrom: &checkoutDay, CreatedTo: ptrTime(checkoutDay.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)
	require.Len(t, got, 1, "filtering to the checkout day must find the order")
	require.Equal(t, id, got[0].ID)

	mintDay := time.Date(2025, 2, 4, 0, 0, 0, 0, time.UTC)
	got, _, err = repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50,
		CreatedFrom: &mintDay, CreatedTo: ptrTime(mintDay.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)
	require.Empty(t, got, "the cart-mint day is not the order date")
}

// checked_out_at is nullable — migration 0009 added it with no backfill, so the
// oldest prod orders have only created_at and must still sort and filter.
func TestListOrders_fallsBackToCreatedAtWhenNeverCheckedOut(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Legacy Buyer")
	book := seedProduct(t, pool, "Buku Legacy", "book", 10000)
	item := []seedItem{{book, "Buku Legacy", "book", 10000, 1}}

	day := time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC)
	legacy := seedOrder(t, pool, student, "paid",
		day.Add(9*time.Hour), 10000, 0, 0, 10000, item)

	newer := seedOrder(t, pool, student, "paid",
		time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), 10000, 0, 0, 10000, item)
	checkOut(t, pool, newer, day.Add(11*time.Hour))

	got, _, err := repo.ListOrders(ctx, OrderFilter{
		StudentID: &student, ExcludeCart: true, Limit: 50,
		CreatedFrom: &day, CreatedTo: ptrTime(day.AddDate(0, 0, 1)),
	})
	require.NoError(t, err)
	require.Len(t, got, 2, "a never-checked-out order still filters by created_at")
	require.Equal(t, newer, got[0].ID, "11:00 checkout outranks the 09:00 fallback")
	require.Equal(t, legacy, got[1].ID)
}

// The keyset predicate, the ORDER BY and the emitted cursor all have to agree on
// the sort key. If the cursor still carried created_at, paging would skip and
// repeat rows the moment the two timestamps disagree.
func TestListOrders_pagingKeyedOnCheckoutVisitsEveryOrderOnce(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Keyset Buyer")
	book := seedProduct(t, pool, "Buku Keyset", "book", 10000)
	item := []seedItem{{book, "Buku Keyset", "book", 10000, 1}}

	minted := time.Date(2025, 4, 1, 8, 0, 0, 0, time.UTC)
	want := map[uuid.UUID]bool{}
	for i := 0; i < 7; i++ {
		// Every order is minted at the same instant, so created_at carries no
		// ordering at all — only checkout does. Two share a checkout instant to
		// keep the id tiebreak honest, and one is never checked out.
		id := seedOrder(t, pool, student, "paid", minted, 10000, 0, 0, 10000, item)
		if i < 6 {
			checkOut(t, pool, id, minted.Add(time.Duration(i/2)*time.Hour))
		}
		want[id] = true
	}

	seen := map[uuid.UUID]int{}
	var order []time.Time
	cursor := ""
	for page := 0; page < 10; page++ {
		got, next, err := repo.ListOrders(ctx, OrderFilter{
			StudentID: &student, ExcludeCart: true, Limit: 3, Cursor: cursor,
		})
		require.NoError(t, err)
		for _, o := range got {
			seen[o.ID]++
			placed := o.CreatedAt
			if o.CheckedOutAt != nil {
				placed = *o.CheckedOutAt
			}
			order = append(order, placed)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	require.Len(t, seen, len(want), "every order visited")
	for id := range want {
		require.Equal(t, 1, seen[id], "order %s visited exactly once", id)
	}
	for i := 1; i < len(order); i++ {
		require.False(t, order[i].After(order[i-1]),
			"page order must be non-increasing on the checkout key, got %v then %v",
			order[i-1], order[i])
	}
}
