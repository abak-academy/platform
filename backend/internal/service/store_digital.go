package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

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
	live, err := s.storeRepo.ListLiveDigitalTargets(ctx, tx, studentID, exceptOrderID)
	if err != nil {
		return nil, err
	}
	unavailable := make(map[model.DigitalTargetKey]string, len(live))
	for _, l := range live {
		key := model.DigitalTargetKey{Kind: l.Kind, TargetID: l.TargetID}
		if l.Reason == model.LiveTargetOwned {
			unavailable[key] = model.DigitalConflictOwned
		} else if unavailable[key] == "" {
			unavailable[key] = model.DigitalConflictClaimed
		}
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
// checkout stays authoritative, since a cart is not a reservation.
func (s *Service) checkDigitalCandidate(ctx context.Context, studentID uuid.UUID, existing []model.OrderItem, candidate model.OrderItem) error {
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

// checkDigitalTargets refuses an order whose digital lines grant nothing,
// repeat a target, or reach a target the student owns or is buying in another
// live order. The order's own lines never block it, so retry passes too. Any
// conflict rejects the whole order, so the caller's rollback also undoes
// physical stock changes.
func (s *Service) checkDigitalTargets(ctx context.Context, tx pgx.Tx, order model.Order) error {
	targets, err := s.resolveDigitalTargets(ctx, tx, order.Items)
	if err != nil {
		return err
	}
	if err := requireTargetsForDigitalLines(order.Items, targets); err != nil {
		return err
	}
	conflicts := repeatedDigitalTargets(targets)
	unavailable, err := s.unavailableDigitalTargets(ctx, tx, order.StudentID, order.ID, targets)
	if err != nil {
		return err
	}
	conflicts = append(conflicts, unavailable...)
	if len(conflicts) > 0 {
		return &DigitalTargetConflictError{Conflicts: conflicts}
	}
	return nil
}

// recheckDigitalTargets re-runs the guard for an order that already checked
// out, before a new payment is created for it. Bulk orders are left to their
// own flow.
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
	participants, err := s.storeRepo.GetOrderParticipantsTx(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	if len(participants) > 0 {
		return nil
	}
	return s.checkDigitalTargets(ctx, tx, order)
}
