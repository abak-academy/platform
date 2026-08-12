package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID                 uuid.UUID            `json:"id"`
	StudentID          uuid.UUID            `json:"student_id"`
	StudentName        string               `json:"student_name"`
	StudentSchool      string               `json:"student_school"`
	StudentGrade       *int                 `json:"student_grade"`
	Status             string               `json:"status"`
	Subtotal           float64              `json:"subtotal"`
	Discount           float64              `json:"discount"`
	ShippingCost       float64              `json:"shipping_cost"`
	Total              float64              `json:"total"`
	PromoCodeID        *uuid.UUID           `json:"promo_code_id"`
	ShippingAddress    json.RawMessage      `json:"shipping_address"`
	SelectedCourier    string               `json:"selected_courier"`
	SelectedService    string               `json:"selected_service"`
	IsEstimate         bool                 `json:"is_estimate"`
	TrackingNumber     string               `json:"tracking_number"`
	ShippedAt          *time.Time           `json:"shipped_at"`
	BiteshipOrderID    *string              `json:"biteship_order_id"`
	ShipmentStatus     *string              `json:"shipment_status"`
	ShipmentAttempt    int                  `json:"shipment_attempt"`
	WaybillSource      *string              `json:"waybill_source"`
	CourierCode        *string              `json:"courier_code"`
	CourierServiceCode *string              `json:"courier_service_code"`
	GatewayRef         string               `json:"gateway_ref"`
	PaymentMethod      string               `json:"payment_method"`
	PaymentProofURL    *string              `json:"payment_proof_url"`
	RefundProofURL     *string              `json:"refund_proof_url"`
	PaymentExpiresAt   *time.Time           `json:"payment_expires_at"`
	PaidAt             *time.Time           `json:"paid_at"`
	InvoiceURL         string               `json:"invoice_url"`
	CheckedOutAt       *time.Time           `json:"checked_out_at"`
	CompletedAt        *time.Time           `json:"completed_at"`
	CancelledAt        *time.Time           `json:"cancelled_at"`
	CancellationReason string               `json:"cancellation_reason"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	Items              []OrderItem          `json:"items"`
	ShipmentEvents     []OrderShipmentEvent `json:"shipment_events"`
}

type OrderShipmentEvent struct {
	ID                uuid.UUID `json:"id"`
	OrderID           uuid.UUID `json:"order_id"`
	Status            string    `json:"status"`
	CourierWaybillID  *string   `json:"courier_waybill_id"`
	CourierDriverName *string   `json:"courier_driver_name"`
	OccurredAt        time.Time `json:"occurred_at"`
	CreatedAt         time.Time `json:"created_at"`
}

type OrderItem struct {
	ID          uuid.UUID  `json:"id"`
	OrderID     uuid.UUID  `json:"order_id"`
	ProductID   uuid.UUID  `json:"product_id"`
	ProductType string     `json:"product_type"`
	Name        string     `json:"name"`
	UnitPrice   float64    `json:"unit_price"`
	Qty         int        `json:"qty"`
	Jumlah      float64    `json:"jumlah"`
	WeightGrams int        `json:"weight_grams"`
	FulfilledAt *time.Time `json:"fulfilled_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
