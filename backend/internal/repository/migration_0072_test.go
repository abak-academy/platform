package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0072_PartnerProgram(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0071_exam_event_info.up.sql")

	requireColumnExists(t, pool, "users", "registered_by", false)
	requireColumnExists(t, pool, "affiliators", "programs", false)
	requireColumnExists(t, pool, "product", "partner_access", false)
	requireColumnExists(t, pool, "promo_code", "partner", false)

	var requiredNullable []string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT array_agg(column_name::text ORDER BY column_name) FROM information_schema.columns
		 WHERE table_name = 'affiliate_settings' AND column_name IN
		   ('cookie_days','binding_months','digital_wait_days','physical_wait_days','min_payout','eval_day')
		   AND is_nullable = 'NO'`,
	).Scan(&requiredNullable))
	require.Len(t, requiredNullable, 6, "before 0072 the six required settings are NOT NULL")

	applyMigrationFile(t, pool, "0072_partner_program.up.sql")

	requireColumnExists(t, pool, "users", "registered_by", true)
	requireColumnExists(t, pool, "affiliators", "programs", true)
	requireColumnExists(t, pool, "product", "partner_access", true)
	requireColumnExists(t, pool, "promo_code", "partner", true)

	var indexDef string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_users_registered_by'`,
	).Scan(&indexDef))
	require.Contains(t, indexDef, "WHERE (registered_by IS NOT NULL)", "the index is partial")

	// With no row, or a row with nothing set, no required setting has a value.
	_, err := pool.Exec(ctx, `INSERT INTO affiliate_settings (id) VALUES (1)`)
	require.NoError(t, err, "a settings row can be created with every required setting unset")
	var cookie, binding, digital, physical, evalDay *int
	var minPayout *float64
	var minExtend float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT cookie_days, binding_months, digital_wait_days, physical_wait_days, min_payout, eval_day, min_extend_amount
		 FROM affiliate_settings WHERE id = 1`,
	).Scan(&cookie, &binding, &digital, &physical, &minPayout, &evalDay, &minExtend))
	for name, v := range map[string]*int{"cookie_days": cookie, "binding_months": binding, "digital_wait_days": digital, "physical_wait_days": physical, "eval_day": evalDay} {
		require.Nil(t, v, "%s has no default", name)
	}
	require.Nil(t, minPayout, "min_payout has no default")
	require.Equal(t, 0.0, minExtend, "min_extend_amount keeps its zero default")

	var registeredBy *string
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ('0072-student@test.local', 'student', 'Ani') RETURNING id::text, registered_by::text`,
	).Scan(&userID, &registeredBy))
	require.Nil(t, registeredBy)

	_, err = pool.Exec(ctx,
		`INSERT INTO users (email, role, name, registered_by) VALUES ('0072-orphan@test.local', 'student', 'Budi', gen_random_uuid())`,
	)
	require.Equal(t, "23503", pgErrCode(t, err), "registered_by must reference a user")

	// Down restores the old shape; a NULL required setting is backfilled first.
	applyMigrationFile(t, pool, "0072_partner_program.down.sql")

	requireColumnExists(t, pool, "users", "registered_by", false)
	requireColumnExists(t, pool, "affiliators", "programs", false)
	requireColumnExists(t, pool, "product", "partner_access", false)
	requireColumnExists(t, pool, "promo_code", "partner", false)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT cookie_days, min_payout FROM affiliate_settings WHERE id = 1`,
	).Scan(&cookie, &minPayout))
	require.NotNil(t, cookie)
	require.Equal(t, 30, *cookie)
	require.Equal(t, 100000.0, *minPayout)
	_, err = pool.Exec(ctx, `UPDATE affiliate_settings SET cookie_days = NULL WHERE id = 1`)
	require.Equal(t, "23502", pgErrCode(t, err), "the required settings are NOT NULL again after down")
}
