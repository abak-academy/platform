package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The payout/violation tests share the real-DB fixture and never reset it:
// each test seeds its own affiliator, level min_payout row and rows, and the
// singleton settings fields reset themselves on cleanup.
func seedAffiliateMeBank(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`UPDATE affiliators SET bank_account = '{"bank_name":"BCA","account_number":"1234567890","account_holder":"Budi Santoso"}' WHERE user_id = $1`,
		affiliatorID)
	require.NoError(t, err)
}

func seedAffiliateMeMinPayout(t *testing.T, repo *repository.Repository, level string, minPayout float64) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, levels) VALUES (1, jsonb_build_object($1::text, jsonb_build_object('min_payout', $2::numeric)))
		 ON CONFLICT (id) DO UPDATE SET levels = affiliate_settings.levels
		     || jsonb_build_object($1::text, jsonb_build_object('min_payout', $2::numeric))`,
		level, minPayout)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `UPDATE affiliate_settings SET levels = levels - $1::text WHERE id = 1`, level)
	})
}

func seedAffiliateMePayoutDay(t *testing.T, repo *repository.Repository, day int) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, payout_day_of_month) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET payout_day_of_month = $1, updated_at = now()`, day)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `UPDATE affiliate_settings SET payout_day_of_month = NULL WHERE id = 1`)
	})
}

func seedAffiliateMePayoutRow(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, status string, amount float64, requestedAt time.Time) uuid.UUID {
	t.Helper()
	return seedAffiliateTxPayout(t, repo, affiliatorID, status, amount, "Budi Santoso", requestedAt)
}

