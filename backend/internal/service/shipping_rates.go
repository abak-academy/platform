package service

import "errors"

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

// resolveShippingRates returns carrier quotes only; otherwise a typed error for the cause, never an invented price.
func resolveShippingRates(rates []CourierRate, clientErr error) ([]CourierRate, string, error) {
	cause := classifyShippingCause(clientErr, len(rates))
	if clientErr == nil && len(rates) > 0 {
		return rates, cause, nil
	}
	switch cause {
	case "route_unserved":
		return nil, cause, ErrShippingRouteUnserved
	case "client_error":
		return nil, cause, ErrShippingTemporarilyUnavailable
	default:
		return nil, cause, ErrShippingUnavailable
	}
}
