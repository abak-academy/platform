package service

import (
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestValidateSpecs(t *testing.T) {
	long := strings.Repeat("a", 501)

	tests := []struct {
		name    string
		specs   []model.ProductSpec
		wantErr bool
	}{
		{"nil is allowed", nil, false},
		{"empty is allowed", []model.ProductSpec{}, false},
		{
			"well-formed row",
			[]model.ProductSpec{{Key: "penerbit", Label: "Penerbit", Value: "Yayasan Abak Cendekia"}},
			false,
		},
		{"missing key", []model.ProductSpec{{Label: "Penerbit", Value: "x"}}, true},
		{"missing label", []model.ProductSpec{{Key: "penerbit", Value: "x"}}, true},
		{
			"key over 100 chars",
			[]model.ProductSpec{{Key: strings.Repeat("k", 101), Label: "L", Value: "v"}},
			true,
		},
		{
			"label over 100 chars",
			[]model.ProductSpec{{Key: "k", Label: strings.Repeat("l", 101), Value: "v"}},
			true,
		},
		{"value over 500 chars", []model.ProductSpec{{Key: "k", Label: "L", Value: long}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpecs(tc.specs)
			if tc.wantErr && !errors.Is(err, ErrInvalidSpecs) {
				t.Fatalf("want ErrInvalidSpecs, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

func TestValidateSpecs_RejectsMoreThanThirtyRows(t *testing.T) {
	specs := make([]model.ProductSpec, 31)
	for i := range specs {
		specs[i] = model.ProductSpec{Key: "k", Label: "L", Value: "v"}
	}
	if err := ValidateSpecs(specs); !errors.Is(err, ErrInvalidSpecs) {
		t.Fatalf("want ErrInvalidSpecs for 31 rows, got %v", err)
	}

	if err := ValidateSpecs(specs[:30]); err != nil {
		t.Fatalf("30 rows should be accepted, got %v", err)
	}
}

// An empty value is legal — the frontend renders the canonical field list with
// blank rows the operator has not filled in yet, and skips them at display time.
func TestValidateSpecs_AllowsEmptyValue(t *testing.T) {
	if err := ValidateSpecs([]model.ProductSpec{{Key: "isbn", Label: "ISBN", Value: ""}}); err != nil {
		t.Fatalf("empty value should be accepted, got %v", err)
	}
}
