package model

import (
	"time"

	"github.com/google/uuid"
)

type PromoCode struct {
	ID                uuid.UUID  `json:"id"`
	Code              string     `json:"code"`
	DiscountPercent   *float64   `json:"discount_percent"`
	DiscountAmount    *float64   `json:"discount_amount"`
	MinOrderAmount    *float64   `json:"min_order_amount"`
	MaxDiscountAmount *float64   `json:"max_discount_amount"`
	MaxUses           *int       `json:"max_uses"`
	UsedCount         int        `json:"used_count"`
	ExpiresAt         *time.Time `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	IsPublic          bool       `json:"is_public"`
}
