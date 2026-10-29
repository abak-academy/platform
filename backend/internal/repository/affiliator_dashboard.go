package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AffiliatorSchoolCount is one allowed school with its student headcount.
type AffiliatorSchoolCount struct {
	SchoolID     string `json:"school_id"`
	SchoolName   string `json:"school_name"`
	StudentCount int    `json:"student_count"`
}

// AffiliatorOrderTotals summarises an affiliator's orders in a period.
// Paid figures count orders that were paid in the period and not refunded;
// pending orders are the ones still waiting for payment right now.
type AffiliatorOrderTotals struct {
	PaidOrders    int     `json:"paid_orders"`
	PaidUnits     int     `json:"paid_units"`
	PaidAmount    float64 `json:"paid_amount"`
	DiscountTotal float64 `json:"discount_total"`
	PendingOrders int     `json:"pending_orders"`
}

// AffiliatorMonthPoint is one month of the purchase trend.
type AffiliatorMonthPoint struct {
	Month  string  `json:"month"`
	Orders int     `json:"orders"`
	Units  int     `json:"units"`
	Amount float64 `json:"amount"`
}

// AffiliatorSchoolProduct is units sold of one product to one school.
type AffiliatorSchoolProduct struct {
	SchoolID    string `json:"school_id"`
	SchoolName  string `json:"school_name"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	ProductType string `json:"product_type"`
	Units       int    `json:"units"`
}

// AffiliatorOrderRow is one order in the affiliator's order list.
type AffiliatorOrderRow struct {
	ID               string     `json:"id"`
	Status           string     `json:"status"`
	Subtotal         float64    `json:"subtotal"`
	Discount         float64    `json:"discount"`
	Total            float64    `json:"total"`
	ProductName      string     `json:"product_name"`
	ProductType      string     `json:"product_type"`
	ParticipantCount int        `json:"participant_count"`
	SchoolName       *string    `json:"school_name"`
	PromoCode        *string    `json:"promo_code"`
	PaymentExpiresAt *time.Time `json:"payment_expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// AffiliatorOrderFilter narrows ListAffiliatorOrders.
type AffiliatorOrderFilter struct {
	Status string
	Cursor string
	Limit  int
}

// AffiliatorSchoolCounts returns the active-or-not student headcount of each school.
func (r *Repository) AffiliatorSchoolCounts(ctx context.Context, schoolIDs []string) ([]AffiliatorSchoolCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id::text, s.name,
			(SELECT COUNT(*) FROM users u WHERE u.school_id = s.id AND u.role = 'student' AND u.status != 'deleted')
		 FROM school s WHERE s.id = ANY($1::uuid[]) ORDER BY s.name`,
		schoolIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AffiliatorSchoolCount{}
	for rows.Next() {
		var c AffiliatorSchoolCount
		if err := rows.Scan(&c.SchoolID, &c.SchoolName, &c.StudentCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const affiliatorPaidPredicate = `o.paid_at IS NOT NULL AND o.cancellation_reason IS DISTINCT FROM 'refunded'`

// AffiliatorTotals aggregates the affiliator's orders; from and to bound paid_at when set.
func (r *Repository) AffiliatorTotals(ctx context.Context, affiliatorID uuid.UUID, from, to *time.Time) (AffiliatorOrderTotals, error) {
	var t AffiliatorOrderTotals
	err := r.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE `+affiliatorPaidPredicate+` AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)),
			COALESCE(SUM((SELECT SUM(oi.qty) FROM order_item oi WHERE oi.order_id = o.id)) FILTER (WHERE `+affiliatorPaidPredicate+` AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)), 0),
			COALESCE(SUM(o.total) FILTER (WHERE `+affiliatorPaidPredicate+` AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)), 0),
			COALESCE(SUM(o.discount) FILTER (WHERE `+affiliatorPaidPredicate+` AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)), 0),
			COUNT(*) FILTER (WHERE o.status = 'payment_pending')
		 FROM orders o WHERE o.student_id = $1`,
		affiliatorID, from, to,
	).Scan(&t.PaidOrders, &t.PaidUnits, &t.PaidAmount, &t.DiscountTotal, &t.PendingOrders)
	return t, err
}

