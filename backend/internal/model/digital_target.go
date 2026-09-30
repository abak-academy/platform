package model

import "github.com/google/uuid"

const (
	DigitalTargetCourse = "course"
	DigitalTargetExam   = "exam"
)

const (
	LiveTargetOwned    = "owned"
	LiveTargetInFlight = "in_flight"
)

// Conflict reasons the API sends to clients. "claimed" is the wire name for a
// live target of another order and is kept so clients need no change.
const (
	DigitalConflictOwned           = "owned"
	DigitalConflictClaimed         = "claimed"
	DigitalConflictDuplicateInCart = "duplicate_in_cart"
)

// DigitalTarget is one course or exam a digital order line grants, resolved
// from the product's links when it is checked.
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

// LiveDigitalTarget is a course or exam a student already holds access to or is
// buying in another live order. OrderID is set only for the latter.
type LiveDigitalTarget struct {
	Kind     string
	TargetID uuid.UUID
	Reason   string
	OrderID  *uuid.UUID
}

// DigitalTargetConflict names a target a student cannot buy again and why.
type DigitalTargetConflict struct {
	ProductID uuid.UUID `json:"product_id"`
	Kind      string    `json:"kind"`
	TargetID  uuid.UUID `json:"target_id"`
	Reason    string    `json:"reason"`
}
