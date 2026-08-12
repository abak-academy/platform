package repository

import (
	"context"
	"fmt"
	"time"
)

type SeriesPoint struct {
	Date            time.Time `json:"date"`
	Revenue         float64   `json:"revenue"`
	OrderCount      int       `json:"order_count"`
	RevenueDigital  float64   `json:"revenue_digital"`
	RevenuePhysical float64   `json:"revenue_physical"`
	NewStudents     int       `json:"new_students"`
	ExamStudents    int       `json:"exam_students"`
	BuyingStudents  int       `json:"buying_students"`
}

// revenueEventCTE is the money ledger every revenue report reads. It replaces
// the old `status IN ('paid','processing','shipped','completed')` filter, which
// asked "is this order in a state where money was taken" — a question about the
// order's state *now*, used to answer a question about a *period*.
//
// One row per money movement, signed: +1 when the order was paid, -1 when a
// refund was issued. Reports aggregate SUM(sign * amount) over the event
// timestamp, which gets three things right that the status filter could not:
//
//   - money lands in the period it arrived (paid_at), not the period the cart
//     happened to be minted (created_at — see placedAtExpr);
//   - a refund reduces the period it was ISSUED, instead of retroactively
//     rewriting a period that was already closed and reported;
//   - an order with no paid_at contributes nothing, so money that was never
//     taken can never be recognised.
//
// Net-zero is the property that makes it a ledger: an order paid in July and
// refunded in August is +total in July and -total in August, so all-time
// revenue is unchanged while both months tell the truth about themselves.
//
// The refund arm matches cancellation_reason explicitly rather than status
// alone. Today they are equivalent — AdminRefundOrder is the only writer of
// status='cancelled', and unpaid orders expire to payment_expired instead —
// but a future cancel path must not silently become a negative revenue event.
const revenueEventCTE = `
revenue_event AS (
    SELECT id, student_id, total, subtotal, discount, shipping_cost,
           paid_at AS at, 1 AS sign
      FROM orders
     WHERE paid_at IS NOT NULL
    UNION ALL
    SELECT id, student_id, total, subtotal, discount, shipping_cost,
           cancelled_at AS at, -1 AS sign
      FROM orders
     WHERE cancellation_reason = 'refunded'
       AND cancelled_at IS NOT NULL
)`

// jakarta is loaded once; every DashboardSeries call reuses it.
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Asia/Jakarta ships with every Go tzdata build; this would mean a
		// broken runtime environment, not a request-time condition.
		panic(fmt.Sprintf("load Asia/Jakarta location: %v", err))
	}
	return loc
}()

