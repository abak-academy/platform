package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestAffiliateTransactions_refundKeepsPastBalanceAndPostsOnce(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-tx-refund-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)
	orderID, commissionID := refundSeed(t, repo, owner, "available")

	beforeRefund := time.Now().UTC()
	require.Equal(t, 3000.0, mustBalanceAt(t, repo, owner, beforeRefund))

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	require.Equal(t, 3000.0, mustBalanceAt(t, repo, owner, beforeRefund), "a past date keeps the balance it had")
	require.Equal(t, 0.0, mustBalanceAt(t, repo, owner, time.Now().UTC().Add(time.Minute)))
	require.Len(t, refundDeductionEntries(t, repo, commissionID), 1)

	require.NoError(t, repo.InsertRefundDeductionTx(ctx, mustBegin(t, repo), commissionID, actor, "again"),
		"a second deduction for the same commission is ignored")
	require.Len(t, refundDeductionEntries(t, repo, commissionID), 1)
}

func TestAffiliateTransactions_payoutRejectionReturnsBalanceOnce(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-tx-payout-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-tx-payout-"+uniqueSuffix()+"@test.local")
	seedAffiliateTxBalance(t, repo, owner.affiliatorID, 200000)

	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 200000, "Tx Holder")
	require.Equal(t, 0.0, mustBalanceAt(t, repo, owner, time.Now().UTC().Add(time.Minute)), "a requested payout debits at once")
	requestedAt := time.Now().UTC()

	time.Sleep(10 * time.Millisecond)
	_, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{Decision: "reject", Reason: "salah rekening"})
	require.NoError(t, err)

	require.Equal(t, 200000.0, mustBalanceAt(t, repo, owner, time.Now().UTC().Add(time.Minute)))
	require.Equal(t, 0.0, mustBalanceAt(t, repo, owner, requestedAt), "the balance while the request stood stays reduced")

	_, err = svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{Decision: "reject", Reason: "lagi"})
	require.ErrorIs(t, err, ErrAffiliatePayoutNotDecidable)
	require.Equal(t, 200000.0, mustBalanceAt(t, repo, owner, time.Now().UTC().Add(time.Minute)))
}

func TestAffiliateTransactions_oneInFlightPayoutPerAffiliator(t *testing.T) {
	_, repo := newRealDBService(t)
	owner := seedAffiliatorOwner(t, repo, "aff-tx-flight-"+uniqueSuffix()+"@test.local", "", "active")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 100000, "Flight Holder")

	tx := mustBegin(t, repo)
	err := repo.InsertAffiliatePayoutTx(context.Background(), tx, &model.AffiliatePayout{AffiliatorID: owner.affiliatorID, Amount: 50000, Status: "requested"})
	require.ErrorIs(t, err, repository.ErrAffiliatePayoutInFlight)
}

func TestAffiliateTransactions_guardRefusesRewritesThroughTheApp(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-tx-guard-"+uniqueSuffix()+"@test.local", "", "active")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 100000, "Guard Holder")

	_, err := repo.Pool().Exec(ctx, `UPDATE affiliate_transactions SET amount = -1 WHERE id = $1`, payoutID)
	require.Error(t, err, "amount is frozen")
	_, err = repo.Pool().Exec(ctx, `UPDATE affiliate_transactions SET status = 'requested' WHERE id = $1`, payoutID)
	require.Error(t, err, "a paid payout is final")
	_, err = repo.Pool().Exec(ctx, `DELETE FROM affiliate_transactions WHERE id = $1`, payoutID)
	require.Error(t, err, "rows are never deleted")
}

func mustBalanceAt(t *testing.T, repo *repository.Repository, owner affiliateTestOwner, at time.Time) float64 {
	t.Helper()
	saldo, err := repo.SumAffiliateBalanceAt(context.Background(), owner.affiliatorID, at)
	require.NoError(t, err)
	return saldo
}

func mustBegin(t *testing.T, repo *repository.Repository) pgx.Tx {
	t.Helper()
	tx, err := repo.BeginTx(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}
