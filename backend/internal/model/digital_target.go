package model

import "github.com/google/uuid"

const (
	DigitalTargetCourse = "course"
	DigitalTargetExam   = "exam"
)

const (
	DigitalConflictOwned           = "owned"
	DigitalConflictClaimed         = "claimed"
	DigitalConflictDuplicateInCart = "duplicate_in_cart"
)

// DigitalTarget is one course or exam a digital order line grants, resolved
// from the product's links at checkout time.
type DigitalTarget struct {
	OrderItemID uuid.UUID `json:"order_item_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Kind        string    `json:"kind"`
	TargetID    uuid.UUID `json:"target_id"`
}

// DigitalTargetKey identifies a course or exam regardless of which product
// or order line reached it.
type DigitalTargetKey struct {
	Kind     string
	TargetID uuid.UUID
}

func (t DigitalTarget) Key() DigitalTargetKey {
	return DigitalTargetKey{Kind: t.Kind, TargetID: t.TargetID}
}

// OrderDigitalSnapshot is the set of targets checkout reserved for an order.
// Taken is false for orders placed before snapshots existed, whose fulfilment
// still resolves the product's live links.
type OrderDigitalSnapshot struct {
	Taken   bool
	Targets []DigitalTarget
}

// DigitalTargetConflict names a target a student cannot buy again and why.
type DigitalTargetConflict struct {
	ProductID uuid.UUID `json:"product_id"`
	Kind      string    `json:"kind"`
	TargetID  uuid.UUID `json:"target_id"`
	Reason    string    `json:"reason"`
}
