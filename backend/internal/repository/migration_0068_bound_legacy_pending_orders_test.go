package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0068_BoundsLegacyStudentDigitalOrders proves 0068 gives a
// deadline-less payment_pending student digital order one 24h window and leaves
// every other order alone.
func TestMigration0068_BoundsLegacyStudentDigitalOrders(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0067_users_password_temporary.up.sql")

	insertUser := func(email, role string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (email, role, name) VALUES ($1, $2, 'Migration 0068 Test') RETURNING id`, email, role,
		).Scan(&id))
		return id
	}
	insertProduct := func(productType string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO product (type, name, price, stock, status) VALUES ($1, 'p', 1000, 5, 'published') RETURNING id`, productType,
		).Scan(&id))
		return id
	}
	insertOrder := func(buyer uuid.UUID, status string, deadline *time.Time, products ...uuid.UUID) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, payment_expires_at) VALUES ($1, $2, $3) RETURNING id`, buyer, status, deadline,
		).Scan(&id))
		for _, p := range products {
			_, err := pool.Exec(ctx,
				`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
				 SELECT $1, id, type, 'x', 1, 1, 1 FROM product WHERE id = $2`, id, p)
			require.NoError(t, err)
		}
		return id
	}

	student := insertUser("student-0068@test.local", "student")
	school := insertUser("school-0068@test.local", "admin_school")
	course, exam, book := insertProduct("course"), insertProduct("exam"), insertProduct("book")

	courseOrder := insertOrder(student, "payment_pending", nil, course)
	mixedOrder := insertOrder(student, "payment_pending", nil, exam, book)
	physicalOnly := insertOrder(student, "payment_pending", nil, book)
	paid := insertOrder(student, "paid", nil, course)
	schoolOrder := insertOrder(school, "payment_pending", nil, course)
	bulk := insertOrder(student, "payment_pending", nil, exam)
	_, err := pool.Exec(ctx, `INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, bulk, student)
	require.NoError(t, err)
	existing := time.Now().Add(90 * time.Minute).Truncate(time.Microsecond)
	withDeadline := insertOrder(student, "payment_pending", &existing, course)

	deadlineOf := func(id uuid.UUID) *time.Time {
		var d *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT payment_expires_at FROM orders WHERE id = $1`, id).Scan(&d))
		return d
	}

	applyMigrationFile(t, pool, "0068_bound_legacy_pending_orders.up.sql")

	for name, id := range map[string]uuid.UUID{"student digital": courseOrder, "student digital + physical": mixedOrder} {
		d := deadlineOf(id)
		require.NotNil(t, d, "%s order must get a deadline", name)
		require.WithinDuration(t, time.Now().Add(24*time.Hour), *d, time.Minute, "%s order window", name)
	}
	for name, id := range map[string]uuid.UUID{
		"physical-only": physicalOnly, "paid": paid, "non-student buyer": schoolOrder, "bulk participant": bulk,
	} {
		require.Nil(t, deadlineOf(id), "%s order must stay untouched", name)
	}
	require.True(t, existing.Equal(*deadlineOf(withDeadline)), "an existing deadline must not move")
}
