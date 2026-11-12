package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func (s *Service) AdminListProductGroupings(ctx context.Context) ([]model.ProductGrouping, error) {
	return s.storeRepo.ListProductGroupings(ctx, false)
}

// ListPublishedProductGroupings is the student-facing landing list.
func (s *Service) ListPublishedProductGroupings(ctx context.Context) ([]model.ProductGrouping, error) {
	return s.storeRepo.ListProductGroupings(ctx, true)
}

func (s *Service) AdminCreateProductGrouping(ctx context.Context, g model.ProductGrouping) (model.ProductGrouping, error) {
	if g.Name == "" {
		return model.ProductGrouping{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	g.Status = model.ProductGroupingStatusDraft
	if err := s.storeRepo.CreateProductGrouping(ctx, &g); err != nil {
		return model.ProductGrouping{}, err
	}
	return g, nil
}

// AdminGetProductGrouping returns a grouping and every one of its member
// products, regardless of status, for the admin management page.
func (s *Service) AdminGetProductGrouping(ctx context.Context, id string) (model.ProductGrouping, []model.Product, error) {
	g, err := s.storeRepo.GetProductGroupingByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, nil, err
	}
	products, err := s.storeRepo.ListProductsByGrouping(ctx, id, false)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	return *g, products, nil
}

// GetPublishedProductGrouping is the student-facing detail read: a draft
// grouping, or one that does not exist, is reported the same way (not found),
// so a guessed id cannot distinguish the two.
func (s *Service) GetPublishedProductGrouping(ctx context.Context, id string) (model.ProductGrouping, []model.Product, error) {
	g, err := s.storeRepo.GetProductGroupingByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, nil, err
	}
	if g.Status != model.ProductGroupingStatusPublished {
		return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
	}
	products, err := s.storeRepo.ListProductsByGrouping(ctx, id, true)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	return *g, products, nil
}

// AdminUpdateProductGrouping overlays only the fields the caller set; nil
// leaves a field unchanged.
func (s *Service) AdminUpdateProductGrouping(ctx context.Context, id string, name, description, status *string) (model.ProductGrouping, error) {
	existing, err := s.storeRepo.GetProductGroupingByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, err
	}
	if name != nil {
		if *name == "" {
			return model.ProductGrouping{}, fmt.Errorf("%w: name is required", ErrValidation)
		}
		existing.Name = *name
	}
	if description != nil {
		existing.Description = *description
	}
	if status != nil {
		if *status != model.ProductGroupingStatusDraft && *status != model.ProductGroupingStatusPublished {
			return model.ProductGrouping{}, fmt.Errorf("%w: status must be draft or published", ErrValidation)
		}
		existing.Status = *status
	}
	if err := s.storeRepo.UpdateProductGrouping(ctx, id, existing); err != nil {
		return model.ProductGrouping{}, err
	}
	return *existing, nil
}

func (s *Service) AdminDeleteProductGrouping(ctx context.Context, id string) error {
	if _, err := s.storeRepo.GetProductGroupingByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProductGroupingNotFound
		}
		return err
	}
	// ON DELETE SET NULL ungroups member products; it does not remove them.
	return s.storeRepo.DeleteProductGrouping(ctx, id)
}

func (s *Service) AdminReorderProductGroupings(ctx context.Context, groupingIDs []string) error {
	ids, err := parseUUIDs(groupingIDs)
	if err != nil {
		return err
	}
	return s.storeRepo.ReorderProductGroupings(ctx, ids)
}

// AdminSetGroupingProducts replaces a grouping's member products in one
// transaction: every product named gets this grouping with grouping_order
// set to its position in productIDs; every product previously in the
// grouping but left out is unassigned.
func (s *Service) AdminSetGroupingProducts(ctx context.Context, groupingID string, productIDs []string) (model.ProductGrouping, []model.Product, error) {
	gID, err := parseUUID(groupingID)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if _, err := s.storeRepo.GetProductGroupingByID(ctx, groupingID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, nil, err
	}
	ids, err := parseUUIDs(productIDs)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.ReplaceGroupingProducts(ctx, tx, gID, ids); err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ProductGrouping{}, nil, err
	}
	return s.AdminGetProductGrouping(ctx, groupingID)
}

func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := parseUUID(id)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, u)
	}
	return parsed, nil
}
