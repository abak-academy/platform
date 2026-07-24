package service

import "context"

type ShippingQuoteRequest struct {
	DestinationPostalCode string
	WeightGrams           int
}

type CourierRate struct {
	Courier       string `json:"courier"`
	Service       string `json:"service"`
	EstimatedDays int    `json:"estimated_days"`
	Price         int64  `json:"price"`
	// IsEstimate marks a rate that did not come from a carrier — currently the
	// configured flat-rate fallback. The storefront must label these so a buyer
	// is never shown an invented figure that looks like a real quote.
	IsEstimate bool `json:"is_estimate"`
}

type LogisticsClient interface {
	GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error)
}

type NoopLogisticsClient struct{}

func (n *NoopLogisticsClient) GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	// Returning fabricated JNE/TIKI quotes here meant that with no Biteship key
	// configured the storefront billed buyers a made-up shipping cost through
	// Midtrans, indistinguishable from a real quote. Failing is the honest
	// answer; the flat-rate fallback in resolveShippingRates handles the rest.
	return nil, ErrShippingUnavailable
}