// DashboardSeries returns one point per bucket across [from, to), with empty
// buckets present and zero-valued.
//
// Bucketing runs in Asia/Jakarta: the timestamps are shifted into the zone
// before bucketing, because an order placed 08:00 WIB belongs to that day and
// UTC puts a 02:00 WIB order on the day before.
//
// Buckets are anchored to `from`, not to the calendar (ISO week/month). A
// week bucket starting 1 Jul covers 1–7 Jul, 8–14 Jul, and so on, regardless
// of what day of the week `from` falls on — date_trunc('week', ...) would
// instead snap to the preceding Monday and produce a short first bucket.
//
// Revenue and the digital/physical split are deliberately two separate
// aggregations. `revenue` sums orders.total once per order; the split sums item
// lines. Summing orders.total across joined item rows counts a three-item order
// three times — the fan-out bug fixed in PR #80.
func (r *Repository) DashboardSeries(
	ctx context.Context, from, to time.Time, bucket string, physicalTypes []string,
) ([]SeriesPoint, error) {
	bucketSeconds, err := bucketUnit(bucket)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
WITH params AS (
    SELECT
        ($1::timestamptz AT TIME ZONE 'Asia/Jakarta')                              AS anchor,
        (($2::timestamptz - interval '1 microsecond') AT TIME ZONE 'Asia/Jakarta') AS to_local
),
spine AS (
    SELECT generate_series(params.anchor, params.to_local, interval '%[1]d seconds') AS b
      FROM params
),%[2]s,
ord AS (
    SELECT params.anchor
             + floor(extract(epoch FROM ((e.at AT TIME ZONE 'Asia/Jakarta') - params.anchor)) / %[1]d)
               * interval '%[1]d seconds' AS b,
           SUM(e.sign * e.total) AS revenue,
           SUM(e.sign)           AS order_count,
           -- Buying students counts people, and a refund does not un-buy a
           -- person: only the +1 events feed it. Signing it would let one
           -- refund erase an unrelated buyer from the same bucket.
           COUNT(DISTINCT e.student_id) FILTER (WHERE e.sign = 1) AS buying_students
      FROM revenue_event e CROSS JOIN params
     WHERE e.at >= $1 AND e.at < $2
     GROUP BY 1
),
split AS (
    SELECT params.anchor
             + floor(extract(epoch FROM ((e.at AT TIME ZONE 'Asia/Jakarta') - params.anchor)) / %[1]d)
               * interval '%[1]d seconds' AS b,
           SUM(e.sign * COALESCE(oi.jumlah, oi.unit_price * oi.qty))
             FILTER (WHERE oi.product_type = ANY($3::text[]))     AS physical,
           SUM(e.sign * COALESCE(oi.jumlah, oi.unit_price * oi.qty))
             FILTER (WHERE NOT (oi.product_type = ANY($3::text[]))) AS digital
      FROM revenue_event e
      JOIN order_item oi ON oi.order_id = e.id
      CROSS JOIN params
     WHERE e.at >= $1 AND e.at < $2
     GROUP BY 1
),
newstud AS (
    SELECT params.anchor
             + floor(extract(epoch FROM ((u.created_at AT TIME ZONE 'Asia/Jakarta') - params.anchor)) / %[1]d)
               * interval '%[1]d seconds' AS b,
           COUNT(*) AS n
      FROM users u CROSS JOIN params
     WHERE u.role = 'student'
       AND u.created_at >= $1 AND u.created_at < $2
     GROUP BY 1
),
examstud AS (
    SELECT params.anchor
             + floor(extract(epoch FROM ((e.started_at AT TIME ZONE 'Asia/Jakarta') - params.anchor)) / %[1]d)
               * interval '%[1]d seconds' AS b,
           COUNT(DISTINCT e.student_id) AS n
      FROM exam_session e CROSS JOIN params
     WHERE e.started_at >= $1 AND e.started_at < $2
     GROUP BY 1
)
SELECT s.b,
       COALESCE(ord.revenue, 0), COALESCE(ord.order_count, 0),
       COALESCE(split.digital, 0), COALESCE(split.physical, 0),
       COALESCE(newstud.n, 0), COALESCE(examstud.n, 0),
       COALESCE(ord.buying_students, 0)
  FROM spine s
  LEFT JOIN ord      ON ord.b = s.b
  LEFT JOIN split    ON split.b = s.b
  LEFT JOIN newstud  ON newstud.b = s.b
  LEFT JOIN examstud ON examstud.b = s.b
 ORDER BY s.b
`, bucketSeconds, revenueEventCTE)

	rows, err := r.pool.Query(ctx, query, from, to, physicalTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// An empty (not nil) slice is the contract: encoding/json marshals a nil
	// slice as `null`, and the frontend's DashboardSeriesPoint[] type isn't
	// nullable — from >= to (generate_series produces no rows) would otherwise
	// serialize `series: null` and crash the page on `d.series.map`.
	out := make([]SeriesPoint, 0)
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(
			&p.Date, &p.Revenue, &p.OrderCount,
			&p.RevenueDigital, &p.RevenuePhysical,
			&p.NewStudents, &p.ExamStudents, &p.BuyingStudents,
		); err != nil {
			return nil, err
		}
		// b comes back as a timezone-naive Postgres timestamp holding Jakarta
		// wall-clock values; pgx scans it with a UTC Location, which would
		// serialize as a false "...Z" instant. Re-anchor the same wall-clock
		// numbers to Asia/Jakarta so the JSON offset is honest.
		p.Date = time.Date(
			p.Date.Year(), p.Date.Month(), p.Date.Day(),
			p.Date.Hour(), p.Date.Minute(), p.Date.Second(), p.Date.Nanosecond(),
			jakarta,
		)
		out = append(out, p)
	}
	return out, rows.Err()
}

// bucketUnit whitelists the caller-supplied bucket and returns its length in
// seconds. The result is interpolated into generated SQL as a literal, so
// bucket must never reach the query un-validated — this switch is the only
// gate.
func bucketUnit(bucket string) (int, error) {
	switch bucket {
	case "day":
		return 86400, nil
	case "week":
		return 7 * 86400, nil
	default:
		return 0, fmt.Errorf("unsupported bucket %q", bucket)
	}
}
