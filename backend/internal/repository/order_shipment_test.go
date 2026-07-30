package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
)

// newOrderShipmentTestPool spins up an ephemeral Postgres container with all
// migrations applied (including 0048), and returns a connected pool.
func newOrderShipmentTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, infra.RunMigrations(ctx, dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// seedShipmentTestOrder creates a student and a bare order for shipment
// tests to attach events to.
func seedShipmentTestOrder(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var studentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"shipment-"+uuid.NewString()+"@test.local", "student", "Budi Test",
	).Scan(&studentID))

	var orderID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'paid', 0, 0, 0, 0) RETURNING id`,
		studentID,
	).Scan(&orderID))

	return orderID
}

// Compile-time check: *Repository must implement all new shipment methods.
var _ interface {
	SetShippedBiteship(context.Context, uuid.UUID, string, string) error
	SetShippedManual(context.Context, uuid.UUID, string) error
	GetOrderByBiteshipOrderID(context.Context, string) (model.Order, error)
	SetShipmentStatus(context.Context, uuid.UUID, string) error
	InsertShipmentEvent(context.Context, model.OrderShipmentEvent) error
	ListShipmentEvents(context.Context, uuid.UUID) ([]model.OrderShipmentEvent, error)
} = (*Repository)(nil)

func TestSetShippedBiteship(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)

	require.NoError(t, repo.SetShippedBiteship(ctx, orderID, "WB-BITESHIP-1", "biteship-order-xyz"))

	order, err := repo.GetOrderByID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "shipped", order.Status)
	require.Equal(t, "WB-BITESHIP-1", order.TrackingNumber)
	require.NotNil(t, order.BiteshipOrderID)
	require.Equal(t, "biteship-order-xyz", *order.BiteshipOrderID)
	require.NotNil(t, order.WaybillSource)
	require.Equal(t, "biteship", *order.WaybillSource)
	require.NotNil(t, order.ShippedAt)
}

func TestSetShippedManual(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)

	require.NoError(t, repo.SetShippedManual(ctx, orderID, "WB-MANUAL-1"))

	order, err := repo.GetOrderByID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "shipped", order.Status)
	require.Equal(t, "WB-MANUAL-1", order.TrackingNumber)
	require.Nil(t, order.BiteshipOrderID, "manual path must not set biteship_order_id")
	require.NotNil(t, order.WaybillSource)
	require.Equal(t, "manual", *order.WaybillSource)
}

func TestGetOrderByBiteshipOrderID(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)
	require.NoError(t, repo.SetShippedBiteship(ctx, orderID, "WB-1", "biteship-lookup-id"))

	found, err := repo.GetOrderByBiteshipOrderID(ctx, "biteship-lookup-id")
	require.NoError(t, err)
	require.Equal(t, orderID, found.ID)

	// FR-C-14: an unmatched Biteship order id must not error — the caller
	// (the webhook handler) distinguishes "no row" via a zero-value ID and
	// responds 404 itself.
	notFound, err := repo.GetOrderByBiteshipOrderID(ctx, "no-such-biteship-order")
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, notFound.ID)
}

func TestSetShipmentStatus_doesNotAdvanceOrderStatus(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)
	require.NoError(t, repo.SetShippedBiteship(ctx, orderID, "WB-1", "biteship-status-id"))

	require.NoError(t, repo.SetShipmentStatus(ctx, orderID, "delivered"))

	order, err := repo.GetOrderByID(ctx, orderID)
	require.NoError(t, err)
	require.NotNil(t, order.ShipmentStatus)
	require.Equal(t, "delivered", *order.ShipmentStatus)
	// FR-C-15: shipment_status advances but orders.status must stay
	// unchanged — completion remains a manual admin action.
	require.Equal(t, "shipped", order.Status)
}

func TestInsertShipmentEvent_replayIsInert(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)
	occurredAt := time.Now().UTC().Truncate(time.Microsecond)
	waybillID := "WB-EVENT-1"
	driverName := "Budi Driver Test"

	event := model.OrderShipmentEvent{
		OrderID:           orderID,
		Status:            "confirmed",
		CourierWaybillID:  &waybillID,
		CourierDriverName: &driverName,
		OccurredAt:        occurredAt,
	}

	// FR-C-13: the same event inserted twice (a replayed webhook) must be
	// inert — no error, and the row count must stay 1, never 2.
	require.NoError(t, repo.InsertShipmentEvent(ctx, event))
	require.NoError(t, repo.InsertShipmentEvent(ctx, event))

	events, err := repo.ListShipmentEvents(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, events, 1, "a replayed identical shipment event must not create a duplicate row")
	require.Equal(t, "confirmed", events[0].Status)
	require.NotNil(t, events[0].CourierWaybillID)
	require.Equal(t, waybillID, *events[0].CourierWaybillID)
}

func TestListShipmentEvents_orderedByOccurredAt(t *testing.T) {
	pool := newOrderShipmentTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	orderID := seedShipmentTestOrder(t, pool)

	base := time.Now().UTC().Truncate(time.Microsecond)
	later := base.Add(1 * time.Hour)

	// Insert the later event first to prove ordering comes from occurred_at,
	// not insertion order.
	require.NoError(t, repo.InsertShipmentEvent(ctx, model.OrderShipmentEvent{
		OrderID: orderID, Status: "allocated", OccurredAt: later,
	}))
	require.NoError(t, repo.InsertShipmentEvent(ctx, model.OrderShipmentEvent{
		OrderID: orderID, Status: "confirmed", OccurredAt: base,
	}))

	events, err := repo.ListShipmentEvents(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "confirmed", events[0].Status, "earlier occurred_at must sort first")
	require.Equal(t, "allocated", events[1].Status)
}
