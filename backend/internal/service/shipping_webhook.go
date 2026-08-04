package service

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// biteshipWebhookPayload is Biteship's "something changed" ping (FR-C-12).
// Biteship does not sign its webhooks — the static shared header compared
// in HandleShippingWebhook is the only authentication mechanism it has — so
// nothing here is trusted as the shipment's state. OrderID only locates our
// row. Status and any timestamp are intentionally never read from the body:
// the re-fetch below is what actually lands, including occurred_at (FR-C-12).
type biteshipWebhookPayload struct {
	OrderID string `json:"order_id"`
}

// getWebhookSecret reads and decrypts biteship_webhook_secret from
// system_config. Returns "" when unset — callers must treat that as a
// rejection, never as "skip verification" (FR-C-11).
func (s *Service) getWebhookSecret(ctx context.Context) (string, error) {
	rows, err := s.storeRepo.ListSystemConfig(ctx)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Key == "biteship_webhook_secret" && row.IsSecret && row.Value != "" {
			return decryptConfigValue(s.cfg.ConfigEncryptionKey, row.Value)
		}
	}
	return "", nil
}

// HandleShippingWebhook verifies X-Biteship-Signature (passed in as
// signature) against the configured biteship_webhook_secret using a
// constant-time compare. An unset or empty configured secret always
// rejects — comparing two zero-length slices with
// subtle.ConstantTimeCompare returns 1 (equal), which is the exact fail-open
// shape this repo shipped once (841cd84), so the empty case is refused
// before any compare runs (FR-C-11).
//
// On a match, the body is treated only as a ping: the authoritative
// shipment status comes from a GetOrder re-fetch, never from the request
// body (FR-C-12), and orders.status is never advanced (FR-C-15).
func (s *Service) HandleShippingWebhook(ctx context.Context, payload []byte, signature string) error {
	secret, err := s.getWebhookSecret(ctx)
	if err != nil {
		return err
	}
	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(signature)) != 1 {
		return ErrInvalidSignature
	}

	// Biteship validates the URL when the webhook is installed by POSTing an
	// empty body, and refuses to install one that answers anything but 2xx.
	// An empty body names no order and changes no state, so it is answered as
	// a no-op — deliberately *after* the signature check above, so this never
	// becomes an unauthenticated 200.
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}

	var ping biteshipWebhookPayload
	if err := json.Unmarshal(payload, &ping); err != nil {
		return fmt.Errorf("parse shipping webhook payload: %w", err)
	}
	if ping.OrderID == "" {
		return ErrOrderNotFound
	}

	order, err := s.storeRepo.GetOrderByBiteshipOrderID(ctx, ping.OrderID)
	if err != nil {
		return err
	}
	if order.ID == uuid.Nil {
		return ErrOrderNotFound
	}

	return s.syncShipmentFromBiteship(ctx, order, ping.OrderID)
}

// syncShipmentFromBiteship re-fetches an order's authoritative state from
// Biteship and lands it. Shared by the webhook, the admin refresh button and
// the cancel action so all three converge on identical rows — an admin who
// presses refresh must not end up with a different record than the webhook
// would have written.
func (s *Service) syncShipmentFromBiteship(ctx context.Context, order model.Order, biteshipOrderID string) error {
	shipment, err := s.logisticsClient().GetOrder(ctx, biteshipOrderID)
	if err != nil {
		return fmt.Errorf("re-fetch shipment status: %w", err)
	}

	if err := s.storeRepo.SetShipmentStatus(ctx, order.ID, shipment.Status); err != nil {
		return err
	}

	// Guarded rather than unconditional: most events carry status only, and
	// writing an empty re-fetched waybill would blank a good resi on every
	// status change. Kept out of the SQL so the rule is visible and testable
	// here instead of hiding in a COALESCE.
	if shipment.WaybillID != "" {
		if err := s.storeRepo.SetTrackingNumber(ctx, order.ID, shipment.WaybillID); err != nil {
			return err
		}
	}

	// Same guard as the waybill, for the same reason: most events carry no
	// link, and writing an empty one would wipe a good tracking page.
	if shipment.TrackingURL != "" {
		if err := s.storeRepo.SetTrackingURL(ctx, order.ID, shipment.TrackingURL); err != nil {
			return err
		}
	}

	// occurredAt must be deterministic per order so a replay lands on the same
	// order_shipment_events row (FR-C-13) — time.Now() here reproduces the
	// exact bug this fixes: every retry gets a fresh value and the
	// UNIQUE(order_id, status, occurred_at) guard never fires. order.ShippedAt
	// is stable once set and is the only way an order gets a biteship_order_id
	// to be looked up by in the first place; order.CreatedAt is the
	// belt-and-braces fallback if it's somehow unset.
	//
	// It is also what makes the refresh button safe to press repeatedly.
	occurredAt := shipment.StatusUpdatedAt
	if occurredAt.IsZero() {
		if order.ShippedAt != nil {
			occurredAt = *order.ShippedAt
		} else {
			occurredAt = order.CreatedAt
		}
	}

	event := model.OrderShipmentEvent{
		OrderID:    order.ID,
		Status:     shipment.Status,
		OccurredAt: occurredAt,
	}
	if shipment.WaybillID != "" {
		event.CourierWaybillID = &shipment.WaybillID
	}
	if shipment.CourierDriverName != "" {
		event.CourierDriverName = &shipment.CourierDriverName
	}
	return s.storeRepo.InsertShipmentEvent(ctx, event)
}
