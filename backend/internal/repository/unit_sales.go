package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
)

type UnitSalesRow struct {
	Type      string
	ProductID *string
	Name      string
	model.UnitSalesFigures
}

func (r *Repository) UnitSales(ctx context.Context, from, to *time.Time) ([]UnitSalesRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`WITH %s
 SELECT oi.product_type, oi.product_id::text, COALESCE(MAX(oi.name), ''),
        COUNT(DISTINCT e.id) FILTER (WHERE e.sign = 1),
        COUNT(DISTINCT e.id) FILTER (WHERE e.sign = -1),
        COALESCE(SUM(oi.qty) FILTER (WHERE e.sign = 1), 0),
        COALESCE(SUM(oi.qty) FILTER (WHERE e.sign = -1), 0)
 FROM revenue_event e JOIN order_item oi ON oi.order_id = e.id
 WHERE ($1::timestamptz IS NULL OR e.at >= $1)
   AND ($2::timestamptz IS NULL OR e.at < $2)
 GROUP BY GROUPING SETS ((oi.product_type), (oi.product_type, oi.product_id))`, revenueEventCTE), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UnitSalesRow{}
	for rows.Next() {
		var row UnitSalesRow
		if err := rows.Scan(&row.Type, &row.ProductID, &row.Name, &row.PaidOrders, &row.RefundOrders, &row.PaidUnits, &row.RefundUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
