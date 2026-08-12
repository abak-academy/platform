package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"akademi-bimbel/internal/model"
)

// shipmentDestination is the shape written by the storefront checkout into
// orders.shipping_address (see web/app/(student)/cart/page.tsx's PatchCart
// call) — penerima/telepon/alamat/kode_pos, all required to address a
// Biteship pickup.
type shipmentDestination struct {
	Penerima string `json:"penerima"`
	Telepon  string `json:"telepon"`
	Alamat   string `json:"alamat"`
	KodePos  string `json:"kode_pos"`
}

func parseShipmentDestination(raw json.RawMessage) (shipmentDestination, error) {
	var dest shipmentDestination
	if len(raw) == 0 {
		return dest, errors.New("shipping address is empty")
	}
	if err := json.Unmarshal(raw, &dest); err != nil {
		return dest, fmt.Errorf("parse shipping address: %w", err)
	}
	if dest.Penerima == "" || dest.Telepon == "" || dest.Alamat == "" || dest.KodePos == "" {
		return dest, errors.New("shipping address incomplete")
	}
	return dest, nil
}

// shippable answers whether a courier can be booked for this order now.
//
// The obvious half is an order that has not shipped yet. The second half is an
// order whose booking died: AdminCancelShipment and the webhook both leave
// orders.status at 'shipped' on purpose (FR-C-15 — the status is never walked
// back), so a cancelled or rejected parcel reads as shipped forever while
// nothing is actually on its way. Gating on status alone left the manual
// escape hatch closed too, which meant a dead shipment had no exit but a
// refund. shipment_status is the only thing that can tell them apart — the
// same signal refundAllowed already uses on the frontend.
func shippable(order model.Order) bool {
	if order.Status == "paid" || order.Status == "processing" {
		return true
	}
	return order.Status == "shipped" && isShipmentFailureStatus(order.ShipmentStatus)
}

// bookingReference is the reference_id sent to Biteship.
//
// Biteship requires it to be unique across all orders and rejects a reuse with
// 40002060, echoing the conflicting booking back — so re-booking under the
// bare order uuid returns the DEAD shipment through the already-booked path
// and writes its resi back as if it were live. Attempt 1 keeps the bare uuid
// so every booking already live at Biteship still resolves; each re-book gets
// its own suffix.
func bookingReference(orderID string, attempt int) string {
	if attempt <= 1 {
		return orderID
	}
	return fmt.Sprintf("%s-%d", orderID, attempt)
}

// nextBookingAttempt is the attempt this booking will be. An order that has
// never been booked stays at its current number; one that carries a previous
// Biteship booking is re-booking and needs a fresh reference.
//
// Deliberately not persisted before the call: if a re-book times out after
// Biteship actually created the shipment, the retry computes the SAME number,
// dedupes on reference_id and adopts the real booking instead of dispatching a
// second courier.
func nextBookingAttempt(order model.Order) int {
	attempt := order.ShipmentAttempt
	if attempt < 1 {
		attempt = 1
	}
	if order.BiteshipOrderID != nil && *order.BiteshipOrderID != "" {
		attempt++
	}
	return attempt
}

func shipmentItemsFromOrder(items []model.OrderItem) []ShipmentItem {
	var out []ShipmentItem
	for _, item := range items {
		if !isPhysicalType(item.ProductType) {
			continue
		}
		out = append(out, ShipmentItem{
			Name:        item.Name,
			Value:       int64(item.Jumlah),
			Quantity:    item.Qty,
			WeightGrams: item.WeightGrams,
		})
	}
	return out
}

