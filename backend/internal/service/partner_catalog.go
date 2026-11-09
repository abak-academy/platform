package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const productAuditPartnerAccess = "product.partner_access_set"

// PartnerVoucherInput is the partner_voucher object of the promo admin API.
type PartnerVoucherInput struct {
	Enabled   bool     `json:"enabled"`
	IDs       []string `json:"ids"`
	QuotaUnit string   `json:"quota_unit"`
}

// PromoQuotaInsufficientError carries how many uses a promo has left. It
// satisfies errors.Is(err, ErrPromoQuotaInsufficient).
type PromoQuotaInsufficientError struct {
	Remaining int
}

func (e *PromoQuotaInsufficientError) Error() string {
	return fmt.Sprintf("promo has %d uses left, fewer than the order needs", e.Remaining)
}

func (e *PromoQuotaInsufficientError) Is(target error) bool {
	return target == ErrPromoQuotaInsufficient
}

// validatePartnerIDs checks that every id is an account in the Mitra program
// and returns them lower-cased and without duplicates.
func (s *Service) validatePartnerIDs(ctx context.Context, ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id, err := parseUUID(raw)
		if err != nil {
			return nil, ErrInvalidUUID
		}
		key := id.String()
		if slices.Contains(out, key) {
			continue
		}
		access, err := s.storeRepo.GetAffiliatorAccess(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get affiliator access: %w", err)
		}
		if access == nil || !access.Programs.HasPartner() {
			return nil, fmt.Errorf("%w: %s is not a Mitra", ErrValidation, key)
		}
		out = append(out, key)
	}
	return out, nil
}

// validatePartnerVoucher normalizes a Mitra voucher setting in place. The quota
// unit has no default: the admin has to choose what max_uses counts.
func (s *Service) validatePartnerVoucher(ctx context.Context, v *model.PartnerVoucher) error {
	if v.QuotaUnit != model.PartnerQuotaPerOrder && v.QuotaUnit != model.PartnerQuotaPerStudent {
		return fmt.Errorf("%w: quota_unit must be order or student", ErrValidation)
	}
	ids, err := s.validatePartnerIDs(ctx, v.IDs)
	if err != nil {
		return err
	}
	v.IDs = ids
	return nil
}

// PartnerPromoResult is a Mitra voucher applied to an order: the discount and
// how many uses of the voucher the order takes.
type PartnerPromoResult struct {
	PromoValidation
	Uses int
}

// ValidatePartnerPromo applies a Mitra voucher to an order of seats seats at
// unitPrice each. Only a voucher made for Mitra, and for this one, applies. The
// quota unit decides what max_uses and the discount cap count: whole orders, or
// each student.
func (s *Service) ValidatePartnerPromo(ctx context.Context, partnerID, code string, unitPrice float64, seats int) (PartnerPromoResult, error) {
	promo, err := s.storeRepo.GetPromoByCode(ctx, code)
	if err != nil {
		return PartnerPromoResult{}, err
	}
	return evaluatePartnerPromo(promo, partnerID, unitPrice, seats, time.Now())
}

