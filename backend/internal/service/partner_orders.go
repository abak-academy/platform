package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// ParticipantAlreadyOwnsError lists the students who already hold what the
// order would give them. It satisfies errors.Is(err, ErrParticipantAlreadyOwns).
type ParticipantAlreadyOwnsError struct {
	Students []repository.PartnerOrderStudent
}

func (e *ParticipantAlreadyOwnsError) Error() string {
	return fmt.Sprintf("%d participant(s) already own this product", len(e.Students))
}

func (e *ParticipantAlreadyOwnsError) Is(target error) bool {
	return target == ErrParticipantAlreadyOwns
}

// PartnerOrderPreview is what a Mitra sees before creating an order: who is
// charged, who is left out, and what the voucher does.
type PartnerOrderPreview struct {
	NetNewCount int                              `json:"net_new_count"`
	Excluded    []BulkOrderExcluded              `json:"excluded,omitempty"`
	Students    []repository.PartnerOrderStudent `json:"students"`
	UnitPrice   float64                          `json:"unit_price"`
	Subtotal    float64                          `json:"subtotal"`
	Discount    float64                          `json:"discount"`
	Total       float64                          `json:"total"`
	PromoCode   string                           `json:"promo_code,omitempty"`
}

// PartnerOrderDetail is an order with the students it grants access to.
type PartnerOrderDetail struct {
	model.Order
	PromoCode string                           `json:"promo_code,omitempty"`
	Students  []repository.PartnerOrderStudent `json:"students"`
}

type partnerOrderPlan struct {
	product  model.Product
	netNew   []uuid.UUID
	students []repository.PartnerOrderStudent
	excluded []BulkOrderExcluded
	subtotal float64
	promo    *PartnerPromoResult
}

