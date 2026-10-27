package repository

import (
	"context"
	"fmt"
	"time"
)

// AffiliateReportQuery carries the period and paging every report shares. From
// and To bound paid_at for sales and created_at for order counts; nil is open.
type AffiliateReportQuery struct {
	From   *time.Time
	To     *time.Time
	Q      string
	Limit  int
	Offset int
	// InactiveOnly keeps affiliators with no order created in the period.
	InactiveOnly bool
}

func (q AffiliateReportQuery) paging() (limit, offset int) {
	limit, offset = q.Limit, q.Offset
	if limit <= 0 {
		limit = 20
	}
	if limit > 10000 {
		limit = 10000
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// period renders "col is inside [$1, $2)" for the shared from/to parameters.
func period(col string) string {
	return fmt.Sprintf("($1::timestamptz IS NULL OR %[1]s >= $1) AND ($2::timestamptz IS NULL OR %[1]s < $2)", col)
}

const reportPaid = `o.paid_at IS NOT NULL AND o.cancellation_reason IS DISTINCT FROM 'refunded'`

// AffiliatorReportRow is one affiliator's results for the period. Pending
// figures are the current backlog; expired and order counts follow the period.
type AffiliatorReportRow struct {
	AffiliatorID  string     `json:"affiliator_id"`
	Name          string     `json:"name"`
	Email         *string    `json:"email"`
	Status        string     `json:"status"`
	SchoolCount   int        `json:"school_count"`
	StudentCount  int        `json:"student_count"`
	PaidOrders    int        `json:"paid_orders"`
	PaidUnits     int        `json:"paid_units"`
	PaidAmount    float64    `json:"paid_amount"`
	DiscountTotal float64    `json:"discount_total"`
	PendingOrders int        `json:"pending_orders"`
	PendingAmount float64    `json:"pending_amount"`
	ExpiredOrders int        `json:"expired_orders"`
	OrdersInRange int        `json:"orders_in_period"`
	LastOrderAt   *time.Time `json:"last_order_at"`
}

// AffiliatorReport returns one row per affiliator, best sellers first, with the total count.
func (r *Repository) AffiliatorReport(ctx context.Context, q AffiliateReportQuery) ([]AffiliatorReportRow, int, error) {
	limit, offset := q.paging()
	paidIn := reportPaid + " AND " + period("o.paid_at")
	createdIn := period("o.created_at")
	sql := `WITH r AS (
		SELECT u.id::text AS id, u.name, u.email, u.status, cardinality(u.school_ids) AS school_count,
			(SELECT COUNT(*) FROM users st WHERE st.role = 'student' AND st.status != 'deleted' AND st.school_id = ANY(u.school_ids)) AS student_count,
			COUNT(o.id) FILTER (WHERE ` + paidIn + `) AS paid_orders,
			COALESCE(SUM(oi.qty) FILTER (WHERE ` + paidIn + `), 0) AS paid_units,
			COALESCE(SUM(o.total) FILTER (WHERE ` + paidIn + `), 0) AS paid_amount,
			COALESCE(SUM(o.discount) FILTER (WHERE ` + paidIn + `), 0) AS discount_total,
			COUNT(o.id) FILTER (WHERE o.status = 'payment_pending') AS pending_orders,
			COALESCE(SUM(o.total) FILTER (WHERE o.status = 'payment_pending'), 0) AS pending_amount,
			COUNT(o.id) FILTER (WHERE o.status = 'payment_expired' AND ` + createdIn + `) AS expired_orders,
			COUNT(o.id) FILTER (WHERE ` + createdIn + `) AS orders_in_period,
			MAX(o.created_at) AS last_order_at
		FROM users u
		LEFT JOIN orders o ON o.student_id = u.id AND o.status <> 'cart'
		LEFT JOIN LATERAL (SELECT COALESCE(SUM(qty), 0) AS qty FROM order_item WHERE order_id = o.id) oi ON true
		WHERE u.role = 'affiliator' AND u.status != 'deleted'
		  AND ($3 = '' OR u.name ILIKE '%' || $3 || '%' OR u.email ILIKE '%' || $3 || '%')
		GROUP BY u.id
	)
	SELECT r.*, COUNT(*) OVER () FROM r
	WHERE ($4 = false OR r.orders_in_period = 0)
	ORDER BY r.paid_amount DESC, r.name, r.id
	LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, sql, q.From, q.To, q.Q, q.InactiveOnly, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AffiliatorReportRow{}
	total := 0
	for rows.Next() {
		var a AffiliatorReportRow
		if err := rows.Scan(&a.AffiliatorID, &a.Name, &a.Email, &a.Status, &a.SchoolCount, &a.StudentCount, &a.PaidOrders, &a.PaidUnits,
			&a.PaidAmount, &a.DiscountTotal, &a.PendingOrders, &a.PendingAmount, &a.ExpiredOrders, &a.OrdersInRange, &a.LastOrderAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// SchoolReportRow is one assigned school: coverage of its students and the sales to it.
type SchoolReportRow struct {
	SchoolID       string  `json:"school_id"`
	SchoolName     string  `json:"school_name"`
	Affiliators    int     `json:"affiliator_count"`
	Students       int     `json:"student_count"`
	CoveredStudent int     `json:"covered_students"`
	Unpurchased    int     `json:"unpurchased_students"`
	PaidOrders     int     `json:"paid_orders"`
	PaidUnits      int     `json:"paid_units"`
	PaidAmount     float64 `json:"paid_amount"`
	DiscountTotal  float64 `json:"discount_total"`
}

// SchoolReport returns every school assigned to at least one affiliator. Coverage
// counts students inside a live order (not cart, cancelled or expired) of any
// affiliator, whatever the period; sales follow the period. An order belongs to
// the school of its participants, and an affiliator order is single-school.
func (r *Repository) SchoolReport(ctx context.Context, q AffiliateReportQuery) ([]SchoolReportRow, int, error) {
	limit, offset := q.paging()
	paidIn := reportPaid + " AND " + period("o.paid_at")
	sql := `WITH assigned AS (
		SELECT s.id, s.name, COUNT(DISTINCT a.id) AS affiliators
		FROM school s JOIN users a ON a.role = 'affiliator' AND a.status != 'deleted' AND s.id = ANY(a.school_ids)
		WHERE ($3 = '' OR s.name ILIKE '%' || $3 || '%')
		GROUP BY s.id, s.name
	), sales AS (
		SELECT o.id, o.total, o.discount,
			(SELECT us.school_id FROM order_participant op JOIN users us ON us.id = op.student_id WHERE op.order_id = o.id LIMIT 1) AS school_id,
			(SELECT COALESCE(SUM(qty), 0) FROM order_item WHERE order_id = o.id) AS units
		FROM orders o JOIN users a ON a.id = o.student_id AND a.role = 'affiliator'
		WHERE ` + paidIn + `
	), r AS (
		SELECT s.id::text AS id, s.name, s.affiliators,
			(SELECT COUNT(*) FROM users st WHERE st.role = 'student' AND st.status != 'deleted' AND st.school_id = s.id) AS students,
			(SELECT COUNT(DISTINCT op.student_id) FROM order_participant op
				JOIN orders po ON po.id = op.order_id AND po.status NOT IN ('cart', 'cancelled', 'payment_expired')
				JOIN users pa ON pa.id = po.student_id AND pa.role = 'affiliator'
				JOIN users us ON us.id = op.student_id AND us.school_id = s.id AND us.status != 'deleted') AS covered,
			(SELECT COUNT(*) FROM sales WHERE sales.school_id = s.id) AS paid_orders,
			(SELECT COALESCE(SUM(units), 0) FROM sales WHERE sales.school_id = s.id) AS paid_units,
			(SELECT COALESCE(SUM(total), 0) FROM sales WHERE sales.school_id = s.id) AS paid_amount,
			(SELECT COALESCE(SUM(discount), 0) FROM sales WHERE sales.school_id = s.id) AS discount_total
		FROM assigned s
	)
	SELECT r.id, r.name, r.affiliators, r.students, r.covered, GREATEST(r.students - r.covered, 0),
		r.paid_orders, r.paid_units, r.paid_amount, r.discount_total, COUNT(*) OVER ()
	FROM r ORDER BY r.paid_amount DESC, r.name, r.id LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, sql, q.From, q.To, q.Q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []SchoolReportRow{}
	total := 0
	for rows.Next() {
		var s SchoolReportRow
		if err := rows.Scan(&s.SchoolID, &s.SchoolName, &s.Affiliators, &s.Students, &s.CoveredStudent, &s.Unpurchased,
			&s.PaidOrders, &s.PaidUnits, &s.PaidAmount, &s.DiscountTotal, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// ProductReportRow is units of one product sold to one school, one unit per participating student.
type ProductReportRow struct {
	SchoolID    string `json:"school_id"`
	SchoolName  string `json:"school_name"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	ProductType string `json:"product_type"`
	Units       int    `json:"units"`
	Orders      int    `json:"orders"`
}

// ProductReport returns best-selling products per school across all affiliators.
func (r *Repository) ProductReport(ctx context.Context, q AffiliateReportQuery) ([]ProductReportRow, int, error) {
	limit, offset := q.paging()
	paidIn := reportPaid + " AND " + period("o.paid_at")
	sql := `WITH r AS (
		SELECT s.id::text AS school_id, s.name AS school_name, oi.product_id::text AS product_id, oi.name AS product_name,
			oi.product_type, COUNT(*) AS units, COUNT(DISTINCT o.id) AS orders
		FROM orders o
		JOIN users a ON a.id = o.student_id AND a.role = 'affiliator'
		JOIN order_item oi ON oi.order_id = o.id
		JOIN order_participant op ON op.order_id = o.id
		JOIN users us ON us.id = op.student_id
		JOIN school s ON s.id = us.school_id
		WHERE ` + paidIn + `
		  AND ($3 = '' OR s.name ILIKE '%' || $3 || '%' OR oi.name ILIKE '%' || $3 || '%')
		GROUP BY s.id, s.name, oi.product_id, oi.name, oi.product_type
	)
	SELECT r.*, COUNT(*) OVER () FROM r ORDER BY r.units DESC, r.school_name, r.product_name LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, sql, q.From, q.To, q.Q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ProductReportRow{}
	total := 0
	for rows.Next() {
		var p ProductReportRow
		if err := rows.Scan(&p.SchoolID, &p.SchoolName, &p.ProductID, &p.ProductName, &p.ProductType, &p.Units, &p.Orders, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// PromoReportRow is one promo code's use on affiliator orders in the period.
// UsedCount and MaxUses are lifetime counters, not period figures.
type PromoReportRow struct {
	Code           string     `json:"code"`
	AffiliatorOnly bool       `json:"affiliator_only"`
	DiscountPct    *float64   `json:"discount_percent"`
	DiscountAmount *float64   `json:"discount_amount"`
	MinQty         *int       `json:"min_qty"`
	MinOrderAmount *float64   `json:"min_order_amount"`
	StartsAt       *time.Time `json:"starts_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	MaxUses        *int       `json:"max_uses"`
	UsedCount      int        `json:"used_count"`
	ActiveNow      bool       `json:"active_now"`
	Orders         int        `json:"orders"`
	DiscountTotal  float64    `json:"discount_total"`
}

// PromoReport lists affiliator-only promos and any promo used on an affiliator
// order, with how much each was used in the period.
func (r *Repository) PromoReport(ctx context.Context, q AffiliateReportQuery) ([]PromoReportRow, int, error) {
	limit, offset := q.paging()
	paidIn := reportPaid + " AND " + period("o.paid_at")
	sql := `WITH r AS (
		SELECT p.code, p.affiliator_only, p.discount_percent, p.discount_amount, p.min_qty, p.min_order_amount,
			p.starts_at, p.expires_at, p.max_uses, p.used_count,
			(p.starts_at IS NULL OR p.starts_at <= now()) AND (p.expires_at IS NULL OR p.expires_at > now())
				AND (p.max_uses IS NULL OR p.used_count < p.max_uses) AS active_now,
			COUNT(o.id) FILTER (WHERE ` + paidIn + `) AS orders,
			COALESCE(SUM(o.discount) FILTER (WHERE ` + paidIn + `), 0) AS discount_total
		FROM promo_code p
		LEFT JOIN orders o ON o.promo_code_id = p.id AND o.status <> 'cart'
			AND EXISTS (SELECT 1 FROM users a WHERE a.id = o.student_id AND a.role = 'affiliator')
		WHERE ($3 = '' OR p.code ILIKE '%' || $3 || '%')
		GROUP BY p.id
		HAVING p.affiliator_only OR COUNT(o.id) > 0
	)
	SELECT r.*, COUNT(*) OVER () FROM r ORDER BY r.orders DESC, r.code LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, sql, q.From, q.To, q.Q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []PromoReportRow{}
	total := 0
	for rows.Next() {
		var p PromoReportRow
		if err := rows.Scan(&p.Code, &p.AffiliatorOnly, &p.DiscountPct, &p.DiscountAmount, &p.MinQty, &p.MinOrderAmount,
			&p.StartsAt, &p.ExpiresAt, &p.MaxUses, &p.UsedCount, &p.ActiveNow, &p.Orders, &p.DiscountTotal, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// TrendReportRow is one month of paid affiliator sales.
type TrendReportRow struct {
	Month       string  `json:"month"`
	Orders      int     `json:"orders"`
	Units       int     `json:"units"`
	Amount      float64 `json:"amount"`
	Discount    float64 `json:"discount"`
	Affiliators int     `json:"active_affiliators"`
}

// TrendReport returns the latest months (Asia/Jakarta) of paid affiliator sales, oldest first.
func (r *Repository) TrendReport(ctx context.Context, q AffiliateReportQuery) ([]TrendReportRow, error) {
	paidIn := reportPaid + " AND " + period("o.paid_at")
	sql := `SELECT * FROM (
		SELECT to_char(date_trunc('month', o.paid_at AT TIME ZONE 'Asia/Jakarta'), 'YYYY-MM') AS month,
			COUNT(*), COALESCE(SUM((SELECT SUM(qty) FROM order_item WHERE order_id = o.id)), 0),
			COALESCE(SUM(o.total), 0), COALESCE(SUM(o.discount), 0), COUNT(DISTINCT o.student_id)
		FROM orders o JOIN users a ON a.id = o.student_id AND a.role = 'affiliator'
		WHERE ` + paidIn + `
		GROUP BY 1 ORDER BY 1 DESC LIMIT 24
	) t ORDER BY month`
	rows, err := r.pool.Query(ctx, sql, q.From, q.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrendReportRow{}
	for rows.Next() {
		var t TrendReportRow
		if err := rows.Scan(&t.Month, &t.Orders, &t.Units, &t.Amount, &t.Discount, &t.Affiliators); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
