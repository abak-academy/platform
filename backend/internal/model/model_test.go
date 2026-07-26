package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// compile-only test: instantiating every exported type with all fields
// forces the compiler to verify the type shapes match what callers expect.

var _ = User{
	ID:             "",
	Email:          nil,
	Username:       nil,
	Phone:          nil,
	PasswordHash:   "",
	Role:           "",
	Name:           "",
	SchoolID:       nil,
	Status:         "",
	OTPEnabled:     false,
	CreatedAt:      time.Time{},
	UpdatedAt:      time.Time{},
	DOB:            nil,
	Gender:         nil,
	Grade:          nil,
	AlamatDomisili: nil,
	TargetExam:     nil,
}

var _ = Order{
	ID:                    uuid.UUID{},
	StudentID:             uuid.UUID{},
	Status:                "",
	Subtotal:              0,
	Discount:              0,
	ShippingCost:          0,
	Total:                 0,
	PromoCodeID:           nil,
	ShippingAddress:       nil,
	SelectedCourier:       "",
	TrackingNumber:        "",
	ShippedAt:             nil,
	GatewayRef:            "",
	PaymentMethod:         "",
	PaymentExpiresAt:      nil,
	PaidAt:                nil,
	InvoiceURL:            "",
	CheckedOutAt:          nil,
	CompletedAt:           nil,
	CancelledAt:           nil,
	CancellationReason:    "",
	CreatedAt:             time.Time{},
	UpdatedAt:             time.Time{},
	Items:                 nil,
}

var _ = OrderItem{
	ID:          uuid.UUID{},
	OrderID:     uuid.UUID{},
	ProductID:   uuid.UUID{},
	ProductType: "",
	Name:        "",
	UnitPrice:   0,
	Qty:         0,
	Jumlah:      0,
	WeightGrams: 0,
	FulfilledAt: nil,
	CreatedAt:   time.Time{},
}

var _ = Product{
	ID:          "",
	Type:        "",
	Name:        "",
	Description: "",
	Price:       0,
	Stock:       0,
	Status:      "",
	WeightGrams: 0,
	ImageURL:    "",
	CourseIDs:   nil,
	CreatedAt:   time.Time{},
	UpdatedAt:   time.Time{},
}

var _ = Section{
	ID:        uuid.UUID{},
	CourseID:  uuid.UUID{},
	Title:     "",
	Position:  0,
	CreatedAt: time.Time{},
}

var _ = Lesson{
	ID:              uuid.UUID{},
	SectionID:       uuid.UUID{},
	Title:           "",
	VideoURL:        "",
	DurationSeconds: 0,
	Position:        0,
	CreatedAt:       time.Time{},
}

var _ = CourseSession{
	ID:               uuid.UUID{},
	StudentID:        uuid.UUID{},
	CourseID:         uuid.UUID{},
	OrderID:          nil,
	Status:           "",
	Source:           "",
	EnrolledAt:       time.Time{},
	RevokedAt:        nil,
	CompletedLessons: nil,
}

var _ = PromoCode{
	ID:                uuid.UUID{},
	Code:              "",
	DiscountPercent:   nil,
	DiscountAmount:    nil,
	MinOrderAmount:    nil,
	MaxDiscountAmount: nil,
	MaxUses:           nil,
	UsedCount:         0,
	ExpiresAt:         nil,
	CreatedAt:         time.Time{},
}

var _ = OutboxEvent{
	ID:            0,
	AggregateType: "",
	AggregateID:   uuid.UUID{},
	EventType:     "",
	Payload:       json.RawMessage(nil),
	CreatedAt:     "",
	Attempts:      0,
	LastError:     nil,
}

var _ = Announcement{
	ID:             "",
	Title:          "",
	Message:        "",
	Type:           "",
	Recipients:     "",
	Status:         "",
	ScheduledAt:    nil,
	SentAt:         nil,
	RecipientCount: nil,
	CreatedBy:      "",
	CreatedAt:      time.Time{},
	UpdatedAt:      time.Time{},
}

var _ = Job{
	ID:        "",
	Type:      "",
	Status:    "",
	Progress:  0,
	InputURL:  nil,
	ResultURL: nil,
	Error:     nil,
	CreatedBy: "",
	CreatedAt: time.Time{},
	UpdatedAt: time.Time{},
}
