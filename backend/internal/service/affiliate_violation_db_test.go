package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func violationCount(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_violations WHERE affiliator_id = $1`, affiliatorID).Scan(&n))
	return n
}

func seedViolationCommission(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	student := seedAffiliateTestStudent(t, repo, "student-viol-"+uniqueSuffix()+"@test.local", "")
	c := seedAffiliateJobCommission(t, repo, affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: status, level: "bronze",
		createdAt: time.Now(), paidAt: time.Now(),
	})
	if status == "available" {
		seedCommissionLedgerEntry(t, repo, affiliatorID, c.ID, 10000)
	}
	return c.ID
}

func TestAdminRecordAffiliateViolation_AppliesEachSanction(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedRevokeActor(t, repo).String()
	days := 7
	newAffiliator := func(status string) uuid.UUID {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-viol-"+uniqueSuffix()+"@test.local", status, "silver")
		return id
	}
	base := AdminViolationInput{Category: "transaction_fraud", Description: "Belanja atas akun sendiri"}

	t.Run("warning only records the violation", func(t *testing.T) {
		id := newAffiliator("active")
		in := base
		in.Sanction = "warning"
		v, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		require.Equal(t, "open", v.Status)
		require.Equal(t, "none", v.AppealStatus)
		require.Equal(t, uuid.MustParse(actor), v.DecidedBy)
		require.Equal(t, 1, violationCount(t, repo, id))
		require.Equal(t, 1, countViolationAudit(t, repo, v.ID))
	})

	t.Run("suspend suspends and dates the violation", func(t *testing.T) {
		id := newAffiliator("active")
		in := base
		in.Sanction, in.Days = "suspend", &days
		v, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		require.NotNil(t, v.SuspensionUntil)
		require.WithinDuration(t, time.Now().AddDate(0, 0, 7), *v.SuspensionUntil, time.Minute)
		var status string
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT status FROM affiliators WHERE id = $1`, id).Scan(&status))
		require.Equal(t, "suspended", status)
	})

	t.Run("deactivate deactivates and cancels pending commissions", func(t *testing.T) {
		id := newAffiliator("active")
		pending := seedViolationCommission(t, repo, id, "pending")
		in := base
		in.Sanction = "deactivate"
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		require.Equal(t, "cancelled", commissionStatusByID(t, repo, pending))
	})

	t.Run("balance_freeze freezes within the configured maximum", func(t *testing.T) {
		id := newAffiliator("active")
		in := base
		in.Sanction, in.Days = "balance_freeze", &days
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		var frozen *time.Time
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT frozen_until FROM affiliators WHERE id = $1`, id).Scan(&frozen))
		require.NotNil(t, frozen)
		require.True(t, frozen.After(time.Now()))
	})

	t.Run("level_change moves the level and writes manual history", func(t *testing.T) {
		id := newAffiliator("active")
		in := base
		in.Sanction, in.Level = "level_change", "bronze"
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		var level string
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT level FROM affiliators WHERE id = $1`, id).Scan(&level))
		require.Equal(t, "bronze", level)
		var n int
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT count(*) FROM affiliate_level_history WHERE affiliator_id = $1 AND source = 'manual'`, id).Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("commission_cancel cancels pending and takes back an available credit", func(t *testing.T) {
		id := newAffiliator("active")
		pending := seedViolationCommission(t, repo, id, "pending")
		available := seedViolationCommission(t, repo, id, "available")
		in := base
		in.Sanction, in.CommissionIDs = "commission_cancel", []string{pending.String(), available.String()}
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
		require.NoError(t, err)
		require.Equal(t, "cancelled", commissionStatusByID(t, repo, pending))
		require.Equal(t, "cancelled", commissionStatusByID(t, repo, available))
		require.Equal(t, 0.0, affiliateLedgerSaldo(t, repo, id), "the +10000 credit is reversed by a violation adjustment")
	})
}

func countViolationAudit(t *testing.T, repo *repository.Repository, violationID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliate_violation' AND target_id = $1 AND action = 'affiliate.violation_record'`,
		violationID.String()).Scan(&n))
	return n
}

