package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrderSortCursors(t *testing.T) {
	id := uuid.New()
	at := time.Date(2026, 8, 4, 9, 30, 15, 123456789, time.UTC)
	for _, value := range []*time.Time{nil, &at} {
		got, gotID, err := DecodePaidOrderCursor(EncodePaidOrderCursor(value, id))
		require.NoError(t, err)
		require.Equal(t, id, gotID)
		if value == nil {
			require.Nil(t, got)
		} else {
			require.True(t, value.Equal(*got))
		}
	}
	for _, qty := range []int64{0, 1, 9999999999} {
		got, gotID, err := DecodeItemQtyOrderCursor(EncodeItemQtyOrderCursor(qty, id))
		require.NoError(t, err)
		require.Equal(t, qty, got)
		require.Equal(t, id, gotID)
	}
	for _, bad := range []string{"", "x,y", "null,nope", "1," + id.String(), "2026-08-04T09:30:15Z,nope"} {
		_, _, err := DecodePaidOrderCursor(bad)
		require.ErrorIs(t, err, ErrInvalidCursor)
	}
	for _, bad := range []string{"", "x,y", "null," + id.String(), "-1," + id.String(), EncodeOrderCursor(at, id), "9223372036854775808," + id.String()} {
		_, _, err := DecodeItemQtyOrderCursor(bad)
		require.ErrorIs(t, err, ErrInvalidCursor)
	}
}

func TestListOrders_AllSortDirectionsMatchUnpagedOrder(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	student := seedStudent(t, pool, "Sort Paging Buyer")
	book := seedProduct(t, pool, "Sort Paging Book", "book", 10000)
	medal := seedProduct(t, pool, "Sort Paging Medal", "medal", 10000)
	at := time.Date(2028, 2, 1, 8, 0, 0, 0, time.UTC)
	for i, qty := range []int{0, 2, 2, 7, 0, 1, 10, 10} {
		var items []seedItem
		if qty > 0 {
			items = []seedItem{{book, "Sort Paging Book", "book", 10000, qty}}
		}
		if i == 7 {
			items = []seedItem{{book, "Sort Paging Book", "book", 10000, 4}, {medal, "Sort Paging Medal", "medal", 10000, 6}}
		}
		status := "paid"
		if i >= 4 {
			status = "payment_pending"
		}
		id := seedOrder(t, pool, student, status, at.Add(time.Duration(i/2)*time.Hour), 0, 0, 0, 0, items)
		checkOut(t, pool, id, at.Add(time.Duration(3-i/2)*time.Hour))
		if i < 4 {
			_, err := pool.Exec(ctx, `UPDATE orders SET paid_at=$1 WHERE id=$2`, at.Add(time.Duration(i/2)*time.Hour), id)
			require.NoError(t, err)
		}
	}
	for _, sort := range []OrderSort{OrderSortOrderAt, OrderSortPaidAt, OrderSortItemQty} {
		for _, dir := range []OrderSortDirection{OrderSortAsc, OrderSortDesc} {
			t.Run(string(sort)+"/"+string(dir), func(t *testing.T) {
				key := map[OrderSort]string{
					OrderSortOrderAt: `COALESCE(checked_out_at,created_at)`,
					OrderSortPaidAt:  `paid_at`,
					OrderSortItemQty: `(SELECT COALESCE(SUM(qty),0) FROM order_item WHERE order_id=orders.id)`,
				}[sort]
				direction := "DESC"
				if dir == OrderSortAsc {
					direction = "ASC"
				}
				rows, err := pool.Query(ctx, `SELECT id FROM orders WHERE student_id=$1 AND status!='cart' ORDER BY `+key+` `+direction+` NULLS LAST, id `+direction, student)
				require.NoError(t, err)
				want := []uuid.UUID{}
				for rows.Next() {
					var id uuid.UUID
					require.NoError(t, rows.Scan(&id))
					want = append(want, id)
				}
				require.NoError(t, rows.Err())
				rows.Close()
				filter := OrderFilter{StudentID: &student, ExcludeCart: true, Sort: sort, Direction: dir, Limit: 100}
				unpaged, _, err := repo.ListOrders(ctx, filter)
				require.NoError(t, err)
				unpagedIDs := []uuid.UUID{}
				for _, row := range unpaged {
					unpagedIDs = append(unpagedIDs, row.ID)
				}
				require.Equal(t, want, unpagedIDs)
				filter.Limit = 2
				got := []uuid.UUID{}
				for page := 0; ; page++ {
					require.Less(t, page, 10, "paging must terminate")
					orders, next, err := repo.ListOrders(ctx, filter)
					require.NoError(t, err)
					for _, order := range orders {
						got = append(got, order.ID)
					}
					if sort == OrderSortPaidAt {
						for _, order := range orders {
							if page < 2 {
								require.NotNil(t, order.PaidAt)
							} else {
								require.Nil(t, order.PaidAt)
							}
						}
						if page == 1 {
							cursorAt, _, err := DecodePaidOrderCursor(next)
							require.NoError(t, err)
							require.NotNil(t, cursorAt, "boundary is last non-null row")
						}
					}
					if next == "" {
						break
					}
					filter.Cursor = next
				}
				require.Equal(t, want, got)
			})
		}
	}
	defaults, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &student, Limit: 100})
	require.NoError(t, err)
	explicit, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &student, Limit: 100, Sort: OrderSortOrderAt, Direction: OrderSortDesc})
	require.NoError(t, err)
	require.Equal(t, explicit, defaults)
}
