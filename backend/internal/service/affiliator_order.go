package service

import (
	"context"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// AffiliatorOrderPreview is the response for PreviewAffiliatorOrder.
type AffiliatorOrderPreview struct {
	NetNewCount int                 `json:"net_new_count"`
	Excluded    []BulkOrderExcluded `json:"excluded,omitempty"`
	UnitPrice   float64             `json:"unit_price"`
	Subtotal    float64             `json:"subtotal"`
	Discount    float64             `json:"discount"`
	Total       float64             `json:"total"`
	PromoCode   string              `json:"promo_code,omitempty"`
}

type affiliatorOrderPlan struct {
	product   model.Product
	productID uuid.UUID
	netNew    []uuid.UUID
	excluded  []BulkOrderExcluded
	subtotal  float64
	promo     *PromoValidation
}

// ListAffiliatorProducts returns the exam and course products an affiliator can order.
func (s *Service) ListAffiliatorProducts(ctx context.Context) ([]model.Product, error) {
	out := []model.Product{}
	for _, typ := range []string{"exam", "course"} {
		cursor := ""
		for {
			products, next, err := s.ListProducts(ctx, repository.ProductFilter{
				Type:              typ,
				Status:            "published",
				VisibleOnly:       true,
				OrderableExamOnly: typ == "exam",
				Limit:             100,
				Cursor:            cursor,
			}, RoleStudent)
			if err != nil {
				return nil, err
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

func (s *Service) planAffiliatorOrder(ctx context.Context, affiliatorID, schoolID, productID string, selector ParticipantSelector, promoCode string) (affiliatorOrderPlan, error) {
	allowed, err := s.AffiliatorSchoolIDs(ctx, affiliatorID)
	if err != nil {
		return affiliatorOrderPlan{}, err
	}
	schoolID = strings.ToLower(schoolID)
	if schoolID == "" || !slices.Contains(allowed, schoolID) {
		return affiliatorOrderPlan{}, ErrAffiliatorSchoolNotAllowed
	}

	pUUID, err := parseUUID(productID)
	if err != nil {
		return affiliatorOrderPlan{}, ErrInvalidUUID
	}
	product, err := s.GetProduct(ctx, productID, RoleStudent)
	if err != nil {
		return affiliatorOrderPlan{}, err
	}
	if product.Type != "exam" && product.Type != "course" {
		return affiliatorOrderPlan{}, ErrAffiliatorProductType
	}

	participantIDs, err := s.ResolveSchoolParticipantSet(ctx, schoolID, selector)
	if err != nil {
		return affiliatorOrderPlan{}, err
	}
	owned, err := s.studentsOwningProduct(ctx, product.Type, pUUID, participantIDs)
	if err != nil {
		return affiliatorOrderPlan{}, err
	}

	plan := affiliatorOrderPlan{product: product, productID: pUUID}
	for _, pid := range participantIDs {
		if !owned[pid] {
			plan.netNew = append(plan.netNew, pid)
			continue
		}
		name := ""
		if st, _ := s.repo.GetUserByID(ctx, pid.String()); st != nil {
			name = st.Name
		}
		plan.excluded = append(plan.excluded, BulkOrderExcluded{StudentID: pid.String(), Name: name, Reason: "already_owned"})
	}
	plan.subtotal = float64(product.Price) * float64(len(plan.netNew))

	if promoCode != "" && len(plan.netNew) > 0 {
		v, err := s.ValidatePromoLines(ctx, promoCode, RoleAffiliator, []PromoLine{{
			ProductID: product.ID, Qty: len(plan.netNew), Amount: plan.subtotal,
		}}, 0)
		if err != nil {
			return affiliatorOrderPlan{}, err
		}
		plan.promo = &v
	}
	return plan, nil
}

// studentsOwningProduct returns the students who already hold every target
// (exam registration or course enrollment) the product grants.
func (s *Service) studentsOwningProduct(ctx context.Context, productType string, productID uuid.UUID, studentIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	var perTarget [][]uuid.UUID
	switch productType {
	case "exam":
		exams, err := s.storeRepo.GetExamsByProductID(ctx, productID)
		if err != nil {
			return nil, err
		}
		if len(exams) == 0 {
			return nil, ErrExamNotOrderable
		}
		for _, e := range exams {
			ids, err := s.storeRepo.FilterAlreadyRegistered(ctx, e.ID, studentIDs)
			if err != nil {
				return nil, err
			}
			perTarget = append(perTarget, ids)
		}
	case "course":
		courses, err := s.storeRepo.GetCoursesByProductID(ctx, productID)
		if err != nil {
			return nil, err
		}
		if len(courses) == 0 {
			return nil, ErrProductNotFound
		}
		for _, c := range courses {
			ids, err := s.storeRepo.FilterAlreadyEnrolled(ctx, c.ID, studentIDs)
			if err != nil {
				return nil, err
			}
			perTarget = append(perTarget, ids)
		}
	}

	owned := make(map[uuid.UUID]bool, len(studentIDs))
	for _, id := range studentIDs {
		all := true
		for _, ids := range perTarget {
			if !slices.Contains(ids, id) {
				all = false
				break
			}
		}
		owned[id] = all
	}
	return owned, nil
}

// PreviewAffiliatorOrder resolves who would be charged and what the promo does, without creating anything.
func (s *Service) PreviewAffiliatorOrder(ctx context.Context, affiliatorID, schoolID, productID string, selector ParticipantSelector, promoCode string) (AffiliatorOrderPreview, error) {
	plan, err := s.planAffiliatorOrder(ctx, affiliatorID, schoolID, productID, selector, promoCode)
	if err != nil {
		return AffiliatorOrderPreview{}, err
	}
	preview := AffiliatorOrderPreview{
		NetNewCount: len(plan.netNew),
		Excluded:    plan.excluded,
		UnitPrice:   float64(plan.product.Price),
		Subtotal:    plan.subtotal,
		Total:       plan.subtotal,
	}
	if plan.promo != nil {
		preview.Discount = plan.promo.Discount
		preview.Total = plan.promo.Total
		preview.PromoCode = plan.promo.Code
	}
	return preview, nil
}

// CreateAffiliatorOrder re-resolves the participants and promo (never trusting a
// cached preview) and creates the order with its participants in one transaction.
// A previous unfinished bulk order of the same affiliator is cancelled so the
// one-open-cart-per-buyer rule does not block a fresh one.
func (s *Service) CreateAffiliatorOrder(ctx context.Context, affiliatorID, schoolID, productID string, selector ParticipantSelector, promoCode string) (model.Order, error) {
	plan, err := s.planAffiliatorOrder(ctx, affiliatorID, schoolID, productID, selector, promoCode)
	if err != nil {
		return model.Order{}, err
	}
	if len(plan.netNew) == 0 {
		return model.Order{}, ErrZeroNetNewParticipants
	}
	buyerID, err := uuid.Parse(affiliatorID)
	if err != nil {
		return model.Order{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Order{}, err
	}
	defer tx.Rollback(ctx)

	stale, err := s.storeRepo.GetCartByStudentID(ctx, buyerID)
	if err != nil {
		return model.Order{}, err
	}
	if stale.ID != uuid.Nil {
		participants, err := s.storeRepo.GetOrderParticipants(ctx, stale.ID)
		if err != nil {
			return model.Order{}, err
		}
		if len(participants) == 0 {
			return model.Order{}, ErrOpenCartExists
		}
		if err := s.storeRepo.SetOrderStatus(ctx, tx, stale.ID, "cancelled", "superseded"); err != nil {
			return model.Order{}, err
		}
	}

	order, err := s.storeRepo.CreateOrderTx(ctx, tx, buyerID)
	if err != nil {
		return model.Order{}, err
	}
	item := model.OrderItem{
		ProductID:   plan.productID,
		ProductType: plan.product.Type,
		Name:        plan.product.Name,
		UnitPrice:   float64(plan.product.Price),
		Qty:         len(plan.netNew),
	}
	if err := s.storeRepo.InsertOrderItemTx(ctx, tx, order.ID, item); err != nil {
		return model.Order{}, err
	}
	if err := s.storeRepo.InsertOrderParticipantsTx(ctx, tx, order.ID, plan.netNew); err != nil {
		return model.Order{}, err
	}
	if plan.promo != nil {
		if err := s.storeRepo.SetOrderPromoTx(ctx, tx, order.ID, plan.promo.PromoID, plan.promo.Discount); err != nil {
			return model.Order{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Order{}, err
	}
	return s.storeRepo.GetOrderByID(ctx, order.ID)
}

// CheckoutAffiliatorOrder re-validates the order's promo against the live promo
// rules, then runs the normal checkout. An order whose promo has since expired,
// run out or changed its discount is refused before any payment exists.
func (s *Service) CheckoutAffiliatorOrder(ctx context.Context, affiliatorID, orderID, idempotencyKey string) (CheckoutResult, error) {
	oID, err := parseUUID(orderID)
	if err != nil {
		return CheckoutResult{}, ErrOrderNotFound
	}
	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if order.ID == uuid.Nil || order.StudentID.String() != affiliatorID {
		return CheckoutResult{}, ErrOrderNotFound
	}

	if order.Status == "cart" && order.PromoCodeID != nil {
		promo, err := s.storeRepo.GetPromoByID(ctx, *order.PromoCodeID)
		if err != nil {
			return CheckoutResult{}, err
		}
		v, err := evaluatePromo(promo, RoleAffiliator, promoLinesFromOrder(order.Items), order.ShippingCost, time.Now())
		if err != nil {
			return CheckoutResult{}, err
		}
		if v.Discount != order.Discount {
			return CheckoutResult{}, ErrInvalidPromo
		}
	}
	return s.Checkout(ctx, affiliatorID, orderID, idempotencyKey)
}
