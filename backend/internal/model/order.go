package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID                 uuid.UUID       `json:"id"`
	StudentID          uuid.UUID       `json:"student_id"`
	Status             string          `json:"status"`
	Subtotal           float64         `json:"subtotal"`
	Discount           float64         `json:"discount"`
	ShippingCost       float64         `json:"shipping_cost"`
	Total              float64         `json:"total"`
	PromoCodeID        *uuid.UUID      `json:"promo_code_id"`
	ShippingAddress    json.RawMessage `json:"shipping_address"`
	SelectedCourier    string          `json:"selected_courier"`
	SelectedService    string          `json:"selected_service"`
	IsEstimate         bool            `json:"is_estimate"`
	TrackingNumber     string          `json:"tracking_number"`
	ShippedAt          *time.Time      `json:"shipped_at"`
	GatewayRef         string          `json:"gateway_ref"`
	PaymentMethod      string          `json:"payment_method"`
	PaymentExpiresAt   *time.Time      `json:"payment_expires_at"`
	PaidAt             *time.Time      `json:"paid_at"`
	InvoiceURL         string          `json:"invoice_url"`
	CheckedOutAt       *time.Time      `json:"checked_out_at"`
	CompletedAt        *time.Time      `json:"completed_at"`
	CancelledAt        *time.Time      `json:"cancelled_at"`
	CancellationReason string          `json:"cancellation_reason"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	Items              []OrderItem     `json:"items"`
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
