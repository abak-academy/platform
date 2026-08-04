package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrderCursor_roundTrip(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 30, 15, 123456789, time.UTC)
	id := uuid.New()

	gotAt, gotID, err := DecodeOrderCursor(EncodeOrderCursor(at, id))
	require.NoError(t, err)
	require.True(t, at.Equal(gotAt), "want %v got %v", at, gotAt)
	require.Equal(t, id, gotID)
}

func TestOrderCursor_rejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "nope", uuid.NewString(), "2026-08-04T09:30:15Z", "x_y"} {
		_, _, err := DecodeOrderCursor(bad)
		require.ErrorIs(t, err, ErrInvalidCursor, "input %q", bad)
	}
}

// The previous keyset filtered `id > cursor` while ordering by created_at DESC.
// Orders sharing a created_at also break a date-only keyset, so they are what is
// seeded here: correctness needs the (created_at, id) composite. Paging must
// visit every order exactly once.
func TestListOrders_pagingVisitsEveryOrderExactlyOnce(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Paging Buyer")
	book := seedProduct(t, pool, "Buku Paging", "book", 10000)

	shared := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	want := map[uuid.UUID]bool{}
	for i := 0; i < 7; i++ {
		at := shared
		if i >= 3 {
			at = shared.Add(-time.Duration(i) * time.Hour)
		}
		id := seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
			[]seedItem{{book, "Buku Paging", "book", 10000, 1}})
		want[id] = true
	}

	seen := map[uuid.UUID]int{}
	cursor := ""
	for page := 0; page < 10; page++ {
		orders, next, err := repo.ListOrders(ctx, OrderFilter{
			StudentID:   &student,
			ExcludeCart: true,
			Limit:       3,
			Cursor:      cursor,
		})
		require.NoError(t, err)
		for _, o := range orders {
			seen[o.ID]++
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
}
