package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func requireOrdersAffiliateCodeColumn(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'orders' AND column_name = 'affiliate_code'
		)`,
	).Scan(&exists))
	require.Equal(t, want, exists, "orders.affiliate_code column existence")
}

func TestMigration0071_OrdersAffiliateCode(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Red pre-state: bring the DB to exactly the 0070 schema.
	applyMigrationsUpTo(t, pool, "0070_affiliate.up.sql")
	requireOrdersAffiliateCodeColumn(t, pool, false)

	var studentUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'Ani') RETURNING id`,
		"0071-student@test.local",
	).Scan(&studentUserID))
	_, err := pool.Exec(ctx,
		`INSERT INTO orders (student_id, status, affiliate_code) VALUES ($1, 'payment_pending', 'AFFX1')`,
		studentUserID,
	)
	require.Error(t, err, "orders must reject affiliate_code before 0071")

	applyMigrationFile(t, pool, "0071_affiliate_order_code.up.sql")
	requireOrdersAffiliateCodeColumn(t, pool, true)

	// The column stores a checkout-time code and stays NULL for the rest.
	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, affiliate_code) VALUES ($1, 'payment_pending', 'AFFX1') RETURNING id`,
		studentUserID,
	).Scan(&orderID))
	var stored *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT affiliate_code FROM orders WHERE id = $1`, orderID).Scan(&stored))
	require.NotNil(t, stored)
	require.Equal(t, "AFFX1", *stored)
	var plainOrderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status) VALUES ($1, 'paid') RETURNING id`,
		studentUserID,
	).Scan(&plainOrderID))
	require.NoError(t, pool.QueryRow(ctx, `SELECT affiliate_code FROM orders WHERE id = $1`, plainOrderID).Scan(&stored))
	require.Nil(t, stored, "an order checked out without a code must store NULL")

	// Down restores the 0070 shape.
	applyMigrationFile(t, pool, "0071_affiliate_order_code.down.sql")
	requireOrdersAffiliateCodeColumn(t, pool, false)
}
