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
	"affiliate_transactions",
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
	"affiliate_commissions",
	"affiliate_payouts",
	"affiliate_adjustments",
	"affiliate_ledger_entries",
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

	const insertCommission = `INSERT INTO affiliate_transactions
		(affiliator_id, type, status, amount, student_id, order_id, order_item_id, product_type, quantity, base_amount, detail)
		VALUES ($1, 'commission', 'pending', 10000, $2, $3, $4, 'course', 1, 100000, '{"rule":{"type":"percent","value":10},"level":"bronze"}')
		RETURNING id`
	var commissionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, insertCommission, affiliatorUserID, studentUserID, orderID, itemID).Scan(&commissionID))
	_, err = pool.Exec(ctx, insertCommission, affiliatorUserID, studentUserID, orderID, itemID)
	require.Equal(t, "23505", pgErrCode(t, err), "a duplicate commission for one order item must violate the order_item_id unique index")
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount) VALUES ($1, 'commission', 'paid', 10000)`,
		affiliatorUserID,
	)
	require.Equal(t, "23514", pgErrCode(t, err), "commission statuses are pending, available and cancelled")
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at) VALUES ($1, 'payout', 'requested', 5000, now())`,
		affiliatorUserID,
	)
	require.Equal(t, "23514", pgErrCode(t, err), "a payout debits the balance, so its amount must be negative")

	// The guard freezes money fields, allows status moves along the lifecycle
	// only, sets effective_at once and refuses DELETE.
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET amount = 99999 WHERE id = $1`, commissionID)
	require.Error(t, err, "amount must be immutable")
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET detail = '{}' WHERE id = $1`, commissionID)
	require.Error(t, err, "a commission's detail must be immutable")
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET status = 'available' WHERE id = $1`, commissionID)
	require.Equal(t, "23514", pgErrCode(t, err), "an available commission needs effective_at")
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET status = 'available', effective_at = now() WHERE id = $1`, commissionID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET effective_at = now() + interval '1 day' WHERE id = $1`, commissionID)
	require.Error(t, err, "effective_at must be settable once")
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET status = 'pending' WHERE id = $1`, commissionID)
	require.Error(t, err, "an available commission cannot go back to pending")
	_, err = pool.Exec(ctx, `DELETE FROM affiliate_transactions WHERE id = $1`, commissionID)
	require.Error(t, err, "rows must refuse DELETE")

	// A refund deduction and a payout reversal each exist once per parent.
	const insertDeduction = `INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, parent_id)
		VALUES ($1, 'refund_deduction', 'posted', -10000, now(), $2)`
	_, err = pool.Exec(ctx, insertDeduction, affiliatorUserID, commissionID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, insertDeduction, affiliatorUserID, commissionID)
	require.Equal(t, "23505", pgErrCode(t, err), "one refund deduction per commission")
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at) VALUES ($1, 'refund_deduction', 'posted', -1, now())`,
		affiliatorUserID,
	)
	require.Equal(t, "23514", pgErrCode(t, err), "a deduction needs a parent")

	// One payout in flight per affiliator; the decided one frees the slot and
	// only moves along requested -> processing -> paid|rejected.
	var payoutID uuid.UUID
	const insertPayout = `INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, detail)
		VALUES ($1, 'payout', 'requested', -10000, now(), '{"bank_account":{"bank_name":"BCA"}}') RETURNING id`
	require.NoError(t, pool.QueryRow(ctx, insertPayout, affiliatorUserID).Scan(&payoutID))
	_, err = pool.Exec(ctx, insertPayout, affiliatorUserID)
	require.Equal(t, "23505", pgErrCode(t, err), "a second in-flight payout must violate the partial unique index")
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET detail = '{}' WHERE id = $1`, payoutID)
	require.Error(t, err, "a payout's detail changes only together with its status")
	_, err = pool.Exec(ctx,
		`UPDATE affiliate_transactions SET status = 'rejected', detail = detail || '{"reject_reason":"x"}' WHERE id = $1`, payoutID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE affiliate_transactions SET status = 'paid' WHERE id = $1`, payoutID)
	require.Error(t, err, "a rejected payout is final")
	require.NoError(t, pool.QueryRow(ctx, insertPayout, affiliatorUserID).Scan(&payoutID), "a decided payout frees the in-flight slot")

	// The balance on a date is the sum of what had taken effect by then.
	var saldo float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_transactions WHERE affiliator_id = $1 AND effective_at <= now()`,
		affiliatorUserID,
	).Scan(&saldo))
	require.Equal(t, -20000.0, saldo, "commission 10000 - deduction 10000 - payout 10000 - payout 10000")

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
