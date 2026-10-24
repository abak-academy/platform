package service

import (
	"context"
	"fmt"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// settleAffiliateRefundTx settles the refunded order's affiliate commissions
// inside the refund's own transaction (BR-KOM-11): pending → cancelled, an
// available/paid commission gets one negative refund_deduction ledger entry,
// and held commissions stay held (BR-PLG-05). The saldo may go negative.
func (s *Service) settleAffiliateRefundTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, actorID string) error {
	commissions, err := s.storeRepo.GetCommissionsByOrderTx(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("get affiliate commissions by order: %w", err)
	}
	if len(commissions) == 0 {
		return nil
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return err
	}
	note := fmt.Sprintf("refund order %s", orderID)
	for _, c := range commissions {
		switch c.Status {
		case "pending":
			if err := s.storeRepo.UpdateCommissionStatusTx(ctx, tx, c.ID, "cancelled"); err != nil {
				return fmt.Errorf("cancel affiliate commission: %w", err)
			}
		case "available", "paid":
			if err := s.storeRepo.InsertRefundDeductionTx(ctx, tx, &model.AffiliateLedgerEntry{
				AffiliatorID: c.AffiliatorID,
				EntryType:    "refund_deduction",
				Amount:       -c.Amount,
				CommissionID: &c.ID,
				Note:         &note,
				ActorID:      &actor,
			}); err != nil {
				return fmt.Errorf("insert refund deduction: %w", err)
			}
		}
	}
	return nil
}
