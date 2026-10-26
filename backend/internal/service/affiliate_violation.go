package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrAffiliateViolationInvalid = errors.New("invalid affiliate violation")

const affiliateAuditViolation = "affiliate.violation_record"

var affiliateViolationCategories = map[string]bool{
	"transaction_fraud":    true,
	"data_misuse":          true,
	"misleading_promotion": true,
	"program_violation":    true,
	"ethics":               true,
	"other":                true,
}

var affiliateViolationSanctions = map[string]bool{
	"warning":           true,
	"commission_cancel": true,
	"balance_freeze":    true,
	"level_change":      true,
	"suspend":           true,
	"deactivate":        true,
}

// AdminViolationInput is the body of POST /admin/affiliates/:id/violations.
// Days applies to balance_freeze and suspend, Level/LockUntil to level_change,
// CommissionIDs to commission_cancel.
type AdminViolationInput struct {
	Category      string
	Description   string
	Sanction      string
	Days          *int
	Level         string
	LockUntil     string
	CommissionIDs []string
}

// AdminRecordAffiliateViolation records one violation (BR-PLG-03/04) and
// applies its sanction in the same transaction, reusing the status, freeze and
// level logic the admin already has. A human decides: nothing here is automatic.
func (s *Service) AdminRecordAffiliateViolation(ctx context.Context, actorID, affiliatorID string, in AdminViolationInput) (*model.AffiliateViolation, error) {
	affID, err := parseUUID(affiliatorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	description := strings.TrimSpace(in.Description)
	if !affiliateViolationCategories[in.Category] || !affiliateViolationSanctions[in.Sanction] || description == "" {
		return nil, ErrAffiliateViolationInvalid
	}

	var (
		suspendUntil *time.Time
		lock         *time.Time
		commissions  []uuid.UUID
	)
	switch in.Sanction {
	case "suspend":
		if in.Days == nil || *in.Days < 1 {
			return nil, ErrAffiliateInvalidDays
		}
		until, _ := affiliateSuspendUntil(*in.Days)
		suspendUntil = &until
	case "balance_freeze":
		if in.Days == nil || *in.Days < 1 {
			return nil, ErrAffiliateInvalidDays
		}
	case "level_change":
		if lock, err = affiliateValidateLevelChange(in.Level, in.LockUntil); err != nil {
			return nil, err
		}
	case "commission_cancel":
		if len(in.CommissionIDs) == 0 {
			return nil, fmt.Errorf("%w: commission_ids", ErrAffiliateViolationInvalid)
		}
		for _, raw := range in.CommissionIDs {
			id, err := parseUUID(raw)
			if err != nil {
				return nil, ErrInvalidUUID
			}
			commissions = append(commissions, id)
		}
	}

	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if in.Sanction == "suspend" || in.Sanction == "deactivate" {
		target := map[string]string{"suspend": "suspended", "deactivate": "deactivated"}[in.Sanction]
		if err := affiliateCheckStatusChange(aff, target, in.Days); err != nil {
			return nil, err
		}
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin violation tx: %w", err)
	}
	defer tx.Rollback(ctx)

	violation := &model.AffiliateViolation{
		AffiliatorID:    aff.ID,
		Category:        in.Category,
		Description:     description,
		Sanction:        in.Sanction,
		SuspensionUntil: suspendUntil,
		DecidedBy:       actor,
	}
	if err := s.storeRepo.InsertAffiliateViolationTx(ctx, tx, violation); err != nil {
		return nil, fmt.Errorf("insert affiliate violation: %w", err)
	}

	meta := map[string]any{"affiliator_id": aff.ID, "category": in.Category, "sanction": in.Sanction}
	switch in.Sanction {
	case "suspend":
		err = s.applyAffiliateStatusTx(ctx, tx, actorID, aff, "suspended", description, in.Days)
	case "deactivate":
		err = s.applyAffiliateStatusTx(ctx, tx, actorID, aff, "deactivated", description, nil)
	case "balance_freeze":
		err = s.applyAffiliateFreezeTx(ctx, tx, actorID, aff.ID, *in.Days, description)
	case "level_change":
		err = s.applyAffiliateLevelTx(ctx, tx, actorID, aff, in.Level, lock, description)
	case "commission_cancel":
		var cancelled int
		cancelled, err = s.cancelAffiliateCommissionsTx(ctx, tx, actor, aff, commissions, description)
		meta["commissions_cancelled"] = cancelled
	}
	if err != nil {
		return nil, err
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_violation", violation.ID.String(), affiliateAuditViolation, meta); err != nil {
		return nil, fmt.Errorf("write violation audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit violation: %w", err)
	}
	return violation, nil
}

// cancelAffiliateCommissionsTx cancels the named commissions as a sanction. A
// pending one just stops; an available one also gets a negative violation
// adjustment so the credit already in the ledger is taken back. Anything else
// (already cancelled, held, paid) or an id that is not this affiliator's refuses
// the whole sanction rather than half-applying it.
func (s *Service) cancelAffiliateCommissionsTx(ctx context.Context, tx pgx.Tx, actor uuid.UUID, aff *model.Affiliator, ids []uuid.UUID, reason string) (int, error) {
	rows, err := s.storeRepo.GetAffiliateCommissionsByIDsTx(ctx, tx, aff.ID, ids)
	if err != nil {
		return 0, fmt.Errorf("lock commissions: %w", err)
	}
	unique := map[uuid.UUID]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	if len(rows) != len(unique) {
		return 0, fmt.Errorf("%w: commission_ids must all belong to the affiliator", ErrAffiliateViolationInvalid)
	}
	for _, c := range rows {
		switch c.Status {
		case "pending":
		case "available":
			adj := &model.AffiliateAdjustment{
				AffiliatorID: aff.ID,
				Category:     "violation",
				Amount:       -c.Amount,
				Reason:       reason,
				CreatedBy:    actor,
			}
			if err := s.storeRepo.InsertAffiliateAdjustmentTx(ctx, tx, adj); err != nil {
				return 0, fmt.Errorf("insert violation adjustment: %w", err)
			}
			if err := s.storeRepo.InsertAffiliateLedgerEntryTx(ctx, tx, &model.AffiliateLedgerEntry{
				AffiliatorID: aff.ID,
				EntryType:    "adjustment",
				Amount:       -c.Amount,
				AdjustmentID: &adj.ID,
				Note:         &reason,
				ActorID:      &actor,
			}); err != nil {
				return 0, fmt.Errorf("insert violation ledger entry: %w", err)
			}
		default:
			return 0, fmt.Errorf("%w: commission %s is %s", ErrAffiliateViolationInvalid, c.ID, c.Status)
		}
		if ok, err := s.storeRepo.SetCommissionStatusFromTx(ctx, tx, c.ID, c.Status, "cancelled"); err != nil {
			return 0, fmt.Errorf("cancel commission: %w", err)
		} else if !ok {
			return 0, fmt.Errorf("%w: commission %s changed", ErrAffiliateViolationInvalid, c.ID)
		}
	}
	return len(rows), nil
}
