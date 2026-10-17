package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUnitSalesLedger(t *testing.T) {
	pool := newReportingTestPool(t)
	ctx := context.Background()
	repo := New(pool)
	prefix := "unit-sales-" + uuid.NewString()
	student := seedStudent(t, pool, prefix)
	book := seedProduct(t, pool, prefix+"-book", "book", 100)
	second := seedProduct(t, pool, prefix+"-second", "book", 100)
	course := seedProduct(t, pool, prefix+"-course", "course", 100)
	at := time.Date(2043, 2, 10, 10, 0, 0, 0, jakarta)
	refundAt := at.AddDate(0, 1, 0)
	id := seedOrder(t, pool, student, "paid", at, 100, 0, 0, 100, []seedItem{
		{book, prefix + "-book", "book", 100, 3}, {second, prefix + "-second", "book", 100, 2}, {course, prefix + "-course", "course", 100, 4},
	})
	_, err := pool.Exec(ctx, `UPDATE orders SET status='cancelled', cancellation_reason='refunded', cancelled_at=$2 WHERE id=$1`, id, refundAt)
	require.NoError(t, err)
	unpaid := seedOrder(t, pool, student, "cancelled", at, 100, 0, 0, 100, []seedItem{{book, prefix + "-book", "book", 100, 99}})
	_, err = pool.Exec(ctx, `UPDATE orders SET cancellation_reason='other', cancelled_at=$2 WHERE id=$1`, unpaid, refundAt)
	require.NoError(t, err)
	seedOrder(t, pool, student, "payment_pending", at, 100, 0, 0, 100, []seedItem{{book, prefix + "-book", "book", 100, 99}})
	feb := time.Date(2043, 2, 1, 0, 0, 0, 0, jakarta)
	mar := feb.AddDate(0, 1, 0)
	apr := mar.AddDate(0, 1, 0)
	for _, tc := range []struct {
		name         string
		from, to     *time.Time
		paid, refund int
	}{
		{"paid period", &feb, &mar, 5, 0}, {"refund period", &mar, &apr, 0, 5},
		{"all time", nil, nil, 5, 5}, {"open from", &mar, nil, 0, 5}, {"open to", nil, &mar, 5, 0},
		{"empty", &apr, nil, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := repo.UnitSales(ctx, tc.from, tc.to)
			require.NoError(t, err)
			paid, refunded, productOrders := 0, 0, 0
			for _, row := range rows {
				if row.ProductID != nil && (*row.ProductID == book.String() || *row.ProductID == second.String()) {
					paid += row.PaidUnits
					refunded += row.RefundUnits
					productOrders += row.PaidOrders
				}
			}
			require.Equal(t, tc.paid, paid)
			require.Equal(t, tc.refund, refunded)
			if tc.name == "paid period" || tc.name == "refund period" {
				for _, row := range rows {
					if row.ProductID == nil && row.Type == "book" {
						require.Equal(t, tc.paid, row.PaidUnits)
						require.Equal(t, tc.refund, row.RefundUnits)
						if tc.paid > 0 {
							require.Equal(t, 1, row.PaidOrders)
							require.Greater(t, productOrders, row.PaidOrders)
						}
					}
					if row.ProductID == nil && row.Type == "course" {
						require.Equal(t, 4, row.PaidUnits+row.RefundUnits)
					}
				}
			}
		})
	}
}

func TestUnitSalesWIBEdges(t *testing.T) {
	pool := newReportingTestPool(t)
	prefix := "unit-edge-" + uuid.NewString()
	student := seedStudent(t, pool, prefix)
	book := seedProduct(t, pool, prefix, "book", 100)
	from := time.Date(2044, 1, 1, 0, 0, 0, 0, jakarta)
	to := from.AddDate(0, 0, 1)
	for _, at := range []time.Time{to.Add(-time.Minute), to} {
		seedOrder(t, pool, student, "paid", at, 100, 0, 0, 100, []seedItem{{book, prefix, "book", 100, 2}})
	}
	rows, err := New(pool).UnitSales(context.Background(), &from, &to)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, 2, row.PaidUnits)
		require.Equal(t, 1, row.PaidOrders)
	}
}
