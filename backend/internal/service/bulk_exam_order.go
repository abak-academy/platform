package service

import (
	"context"
	"errors"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// BulkOrderPreview is the response for PreviewBulkExamOrder.
type BulkOrderPreview struct {
	NetNewCount int                  `json:"net_new_count"`
	Excluded    []BulkOrderExcluded  `json:"excluded,omitempty"`
	UnitPrice   float64              `json:"unit_price"`
	Total       float64              `json:"total"`
}

// BulkOrderExcluded describes a student excluded from the bulk order.
type BulkOrderExcluded struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
}

// ListOrderableExams returns published exam-type products visible to the caller's
// role. Thin wrapper reusing ListProducts.
func (s *Service) ListOrderableExams(ctx context.Context, role string) ([]model.Product, error) {
	products, _, err := s.ListProducts(ctx, repository.ProductFilter{
		Type: "exam",
	}, role)
	return products, err
}

// PreviewBulkExamOrder resolves the participant set, excludes already-registered
// students, and returns a preview with net-new count, excluded list, unit price,
// and total.
func (s *Service) PreviewBulkExamOrder(ctx context.Context, schoolID, examID string, selector ParticipantSelector) (BulkOrderPreview, error) {
	participantIDs, err := s.ResolveSchoolParticipantSet(ctx, schoolID, selector)
	if err != nil {
		return BulkOrderPreview{}, err
	}

	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return BulkOrderPreview{}, err
	}

	alreadyReg, err := s.storeRepo.FilterAlreadyRegistered(ctx, examUUID, participantIDs)
	if err != nil {
		return BulkOrderPreview{}, err
	}

	regSet := make(map[string]bool, len(alreadyReg))
	for _, id := range alreadyReg {
		regSet[id.String()] = true
	}

	var excluded []BulkOrderExcluded
	for _, pid := range participantIDs {
		if regSet[pid.String()] {
			student, _ := s.repo.GetUserByID(ctx, pid.String())
			name := ""
			if student != nil {
				name = student.Name
			}
			excluded = append(excluded, BulkOrderExcluded{
				StudentID: pid.String(),
				Name:      name,
				Reason:    "already_registered",
			})
		}
	}

	product, err := s.storeRepo.GetProductByExamID(ctx, examUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return BulkOrderPreview{}, ErrExamNotOrderable
		}
		return BulkOrderPreview{}, err
	}

	netNewCount := len(participantIDs) - len(alreadyReg)
	unitPrice := float64(product.Price)

	return BulkOrderPreview{
		NetNewCount: netNewCount,
		Excluded:    excluded,
		UnitPrice:   unitPrice,
		Total:       unitPrice * float64(netNewCount),
	}, nil
}

// CreateBulkExamOrder re-resolves the participant set and net-new subset (never
// trusts a client-cached preview number), creates an Order + OrderItem + participant
// rows in one transaction, and rejects zero net-new (FR-BULK-05).
func (s *Service) CreateBulkExamOrder(ctx context.Context, buyerAdminID, schoolID, examID string, selector ParticipantSelector) (model.Order, error) {
	participantIDs, err := s.ResolveSchoolParticipantSet(ctx, schoolID, selector)
	if err != nil {
		return model.Order{}, err
	}

	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return model.Order{}, err
	}

	alreadyReg, err := s.storeRepo.FilterAlreadyRegistered(ctx, examUUID, participantIDs)
	if err != nil {
		return model.Order{}, err
	}

	netNew := subRegistered(participantIDs, alreadyReg)
	if len(netNew) == 0 {
		return model.Order{}, ErrZeroNetNewParticipants
	}

	product, err := s.storeRepo.GetProductByExamID(ctx, examUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Order{}, ErrExamNotOrderable
		}
		return model.Order{}, err
	}

	buyerID, err := uuid.Parse(buyerAdminID)
	if err != nil {
		return model.Order{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Order{}, err
	}
	defer tx.Rollback(ctx)

	order, err := s.storeRepo.CreateOrderTx(ctx, tx, buyerID)
	if err != nil {
		return model.Order{}, err
	}

	item := model.OrderItem{
		ProductID:   uuid.MustParse(product.ID),
		ProductType: "exam",
		Name:        product.Name,
		UnitPrice:   float64(product.Price),
		Qty:         len(netNew),
	}
	if err := s.storeRepo.InsertOrderItemTx(ctx, tx, order.ID, item); err != nil {
		return model.Order{}, err
	}

	if err := s.storeRepo.InsertOrderParticipantsTx(ctx, tx, order.ID, netNew); err != nil {
		return model.Order{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Order{}, err
	}

	return s.storeRepo.GetOrderByID(ctx, order.ID)
}

// subRegistered returns participantIDs minus alreadyReg.
func subRegistered(participantIDs, alreadyReg []uuid.UUID) []uuid.UUID {
	regSet := make(map[string]struct{}, len(alreadyReg))
	for _, id := range alreadyReg {
		regSet[id.String()] = struct{}{}
	}
	netNew := make([]uuid.UUID, 0, len(participantIDs))
	for _, pid := range participantIDs {
		if _, ok := regSet[pid.String()]; !ok {
			netNew = append(netNew, pid)
		}
	}
	return netNew
}
