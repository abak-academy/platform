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
// binding, sanctioned affiliator, zero commission value and a fully-discounted item
// simply skip so fulfilment flows are never entangled with commission policy.
func (s *Service) CreateCommissionsForOrder(ctx context.Context, tx pgx.Tx, order model.Order, items []model.OrderItem) error {
	enabled, err := s.affiliateEnabledErr(ctx)
	if err != nil {
		return err
	}
	if !enabled {
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
	if aff == nil || aff.Status != affiliateAffiliatorStatusActive {
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
	commissionRule := resolveAffiliateCommissionRule(override, rule, item.ProductType)
	if commissionRule.Value == 0 {
		return nil // BR-KOM-02: medal and unset categories earn nothing
	}
	base := affiliateItemBase(order, item)
	if base <= 0 {
		return nil // BR-KOM-03: a fully discounted or free item earns nothing
	}
	amount := affiliateCommissionAmount(base, item.Qty, commissionRule)
	if amount <= 0 {
		return nil
	}
	commission := &model.AffiliateCommission{
		AffiliatorID:    aff.ID,
		StudentID:       order.StudentID,
		OrderID:         order.ID,
		OrderItemID:     item.ID,
		ProductType:     item.ProductType,
		Quantity:        item.Qty,
		BaseAmount:      base,
		CommissionType:  commissionRule.Type,
		CommissionValue: commissionRule.Value,
		LevelSnapshot:   aff.Level,
		Amount:          amount,
		Status:          "pending",
		WaitUntil:       affiliateWaitUntil(item.ProductType, order.PaidAt, rule),
		PaidAt:          order.PaidAt,
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

// affiliateCommissionAmount is the whole-rupiah commission for one item. A
// percent rule floors base*value/100, rounding the product to a hundredth of a
// percent first so float error in a decimal value (7.35 is not exactly
// representable) cannot drop a rupiah. A fixed rule pays value per unit. Either
// way the commission never exceeds the item's discounted price.
func affiliateCommissionAmount(base float64, qty int, rule model.AffiliateCommissionRule) float64 {
	var amount float64
	switch rule.Type {
	case affiliateCommissionFixed:
		amount = rule.Value * float64(qty)
	default:
		amount = math.Floor(math.Round(base*rule.Value*100) / 10000)
	}
	return math.Min(amount, math.Floor(base))
}
