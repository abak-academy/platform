package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ShippingQuoteRequest struct {
	DestinationPostalCode string
	WeightGrams           int
	// ItemValue is the item's value in rupiah, used for Biteship's insurance
	// valuation. Zero means unknown; callers that don't set it (the standalone
	// POST /orders/shipping endpoint) fall back to the pre-existing hardcoded 1.
	ItemValue int64
}

type CourierRate struct {
	Courier       string `json:"courier"`
	Service       string `json:"service"`
	EstimatedDays int    `json:"estimated_days"`
	Price         int64  `json:"price"`
	// IsEstimate marks a rate that did not come from a carrier — currently the
	// configured flat-rate fallback. The storefront must label these so a buyer
	// is never shown an invented figure that looks like a real quote.
	IsEstimate bool `json:"is_estimate"`
	// CourierCode/ServiceCode are Biteship's codes (e.g. "jne"/"reg"), distinct
	// from Courier/Service which are display names. POST /v1/orders requires
	// the codes; a booking must use the code the buyer's quote was priced under.
	CourierCode string `json:"courier_code"`
	ServiceCode string `json:"service_code"`
}

// ShipmentItem is one line item on a CreateShipmentRequest.
type ShipmentItem struct {
	Name        string
	Value       int64
	Quantity    int
	WeightGrams int
}

// CreateShipmentRequest is the input to LogisticsClient.CreateOrder.
type CreateShipmentRequest struct {
	ReferenceID string

	OriginContactName  string
	OriginContactPhone string
	OriginAddress      string
	OriginPostalCode   string

	DestinationContactName  string
	DestinationContactPhone string
	DestinationAddress      string
	DestinationPostalCode   string

	// CourierCode/ServiceCode are the codes the rate was quoted under
	// (CourierRate.CourierCode/ServiceCode), not display names.
	CourierCode string
	ServiceCode string

	// DeliveryDate/DeliveryTime schedule the pickup. Both empty means "now",
	// which is what every booking did before this was plumbed through.
	// Biteship's formats: date "2026-08-10", time "13:00".
	DeliveryDate string
	DeliveryTime string

	Items []ShipmentItem
}

// Shipment is the result of CreateOrder or GetOrder.
type Shipment struct {
	BiteshipOrderID   string
	WaybillID         string
	Status            string
	CourierDriverName string
	// StatusUpdatedAt is the re-fetch's own timestamp for this status,
	// sourced for order_shipment_events.occurred_at (FR-C-12). Zero when the
	// adapter had nothing parseable — callers must fall back to something
	// deterministic per order, never time.Now() (FR-C-13).
	StatusUpdatedAt time.Time

	// TrackingURL is courier.link — Biteship's public tracking page for this
	// shipment. Empty until the courier issues one.
	TrackingURL string
}

// ErrShipmentAlreadyBooked is the sentinel callers check with errors.Is.
// BiteshipClient.CreateOrder returns a *ShipmentAlreadyBookedError wrapping
// it so a retry can also recover the pre-existing Biteship order id
// (FR-C-7) — an error that only satisfies errors.Is but drops the id is
// useless to a retry that needs to adopt it rather than book a second pickup.
var ErrShipmentAlreadyBooked = errors.New("shipment already booked")

// ShipmentAlreadyBookedError carries the Biteship order id echoed back by
// duplicate-detection error 40002060.
type ShipmentAlreadyBookedError struct {
	ExistingBiteshipOrderID string
}

func (e *ShipmentAlreadyBookedError) Error() string {
	return fmt.Sprintf("shipment already booked: existing biteship order id %s", e.ExistingBiteshipOrderID)
}

func (e *ShipmentAlreadyBookedError) Is(target error) bool {
	return target == ErrShipmentAlreadyBooked
}

type LogisticsClient interface {
	GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error)
	CreateOrder(ctx context.Context, req CreateShipmentRequest) (Shipment, error)
	GetOrder(ctx context.Context, biteshipOrderID string) (Shipment, error)

	// CancelOrder cancels a booked pickup. reason is forwarded to the carrier.
	CancelOrder(ctx context.Context, biteshipOrderID, reason string) error
}

type NoopLogisticsClient struct{}

var _ LogisticsClient = (*NoopLogisticsClient)(nil)

func (n *NoopLogisticsClient) GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	// Returning fabricated JNE/TIKI quotes here meant that with no Biteship key
	// configured the storefront billed buyers a made-up shipping cost through
	// Midtrans, indistinguishable from a real quote. Failing is the honest
	// answer; the flat-rate fallback in resolveShippingRates handles the rest.
	return nil, ErrShippingUnavailable
}

func (n *NoopLogisticsClient) CreateOrder(ctx context.Context, req CreateShipmentRequest) (Shipment, error) {
	// An absent key must fail honestly rather than fabricating a booking.
	return Shipment{}, ErrShippingUnavailable
}

func (n *NoopLogisticsClient) GetOrder(ctx context.Context, biteshipOrderID string) (Shipment, error) {
	return Shipment{}, ErrShippingUnavailable
}

func (n *NoopLogisticsClient) CancelOrder(ctx context.Context, biteshipOrderID, reason string) error {
	return ErrShippingUnavailable
}
