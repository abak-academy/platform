package service

import (
	"fmt"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

const (
	maxAgreementCheckboxRows     = 10
	maxAgreementCheckboxTitleLen = 200
	maxAgreementCheckboxBodyLen  = 5000
)

// ValidateAgreementCheckboxes bounds the shape of the agreement checkboxes array, mirroring ValidateSpecs for product.specs.
func ValidateAgreementCheckboxes(checkboxes []model.ProductAgreementCheckbox) error {
	if len(checkboxes) > maxAgreementCheckboxRows {
		return fmt.Errorf("%w: at most %d rows allowed, got %d", ErrInvalidAgreementCheckboxes, maxAgreementCheckboxRows, len(checkboxes))
	}
	for i, cb := range checkboxes {
		if cb.Title == "" {
			return fmt.Errorf("%w: row %d has an empty title", ErrInvalidAgreementCheckboxes, i)
		}
		if cb.Body == "" {
			return fmt.Errorf("%w: row %d has an empty body", ErrInvalidAgreementCheckboxes, i)
		}
		if len(cb.Title) > maxAgreementCheckboxTitleLen {
			return fmt.Errorf("%w: row %d title exceeds %d characters", ErrInvalidAgreementCheckboxes, i, maxAgreementCheckboxTitleLen)
		}
		if len(cb.Body) > maxAgreementCheckboxBodyLen {
			return fmt.Errorf("%w: row %d body exceeds %d characters", ErrInvalidAgreementCheckboxes, i, maxAgreementCheckboxBodyLen)
		}
	}
	return nil
}

// NormalizeAgreementCheckboxIDs mints an ID for any checkbox left blank, leaving a supplied ID untouched so it stays stable across edits.
func NormalizeAgreementCheckboxIDs(checkboxes []model.ProductAgreementCheckbox) []model.ProductAgreementCheckbox {
	out := make([]model.ProductAgreementCheckbox, len(checkboxes))
	for i, cb := range checkboxes {
		if cb.ID == "" {
			cb.ID = uuid.NewString()
		}
		out[i] = cb
	}
	return out
}
