package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

var (
	// ErrAffiliateNotFound reports an unknown affiliator, adjustment or
	// payout id; affiliate_admin.go and affiliate_me.go reuse it.
	ErrAffiliateNotFound = errors.New("affiliate resource not found")
	// ErrAffiliateSelfAdjustment is BR-ADJ-04: an admin must not adjust the
	// affiliator account backed by their own user id.
	ErrAffiliateSelfAdjustment = errors.New("cannot adjust your own affiliator account")
	// ErrAffiliateAdjustmentInvalid is BR-ADJ-01: the category must be known,
	// the amount non-zero and the reason mandatory.
	ErrAffiliateAdjustmentInvalid = errors.New("adjustment needs a known category, a non-zero amount and a reason")
	// ErrAffiliatePayoutInvalid is BR-PCR-03/04: a paid decision needs a proof
	// upload key and a rejection a reason.
	ErrAffiliatePayoutInvalid = errors.New("invalid payout decision")
	// ErrAffiliatePayoutNotDecidable reports a payout already paid or rejected.
	ErrAffiliatePayoutNotDecidable = errors.New("affiliate payout is no longer decidable")
)

var affiliateAdjustmentCategories = map[string]bool{
	"correction": true,
	"offline":    true,
	"bonus":      true,
	"violation":  true,
	"other":      true,
}

var affiliatePayoutStatuses = map[string]bool{
	"requested":  true,
	"processing": true,
	"paid":       true,
	"rejected":   true,
}

// affiliatePayoutQueueStatuses is the undecided queue: what the admin list
// shows by default and what the CSV export covers.
var affiliatePayoutQueueStatuses = []string{"requested", "processing"}

var affiliateApprovalPending = "pending"

// AdminAdjustmentInput is the body of POST /admin/affiliates/:id/adjustments.
type AdminAdjustmentInput struct {
	Category string
	Amount   float64
	Reason   string
	Note     *string
}

// AdminPayoutDecisionInput is the body of POST /admin/affiliate-payouts/:id/decide.
type AdminPayoutDecisionInput struct {
	Decision string
	ProofURL string
	Reason   string
}

// AdminCreateAffiliateAdjustment writes one admin adjustment (BR-ADJ-01..06).
// Phase 1 applies it immediately: one transaction writes the adjustment row
// and its signed ledger entry. BR-ADJ-03 stays dormant: when the threshold is
// enabled and |amount| exceeds it, the row is created as pending with no
// ledger entry, awaiting the second-approval flow that is not built yet.
func (s *Service) AdminCreateAffiliateAdjustment(ctx context.Context, actorID, affiliatorID string, in AdminAdjustmentInput) (*model.AffiliateAdjustment, error) {
	affID, err := parseUUID(affiliatorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	reason := strings.TrimSpace(in.Reason)
	if !affiliateAdjustmentCategories[in.Category] || in.Amount == 0 || reason == "" {
		return nil, ErrAffiliateAdjustmentInvalid
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, err
	}
	pending := settings.AdjustmentThresholdEnabled && math.Abs(in.Amount) > settings.AdjustmentThreshold

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin adjustment tx: %w", err)
	}
	defer tx.Rollback(ctx)

	aff, err := s.storeRepo.GetAffiliatorByID(ctx, tx, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if aff.UserID == actor {
		return nil, ErrAffiliateSelfAdjustment
	}

	adj := &model.AffiliateAdjustment{
		AffiliatorID: aff.ID,
		Category:     in.Category,
		Amount:       in.Amount,
		Reason:       reason,
		Note:         in.Note,
		CreatedBy:    actor,
	}
	if pending {
		adj.ApprovalStatus = &affiliateApprovalPending
	}
	if err := s.storeRepo.InsertAffiliateAdjustmentTx(ctx, tx, adj); err != nil {
		return nil, fmt.Errorf("insert affiliate adjustment: %w", err)
	}
	if !pending {
		if err := s.storeRepo.InsertAffiliateLedgerEntryTx(ctx, tx, &model.AffiliateLedgerEntry{
			AffiliatorID: aff.ID,
			EntryType:    "adjustment",
			Amount:       in.Amount,
			AdjustmentID: &adj.ID,
			Note:         &reason,
			ActorID:      &actor,
		}); err != nil {
			return nil, fmt.Errorf("insert adjustment ledger entry: %w", err)
		}
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_adjustment", adj.ID.String(), "affiliate.adjustment", map[string]any{
		"affiliator_id": aff.ID,
		"category":      in.Category,
		"amount":        in.Amount,
		"reason":        reason,
	}); err != nil {
		return nil, fmt.Errorf("insert adjustment audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit adjustment: %w", err)
	}
	return adj, nil
}

