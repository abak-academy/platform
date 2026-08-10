package handler

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

// OrderSummaryProduct is the money-free projection of repository.TopProduct.
// It exists so that the summary endpoint — which admin_store may call, and which
// must never carry revenue — cannot leak an amount by accident. Hiding the field
// in the UI would still ship the bytes; there is no field here to ship.
type OrderSummaryProduct struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	ProductType string `json:"product_type"`
	QtySold     int    `json:"qty_sold"`
	OrderCount  int    `json:"order_count"`
}

type OrderSummaryResponse struct {
	Buckets     repository.OrderBucketCounts `json:"buckets"`
	TopProducts []OrderSummaryProduct        `json:"top_products"`
}

func (h *Handler) AdminListOrders(c echo.Context) error {
	filter := repository.OrderFilter{
		ProductType: c.QueryParam("type"),
		Cursor:      c.QueryParam("cursor"),
		Search:      strings.TrimSpace(c.QueryParam("q")),
		Limit:       parseLimit(c.QueryParam("limit"), 20, 100),
	}
	// "ready_to_ship" is a synthetic status: it names CountOrdersByBucket's
	// ready_to_ship bucket (paid/processing + a physical item), not a single
	// orders.status value — see OrderFilter.ReadyToShip.
	if status := c.QueryParam("status"); status == "ready_to_ship" {
		filter.ReadyToShip = true
	} else {
		filter.Status = status
	}

	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}
	filter.CreatedFrom, filter.CreatedTo = from, to

	// ?shipment=failed is the admin's only way to find parcels that died:
	// orders.status is never walked back by the webhook (FR-C-15), so a
	// courier-not-found order still reads "shipped" in every other view.
	if c.QueryParam("shipment") == "failed" {
		filter.ShipmentStatusIn = service.ShipmentFailureStatusValues()
	}

	orders, nextCursor, err := h.svc.AdminListOrders(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        orders,
		"next_cursor": nextCursor,
	})
}

// AdminOrdersSummary returns counts and quantity-ranked products. Deliberately
// no money: see OrderSummaryProduct.
func (h *Handler) AdminOrdersSummary(c echo.Context) error {
	filter := repository.OrderFilter{
		Search: strings.TrimSpace(c.QueryParam("q")),
	}
	if status := c.QueryParam("status"); status == "ready_to_ship" {
		filter.ReadyToShip = true
	} else {
		filter.Status = status
	}
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}
	filter.CreatedFrom, filter.CreatedTo = from, to
	if c.QueryParam("shipment") == "failed" {
		filter.ShipmentStatusIn = service.ShipmentFailureStatusValues()
	}

	// Month-to-date is computed in Jakarta time: an order placed at 08:00 WIB on
	// the 1st belongs to this month, and UTC would put it in the last one.
	now := time.Now().In(jakarta)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, jakarta)
	monthEnd := monthStart.AddDate(0, 1, 0)

	buckets, err := h.svc.AdminOrderBuckets(c.Request().Context(), filter, monthStart, monthEnd)
	if err != nil {
		return mapServiceError(c, err)
	}

	products, err := h.svc.AdminTopProducts(c.Request().Context(), monthStart, monthEnd, "qty", 5)
	if err != nil {
		return mapServiceError(c, err)
	}

	rows := make([]OrderSummaryProduct, 0, len(products))
	for _, p := range products {
		rows = append(rows, OrderSummaryProduct{
			ProductID:   p.ProductID.String(),
			Name:        p.Name,
			ProductType: p.ProductType,
			QtySold:     p.QtySold,
			OrderCount:  p.OrderCount,
		})
	}

	return c.JSON(http.StatusOK, OrderSummaryResponse{Buckets: buckets, TopProducts: rows})
}

var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

func parseLimit(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// parseDayRange reads YYYY-MM-DD bounds and returns a half-open [from, to)
// window: `to` is advanced a day so the caller's last day is included whole,
// without the double-counting an inclusive upper bound causes on adjacent
// periods.
func parseDayRange(fromStr, toStr string) (*time.Time, *time.Time, error) {
	var from, to *time.Time
	if fromStr != "" {
		v, err := time.ParseInLocation("2006-01-02", fromStr, jakarta)
		if err != nil {
			return nil, nil, errors.New("invalid from date format (use YYYY-MM-DD)")
		}
		from = &v
	}
	if toStr != "" {
		v, err := time.ParseInLocation("2006-01-02", toStr, jakarta)
		if err != nil {
			return nil, nil, errors.New("invalid to date format (use YYYY-MM-DD)")
		}
		next := v.AddDate(0, 0, 1)
		to = &next
	}
	return from, to, nil
}

func (h *Handler) AdminGetOrder(c echo.Context) error {
	orderID := c.Param("id")

	order, err := h.svc.AdminGetOrder(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, order)
}

// AdminGetPaymentProof mints a short-lived link to an order's payment proof.
// Proofs are deliberately absent from the unauthenticated /files/* proxy's
// allowlist, so this route — already behind orders:write — is the only way to
// read one.
func (h *Handler) AdminGetPaymentProof(c echo.Context) error {
	url, err := h.svc.PresignPaymentProofURL(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"url": url})
}

