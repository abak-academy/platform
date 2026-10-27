package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigration0073_AffiliateDecisions(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0072_affiliator_code_nullable.up.sql")

	var affiliatorUserID, studentUserID, productID, orderID, itemID, affiliatorID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ('0073-aff@test.local', 'affiliator', 'Pak Budi') RETURNING id`).Scan(&affiliatorUserID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ('0073-student@test.local', 'student', 'Ani') RETURNING id`).Scan(&studentUserID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliators (user_id, code, terms_version, status) VALUES ($1, 'AFF73001', 'v1', 'active') RETURNING id`,
		affiliatorUserID).Scan(&affiliatorID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price) VALUES ('merchandise', 'Kaos', 50000) RETURNING id`).Scan(&productID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, total) VALUES ($1, 'paid', 150000, 150000) RETURNING id`,
		studentUserID).Scan(&orderID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
		 VALUES ($1, $2, 'merchandise', 'Kaos', 50000, 3, 150000) RETURNING id`, orderID, productID).Scan(&itemID))
	_, err := pool.Exec(ctx,
		`INSERT INTO affiliate_commissions (affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, percentage, level_snapshot, amount)
		 VALUES ($1, $2, $3, $4, 'merchandise', 150000, 7.5, 'bronze', 11250)`,
		affiliatorID, studentUserID, orderID, itemID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO affiliate_product_overrides (product_id, percentage) VALUES ($1, 12.5)`, productID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_settings (id, physical_received_days, physical_wait_days) VALUES (1, 7, 7)
		 ON CONFLICT (id) DO UPDATE SET physical_received_days = 7, physical_wait_days = 7`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_level_settings (level, commission_pct, physical_received_days, physical_wait_days) VALUES
		     ('bronze', NULL, NULL, NULL),
		     ('silver', '{"exam": 12, "book": 5.5}', 2, 3),
		     ('gold', '{}', 4, NULL)`)
	require.NoError(t, err)

	applyMigrationFile(t, pool, "0073_affiliate_decisions.up.sql")

	t.Run("physical wait folds received into wait", func(t *testing.T) {
		var global int
		require.NoError(t, pool.QueryRow(ctx, `SELECT physical_wait_days FROM affiliate_settings WHERE id = 1`).Scan(&global))
		require.Equal(t, 14, global)

		waits := map[string]*int{}
		rows, err := pool.Query(ctx, `SELECT level, physical_wait_days FROM affiliate_level_settings`)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var level string
			var wait *int
			require.NoError(t, rows.Scan(&level, &wait))
			waits[level] = wait
		}
		require.Nil(t, waits["bronze"], "a level that set neither half keeps following the global value")
		require.Equal(t, 5, *waits["silver"])
		require.Equal(t, 11, *waits["gold"], "level received 4 plus the global wait 7")
	})

	t.Run("received column is gone and the new default is 14", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE affiliate_settings SET physical_received_days = 1`)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM affiliate_settings`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO affiliate_settings (id) VALUES (1)`)
		require.NoError(t, err)
		var wait int
		require.NoError(t, pool.QueryRow(ctx, `SELECT physical_wait_days FROM affiliate_settings WHERE id = 1`).Scan(&wait))
		require.Equal(t, 14, wait)
	})

	t.Run("commission keeps its percentage as a snapshot and gains quantity", func(t *testing.T) {
		var ctype string
		var value float64
		var qty int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT commission_type, commission_value, quantity FROM affiliate_commissions WHERE order_item_id = $1`, itemID,
		).Scan(&ctype, &value, &qty))
		require.Equal(t, "percent", ctype)
		require.Equal(t, 7.5, value)
		require.Equal(t, 3, qty)
	})

	t.Run("override becomes a typed value", func(t *testing.T) {
		var otype string
		var value float64
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT type, value FROM affiliate_product_overrides WHERE product_id = $1`, productID).Scan(&otype, &value))
		require.Equal(t, "percent", otype)
		require.Equal(t, 12.5, value)

		_, err := pool.Exec(ctx, `UPDATE affiliate_product_overrides SET type = 'fixed', value = 4500 WHERE product_id = $1`, productID)
		require.NoError(t, err, "a fixed override may exceed 100")
		_, err = pool.Exec(ctx, `UPDATE affiliate_product_overrides SET type = 'percent', value = 101 WHERE product_id = $1`, productID)
		require.Equal(t, "23514", pgErrCode(t, err), "a percent override above 100 must violate the check")
	})

	t.Run("level commission map becomes typed rules", func(t *testing.T) {
		var examType, bookType string
		var examValue, bookValue float64
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT commission_rule #>> '{exam,type}', (commission_rule #>> '{exam,value}')::numeric,
			        commission_rule #>> '{book,type}', (commission_rule #>> '{book,value}')::numeric
			 FROM affiliate_level_settings WHERE level = 'silver'`,
		).Scan(&examType, &examValue, &bookType, &bookValue))
		require.Equal(t, "percent", examType)
		require.Equal(t, 12.0, examValue)
		require.Equal(t, "percent", bookType)
		require.Equal(t, 5.5, bookValue)

		var bronzeRule *string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT commission_rule::text FROM affiliate_level_settings WHERE level = 'bronze'`).Scan(&bronzeRule))
		require.Nil(t, bronzeRule, "a level with no rule keeps following the global rules")
	})

	t.Run("level gains an optional binding length", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE affiliate_level_settings SET binding_months = 6 WHERE level = 'gold'`)
		require.NoError(t, err)
	})

	t.Run("one automatic level row per affiliator and quarter", func(t *testing.T) {
		insert := func(source string) error {
			_, err := pool.Exec(ctx,
				`INSERT INTO affiliate_level_history (affiliator_id, quarter, to_level, source)
				 VALUES ($1, '2026-Q3', 'silver', $2)`, affiliatorID, source)
			return err
		}
		require.NoError(t, insert("auto"))
		require.Equal(t, "23505", pgErrCode(t, insert("auto")), "a second auto row for the quarter must be rejected")
		require.NoError(t, insert("manual"), "manual changes in the same quarter stay allowed")
		require.NoError(t, insert("manual"))
	})
}
