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
func (s *Service) claimsDigitalTargets(ctx context.Context, tx pgx.Tx, buyerID uuid.UUID) (bool, error) {
	if tx != nil {
		role, err := s.storeRepo.GetUserRoleTx(ctx, tx, buyerID)
		return role == RoleStudent, err
	}
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

func (s *Service) unavailableDigitalTargets(ctx context.Context, tx pgx.Tx, studentID, exceptOrderID uuid.UUID, targets []model.DigitalTarget) ([]model.DigitalTargetConflict, error) {
	unavailable, err := s.storeRepo.FindUnavailableDigitalTargets(ctx, tx, studentID, targets, exceptOrderID)
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
	claims, err := s.claimsDigitalTargets(ctx, nil, studentID)
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
	unavailable, err := s.unavailableDigitalTargets(ctx, nil, studentID, uuid.Nil, candidateTargets)
	if err != nil {
		return err
	}
	conflicts = append(conflicts, unavailable...)
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}
	return nil
}

// requireTargetsForDigitalLines refuses an order with a digital line that
// resolves to nothing, since the customer would pay for a line that grants
// nothing.
func requireTargetsForDigitalLines(items []model.OrderItem, targets []model.DigitalTarget) error {
	resolved := make(map[uuid.UUID]bool, len(targets))
	for _, t := range targets {
		resolved[t.OrderItemID] = true
	}
	for _, item := range items {
		if !isPhysicalType(item.ProductType) && !resolved[item.ID] {
			return ErrDigitalProductNoTargets
		}
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
	if err := requireTargetsForDigitalLines(order.Items, targets); err != nil {
		return err
	}
	conflicts := repeatedDigitalTargets(targets)
	unavailable, err := s.unavailableDigitalTargets(ctx, tx, order.StudentID, uuid.Nil, targets)
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

// recheckDigitalTargets re-runs the ownership and claim guard for an order that
// already checked out, before a new payment is created for it. The order's own
// claims are its reservation, not a conflict; what can still block it is access
// granted since checkout, or another order's claim on a target the order never
// claimed. An order without a snapshot (placed before claims existed, possibly
// backfilled with only some of its targets) is fulfilled from live links, so all
// of them are checked, not just the ones it owns.
func (s *Service) recheckDigitalTargets(ctx context.Context, tx pgx.Tx, order model.Order) error {
	digital := false
	for _, item := range order.Items {
		if !isPhysicalType(item.ProductType) {
			digital = true
		}
	}
	if !digital {
		return nil
	}
	participants, err := s.storeRepo.GetOrderParticipants(ctx, order.ID)
	if err != nil {
		return err
	}
	if len(participants) > 0 {
		return nil
	}
	claims, err := s.claimsDigitalTargets(ctx, tx, order.StudentID)
	if err != nil || !claims {
		return err
	}

	snapshot, err := s.storeRepo.GetOrderDigitalSnapshot(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	var targets []model.DigitalTarget
	if snapshot.Taken {
		targets, err = s.storeRepo.ListOrderDigitalTargetsTx(ctx, tx, order.ID)
		if err != nil {
			return err
		}
	}
	if len(targets) == 0 {
		targets, err = s.resolveDigitalTargets(ctx, tx, order.Items)
		if err != nil {
			return err
		}
		if err := requireTargetsForDigitalLines(order.Items, targets); err != nil {
			return err
		}
	}
	conflicts, err := s.unavailableDigitalTargets(ctx, tx, order.StudentID, order.ID, targets)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}
	return nil
}
