package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// affiliateTables is every table migration 0070 creates, in creation order.
var affiliateTables = []string{
	"affiliators",
	"affiliate_bindings",
	"affiliate_commissions",
	"affiliate_payouts",
	"affiliate_adjustments",
	"affiliate_ledger_entries",
	"affiliate_settings",
}

// removedAffiliateTables are the tables of earlier drafts that the schema no
// longer has.
var removedAffiliateTables = []string{
	"affiliate_clicks",
	"affiliate_flags",
	"affiliate_violations",
	"affiliate_level_history",
	"affiliate_level_settings",
	"affiliate_product_overrides",
}

func requireAffiliateTableExists(t *testing.T, pool *pgxpool.Pool, table string, want bool) {
	t.Helper()
	ctx := context.Background()
	var exists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`,
		table,
	).Scan(&exists))
	require.Equal(t, want, exists, "table %s existence", table)
}

func pgErrCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	pgErr, ok := err.(*pgconn.PgError)
	require.True(t, ok, "expected a pgconn.PgError, got %T: %v", err, err)
	return pgErr.Code
}

func TestMigration0070_AffiliateSchema(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Red pre-state: bring the DB to exactly the 0069 schema.
	applyMigrationsUpTo(t, pool, "0069_exam_product_stock.up.sql")

	for _, table := range affiliateTables {
		requireAffiliateTableExists(t, pool, table, false)
	}
	requireColumnExists(t, pool, "orders", "affiliate_code", false)
	requireColumnExists(t, pool, "product", "affiliate_commission", false)
	_, err := pool.Exec(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'affiliator', 'Before')`,
		"0070-pre@test.local",
	)
	require.Error(t, err, "role 'affiliator' must be rejected before 0070")

	// Apply 0070 up.
	applyMigrationFile(t, pool, "0070_affiliate.up.sql")

	for _, table := range affiliateTables {
		requireAffiliateTableExists(t, pool, table, true)
	}
	for _, table := range removedAffiliateTables {
		requireAffiliateTableExists(t, pool, table, false)
	}
	requireColumnExists(t, pool, "orders", "affiliate_code", true)
	requireColumnExists(t, pool, "product", "affiliate_commission", true)

	// The affiliator role is now a valid users.role, and an unknown role still fails.
	var affiliatorUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'affiliator', 'Pak Budi') RETURNING id`,
		"0070-affiliator@test.local",
	).Scan(&affiliatorUserID))
	var studentUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'Ani') RETURNING id`,
		"0070-student@test.local",
	).Scan(&studentUserID))
	_, err = pool.Exec(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'ghost_role', 'Ghost')`,
		"0070-ghost@test.local",
	)
	require.Equal(t, "23514", pgErrCode(t, err), "unknown role must still violate the CHECK constraint")

	// The affiliate profile hangs off users: user_id is its key, the referral
	// code is empty until approval and unique afterwards.
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliators (user_id, terms_version) VALUES ($1, 'v1')`,
		affiliatorUserID,
	)
	require.NoError(t, err, "a profile starts without a referral code")
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliators (user_id, terms_version) VALUES ($1, 'v1')`,
		affiliatorUserID,
	)
	require.Equal(t, "23505", pgErrCode(t, err), "one profile per user")
	_, err = pool.Exec(ctx,
		`UPDATE affiliators SET referral_code = 'AFFTEST1', program_status = 'active' WHERE user_id = $1`,
		affiliatorUserID,
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE affiliators SET program_status = 'banned' WHERE user_id = $1`,
		affiliatorUserID,
	)
	require.Equal(t, "23514", pgErrCode(t, err), "an unknown program status must violate the CHECK constraint")
	var level string
	var missed int
	var lastEvaluated *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT level, missed_quarters, last_evaluated_quarter FROM affiliators WHERE user_id = $1`,
		affiliatorUserID,
	).Scan(&level, &missed, &lastEvaluated))
	require.Equal(t, "bronze", level)
	require.Equal(t, 0, missed)
	require.Nil(t, lastEvaluated)

	// One active binding per student: a second active insert violates the
	// partial unique index; an ended binding never blocks a new active one.
	var bindingID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, source) VALUES ($1, $2, 'link') RETURNING id`,
		affiliatorUserID, studentUserID,
	).Scan(&bindingID))
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, source) VALUES ($1, $2, 'manual_code')`,
		affiliatorUserID, studentUserID,
	)
	require.Equal(t, "23505", pgErrCode(t, err), "a second active binding for one student must violate uq_affiliate_bindings_student_active")

	_, err = pool.Exec(ctx,
		`UPDATE affiliate_bindings SET status = 'ended', expires_at = now() WHERE id = $1`,
		bindingID,
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, source) VALUES ($1, $2, 'checkout_code')`,
		affiliatorUserID, studentUserID,
	)
	require.NoError(t, err, "a new active binding must be allowed after the old one ended")

	// Exactly one commission per order item: the unique order_item_id rejects
	// the duplicate while the first insert stands.
	var productID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price) VALUES ('course', 'Kursus', 100000) RETURNING id`,
	).Scan(&productID))
	_, err = pool.Exec(ctx,
		`UPDATE product SET affiliate_commission = '{"type":"percent","value":5}' WHERE id = $1`,
		productID,
	)
	require.NoError(t, err, "a product can carry its own commission override")
	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total, affiliate_code) VALUES ($1, 'paid', 100000, 100000, 'AFFTEST1') RETURNING id`,
		studentUserID,
	).Scan(&orderID))
	var itemID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah) VALUES ($1, $2, 'course', 'Kursus', 100000, 1, 100000) RETURNING id`,
		orderID, productID,
	).Scan(&itemID))

	var commissionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_commissions (affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, commission_value, level_snapshot, amount, paid_at)
		VALUES ($1, $2, $3, $4, 'course', 100000, 10, 'bronze', 10000, now()) RETURNING id`,
		affiliatorUserID, studentUserID, orderID, itemID,
	).Scan(&commissionID))
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_commissions (affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, commission_value, level_snapshot, amount, paid_at)
		VALUES ($1, $2, $3, $4, 'course', 100000, 10, 'bronze', 10000, now())`,
		affiliatorUserID, studentUserID, orderID, itemID,
	)
	require.Equal(t, "23505", pgErrCode(t, err), "a duplicate commission for one order item must violate the order_item_id UNIQUE constraint")
	_, err = pool.Exec(ctx, `UPDATE affiliate_commissions SET status = 'paid' WHERE id = $1`, commissionID)
	require.Equal(t, "23514", pgErrCode(t, err), "commission statuses are pending, available and cancelled")

	// Ledger: append-only trigger rejects UPDATE and DELETE; the unique
	// (commission_id, entry_type) pair rejects a second availability entry.
	var ledgerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id) VALUES ($1, 'commission', 10000, $2) RETURNING id`,
		affiliatorUserID, commissionID,
	).Scan(&ledgerID))
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id) VALUES ($1, 'commission', 10000, $2)`,
		affiliatorUserID, commissionID,
	)
	require.Equal(t, "23505", pgErrCode(t, err), "a second commission entry for one commission must violate uq_affiliate_ledger_commission")
	_, err = pool.Exec(ctx, `UPDATE affiliate_ledger_entries SET amount = 0 WHERE id = $1`, ledgerID)
	require.Error(t, err, "ledger rows must refuse UPDATE (append-only)")
	_, err = pool.Exec(ctx, `DELETE FROM affiliate_ledger_entries WHERE id = $1`, ledgerID)
	require.Error(t, err, "ledger rows must refuse DELETE (append-only)")

	// Settings is a singleton whose levels column starts empty.
	_, err = pool.Exec(ctx, `INSERT INTO affiliate_settings (id) VALUES (1)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO affiliate_settings (id) VALUES (2)`)
	require.Equal(t, "23514", pgErrCode(t, err), "only the id = 1 settings row may exist")
	var levels string
	require.NoError(t, pool.QueryRow(ctx, `SELECT levels::text FROM affiliate_settings WHERE id = 1`).Scan(&levels))
	require.Equal(t, "{}", levels)

	// Apply 0070 down: schema reverts and the affiliator role is rejected again.
	applyMigrationFile(t, pool, "0070_affiliate.down.sql")

	for _, table := range affiliateTables {
		requireAffiliateTableExists(t, pool, table, false)
	}
	requireColumnExists(t, pool, "orders", "affiliate_code", false)
	requireColumnExists(t, pool, "product", "affiliate_commission", false)
	_, err = pool.Exec(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'affiliator', 'After')`,
		"0070-after@test.local",
	)
	require.Error(t, err, "role 'affiliator' must be rejected again after down")
}
