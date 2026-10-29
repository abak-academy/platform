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

// Refund tests share the package real-DB fixture (rows never reset), so each
// test seeds its own affiliator/student/order/commission with uniqueSuffix
// names and its own flag row; tests never run in parallel.
func seedRefundCommission(t *testing.T, repo *repository.Repository, affiliatorID, studentID, orderID, orderItemID uuid.UUID, status string, amount float64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliate_commissions
		     (affiliator_id, student_id, order_id, order_item_id, product_type,
		      base_amount, commission_value, level_snapshot, amount, status)
		 VALUES ($1, $2, $3, $4, 'book', 100000, 3, 'bronze', $5, $6) RETURNING id`,
		affiliatorID, studentID, orderID, orderItemID, amount, status,
	).Scan(&id))
	return id
}

func seedCommissionLedgerEntry(t *testing.T, repo *repository.Repository, affiliatorID, commissionID uuid.UUID, amount float64) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id)
		 VALUES ($1, 'commission', $2, $3)`,
		affiliatorID, amount, commissionID,
	)
	require.NoError(t, err)
}

func refundDeductionEntries(t *testing.T, repo *repository.Repository, commissionID uuid.UUID) []model.AffiliateLedgerEntry {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT id, affiliator_id, entry_type, amount, commission_id, payout_id, adjustment_id, note, actor_id, created_at
		 FROM affiliate_ledger_entries WHERE commission_id = $1 AND entry_type = 'refund_deduction' ORDER BY created_at`, commissionID)
	require.NoError(t, err)
	defer rows.Close()

	var out []model.AffiliateLedgerEntry
	for rows.Next() {
		var e model.AffiliateLedgerEntry
		require.NoError(t, rows.Scan(&e.ID, &e.AffiliatorID, &e.EntryType, &e.Amount, &e.CommissionID,
			&e.PayoutID, &e.AdjustmentID, &e.Note, &e.ActorID, &e.CreatedAt))
		out = append(out, e)
	}
	require.NoError(t, rows.Err())
	return out
}

func commissionLedgerCount(t *testing.T, repo *repository.Repository, commissionID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_ledger_entries WHERE commission_id = $1`, commissionID,
	).Scan(&n))
	return n
}

func affiliateLedgerSaldo(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) float64 {
	t.Helper()
	var saldo float64
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_ledger_entries WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&saldo))
	return saldo
}

func commissionStatusByID(t *testing.T, repo *repository.Repository, commissionID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT status FROM affiliate_commissions WHERE id = $1`, commissionID,
	).Scan(&status))
	return status
}

func refundSeed(t *testing.T, repo *repository.Repository, owner affiliateTestOwner, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	student := seedAffiliateTestStudent(t, repo, "student-refund-"+uniqueSuffix()+"@test.local", "")
	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "book", jumlah: 100000},
	)
	commissionID := seedRefundCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), order.ID, order.Items[0].ID, status, 3000)
	return order.ID, commissionID
}

func TestRefundOrder_PendingCommissionCancelledWithoutLedgerRow(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-refund-pend-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)

	orderID, commissionID := refundSeed(t, repo, owner, "pending")

	require.NoError(t, svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	require.Equal(t, "cancelled", commissionStatusByID(t, repo, commissionID))
	require.Equal(t, 0, commissionLedgerCount(t, repo, commissionID))
}

func TestRefundOrder_AvailableCommissionWritesNegativeDeduction(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-refund-avail-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)

	orderID, commissionID := refundSeed(t, repo, owner, "available")
	seedCommissionLedgerEntry(t, repo, owner.affiliatorID, commissionID, 3000)

	require.NoError(t, svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	require.Equal(t, "cancelled", commissionStatusByID(t, repo, commissionID))
	rows := refundDeductionEntries(t, repo, commissionID)
	require.Len(t, rows, 1)
	deduction := rows[0]
	require.Equal(t, "refund_deduction", deduction.EntryType)
	require.Equal(t, -3000.0, deduction.Amount)
	require.NotNil(t, deduction.Note)
	require.Contains(t, *deduction.Note, orderID.String())
	require.Equal(t, 0.0, affiliateLedgerSaldo(t, repo, owner.affiliatorID))
}

func TestRefundOrder_FlagOffLeavesCommissionsAndLedgerUntouched(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "false")
	owner := seedAffiliatorOwner(t, repo, "aff-refund-off-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)

	orderID, commissionID := refundSeed(t, repo, owner, "available")
	seedCommissionLedgerEntry(t, repo, owner.affiliatorID, commissionID, 3000)

	require.NoError(t, svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	require.Equal(t, "available", commissionStatusByID(t, repo, commissionID))
	require.Empty(t, refundDeductionEntries(t, repo, commissionID))
	require.Equal(t, 1, commissionLedgerCount(t, repo, commissionID))
}

func TestRefundOrder_DoubleRefundRefusedBeforeAffiliateSettlement(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-refund-twice-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)

	orderID, commissionID := refundSeed(t, repo, owner, "available")
	seedCommissionLedgerEntry(t, repo, owner.affiliatorID, commissionID, 3000)

	require.NoError(t, svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	err := svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png")
	require.ErrorIs(t, err, ErrOrderNotRefundable)

	require.Equal(t, "cancelled", commissionStatusByID(t, repo, commissionID))
	rows := refundDeductionEntries(t, repo, commissionID)
	require.Len(t, rows, 1)
	require.Equal(t, "refund_deduction", rows[0].EntryType)
	require.Equal(t, 2, commissionLedgerCount(t, repo, commissionID))
}

// A release that starts while the refund holds the commission row must wait and
// then find it cancelled; without the row lock it credited the ledger first and
// the refund then overwrote the status, leaving a credit with no deduction.
func TestRefundOrder_ConcurrentReleaseWaitsAndFindsCommissionCancelled(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-refund-race-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedRevokeActor(t, repo)

	orderID, commissionID := refundSeed(t, repo, owner, "pending")

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	_, err = repo.GetCommissionsByOrderTx(ctx, tx, orderID)
	require.NoError(t, err)

	type result struct {
		done bool
		err  error
	}
	released := make(chan result, 1)
	go func() {
		done, err := svc.releaseAffiliateCommission(ctx, repository.AffiliateDueCommission{
			Commission: model.AffiliateCommission{ID: commissionID, OrderID: orderID},
			ItemName:   "book",
		})
		released <- result{done, err}
	}()
	time.Sleep(300 * time.Millisecond)

	require.NoError(t, svc.settleAffiliateRefundTx(ctx, tx, orderID, actor.String()))
	require.NoError(t, tx.Commit(ctx))

	select {
	case res := <-released:
		require.NoError(t, res.err)
		require.False(t, res.done, "the release must lose to the refund")
	case <-time.After(10 * time.Second):
		t.Fatal("release never returned")
	}
	require.Equal(t, "cancelled", commissionStatusByID(t, repo, commissionID))
	require.Equal(t, 0, commissionLedgerCount(t, repo, commissionID))
	require.Equal(t, 0.0, affiliateLedgerSaldo(t, repo, owner.affiliatorID))
}
