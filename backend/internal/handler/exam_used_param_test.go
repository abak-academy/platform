package handler

import "testing"

func TestParseUsedParam(t *testing.T) {
	tests := []struct {
		in      string
		want    *bool
		wantErr bool
	}{
		{"", nil, false},
		{"true", ptrBool(true), false},
		{"false", ptrBool(false), false},
		{"maybe", nil, true},
		{"1", nil, true},
		{"TRUE", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseUsedParam(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func ptrBool(b bool) *bool { return &b }
