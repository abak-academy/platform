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
		`INSERT INTO affiliate_level_settings (level, rise_threshold, hold_threshold)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (level) DO UPDATE SET rise_threshold = $2, hold_threshold = $3`,
		level, rise, hold,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`DELETE FROM affiliate_level_settings WHERE level = $1`, level)
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
		`INSERT INTO affiliate_commissions (affiliator_id, student_id, order_id, order_item_id, product_type,
		     base_amount, commission_value, level_snapshot, amount, status, wait_until, paid_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount,
		           commission_type, commission_value, level_snapshot, amount, status, wait_until, paid_at, created_at`,
		affiliatorID, studentID, orderID, itemID, seed.productType, seed.base, seed.pct, seed.level,
		seed.amount, seed.status, seed.waitUntil, seed.paidAt, seed.createdAt,
	).Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType, &c.BaseAmount,
		&c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt, &c.CreatedAt))
	return c
}

func setAffiliateJobAffiliator(t *testing.T, repo *repository.Repository, id uuid.UUID, level string, warn int, createdAt time.Time, lockedUntil *time.Time) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`UPDATE affiliators SET level = $2, warn_quarters = $3, created_at = $4, level_locked_until = $5, updated_at = now()
		 WHERE id = $1`,
		id, level, warn, createdAt, lockedUntil,
	)
	require.NoError(t, err)
}

func getAffiliateJobAffiliator(t *testing.T, repo *repository.Repository, id uuid.UUID) *model.Affiliator {
	t.Helper()
	var a model.Affiliator
	err := repo.Pool().QueryRow(context.Background(),
		`SELECT id, user_id, code, status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators WHERE id = $1`, id,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level, &a.LevelLockedUntil,
		&a.WarnQuarters, &a.SuspendedUntil, &a.FrozenUntil, &a.TermsVersion, &a.TermsAcceptedAt,
		&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		t.Fatalf("get affiliator: %v", err)
	}
	return &a
}

