package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type OrderFilter struct {
	StudentID   *uuid.UUID
	Status      string
	ProductType string
	ExcludeCart bool
	Cursor      string
	Limit       int

	// ShipmentStatusIn matches orders.shipment_status against an explicit set.
	// The caller supplies every spelling it wants matched — shipment_status
	// holds whatever Biteship sent, unnormalised, so deciding which spellings
	// count is a domain question and stays in the service.
	ShipmentStatusIn []string
}

type OrderPatch struct {
	ShippingAddress    json.RawMessage
	SelectedCourier    string
	SelectedService    string
	IsEstimate         bool
	PromoCodeID        *uuid.UUID
	Discount           float64
	ShippingCost       float64
	Total              float64
	ProvinceID         *string
	CityID             *string
	DistrictID         *string
	KodePos            *string
	CourierCode        *string
	CourierServiceCode *string
}

const orderColumns = `id, student_id, status, subtotal, discount, shipping_cost, total,
	promo_code_id, shipping_address, selected_courier, selected_service, tracking_number, shipped_at,
	gateway_ref, payment_method, payment_expires_at, paid_at, invoice_url,
	checked_out_at, completed_at, cancelled_at, cancellation_reason,
	created_at, updated_at, is_estimate,
	biteship_order_id, shipment_status, waybill_source, courier_code, courier_service_code,
	payment_proof_url, refund_proof_url`

func scanOrder(row interface {
	Scan(dest ...any) error
}, order *model.Order) error {
	// Nullable TEXT columns must be scanned into *string so pgx v5 can set nil for SQL NULL.
	var selectedCourier, selectedService, trackingNumber, gatewayRef, paymentMethod, invoiceURL,
		cancellationReason *string
	err := row.Scan(
		&order.ID, &order.StudentID, &order.Status, &order.Subtotal, &order.Discount,
		&order.ShippingCost, &order.Total, &order.PromoCodeID, &order.ShippingAddress,
		&selectedCourier, &selectedService, &trackingNumber, &order.ShippedAt,
		&gatewayRef, &paymentMethod, &order.PaymentExpiresAt, &order.PaidAt, &invoiceURL,
		&order.CheckedOutAt, &order.CompletedAt, &order.CancelledAt, &cancellationReason,
		&order.CreatedAt, &order.UpdatedAt, &order.IsEstimate,
		&order.BiteshipOrderID, &order.ShipmentStatus, &order.WaybillSource,
		&order.CourierCode, &order.CourierServiceCode,
		&order.PaymentProofURL, &order.RefundProofURL,
	)
	if err != nil {
		return err
	}
	if selectedCourier != nil {
		order.SelectedCourier = *selectedCourier
	}
	if selectedService != nil {
		order.SelectedService = *selectedService
	}
	if trackingNumber != nil {
		order.TrackingNumber = *trackingNumber
	}
	if gatewayRef != nil {
		order.GatewayRef = *gatewayRef
	}
	if paymentMethod != nil {
		order.PaymentMethod = *paymentMethod
	}
	if invoiceURL != nil {
		order.InvoiceURL = *invoiceURL
	}
	if cancellationReason != nil {
		order.CancellationReason = *cancellationReason
	}
	return nil
}

func (r *Repository) fetchItems(ctx context.Context, orderID uuid.UUID) ([]model.OrderItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, fulfilled_at, created_at
		 FROM order_item
		 WHERE order_id = $1
		 ORDER BY created_at`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		item := model.OrderItem{}
		err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductType,
			&item.Name, &item.UnitPrice, &item.Qty, &item.Jumlah, &item.WeightGrams, &item.FulfilledAt, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) MintCart(ctx context.Context, studentID uuid.UUID) (model.Order, bool, error) {
	order := model.Order{}
	err := scanOrder(r.pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'cart', 0, 0, 0, 0)
		 ON CONFLICT (student_id) WHERE status = 'cart' DO NOTHING
		 RETURNING `+orderColumns,
		studentID,
	), &order)
	if err != nil {
		if isNotFound(err) {
			err = scanOrder(r.pool.QueryRow(ctx,
				`SELECT `+orderColumns+` FROM orders WHERE student_id = $1 AND status = 'cart'`,
				studentID,
			), &order)
			if err != nil {
				return model.Order{}, false, err
			}
			items, err := r.fetchItems(ctx, order.ID)
			if err != nil {
				return model.Order{}, false, err
			}
			order.Items = items
			return order, false, nil
		}
		return model.Order{}, false, err
	}
	return order, true, nil
}