// AdminListAffiliateAdjustments returns one affiliator's adjustment history.
func (s *Service) AdminListAffiliateAdjustments(ctx context.Context, affiliatorID string, limit, offset int) ([]model.AffiliateAdjustment, int, error) {
	id, err := parseUUID(affiliatorID)
	if err != nil {
		return nil, 0, ErrInvalidUUID
	}
	rows, total, err := s.storeRepo.ListAffiliateAdjustments(ctx, id, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate adjustments: %w", err)
	}
	return rows, total, nil
}

// AdminListAffiliatePayouts backs GET /admin/affiliate-payouts. Without a
// status it lists the undecided queue; "all" lists every payout.
func (s *Service) AdminListAffiliatePayouts(ctx context.Context, f AffiliateAdminQueueFilter) ([]repository.AffiliatePayoutQueueRow, int, error) {
	statuses := affiliatePayoutQueueStatuses
	switch f.Status {
	case "":
	case "all":
		statuses = []string{"requested", "processing", "paid", "rejected"}
	default:
		statuses = nil
		for _, status := range strings.Split(f.Status, ",") {
			status = strings.TrimSpace(status)
			if !affiliatePayoutStatuses[status] {
				return nil, 0, ErrInvalidStatusFilter
			}
			statuses = append(statuses, status)
		}
	}
	rows, total, err := s.storeRepo.ListAffiliatePayoutQueue(ctx, statuses, f.repoQuery(), f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate payouts: %w", err)
	}
	return rows, total, nil
}

// AdminAffiliatePayoutsCSV builds the manual-transfer export (BR-PCR-03) of the
// payouts matching the filter (the undecided queue by default), capped at
// affiliateAdminExportLimit rows. Text columns are neutralised by csvSafeField;
// amount and timestamp stay typed.
func (s *Service) AdminAffiliatePayoutsCSV(ctx context.Context, f AffiliateAdminQueueFilter) ([]byte, error) {
	f.Limit, f.Offset = affiliateAdminExportLimit, 0
	rows, _, err := s.AdminListAffiliatePayouts(ctx, f)
	if err != nil {
		return nil, err
	}
	return buildAdminCSV(
		[]string{"affiliator_name", "code", "bank_name", "bank_account_number", "bank_account_holder", "amount", "status", "requested_at"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				csvSafeField(row.AffiliatorName),
				csvSafeField(row.AffiliatorCode),
				csvSafeField(row.BankName),
				csvSafeField(row.BankAccountNumber),
				csvSafeField(row.BankAccountHolder),
				strconv.FormatFloat(row.Amount, 'f', 2, 64),
				row.Status,
				row.RequestedAt.Format(time.RFC3339),
			}
		},
	)
}

