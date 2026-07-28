package service

import (
	"context"
	"errors"
	"testing"
)

func TestValidateKodePos(t *testing.T) {
	tests := []struct {
		name    string
		kodePos string
		wantErr error
	}{
		{"a five-digit code is fine", "15310", nil},
		{"a leading zero survives, which is why this is text", "01234", nil},
		{"a short all-digit code is accepted — length is not this guard's job", "1", nil},
		{"empty means absent, not invalid", "", nil},
		{"letters are rejected", "1531O", ErrInvalidKodePos},
		{"a space is rejected", "15 310", ErrInvalidKodePos},
		{"punctuation is rejected", "15310-", ErrInvalidKodePos},
		{"a sign is not part of a postal code", "+15310", ErrInvalidKodePos},
		{"decimals are not part of a postal code", "153.10", ErrInvalidKodePos},
		{"non-ASCII digits are rejected", "１５３１０", ErrInvalidKodePos},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKodePos(tc.kodePos)
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

// The quote is the path that matters most: an unresolvable destination is not
// reported by the courier API, it silently degrades to the flat rate, so a
// postcode typo would otherwise surface as a wrong price rather than an error.
func TestGetShippingRates_RejectsNonNumericKodePos(t *testing.T) {
	svc := &Service{}

	_, err := svc.GetShippingRates(context.Background(), ShippingQuoteRequest{
		DestinationPostalCode: "BSD15310",
		WeightGrams:           500,
	})

	if !errors.Is(err, ErrInvalidKodePos) {
		t.Fatalf("want ErrInvalidKodePos, got %v", err)
	}
}
