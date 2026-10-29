package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Level evaluation tests seed the 2026-Q3 window (Jul 1 – Oct 1 WIB) with
// silver hold 3M / gold hold 9M and rises at 5M/15M, then run the forced
// quarter so the outcome does not depend on the real calendar.
const affiliateLevelTestQuarter = "2026-Q3"

func quarterPtr(s string) *string { return &s }

func seedAffiliateLevelScenario(t *testing.T, repo *repository.Repository, name, level string, warn int, createdAt time.Time, status string) uuid.UUID {
	t.Helper()
	owner := seedAffiliatorOwner(t, repo, "aff-lvl-"+name+"-"+uniqueSuffix()+"@test.local", "", status)
	setAffiliateJobAffiliator(t, repo, owner.affiliatorID, level, warn, createdAt)
	return owner.affiliatorID
}

func seedAffiliateLevelCommission(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, base float64, status string, paidAt time.Time) {
	t.Helper()
	student := seedAffiliateTestStudent(t, repo, "student-lvl-"+uniqueSuffix()+"@test.local", "")
	seedAffiliateJobCommission(t, repo, affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: base, pct: 10, amount: base / 10, status: status, level: "bronze",
		createdAt: paidAt, paidAt: paidAt,
	})
}

func TestAffiliateLevelEvaluation_matrix(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8})
	seedAffiliateJobLevelThresholds(t, repo, "silver", 5000000, 3000000)
	seedAffiliateJobLevelThresholds(t, repo, "gold", 15000000, 9000000)

	createdBeforeQuarter := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	createdMidQuarter := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// Bronze 17M in-quarter (one inside the WIB quarter from a UTC Jun 30
	// evening) clears both rises: jump to gold.
	jump := seedAffiliateLevelScenario(t, repo, "jump", "bronze", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, jump, 15000000, "available", time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, jump, 1000000, "available", time.Date(2026, 6, 30, 20, 0, 0, 0, time.UTC))

	rise := seedAffiliateLevelScenario(t, repo, "rise", "bronze", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, rise, 6000000, "available", time.Date(2026, 8, 20, 3, 0, 0, 0, time.UTC))

	// Silver 5M (paid + available) clears the 3M hold: stays silver, resets a
	// carried warning; held commission is excluded.
	hold := seedAffiliateLevelScenario(t, repo, "hold", "silver", 1, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, hold, 4000000, "available", time.Date(2026, 7, 2, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, hold, 1000000, "available", time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, hold, 9000000, "pending", time.Date(2026, 9, 3, 3, 0, 0, 0, time.UTC))

	// Silver 2M below the 3M hold: first miss warns via the counter + marker row.
	warn := seedAffiliateLevelScenario(t, repo, "warn", "silver", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, warn, 1000000, "available", time.Date(2026, 7, 10, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, warn, 1000000, "available", time.Date(2026, 8, 10, 3, 0, 0, 0, time.UTC))

	// Silver second miss (carried warning) drops; the 100M cancelled refund and
	// the pending 50M do not count toward net.
	drop := seedAffiliateLevelScenario(t, repo, "drop", "silver", 1, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, drop, 1500000, "available", time.Date(2026, 7, 5, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, drop, 100000000, "cancelled", time.Date(2026, 8, 5, 3, 0, 0, 0, time.UTC))
	seedAffiliateLevelCommission(t, repo, drop, 50000000, "pending", time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC))

	// Commissions paid before the quarter do not count: net 0 warns with 0.
	outside := seedAffiliateLevelScenario(t, repo, "outside", "silver", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, outside, 50000000, "available", time.Date(2026, 6, 28, 3, 0, 0, 0, time.UTC))

	// BR-LVL-06: joined inside the evaluated quarter — never evaluated for it.
	newcomer := seedAffiliateLevelScenario(t, repo, "newcomer", "bronze", 0, createdMidQuarter, "active")
	seedAffiliateLevelCommission(t, repo, newcomer, 20000000, "available", time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC))

	// Not fetched by the evaluation at all.
	deactivated := seedAffiliateLevelScenario(t, repo, "deactivated", "bronze", 0, createdBeforeQuarter, "deactivated")
	seedAffiliateLevelCommission(t, repo, deactivated, 20000000, "available", time.Date(2026, 7, 16, 3, 0, 0, 0, time.UTC))

	res, err := svc.RunAffiliateJob(ctx, "levels", quarterPtr(affiliateLevelTestQuarter))
	require.NoError(t, err)
	require.Equal(t, affiliateLevelTestQuarter, res.Quarter)
	require.Equal(t, 6, res.LevelsEvaluated, "newcomer and deactivated are not evaluated")
	require.Equal(t, 3, res.LevelsChanged, "jump, rise and drop")

	require.Equal(t, "gold", getAffiliateJobAffiliator(t, repo, jump).Level)
	jumpRows := affiliateJobHistoryRows(t, repo, jump)
	require.Len(t, jumpRows, 1)
	require.Equal(t, affiliateLevelTestQuarter, jumpRows[0].Quarter)
	require.NotNil(t, jumpRows[0].FromLevel)
	require.Equal(t, "bronze", *jumpRows[0].FromLevel)
	require.Equal(t, "gold", jumpRows[0].ToLevel)
	require.InDelta(t, 16000000, jumpRows[0].NetRevenue, 0)
	require.Equal(t, "auto", jumpRows[0].Source)
	require.Equal(t, "auto_eval", *jumpRows[0].Reason)
	require.Nil(t, jumpRows[0].CreatedBy)
	require.Equal(t, 0, getAffiliateJobAffiliator(t, repo, jump).WarnQuarters)

	require.Equal(t, "silver", getAffiliateJobAffiliator(t, repo, rise).Level)
	require.Len(t, affiliateJobHistoryRows(t, repo, rise), 1)

	holdAff := getAffiliateJobAffiliator(t, repo, hold)
	require.Equal(t, "silver", holdAff.Level)
	require.Equal(t, 0, holdAff.WarnQuarters, "recovered net resets the warning counter")
	require.Empty(t, affiliateJobHistoryRows(t, repo, hold), "hold writes no history row")

	warnAff := getAffiliateJobAffiliator(t, repo, warn)
	require.Equal(t, "silver", warnAff.Level)
	require.Equal(t, 1, warnAff.WarnQuarters)
	warnRows := affiliateJobHistoryRows(t, repo, warn)
	require.Len(t, warnRows, 1)
	require.Equal(t, "silver", warnRows[0].ToLevel)
	require.NotNil(t, warnRows[0].FromLevel)
	require.Equal(t, "silver", *warnRows[0].FromLevel, "warn marker row keeps the level")
	require.Equal(t, "auto_warn", *warnRows[0].Reason)
	require.InDelta(t, 2000000, warnRows[0].NetRevenue, 0)

	require.Equal(t, "bronze", getAffiliateJobAffiliator(t, repo, drop).Level)
	require.Equal(t, 0, getAffiliateJobAffiliator(t, repo, drop).WarnQuarters)
	dropRows := affiliateJobHistoryRows(t, repo, drop)
	require.Len(t, dropRows, 1)
	require.NotNil(t, dropRows[0].FromLevel)
	require.Equal(t, "silver", *dropRows[0].FromLevel)
	require.Equal(t, "bronze", dropRows[0].ToLevel)
	require.InDelta(t, 1500000, dropRows[0].NetRevenue, 0, "cancelled and held are excluded")

	outsideAff := getAffiliateJobAffiliator(t, repo, outside)
	require.Equal(t, "silver", outsideAff.Level)
	require.Equal(t, 1, outsideAff.WarnQuarters)
	outsideRows := affiliateJobHistoryRows(t, repo, outside)
	require.Len(t, outsideRows, 1)
	require.InDelta(t, 0, outsideRows[0].NetRevenue, 0, "pre-quarter paid_at does not count")

	require.Equal(t, "bronze", getAffiliateJobAffiliator(t, repo, newcomer).Level)
	require.Empty(t, affiliateJobHistoryRows(t, repo, newcomer), "join-quarter affiliator skipped")
	require.Equal(t, "bronze", getAffiliateJobAffiliator(t, repo, deactivated).Level)
}