// CreateOrderTx inserts a new cart-order row inside the caller's transaction.
// Unlike MintCart, there is no ON CONFLICT guard — the caller always gets a
// fresh row, appropriate for bulk orders where each creation is one-shot.
func (r *Repository) CreateOrderTx(ctx context.Context, tx pgx.Tx, studentID uuid.UUID) (model.Order, error) {
	order := model.Order{}
	err := scanOrder(tx.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'cart', 0, 0, 0, 0)
		 RETURNING `+orderColumns,
		studentID,
	), &order)
	return order, err
}

// InsertOrderItemTx inserts a single order item inside the caller's transaction
// and recalculates order totals within the same transaction.
func (r *Repository) InsertOrderItemTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, item model.OrderItem) error {
	item.ID = uuid.New()
	item.OrderID = orderID
	jumlah := item.UnitPrice * float64(item.Qty)
	_, err := tx.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())`,
		item.ID, item.OrderID, item.ProductID, item.ProductType, item.Name, item.UnitPrice, item.Qty, jumlah, item.WeightGrams,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE orders SET
		  subtotal   = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0),
		  total      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0) - discount + shipping_cost,
		  updated_at = now()
		WHERE id = $1`, orderID)
	return err
}

func (r *Repository) GetCartByStudentID(ctx context.Context, studentID uuid.UUID) (model.Order, error) {
	order := model.Order{}
	err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE student_id = $1 AND status = 'cart'`,
		studentID,
	), &order)
	if err != nil {
		if isNotFound(err) {
			return model.Order{}, nil
		}
		return model.Order{}, err
	}

	items, err := r.fetchItems(ctx, order.ID)
	if err != nil {
		return model.Order{}, err
	}
	order.Items = items
	return order, nil
}

