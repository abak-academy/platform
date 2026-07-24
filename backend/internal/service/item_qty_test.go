package service

import (
	"errors"
	"testing"
)

func TestValidateItemQty(t *testing.T) {
	tests := []struct {
		name        string
		productType string
		qty         int
		wantErr     error
	}{
		{"exam qty 1 is fine", "exam", 1, nil},
		{"course qty 1 is fine", "course", 1, nil},
		{"exam qty 2 is rejected", "exam", 2, ErrDigitalQtyLimit},
		{"course qty 5 is rejected", "course", 5, ErrDigitalQtyLimit},
		{"book qty 3 is fine", "book", 3, nil},
		{"merchandise qty 10 is fine", "merchandise", 10, nil},
		{"medal qty 4 is fine", "medal", 4, nil},
		{"zero qty is rejected for any type", "book", 0, ErrInvalidQty},
		{"negative qty is rejected for any type", "exam", -1, ErrInvalidQty},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateItemQty(tc.productType, tc.qty)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}
