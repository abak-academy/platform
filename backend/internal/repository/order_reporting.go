package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidOrderBy = errors.New("invalid order by key")

type OrderBucketCounts struct {
	NeedsConfirm       int `json:"needs_confirm"`
	ReadyToShip        int `json:"ready_to_ship"`
	ShipmentFailed     int `json:"shipment_failed"`
	InTransit          int `json:"in_transit"`
	Shipped            int `json:"shipped"`
	Completed          int `json:"completed"`
	PaymentExpired     int `json:"payment_expired"`
	Cancelled          int `json:"cancelled"`
	CreatedThisMonth   int `json:"created_this_month"`
	CompletedThisMonth int `json:"completed_this_month"`
	Total              int `json:"total"`
}

type TopProduct struct {
	ProductID      uuid.UUID `json:"product_id"`
	Name           string    `json:"name"`
	ProductType    string    `json:"product_type"`
	QtySold        int       `json:"qty_sold"`
	OrderCount     int       `json:"order_count"`
	ProductRevenue float64   `json:"product_revenue"`
}

// CountOrdersByBucket answers "what needs me" in one pass.
//
// Every status bucket and Total run through the same predicate builder as
// ListOrders, so they always describe the rows the table is showing. The two
// month figures deliberately ignore the filters: they are the store dashboard's
// fixed month-to-date tiles and must not move when someone types in the orders
// search box.
//
// CreatedThisMonth is the "Pesanan masuk" tile — orders received, paid or not —
// so it keys on placedAtExpr and deliberately NOT on the revenueEventCTE ledger
// the money reports use. An unpaid order still arrived and still needs someone
// to look at it.
func (r *Repository) CountOrdersByBucket(
	ctx context.Context, filter OrderFilter, failureStatuses []string,
	monthStart, monthEnd time.Time,
) (OrderBucketCounts, error) {
	var out OrderBucketCounts
	where, args, nextArg := orderFilterPredicateFrom(filter, 3)
	bucketPredicate := func(bucket OrderFilter) string {
		predicate, bucketArgs, next := orderFilterPredicateFrom(bucket, nextArg)
		args = append(args, bucketArgs...)
		nextArg = next
		return strings.TrimPrefix(predicate, " AND ")
	}
	// The in-transit predicate COALESCEs the ANY test because shipment_status is
	// nullable and `NULL = ANY(...)` is NULL: a shipped parcel with no courier
	// status yet would otherwise fall out of both the failed and in-transit buckets.
	needsConfirm := bucketPredicate(OrderFilter{Status: "payment_pending"})
	readyToShip := bucketPredicate(OrderFilter{ReadyToShip: true})
	shipmentFailed := "false"
	if len(failureStatuses) > 0 {
		shipmentFailed = bucketPredicate(OrderFilter{ShipmentStatusIn: failureStatuses})
	}
	inTransit := bucketPredicate(OrderFilter{InTransit: true, ShipmentStatusNotIn: failureStatuses})
	shipped := bucketPredicate(OrderFilter{Status: "shipped"})
	completed := bucketPredicate(OrderFilter{Status: "completed"})
	paymentExpired := bucketPredicate(OrderFilter{Status: "payment_expired"})
	cancelled := bucketPredicate(OrderFilter{Status: "cancelled"})
	q := `SELECT
  COUNT(*) FILTER (WHERE ` + needsConfirm + `),
  COUNT(*) FILTER (WHERE ` + readyToShip + `),
  COUNT(*) FILTER (WHERE ` + shipmentFailed + `),
  COUNT(*) FILTER (WHERE ` + inTransit + `),
  COUNT(*) FILTER (WHERE ` + shipped + `),
  COUNT(*) FILTER (WHERE ` + completed + `),
  COUNT(*) FILTER (WHERE ` + paymentExpired + `),
  COUNT(*) FILTER (WHERE ` + cancelled + `),
  COUNT(*) FILTER (WHERE ` + placedAtExpr + ` >= $1 AND ` + placedAtExpr + ` < $2),
  COUNT(*) FILTER (WHERE completed_at >= $1 AND completed_at < $2),
  COUNT(*) FROM orders WHERE status != 'cart'` + where
	full := append([]interface{}{monthStart, monthEnd}, args...)
	err := r.pool.QueryRow(ctx, q, full...).Scan(
		&out.NeedsConfirm, &out.ReadyToShip, &out.ShipmentFailed, &out.InTransit,
		&out.Shipped, &out.Completed, &out.PaymentExpired, &out.Cancelled,
		&out.CreatedThisMonth, &out.CompletedThisMonth, &out.Total,
	)
	return out, err
}

// TopProducts ranks products over [from, to) off the same revenueEventCTE
// ledger as the revenue report, so the bars and the table cannot disagree.
// Money sums from the item line and orders are counted distinctly — quantity
// sold and order count are different questions. A refund subtracts the item
// lines it returned, so a product refunded more than it sold in a window can
// legitimately rank negative.
//
// orderBy is interpolated into the SQL, so it is an allowlist, not a passthrough.
func (r *Repository) TopProducts(
	ctx context.Context, from, to time.Time, orderBy string, limit int,
) ([]TopProduct, error) {
	var sortCol string
	switch orderBy {
	case "revenue":
		sortCol = "product_revenue"
	case "qty":
		sortCol = "qty_sold"
	default:
		return nil, ErrInvalidOrderBy
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH %s
		SELECT oi.product_id,
		       MAX(oi.name)         AS name,
		       MAX(oi.product_type) AS product_type,
		       SUM(e.sign * oi.qty) AS qty_sold,
		       COUNT(DISTINCT e.id) FILTER (WHERE e.sign = 1) AS order_count,
		       COALESCE(SUM(e.sign * COALESCE(oi.jumlah, oi.unit_price * oi.qty)), 0) AS product_revenue
		  FROM revenue_event e
		  JOIN order_item oi ON oi.order_id = e.id
		 WHERE e.at >= $1 AND e.at < $2
		 GROUP BY oi.product_id
		 ORDER BY %s DESC
		 LIMIT $3
	`, revenueEventCTE, sortCol), from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TopProduct{}
	for rows.Next() {
		var p TopProduct
		if err := rows.Scan(&p.ProductID, &p.Name, &p.ProductType,
			&p.QtySold, &p.OrderCount, &p.ProductRevenue); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) ListOrderCouriers(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT courier_code FROM orders
  WHERE status != 'cart' AND courier_code IS NOT NULL AND courier_code != '' ORDER BY courier_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	couriers := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		couriers = append(couriers, code)
	}
	return couriers, rows.Err()
}
