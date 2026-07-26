package service

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

// resolveShippingRates decides what the storefront may show for a shipping
// quote. Carrier quotes win. Otherwise the configured flat rate stands in,
// explicitly flagged as an estimate. If neither exists the caller gets an
// error — never an invented figure.
func resolveShippingRates(rates []CourierRate, clientErr error, flatRate int64) ([]CourierRate, error) {
	if clientErr == nil && len(rates) > 0 {
		return rates, nil
	}
	if flatRate > 0 {
		return []CourierRate{{
			Courier:    FallbackCourier,
			Service:    FallbackService,
			Price:      flatRate,
			IsEstimate: true,
		}}, nil
	}
	return nil, ErrShippingUnavailable
}
