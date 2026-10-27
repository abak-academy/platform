package service

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The real-DB fixture is shared and never reset, so every test seeds unique
// users/codes and asserts on its own affiliator's rows. Money endpoints share
// the singleton settings row, so the threshold seed resets itself on cleanup.
func seedAffiliateAdminActor(t *testing.T, repo *repository.Repository, email string) string {
	t.Helper()
	return seedAffiliateBindingUser(t, repo, email, "", "super_admin")
}

func seedAffiliateThresholdSettings(t *testing.T, repo *repository.Repository, enabled bool, threshold float64) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, adjustment_threshold, adjustment_threshold_enabled)
		 VALUES (1, $1, $2)
		 ON CONFLICT (id) DO UPDATE SET adjustment_threshold = $1, adjustment_threshold_enabled = $2, updated_at = now()`,
		threshold, enabled,
	)
	if err != nil {
		t.Fatalf("seed affiliate threshold settings: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`UPDATE affiliate_settings SET adjustment_threshold = 1000000, adjustment_threshold_enabled = false WHERE id = 1`)
	})
}

func seedAffiliateMoneyPayout(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, status string, amount float64, holder string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliate_payouts (affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder)
		 VALUES ($1, $2, $3, 'BCA', '1234567890', $4)
		 RETURNING id`,
		affiliatorID, amount, status, holder,
	).Scan(&id); err != nil {
		t.Fatalf("seed affiliate payout: %v", err)
	}
	return id
}

func seedAffiliatePayoutDeduction(t *testing.T, repo *repository.Repository, affiliatorID, payoutID uuid.UUID, amount float64) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, payout_id)
		 VALUES ($1, 'payout', $2, $3)`,
		affiliatorID, -amount, payoutID,
	); err != nil {
		t.Fatalf("seed payout deduction: %v", err)
	}
}

func countAffiliateAdjustmentsFor(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_adjustments WHERE affiliator_id = $1`, affiliatorID,
	).Scan(&n); err != nil {
		t.Fatalf("count adjustments: %v", err)
	}
	return n
}

func countAffiliateLedgerByTypeFor(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, entryType string) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_ledger_entries WHERE affiliator_id = $1 AND entry_type = $2`,
		affiliatorID, entryType,
	).Scan(&n); err != nil {
		t.Fatalf("count ledger entries: %v", err)
	}
	return n
}

func countAffiliateAudit(t *testing.T, repo *repository.Repository, action, targetID string) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = $1 AND target_id = $2`, action, targetID,
	).Scan(&n); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	return n
}

func TestAdminCreateAffiliateAdjustment_immediateApplication(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-imm-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-imm-"+uniqueSuffix()+"@test.local")

	adj, err := svc.AdminCreateAffiliateAdjustment(ctx, actor, owner.affiliatorID.String(), AdminAdjustmentInput{
		Category: "correction",
		Amount:   -25000,
		Reason:   "koreksi double refund",
	})
	if err != nil {
		t.Fatalf("AdminCreateAffiliateAdjustment: %v", err)
	}
	if adj.ID == uuid.Nil {
		t.Fatal("want adjustment id")
	}
	if adj.ApprovalStatus != nil {
		t.Errorf("approval_status = %v, want nil (Phase 1 applies immediately)", *adj.ApprovalStatus)
	}
	if n := countAffiliateAdjustmentsFor(t, repo, owner.affiliatorID); n != 1 {
		t.Fatalf("affiliate_adjustments rows = %d, want 1", n)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "adjustment"); n != 1 {
		t.Fatalf("adjustment ledger rows = %d, want 1", n)
	}
	var amount float64
	var note string
	var adjustmentID, actorID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`SELECT amount, note, adjustment_id, actor_id FROM affiliate_ledger_entries
		 WHERE affiliator_id = $1 AND entry_type = 'adjustment'`, owner.affiliatorID,
	).Scan(&amount, &note, &adjustmentID, &actorID); err != nil {
		t.Fatalf("read ledger row: %v", err)
	}
	if amount != -25000 {
		t.Errorf("ledger amount = %v, want -25000 (signed)", amount)
	}
	if note != "koreksi double refund" {
		t.Errorf("ledger note = %q, want the reason", note)
	}
	if adjustmentID != adj.ID {
		t.Errorf("ledger adjustment_id = %v, want %v", adjustmentID, adj.ID)
	}
	if actorID.String() != actor {
		t.Errorf("ledger actor_id = %v, want %v", actorID, actor)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.adjustment", adj.ID.String()); n != 1 {
		t.Errorf("affiliate.adjustment audit rows = %d, want 1", n)
	}
}

