package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

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

var _ service.LogisticsClient = (*BiteshipClient)(nil)

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
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%w: Biteship API returned status %d: %s", service.ErrShippingAuthRejected, resp.StatusCode, string(respBody))
		}
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

	return "", fmt.Errorf("%w: app_kode_pos not configured in system_config", service.ErrShippingOriginUnset)
}

// buildBiteshipRequest constructs the request body for Biteship's Rates API.
func (c *BiteshipClient) buildBiteshipRequest(originPostalCode string, req service.ShippingQuoteRequest) map[string]interface{} {
	itemValue := req.ItemValue
	if itemValue == 0 {
		itemValue = 1
	}
	return map[string]interface{}{
		"origin_postal_code":      originPostalCode,
		"destination_postal_code": req.DestinationPostalCode,
		"couriers":                "anteraja,jne,sicepat,tiki",
		"items": []map[string]interface{}{
			{
				"name":     "items",
				"value":    itemValue,
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
			CourierCode:   item.CourierCode,
			ServiceCode:   item.CourierServiceCode,
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
	CourierCode        string `json:"courier_code"`
	CourierServiceCode string `json:"courier_service_code"`
	Price              int64  `json:"price"`
	Duration           string `json:"duration"`
}

// CreateOrder calls Biteship's POST /v1/orders to book a pickup, using
// req.ReferenceID as reference_id so a retry is detected as a duplicate by
// Biteship itself rather than booking a second pickup.
func (c *BiteshipClient) CreateOrder(ctx context.Context, req service.CreateShipmentRequest) (service.Shipment, error) {
	body, err := json.Marshal(buildBiteshipOrderRequest(req))
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to marshal Biteship order request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/orders", bytes.NewReader(body))
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("authorization", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to call Biteship API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to read Biteship response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return service.Shipment{}, classifyBiteshipOrderError(resp.StatusCode, respBody)
	}

	// Biteship's duplicate-reference case has been seen answering with a 2xx
	// status and "success": false in the body, so check both.
	var errProbe biteshipOrderErrorResponse
	if jsonErr := json.Unmarshal(respBody, &errProbe); jsonErr == nil && !errProbe.Success && errProbe.Code != 0 {
		return service.Shipment{}, classifyBiteshipOrderError(resp.StatusCode, respBody)
	}

	return parseBiteshipOrderResponse(respBody)
}

// GetOrder calls Biteship's GET /v1/orders/:id to re-fetch the authoritative
// status of a previously created order.
func (c *BiteshipClient) GetOrder(ctx context.Context, biteshipOrderID string) (service.Shipment, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/orders/"+biteshipOrderID, nil)
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("authorization", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to call Biteship API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return service.Shipment{}, fmt.Errorf("failed to read Biteship response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return service.Shipment{}, classifyBiteshipOrderError(resp.StatusCode, respBody)
	}

	return parseBiteshipOrderResponse(respBody)
}

// CancelOrder cancels a booked pickup via POST /v1/orders/:id/cancel.
//
// TODO: uncertain — the request body's field name is unverified. Biteship's
// own docs page for this endpoint renders client-side and returns no content
// to a fetch, and the changelog documents only the method and path. Two
// independent unofficial SDKs (Go toel-app/biteship, a Laravel client) both
// send "reason", which is what this sends. If a live cancel is rejected as a
// validation error, try "cancellation_reason" before looking anywhere else.
func (c *BiteshipClient) CancelOrder(ctx context.Context, biteshipOrderID, reason string) error {
	body, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return fmt.Errorf("failed to marshal Biteship cancel request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/orders/"+biteshipOrderID+"/cancel", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("authorization", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call Biteship API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Biteship response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifyBiteshipOrderError(resp.StatusCode, respBody)
	}

	// Same trap as CreateOrder: Biteship has been seen answering 2xx with
	// "success": false, so a status code alone is not proof it worked.
	var errProbe biteshipOrderErrorResponse
	if jsonErr := json.Unmarshal(respBody, &errProbe); jsonErr == nil && !errProbe.Success && errProbe.Code != 0 {
		return classifyBiteshipOrderError(resp.StatusCode, respBody)
	}
	return nil
}

// TrackWaybill calls GET /v1/trackings/:waybill_id/couriers/:courier_code —
// Biteship's track-any-waybill endpoint, which answers for a waybill we never
// booked.
//
// TODO: uncertain — the response shape is unverified. Biteship's docs page for
// this endpoint renders client-side and returns nothing to a fetch, and the
// unofficial SDKs expose only the method signature, not the struct. The
// parsing below accepts the field names their order endpoints already use
// (history[].status/note/updated_at) and treats anything it cannot read as an
// empty log, so a wrong guess degrades to the stored events rather than
// showing a broken timeline. Confirm against a real response before relying
// on it.
func (c *BiteshipClient) TrackWaybill(ctx context.Context, waybillID, courierCode string) (service.WaybillTracking, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/trackings/"+url.PathEscape(waybillID)+"/couriers/"+url.PathEscape(courierCode), nil)
	if err != nil {
		return service.WaybillTracking{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("authorization", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return service.WaybillTracking{}, fmt.Errorf("failed to call Biteship API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return service.WaybillTracking{}, fmt.Errorf("failed to read Biteship response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return service.WaybillTracking{}, classifyBiteshipOrderError(resp.StatusCode, respBody)
	}

	return parseBiteshipTracking(respBody)
}

func parseBiteshipTracking(body []byte) (service.WaybillTracking, error) {
	var resp biteshipTrackingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return service.WaybillTracking{}, fmt.Errorf("failed to parse Biteship tracking response: %w", err)
	}

	out := service.WaybillTracking{Status: resp.Status}
	for _, h := range resp.History {
		// A checkpoint with no timestamp cannot be placed on a timeline, and
		// showing it at the zero time would date it to year 1.
		if h.UpdatedAt.Time.IsZero() {
			continue
		}
		out.History = append(out.History, service.TrackingEntry{
			Status:     h.Status,
			Note:       h.Note,
			OccurredAt: h.UpdatedAt.Time,
		})
	}
	return out, nil
}

// biteshipTrackingResponse reuses the courier-history shape the order
// endpoints already return, which is the only shape we have actually seen.
type biteshipTrackingResponse struct {
	Status  string                        `json:"status"`
	History []biteshipCourierHistoryEntry `json:"history"`
}

// buildBiteshipOrderRequest maps CreateShipmentRequest onto Biteship's
// POST /v1/orders body. courier_company/courier_type take Biteship's codes
// (e.g. "jne"/"reg"), not the display names orders.selected_courier persists.
func buildBiteshipOrderRequest(req service.CreateShipmentRequest) biteshipOrderRequest {
	items := make([]biteshipOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, biteshipOrderItem{
			Name:     it.Name,
			Value:    it.Value,
			Quantity: it.Quantity,
			Weight:   it.WeightGrams,
		})
	}
	out := biteshipOrderRequest{
		ReferenceID:             req.ReferenceID,
		ShipperContactName:      req.OriginContactName,
		ShipperContactPhone:     req.OriginContactPhone,
		OriginContactName:       req.OriginContactName,
		OriginContactPhone:      req.OriginContactPhone,
		OriginAddress:           req.OriginAddress,
		OriginPostalCode:        req.OriginPostalCode,
		DestinationContactName:  req.DestinationContactName,
		DestinationContactPhone: req.DestinationContactPhone,
		DestinationAddress:      req.DestinationAddress,
		DestinationPostalCode:   req.DestinationPostalCode,
		CourierCompany:          req.CourierCode,
		CourierType:             req.ServiceCode,
		DeliveryType:            "now",
		Items:                   items,
	}

	// "scheduled" is only meaningful with both a date and a time; sending the
	// type without them would have Biteship reject the booking, so an
	// incomplete schedule falls back to an immediate pickup rather than
	// failing the whole ship action.
	if req.DeliveryDate != "" && req.DeliveryTime != "" {
		out.DeliveryType = "scheduled"
		out.DeliveryDate = req.DeliveryDate
		out.DeliveryTime = req.DeliveryTime
	}
	return out
}

// classifyBiteshipOrderError turns a non-2xx (or success:false) /v1/orders
// response into a typed error. Code 40002060 is Biteship's duplicate
// reference_id detection; its response echoes the pre-existing order id.
func classifyBiteshipOrderError(statusCode int, body []byte) error {
	var errResp biteshipOrderErrorResponse
	if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Code == 40002060 {
		orderID := errResp.Details.OrderID
		if orderID == "" {
			orderID = errResp.OrderID
		}
		return &service.ShipmentAlreadyBookedError{ExistingBiteshipOrderID: orderID}
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return fmt.Errorf("%w: Biteship API returned status %d: %s", service.ErrShippingAuthRejected, statusCode, string(body))
	}
	return fmt.Errorf("Biteship API returned status %d: %s", statusCode, string(body))
}

func parseBiteshipOrderResponse(body []byte) (service.Shipment, error) {
	var orderResp biteshipOrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return service.Shipment{}, fmt.Errorf("failed to parse Biteship order response: %w", err)
	}
	return service.Shipment{
		BiteshipOrderID:   orderResp.ID,
		WaybillID:         orderResp.Courier.WaybillID,
		Status:            orderResp.Status,
		CourierDriverName: orderResp.Courier.DriverName,
		StatusUpdatedAt:   statusUpdatedAtFromHistory(orderResp.Status, orderResp.Courier.History),
	}, nil
}

// statusUpdatedAtFromHistory picks the courier.history[] entry whose status
// matches the response's top-level status; if none matches, the latest entry
// by updated_at wins. Zero when history is absent or empty, so the caller's
// deterministic fallback chain still engages.
func statusUpdatedAtFromHistory(status string, history []biteshipCourierHistoryEntry) time.Time {
	var latest time.Time
	for _, h := range history {
		if h.Status == status {
			return h.UpdatedAt.Time
		}
		if h.UpdatedAt.Time.After(latest) {
			latest = h.UpdatedAt.Time
		}
	}
	return latest
}

// biteshipOrderRequest is the POST /v1/orders body.
type biteshipOrderRequest struct {
	ReferenceID string `json:"reference_id"`

	ShipperContactName  string `json:"shipper_contact_name"`
	ShipperContactPhone string `json:"shipper_contact_phone"`

	OriginContactName  string `json:"origin_contact_name"`
	OriginContactPhone string `json:"origin_contact_phone"`
	OriginAddress      string `json:"origin_address"`
	OriginPostalCode   string `json:"origin_postal_code"`

	DestinationContactName  string `json:"destination_contact_name"`
	DestinationContactPhone string `json:"destination_contact_phone"`
	DestinationAddress      string `json:"destination_address"`
	DestinationPostalCode   string `json:"destination_postal_code"`

	CourierCompany string `json:"courier_company"`
	CourierType    string `json:"courier_type"`
	DeliveryType   string `json:"delivery_type"`
	DeliveryDate   string `json:"delivery_date,omitempty"`
	DeliveryTime   string `json:"delivery_time,omitempty"`

	Items []biteshipOrderItem `json:"items"`
}

type biteshipOrderItem struct {
	Name     string `json:"name"`
	Value    int64  `json:"value"`
	Quantity int    `json:"quantity"`
	Weight   int    `json:"weight"`
}

// biteshipOrderResponse is the shared success shape of POST /v1/orders and
// GET /v1/orders/:id. There is no top-level updated_at on this response
// (https://biteship.com/id/docs/api/orders/retrieve) — the per-status
// timestamp used for order_shipment_events.occurred_at (FR-C-12) lives in
// courier.history[] instead.
type biteshipOrderResponse struct {
	ID          string `json:"id"`
	ReferenceID string `json:"reference_id"`
	Status      string `json:"status"`
	Courier     struct {
		WaybillID  string                        `json:"waybill_id"`
		DriverName string                        `json:"driver_name"`
		History    []biteshipCourierHistoryEntry `json:"history"`
	} `json:"courier"`
}

// biteshipCourierHistoryEntry is one entry of courier.history[] on
// GET /v1/orders/:id (https://biteship.com/id/docs/api/orders/retrieve), e.g.
// {"service_type": "-", "status": "confirmed", "note": "...",
// "updated_at": "2021-01-11T14:03:41+07:00"}.
type biteshipCourierHistoryEntry struct {
	ServiceType string            `json:"service_type"`
	Status      string            `json:"status"`
	Note        string            `json:"note"`
	UpdatedAt   biteshipTimestamp `json:"updated_at"`
}

// biteshipTimestamp unmarshals updated_at without ever failing: RFC 3339,
// unix seconds, and "2006-01-02 15:04:05" all parse; anything else (or
// null/absent) yields the zero time rather than aborting the parse of the
// whole response.
type biteshipTimestamp struct {
	time.Time
}

func (t *biteshipTimestamp) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		t.Time = time.Time{}
		return nil
	}

	if unquoted, err := strconv.Unquote(s); err == nil {
		if parsed, err := time.Parse(time.RFC3339, unquoted); err == nil {
			t.Time = parsed
			return nil
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", unquoted); err == nil {
			t.Time = parsed
			return nil
		}
		t.Time = time.Time{}
		return nil
	}

	if seconds, err := strconv.ParseInt(s, 10, 64); err == nil {
		t.Time = time.Unix(seconds, 0).UTC()
		return nil
	}

	t.Time = time.Time{}
	return nil
}

// biteshipOrderErrorResponse is Biteship's error shape for /v1/orders. Per
// https://biteship.com/id/docs/api/orders/create, the documented 40002060
// response nests the echoed pre-existing order under "details":
//
//	{"success":false,"error":"...","code":40002060,
//	 "details":{"order_id":"...","waybill_id":"...","reference_id":"..."}}
//
// OrderID is kept as a defensive fallback for a legacy/undocumented
// top-level echo — classifyBiteshipOrderError prefers Details.OrderID and
// only falls back to it when Details.OrderID is empty.
type biteshipOrderErrorResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Error   string `json:"error"`
	OrderID string `json:"order_id"`
	Details struct {
		OrderID     string `json:"order_id"`
		WaybillID   string `json:"waybill_id"`
		ReferenceID string `json:"reference_id"`
	} `json:"details"`
}
