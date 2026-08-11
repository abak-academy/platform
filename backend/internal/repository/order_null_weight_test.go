package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// order_item.weight_grams has been nullable since 0004_commerce, and
// 0009_schema_fixes backfilled the sibling `jumlah` column but missed this
// one. Migration 0059 backfills and locks the column NOT NULL going forward,
// but these tests exist to prove the *scan* itself survives a NULL row —
// belt and suspenders for any row that predates 0059, or reaches the table
// through a path that bypasses the constraint. The NOT NULL constraint is
// lifted just long enough to write one such row, the same shape a pre-0059
// row would have had, then restored so no other test in the shared pool sees
// a nullable column.
func TestFetchItems_NullWeightGramsScansAsZero(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	studentID := insertGradingUser(t, pool, "student", "Null Weight Buyer")
	prodID := seedPhysicalProductRow(t, pool, "book", 10)

	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'paid', 50000, 0, 0, 50000) RETURNING id`, studentID,
	).Scan(&orderID))

	_, err := pool.Exec(ctx, `ALTER TABLE order_item ALTER COLUMN weight_grams DROP NOT NULL`)
	require.NoError(t, err)
	defer func() {
		// SET NOT NULL would itself fail with 23502 if the row this test just
		// inserted is still NULL, so back it out before restoring the constraint.
		_, err := pool.Exec(ctx, `UPDATE order_item SET weight_grams = 0 WHERE weight_grams IS NULL`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `ALTER TABLE order_item ALTER COLUMN weight_grams SET NOT NULL`)
		require.NoError(t, err)
	}()

	_, err = pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, 'book', 'Null Weight Book', 50000, 1, 50000, NULL, now())`,
		uuid.New(), orderID, prodID,
	)
	require.NoError(t, err)

	orders, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &studentID, Limit: 10})
	require.NoError(t, err, "a NULL weight_grams must not error the scan")
	require.Len(t, orders, 1)
	require.Len(t, orders[0].Items, 1)
	require.Equal(t, 0, orders[0].Items[0].WeightGrams, "NULL weight_grams must read back as 0")
}

func TestCheckoutOrder_NullWeightGramsDoesNotError(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	studentID := insertGradingUser(t, pool, "student", "Null Weight Checkout Buyer")
	prodID := seedPhysicalProductRow(t, pool, "book", 10)

	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'cart', 50000, 0, 0, 50000) RETURNING id`, studentID,
	).Scan(&orderID))

	_, err := pool.Exec(ctx, `ALTER TABLE order_item ALTER COLUMN weight_grams DROP NOT NULL`)
	require.NoError(t, err)
	defer func() {
		// SET NOT NULL would itself fail with 23502 if the row this test just
		// inserted is still NULL, so back it out before restoring the constraint.
		_, err := pool.Exec(ctx, `UPDATE order_item SET weight_grams = 0 WHERE weight_grams IS NULL`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `ALTER TABLE order_item ALTER COLUMN weight_grams SET NOT NULL`)
		require.NoError(t, err)
	}()

	_, err = pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, 'book', 'Null Weight Book', 50000, 1, 50000, NULL, now())`,
		uuid.New(), orderID, prodID,
	)
	require.NoError(t, err)

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	// Runs before the NOT NULL restore above (LIFO): an aborted scan must not
	// leave the transaction open, or the later ALTER TABLE deadlocks on its
	// ACCESS SHARE lock on order_item.
	defer func() { _ = tx.Rollback(ctx) }()
	err = repo.CheckoutOrder(ctx, tx, orderID)
	require.NoError(t, err, "a NULL weight_grams must not error CheckoutOrder's scan")
	require.NoError(t, tx.Commit(ctx))
}