func TestAdminCreateAffiliateAdjustment_invalidInput(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-bad-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-bad-"+uniqueSuffix()+"@test.local")

	tests := []struct {
		name string
		in   AdminAdjustmentInput
	}{
		{"unknown category", AdminAdjustmentInput{Category: "gift", Amount: 1000, Reason: "why"}},
		{"zero amount", AdminAdjustmentInput{Category: "bonus", Amount: 0, Reason: "why"}},
		{"empty reason", AdminAdjustmentInput{Category: "bonus", Amount: 1000, Reason: "   "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.AdminCreateAffiliateAdjustment(ctx, actor, owner.affiliatorID.String(), tt.in)
			if !errors.Is(err, ErrAffiliateAdjustmentInvalid) {
				t.Fatalf("err = %v, want ErrAffiliateAdjustmentInvalid", err)
			}
			if n := countAffiliateAdjustmentsFor(t, repo, owner.affiliatorID); n != 0 {
				t.Errorf("adjustments persisted = %d, want 0", n)
			}
			if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "adjustment"); n != 0 {
				t.Errorf("ledger rows persisted = %d, want 0", n)
			}
		})
	}
}

func TestAdminCreateAffiliateAdjustment_selfTargetRejected(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-self-"+uniqueSuffix()+"@test.local", "", "active")

	_, err := svc.AdminCreateAffiliateAdjustment(ctx, owner.userID, owner.affiliatorID.String(), AdminAdjustmentInput{
		Category: "bonus",
		Amount:   1000,
		Reason:   "self bonus",
	})
	if !errors.Is(err, ErrAffiliateSelfAdjustment) {
		t.Fatalf("err = %v, want ErrAffiliateSelfAdjustment", err)
	}
	if n := countAffiliateAdjustmentsFor(t, repo, owner.affiliatorID); n != 0 {
		t.Errorf("adjustments persisted = %d, want 0", n)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "adjustment"); n != 0 {
		t.Errorf("ledger rows persisted = %d, want 0", n)
	}
}

func TestAdminCreateAffiliateAdjustment_thresholdPendingPath(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-pend-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-pend-"+uniqueSuffix()+"@test.local")
	seedAffiliateThresholdSettings(t, repo, true, 50000)

	adj, err := svc.AdminCreateAffiliateAdjustment(ctx, actor, owner.affiliatorID.String(), AdminAdjustmentInput{
		Category: "bonus",
		Amount:   150000,
		Reason:   "large bonus",
	})
	if err != nil {
		t.Fatalf("AdminCreateAffiliateAdjustment: %v", err)
	}
	if adj.ApprovalStatus == nil || *adj.ApprovalStatus != "pending" {
		t.Fatalf("approval_status = %v, want pending", adj.ApprovalStatus)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "adjustment"); n != 0 {
		t.Errorf("ledger rows = %d, want 0 while pending", n)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.adjustment", adj.ID.String()); n != 1 {
		t.Errorf("affiliate.adjustment audit rows = %d, want 1", n)
	}
}

func TestAdminCreateAffiliateAdjustment_thresholdBelowApplies(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-under-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-under-"+uniqueSuffix()+"@test.local")
	seedAffiliateThresholdSettings(t, repo, true, 50000)

	adj, err := svc.AdminCreateAffiliateAdjustment(ctx, actor, owner.affiliatorID.String(), AdminAdjustmentInput{
		Category: "offline",
		Amount:   30000,
		Reason:   "offline payment",
	})
	if err != nil {
		t.Fatalf("AdminCreateAffiliateAdjustment: %v", err)
	}
	if adj.ApprovalStatus != nil {
		t.Errorf("approval_status = %v, want nil below threshold", *adj.ApprovalStatus)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "adjustment"); n != 1 {
		t.Errorf("ledger rows = %d, want 1", n)
	}
}