func getAffiliateJobCommission(t *testing.T, repo *repository.Repository, id uuid.UUID) *model.AffiliateCommission {
	t.Helper()
	var c model.AffiliateCommission
	err := repo.Pool().QueryRow(context.Background(),
		`SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount,
		        commission_type, commission_value, level_snapshot, amount, status, wait_until, paid_at, created_at
		 FROM affiliate_commissions WHERE id = $1`, id,
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
		`SELECT id, affiliator_id, entry_type, amount, commission_id, note, actor_id, created_at
		 FROM affiliate_ledger_entries WHERE commission_id = $1 ORDER BY created_at`, commissionID)
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

func affiliateJobHistoryRows(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) []model.AffiliateLevelHistory {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT id, affiliator_id, quarter, from_level, to_level, net_revenue, source, reason, created_by, created_at
		 FROM affiliate_level_history WHERE affiliator_id = $1 ORDER BY created_at`, affiliatorID)
	require.NoError(t, err)
	defer rows.Close()

	var out []model.AffiliateLevelHistory
	for rows.Next() {
		var h model.AffiliateLevelHistory
		require.NoError(t, rows.Scan(&h.ID, &h.AffiliatorID, &h.Quarter, &h.FromLevel, &h.ToLevel, &h.NetRevenue, &h.Source, &h.Reason, &h.CreatedBy, &h.CreatedAt))
		out = append(out, h)
	}
	require.NoError(t, rows.Err())
	return out
}

func seedAffiliateJobBinding(t *testing.T, repo *repository.Repository, affiliatorID, studentID uuid.UUID, lastExtendedAt *time.Time) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, ended_at, last_extended_at)
		 VALUES ($1, $2, 'active', 'manual_code', now() - interval '30 days', now() + interval '60 days', $3)`,
		affiliatorID, studentID, lastExtendedAt,
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
		`UPDATE affiliate_bindings SET ended_at = $2 WHERE student_id = $1 AND status = 'active'`,
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

	t.Run("existing ledger entry is treated as already done", func(t *testing.T) {
		stuck := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
			productType: "exam", base: 100000, pct: 10, amount: 10000, status: "pending", level: "bronze",
			createdAt: time.Now().UTC().AddDate(0, 0, -10), paidAt: time.Now().UTC().AddDate(0, 0, -10),
			waitUntil: timePtr(time.Now().UTC().Add(-time.Hour)),
		})
		note := "seeded entry"
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id, note)
			 VALUES ($1, 'commission', $2, $3, $4)`,
			owner.affiliatorID, stuck.Amount, stuck.ID, note)
		require.NoError(t, err)

		res, err := svc.RunAffiliateJob(ctx, "commissions", nil)
		require.NoError(t, err)
		require.Equal(t, "pending", getAffiliateJobCommission(t, repo, stuck.ID).Status,
			"ledger conflict means already released: no second transition")
		require.Len(t, affiliateJobLedgerRows(t, repo, stuck.ID), 1)
		_ = res
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

func TestAffiliateJobSanctionExpiry(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	clearAffiliateJobKeys(t, repo)
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8})

	owner := seedAffiliatorOwner(t, repo, "aff-job-sanc-"+uniqueSuffix()+"@test.local", "", "active")
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC) // unique WIB day, before eval day

	_, err := repo.Pool().Exec(ctx,
		`UPDATE affiliators SET status = 'suspended', suspended_until = $2, frozen_until = $3 WHERE id = $1`,
		owner.affiliatorID, now.Add(-time.Hour), now.Add(-2*time.Hour))
	require.NoError(t, err)
	permanent := seedAffiliatorOwner(t, repo, "aff-job-sanc-p-"+uniqueSuffix()+"@test.local", "", "active")
	_, err = repo.Pool().Exec(ctx,
		`UPDATE affiliators SET status = 'suspended', suspended_until = NULL WHERE id = $1`, permanent.affiliatorID)
	require.NoError(t, err)
	stillSuspended := seedAffiliatorOwner(t, repo, "aff-job-sanc-f-"+uniqueSuffix()+"@test.local", "", "active")
	_, err = repo.Pool().Exec(ctx,
		`UPDATE affiliators SET status = 'suspended', suspended_until = $2 WHERE id = $1`,
		stillSuspended.affiliatorID, now.Add(24*time.Hour))
	require.NoError(t, err)
	frozenOnly := seedAffiliatorOwner(t, repo, "aff-job-sanc-z-"+uniqueSuffix()+"@test.local", "", "active")
	_, err = repo.Pool().Exec(ctx,
		`UPDATE affiliators SET frozen_until = $2 WHERE id = $1`, frozenOnly.affiliatorID, now.Add(-time.Hour))
	require.NoError(t, err)

	require.NoError(t, svc.RunAffiliateDaily(ctx, now))

	reinstated := getAffiliateJobAffiliator(t, repo, owner.affiliatorID)
	require.Equal(t, "active", reinstated.Status)
	require.Nil(t, reinstated.SuspendedUntil)
	require.Nil(t, reinstated.FrozenUntil)

	require.Equal(t, "suspended", getAffiliateJobAffiliator(t, repo, permanent.affiliatorID).Status,
		"permanent suspension without suspended_until stays")
	require.Equal(t, "suspended", getAffiliateJobAffiliator(t, repo, stillSuspended.affiliatorID).Status,
		"future suspended_until stays")
	require.Nil(t, getAffiliateJobAffiliator(t, repo, frozenOnly.affiliatorID).FrozenUntil,
		"expired freeze clears regardless of status")

	require.NoError(t, svc.RunAffiliateDaily(ctx, now.Add(time.Hour)))
	require.Equal(t, "active", getAffiliateJobAffiliator(t, repo, owner.affiliatorID).Status,
		"rerun stays idempotent")
}

func TestAffiliateJobReleasesScope_isRerunnable(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")

	due := seedAffiliatorOwner(t, repo, "aff-job-rel-"+uniqueSuffix()+"@test.local", "", "active")
	past := time.Now().UTC().Add(-time.Hour)
	_, err := repo.Pool().Exec(ctx,
		`UPDATE affiliators SET status = 'suspended', suspended_until = $2, frozen_until = $2 WHERE id = $1`,
		due.affiliatorID, past)
	require.NoError(t, err)

	first, err := svc.RunAffiliateJob(ctx, "releases", nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, first.SanctionsExpired, 2, "the suspension and the freeze both lapse")
	released := getAffiliateJobAffiliator(t, repo, due.affiliatorID)
	require.Equal(t, "active", released.Status)
	require.Nil(t, released.FrozenUntil)

	second, err := svc.RunAffiliateJob(ctx, "releases", nil)
	require.NoError(t, err)
	require.Equal(t, 0, second.SanctionsExpired, "a second run changes nothing")
	require.Equal(t, "active", getAffiliateJobAffiliator(t, repo, due.affiliatorID).Status)
}