// AdminShipOrder books a real courier pickup through the logistics client and
// moves the order to shipped, stamping waybill_source='biteship' (FR-C-3/
// FR-C-6/FR-C-7). On a duplicate-booking response (ErrShipmentAlreadyBooked)
// it adopts the Biteship order id already on record and re-fetches it instead
// of booking a second pickup, so a retry converges on the same end state. Any
// other failure leaves the order untouched and returns the real reason —
// there is no silent fall-back to manual entry.
// deliveryDate/deliveryTime are optional; both empty books an immediate
// pickup, which is what every booking did before scheduling existed.
func (s *Service) AdminShipOrder(ctx context.Context, orderID, deliveryDate, deliveryTime string) error {
	// A half-filled schedule is refused rather than quietly downgraded to an
	// immediate pickup. Booking "now" for an admin who asked for a date
	// dispatches a courier today against a parcel meant to go later — a real
	// carrier action nobody requested, and far worse than an error message.
	if (deliveryDate == "") != (deliveryTime == "") {
		return ErrIncompleteSchedule
	}

	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if !shippable(order) {
		return ErrOrderNotShippable
	}
	if order.CourierCode == nil || *order.CourierCode == "" ||
		order.CourierServiceCode == nil || *order.CourierServiceCode == "" {
		return ErrNoCarrierCode
	}

	dest, err := parseShipmentDestination(order.ShippingAddress)
	if err != nil {
		return ErrIncompleteAddress
	}

	cfg, err := s.GetSystemConfig(ctx)
	if err != nil {
		return err
	}
	senderName := cfg["app_name"]
	senderPhone := cfg["app_contact_phone"]
	senderAddress := cfg["app_address"]
	senderPostal := cfg["app_kode_pos"]
	if senderName == "" || senderPhone == "" || senderAddress == "" || senderPostal == "" {
		return errors.New("sender configuration incomplete: set app_name, app_contact_phone, app_address, app_kode_pos")
	}

	attempt := nextBookingAttempt(order)

	req := CreateShipmentRequest{
		ReferenceID: bookingReference(order.ID.String(), attempt),

		OriginContactName:  senderName,
		OriginContactPhone: senderPhone,
		OriginAddress:      senderAddress,
		OriginPostalCode:   senderPostal,

		DestinationContactName:  dest.Penerima,
		DestinationContactPhone: dest.Telepon,
		DestinationAddress:      dest.Alamat,
		DestinationPostalCode:   dest.KodePos,

		CourierCode: *order.CourierCode,
		ServiceCode: *order.CourierServiceCode,

		DeliveryDate: deliveryDate,
		DeliveryTime: deliveryTime,

		Items: shipmentItemsFromOrder(order.Items),
	}

	logistics := s.logisticsClient()
	shipment, err := logistics.CreateOrder(ctx, req)
	if err != nil {
		var already *ShipmentAlreadyBookedError
		if !errors.As(err, &already) {
			return fmt.Errorf("book shipment: %w", err)
		}
		// TODO: uncertain — biteshipOrderErrorResponse's echoed id field name
		// (adapter/biteship.go) is unverified against the real API; if it's
		// wrong, ExistingBiteshipOrderID arrives empty here at runtime even
		// though every unit test supplies the shape itself. Refuse rather
		// than GetOrder(""), which would either 404 or address the wrong
		// shipment.
		if already.ExistingBiteshipOrderID == "" {
			return errors.New("shipment already booked but no existing biteship order id was echoed back")
		}
		shipment, err = logistics.GetOrder(ctx, already.ExistingBiteshipOrderID)
		if err != nil {
			return fmt.Errorf("shipment already booked, re-fetch failed: %w", err)
		}
	}

	return s.storeRepo.SetShippedBiteship(ctx, id, shipment.WaybillID, shipment.BiteshipOrderID, attempt)
}

// AdminShipOrderManual is the escape hatch (FR-C-9) for couriers or
// situations the automated booking path can't handle: the admin enters a
// tracking number directly, no Biteship call is made, and waybill_source is
// stamped 'manual'. Preserves the pre-split AdminShipOrder behaviour exactly.
func (s *Service) AdminShipOrderManual(ctx context.Context, orderID, trackingNumber string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if !shippable(order) {
		return ErrOrderNotShippable
	}

	return s.storeRepo.SetShippedManual(ctx, id, trackingNumber)
}
