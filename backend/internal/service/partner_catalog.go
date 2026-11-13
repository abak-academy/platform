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
	Enabled     bool     `json:"enabled"`
	IDs         []string `json:"ids"`
	QuotaUnit   string   `json:"quota_unit"`
	MinStudents int      `json:"min_students"`
}

// PromoMinStudentsError carries how many students a voucher needs in one order.
// It satisfies errors.Is(err, ErrPromoMinStudents).
type PromoMinStudentsError struct {
	Min int
}

func (e *PromoMinStudentsError) Error() string {
	return fmt.Sprintf("promo needs at least %d students in one order", e.Min)
}

func (e *PromoMinStudentsError) Is(target error) bool {
	return target == ErrPromoMinStudents
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
	if v.MinStudents < 0 {
		return fmt.Errorf("%w: min_students cannot be negative", ErrValidation)
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

// promoSeatGroup is one product's own (unit price, net-new seat count) pair
// in a multi-product order. A single-product order is just one group — the
// math below is written to be exactly today's behavior when len(groups) == 1.
type promoSeatGroup struct {
	UnitPrice float64
	Seats     int
}

// ValidatePartnerPromo applies a Mitra voucher across every product group in
// an order (distinctStudents is the order's served-student count, used for
// min_students — it can differ from the sum of each group's seats, since one
// student may be net-new on more than one product). Only a voucher made for
// Mitra, and for this one, applies. The quota unit decides what max_uses and
// the discount cap count: the whole order once, or each student on each
// product they're net-new for.
func (s *Service) ValidatePartnerPromo(ctx context.Context, partnerID, code string, groups []promoSeatGroup, distinctStudents int) (PartnerPromoResult, error) {
	promo, err := s.storeRepo.GetPromoByCode(ctx, code)
	if err != nil {
		return PartnerPromoResult{}, err
	}
	return evaluatePartnerPromo(promo, partnerID, groups, distinctStudents, time.Now())
}

func evaluatePartnerPromo(promo model.PromoCode, partnerID string, groups []promoSeatGroup, distinctStudents int, now time.Time) (PartnerPromoResult, error) {
	if promo.Code == "" || promo.Partner == nil || distinctStudents < 1 || len(groups) == 0 {
		return PartnerPromoResult{}, ErrInvalidPromo
	}
	if len(promo.Partner.IDs) > 0 && !slices.Contains(promo.Partner.IDs, partnerID) {
		return PartnerPromoResult{}, ErrInvalidPromo
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(now) {
		return PartnerPromoResult{}, ErrInvalidPromo
	}

	if distinctStudents < promo.Partner.MinStudents {
		return PartnerPromoResult{}, &PromoMinStudentsError{Min: promo.Partner.MinStudents}
	}

	perStudent := promo.Partner.QuotaUnit == model.PartnerQuotaPerStudent

	var subtotal float64
	for _, g := range groups {
		subtotal += g.UnitPrice * float64(g.Seats)
	}

	uses := 1
	if perStudent {
		uses = 0
		for _, g := range groups {
			uses += g.Seats
		}
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

	if promo.MinOrderAmount != nil && subtotal < *promo.MinOrderAmount {
		return PartnerPromoResult{}, ErrPromoMinOrder
	}

	var discount float64
	if perStudent {
		// Per student-product pair: each product's own price and seat count
		// get their own discount-per-unit (capped per product), summed.
		for _, g := range groups {
			var perUnit float64
			switch {
			case promo.DiscountPercent != nil:
				perUnit = g.UnitPrice * (*promo.DiscountPercent / 100)
				if promo.MaxDiscountAmount != nil && perUnit > *promo.MaxDiscountAmount {
					perUnit = *promo.MaxDiscountAmount
				}
			case promo.DiscountAmount != nil:
				perUnit = math.Min(*promo.DiscountAmount, g.UnitPrice)
			}
			discount += perUnit * float64(g.Seats)
		}
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

// partnerCanOrder reports whether the Mitra may buy right now. A suspended
// account, a program that is off, or a registration not yet approved shows no
// vouchers or products; any other failure is an error, never "cannot".
func (s *Service) partnerCanOrder(ctx context.Context, partnerID string) (bool, error) {
	_, err := s.requirePartnerCanBuy(ctx, partnerID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrAffiliatorNotPartner), errors.Is(err, ErrAccountDeactivated), errors.Is(err, ErrPartnerProgramOff):
		return false, nil
	default:
		return false, err
	}
}

// ListPartnerVouchers lists the Mitra vouchers a Mitra can use now: none while
// the account cannot buy.
func (s *Service) ListPartnerVouchers(ctx context.Context, partnerID string) ([]model.PromoCode, error) {
	ok, err := s.partnerCanOrder(ctx, partnerID)
	if err != nil || !ok {
		return []model.PromoCode{}, err
	}
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
	if ok, err := s.partnerCanOrder(ctx, partnerID); err != nil || !ok {
		return out, err
	}
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