func TestAdminCreateAffiliateAdjustment_unknownAffiliator(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-unknown-"+uniqueSuffix()+"@test.local")

	_, err := svc.AdminCreateAffiliateAdjustment(context.Background(), actor, uuid.NewString(), AdminAdjustmentInput{
		Category: "bonus",
		Amount:   1000,
		Reason:   "why",
	})
	if !errors.Is(err, ErrAffiliateNotFound) {
		t.Fatalf("err = %v, want ErrAffiliateNotFound", err)
	}
}

func TestAdminListAffiliateAdjustments_paginates(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-adj-list-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-adj-list-"+uniqueSuffix()+"@test.local")

	for i := 0; i < 3; i++ {
		if _, err := svc.AdminCreateAffiliateAdjustment(ctx, actor, owner.affiliatorID.String(), AdminAdjustmentInput{
			Category: "other",
			Amount:   float64(i + 1),
			Reason:   "list seed",
		}); err != nil {
			t.Fatalf("seed adjustment %d: %v", i, err)
		}
	}

	rows, total, err := svc.AdminListAffiliateAdjustments(ctx, owner.affiliatorID.String(), 2, 0)
	if err != nil {
		t.Fatalf("AdminListAffiliateAdjustments: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 2 {
		t.Fatalf("page rows = %d, want 2", len(rows))
	}
	rows, total, err = svc.AdminListAffiliateAdjustments(ctx, owner.affiliatorID.String(), 2, 2)
	if err != nil {
		t.Fatalf("AdminListAffiliateAdjustments page 2: %v", err)
	}
	if total != 3 || len(rows) != 1 {
		t.Errorf("page 2: total = %d, rows = %d, want 3 and 1", total, len(rows))
	}
}

func TestAdminListAffiliatePayouts_defaultAndFilters(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-list-"+uniqueSuffix()+"@test.local", "", "active")

	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 100000, "Payer One")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 200000, "Payer Two")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "processing", 300000, "Payer Three")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 400000, "ShouldNotAppear")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "rejected", 500000, "ShouldNotAppear")

	rows, total, err := svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: owner.code, Limit: 100})
	if err != nil {
		t.Fatalf("AdminListAffiliatePayouts default: %v", err)
	}
	if total < 3 {
		t.Errorf("total = %d, want at least 3", total)
	}
	mine := 0
	for _, row := range rows {
		if row.AffiliatorCode != owner.code {
			continue
		}
		mine++
		if row.Status != "requested" && row.Status != "processing" {
			t.Errorf("default filter leaked status %q", row.Status)
		}
		if row.AffiliatorName == "" || row.BankName == "" {
			t.Errorf("queue row missing affiliator/bank snapshot: %+v", row)
		}
	}
	if mine != 3 {
		t.Errorf("default queue rows for this affiliator = %d, want 3", mine)
	}

	rows, _, err = svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "paid", Limit: 100})
	if err != nil {
		t.Fatalf("AdminListAffiliatePayouts paid: %v", err)
	}
	mine = 0
	for _, row := range rows {
		if row.AffiliatorCode == owner.code {
			mine++
			if row.Status != "paid" {
				t.Errorf("paid filter returned status %q", row.Status)
			}
		}
	}
	if mine != 1 {
		t.Errorf("paid filter rows for this affiliator = %d, want 1", mine)
	}

	if _, _, err = svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Status: "bogus", Limit: 100}); !errors.Is(err, ErrInvalidStatusFilter) {
		t.Errorf("err = %v, want ErrInvalidStatusFilter", err)
	}
}

func TestAdminAffiliatePayoutsCSV_queueContentsAndInjection(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-csv-"+uniqueSuffix()+"@test.local", "", "active")

	requested := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 150000, "=EVILCELL")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "processing", 250000, "Normal Holder")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 350000, "ShouldNotAppear")

	data, err := svc.AdminAffiliatePayoutsCSV(ctx, AffiliateAdminQueueFilter{Q: owner.code})
	if err != nil {
		t.Fatalf("AdminAffiliatePayoutsCSV: %v", err)
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf")))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("export is not parseable CSV: %v", err)
	}
	if len(records) < 3 {
		t.Fatalf("records = %d, want header plus at least 2 rows", len(records))
	}
	wantHeader := []string{"affiliator_name", "code", "bank_name", "bank_account_number", "bank_account_holder", "amount", "status", "requested_at"}
	for i, col := range wantHeader {
		if records[0][i] != col {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], col)
		}
	}
	escaped := false
	paidLeaked := false
	for _, record := range records[1:] {
		if record[5] == "ShouldNotAppear" {
			paidLeaked = true
		}
		if record[4] == "'=EVILCELL" {
			escaped = true
		}
	}
	if paidLeaked {
		t.Error("paid payout leaked into the queue export")
	}
	if !escaped {
		t.Error("formula-injection guard did not fire: '=EVILCELL holder not neutralised")
	}
	_ = requested
}

