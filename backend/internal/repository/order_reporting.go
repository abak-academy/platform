package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidOrderBy = errors.New("invalid order by key")

type OrderBucketCounts struct {
	NeedsConfirm       int `json:"needs_confirm"`
	ReadyToShip        int `json:"ready_to_ship"`
	ShipmentFailed     int `json:"shipment_failed"`
	InTransit          int `json:"in_transit"`
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
// The first four buckets and Total run through the same predicate builder as
// ListOrders, so they always describe the rows the table is showing. The two
// month figures deliberately ignore the filters: they are the store dashboard's
// fixed month-to-date tiles and must not move when someone types in the orders
// search box.
func (r *Repository) CountOrdersByBucket(
	ctx context.Context, filter OrderFilter, failureStatuses []string,
	monthStart, monthEnd time.Time,
) (OrderBucketCounts, error) {
	var out OrderBucketCounts

	// $1..$3 are bound here; the shared predicate continues from $4.
	where, args, _ := orderFilterPredicateFrom(filter, 4)

	// COALESCE around the ANY test because shipment_status is nullable, and
	// `NULL = ANY(...)` is NULL — a shipped parcel with no courier status yet
	// would otherwise fall out of both the failed and the in-transit bucket.
	//
	// ready_to_ship also requires a physical item: the orders page gates its Ship
	// action on one, so counting digital-only orders here would promise work no
	// row could actually offer.
	q := `
		SELECT
		  COUNT(*) FILTER (WHERE status = 'payment_pending'),
		  COUNT(*) FILTER (WHERE status IN ('paid','processing')
		                     AND EXISTS (SELECT 1 FROM order_item oi
		                                  WHERE oi.order_id = orders.id
		                                    AND oi.product_type IN ('book','merchandise','medal'))),
		  COUNT(*) FILTER (WHERE COALESCE(shipment_status = ANY($1), false)),
		  COUNT(*) FILTER (WHERE status = 'shipped'
		                     AND NOT COALESCE(shipment_status = ANY($1), false)),
		  COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
		  COUNT(*) FILTER (WHERE completed_at >= $2 AND completed_at < $3),
		  COUNT(*)
		FROM orders
		WHERE status != 'cart'` + where

	full := append([]interface{}{failureStatuses, monthStart, monthEnd}, args...)

	err := r.pool.QueryRow(ctx, q, full...).Scan(
		&out.NeedsConfirm, &out.ReadyToShip, &out.ShipmentFailed, &out.InTransit,
		&out.CreatedThisMonth, &out.CompletedThisMonth, &out.Total,
	)
	return out, err
}

// TopProducts ranks products over [from, to) using the same status set as the
// revenue report, so the bars and the table cannot disagree. Money sums from the
// item line and orders are counted distinctly — quantity sold and order count
// are different questions.
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
		SELECT oi.product_id,
		       MAX(oi.name)         AS name,
		       MAX(oi.product_type) AS product_type,
		       SUM(oi.qty)          AS qty_sold,
		       COUNT(DISTINCT o.id) AS order_count,
		       COALESCE(SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty)), 0) AS product_revenue
		  FROM orders o
		  JOIN order_item oi ON oi.order_id = o.id
		 WHERE o.status IN %s
		   AND o.created_at >= $1 AND o.created_at < $2
		 GROUP BY oi.product_id
		 ORDER BY %s DESC
		 LIMIT $3
	`, paidStatusList, sortCol), from, to, limit)
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