func TestAffiliateLevelEvaluation_RerunIdempotent(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8})
	seedAffiliateJobLevelThresholds(t, repo, "silver", 5000000, 3000000)

	createdBeforeQuarter := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	jump := seedAffiliateLevelScenario(t, repo, "rerun-jump", "bronze", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, jump, 16000000, "available", time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC))
	warn := seedAffiliateLevelScenario(t, repo, "rerun-warn", "silver", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, warn, 2000000, "available", time.Date(2026, 7, 16, 3, 0, 0, 0, time.UTC))
	drop := seedAffiliateLevelScenario(t, repo, "rerun-drop", "silver", 1, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, drop, 1500000, "available", time.Date(2026, 7, 17, 3, 0, 0, 0, time.UTC))

	_, err := svc.RunAffiliateJob(ctx, "levels", quarterPtr(affiliateLevelTestQuarter))
	require.NoError(t, err)

	res, err := svc.RunAffiliateJob(ctx, "levels", quarterPtr(affiliateLevelTestQuarter))
	require.NoError(t, err)
	// LevelsEvaluated is not asserted on the rerun: the shared fixture keeps
	// hold-plan affiliators (no history row, no change) re-evaluable by design;
	// the per-affiliator assertions below pin the idempotence.
	require.Equal(t, 0, res.LevelsChanged, "rerun must not repeat a change")
	require.Len(t, affiliateJobHistoryRows(t, repo, jump), 1)
	require.Len(t, affiliateJobHistoryRows(t, repo, warn), 1)
	require.Len(t, affiliateJobHistoryRows(t, repo, drop), 1)
	require.Equal(t, 1, getAffiliateJobAffiliator(t, repo, warn).WarnQuarters, "warn counter not re-incremented")
	require.Equal(t, 0, getAffiliateJobAffiliator(t, repo, drop).WarnQuarters, "no double drop")

	t.Run("auto rerun respects the quarter guard", func(t *testing.T) {
		current, err := affiliatePrevQuarterKey(affiliateQuarterKey(time.Now().In(affiliateWIBLocation())))
		require.NoError(t, err)
		setAffiliateJobConfigKey(t, repo, affiliateLevelsLastQuarterKey, current)

		// A fresh quarter-skip case must not re-evaluate the guarded quarter
		// through the scope rerun either: manual changes stay untouched.
		manualLevel := "gold"
		_, err = repo.Pool().Exec(ctx,
			`UPDATE affiliators SET level = $2, updated_at = now() WHERE id = $1`, drop, manualLevel)
		require.NoError(t, err)

		res, err := svc.RunAffiliateJob(ctx, "levels", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.LevelsChanged, "guarded quarter is skipped")
		require.Equal(t, "gold", getAffiliateJobAffiliator(t, repo, drop).Level)
	})

	t.Run("invalid quarter is rejected", func(t *testing.T) {
		bad := "2026-Q9"
		_, err := svc.RunAffiliateJob(ctx, "levels", &bad)
		require.ErrorIs(t, err, ErrAffiliateJobInvalidQuarter)
	})
}