func TestAdminListAffiliatePayouts_allStatusesSearchAndDates(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-all-"+uniqueSuffix()+"@test.local", "", "active")

	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 100000, "Payer One")
	holder := "Holder" + uniqueSuffix()
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 400000, holder)
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "rejected", 500000, "Payer Three")

	all, total, err := svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all", Limit: 100})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, all, 3)
	require.Equal(t, "requested", all[0].Status, "undecided payouts come first")

	byHolder, total, err := svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: strings.ToLower(holder), Status: "all", Limit: 100})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "paid", byHolder[0].Status)

	future := time.Now().Add(48 * time.Hour)
	none, total, err := svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all", From: &future, Limit: 100})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, none)

	page2, total, err := svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all", Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page2, 1)
}

func TestAdminDecideAffiliatePayout_paid(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-paid-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-paid-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 300000, "Paid Holder")
	seedAffiliatePayoutDeduction(t, repo, owner.affiliatorID, payoutID, 300000)
	proof := "payment_proof/" + actor + "/proof-" + uniqueSuffix() + ".jpg"

	updated, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{
		Decision: "paid",
		ProofURL: proof,
	})
	if err != nil {
		t.Fatalf("AdminDecideAffiliatePayout paid: %v", err)
	}
	if updated.Status != "paid" {
		t.Errorf("status = %q, want paid", updated.Status)
	}
	if updated.ProofURL == nil || *updated.ProofURL != proof {
		t.Errorf("proof_url = %v, want %q", updated.ProofURL, proof)
	}
	if updated.ProcessedBy == nil || updated.ProcessedBy.String() != actor {
		t.Errorf("processed_by = %v, want %v", updated.ProcessedBy, actor)
	}
	if updated.ProcessedAt == nil {
		t.Error("processed_at = nil, want set")
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "payout_reversal"); n != 0 {
		t.Errorf("payout_reversal rows = %d, want 0 on paid", n)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.payout_paid", payoutID.String()); n != 1 {
		t.Errorf("affiliate.payout_paid audit rows = %d, want 1", n)
	}
}

func TestAdminDecideAffiliatePayout_rejectWithDeductionWritesReversal(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-rej-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-rej-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 200000, "Reject Holder")
	seedAffiliatePayoutDeduction(t, repo, owner.affiliatorID, payoutID, 200000)

	updated, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{
		Decision: "reject",
		Reason:   "rekening tidak cocok",
	})
	if err != nil {
		t.Fatalf("AdminDecideAffiliatePayout reject: %v", err)
	}
	if updated.Status != "rejected" {
		t.Errorf("status = %q, want rejected", updated.Status)
	}
	if updated.RejectReason == nil || *updated.RejectReason != "rekening tidak cocok" {
		t.Errorf("reject_reason = %v, want the reason", updated.RejectReason)
	}
	var amount float64
	var note string
	var payoutRef, actorRef uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`SELECT amount, note, payout_id, actor_id FROM affiliate_ledger_entries
		 WHERE affiliator_id = $1 AND entry_type = 'payout_reversal'`, owner.affiliatorID,
	).Scan(&amount, &note, &payoutRef, &actorRef); err != nil {
		t.Fatalf("read reversal row: %v", err)
	}
	if amount != 200000 {
		t.Errorf("reversal amount = %v, want +200000", amount)
	}
	if payoutRef != payoutID {
		t.Errorf("reversal payout_id = %v, want %v", payoutRef, payoutID)
	}
	if actorRef.String() != actor {
		t.Errorf("reversal actor_id = %v, want %v", actorRef, actor)
	}
	_ = note
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "payout_reversal"); n != 1 {
		t.Errorf("payout_reversal rows = %d, want exactly 1", n)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.payout_rejected", payoutID.String()); n != 1 {
		t.Errorf("affiliate.payout_rejected audit rows = %d, want 1", n)
	}
}

