package service

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
			Courier:    "Flat",
			Service:    "Standard",
			Price:      flatRate,
			IsEstimate: true,
		}}, nil
	}
	return nil, ErrShippingUnavailable
}
