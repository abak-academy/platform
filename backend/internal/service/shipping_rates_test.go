package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolveShippingRates(t *testing.T) {
	real := []CourierRate{{Courier: "JNE", Service: "REG", Price: 18000}}
	tests := []struct {
		name               string
		rates              []CourierRate
		clientErr, wantErr error
		cause              string
	}{
		{"carrier quote", real, nil, nil, ""},
		{"client error", nil, errors.New("upstream down"), ErrShippingTemporarilyUnavailable, "client_error"},
		{"empty rates", nil, nil, ErrShippingRouteUnserved, "route_unserved"},
		{"origin unset", nil, ErrShippingOriginUnset, ErrShippingUnavailable, "origin_unset"},
		{"auth rejected", nil, ErrShippingAuthRejected, ErrShippingUnavailable, "auth_rejected"},
		{"noop", nil, ErrShippingUnavailable, ErrShippingUnavailable, "client_noop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cause, err := resolveShippingRates(tt.rates, tt.clientErr)
			if !errors.Is(err, tt.wantErr) || cause != tt.cause {
				t.Fatalf("rates=%+v cause=%q err=%v, want cause=%q err=%v", got, cause, err, tt.cause, tt.wantErr)
			}
			if tt.wantErr != nil && len(got) != 0 {
				t.Fatalf("failed quote returned a price: %+v", got)
			}
			if tt.wantErr == nil && (len(got) != 1 || got[0].Price != 18000) {
				t.Fatalf("carrier quote changed: %+v", got)
			}
		})
	}
}

// TestClassifyShippingCause covers FR-A-2..A-5: the four ways a shipping
// quote can degrade to something other than a live carrier rate, plus the
// carrier-quote-wins empty cause.
func TestClassifyShippingCause(t *testing.T) {
	tests := []struct {
		name      string
		clientErr error
		rateCount int
		want      string
	}{
		{"carrier quote wins", nil, 1, ""},
		{"origin postal code not configured", ErrShippingOriginUnset, 0, "origin_unset"},
		{"biteship rejected credentials", ErrShippingAuthRejected, 0, "auth_rejected"},
		{"no biteship key, noop client", ErrShippingUnavailable, 0, "client_noop"},
		{"200 with an empty pricing array", nil, 0, "route_unserved"},
		{"an unclassified client error", errors.New("dial tcp: timeout"), 0, "client_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyShippingCause(tc.clientErr, tc.rateCount)
			if got != tc.want {
				t.Errorf("classifyShippingCause(%v, %d) = %q, want %q", tc.clientErr, tc.rateCount, got, tc.want)
			}
		})
	}
}

// TestClassifyShippingCause_NeverEchoesUnderlyingErrorText pins that the
// cause is always one of the fixed discriminated strings, never the error's
// own message — which for auth rejection carries the raw Biteship response
// body (FR-A-3) and could echo back secret material.
func TestClassifyShippingCause_NeverEchoesUnderlyingErrorText(t *testing.T) {
	secretish := "sk_live_FAKESECRET_notreal_12345"
	err := errors.New(secretish + ": " + ErrShippingAuthRejected.Error())
	wrapped := errors.Join(ErrShippingAuthRejected, err)

	cause := classifyShippingCause(wrapped, 0)

	if cause != "auth_rejected" {
		t.Fatalf("want auth_rejected, got %q", cause)
	}
	if strings.Contains(cause, secretish) {
		t.Fatalf("cause leaked secret material: %q", cause)
	}
}

// The Noop client stands in for "no Biteship key configured". It used to return
// hardcoded JNE and TIKI quotes with a nil error, which meant GetShippingRates
// returned early and billed the buyer an invented amount.
func TestNoopLogisticsClient_ReturnsNoRates(t *testing.T) {
	rates, err := (&NoopLogisticsClient{}).GetRates(context.Background(), ShippingQuoteRequest{})
	if len(rates) != 0 {
		t.Fatalf("the noop client must not invent carrier quotes, got %+v", rates)
	}
	if err == nil {
		t.Fatal("the noop client must report that shipping is unavailable")
	}
}
