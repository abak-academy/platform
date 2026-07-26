package service

import (
	"fmt"
)

// ValidateItemQty enforces the per-line quantity rule.
//
// Digital products are capped at one because fulfilment ignores qty entirely:
// the outbox worker creates a single exam registration or course enrolment per
// order item regardless of quantity, so qty 3 charged the buyer three times and
// delivered one. Capping here is the fix; the worker is deliberately untouched.
//
// admin_school multi-seat purchases do not pass through this path — they go via
// CreateBulkExamOrder, which fans out through order_participants.
func ValidateItemQty(productType string, qty int) error {
	if qty < 1 {
		return fmt.Errorf("%w: qty must be at least 1", ErrInvalidQty)
	}
	if !isPhysicalType(productType) && qty > 1 {
		return fmt.Errorf("%w: %s accepts qty 1, got %d", ErrDigitalQtyLimit, productType, qty)
	}
	return nil
}
