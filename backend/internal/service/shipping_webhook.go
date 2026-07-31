package service

import (
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

	shipment, err := s.logisticsClient().GetOrder(ctx, ping.OrderID)
	if err != nil {
		return fmt.Errorf("re-fetch shipment status: %w", err)
	}

	if err := s.storeRepo.SetShipmentStatus(ctx, order.ID, shipment.Status); err != nil {
		return err
	}

	// occurredAt must be deterministic per order so a replay lands on the same
	// order_shipment_events row (FR-C-13) — time.Now() here reproduces the
	// exact bug this fixes: every retry gets a fresh value and the
	// UNIQUE(order_id, status, occurred_at) guard never fires. order.ShippedAt
	// is stable once set and is the only way an order gets a biteship_order_id
	// to be looked up by in the first place; order.CreatedAt is the
	// belt-and-braces fallback if it's somehow unset.
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
