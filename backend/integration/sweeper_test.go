package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/worker"
)

func TestSweeper(t *testing.T) {
	t.Run("FR-INT-20 expired payment_pending order swept to payment_expired, stock restored", func(t *testing.T) {
		env := newTestEnv(t)
		ctx := context.Background()

		studentID := seedUser(t, env, "student", "active", false)
		productID := seedProduct(t, env, "book", "Buku Sweeper", 50000)

		// Record stock before reservation.
		var stockBefore int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT stock FROM product WHERE id=$1`, productID,
		).Scan(&stockBefore))

		// Seed a payment_pending order with payment_expires_at in the past.
		var orderID string
		err := env.pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, total, payment_expires_at)
			 VALUES ($1, 'payment_pending', 50000, 50000, $2) RETURNING id`,
			studentID, time.Now().Add(-1*time.Hour),
		).Scan(&orderID)
		require.NoError(t, err)

		// Insert an order_item so the sweeper has stock to restore.
		_, err = env.pool.Exec(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
			 VALUES ($1, $2, 'book', 'Buku Sweeper', 50000, 1, 50000, 0)`,
			orderID, productID,
		)
		require.NoError(t, err)

		// Decrement stock to simulate the reservation that happened at checkout.
		_, err = env.pool.Exec(ctx,
			`UPDATE product SET stock = stock - 1, updated_at = now() WHERE id = $1`, productID,
		)
		require.NoError(t, err)

		var stockReserved int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT stock FROM product WHERE id=$1`, productID,
		).Scan(&stockReserved))
		assert.Equal(t, stockBefore-1, stockReserved, "pre-condition: stock must be decremented")

		// Run the worker with a very short sweeper interval.
		repo := repository.New(env.pool)
		w := worker.New(env.pool, env.rdb, repo, 500*time.Millisecond, 100*time.Millisecond, 5*time.Minute, nil, nil, nil, nil, time.Hour, "", nil)
		wCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go w.Run(wCtx)

		// Poll until the order reaches payment_expired.
		ok := pollUntil(t, 5*time.Second, func() bool {
			var status string
			_ = env.pool.QueryRow(ctx,
				`SELECT status FROM orders WHERE id=$1`, orderID,
			).Scan(&status)
			return status == "payment_expired"
		})
		cancel()
		require.True(t, ok, "timed out waiting for order to reach payment_expired")

		// Assert order status.
		var finalStatus string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&finalStatus))
		assert.Equal(t, "payment_expired", finalStatus)

		// Assert stock is restored to pre-checkout level.
		var stockAfter int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT stock FROM product WHERE id=$1`, productID,
		).Scan(&stockAfter))
		assert.Equal(t, stockBefore, stockAfter, "stock must be restored after payment expiry")
	})
}
