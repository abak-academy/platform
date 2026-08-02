package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// Compile-time check: *Repository must implement all order methods.
var _ interface {
	MintCart(context.Context, uuid.UUID) (model.Order, bool, error)
	GetCartByStudentID(context.Context, uuid.UUID) (model.Order, error)
	GetOrderByID(context.Context, uuid.UUID) (model.Order, error)
	ListOrders(context.Context, OrderFilter) ([]model.Order, string, error)
	AddItem(context.Context, uuid.UUID, model.OrderItem, bool) error
	RemoveItem(context.Context, uuid.UUID, uuid.UUID, bool) error
	UpdateItemQty(context.Context, uuid.UUID, uuid.UUID, int, bool) error
	PatchCart(context.Context, uuid.UUID, OrderPatch) error
	SetOrderStatus(context.Context, pgx.Tx, uuid.UUID, string, string) error
	SetShipped(context.Context, uuid.UUID, string) error
	SetPaymentRef(context.Context, uuid.UUID, string, time.Time) error
	CheckoutOrder(context.Context, pgx.Tx, uuid.UUID) error
} = (*Repository)(nil)
