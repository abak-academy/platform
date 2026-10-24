package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/jackc/pgx/v5"
)

// CreateCommissionsForOrder creates the pending affiliate commissions for a
// paid order's items inside the caller's transaction (the worker's OrderPaid
// step). Every gate here is a no-op, never an error: flag off, no active
// binding, sanctioned affiliator, zero percentage and a fully-discounted item
// simply skip so fulfilment flows are never entangled with commission policy.
func (s *Service) CreateCommissionsForOrder(ctx context.Context, tx pgx.Tx, order model.Order, items []model.OrderItem) error {
	if !s.affiliateEnabled(ctx) {
		return nil
	}
	binding, err := s.storeRepo.GetActiveBindingByStudent(ctx, tx, order.StudentID.String())
	if err != nil {
		return fmt.Errorf("get active binding: %w", err)
	}
	if binding == nil {
		return nil
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, tx, binding.AffiliatorID)
	if err != nil {
		return fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil || aff.Status != affiliateAffiliatorStatusActive || affiliateSanctionActive(aff, time.Now()) {
		return nil
	}
	_, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return fmt.Errorf("load affiliate effective rules: %w", err)
	}
	rule, ok := rules[aff.Level]
	if !ok {
		return nil
	}
	for _, item := range items {
		if err := s.createCommissionForItem(ctx, tx, aff, order, item, rule); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) createCommissionForItem(ctx context.Context, tx pgx.Tx, aff *model.Affiliator, order model.Order, item model.OrderItem, rule AffiliateEffectiveRule) error {
	override, err := s.storeRepo.LoadProductOverride(ctx, item.ProductID)
	if err != nil {
		return fmt.Errorf("load product override: %w", err)
	}
	pct := resolveAffiliateCommissionPct(override, rule, item.ProductType)
	if pct == 0 {
		return nil // BR-KOM-02: medal and unset categories earn nothing
	}
	base := affiliateItemBase(order, item)
	if base <= 0 {
		return nil // BR-KOM-03: a fully discounted or free item earns nothing
	}
	commission := &model.AffiliateCommission{
		AffiliatorID:  aff.ID,
		StudentID:     order.StudentID,
		OrderID:       order.ID,
		OrderItemID:   item.ID,
		ProductType:   item.ProductType,
		BaseAmount:    base,
		Percentage:    pct,
		LevelSnapshot: aff.Level,
		Amount:        math.Floor(base * pct / 100),
		Status:        "pending",
		WaitUntil:     affiliateWaitUntil(item.ProductType, order.PaidAt, rule),
		PaidAt:        order.PaidAt,
	}
	if err := s.storeRepo.InsertAffiliateCommissionTx(ctx, tx, commission); err != nil {
		// A replayed OrderPaid event hits the unique order_item_id: that is the
		// idempotency contract (BR-KOM-13), not a failure.
		if errors.Is(err, repository.ErrAffiliateCommissionExists) {
			return nil
		}
		return fmt.Errorf("insert affiliate commission: %w", err)
	}
	return nil
}

// affiliateItemBase allocates the order discount proportionally across items.
// orders.subtotal is SUM(order_item.jumlah) and never includes shipping_cost
// (the cart recompute in repository/order.go), so the divisor is the item
// subtotal itself.
func affiliateItemBase(order model.Order, item model.OrderItem) float64 {
	if order.Discount == 0 || order.Subtotal <= 0 {
		return item.Jumlah
	}
	base := item.Jumlah - order.Discount*(item.Jumlah/order.Subtotal)
	if base < 0 {
		return 0
	}
	return base
}

// affiliateWaitUntil dates a commission's availability: digital products wait
// from paid_at, physical ones stay NULL until the daily job derives it from
// completed_at.
func affiliateWaitUntil(productType string, paidAt *time.Time, rule AffiliateEffectiveRule) *time.Time {
	if paidAt == nil {
		return nil
	}
	switch productType {
	case "course", "exam":
		wait := paidAt.AddDate(0, 0, rule.DigitalWaitDays)
		return &wait
	default:
		return nil
	}
}

// affiliateSanctionActive is BR-ATR-12: a suspended or frozen affiliator
// earns nothing while the sanction lasts.
func affiliateSanctionActive(aff *model.Affiliator, now time.Time) bool {
	return (aff.SuspendedUntil != nil && aff.SuspendedUntil.After(now)) ||
		(aff.FrozenUntil != nil && aff.FrozenUntil.After(now))
}
