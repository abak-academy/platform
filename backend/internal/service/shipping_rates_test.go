package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolveShippingRates(t *testing.T) {
	real := []CourierRate{{Courier: "JNE", Service: "REG", Price: 18000}}

	t.Run("real quotes pass through untouched and are not flagged", func(t *testing.T) {
		got, cause, err := resolveShippingRates(real, nil, 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if cause != "" {
			t.Errorf("want empty cause when a carrier quote wins, got %q", cause)
		}
		if len(got) != 1 || got[0].Courier != "JNE" {
			t.Fatalf("want the carrier quote, got %+v", got)
		}
		if got[0].IsEstimate {
			t.Error("a real carrier quote must not be flagged as an estimate")
		}
	})

	t.Run("client failure falls back to the configured flat rate, flagged", func(t *testing.T) {
		got, _, err := resolveShippingRates(nil, errors.New("upstream down"), 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if len(got) != 1 || got[0].Price != 12000 {
			t.Fatalf("want the flat rate, got %+v", got)
		}
		if !got[0].IsEstimate {
			t.Error("the flat-rate fallback must be flagged as an estimate")
		}
		// Compared against literals on purpose: asserting against the constants
		// would move with any rename and could never fail. The frontend coupling
		// this used to guard is gone — the badge now reads the persisted
		// orders.is_estimate rather than matching courier names — but the strings
		// still matter, because 0047_order_is_estimate.up.sql backfilled historical
		// rows with `WHERE selected_courier IN ('Ongkir Flat', 'Flat')`. A rename
		// leaves those rows unreachable by any later backfill, and changes a label
		// buyers see.
		if got[0].Courier != "Ongkir Flat" || got[0].Service != "Standar" {
			t.Errorf("fallback labels changed to %q/%q — these are the strings "+
				"0047_order_is_estimate.up.sql backfilled on, and are user-visible",
				got[0].Courier, got[0].Service)
		}
	})

	t.Run("empty quote list falls back too", func(t *testing.T) {
		got, cause, err := resolveShippingRates([]CourierRate{}, nil, 12000)
		if err != nil || len(got) != 1 || !got[0].IsEstimate {
			t.Fatalf("want a flagged flat rate, got %+v err=%v", got, err)
		}
		if cause != "route_unserved" {
			t.Errorf("want cause=route_unserved, got %q", cause)
		}
	})

	t.Run("no quotes and no flat rate is an explicit failure, never a made-up number", func(t *testing.T) {
		got, _, err := resolveShippingRates(nil, errors.New("upstream down"), 0)
		if !errors.Is(err, ErrShippingUnavailable) {
			t.Fatalf("want ErrShippingUnavailable, got %v", err)
		}
		if got != nil {
			t.Fatalf("want no rates, got %+v", got)
		}
	})
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
