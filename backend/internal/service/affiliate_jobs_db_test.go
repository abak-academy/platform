package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Job tests share the package real-DB fixture (rows never reset), so each test
// seeds its own settings singleton values, unique users/affiliators and unique
// WIB day keys, and never runs in parallel.
type affiliateJobSettingsSeed struct {
	minExtend    float64
	evalDay      int
	physicalWait int
}

func seedAffiliateJobSettings(t *testing.T, repo *repository.Repository, seed affiliateJobSettingsSeed) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, min_extend_amount, eval_day, physical_wait_days)
		 VALUES (1, $1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET min_extend_amount = $1, eval_day = $2,
		     physical_wait_days = $3, updated_at = now()`,
		seed.minExtend, seed.evalDay, seed.physicalWait,
	)
	require.NoError(t, err)
}

func restoreAffiliateJobSettings(t *testing.T, repo *repository.Repository) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`INSERT INTO affiliate_settings (id, min_extend_amount, eval_day, physical_wait_days)
			 VALUES (1, 0, 8, 14) ON CONFLICT (id) DO UPDATE SET min_extend_amount = 0, eval_day = 8,
			     physical_wait_days = 14, updated_at = now()`)
	})
}

func setAffiliateJobConfigKey(t *testing.T, repo *repository.Repository, key, value string) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ($1, $2, false)
		 ON CONFLICT (key) DO UPDATE SET value = $2`, key, value)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM system_config WHERE key = $1`, key)
	})
}

func clearAffiliateJobKeys(t *testing.T, repo *repository.Repository) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`DELETE FROM system_config WHERE key IN ($1, $2)`,
		affiliateDailyLastRunKey, affiliateLevelsLastQuarterKey)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`DELETE FROM system_config WHERE key IN ($1, $2)`,
			affiliateDailyLastRunKey, affiliateLevelsLastQuarterKey)
	})
}

func readAffiliateJobConfig(t *testing.T, repo *repository.Repository, key string) string {
	t.Helper()
	var value *string
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value))
	if value == nil {
		return ""
	}
	return *value
}

func seedAffiliateJobLevelThresholds(t *testing.T, repo *repository.Repository, level string, rise, hold float64) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, levels)
		 VALUES (1, jsonb_build_object($1::text, jsonb_build_object('rise_threshold', $2::numeric, 'hold_threshold', $3::numeric)))
		 ON CONFLICT (id) DO UPDATE SET levels = affiliate_settings.levels
		     || jsonb_build_object($1::text, jsonb_build_object('rise_threshold', $2::numeric, 'hold_threshold', $3::numeric))`,
		level, rise, hold,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`UPDATE affiliate_settings SET levels = levels - $1::text WHERE id = 1`, level)
	})
}

type affiliateJobCommissionSeed struct {
	productType string
	base        float64
	pct         float64
	amount      float64
	status      string
	level       string
	createdAt   time.Time
	paidAt      time.Time
	waitUntil   *time.Time
	completedAt *time.Time
}

func seedAffiliateJobCommission(t *testing.T, repo *repository.Repository, affiliatorID, studentID uuid.UUID, seed affiliateJobCommissionSeed) model.AffiliateCommission {
	t.Helper()
	ctx := context.Background()
	var productID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ($1, $2, 100000, NULL, 'published') RETURNING id`,
		seed.productType, "Job "+seed.productType+" "+uniqueSuffix(),
	).Scan(&productID))
	var orderID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, paid_at, completed_at)
		 VALUES ($1, 'processing', $2, 0, 0, $2, $3, $4) RETURNING id`,
		studentID, seed.base, seed.paidAt, seed.completedAt,
	).Scan(&orderID))
	var itemID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
		 VALUES ($1, $2, $3, $4, $5, 1, $5) RETURNING id`,
		orderID, productID, seed.productType, "Job item "+uniqueSuffix(), seed.base,
	).Scan(&itemID))
	var c model.AffiliateCommission
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, student_id, order_id, order_item_id, product_type,
		     quantity, base_amount, detail, amount, status, release_at, order_paid_at, created_at, effective_at)
		 VALUES ($1, 'commission', $2, $3, $4, $5, 1, $6,
		     jsonb_build_object('rule', jsonb_build_object('type', 'percent', 'value', $7::numeric), 'level', $8::text),
		     $9, $10, $11, $12, $13, CASE WHEN $10::text = 'available' THEN $13::timestamptz END)
		 RETURNING `+testCommissionSelect,
		affiliatorID, studentID, orderID, itemID, seed.productType, seed.base, seed.pct, seed.level,
		seed.amount, seed.status, seed.waitUntil, seed.paidAt, seed.createdAt,
	).Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType, &c.BaseAmount,
		&c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt, &c.CreatedAt))
	return c
}

func setAffiliateJobAffiliator(t *testing.T, repo *repository.Repository, id uuid.UUID, level string, warn int, createdAt time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := repo.Pool().Exec(ctx, `UPDATE affiliators SET level = $2, missed_quarters = $3 WHERE user_id = $1`, id, level, warn)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx, `UPDATE users SET created_at = $2 WHERE id = $1`, id, createdAt)
	require.NoError(t, err)
}