// AffiliatorMonthly returns the paid purchase trend per month (Asia/Jakarta), oldest first.
func (r *Repository) AffiliatorMonthly(ctx context.Context, affiliatorID uuid.UUID, from, to *time.Time) ([]AffiliatorMonthPoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT to_char(date_trunc('month', o.paid_at AT TIME ZONE 'Asia/Jakarta'), 'YYYY-MM'),
			COUNT(*),
			COALESCE(SUM((SELECT SUM(oi.qty) FROM order_item oi WHERE oi.order_id = o.id)), 0),
			COALESCE(SUM(o.total), 0)
		 FROM orders o
		 WHERE o.student_id = $1 AND `+affiliatorPaidPredicate+`
		   AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)
		 GROUP BY 1 ORDER BY 1`,
		affiliatorID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AffiliatorMonthPoint{}
	for rows.Next() {
		var p AffiliatorMonthPoint
		if err := rows.Scan(&p.Month, &p.Orders, &p.Units, &p.Amount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AffiliatorTopProducts returns units sold per school and product, one unit per
// participating student, best sellers first.
func (r *Repository) AffiliatorTopProducts(ctx context.Context, affiliatorID uuid.UUID, from, to *time.Time, limit int) ([]AffiliatorSchoolProduct, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx,
		`SELECT s.id::text, s.name, oi.product_id::text, oi.name, oi.product_type, COUNT(*)
		 FROM orders o
		 JOIN order_item oi ON oi.order_id = o.id
		 JOIN order_participant op ON op.order_id = o.id
		 JOIN users u ON u.id = op.student_id
		 JOIN school s ON s.id = u.school_id
		 WHERE o.student_id = $1 AND `+affiliatorPaidPredicate+`
		   AND ($2::timestamptz IS NULL OR o.paid_at >= $2) AND ($3::timestamptz IS NULL OR o.paid_at < $3)
		 GROUP BY s.id, s.name, oi.product_id, oi.name, oi.product_type
		 ORDER BY COUNT(*) DESC, s.name, oi.name
		 LIMIT $4`,
		affiliatorID, from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AffiliatorSchoolProduct{}
	for rows.Next() {
		var p AffiliatorSchoolProduct
		if err := rows.Scan(&p.SchoolID, &p.SchoolName, &p.ProductID, &p.ProductName, &p.ProductType, &p.Units); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListAffiliatorOrders returns the affiliator's orders other than open carts,
// newest first, cursor-paginated.
func (r *Repository) ListAffiliatorOrders(ctx context.Context, affiliatorID uuid.UUID, f AffiliatorOrderFilter) ([]AffiliatorOrderRow, string, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	var cursorAt time.Time
	cursorID := uuid.Nil
	if f.Cursor != "" {
		at, id, err := DecodeOrderCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		cursorAt, cursorID = at, id
	}

	rows, err := r.pool.Query(ctx,
		`SELECT o.id::text, o.status, o.subtotal, o.discount, o.total,
			COALESCE((SELECT oi.name FROM order_item oi WHERE oi.order_id = o.id ORDER BY oi.created_at LIMIT 1), ''),
			COALESCE((SELECT oi.product_type FROM order_item oi WHERE oi.order_id = o.id ORDER BY oi.created_at LIMIT 1), ''),
			(SELECT COUNT(*) FROM order_participant op WHERE op.order_id = o.id),
			(SELECT string_agg(sn.name, ', ' ORDER BY sn.name) FROM
				(SELECT DISTINCT s.name FROM order_participant op JOIN users u ON u.id = op.student_id JOIN school s ON s.id = u.school_id
				  WHERE op.order_id = o.id) sn),
			pc.code, o.payment_expires_at, o.created_at
		 FROM orders o
		 LEFT JOIN promo_code pc ON pc.id = o.promo_code_id
		 WHERE o.student_id = $1 AND o.status <> 'cart'
		   AND ($2 = '' OR o.status = $2)
		   AND ($3::timestamptz IS NULL OR (o.created_at, o.id) < ($3, $4::uuid))
		 ORDER BY o.created_at DESC, o.id DESC
		 LIMIT $5`,
		affiliatorID, f.Status, nullableTime(cursorAt), cursorID, f.Limit+1,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := []AffiliatorOrderRow{}
	for rows.Next() {
		var o AffiliatorOrderRow
		if err := rows.Scan(&o.ID, &o.Status, &o.Subtotal, &o.Discount, &o.Total, &o.ProductName, &o.ProductType,
			&o.ParticipantCount, &o.SchoolName, &o.PromoCode, &o.PaymentExpiresAt, &o.CreatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > f.Limit {
		out = out[:f.Limit]
		last := out[len(out)-1]
		next = EncodeOrderCursor(last.CreatedAt, uuid.MustParse(last.ID))
	}
	return out, next, nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