func (r *Repository) GetOrderByID(ctx context.Context, id uuid.UUID) (model.Order, error) {
	order := model.Order{}
	err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE id = $1`,
		id,
	), &order)
	if err != nil {
		if isNotFound(err) {
			return model.Order{}, nil
		}
		return model.Order{}, err
	}

	items, err := r.fetchItems(ctx, order.ID)
	if err != nil {
		return model.Order{}, err
	}
	order.Items = items

	events, err := r.ListShipmentEvents(ctx, order.ID)
	if err != nil {
		return model.Order{}, err
	}
	order.ShipmentEvents = events
	return order, nil
}

// GetUserNamesByIDs resolves display names for a batch of student ids in a
// single query, keyed by id string. Ids missing from the returned map (e.g. a
// deleted student row) are the caller's responsibility to fall back on.
// ids is []string, not []uuid.UUID — pgx's exec mode under PgBouncer breaks
// on []uuid.UUID array parameters, and that only surfaces in a deployed
// environment, never against a local test database.
func (r *Repository) GetUserNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	names := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, name FROM users WHERE id = ANY($1::uuid[])`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

func (r *Repository) ListOrders(ctx context.Context, filter OrderFilter) ([]model.Order, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 10
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT ` + orderColumns + ` FROM orders WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filter.StudentID != nil {
		query += fmt.Sprintf(` AND student_id = $%d`, argNum)
		args = append(args, *filter.StudentID)
		argNum++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.ProductType != "" {
		query += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM order_item WHERE order_item.order_id = orders.id AND product_type = $%d)`, argNum)
		args = append(args, filter.ProductType)
		argNum++
	}
	if filter.ExcludeCart {
		query += ` AND status != 'cart'`
	}
	if len(filter.ShipmentStatusIn) > 0 {
		query += fmt.Sprintf(` AND shipment_status = ANY($%d)`, argNum)
		args = append(args, filter.ShipmentStatusIn)
		argNum++
	}
	if filter.Cursor != "" {
		curAt, curID, err := DecodeOrderCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(` AND (created_at, id) < ($%d, $%d)`, argNum, argNum+1)
		args = append(args, curAt, curID)
		argNum += 2
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	orders := []model.Order{}
	nextCursor := ""
	hasMore := false

	for rows.Next() {
		order := model.Order{}
		if err := scanOrder(rows, &order); err != nil {
			return nil, "", err
		}
		if len(orders) < filter.Limit {
			orders = append(orders, order)
		} else {
			// One row beyond the page proves there is a next page, but it must
			// not become the cursor: the predicate is strictly less-than, so
			// pointing at this row would skip it. The cursor is the last row we
			// actually returned.
			hasMore = true
		}
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	if hasMore && len(orders) > 0 {
		last := orders[len(orders)-1]
		nextCursor = EncodeOrderCursor(last.CreatedAt, last.ID)
	}

	for i := range orders {
		items, err := r.fetchItems(ctx, orders[i].ID)
		if err != nil {
			return nil, "", err
		}
		orders[i].Items = items
	}

	return orders, nextCursor, nil
}

func (r *Repository) AddItem(ctx context.Context, orderID uuid.UUID, item model.OrderItem, clearShipping bool) error {
	item.ID = uuid.New()
	item.OrderID = orderID
	jumlah := item.UnitPrice * float64(item.Qty)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO order_item (id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())`,
		item.ID, item.OrderID, item.ProductID, item.ProductType, item.Name, item.UnitPrice, item.Qty, jumlah, item.WeightGrams,
	)
	if err != nil {
		return err
	}
	if clearShipping {
		_, err = r.pool.Exec(ctx, `
			UPDATE orders SET
			  subtotal   = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0),
			  total      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0) - discount,
			  shipping_cost = 0,
			  selected_courier = '',
			  selected_service = '',
			  updated_at = now()
			WHERE id = $1`, orderID)
		return err
	}
	return r.recalcOrderTotals(ctx, orderID)
}

func (r *Repository) RemoveItem(ctx context.Context, orderID, itemID uuid.UUID, clearShipping bool) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM order_item WHERE id = $1 AND order_id = $2`,
		itemID, orderID,
	)
	if err != nil {
		return err
	}
	if clearShipping {
		_, err = r.pool.Exec(ctx, `
			UPDATE orders SET
			  subtotal   = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0),
			  total      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0) - discount,
			  shipping_cost = 0,
			  selected_courier = '',
			  selected_service = '',
			  updated_at = now()
			WHERE id = $1`, orderID)
		return err
	}
	return r.recalcOrderTotals(ctx, orderID)
}

func (r *Repository) UpdateItemQty(ctx context.Context, orderID, itemID uuid.UUID, qty int, clearShipping bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE order_item SET qty = $1, jumlah = unit_price * $2 WHERE id = $3 AND order_id = $4`,
		qty, qty, itemID, orderID,
	)
	if err != nil {
		return err
	}
	if clearShipping {
		_, err = r.pool.Exec(ctx, `
			UPDATE orders SET
			  subtotal   = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0),
			  total      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0) - discount,
			  shipping_cost = 0,
			  selected_courier = '',
			  selected_service = '',
			  updated_at = now()
			WHERE id = $1`, orderID)
		return err
	}
	return r.recalcOrderTotals(ctx, orderID)
}

func (r *Repository) recalcOrderTotals(ctx context.Context, orderID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE orders SET
		  subtotal   = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0),
		  total      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = $1), 0) - discount + shipping_cost,
		  updated_at = now()
		WHERE id = $1`, orderID)
	return err
}

func (r *Repository) PatchCart(ctx context.Context, orderID uuid.UUID, patch OrderPatch) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders
		 SET shipping_address = COALESCE($1, shipping_address), selected_courier = $2, selected_service = $3, promo_code_id = $4,
		     discount = $5, shipping_cost = $6, total = $7,
		     province_id = COALESCE($8, province_id), city_id = COALESCE($9, city_id), district_id = COALESCE($10, district_id), kode_pos = COALESCE($11, kode_pos),
		     is_estimate = $12,
		     courier_code = $13, courier_service_code = $14,
		     updated_at = now()
		 WHERE id = $15`,
		patch.ShippingAddress, patch.SelectedCourier, patch.SelectedService, patch.PromoCodeID,
		patch.Discount, patch.ShippingCost, patch.Total,
		patch.ProvinceID, patch.CityID, patch.DistrictID, patch.KodePos,
		patch.IsEstimate,
		patch.CourierCode, patch.CourierServiceCode,
		orderID,
	)
	return err
}

func (r *Repository) SetOrderStatus(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, status, reason string) error {
	q := `UPDATE orders SET
		status = $1,
		cancellation_reason = $2,
		paid_at       = CASE WHEN $1 = 'paid'      THEN now() ELSE paid_at      END,
		completed_at  = CASE WHEN $1 = 'completed' THEN now() ELSE completed_at END,
		cancelled_at  = CASE WHEN $1 = 'cancelled' THEN now() ELSE cancelled_at END,
		updated_at    = now()
		WHERE id = $3`
	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, q, status, reason, orderID)
	} else {
		_, err = r.pool.Exec(ctx, q, status, reason, orderID)
	}
	return err
}