func evaluatePartnerPromo(promo model.PromoCode, partnerID string, unitPrice float64, seats int, now time.Time) (PartnerPromoResult, error) {
	if promo.Code == "" || promo.Partner == nil || seats < 1 {
		return PartnerPromoResult{}, ErrInvalidPromo
	}
	if len(promo.Partner.IDs) > 0 && !slices.Contains(promo.Partner.IDs, partnerID) {
		return PartnerPromoResult{}, ErrInvalidPromo
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(now) {
		return PartnerPromoResult{}, ErrInvalidPromo
	}

	perStudent := promo.Partner.QuotaUnit == model.PartnerQuotaPerStudent
	uses := 1
	if perStudent {
		uses = seats
	}
	if promo.MaxUses != nil {
		remaining := *promo.MaxUses - promo.UsedCount
		if remaining <= 0 {
			return PartnerPromoResult{}, ErrInvalidPromo
		}
		if remaining < uses {
			return PartnerPromoResult{}, &PromoQuotaInsufficientError{Remaining: remaining}
		}
	}

	subtotal := unitPrice * float64(seats)
	if promo.MinOrderAmount != nil && subtotal < *promo.MinOrderAmount {
		return PartnerPromoResult{}, ErrPromoMinOrder
	}

	var discount float64
	if perStudent {
		var perUnit float64
		switch {
		case promo.DiscountPercent != nil:
			perUnit = unitPrice * (*promo.DiscountPercent / 100)
			if promo.MaxDiscountAmount != nil && perUnit > *promo.MaxDiscountAmount {
				perUnit = *promo.MaxDiscountAmount
			}
		case promo.DiscountAmount != nil:
			perUnit = math.Min(*promo.DiscountAmount, unitPrice)
		}
		discount = perUnit * float64(seats)
	} else {
		switch {
		case promo.DiscountPercent != nil:
			discount = subtotal * (*promo.DiscountPercent / 100)
			if promo.MaxDiscountAmount != nil && discount > *promo.MaxDiscountAmount {
				discount = *promo.MaxDiscountAmount
			}
		case promo.DiscountAmount != nil:
			discount = math.Min(*promo.DiscountAmount, subtotal)
		}
	}
	return PartnerPromoResult{
		PromoValidation: PromoValidation{PromoID: promo.ID, Code: promo.Code, Discount: discount, Total: subtotal - discount},
		Uses:            uses,
	}, nil
}

// promoUses is how many uses of its promo an order takes: one, or one per seat
// for a Mitra voucher that counts students.
func (s *Service) promoUses(ctx context.Context, order model.Order) (int, error) {
	if order.PromoCodeID == nil {
		return 0, nil
	}
	promo, err := s.storeRepo.GetPromoByID(ctx, *order.PromoCodeID)
	if err != nil {
		return 0, err
	}
	if promo.Partner != nil && promo.Partner.QuotaUnit == model.PartnerQuotaPerStudent {
		seats := 0
		for _, item := range order.Items {
			seats += item.Qty
		}
		return seats, nil
	}
	return 1, nil
}

// consumePromoTx takes the promo's uses for the order inside the checkout
// transaction. A promo with fewer uses left than the order needs fails the
// checkout, so a racing checkout cannot pass max_uses.
func (s *Service) consumePromoTx(ctx context.Context, tx pgx.Tx, order model.Order) error {
	if order.PromoCodeID == nil {
		return nil
	}
	uses, err := s.promoUses(ctx, order)
	if err != nil {
		return err
	}
	if err := s.storeRepo.IncrementPromoUsesTx(ctx, tx, *order.PromoCodeID, uses); err != nil {
		if errors.Is(err, repository.ErrPromoQuotaExceeded) {
			return ErrInvalidPromo
		}
		return err
	}
	return nil
}

// ListPartnerVouchers lists the Mitra vouchers a Mitra can use now.
func (s *Service) ListPartnerVouchers(ctx context.Context, partnerID string) ([]model.PromoCode, error) {
	return s.storeRepo.ListPartnerPromos(ctx, partnerID)
}

// AdminGetProductPartnerAccess returns a product's Mitra access, "none" when
// nothing has been set.
func (s *Service) AdminGetProductPartnerAccess(ctx context.Context, productID string) (*model.PartnerAccess, error) {
	id, err := parseUUID(productID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if _, err := s.storeRepo.GetProductByID(ctx, id.String()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return s.currentPartnerAccess(ctx, id)
}

func (s *Service) currentPartnerAccess(ctx context.Context, id uuid.UUID) (*model.PartnerAccess, error) {
	access, err := s.storeRepo.GetProductPartnerAccess(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get product partner access: %w", err)
	}
	if access == nil {
		return &model.PartnerAccess{Scope: model.PartnerScopeNone, IDs: []string{}}, nil
	}
	return access, nil
}

// AdminSetProductPartnerAccess sets which Mitra may see and order a product.
// A product starts with none; "selected" needs at least one Mitra.
func (s *Service) AdminSetProductPartnerAccess(ctx context.Context, actorID, productID, scope string, ids []string) (*model.PartnerAccess, error) {
	id, err := parseUUID(productID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if _, err := s.storeRepo.GetProductByID(ctx, id.String()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	next := &model.PartnerAccess{Scope: scope, IDs: []string{}}
	switch scope {
	case model.PartnerScopeNone, model.PartnerScopeAll:
	case model.PartnerScopeSelected:
		if next.IDs, err = s.validatePartnerIDs(ctx, ids); err != nil {
			return nil, err
		}
		if len(next.IDs) == 0 {
			return nil, fmt.Errorf("%w: choose at least one Mitra", ErrValidation)
		}
	default:
		return nil, fmt.Errorf("%w: scope must be none, all or selected", ErrValidation)
	}

	before, err := s.currentPartnerAccess(ctx, id)
	if err != nil {
		return nil, err
	}
	stored := next
	if scope == model.PartnerScopeNone {
		stored = nil
	}
	if err := s.storeRepo.SetProductPartnerAccess(ctx, id, stored); err != nil {
		return nil, fmt.Errorf("set product partner access: %w", err)
	}
	actor := actorID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &actor, "product", id.String(), productAuditPartnerAccess, map[string]any{
		"from": before,
		"to":   next,
	}); err != nil {
		return nil, fmt.Errorf("write partner access audit: %w", err)
	}
	return next, nil
}

// ListPartnerProducts lists the exam and course products a Mitra may order:
// published, inside their availability window, exam products with an open exam,
// and opened to this Mitra by the admin.
func (s *Service) ListPartnerProducts(ctx context.Context, partnerID string) ([]model.Product, error) {
	out := []model.Product{}
	for _, typ := range []string{"exam", "course"} {
		cursor := ""
		for {
			products, next, err := s.storeRepo.ListProducts(ctx, repository.ProductFilter{
				Type:        typ,
				Status:      "published",
				VisibleOnly: true,
				PartnerID:   partnerID,
				Limit:       100,
				Cursor:      cursor,
			})
			if err != nil {
				return nil, fmt.Errorf("list partner products: %w", err)
			}
			out = append(out, products...)
			if next == "" {
				break
			}
			cursor = next
		}
	}
	return out, nil
}
