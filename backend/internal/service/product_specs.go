package service

import (
	"errors"
	"fmt"

	"akademi-bimbel/internal/model"
)

// ErrInvalidSpecs is returned when a product specification array is malformed.
var ErrInvalidSpecs = errors.New("invalid product specs")

const (
	maxSpecRows     = 30
	maxSpecKeyLen   = 100
	maxSpecLabelLen = 100
	maxSpecValueLen = 500
)

// ValidateSpecs bounds the shape of the specs array. It deliberately knows
// nothing about which fields belong to which product type — that catalogue
// lives in the frontend. These limits are what keep a free-form JSONB column
// from growing without bound.
func ValidateSpecs(specs []model.ProductSpec) error {
	if len(specs) > maxSpecRows {
		return fmt.Errorf("%w: at most %d rows allowed, got %d", ErrInvalidSpecs, maxSpecRows, len(specs))
	}
	for i, s := range specs {
		if s.Key == "" {
			return fmt.Errorf("%w: row %d has an empty key", ErrInvalidSpecs, i)
		}
		if s.Label == "" {
			return fmt.Errorf("%w: row %d has an empty label", ErrInvalidSpecs, i)
		}
		if len(s.Key) > maxSpecKeyLen {
			return fmt.Errorf("%w: row %d key exceeds %d characters", ErrInvalidSpecs, i, maxSpecKeyLen)
		}
		if len(s.Label) > maxSpecLabelLen {
			return fmt.Errorf("%w: row %d label exceeds %d characters", ErrInvalidSpecs, i, maxSpecLabelLen)
		}
		if len(s.Value) > maxSpecValueLen {
			return fmt.Errorf("%w: row %d value exceeds %d characters", ErrInvalidSpecs, i, maxSpecValueLen)
		}
	}
	return nil
}