func getAffiliateJobAffiliator(t *testing.T, repo *repository.Repository, id uuid.UUID) *model.Affiliator {
	t.Helper()
	a, err := repo.GetAffiliatorByID(context.Background(), nil, id)
	require.NoError(t, err)
	require.NotNil(t, a, "affiliator %s must exist", id)
	return a
}

const testCommissionSelect = `id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount,
		detail->'rule'->>'type', (detail->'rule'->>'value')::numeric, detail->>'level', amount, status, release_at, order_paid_at, created_at`

func getAffiliateJobCommission(t *testing.T, repo *repository.Repository, id uuid.UUID) *model.AffiliateCommission {
	t.Helper()
	var c model.AffiliateCommission
	err := repo.Pool().QueryRow(context.Background(),
		`SELECT `+testCommissionSelect+` FROM affiliate_transactions WHERE type = 'commission' AND id = $1`, id,
	).Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType, &c.BaseAmount,
		&c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt, &c.CreatedAt)
	if err != nil {
		t.Fatalf("get commission: %v", err)
	}
	return &c
}

func affiliateJobLedgerRows(t *testing.T, repo *repository.Repository, commissionID uuid.UUID) []model.AffiliateLedgerEntry {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT id, affiliator_id, type, amount, CASE type WHEN 'commission' THEN id ELSE parent_id END, note, actor_id, effective_at
		 FROM affiliate_transactions
		 WHERE effective_at IS NOT NULL AND (id = $1 OR parent_id = $1) ORDER BY effective_at`, commissionID)
	require.NoError(t, err)
	defer rows.Close()

	var out []model.AffiliateLedgerEntry
	for rows.Next() {
		var e model.AffiliateLedgerEntry
		require.NoError(t, rows.Scan(&e.ID, &e.AffiliatorID, &e.EntryType, &e.Amount, &e.CommissionID, &e.Note, &e.ActorID, &e.CreatedAt))
		out = append(out, e)
	}
	require.NoError(t, rows.Err())
	return out
}

// affiliateJobEvaluatedQuarter is the quarter the level job last settled for an
// affiliator, or "" when it never did.
func affiliateJobEvaluatedQuarter(t *testing.T, repo *repository.Repository, id uuid.UUID) string {
	t.Helper()
	if q := getAffiliateJobAffiliator(t, repo, id).LastEvaluatedQuarter; q != nil {
		return *q
	}
	return ""
}

func seedAffiliateJobBinding(t *testing.T, repo *repository.Repository, affiliatorID, studentID uuid.UUID) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, expires_at)
		 VALUES ($1, $2, 'active', 'manual_code', now() - interval '30 days', now() + interval '60 days')`,
		affiliatorID, studentID,
	)
	require.NoError(t, err)
}

func TestAffiliateJobDailyDayKey_runsOncePerWIBDay(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	clearAffiliateJobKeys(t, repo)
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8, physicalWait: 14})

	owner := seedAffiliatorOwner(t, repo, "aff-job-day-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-job-day-"+uniqueSuffix()+"@test.local", "")

	now1 := time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC) // 10:00 WIB, before eval day 8
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "manual_code")
	_, err := repo.Pool().Exec(ctx,
		`UPDATE affiliate_bindings SET expires_at = $2 WHERE student_id = $1 AND status = 'active'`,
		student.ID, now1.Add(-time.Hour))
	require.NoError(t, err)

	// Drain stale due rows from sibling tests so day one settles exactly ours.
	_, err = svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)

	dueA := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: "pending", level: "bronze",
		createdAt: now1.AddDate(0, 0, -10), paidAt: now1.AddDate(0, 0, -10), waitUntil: timePtr(now1.Add(-time.Hour)),
	})

	require.NoError(t, svc.RunAffiliateDaily(ctx, now1))
	require.Equal(t, "2026-10-05", readAffiliateJobConfig(t, repo, affiliateDailyLastRunKey))
	require.Nil(t, getActiveAffiliateBinding(t, repo, student.ID), "due binding must be ended by the daily pass")

	commission := getAffiliateJobCommission(t, repo, dueA.ID)
	require.Equal(t, "available", commission.Status)
	require.Len(t, affiliateJobLedgerRows(t, repo, dueA.ID), 1)

	// Same WIB day: the day-key guard must keep the new due row untouched.
	dueB := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: "pending", level: "bronze",
		createdAt: now1.AddDate(0, 0, -10), paidAt: now1.AddDate(0, 0, -10), waitUntil: timePtr(now1.Add(-time.Hour)),
	})
	require.NoError(t, svc.RunAffiliateDaily(ctx, now1))
	require.Equal(t, "pending", getAffiliateJobCommission(t, repo, dueB.ID).Status)
	require.Empty(t, affiliateJobLedgerRows(t, repo, dueB.ID))

	// Next WIB day runs again and settles it.
	now2 := now1.AddDate(0, 0, 1)
	require.NoError(t, svc.RunAffiliateDaily(ctx, now2))
	require.Equal(t, "2026-10-06", readAffiliateJobConfig(t, repo, affiliateDailyLastRunKey))
	require.Equal(t, "available", getAffiliateJobCommission(t, repo, dueB.ID).Status)
	require.Len(t, affiliateJobLedgerRows(t, repo, dueB.ID), 1)
}