// AdminGetRefundProof mints a short-lived link to an order's refund transfer
// receipt. Like payment proofs, refund_proof is absent from the public file
// proxy's allowlist, so this route is the only way to read one.
func (h *Handler) AdminGetRefundProof(c echo.Context) error {
	url, err := h.svc.PresignRefundProofURL(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"url": url})
}

func (h *Handler) AdminConfirmOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	var req struct {
		PaymentProofURL string `json:"payment_proof_url"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if !validPaymentProofKey(req.PaymentProofURL) {
		return badRequest(c, "payment_proof_url is required and must be a payment_proof/ upload key")
	}

	err := h.svc.AdminConfirmOrder(c.Request().Context(), actorID, orderID, key, req.PaymentProofURL)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order confirmed",
	})
}

// validPaymentProofKey enforces invariant 6 (FR-25/FR-26): a confirmed order
// must never exist without its evidence, and the evidence key must resolve
// inside the payment_proof/ upload prefix with no path-traversal segment.
func validPaymentProofKey(key string) bool { return validProofKey(key, "payment_proof") }

// validRefundProofKey applies the same rule to the manual transfer receipt that
// evidences a refund — the money is moved by a human, so this key is the only
// record the system gets that it happened.
func validRefundProofKey(key string) bool { return validProofKey(key, "refund_proof") }

func validProofKey(key, prefix string) bool {
	if key == "" {
		return false
	}
	if strings.Contains(key, "..") {
		return false
	}
	return strings.HasPrefix(key, prefix+"/")
}

func (h *Handler) AdminShipOrder(c echo.Context) error {
	orderID := c.Param("id")

	// Both optional: an empty body still books an immediate pickup, so the
	// existing call shape keeps working unchanged.
	var req struct {
		DeliveryDate string `json:"delivery_date"`
		DeliveryTime string `json:"delivery_time"`
	}
	_ = c.Bind(&req)

	err := h.svc.AdminShipOrder(c.Request().Context(), orderID, req.DeliveryDate, req.DeliveryTime)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order shipped",
	})
}

// AdminGetShippingLabel streams a printable packing slip PDF for an order's
// waybill (FR-D-1..D-4) — a packing slip, not the scannable carrier label,
// which comes from Biteship's dashboard.
func (h *Handler) AdminGetShippingLabel(c echo.Context) error {
	orderID := c.Param("id")

	pdf, err := h.svc.GetShippingLabel(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	return c.Stream(http.StatusOK, "application/pdf", bytes.NewReader(pdf))
}

func (h *Handler) AdminShipOrderManual(c echo.Context) error {
	orderID := c.Param("id")

	var req struct {
		TrackingNumber string `json:"tracking_number"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.TrackingNumber == "" {
		return badRequest(c, "tracking_number is required")
	}

	err := h.svc.AdminShipOrderManual(c.Request().Context(), orderID, req.TrackingNumber)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order shipped",
	})
}

// AdminRefreshShipment pulls an order's state from Biteship on demand — the
// manual fallback for a webhook that never arrived.
func (h *Handler) AdminRefreshShipment(c echo.Context) error {
	if err := h.svc.AdminRefreshShipment(c.Request().Context(), c.Param("id")); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "shipment refreshed"})
}

// AdminCancelShipment cancels the courier booking. It does not refund and does
// not move orders.status — see issue #72 for refund semantics.
func (h *Handler) AdminCancelShipment(c echo.Context) error {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return badRequest(c, "reason is required")
	}

	if err := h.svc.AdminCancelShipment(c.Request().Context(), c.Param("id"), req.Reason); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "shipment cancelled"})
}

// AdminGetOrderTracking returns the parcel's journey for the tracking dialog.
func (h *Handler) AdminGetOrderTracking(c echo.Context) error {
	view, err := h.svc.GetOrderTracking(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, view)
}

func (h *Handler) AdminCompleteOrder(c echo.Context) error {
	orderID := c.Param("id")

	err := h.svc.AdminCompleteOrder(c.Request().Context(), orderID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order completed",
	})
}

func (h *Handler) AdminRefundOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")

	var req struct {
		RefundProofURL string `json:"refund_proof_url"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if !validRefundProofKey(req.RefundProofURL) {
		return badRequest(c, "refund_proof_url is required and must be a refund_proof/ upload key")
	}

	err := h.svc.AdminRefundOrder(c.Request().Context(), actorID, orderID, req.RefundProofURL)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order refunded",
	})
}

func (h *Handler) AdminReconcileOrder(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	orderID := c.Param("id")
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}

	err := h.svc.AdminReconcileOrder(c.Request().Context(), actorID, orderID, key)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "order reconciled",
	})
}