// requirePartnerCanBuy is the service-side twin of PartnerGate for the write
// paths: a Mitra whose account is suspended, or whose program is switched off,
// cannot start or pay an order, whichever route reached here.
func (s *Service) requirePartnerCanBuy(ctx context.Context, partnerID string) (*AffiliatorProgramStates, error) {
	states, err := s.AffiliatorProgramStates(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	if states.Partner != ProgramActive {
		return nil, ErrAffiliatorNotPartner
	}
	if !states.AccountActive {
		return nil, ErrAccountDeactivated
	}
	on, err := s.partnerEnabledErr(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, ErrPartnerProgramOff
	}
	return states, nil
}

// partnerProduct returns a product a Mitra may order: published and visible to
// students, an exam or course, and opened to this Mitra by the admin.
func (s *Service) partnerProduct(ctx context.Context, partnerID, productID string) (model.Product, uuid.UUID, error) {
	pID, err := parseUUID(productID)
	if err != nil {
		return model.Product{}, uuid.Nil, ErrInvalidUUID
	}
	product, err := s.GetProduct(ctx, productID, RoleStudent)
	if err != nil {
		return model.Product{}, uuid.Nil, err
	}
	if product.Type != "exam" && product.Type != "course" {
		return model.Product{}, uuid.Nil, ErrPartnerProductType
	}
	open, err := s.storeRepo.ProductOpenToPartner(ctx, pID, partnerID)
	if err != nil {
		return model.Product{}, uuid.Nil, fmt.Errorf("check partner product access: %w", err)
	}
	if !open {
		return model.Product{}, uuid.Nil, ErrPartnerProductNotAllowed
	}
	return product, pID, nil
}

// studentsOwningProduct returns the students who already hold every target the
// product grants: an exam registration, or a course enrollment.
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
		all := len(perTarget) > 0
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

// planPartnerOrder resolves an order request from scratch: nothing a Mitra
// sends, or saw in an earlier preview, is trusted.
func (s *Service) planPartnerOrder(ctx context.Context, partnerID, productID string, studentIDs []string, promoCode string) (partnerOrderPlan, error) {
	if len(studentIDs) == 0 {
		return partnerOrderPlan{}, ErrEmptySelector
	}
	states, err := s.requirePartnerCanBuy(ctx, partnerID)
	if err != nil {
		return partnerOrderPlan{}, err
	}

	ids := make([]uuid.UUID, 0, len(studentIDs))
	for _, raw := range studentIDs {
		id, err := parseUUID(raw)
		if err != nil {
			return partnerOrderPlan{}, ErrInvalidUUID
		}
		if slices.Contains(ids, id) {
			return partnerOrderPlan{}, fmt.Errorf("%w: %s", ErrDuplicateParticipant, raw)
		}
		ids = append(ids, id)
	}

	product, pID, err := s.partnerProduct(ctx, partnerID, productID)
	if err != nil {
		return partnerOrderPlan{}, err
	}

	students, err := s.storeRepo.ListPartnerOrderStudents(ctx, ids, states.PartnerSchools, partnerID)
	if err != nil {
		return partnerOrderPlan{}, fmt.Errorf("list partner order students: %w", err)
	}
	if len(students) != len(ids) {
		return partnerOrderPlan{}, ErrPartnerStudentNotAllowed
	}
	byID := make(map[uuid.UUID]repository.PartnerOrderStudent, len(students))
	for _, st := range students {
		byID[st.ID] = st
	}

	owned, err := s.studentsOwningProduct(ctx, product.Type, pID, ids)
	if err != nil {
		return partnerOrderPlan{}, err
	}
	plan := partnerOrderPlan{product: product}
	for _, id := range ids {
		if owned[id] {
			plan.excluded = append(plan.excluded, BulkOrderExcluded{StudentID: id.String(), Name: byID[id].Name, Reason: "already_owned"})
			continue
		}
		plan.netNew = append(plan.netNew, id)
		plan.students = append(plan.students, byID[id])
	}
	plan.subtotal = float64(product.Price) * float64(len(plan.netNew))

	if promoCode != "" && len(plan.netNew) > 0 {
		res, err := s.ValidatePartnerPromo(ctx, partnerID, promoCode, float64(product.Price), len(plan.netNew))
		if err != nil {
			return partnerOrderPlan{}, err
		}
		plan.promo = &res
	}
	return plan, nil
}

// PreviewPartnerOrder shows who would be charged and what the voucher does,
// without creating anything.
func (s *Service) PreviewPartnerOrder(ctx context.Context, partnerID, productID string, studentIDs []string, promoCode string) (PartnerOrderPreview, error) {
	plan, err := s.planPartnerOrder(ctx, partnerID, productID, studentIDs, promoCode)
	if err != nil {
		return PartnerOrderPreview{}, err
	}
	preview := PartnerOrderPreview{
		NetNewCount: len(plan.netNew),
		Excluded:    plan.excluded,
		Students:    plan.students,
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

// CreatePartnerOrder resolves the request again and creates the order with its
// participants and voucher in one transaction. A Mitra has one unpaid order
// open at a time: an earlier one that was never checked out is cancelled.
func (s *Service) CreatePartnerOrder(ctx context.Context, partnerID, productID string, studentIDs []string, promoCode string) (model.Order, error) {
	plan, err := s.planPartnerOrder(ctx, partnerID, productID, studentIDs, promoCode)
	if err != nil {
		return model.Order{}, err
	}
	if len(plan.netNew) == 0 {
		return model.Order{}, ErrZeroNetNewParticipants
	}
	buyerID, err := uuid.Parse(partnerID)
	if err != nil {
		return model.Order{}, ErrInvalidUUID
	}
	productUUID, err := uuid.Parse(plan.product.ID)
	if err != nil {
		return model.Order{}, ErrInvalidUUID
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
		if err := s.storeRepo.SetOrderStatus(ctx, tx, stale.ID, "cancelled", "superseded"); err != nil {
			return model.Order{}, err
		}
	}

	order, err := s.storeRepo.CreateOrderTx(ctx, tx, buyerID)
	if err != nil {
		return model.Order{}, err
	}
	item := model.OrderItem{
		ProductID:   productUUID,
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

// partnerOwnedOrder loads an order and refuses it unless the Mitra is its buyer.
func (s *Service) partnerOwnedOrder(ctx context.Context, partnerID, orderID string) (model.Order, error) {
	oID, err := parseUUID(orderID)
	if err != nil {
		return model.Order{}, ErrOrderNotFound
	}
	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return model.Order{}, err
	}
	if order.ID == uuid.Nil || order.StudentID.String() != partnerID {
		return model.Order{}, ErrOrderNotFound
	}
	return order, nil
}

// CheckoutPartnerOrder checks the order against today's rules, not the rules of
// the day it was created, and only then runs the normal checkout: the Mitra is
// still allowed and the program still on, the product is still open to this
// Mitra, every student is still in scope and does not own the product yet, and
// the voucher is still valid and still gives the discount the order carries.
// Anything that changed is refused before a payment exists.
func (s *Service) CheckoutPartnerOrder(ctx context.Context, partnerID, orderID, key string) (CheckoutResult, error) {
	order, err := s.partnerOwnedOrder(ctx, partnerID, orderID)
	if err != nil {
		return CheckoutResult{}, err
	}
	states, err := s.requirePartnerCanBuy(ctx, partnerID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if order.Status == "cart" {
		if err := s.revalidatePartnerOrder(ctx, partnerID, states, order); err != nil {
			return CheckoutResult{}, err
		}
	}
	return s.Checkout(ctx, partnerID, orderID, key, "")
}

func (s *Service) revalidatePartnerOrder(ctx context.Context, partnerID string, states *AffiliatorProgramStates, order model.Order) error {
	if len(order.Items) != 1 {
		return ErrOrderNotEditable
	}
	item := order.Items[0]
	product, pID, err := s.partnerProduct(ctx, partnerID, item.ProductID.String())
	if err != nil {
		return err
	}

	participants, err := s.storeRepo.GetOrderParticipants(ctx, order.ID)
	if err != nil {
		return err
	}
	if len(participants) == 0 {
		return ErrOrderNotEditable
	}
	allowed, err := s.storeRepo.ListPartnerOrderStudents(ctx, participants, states.PartnerSchools, partnerID)
	if err != nil {
		return fmt.Errorf("list partner order students: %w", err)
	}
	if len(allowed) != len(participants) {
		return ErrPartnerStudentNotAllowed
	}

	owned, err := s.studentsOwningProduct(ctx, product.Type, pID, participants)
	if err != nil {
		return err
	}
	var owners []repository.PartnerOrderStudent
	for _, st := range allowed {
		if owned[st.ID] {
			owners = append(owners, st)
		}
	}
	if len(owners) > 0 {
		return &ParticipantAlreadyOwnsError{Students: owners}
	}

	if order.PromoCodeID != nil {
		promo, err := s.storeRepo.GetPromoByID(ctx, *order.PromoCodeID)
		if err != nil {
			return err
		}
		res, err := evaluatePartnerPromo(promo, partnerID, item.UnitPrice, item.Qty, time.Now())
		if err != nil {
			return err
		}
		if res.Discount != order.Discount {
			return ErrInvalidPromo
		}
	}
	return nil
}

// RetryPartnerOrder gives an order that was checked out a fresh payment session.
func (s *Service) RetryPartnerOrder(ctx context.Context, partnerID, orderID, key string) (CheckoutResult, error) {
	if _, err := s.partnerOwnedOrder(ctx, partnerID, orderID); err != nil {
		return CheckoutResult{}, err
	}
	if _, err := s.requirePartnerCanBuy(ctx, partnerID); err != nil {
		return CheckoutResult{}, err
	}
	return s.RetryPayment(ctx, partnerID, orderID, key)
}

// ListPartnerOrders lists one page of the Mitra's orders, unpaid ones included,
// narrowed by status, a search and a date range.
func (s *Service) ListPartnerOrders(ctx context.Context, partnerID string, filter repository.PartnerOrderFilter, page, perPage int) ([]model.Order, int, error) {
	pID, err := parseUUID(partnerID)
	if err != nil {
		return nil, 0, ErrInvalidUUID
	}
	filter.Q = strings.TrimSpace(filter.Q)
	orders, total, err := s.storeRepo.ListPartnerOrdersPage(ctx, pID, filter, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("list partner orders: %w", err)
	}
	return orders, total, nil
}

// GetPartnerOrder returns one of the Mitra's orders with the students it covers.
func (s *Service) GetPartnerOrder(ctx context.Context, partnerID, orderID string) (*PartnerOrderDetail, error) {
	order, err := s.partnerOwnedOrder(ctx, partnerID, orderID)
	if err != nil {
		return nil, err
	}
	students, err := s.storeRepo.ListOrderParticipantStudents(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("list order participants: %w", err)
	}
	detail := &PartnerOrderDetail{Order: order, Students: students}
	if order.PromoCodeID != nil {
		promo, err := s.storeRepo.GetPromoByID(ctx, *order.PromoCodeID)
		if err != nil {
			return nil, fmt.Errorf("get order promo: %w", err)
		}
		detail.PromoCode = promo.Code
	}
	return detail, nil
}

// PartnerDashboard is the home page of a Mitra: its counts and what it can buy with.
type PartnerDashboard struct {
	repository.PartnerDashboardCounts
	VouchersAvailable int `json:"vouchers_available"`
	ProductsOpen      int `json:"products_open"`
}

// GetPartnerDashboard gathers the counts of a Mitra's home page.
func (s *Service) GetPartnerDashboard(ctx context.Context, partnerID string) (*PartnerDashboard, error) {
	if _, err := parseUUID(partnerID); err != nil {
		return nil, ErrInvalidUUID
	}
	counts, err := s.storeRepo.GetPartnerDashboardCounts(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("get partner dashboard counts: %w", err)
	}
	vouchers, err := s.ListPartnerVouchers(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("list partner promos: %w", err)
	}
	products, err := s.ListPartnerProducts(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	return &PartnerDashboard{PartnerDashboardCounts: counts, VouchersAvailable: len(vouchers), ProductsOpen: len(products)}, nil
}
