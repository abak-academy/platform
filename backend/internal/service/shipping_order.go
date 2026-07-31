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
func (s *Service) AdminShipOrder(ctx context.Context, orderID string) error {
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
	if order.Status != "paid" && order.Status != "processing" {
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

	req := CreateShipmentRequest{
		ReferenceID: order.ID.String(),

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

	return s.storeRepo.SetShippedBiteship(ctx, id, shipment.WaybillID, shipment.BiteshipOrderID)
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
	if order.Status != "paid" && order.Status != "processing" {
		return ErrOrderNotShippable
	}

	return s.storeRepo.SetShippedManual(ctx, id, trackingNumber)
}