func TestAdminDecideAffiliatePayout_rejectWithoutDeduction(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-rej2-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-rej2-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "processing", 100000, "No Ledger Holder")

	updated, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{
		Decision: "reject",
		Reason:   "flagged for review",
	})
	if err != nil {
		t.Fatalf("AdminDecideAffiliatePayout reject: %v", err)
	}
	if updated.Status != "rejected" {
		t.Errorf("status = %q, want rejected", updated.Status)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "payout_reversal"); n != 0 {
		t.Errorf("payout_reversal rows = %d, want 0 without a deduction", n)
	}
}

func TestAdminDecideAffiliatePayout_invalidDecisions(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-bad-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-bad-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 100000, "Bad Holder")

	tests := []struct {
		name string
		in   AdminPayoutDecisionInput
	}{
		{"paid without proof", AdminPayoutDecisionInput{Decision: "paid"}},
		{"unknown decision", AdminPayoutDecisionInput{Decision: "approve", Reason: "why"}},
		{"reject without reason", AdminPayoutDecisionInput{Decision: "reject", Reason: "   "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), tt.in)
			if !errors.Is(err, ErrAffiliatePayoutInvalid) {
				t.Fatalf("err = %v, want ErrAffiliatePayoutInvalid", err)
			}
			var status string
			if err := repo.Pool().QueryRow(ctx, `SELECT status FROM affiliate_payouts WHERE id = $1`, payoutID).Scan(&status); err != nil {
				t.Fatalf("read payout: %v", err)
			}
			if status != "requested" {
				t.Errorf("status = %q, want unchanged requested", status)
			}
		})
	}
}

func TestAdminDecideAffiliatePayout_alreadyDecided(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-done-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-done-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 100000, "Done Holder")

	_, err := svc.AdminDecideAffiliatePayout(ctx, actor, payoutID.String(), AdminPayoutDecisionInput{
		Decision: "reject",
		Reason:   "double decide",
	})
	if !errors.Is(err, ErrAffiliatePayoutNotDecidable) {
		t.Fatalf("err = %v, want ErrAffiliatePayoutNotDecidable", err)
	}
	if n := countAffiliateLedgerByTypeFor(t, repo, owner.affiliatorID, "payout_reversal"); n != 0 {
		t.Errorf("payout_reversal rows = %d, want 0", n)
	}
}

func TestAdminDecideAffiliatePayout_unknownPayout(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-unknown-"+uniqueSuffix()+"@test.local")

	_, err := svc.AdminDecideAffiliatePayout(context.Background(), actor, uuid.NewString(), AdminPayoutDecisionInput{
		Decision: "paid",
		ProofURL: "payment_proof/" + actor + "/proof.jpg",
	})
	if !errors.Is(err, ErrAffiliateNotFound) {
		t.Fatalf("err = %v, want ErrAffiliateNotFound", err)
	}
}

func TestAdminProcessAffiliatePayout_requestedToProcessing(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-proc-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-proc-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 100000, "Process Holder")

	updated, err := svc.AdminProcessAffiliatePayout(ctx, actor, payoutID.String())
	if err != nil {
		t.Fatalf("AdminProcessAffiliatePayout: %v", err)
	}
	if updated.Status != "processing" {
		t.Errorf("status = %q, want processing", updated.Status)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.payout_processed", payoutID.String()); n != 1 {
		t.Errorf("affiliate.payout_processed audit rows = %d, want 1", n)
	}

	updated, err = svc.AdminProcessAffiliatePayout(ctx, actor, payoutID.String())
	if err != nil {
		t.Fatalf("AdminProcessAffiliatePayout repeat: %v", err)
	}
	if updated.Status != "processing" {
		t.Errorf("repeat status = %q, want processing", updated.Status)
	}
	if n := countAffiliateAudit(t, repo, "affiliate.payout_processed", payoutID.String()); n != 1 {
		t.Errorf("repeat audit rows = %d, want still 1", n)
	}
}

func TestAdminProcessAffiliatePayout_rejectsDecidedPayout(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-pay-proc2-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "admin-pay-proc2-"+uniqueSuffix()+"@test.local")
	payoutID := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "rejected", 100000, "Decided Holder")

	_, err := svc.AdminProcessAffiliatePayout(ctx, actor, payoutID.String())
	if !errors.Is(err, ErrAffiliatePayoutNotDecidable) {
		t.Fatalf("err = %v, want ErrAffiliatePayoutNotDecidable", err)
	}
}
