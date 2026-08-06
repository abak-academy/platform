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

// paidStatuses are the order states that count as money earned. Same list the
// revenue report uses (GetRevenue, order.go); kept in sync deliberately.
const paidStatusList = `('paid', 'processing', 'shipped', 'completed')`

// DashboardSeries returns one point per bucket across [from, to), with empty
// buckets present and zero-valued.
//
// Bucketing runs in Asia/Jakarta: the timestamps are shifted into the zone
// before date_trunc, because an order placed 08:00 WIB belongs to that day and
// UTC puts a 02:00 WIB order on the day before.
//
// Revenue and the digital/physical split are deliberately two separate
// aggregations. `revenue` sums orders.total once per order; the split sums item
// lines. Summing orders.total across joined item rows counts a three-item order
// three times — the fan-out bug fixed in PR #80.
func (r *Repository) DashboardSeries(
	ctx context.Context, from, to time.Time, bucket string, physicalTypes []string,
) ([]SeriesPoint, error) {
	unit, err := bucketUnit(bucket)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
WITH spine AS (
    SELECT generate_series(
        date_trunc('%[1]s', $1::timestamptz AT TIME ZONE 'Asia/Jakarta'),
        date_trunc('%[1]s', ($2::timestamptz - interval '1 microsecond') AT TIME ZONE 'Asia/Jakarta'),
        interval '1 %[1]s'
    ) AS b
),
ord AS (
    SELECT date_trunc('%[1]s', created_at AT TIME ZONE 'Asia/Jakarta') AS b,
           SUM(total)                  AS revenue,
           COUNT(*)                    AS order_count,
           COUNT(DISTINCT student_id)  AS buying_students
      FROM orders
     WHERE status IN %[2]s
       AND created_at >= $1 AND created_at < $2
     GROUP BY 1
),
split AS (
    SELECT date_trunc('%[1]s', o.created_at AT TIME ZONE 'Asia/Jakarta') AS b,
           SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty))
             FILTER (WHERE oi.product_type = ANY($3::text[]))     AS physical,
           SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty))
             FILTER (WHERE NOT (oi.product_type = ANY($3::text[]))) AS digital
      FROM orders o
      JOIN order_item oi ON oi.order_id = o.id
     WHERE o.status IN %[2]s
       AND o.created_at >= $1 AND o.created_at < $2
     GROUP BY 1
),
newstud AS (
    SELECT date_trunc('%[1]s', created_at AT TIME ZONE 'Asia/Jakarta') AS b,
           COUNT(*) AS n
      FROM users
     WHERE role = 'student'
       AND created_at >= $1 AND created_at < $2
     GROUP BY 1
),
examstud AS (
    SELECT date_trunc('%[1]s', started_at AT TIME ZONE 'Asia/Jakarta') AS b,
           COUNT(DISTINCT student_id) AS n
      FROM exam_session
     WHERE started_at >= $1 AND started_at < $2
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
`, unit, paidStatusList)

	rows, err := r.pool.Query(ctx, query, from, to, physicalTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(
			&p.Date, &p.Revenue, &p.OrderCount,
			&p.RevenueDigital, &p.RevenuePhysical,
			&p.NewStudents, &p.ExamStudents, &p.BuyingStudents,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// bucketUnit whitelists the interpolated unit. It is concatenated into SQL, so
// it must never come straight from a caller-supplied bucket parameter.
func bucketUnit(bucket string) (string, error) {
	switch bucket {
	case "day":
		return "day", nil
	case "week":
		return "week", nil
	default:
		return "", fmt.Errorf("unsupported bucket %q", bucket)
	}
}
