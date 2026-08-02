package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/worker"
)

// seedPaidOrder inserts an order row with status='paid' and returns its ID.
func seedPaidOrder(t *testing.T, env *testEnv, studentID, productID string, productType string) string {
	t.Helper()
	ctx := context.Background()
	var orderID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total)
		 VALUES ($1, 'paid', 1000, 1000) RETURNING id`,
		studentID,
	).Scan(&orderID)
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
		 VALUES ($1, $2, $3, 'Test Product', 1000, 1, 1000, 0)`,
		orderID, productID, productType,
	)
	require.NoError(t, err)
	return orderID
}

// seedOutboxOrderPaid inserts an unprocessed OrderPaid outbox row for the given order.
// Uses worker.OrderPaidPayload (snake_case JSON tags) to match the payload the service writes.
func seedOutboxOrderPaid(t *testing.T, env *testEnv, orderID string, productID string, productType string) {
	t.Helper()
	ctx := context.Background()

	oID, err := uuid.Parse(orderID)
	require.NoError(t, err)
	pID, err := uuid.Parse(productID)
	require.NoError(t, err)

	payload := worker.OrderPaidPayload{
		OrderID: oID,
		Items: []worker.OrderItemMini{
			{ProductID: pID, ProductType: productType},
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx,
		`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
		 VALUES ('order', $1, 'OrderPaid', $2)`,
		orderID, b,
	)
	require.NoError(t, err)
}

// pollUntil polls fn until it returns true or the deadline passes.
func pollUntil(t *testing.T, deadline time.Duration, fn func() bool) bool {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(deadline)
	for {
		select {
		case <-timeout:
			return false
		case <-ticker.C:
			if fn() {
				return true
			}
		}
	}
}

func TestWorkerFanout(t *testing.T) {
	t.Run("FR-INT-14 N linked courses produce N active sessions, order completed, outbox processed", func(t *testing.T) {
		env := newTestEnv(t)
		ctx := context.Background()

		studentID := seedUser(t, env, "student", "active", false)
		productID := seedProduct(t, env, "course", "Course Bundle", 99000)
		course1ID := seedCourse(t, env, "Math 101")
		course2ID := seedCourse(t, env, "Science 101")
		linkProductCourse(t, env, productID, course1ID)
		linkProductCourse(t, env, productID, course2ID)

		orderID := seedPaidOrder(t, env, studentID, productID, "course")
		seedOutboxOrderPaid(t, env, orderID, productID, "course")

		repo := repository.New(env.pool)
		w := worker.New(env.pool, env.rdb, repo, 50*time.Millisecond, 200*time.Millisecond, 5*time.Minute, nil, nil, nil, nil, time.Hour, "", nil)
		wCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go w.Run(wCtx)

		// Poll until exactly 2 active course_session rows exist.
		ok := pollUntil(t, 5*time.Second, func() bool {
			var count int
			_ = env.pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM course_session WHERE student_id=$1 AND order_id=$2 AND status='active'`,
				studentID, orderID,
			).Scan(&count)
			return count == 2
		})
		cancel()
		require.True(t, ok, "timed out waiting for 2 course_session rows")

		// Assert exact count.
		var sessionCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM course_session WHERE student_id=$1 AND order_id=$2 AND status='active' AND source='order'`,
			studentID, orderID,
		).Scan(&sessionCount))
		assert.Equal(t, 2, sessionCount)

		// Assert order is completed (digital-only order skips shipping).
		var orderStatus string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&orderStatus))
		assert.Equal(t, "completed", orderStatus)

		// Assert outbox row is processed.
		var processedCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid' AND processed_at IS NOT NULL`,
			orderID,
		).Scan(&processedCount))
		assert.Equal(t, 1, processedCount)
	})

	t.Run("FR-INT-15 replay produces no duplicate sessions (ON CONFLICT DO NOTHING)", func(t *testing.T) {
		env := newTestEnv(t)
		ctx := context.Background()

		studentID := seedUser(t, env, "student", "active", false)
		productID := seedProduct(t, env, "course", "Replay Course", 50000)
		courseID := seedCourse(t, env, "Biology 101")
		linkProductCourse(t, env, productID, courseID)

		orderID := seedPaidOrder(t, env, studentID, productID, "course")
		seedOutboxOrderPaid(t, env, orderID, productID, "course")

		repo := repository.New(env.pool)
		w := worker.New(env.pool, env.rdb, repo, 50*time.Millisecond, 200*time.Millisecond, 5*time.Minute, nil, nil, nil, nil, time.Hour, "", nil)
		wCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go w.Run(wCtx)

		// Wait for the first outbox event to be processed.
		ok := pollUntil(t, 5*time.Second, func() bool {
			var processedCount int
			_ = env.pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid' AND processed_at IS NOT NULL`,
				orderID,
			).Scan(&processedCount)
			return processedCount == 1
		})
		require.True(t, ok, "timed out waiting for first outbox event to be processed")

		// Insert a second unprocessed outbox row for the same order to simulate replay.
		seedOutboxOrderPaid(t, env, orderID, productID, "course")

		// Wait for the second event to be processed too.
		ok = pollUntil(t, 5*time.Second, func() bool {
			var processedCount int
			_ = env.pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid' AND processed_at IS NOT NULL`,
				orderID,
			).Scan(&processedCount)
			return processedCount == 2
		})
		cancel()
		require.True(t, ok, "timed out waiting for replay outbox event to be processed")

		// Session count must still be 1 (ON CONFLICT DO NOTHING).
		var sessionCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM course_session WHERE student_id=$1 AND course_id=$2 AND status='active'`,
			studentID, courseID,
		).Scan(&sessionCount))
		assert.Equal(t, 1, sessionCount, "ON CONFLICT DO NOTHING must prevent duplicate sessions")
	})

	t.Run("exam product: N linked exams produce N registrations, order completed, order_item stamped", func(t *testing.T) {
		env := newTestEnv(t)
		ctx := context.Background()

		studentID := seedUser(t, env, "student", "active", false)
		productID := seedProduct(t, env, "exam", "Exam Bundle", 75000)
		examID := seedExam(t, env, "TKA Try Out", "hidden", nil)
		linkProductExam(t, env, productID, examID)

		orderID := seedPaidOrder(t, env, studentID, productID, "exam")
		seedOutboxOrderPaid(t, env, orderID, productID, "exam")

		repo := repository.New(env.pool)
		w := worker.New(env.pool, env.rdb, repo, 50*time.Millisecond, 200*time.Millisecond, 5*time.Minute, nil, nil, nil, nil, time.Hour, "", nil)
		wCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go w.Run(wCtx)

		// Poll until the order reaches completed.
		ok := pollUntil(t, 5*time.Second, func() bool {
			var status string
			_ = env.pool.QueryRow(ctx,
				`SELECT status FROM orders WHERE id=$1`, orderID,
			).Scan(&status)
			return status == "completed"
		})
		cancel()
		require.True(t, ok, "timed out waiting for order to reach completed — StampOrderItemFulfilledAt likely erroring and rolling back the tx")

		// Registration must exist.
		var regCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM exam_registration WHERE student_id=$1 AND exam_id=$2`,
			studentID, examID,
		).Scan(&regCount))
		assert.Equal(t, 1, regCount, "exam registration must survive the transaction")

		// order_item.fulfilled_at must be stamped.
		var fulfilledAt *time.Time
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT fulfilled_at FROM order_item WHERE order_id=$1 AND product_id=$2`,
			orderID, productID,
		).Scan(&fulfilledAt))
		assert.NotNil(t, fulfilledAt, "order_item.fulfilled_at must be stamped")

		// Outbox must be processed.
		var processedCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid' AND processed_at IS NOT NULL`,
			orderID,
		).Scan(&processedCount))
		assert.Equal(t, 1, processedCount)
	})

	t.Run("FR-INT-16 zero-linked course product: no session, order completed, outbox processed", func(t *testing.T) {
		env := newTestEnv(t)
		ctx := context.Background()

		studentID := seedUser(t, env, "student", "active", false)
		// Course product with no product_course links.
		productID := seedProduct(t, env, "course", "Orphan Course Product", 30000)

		orderID := seedPaidOrder(t, env, studentID, productID, "course")
		seedOutboxOrderPaid(t, env, orderID, productID, "course")

		repo := repository.New(env.pool)
		w := worker.New(env.pool, env.rdb, repo, 50*time.Millisecond, 200*time.Millisecond, 5*time.Minute, nil, nil, nil, nil, time.Hour, "", nil)
		wCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go w.Run(wCtx)

		// Poll until the order is completed (digital-only, worker skips shipping).
		ok := pollUntil(t, 5*time.Second, func() bool {
			var status string
			_ = env.pool.QueryRow(ctx,
				`SELECT status FROM orders WHERE id=$1`, orderID,
			).Scan(&status)
			return status == "completed"
		})
		cancel()
		require.True(t, ok, "timed out waiting for order to reach completed")

		// No sessions must have been created.
		var sessionCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM course_session WHERE student_id=$1 AND order_id=$2`,
			studentID, orderID,
		).Scan(&sessionCount))
		assert.Equal(t, 0, sessionCount, "no session should be created for zero-linked product")

		// Outbox must be processed.
		var processedCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='OrderPaid' AND processed_at IS NOT NULL`,
			orderID,
		).Scan(&processedCount))
		assert.Equal(t, 1, processedCount)
	})
}