// SetOrderManualPayment writes the manual-confirmation payment projection
// (payment_method = 'manual' plus the proof object key) inside the caller's
// transaction, alongside the status flip and audit row AdminConfirmOrder
// writes in the same tx — invariant 6 requires all three to commit together.
func (r *Repository) SetOrderManualPayment(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, proofURL string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET payment_method = 'manual', payment_proof_url = $1, updated_at = now() WHERE id = $2`,
		proofURL, orderID,
	)
	return err
}

// SetOrderRefundProof records the manual transfer receipt for a refund inside
// the caller's transaction, alongside the status flip, enrollment revocation
// and audit row AdminRefundOrder writes in the same tx. The money itself is
// moved by a human — this is the only evidence the system ever gets.
func (r *Repository) SetOrderRefundProof(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, proofURL string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET refund_proof_url = $1, updated_at = now() WHERE id = $2`,
		proofURL, orderID,
	)
	return err
}

// SetOrderPaymentMethod writes the gateway-reported payment_type onto the
// order's payment_method projection inside the caller's transaction. Callers
// must only invoke this when method is non-empty — an empty value must never
// overwrite a payment_method already on the row (FR-29).
func (r *Repository) SetOrderPaymentMethod(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, method string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET payment_method = $1, updated_at = now() WHERE id = $2`,
		method, orderID,
	)
	return err
}

func (r *Repository) SetShipped(ctx context.Context, orderID uuid.UUID, trackingNumber string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET status = 'shipped', tracking_number = $1, shipped_at = now(), updated_at = now() WHERE id = $2`,
		trackingNumber, orderID,
	)
	return err
}

// SetShippedBiteship moves an order to shipped via the Biteship booking
// path (FR-C-6): tracking_number and biteship_order_id come from Biteship's
// response, waybill_source is stamped 'biteship'.
func (r *Repository) SetShippedBiteship(ctx context.Context, orderID uuid.UUID, trackingNumber, biteshipOrderID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET status = 'shipped', tracking_number = $1, biteship_order_id = $2,
		     waybill_source = 'biteship', shipped_at = now(), updated_at = now()
		 WHERE id = $3`,
		trackingNumber, biteshipOrderID, orderID,
	)
	return err
}

// SetShippedManual moves an order to shipped via the manual-resi escape
// hatch (FR-C-9): no Biteship call is made, waybill_source is stamped
// 'manual'.
func (r *Repository) SetShippedManual(ctx context.Context, orderID uuid.UUID, trackingNumber string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET status = 'shipped', tracking_number = $1,
		     waybill_source = 'manual', shipped_at = now(), updated_at = now()
		 WHERE id = $2`,
		trackingNumber, orderID,
	)
	return err
}

// GetOrderByBiteshipOrderID looks up an order by the Biteship-side order id
// so an inbound webhook (FR-C-12) can be matched to our row. Following this
// file's not-found convention, an unmatched id returns a zero-value Order
// with a nil error rather than a sentinel — callers distinguish via
// order.ID == uuid.Nil (see GetOrderByID/GetCartByStudentID above).
func (r *Repository) GetOrderByBiteshipOrderID(ctx context.Context, biteshipOrderID string) (model.Order, error) {
	order := model.Order{}
	err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE biteship_order_id = $1`,
		biteshipOrderID,
	), &order)
	if err != nil {
		if isNotFound(err) {
			return model.Order{}, nil
		}
		return model.Order{}, err
	}

	items, err := r.fetchItems(ctx, order.ID)
	if err != nil {
		return model.Order{}, err
	}
	order.Items = items
	return order, nil
}

// SetShipmentStatus writes the authoritative status from a Biteship
// GET /v1/orders/:id re-fetch (FR-C-12) onto the order row. It intentionally
// does not touch orders.status — completion stays a manual admin action
// (FR-C-15).
func (r *Repository) SetShipmentStatus(ctx context.Context, orderID uuid.UUID, shipmentStatus string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET shipment_status = $1, updated_at = now() WHERE id = $2`,
		shipmentStatus, orderID,
	)
	return err
}

