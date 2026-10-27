package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// PromoLine is one order line as the promo rules see it.
type PromoLine struct {
	ProductID string
	Qty       int
	Amount    float64
	// Unscoped marks a line standing in for a whole subtotal whose products and
	// quantities are unknown; promos that depend on either cannot apply to it.
	Unscoped bool
}

// evaluatePromo applies every promo rule to the order lines. Rules that restrict
// the promo to products count only the matching lines towards the minimums and
// the discount.
func evaluatePromo(promo model.PromoCode, role string, lines []PromoLine, shippingCost float64, now time.Time) (PromoValidation, error) {
	if promo.Code == "" {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.AffiliatorOnly && role != RoleAffiliator {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.StartsAt != nil && promo.StartsAt.After(now) {
		return PromoValidation{}, ErrPromoNotStarted
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(now) {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.MaxUses != nil && promo.UsedCount >= *promo.MaxUses {
		return PromoValidation{}, ErrInvalidPromo
	}

	scoped := len(promo.ProductIDs) > 0
	var qty int
	var subtotal, orderSubtotal float64
	for _, l := range lines {
		orderSubtotal += l.Amount
		if l.Unscoped && (scoped || promo.MinQty != nil) {
			return PromoValidation{}, ErrInvalidPromo
		}
		if scoped && !slices.Contains(promo.ProductIDs, strings.ToLower(l.ProductID)) {
			continue
		}
		qty += l.Qty
		subtotal += l.Amount
	}
	if scoped && subtotal == 0 {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.MinQty != nil && qty < *promo.MinQty {
		return PromoValidation{}, ErrPromoMinQty
	}
	if promo.MinOrderAmount != nil && subtotal < *promo.MinOrderAmount {
		return PromoValidation{}, ErrPromoMinOrder
	}

	var discount float64
	if promo.DiscountPercent != nil {
		discount = subtotal * (*promo.DiscountPercent / 100)
		if promo.MaxDiscountAmount != nil && discount > *promo.MaxDiscountAmount {
			discount = *promo.MaxDiscountAmount
		}
	} else if promo.DiscountAmount != nil {
		discount = *promo.DiscountAmount
		if discount > subtotal {
			discount = subtotal
		}
	}

	return PromoValidation{PromoID: promo.ID, Code: promo.Code, Discount: discount, Total: orderSubtotal - discount + shippingCost}, nil
}

// ValidatePromoLines validates a promo against an order's lines for the given actor role.
func (s *Service) ValidatePromoLines(ctx context.Context, code, role string, lines []PromoLine, shippingCost float64) (PromoValidation, error) {
	promo, err := s.storeRepo.GetPromoByCode(ctx, code)
	if err != nil {
		return PromoValidation{}, err
	}
	v, err := evaluatePromo(promo, role, lines, shippingCost, time.Now())
	if err != nil {
		return PromoValidation{}, err
	}
	v.Code = code
	return v, nil
}

func promoLinesFromOrder(items []model.OrderItem) []PromoLine {
	lines := make([]PromoLine, len(items))
	for i, it := range items {
		lines[i] = PromoLine{ProductID: it.ProductID.String(), Qty: it.Qty, Amount: it.Jumlah}
	}
	return lines
}

// validatePromoInput rejects a promo definition that could never apply or would apply ambiguously.
func (s *Service) validatePromoInput(ctx context.Context, p model.PromoCode) (model.PromoCode, error) {
	if (p.DiscountPercent == nil) == (p.DiscountAmount == nil) {
		return p, fmt.Errorf("%w: set either discount_percent or discount_amount", ErrValidation)
	}
	if p.DiscountPercent != nil && (*p.DiscountPercent <= 0 || *p.DiscountPercent > 100) {
		return p, fmt.Errorf("%w: discount_percent must be between 0 and 100", ErrValidation)
	}
	if p.DiscountAmount != nil && *p.DiscountAmount <= 0 {
		return p, fmt.Errorf("%w: discount_amount must be positive", ErrValidation)
	}
	if p.MinQty != nil && *p.MinQty < 0 {
		return p, fmt.Errorf("%w: min_qty must not be negative", ErrValidation)
	}
	if p.StartsAt != nil && p.ExpiresAt != nil && !p.StartsAt.Before(*p.ExpiresAt) {
		return p, fmt.Errorf("%w: starts_at must be before expires_at", ErrValidation)
	}
	if p.AffiliatorOnly && p.IsPublic {
		return p, fmt.Errorf("%w: an affiliator-only promo cannot be public", ErrValidation)
	}
	ids := make([]string, 0, len(p.ProductIDs))
	for _, id := range p.ProductIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if _, err := parseUUID(id); err != nil {
			return p, ErrInvalidUUID
		}
		if slices.Contains(ids, id) {
			continue
		}
		if _, err := s.storeRepo.GetProductByID(ctx, id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return p, fmt.Errorf("%w: product %s not found", ErrValidation, id)
			}
			return p, err
		}
		ids = append(ids, id)
	}
	p.ProductIDs = ids
	return p, nil
}

// ListActiveAffiliatorPromos returns the affiliator-only promos an affiliator can use now.
func (s *Service) ListActiveAffiliatorPromos(ctx context.Context) ([]model.PromoCode, error) {
	return s.storeRepo.ListActiveAffiliatorPromos(ctx)
}
