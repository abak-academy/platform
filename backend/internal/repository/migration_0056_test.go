package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0056_DigitalQtyBackfill proves FR-17: the report is written
// before anything is corrected, the corrective UPDATE touches only carts, and
// a corrected cart never keeps a discount that was sized against the inflated
// subtotal.
//
// The discount half is the sharp one. An exam at qty 3 x 100000 with a 150000
// promo has subtotal 300000 and total 150000. Correcting qty to 1 drops the
// subtotal to 100000, so carrying the discount forward would compute
// 100000 - 150000 = -50000 — a negative total on a live cart that checkout
// would hand to the payment gateway.
func TestMigration0056_DigitalQtyBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0055_order_payment_proof.up.sql")

	var studentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"migration-0056@test.local", "student", "Migration 0056 Test",
	).Scan(&studentID))

	var examProductID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price, status) VALUES ('exam', 'Paket Ujian', 100000, 'published') RETURNING id`,
	).Scan(&examProductID))

	var promoID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO promo_code (code) VALUES ('MIGRATION-0056') RETURNING id`,
	).Scan(&promoID))

	// A cart carrying the overcharge AND a promo sized against it.
	var cartID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, promo_code_id)
		 VALUES ($1, 'cart', 300000, 150000, 0, 150000, $2) RETURNING id`,
		studentID, promoID,
	).Scan(&cartID))
	cartItemID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, created_at)
		 VALUES ($1, $2, $3, 'exam', 'Paket Ujian', 100000, 3, 300000, now())`,
		cartItemID, cartID, examProductID,
	)
	require.NoError(t, err)

	// A paid order with the same defect: reported, never rewritten.
	var paidID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, paid_at)
		 VALUES ($1, 'paid', 200000, 0, 0, 200000, now()) RETURNING id`,
		studentID,
	).Scan(&paidID))
	paidItemID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, created_at)
		 VALUES ($1, $2, $3, 'exam', 'Paket Ujian', 100000, 2, 200000, now())`,
		paidItemID, paidID, examProductID,
	)
	require.NoError(t, err)

	applyMigrationFile(t, pool, "0056_digital_qty_backfill.up.sql")

	// Both rows are reported, with the overcharge derived from what was charged.
	var reported int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM digital_qty_overcharge_report WHERE order_id IN ($1, $2)`,
		cartID, paidID,
	).Scan(&reported))
	require.Equal(t, 2, reported, "both the cart and the paid order must be reported")

	var cartOvercharge, paidOvercharge float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT overcharge FROM digital_qty_overcharge_report WHERE order_id = $1`, cartID,
	).Scan(&cartOvercharge))
	require.InDelta(t, 200000, cartOvercharge, 0.01, "cart overcharge = unit_price * (qty - 1)")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT overcharge FROM digital_qty_overcharge_report WHERE order_id = $1`, paidID,
	).Scan(&paidOvercharge))
	require.InDelta(t, 100000, paidOvercharge, 0.01, "paid overcharge = unit_price * (qty - 1)")

	// The cart is corrected and its money recomputed coherently.
	var qty int
	var jumlah float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT qty, jumlah FROM order_item WHERE id = $1`, cartItemID,
	).Scan(&qty, &jumlah))
	require.Equal(t, 1, qty, "digital line on a cart must be corrected to qty 1")
	require.InDelta(t, 100000, jumlah, 0.01, "jumlah must follow qty, or qty * unit_price != jumlah")

	var subtotal, discount, total float64
	var promoCodeID *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT subtotal, discount, total, promo_code_id FROM orders WHERE id = $1`, cartID,
	).Scan(&subtotal, &discount, &total, &promoCodeID))
	require.InDelta(t, 100000, subtotal, 0.01, "subtotal must be recomputed from the corrected items")
	require.InDelta(t, 0, discount, 0.01, "a discount sized against the inflated subtotal must not survive")
	require.Nil(t, promoCodeID, "the promo must be detached so the buyer re-applies it against the real subtotal")
	require.InDelta(t, 100000, total, 0.01, "total = corrected subtotal + shipping")
	require.GreaterOrEqual(t, total, 0.0, "a corrected cart must never carry a negative total")

	// The paid order is evidence of money actually taken — byte-for-byte intact.
	var paidQty int
	var paidJumlah, paidSubtotal, paidTotal float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT qty, jumlah FROM order_item WHERE id = $1`, paidItemID,
	).Scan(&paidQty, &paidJumlah))
	require.Equal(t, 2, paidQty, "a paid order must not be rewritten")
	require.InDelta(t, 200000, paidJumlah, 0.01, "a paid order must not be rewritten")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT subtotal, total FROM orders WHERE id = $1`, paidID,
	).Scan(&paidSubtotal, &paidTotal))
	require.InDelta(t, 200000, paidSubtotal, 0.01, "a paid order's subtotal must not be rewritten")
	require.InDelta(t, 200000, paidTotal, 0.01, "a paid order's total must not be rewritten")
}
