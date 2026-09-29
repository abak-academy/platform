package repository

import (
	"context"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

func (r *Repository) GetOrderDigitalSnapshot(ctx context.Context, orderID uuid.UUID) (model.OrderDigitalSnapshot, error) {
	return model.OrderDigitalSnapshot{}, nil
}
