package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// seedExamProductRow inserts an exam product; a nil stock is stored as NULL
// (unlimited), a number as a limited quota.
func seedExamProductRow(t *testing.T, pool *pgxpool.Pool, stock *int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ('exam', $1, 100000, $2, 'published') RETURNING id`,
		"Exam "+uuid.NewString(), stock,
	).Scan(&id))
	return id
}

func seedExamOrderRow(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, qty int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	studentID := insertGradingUser(t, pool, "student", "Exam Buyer")
	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'cart', 0, 0, 0, 0) RETURNING id`, studentID,
	).Scan(&orderID))
	_, err := pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, 'exam', 'Exam', 100000, $4, $5, 0, now())`,
		uuid.New(), orderID, productID, qty, float64(100000*qty),
	)
	require.NoError(t, err)
	return orderID
}

// seedExamPendingOrderRow creates a payment_pending order whose deadline has
// elapsed, ready for ExpirePaymentOrderTx to act on.
func seedExamPendingOrderRow(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, qty int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	studentID := insertGradingUser(t, pool, "student", "Exam Expired Buyer")
	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, payment_expires_at)
		 VALUES ($1, 'payment_pending', 0, 0, 0, 0, now() - interval '1 minute') RETURNING id`, studentID,
	).Scan(&orderID))
	_, err := pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, 'exam', 'Exam', 100000, $4, $5, 0, now())`,
		uuid.New(), orderID, productID, qty, float64(100000*qty),
	)
	require.NoError(t, err)
	return orderID
}

func seedBulkOrderParticipant(t *testing.T, pool *pgxpool.Pool, orderID uuid.UUID) {
	t.Helper()
	studentID := insertGradingUser(t, pool, "student", "Bulk Participant")
	_, err := pool.Exec(context.Background(),
		`INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, orderID, studentID)
	require.NoError(t, err)
}

func examStock(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID) *int {
	t.Helper()
	var stock *int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT stock FROM product WHERE id = $1`, productID).Scan(&stock))
	return stock
}

func TestCheckoutOrder_ExamStock(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	tests := []struct {
		name      string
		stock     *int
		qty       int
		bulk      bool
		wantErr   error
		wantStock *int
	}{
		{"limited stock decrements by qty", intPtr(5), 2, false, nil, intPtr(3)},
		{"limited stock insufficient rejected", intPtr(1), 3, false, ErrInsufficientStock, intPtr(1)},
		{"unlimited stays NULL", nil, 1, false, nil, nil},
		{"bulk order skips stock", intPtr(5), 2, true, nil, intPtr(5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prodID := seedExamProductRow(t, pool, tt.stock)
			orderID := seedExamOrderRow(t, pool, prodID, tt.qty)
			if tt.bulk {
				seedBulkOrderParticipant(t, pool, orderID)
			}

			tx, err := repo.BeginTx(ctx)
			require.NoError(t, err)
			err = repo.CheckoutOrder(ctx, tx, orderID, time.Hour)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				_ = tx.Rollback(ctx)
			} else {
				require.NoError(t, err)
				require.NoError(t, tx.Commit(ctx))
			}
			require.Equal(t, tt.wantStock, examStock(t, pool, prodID))
		})
	}
}

func TestCheckoutOrder_ExamLastSeat_ExactlyOneSucceeds(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	prodID := seedExamProductRow(t, pool, intPtr(1))
	orderIDs := []uuid.UUID{
		seedExamOrderRow(t, pool, prodID, 1),
		seedExamOrderRow(t, pool, prodID, 1),
	}

	results := make(chan error, len(orderIDs))
	var wg sync.WaitGroup
	for _, oid := range orderIDs {
		wg.Add(1)
		go func(oid uuid.UUID) {
			defer wg.Done()
			tx, err := repo.BeginTx(ctx)
			if err != nil {
				results <- err
				return
			}
			defer tx.Rollback(ctx)
			if err := repo.CheckoutOrder(ctx, tx, oid, time.Hour); err != nil {
				results <- err
				return
			}
			results <- tx.Commit(ctx)
		}(oid)
	}
	wg.Wait()
	close(results)

	var ok, insufficient int
	for err := range results {
		switch {
		case err == nil:
			ok++
		case err == ErrInsufficientStock:
			insufficient++
		default:
			t.Fatalf("unexpected checkout error: %v", err)
		}
	}
	if ok != 1 || insufficient != 1 {
		t.Fatalf("want exactly one success and one ErrInsufficientStock, got ok=%d insufficient=%d", ok, insufficient)
	}
	require.Equal(t, intPtr(0), examStock(t, pool, prodID), "the last seat must be fully consumed")
}

func TestExpirePaymentOrderTx_ExamStock(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	tests := []struct {
		name      string
		stock     *int
		qty       int
		bulk      bool
		wantStock *int
	}{
		{"limited restores its seat once", intPtr(0), 1, false, intPtr(1)},
		{"unlimited stays NULL", nil, 1, false, nil},
		{"bulk order restores nothing", intPtr(5), 2, true, intPtr(5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prodID := seedExamProductRow(t, pool, tt.stock)
			orderID := seedExamPendingOrderRow(t, pool, prodID, tt.qty)
			if tt.bulk {
				seedBulkOrderParticipant(t, pool, orderID)
			}

			tx, err := repo.BeginTx(ctx)
			require.NoError(t, err)
			expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
			require.NoError(t, err)
			require.True(t, expired)
			require.NoError(t, tx.Commit(ctx))
			require.Equal(t, tt.wantStock, examStock(t, pool, prodID))

			tx2, err := repo.BeginTx(ctx)
			require.NoError(t, err)
			expired, err = repo.ExpirePaymentOrderTx(ctx, tx2, orderID)
			require.NoError(t, err)
			require.False(t, expired, "a second expiry must be a no-op")
			_ = tx2.Rollback(ctx)
			require.Equal(t, tt.wantStock, examStock(t, pool, prodID))
		})
	}
}

func TestExamUnlimitedDoesNotLockProductRow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	tests := []struct {
		name string
		seed func(*testing.T, uuid.UUID) uuid.UUID
		call func(*testing.T, context.Context, pgx.Tx, uuid.UUID)
	}{
		{
			"checkout does not lock the product row",
			func(t *testing.T, prodID uuid.UUID) uuid.UUID { return seedExamOrderRow(t, pool, prodID, 1) },
			func(t *testing.T, ctx context.Context, tx pgx.Tx, orderID uuid.UUID) {
				require.NoError(t, repo.CheckoutOrder(ctx, tx, orderID, time.Hour), "an unlimited exam checkout must not wait on the product row lock")
			},
		},
		{
			"expiry does not lock the product row",
			func(t *testing.T, prodID uuid.UUID) uuid.UUID { return seedExamPendingOrderRow(t, pool, prodID, 1) },
			func(t *testing.T, ctx context.Context, tx pgx.Tx, orderID uuid.UUID) {
				expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
				require.NoError(t, err, "an unlimited exam expiry must not wait on the product row lock")
				require.True(t, expired)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prodID := seedExamProductRow(t, pool, nil)
			orderID := tt.seed(t, prodID)

			holder, err := repo.BeginTx(ctx)
			require.NoError(t, err)
			defer holder.Rollback(ctx)
			_, err = holder.Exec(ctx, `SELECT 1 FROM product WHERE id = $1 FOR UPDATE`, prodID)
			require.NoError(t, err)

			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			tx, err := repo.BeginTx(cctx)
			require.NoError(t, err)

			tt.call(t, cctx, tx, orderID)
			require.NoError(t, tx.Commit(cctx))
		})
	}
}