func TestAdminRecordAffiliateViolation_RefusesBadInputWithoutLeavingARow(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedRevokeActor(t, repo).String()
	days := 5
	id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-violbad-"+uniqueSuffix()+"@test.local", "active", "bronze")
	otherID, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-violother-"+uniqueSuffix()+"@test.local", "active", "bronze")
	foreign := seedViolationCommission(t, repo, otherID, "pending")
	cancelled := seedViolationCommission(t, repo, id, "cancelled")
	good := AdminViolationInput{Category: "ethics", Description: "Promosi menyesatkan", Sanction: "warning"}

	cases := []struct {
		name string
		mut  func(*AdminViolationInput)
		want error
	}{
		{"unknown category", func(in *AdminViolationInput) { in.Category = "bogus" }, ErrAffiliateViolationInvalid},
		{"unknown sanction", func(in *AdminViolationInput) { in.Sanction = "bogus" }, ErrAffiliateViolationInvalid},
		{"empty description", func(in *AdminViolationInput) { in.Description = "  " }, ErrAffiliateViolationInvalid},
		{"suspend without days", func(in *AdminViolationInput) { in.Sanction = "suspend" }, ErrAffiliateInvalidDays},
		{"freeze without days", func(in *AdminViolationInput) { in.Sanction = "balance_freeze" }, ErrAffiliateInvalidDays},
		{"level change without a level or lock", func(in *AdminViolationInput) { in.Sanction = "level_change" }, ErrMissingField},
		{"commission cancel without ids", func(in *AdminViolationInput) { in.Sanction = "commission_cancel" }, ErrAffiliateViolationInvalid},
		{"commission of another affiliator", func(in *AdminViolationInput) {
			in.Sanction, in.CommissionIDs = "commission_cancel", []string{foreign.String()}
		}, ErrAffiliateViolationInvalid},
		{"commission that is already cancelled", func(in *AdminViolationInput) {
			in.Sanction, in.CommissionIDs = "commission_cancel", []string{cancelled.String()}
		}, ErrAffiliateViolationInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := good
			tc.mut(&in)
			_, err := svc.AdminRecordAffiliateViolation(ctx, actor, id.String(), in)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, 0, violationCount(t, repo, id), "a refused sanction must not leave a violation behind")
		})
	}
	require.Equal(t, "pending", commissionStatusByID(t, repo, foreign), "another affiliator's commission is untouched")

	t.Run("a pending profile cannot be suspended", func(t *testing.T) {
		pendingID, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-violpend-"+uniqueSuffix()+"@test.local", "pending_approval", "bronze")
		in := good
		in.Sanction, in.Days = "suspend", &days
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, pendingID.String(), in)
		require.ErrorIs(t, err, ErrAffiliateInvalidTransition)
		require.Equal(t, 0, violationCount(t, repo, pendingID))
	})

	t.Run("an unknown affiliator is not found", func(t *testing.T) {
		_, err := svc.AdminRecordAffiliateViolation(ctx, actor, uuid.NewString(), good)
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

// The appeal rule only works once violations can be recorded: a freshly
// recorded one is open and appealable, and deciding the appeal resolves it.
func TestRecordedViolationCanBeAppealedAndDecided(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	actor := seedRevokeActor(t, repo).String()
	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Loop "+uniqueSuffix(), "active", "bronze")

	v, err := svc.AdminRecordAffiliateViolation(ctx, actor, aff.ID.String(), AdminViolationInput{
		Category: "transaction_fraud", Description: "Pembelian atas akun sendiri", Sanction: "warning",
	})
	require.NoError(t, err)

	appealed, err := svc.AppealMyAffiliateViolation(ctx, userID, v.ID.String(), "Itu pembelian keluarga")
	require.NoError(t, err)
	require.Equal(t, "pending", appealed.AppealStatus)

	decided, err := svc.AdminDecideAffiliateAppeal(ctx, actor, v.ID.String(), AffiliateAdminAppealDecisionInput{Outcome: "rejected"})
	require.NoError(t, err)
	require.Equal(t, "resolved", decided.Status)
	require.Equal(t, "rejected", decided.AppealStatus)

	_, err = svc.AppealMyAffiliateViolation(ctx, userID, v.ID.String(), "banding lagi")
	require.ErrorIs(t, err, ErrAffiliateNotAppealable)
}
