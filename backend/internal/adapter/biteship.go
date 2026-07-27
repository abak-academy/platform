package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

// configReader provides access to system configuration.
type configReader interface {
	ListSystemConfig(context.Context) ([]repository.SystemConfigRow, error)
}

type BiteshipClient struct {
	repo       configReader
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewBiteshipClient creates a real BiteshipClient.
func NewBiteshipClient(repo configReader, apiKey, baseURL string, httpClient *http.Client) *BiteshipClient {
	return &BiteshipClient{
		repo:       repo,
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// GetRates calls Biteship's Rates API with the given request.
// It reads the origin postal code from system_config and returns the parsed rates
// or an error if any step fails.
func (c *BiteshipClient) GetRates(ctx context.Context, req service.ShippingQuoteRequest) ([]service.CourierRate, error) {
	// Read origin postal code from system_config
	originPostalCode, err := c.getOriginPostalCode(ctx)
	if err != nil {
		return nil, err
	}

	// Build request to Biteship
	biteshipReq := c.buildBiteshipRequest(originPostalCode, req)

	// Make HTTP request
	body, err := json.Marshal(biteshipReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Biteship request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/rates/couriers", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("authorization", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Biteship API: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Biteship API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var biteshipResp biteshipRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&biteshipResp); err != nil {
		return nil, fmt.Errorf("failed to parse Biteship response: %w", err)
	}

	// Convert to service.CourierRate
	rates := c.parsePricing(biteshipResp.Pricing)
	return rates, nil
}

// getOriginPostalCode reads app_kode_pos from system_config.
func (c *BiteshipClient) getOriginPostalCode(ctx context.Context) (string, error) {
	rows, err := c.repo.ListSystemConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read system_config: %w", err)
	}

	for _, row := range rows {
		if row.Key == "app_kode_pos" && row.Value != "" {
			return row.Value, nil
		}
	}

	return "", fmt.Errorf("app_kode_pos not configured in system_config")
}

// buildBiteshipRequest constructs the request body for Biteship's Rates API.
func (c *BiteshipClient) buildBiteshipRequest(originPostalCode string, req service.ShippingQuoteRequest) map[string]interface{} {
	return map[string]interface{}{
		"origin_postal_code":      originPostalCode,
		"destination_postal_code": req.DestinationPostalCode,
		"couriers":                "anteraja,jne,sicepat,tiki",
		"items": []map[string]interface{}{
			{
				"name":     "items",
				"value":    1, // Default value
				"quantity": 1,
				"weight":   req.WeightGrams,
			},
		},
	}
}

// durationDigits pulls every run of digits out of a duration string.
var durationDigits = regexp.MustCompile(`\d+`)

// parseDurationDays reads a day count out of Biteship's duration field, which is
// prose rather than a number — "1 - 2 days", "2 hari", "Same day". strconv.Atoi
// fails on all of those, which is why every rate previously reported 0.
//
// Ranges resolve to their UPPER bound: "1 - 2 days" is 2. Quoting the lower one
// would show the buyer a delivery date the carrier never promised.
//
// Returns 0 when the string holds no digits at all ("Same day"), which the
// storefront renders as no estimate rather than as zero days.
func parseDurationDays(duration string) int {
	max := 0
	for _, match := range durationDigits.FindAllString(duration, -1) {
		n, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max
}

// parsePricing converts Biteship pricing array to service.CourierRate slice.
func (c *BiteshipClient) parsePricing(pricing []biteshipPricingItem) []service.CourierRate {
	var rates []service.CourierRate
	for _, item := range pricing {
		estimatedDays := parseDurationDays(item.Duration)
		if estimatedDays == 0 && item.Duration != "" {
			// Not necessarily wrong — "Same day" legitimately has no day count —
			// but it is the only signal that a format we cannot read has appeared.
			slog.Info("biteship duration carried no day count",
				"duration", item.Duration,
				"courier", item.CourierName,
				"service", item.CourierServiceName)
		}

		rate := service.CourierRate{
			Courier:       item.CourierName,
			Service:       item.CourierServiceName,
			EstimatedDays: estimatedDays,
			Price:         int64(item.Price),
		}
		rates = append(rates, rate)
	}
	return rates
}

// biteshipRatesResponse represents the response from Biteship Rates API.
type biteshipRatesResponse struct {
	Pricing []biteshipPricingItem `json:"pricing"`
}

// biteshipPricingItem represents a single rate option from Biteship.
type biteshipPricingItem struct {
	CourierName        string `json:"courier_name"`
	CourierServiceName string `json:"courier_service_name"`
	Price              int64  `json:"price"`
	Duration           string `json:"duration"`
}