func TestAffiliateJobCommissionRelease_transitionsLedgerAndIdempotent(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8})

	owner := seedAffiliatorOwner(t, repo, "aff-job-rel-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-job-rel-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "manual_code")

	// Drain stale due rows so the release counts below are exact.
	_, err := svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)

	paidAt := time.Now().UTC().AddDate(0, 0, -8)
	order := seedCommissionOrder(t, repo, student.ID, paidAt, 0,
		commissionSeedItem{productType: "exam", jumlah: 100000})
	require.NoError(t, runCreateCommissions(t, svc, repo, order))
	created := affiliateCommissionsForOrder(t, repo, order.ID)
	require.Len(t, created, 1)
	require.Equal(t, "pending", created[0].Status)

	res, err := svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Equal(t, 1, res.CommissionsReleased)

	released := affiliateCommissionsForOrder(t, repo, order.ID)
	require.Len(t, released, 1)
	require.Equal(t, "available", released[0].Status)

	entries := affiliateJobLedgerRows(t, repo, released[0].ID)
	require.Len(t, entries, 1)
	require.Equal(t, "commission", entries[0].EntryType)
	require.Equal(t, 10000.0, entries[0].Amount)
	require.NotNil(t, entries[0].CommissionID)
	require.Equal(t, released[0].ID, *entries[0].CommissionID)
	require.Nil(t, entries[0].ActorID, "job entries carry no actor")
	require.NotNil(t, entries[0].Note)
	require.Contains(t, *entries[0].Note, order.ID.String())

	res, err = svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Equal(t, 0, res.CommissionsReleased, "rerun must not release again")
	require.Len(t, affiliateJobLedgerRows(t, repo, released[0].ID), 1)

	t.Run("not yet due stays pending", func(t *testing.T) {
		fresh := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
			commissionSeedItem{productType: "exam", jumlah: 50000})
		require.NoError(t, runCreateCommissions(t, svc, repo, fresh))

		res, err := svc.RunAffiliateJob(ctx, "commissions", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.CommissionsReleased)
		require.Equal(t, "pending", affiliateCommissionsForOrder(t, repo, fresh.ID)[0].Status)
	})
}

func TestAffiliateJobPhysicalBackfill_setsWaitUntilFromCompletedAt(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8, physicalWait: 5})
	restoreAffiliateJobSettings(t, repo)

	// Drain stale due rows so the counts below are exact.
	_, err := svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)

	owner := seedAffiliatorOwner(t, repo, "aff-job-phys-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-job-phys-"+uniqueSuffix()+"@test.local", "")

	completedAt := time.Now().UTC().AddDate(0, 0, -1)
	waiting := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "book", base: 100000, pct: 3, amount: 3000, status: "pending", level: "bronze",
		createdAt: completedAt.AddDate(0, 0, -2), paidAt: completedAt.AddDate(0, 0, -2), completedAt: &completedAt,
	})
	notCompleted := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "book", base: 100000, pct: 3, amount: 3000, status: "pending", level: "bronze",
		createdAt: completedAt.AddDate(0, 0, -2), paidAt: completedAt.AddDate(0, 0, -2),
	})
	longAgo := time.Now().UTC().AddDate(0, 0, -30)
	dueAfterBackfill := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "merchandise", base: 100000, pct: 3, amount: 3000, status: "pending", level: "bronze",
		createdAt: longAgo, paidAt: longAgo, completedAt: &longAgo,
	})

	res, err := svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Equal(t, 2, res.PhysicalBackfilled)
	require.Equal(t, 1, res.CommissionsReleased, "a backfilled wait already in the past releases in the same pass")

	want := completedAt.AddDate(0, 0, 5) // physical_wait_days 5
	backfilled := getAffiliateJobCommission(t, repo, waiting.ID)
	require.NotNil(t, backfilled.WaitUntil)
	require.WithinDuration(t, want, *backfilled.WaitUntil, time.Second)
	require.Equal(t, "pending", backfilled.Status, "not yet due after backfill")

	require.Nil(t, getAffiliateJobCommission(t, repo, notCompleted.ID).WaitUntil,
		"physical commission without completed_at stays NULL")
	require.Equal(t, "pending", getAffiliateJobCommission(t, repo, notCompleted.ID).Status)

	require.Equal(t, "available", getAffiliateJobCommission(t, repo, dueAfterBackfill.ID).Status)
	require.Len(t, affiliateJobLedgerRows(t, repo, dueAfterBackfill.ID), 1)

	res, err = svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Equal(t, 0, res.PhysicalBackfilled, "rerun must not backfill again")
}
