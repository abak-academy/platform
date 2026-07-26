package service

import (
	"context"
	"errors"
	"testing"
)

func TestResolveShippingRates(t *testing.T) {
	real := []CourierRate{{Courier: "JNE", Service: "REG", Price: 18000}}

	t.Run("real quotes pass through untouched and are not flagged", func(t *testing.T) {
		got, err := resolveShippingRates(real, nil, 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if len(got) != 1 || got[0].Courier != "JNE" {
			t.Fatalf("want the carrier quote, got %+v", got)
		}
		if got[0].IsEstimate {
			t.Error("a real carrier quote must not be flagged as an estimate")
		}
	})

	t.Run("client failure falls back to the configured flat rate, flagged", func(t *testing.T) {
		got, err := resolveShippingRates(nil, errors.New("upstream down"), 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if len(got) != 1 || got[0].Price != 12000 {
			t.Fatalf("want the flat rate, got %+v", got)
		}
		if !got[0].IsEstimate {
			t.Error("the flat-rate fallback must be flagged as an estimate")
		}
		// Compared against literals on purpose. The order page has no is_estimate
		// column to read, so it decides whether to show the estimate badge by
		// matching the stored courier name against a copy of these strings in
		// web/components/orders/ShippingInfo.tsx (ESTIMATE_COURIERS). Asserting
		// against the constants instead would move with any rename and never
		// fail, leaving the frontend to break in silence.
		if got[0].Courier != "Ongkir Flat" || got[0].Service != "Standar" {
			t.Errorf("fallback labels changed to %q/%q — update ESTIMATE_COURIERS in "+
				"web/components/orders/ShippingInfo.tsx and this assertion together",
				got[0].Courier, got[0].Service)
		}
	})

	t.Run("empty quote list falls back too", func(t *testing.T) {
		got, err := resolveShippingRates([]CourierRate{}, nil, 12000)
		if err != nil || len(got) != 1 || !got[0].IsEstimate {
			t.Fatalf("want a flagged flat rate, got %+v err=%v", got, err)
		}
	})

	t.Run("no quotes and no flat rate is an explicit failure, never a made-up number", func(t *testing.T) {
		got, err := resolveShippingRates(nil, errors.New("upstream down"), 0)
		if !errors.Is(err, ErrShippingUnavailable) {
			t.Fatalf("want ErrShippingUnavailable, got %v", err)
		}
		if got != nil {
			t.Fatalf("want no rates, got %+v", got)
		}
	})
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