func countAffiliateMePayouts(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_transactions WHERE type = 'payout' AND affiliator_id = $1`, affiliatorID).Scan(&n))
	return n
}

func TestRequestMyAffiliatePayout_rejectsBelowMinPayout(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Below Min "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 90000, time.Now().UTC())

	_, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.ErrorIs(t, err, ErrAffiliatePayoutBelowMin)
	require.Equal(t, 0, countAffiliateMePayouts(t, repo, aff.ID))
	require.Equal(t, 0, countAffiliateLedgerByTypeFor(t, repo, aff.ID, "payout"))
}

func TestRequestMyAffiliatePayout_rejectsMissingBank(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator No Bank "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 200000, time.Now().UTC())

	_, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.ErrorIs(t, err, ErrAffiliateBankMissing)
	require.Equal(t, 0, countAffiliateMePayouts(t, repo, aff.ID))
}

func TestRequestMyAffiliatePayout_rejectsWhileInFlight(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Inflight "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 200000, time.Now().UTC())
	seedAffiliateMeBank(t, repo, aff.ID)
	seedAffiliateMePayoutRow(t, repo, aff.ID, "requested", 50000, time.Now().UTC().Add(-time.Hour))

	_, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.ErrorIs(t, err, ErrAffiliatePayoutInFlight)
	require.Equal(t, 1, countAffiliateMePayouts(t, repo, aff.ID))
}

func TestRequestMyAffiliatePayout_rejectsProcessingInFlight(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Processing "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 200000, time.Now().UTC())
	seedAffiliateMeBank(t, repo, aff.ID)
	seedAffiliateMePayoutRow(t, repo, aff.ID, "processing", 50000, time.Now().UTC().Add(-2*time.Hour))

	_, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.ErrorIs(t, err, ErrAffiliatePayoutInFlight)
}

func TestRequestMyAffiliatePayout_rejectedWhenNotActive(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Susp Payout "+uniqueSuffix(), "suspended", "bronze")

	_, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.ErrorIs(t, err, ErrAffiliateNotActive)
	require.Equal(t, 0, countAffiliateMePayouts(t, repo, aff.ID))
}

func TestRequestMyAffiliatePayout_fullSaldoDefaultWritesDeduction(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Payout "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 100000, time.Now().UTC().Add(-48*time.Hour))
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 50000, time.Now().UTC())
	seedAffiliateMeBank(t, repo, aff.ID)

	payout, err := svc.RequestMyAffiliatePayout(ctx, userID, nil)
	require.NoError(t, err)
	require.Equal(t, aff.ID, payout.AffiliatorID)
	require.InDelta(t, 150000, payout.Amount, 0.001)
	require.Equal(t, "requested", payout.Status)
	require.Equal(t, "BCA", payout.BankName)
	require.Equal(t, "1234567890", payout.BankAccountNumber)
	require.Equal(t, "Budi Santoso", payout.BankAccountHolder)
	require.False(t, payout.RequestedAt.IsZero())

	var ledgerAmount float64
	var payoutID, actorID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT amount, id, actor_id FROM affiliate_transactions
		 WHERE affiliator_id = $1 AND type = 'payout'`, aff.ID,
	).Scan(&ledgerAmount, &payoutID, &actorID))
	require.InDelta(t, -150000, ledgerAmount, 0.001)
	require.Equal(t, payout.ID, payoutID)
	require.Equal(t, aff.ID, actorID)

	saldo, err := repo.SumAffiliateLedgerForAffiliator(ctx, aff.ID)
	require.NoError(t, err)
	require.InDelta(t, 0, saldo, 0.001)

	require.Equal(t, 1, countAffiliateAudit(t, repo, "affiliate.payout_requested", payout.ID.String()))
	var auditActor string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT actor_id FROM audit_log WHERE action = 'affiliate.payout_requested' AND target_id = $1`,
		payout.ID.String(),
	).Scan(&auditActor))
	require.Equal(t, userID, auditActor)
}

func TestRequestMyAffiliatePayout_explicitAmountBounds(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Amount "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 150000, time.Now().UTC())
	seedAffiliateMeBank(t, repo, aff.ID)

	over := 200000.0
	_, err := svc.RequestMyAffiliatePayout(ctx, userID, &over)
	require.ErrorIs(t, err, ErrAffiliatePayoutAmountInvalid)

	under := 50000.0
	_, err = svc.RequestMyAffiliatePayout(ctx, userID, &under)
	require.ErrorIs(t, err, ErrAffiliatePayoutAmountInvalid)

	zero := 0.0
	_, err = svc.RequestMyAffiliatePayout(ctx, userID, &zero)
	require.ErrorIs(t, err, ErrAffiliatePayoutAmountInvalid)

	valid := 100000.0
	payout, err := svc.RequestMyAffiliatePayout(ctx, userID, &valid)
	require.NoError(t, err)
	require.InDelta(t, 100000, payout.Amount, 0.001)

	saldo, err := repo.SumAffiliateLedgerForAffiliator(ctx, aff.ID)
	require.NoError(t, err)
	require.InDelta(t, 50000, saldo, 0.001)
}

func TestListMyAffiliatePayouts_newestFirstWithSchedule(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateMeMinPayout(t, repo, "bronze", 100000)
	seedAffiliateMePayoutDay(t, repo, 15)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator PayList "+uniqueSuffix(), "active", "bronze")
	base := time.Now().UTC().AddDate(0, 0, -3)
	seedAffiliateMePayoutRow(t, repo, aff.ID, "requested", 100000, base)
	seedAffiliateMePayoutRow(t, repo, aff.ID, "paid", 50000, base.Add(time.Hour))
	seedAffiliateMePayoutRow(t, repo, aff.ID, "rejected", 25000, base.Add(2*time.Hour))
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 350000, time.Now().UTC())

	_, other := seedAffiliateMeAffiliator(t, repo, "Affiliator PayList B "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMePayoutRow(t, repo, other.ID, "paid", 999999, time.Now().UTC())

	page, err := svc.ListMyAffiliatePayouts(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Len(t, page.Data, 3)
	require.InDelta(t, 25000, page.Data[0].Amount, 0.001) // newest first
	require.InDelta(t, 50000, page.Data[1].Amount, 0.001)
	require.InDelta(t, 100000, page.Data[2].Amount, 0.001)
	require.InDelta(t, 175000, page.Saldo, 0.001)
	require.InDelta(t, 100000, page.MinPayout, 0.001)
	require.NotNil(t, page.PayoutDayOfMonth)
	require.Equal(t, 15, *page.PayoutDayOfMonth)

	paged, err := svc.ListMyAffiliatePayouts(ctx, userID, AffiliateMeListFilter{Page: 2, PerPage: 2})
	require.NoError(t, err)
	require.Equal(t, 3, paged.Total)
	require.Len(t, paged.Data, 1)
}

func TestListMyAffiliatePayouts_flagOffReturnsSentinel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "false")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator PayFlagOff "+uniqueSuffix(), "active", "bronze")

	_, err := svc.ListMyAffiliatePayouts(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
}

func TestListMyAffiliateAdjustments_listsOwnPostedRows(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	decider := seedAffiliateMeUser(t, repo, "Admin Adj Me "+uniqueSuffix(), "super_admin")
	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Adj "+uniqueSuffix(), "active", "bronze")
	_, affB := seedAffiliateMeAffiliator(t, repo, "Affiliator Adj B "+uniqueSuffix(), "active", "bronze")

	now := time.Now().UTC()
	deciderID := uuid.MustParse(decider)
	appliedID := seedAffiliateTxAdjustment(t, repo, aff.ID, "correction", -20000, "koreksi", &deciderID, now)
	foreignID := seedAffiliateTxAdjustment(t, repo, affB.ID, "other", 5000, "milik affiliator lain", &deciderID, now)

	page, err := svc.ListMyAffiliateAdjustments(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Data, 1)
	require.Equal(t, appliedID, page.Data[0].ID)
	require.True(t, page.Data[0].Applied, "a posted adjustment is always applied")
	require.NotEqual(t, foreignID, page.Data[0].ID, "another affiliator's adjustment must not leak")

	_, err = svc.ListMyAffiliateAdjustments(ctx, seedAffiliateMeUser(t, repo, "Affiliator Adj None "+uniqueSuffix(), "affiliator"), AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
}
