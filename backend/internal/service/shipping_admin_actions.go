package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// ErrNoBiteshipBooking is returned for an order that was never booked through
// Biteship — a manual-resi order, or one not shipped yet. There is nothing at
// Biteship to refresh or cancel, and asking about an empty order id would
// either 404 or address someone else's shipment.
var ErrNoBiteshipBooking = errors.New("order has no Biteship booking")

// AdminRefreshShipment pulls an order's current state from Biteship by hand.
//
// The webhook is the normal path, but it is a single unauthenticated delivery
// with no retry we control: if it is missed, misconfigured, or arrives while
// the API is down, the order's status simply stops moving with nothing to
// indicate why. This is the manual pull for that case.
//
// It writes through the same routine the webhook uses, so pressing it can
// never produce a record the webhook would not have produced — including
// staying inert on repeat presses (FR-C-13).
func (s *Service) AdminRefreshShipment(ctx context.Context, orderID string) error {
	order, biteshipOrderID, err := s.orderWithBiteshipBooking(ctx, orderID)
	if err != nil {
		return err
	}
	return s.syncShipmentFromBiteship(ctx, order, biteshipOrderID)
}

// AdminCancelShipment cancels the courier booking at Biteship, then lands
// whatever Biteship reports afterwards rather than assuming the cancellation
// took effect.
//
// This cancels a *shipment*, not an order: orders.status is deliberately left
// alone, and no money moves. Refund semantics are a separate, undecided
// question tracked on issue #72 — conflating them here would quietly cancel
// deliveries that were paid for.
func (s *Service) AdminCancelShipment(ctx context.Context, orderID, reason string) error {
	order, biteshipOrderID, err := s.orderWithBiteshipBooking(ctx, orderID)
	if err != nil {
		return err
	}

	if err := s.logisticsClient().CancelOrder(ctx, biteshipOrderID, reason); err != nil {
		return fmt.Errorf("cancel shipment: %w", err)
	}

	return s.syncShipmentFromBiteship(ctx, order, biteshipOrderID)
}

func (s *Service) orderWithBiteshipBooking(ctx context.Context, orderID string) (order model.Order, biteshipOrderID string, err error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return order, "", err
	}

	order, err = s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return order, "", err
	}
	if order.ID == uuid.Nil {
		return order, "", ErrOrderNotFound
	}
	if order.BiteshipOrderID == nil || *order.BiteshipOrderID == "" {
		return order, "", ErrNoBiteshipBooking
	}
	return order, *order.BiteshipOrderID, nil
}