// AdminDecideAffiliatePayout marks a queued payout paid (with the mandatory
// transfer proof, BR-PCR-03) or rejected (with the mandatory reason, BR-PCR-04).
// The rejection compensates the balance only when the request-time deduction
// ledger row exists (D2 owns that write); without it there is nothing to
// reverse. uq_affiliate_ledger only constrains commission rows, so reversal
// idempotence is the payout status guard plus this transaction's existence
// check.
func (s *Service) AdminDecideAffiliatePayout(ctx context.Context, actorID, payoutID string, in AdminPayoutDecisionInput) (*model.AffiliatePayout, error) {
	id, err := parseUUID(payoutID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	switch in.Decision {
	case "paid":
		if strings.TrimSpace(in.ProofURL) == "" {
			return nil, ErrAffiliatePayoutInvalid
		}
		if err := s.requireStoredObject(ctx, in.ProofURL); err != nil {
			return nil, err
		}
	case "reject":
		if strings.TrimSpace(in.Reason) == "" {
			return nil, ErrAffiliatePayoutInvalid
		}
	default:
		return nil, ErrAffiliatePayoutInvalid
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin payout decide tx: %w", err)
	}
	defer tx.Rollback(ctx)

	payout, err := s.storeRepo.GetAffiliatePayoutForUpdateTx(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("lock affiliate payout: %w", err)
	}
	if payout == nil {
		return nil, ErrAffiliateNotFound
	}
	switch payout.Status {
	case "requested", "processing":
	default:
		return nil, ErrAffiliatePayoutNotDecidable
	}

	var updated *model.AffiliatePayout
	switch in.Decision {
	case "paid":
		proof := in.ProofURL
		updated, err = s.storeRepo.UpdateAffiliatePayoutDecisionTx(ctx, tx, id, "paid", &proof, nil, actor)
		if err != nil {
			return nil, fmt.Errorf("mark payout paid: %w", err)
		}
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_payout", id.String(), "affiliate.payout_paid", map[string]any{
			"affiliator_id": payout.AffiliatorID,
			"amount":        payout.Amount,
			"proof_url":     proof,
		}); err != nil {
			return nil, fmt.Errorf("insert payout paid audit: %w", err)
		}
	case "reject":
		reason := strings.TrimSpace(in.Reason)
		updated, err = s.storeRepo.UpdateAffiliatePayoutDecisionTx(ctx, tx, id, "rejected", nil, &reason, actor)
		if err != nil {
			return nil, fmt.Errorf("mark payout rejected: %w", err)
		}
		deducted, err := s.storeRepo.HasAffiliateLedgerEntryTx(ctx, tx, id, "payout")
		if err != nil {
			return nil, fmt.Errorf("check payout deduction: %w", err)
		}
		if deducted {
			reversed, err := s.storeRepo.HasAffiliateLedgerEntryTx(ctx, tx, id, "payout_reversal")
			if err != nil {
				return nil, fmt.Errorf("check payout reversal: %w", err)
			}
			if !reversed {
				note := "payout rejected"
				if err := s.storeRepo.InsertAffiliateLedgerEntryTx(ctx, tx, &model.AffiliateLedgerEntry{
					AffiliatorID: payout.AffiliatorID,
					EntryType:    "payout_reversal",
					Amount:       payout.Amount,
					PayoutID:     &id,
					Note:         &note,
					ActorID:      &actor,
				}); err != nil {
					return nil, fmt.Errorf("insert payout reversal: %w", err)
				}
			}
		}
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_payout", id.String(), "affiliate.payout_rejected", map[string]any{
			"affiliator_id": payout.AffiliatorID,
			"amount":        payout.Amount,
			"reason":        reason,
		}); err != nil {
			return nil, fmt.Errorf("insert payout rejected audit: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit payout decision: %w", err)
	}
	return updated, nil
}

// AdminProcessAffiliatePayout moves a requested payout into processing
// (BR-PCR-04). An already-processing payout is a no-op; decided ones refuse.
func (s *Service) AdminProcessAffiliatePayout(ctx context.Context, actorID, payoutID string) (*model.AffiliatePayout, error) {
	id, err := parseUUID(payoutID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin payout process tx: %w", err)
	}
	defer tx.Rollback(ctx)

	payout, err := s.storeRepo.GetAffiliatePayoutForUpdateTx(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("lock affiliate payout: %w", err)
	}
	if payout == nil {
		return nil, ErrAffiliateNotFound
	}

	updated := payout
	switch payout.Status {
	case "requested":
		updated, err = s.storeRepo.UpdateAffiliatePayoutDecisionTx(ctx, tx, id, "processing", nil, nil, actor)
		if err != nil {
			return nil, fmt.Errorf("mark payout processing: %w", err)
		}
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_payout", id.String(), "affiliate.payout_processed", map[string]any{
			"affiliator_id": payout.AffiliatorID,
			"amount":        payout.Amount,
		}); err != nil {
			return nil, fmt.Errorf("insert payout process audit: %w", err)
		}
	case "processing":
		// Nothing to move; keep the repeat call harmless.
	default:
		return nil, ErrAffiliatePayoutNotDecidable
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit payout process: %w", err)
	}
	return updated, nil
}