// SetTrackingNumber replaces the waybill on the order row itself. Couriers
// can issue the real resi after the booking is confirmed (Biteship's
// order.waybill_id event), and admin, the student order page and the packing
// slip all read this column — never order_shipment_events.
func (r *Repository) SetTrackingNumber(ctx context.Context, orderID uuid.UUID, trackingNumber string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET tracking_number = $1, updated_at = now() WHERE id = $2`,
		trackingNumber, orderID,
	)
	return err
}

func (r *Repository) SetPaymentRef(ctx context.Context, orderID uuid.UUID, ref string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET gateway_ref = $1, payment_expires_at = $2, checked_out_at = now(), updated_at = now() WHERE id = $3`,
		ref, expiresAt, orderID,
	)
	return err
}

func (r *Repository) GetExpiredPaymentOrders(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM orders
		WHERE status = 'payment_pending'
		  AND payment_expires_at < now()
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orderIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, id)
	}
	return orderIDs, rows.Err()
}

func (r *Repository) CheckoutOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	rows, err := tx.Query(ctx,
		`SELECT id, order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams, fulfilled_at, created_at
		 FROM order_item WHERE order_id = $1`,
		orderID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		item := model.OrderItem{}
		err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductType,
			&item.Name, &item.UnitPrice, &item.Qty, &item.Jumlah, &item.WeightGrams, &item.FulfilledAt, &item.CreatedAt)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	// Only enforce stock for physical inventory (book, merchandise, medal); course and exam
	// products have no inventory constraint. Physical-type classification is a
	// pre-existing chokepoint widened in place (logic normally lives in the service
	// layer, but inlined here to avoid a repo→service import).
	for _, item := range items {
		if item.ProductType != "book" && item.ProductType != "merchandise" && item.ProductType != "medal" {
			continue
		}
		var currentStock int
		err := tx.QueryRow(ctx,
			`SELECT stock FROM product WHERE id = $1 FOR UPDATE`,
			item.ProductID,
		).Scan(&currentStock)
		if err != nil {
			return err
		}
		if currentStock < item.Qty {
			return ErrInsufficientStock
		}
		_, err = tx.Exec(ctx,
			`UPDATE product SET stock = $1, updated_at = now() WHERE id = $2`,
			currentStock-item.Qty, item.ProductID,
		)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = 'payment_pending', updated_at = now() WHERE id = $1`,
		orderID,
	)
	return err
}

func (r *Repository) InsertWebhookLog(ctx context.Context, tx pgx.Tx, eventType string, payload []byte, gatewayRef string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO webhook_log (event_type, payload, gateway_ref) VALUES ($1, $2, $3)`,
		eventType, payload, gatewayRef,
	)
	return err
}

func (r *Repository) ClearOrderTracking(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET tracking_number = '', shipped_at = NULL, updated_at = now() WHERE id = $1`,
		orderID,
	)
	return err
}

// GetRevenue reports money for orders created in [from, to).
//
// Two aggregations, deliberately kept apart: `total` is what buyers were
// actually charged (orders.total, counted once per order), while by_type and
// product_revenue sum ITEM lines. They differ by shipping and discount, which
// live on the order. Summing orders.total once per joined item row is what
// inflated this report before — a three-item order counted its total
// three times, and landed the whole of it in every product type it touched.
func (r *Repository) GetRevenue(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	var (
		total      float64
		shipping   float64
		discount   float64
		orderCount int
	)
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total), 0), COALESCE(SUM(shipping_cost), 0),
		       COALESCE(SUM(discount), 0), COUNT(*)
		  FROM orders
		 WHERE status IN ('paid', 'processing', 'completed')
		   AND created_at >= $1 AND created_at < $2
	`, from, to).Scan(&total, &shipping, &discount, &orderCount)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT oi.product_type,
		       COALESCE(SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty)), 0),
		       COUNT(DISTINCT o.id)
		  FROM orders o
		  JOIN order_item oi ON oi.order_id = o.id
		 WHERE o.status IN ('paid', 'processing', 'completed')
		   AND o.created_at >= $1 AND o.created_at < $2
		 GROUP BY oi.product_type
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byType := map[string]interface{}{}
	var productRevenue float64

	for rows.Next() {
		var productType string
		var typeTotal float64
		var count int
		if err := rows.Scan(&productType, &typeTotal, &count); err != nil {
			return nil, err
		}
		productRevenue += typeTotal
		byType[productType] = map[string]interface{}{
			"total": typeTotal,
			"count": count,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":           total,
		"product_revenue": productRevenue,
		"shipping_total":  shipping,
		"discount_total":  discount,
		"order_count":     orderCount,
		"by_type":         byType,
	}, nil
}
