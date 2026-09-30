package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Site f (checkout stock enforcement): merchandise must be range-checked and
// decremented at checkout exactly like a book, else it oversells and never
// decrements.
func TestCheckoutOrder_MerchandiseStockEnforcedAndDecremented(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	// Sufficient stock: checkout decrements stock by qty.
	prodID := seedMerchProductRow(t, pool, 5)
	orderID := seedMerchOrderRow(t, pool, prodID, 2)
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CheckoutOrder(ctx, tx, orderID))
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, 3, merchStock(t, pool, prodID), "merchandise stock must decrement by qty")

	// Insufficient stock: checkout fails and stock is unchanged.
	prodID2 := seedMerchProductRow(t, pool, 1)
	orderID2 := seedMerchOrderRow(t, pool, prodID2, 3)
	tx2, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	err = repo.CheckoutOrder(ctx, tx2, orderID2)
	require.ErrorIs(t, err, ErrInsufficientStock)
	_ = tx2.Rollback(ctx)
	require.Equal(t, 1, merchStock(t, pool, prodID2), "stock must be unchanged after an insufficient-stock checkout")
}

func TestCheckoutOrder_MedalStockEnforcedAndDecremented(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	prodID := seedPhysicalProductRow(t, pool, "medal", 5)
	orderID := seedPhysicalOrderRow(t, pool, prodID, "medal", 2)
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CheckoutOrder(ctx, tx, orderID))
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, 3, merchStock(t, pool, prodID))
}

func seedMerchProductRow(t *testing.T, pool *pgxpool.Pool, stock int) uuid.UUID {
	return seedPhysicalProductRow(t, pool, "merchandise", stock)
}

func seedPhysicalProductRow(t *testing.T, pool *pgxpool.Pool, productType string, stock int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ($1, 'Academy Tee', 100, $2, 'published') RETURNING id`,
		productType, stock,
	).Scan(&id))
	return id
}

func seedMerchOrderRow(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, qty int) uuid.UUID {
	return seedPhysicalOrderRow(t, pool, productID, "merchandise", qty)
}

func seedPhysicalOrderRow(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, productType string, qty int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	studentID := insertGradingUser(t, pool, "student", "Merch Buyer")
	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'cart', 0, 0, 0, 0) RETURNING id`, studentID,
	).Scan(&orderID))
	_, err := pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, $4, 'Academy Tee', 100, $5, $6, 0, now())`,
		uuid.New(), orderID, productID, productType, qty, float64(100*qty),
	)
	require.NoError(t, err)
	return orderID
}

func merchStock(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID) int {
	t.Helper()
	var stock int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT stock FROM product WHERE id = $1`, productID).Scan(&stock))
	return stock
}

// TestRemoveItem_ClearsShippingCostInvariant_Real verifies the total invariant
// is maintained when clearShipping=true clears shipping cost.
// This is a real DB test because the bug (pre-image evaluation in SQL SET clauses)
// only manifests in actual Postgres, not in shim/in-memory reimplementations.
func TestRemoveItem_ClearsShippingCostInvariant_Real(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	studentID := insertGradingUser(t, pool, "student", "Test Buyer")
	bookProdID := seedPhysicalProductRow(t, pool, "book", 10)
	courseProdID := seedPhysicalProductRow(t, pool, "course", 10)

	orderID := uuid.New()
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (id, student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, $2, 'cart', 100000, 0, 15000, 115000)
		 RETURNING id`, orderID, studentID).Scan(&orderID))

	bookItemID := uuid.New()
	courseItemID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, created_at)
		 VALUES ($1, $2, $3, 'book', 'Book', 50000, 1, 50000, now())`,
		bookItemID, orderID, bookProdID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, created_at)
		 VALUES ($1, $2, $3, 'course', 'Course', 50000, 1, 50000, now())`,
		courseItemID, orderID, courseProdID,
	)
	require.NoError(t, err)

	require.NoError(t, repo.RemoveItem(ctx, orderID, bookItemID, true))

	var subtotal, discount, shippingCost, total float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT subtotal, discount, shipping_cost, total FROM orders WHERE id = $1`,
		orderID,
	).Scan(&subtotal, &discount, &shippingCost, &total))

	require.Equal(t, 50000.0, subtotal, "subtotal should be sum of remaining items")
	require.Equal(t, 0.0, discount, "discount unchanged")
	require.Equal(t, 0.0, shippingCost, "shipping_cost should be cleared")

	expectedTotal := subtotal - discount + shippingCost
	require.Equal(t, expectedTotal, total, "total invariant: total = subtotal - discount + shipping_cost; got total=%v, expected=%v", total, expectedTotal)
}

// TestPatchCart_OmitsAddressFields_PreservesExisting verifies that PatchCart
// with omitted address fields (NULL in OrderPatch) preserves the order's
// existing address values via COALESCE, instead of corrupting them with
// FK-violating empty strings. This is a regression test for Bug A: nullable
// address field handling.
func TestPatchCart_OmitsAddressFields_PreservesExisting(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	studentID := insertGradingUser(t, pool, "student", "Test Buyer")

	// Insert order with NULL address fields initially
	orderID := uuid.New()
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (id, student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, $2, 'cart', 100000, 0, 15000, 115000)
		 RETURNING id`, orderID, studentID).Scan(&orderID))

	// Simulate a prior PatchCart call that set some address fields
	_, err := pool.Exec(ctx,
		`UPDATE orders SET shipping_address = $1, selected_courier = $2 WHERE id = $3`,
		[]byte(`{"street":"Jl. Merdeka"}`), "JNE", orderID)
	require.NoError(t, err)

	// Now patch with omitted address fields (all nil) — they should remain unchanged
	patch := OrderPatch{
		ShippingAddress: []byte(`{"street":"Jl. Merdeka"}`),
		SelectedCourier: "UPDATED",
		Discount:        0,
		ShippingCost:    15000,
		Total:           115000,
		ProvinceID:      nil,
		CityID:          nil,
		DistrictID:      nil,
		KodePos:         nil,
	}

	require.NoError(t, repo.PatchCart(ctx, orderID, patch))

	var courier string
	var provinceID, cityID, districtID *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT selected_courier, province_id, city_id, district_id FROM orders WHERE id = $1`, orderID,
	).Scan(&courier, &provinceID, &cityID, &districtID))

	require.Equal(t, "UPDATED", courier, "selected_courier should be updated")
	require.Nil(t, provinceID, "province_id must remain NULL when omitted from patch")
	require.Nil(t, cityID, "city_id must remain NULL when omitted from patch")
	require.Nil(t, districtID, "district_id must remain NULL when omitted from patch")
}
