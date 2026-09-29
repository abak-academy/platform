package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// claimsDigitalTargets reports whether digital lines bought by this user are
// protected by per-target claims. School and other non-student buyers keep
// their existing behaviour.
func (s *Service) claimsDigitalTargets(ctx context.Context, buyerID uuid.UUID) (bool, error) {
	buyer, err := s.repo.GetUserByID(ctx, buyerID.String())
	if err != nil {
		return false, err
	}
	return buyer != nil && buyer.Role == RoleStudent, nil
}

// resolveDigitalTargets expands digital lines into the courses or exams their
// products link to right now. Physical lines contribute nothing.
func (s *Service) resolveDigitalTargets(ctx context.Context, tx pgx.Tx, items []model.OrderItem) ([]model.DigitalTarget, error) {
	var targets []model.DigitalTarget
	for _, item := range items {
		if isPhysicalType(item.ProductType) {
			continue
		}
		ids, err := s.storeRepo.ListProductDigitalTargets(ctx, tx, item.ProductType, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("resolve targets of product %s: %w", item.ProductID, err)
		}
		for _, id := range ids {
			targets = append(targets, model.DigitalTarget{
				OrderItemID: item.ID,
				ProductID:   item.ProductID,
				Kind:        item.ProductType,
				TargetID:    id,
			})
		}
	}
	return targets, nil
}

// repeatedDigitalTargets flags every occurrence of a target after its first,
// so two cart lines reaching the same course or exam are caught even when
// they are different SKUs.
func repeatedDigitalTargets(targets []model.DigitalTarget) []model.DigitalTargetConflict {
	seen := make(map[model.DigitalTargetKey]bool, len(targets))
	var repeated []model.DigitalTargetConflict
	for _, t := range targets {
		if seen[t.Key()] {
			repeated = append(repeated, model.DigitalTargetConflict{
				ProductID: t.ProductID, Kind: t.Kind, TargetID: t.TargetID, Reason: model.DigitalConflictDuplicateInCart,
			})
			continue
		}
		seen[t.Key()] = true
	}
	return repeated
}

func (s *Service) unavailableDigitalTargets(ctx context.Context, tx pgx.Tx, studentID uuid.UUID, targets []model.DigitalTarget) ([]model.DigitalTargetConflict, error) {
	unavailable, err := s.storeRepo.FindUnavailableDigitalTargets(ctx, tx, studentID, targets)
	if err != nil {
		return nil, err
	}
	var conflicts []model.DigitalTargetConflict
	for _, t := range targets {
		if reason, ok := unavailable[t.Key()]; ok {
			conflicts = append(conflicts, model.DigitalTargetConflict{
				ProductID: t.ProductID, Kind: t.Kind, TargetID: t.TargetID, Reason: reason,
			})
		}
	}
	return conflicts, nil
}

// checkDigitalCandidate is the cart-time early warning for adding one product:
// checkout stays authoritative, since a cart reserves nothing.
func (s *Service) checkDigitalCandidate(ctx context.Context, studentID uuid.UUID, existing []model.OrderItem, candidate model.OrderItem) error {
	claims, err := s.claimsDigitalTargets(ctx, studentID)
	if err != nil || !claims {
		return err
	}
	candidateTargets, err := s.resolveDigitalTargets(ctx, nil, []model.OrderItem{candidate})
	if err != nil {
		return err
	}
	inCart, err := s.resolveDigitalTargets(ctx, nil, existing)
	if err != nil {
		return err
	}
	var conflicts []model.DigitalTargetConflict
	for _, c := range repeatedDigitalTargets(append(inCart, candidateTargets...)) {
		if c.ProductID == candidate.ProductID {
			conflicts = append(conflicts, c)
		}
	}
	unavailable, err := s.unavailableDigitalTargets(ctx, nil, studentID, candidateTargets)
	if err != nil {
		return err
	}
	conflicts = append(conflicts, unavailable...)
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}
	return nil
}

// reserveDigitalTargets validates and claims every digital target of the order
// inside the checkout transaction. Any conflict rejects the whole order, so the
// caller's rollback also undoes physical stock changes.
func (s *Service) reserveDigitalTargets(ctx context.Context, tx pgx.Tx, order model.Order) error {
	targets, err := s.resolveDigitalTargets(ctx, tx, order.Items)
	if err != nil {
		return err
	}
	conflicts := repeatedDigitalTargets(targets)
	unavailable, err := s.unavailableDigitalTargets(ctx, tx, order.StudentID, targets)
	if err != nil {
		return err
	}
	conflicts = append(conflicts, unavailable...)
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}

	lost, err := s.storeRepo.ClaimDigitalTargetsTx(ctx, tx, order.ID, order.StudentID, targets)
	if err != nil {
		return err
	}
	for _, t := range lost {
		conflicts = append(conflicts, model.DigitalTargetConflict{
			ProductID: t.ProductID, Kind: t.Kind, TargetID: t.TargetID, Reason: model.DigitalConflictClaimed,
		})
	}
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}
	return nil
}
