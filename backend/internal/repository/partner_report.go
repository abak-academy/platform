package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PartnerReportFilter narrows the admin's Mitra purchase report. PartnerID is
// optional (every Mitra when empty); From and To bound the day the order was
// made, To exclusive. Cart orders are not purchases and never appear.
type PartnerReportFilter struct {
	PartnerID string
	Status    string
	Q         string
	From      *time.Time
	To        *time.Time
}

// PartnerReportRow is one Mitra order: who bought, what, for which students,
// with which voucher, and what it came to.
type PartnerReportRow struct {
	OrderID     uuid.UUID `json:"order_id"`
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"`
	PartnerID   uuid.UUID `json:"partner_id"`
	PartnerName string    `json:"partner_name"`
	Products    string    `json:"products"`
	Seats       int       `json:"seats"`
	Students    string    `json:"students"`
	StudentList []string  `json:"student_list"`
	PromoCode   string    `json:"promo_code"`
	Subtotal    float64   `json:"subtotal"`
	Discount    float64   `json:"discount"`
	Total       float64   `json:"total"`
}

// PartnerReportSummary totals what the filters match; revenue counts only paid
// orders.
type PartnerReportSummary struct {
	Orders       int     `json:"orders"`
	PaidOrders   int     `json:"paid_orders"`
	Seats        int     `json:"seats"`
	PaidTotal    float64 `json:"paid_total"`
	PaidDiscount float64 `json:"paid_discount"`
}

const partnerReportFrom = ` FROM orders o
	JOIN users u ON u.id = o.student_id AND u.role = 'affiliator'
	LEFT JOIN promo_code pc ON pc.id = o.promo_code_id`

func partnerReportWhere(f PartnerReportFilter) (string, []any, int) {
	where := ` WHERE o.status <> 'cart' AND EXISTS (SELECT 1 FROM order_participant op0 WHERE op0.order_id = o.id)`
	var args []any
	n := 1
	if f.PartnerID != "" {
		where += fmt.Sprintf(` AND o.student_id = $%d::uuid`, n)
		args = append(args, f.PartnerID)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(` AND o.status = $%d`, n)
		args = append(args, f.Status)
		n++
	}
	if f.From != nil {
		where += fmt.Sprintf(` AND o.created_at >= $%d`, n)
		args = append(args, *f.From)
		n++
	}
	if f.To != nil {
		where += fmt.Sprintf(` AND o.created_at < $%d`, n)
		args = append(args, *f.To)
		n++
	}
	if f.Q != "" {
		where += fmt.Sprintf(` AND (u.name ILIKE $%d OR pc.code ILIKE $%d OR o.id::text ILIKE $%d
			OR EXISTS (SELECT 1 FROM order_item oi WHERE oi.order_id = o.id AND oi.name ILIKE $%d)
			OR EXISTS (SELECT 1 FROM order_participant op JOIN users s ON s.id = op.student_id
				WHERE op.order_id = o.id AND s.name ILIKE $%d))`, n, n, n, n, n)
		args = append(args, "%"+f.Q+"%")
		n++
	}
	return where, args, n
}

// ListPartnerReport returns one page of Mitra orders, newest first. A perPage of
// 0 returns every match, for the export.
func (r *Repository) ListPartnerReport(ctx context.Context, f PartnerReportFilter, page, perPage int) ([]PartnerReportRow, int, error) {
	where, args, n := partnerReportWhere(f)

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+partnerReportFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT o.id, o.created_at, o.status, u.id, u.name,
			COALESCE((SELECT string_agg(oi.name, '; ' ORDER BY oi.created_at) FROM order_item oi WHERE oi.order_id = o.id), ''),
			COALESCE((SELECT sum(oi.qty) FROM order_item oi WHERE oi.order_id = o.id), 0)::int,
			COALESCE((SELECT string_agg(s.name, '; ' ORDER BY s.name) FROM order_participant op JOIN users s ON s.id = op.student_id WHERE op.order_id = o.id), ''),
			ARRAY(SELECT s.name FROM order_participant op JOIN users s ON s.id = op.student_id WHERE op.order_id = o.id ORDER BY s.name),
			COALESCE(pc.code, ''), o.subtotal, o.discount, o.total` + partnerReportFrom + where + ` ORDER BY o.created_at DESC, o.id DESC`
	if perPage > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, n, n+1)
		args = append(args, perPage, (page-1)*perPage)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []PartnerReportRow{}
	for rows.Next() {
		var row PartnerReportRow
		if err := rows.Scan(&row.OrderID, &row.CreatedAt, &row.Status, &row.PartnerID, &row.PartnerName,
			&row.Products, &row.Seats, &row.Students, &row.StudentList, &row.PromoCode, &row.Subtotal, &row.Discount, &row.Total); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// PartnerReportTotals sums what the filters match.
func (r *Repository) PartnerReportTotals(ctx context.Context, f PartnerReportFilter) (PartnerReportSummary, error) {
	where, args, _ := partnerReportWhere(f)
	var s PartnerReportSummary
	err := r.pool.QueryRow(ctx,
		`SELECT count(*),
			count(*) FILTER (WHERE o.status IN ('paid','processing','shipped','completed')),
			COALESCE(sum((SELECT sum(oi.qty) FROM order_item oi WHERE oi.order_id = o.id)), 0)::int,
			COALESCE(sum(o.total) FILTER (WHERE o.status IN ('paid','processing','shipped','completed')), 0),
			COALESCE(sum(o.discount) FILTER (WHERE o.status IN ('paid','processing','shipped','completed')), 0)`+
			partnerReportFrom+where, args...,
	).Scan(&s.Orders, &s.PaidOrders, &s.Seats, &s.PaidTotal, &s.PaidDiscount)
	return s, err
}
