package service

import "errors"

// FallbackCourier and FallbackService label the flat-rate stand-in shown when no
// carrier quote is available. They are deliberately generic: naming a real
// carrier on a price that carrier never quoted is exactly the defect this
// fallback exists to avoid.
//
// The order page infers "this was an estimate" by comparing the persisted
// selected_courier against these, because is_estimate is not stored on the
// order. That coupling is why these are constants rather than literals, and it
// goes away once the order carries the flag itself — see
// docs/backlog/shipping-estimate-flag.md.
const (
	FallbackCourier = "Ongkir Flat"
	FallbackService = "Standar"

	// LegacyFallbackCourier is what the fallback was called before this rename.
	// Orders placed under the old label still exist on staging, so the order
	// page keeps recognising it and their estimate badge does not vanish.
	LegacyFallbackCourier = "Flat"
)

// classifyShippingCause explains why a shipping quote could not be shown as a
// live carrier rate. It returns one of a fixed set of discriminated strings —
// never the underlying error's own text, which for auth rejection carries the
// raw Biteship response body (FR-A-3) — so it is safe to log verbatim and
// testable as a pure function, without a log-capturing harness.
func classifyShippingCause(clientErr error, rateCount int) string {
	if clientErr == nil && rateCount > 0 {
		return ""
	}
	switch {
	case errors.Is(clientErr, ErrShippingOriginUnset):
		return "origin_unset"
	case errors.Is(clientErr, ErrShippingAuthRejected):
		return "auth_rejected"
	case errors.Is(clientErr, ErrShippingUnavailable):
		return "client_noop"
	case clientErr == nil && rateCount == 0:
		return "route_unserved"
	default:
		return "client_error"
	}
}

// resolveShippingRates decides what the storefront may show for a shipping
// quote. Carrier quotes win. Otherwise the configured flat rate stands in,
// explicitly flagged as an estimate. If neither exists the caller gets an
// error — never an invented figure. The returned cause names why a carrier
// quote did not win (empty when it did), for the caller to log.
func resolveShippingRates(rates []CourierRate, clientErr error, flatRate int64) ([]CourierRate, string, error) {
	cause := classifyShippingCause(clientErr, len(rates))
	if clientErr == nil && len(rates) > 0 {
		return rates, cause, nil
	}
	if flatRate > 0 {
		return []CourierRate{{
			Courier:    FallbackCourier,
			Service:    FallbackService,
			Price:      flatRate,
			IsEstimate: true,
		}}, cause, nil
	}
	return nil, cause, ErrShippingUnavailable
}