func TestRunAffiliateJob_scopesAndGates(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	_, err := svc.RunAffiliateJob(ctx, "nonsense", nil)
	require.ErrorIs(t, err, ErrAffiliateJobUnknownScope)

	// Flag off: every valid scope is a no-op.
	res, err := svc.RunAffiliateJob(ctx, "bindings", nil)
	require.NoError(t, err)
	require.Equal(t, "bindings", res.Scope)
	require.Equal(t, 0, res.BindingsExpired)

	res, err = svc.RunAffiliateJob(ctx, "levels", nil)
	require.NoError(t, err)
	require.Equal(t, 0, res.LevelsChanged)
}

func TestAffiliateJobQuarterlyWiring_runsFromDailyLoop(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	clearAffiliateJobKeys(t, repo)
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliateJobSettings(t, repo, affiliateJobSettingsSeed{evalDay: 8})
	seedAffiliateJobLevelThresholds(t, repo, "silver", 5000000, 3000000)

	createdBeforeQuarter := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	warn := seedAffiliateLevelScenario(t, repo, "wiring", "silver", 0, createdBeforeQuarter, "active")
	seedAffiliateLevelCommission(t, repo, warn, 2000000, "available", time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC))

	evalDay := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC) // 10:00 WIB on the eval day
	require.NoError(t, svc.RunAffiliateDaily(ctx, evalDay))

	require.Equal(t, "2026-Q3", readAffiliateJobConfig(t, repo, affiliateLevelsLastQuarterKey))
	require.Equal(t, "2026-10-08", readAffiliateJobConfig(t, repo, affiliateDailyLastRunKey))
	require.Equal(t, 1, getAffiliateJobAffiliator(t, repo, warn).WarnQuarters)
	require.Len(t, affiliateJobHistoryRows(t, repo, warn), 1)

	// Same tick pattern again: the quarter key blocks a re-evaluation.
	require.NoError(t, svc.RunAffiliateDaily(ctx, evalDay.Add(15*time.Minute)))
	require.Len(t, affiliateJobHistoryRows(t, repo, warn), 1)
}
