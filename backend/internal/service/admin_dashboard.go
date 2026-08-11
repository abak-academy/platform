package service

import (
	"context"
	"time"

	"akademi-bimbel/internal/repository"
)

// jakarta is used to compute the calendar month for AdminOrderBuckets, the
// same way admin_order.go:84 does.
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return loc
}()

type KPI struct {
	Value float64  `json:"value"`
	Prev  *float64 `json:"prev,omitempty"`
}

type DashboardPeriod struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Bucket string `json:"bucket"`
}

type DashboardAttention struct {
	NeedsConfirm   int `json:"needs_confirm"`
	ReadyToShip    int `json:"ready_to_ship"`
	ShipmentFailed int `json:"shipment_failed"`
	ActiveSessions int `json:"active_sessions"`
}

type DashboardTopProduct struct {
	ProductID      string  `json:"product_id"`
	Name           string  `json:"name"`
	ProductType    string  `json:"product_type"`
	IsPhysical     bool    `json:"is_physical"`
	QtySold        int     `json:"qty_sold"`
	ProductRevenue float64 `json:"product_revenue"`
}

type DashboardResponse struct {
	Period        DashboardPeriod           `json:"period"`
	KPI           map[string]KPI            `json:"kpi"`
	Series        []repository.SeriesPoint  `json:"series"`
	Attention     DashboardAttention        `json:"attention"`
	TopProducts   []DashboardTopProduct     `json:"top_products"`
	UpcomingExams []repository.UpcomingExam `json:"upcoming_exams"`
}

const upcomingExamHorizonDays = 14

// PhysicalTypeValues exposes the physical product types as data, so the
// dashboard queries can filter on them without re-implementing the rule.
// isPhysicalType stays the single definition.
func PhysicalTypeValues() []string {
	return []string{"book", "merchandise", "medal"}
}

// previousWindow returns the equal-length window immediately before [from, to).
func previousWindow(from, to time.Time) (time.Time, time.Time) {
	d := to.Sub(from)
	return from.Add(-d), from
}

// resolveBucket honours an explicit choice and otherwise switches to weekly
// past 31 days, where daily points stop being legible.
func resolveBucket(requested string, from, to time.Time) string {
	if requested == "day" || requested == "week" {
		return requested
	}
	if to.Sub(from) > 31*24*time.Hour {
		return "week"
	}
	return "day"
}

// makeKPI omits prev when the previous window had no data — a delta against
// zero reads as +100% and means nothing.
func makeKPI(value, prev float64, prevHasData bool) KPI {
	if !prevHasData {
		return KPI{Value: value}
	}
	p := prev
	return KPI{Value: value, Prev: &p}
}

func sumSeries(points []repository.SeriesPoint, pick func(repository.SeriesPoint) float64) (float64, bool) {
	var total float64
	var any bool
	for _, p := range points {
		v := pick(p)
		total += v
		if v != 0 {
			any = true
		}
	}
	return total, any
}

func (s *Service) AdminDashboard(
	ctx context.Context, from, to time.Time, bucket string,
) (DashboardResponse, error) {
	var out DashboardResponse

	bucket = resolveBucket(bucket, from, to)
	physical := PhysicalTypeValues()

	series, err := s.storeRepo.DashboardSeries(ctx, from, to, bucket, physical)
	if err != nil {
		return out, err
	}

	prevFrom, prevTo := previousWindow(from, to)
	prevSeries, err := s.storeRepo.DashboardSeries(ctx, prevFrom, prevTo, bucket, physical)
	if err != nil {
		return out, err
	}

	revenue, _ := sumSeries(series, func(p repository.SeriesPoint) float64 { return p.Revenue })
	orders, _ := sumSeries(series, func(p repository.SeriesPoint) float64 { return float64(p.OrderCount) })
	newStud, _ := sumSeries(series, func(p repository.SeriesPoint) float64 { return float64(p.NewStudents) })

	prevRevenue, prevRevOK := sumSeries(prevSeries, func(p repository.SeriesPoint) float64 { return p.Revenue })
	prevOrders, prevOrdOK := sumSeries(prevSeries, func(p repository.SeriesPoint) float64 { return float64(p.OrderCount) })
	prevNew, prevNewOK := sumSeries(prevSeries, func(p repository.SeriesPoint) float64 { return float64(p.NewStudents) })

	studentsTotal, schools, err := s.storeRepo.CountStudentsAndSchools(ctx)
	if err != nil {
		return out, err
	}

	activeSessions, err := s.storeRepo.CountActiveExamSessions(ctx)
	if err != nil {
		return out, err
	}

	now := time.Now()
	upcoming, err := s.storeRepo.UpcomingExams(ctx, now, now.AddDate(0, 0, upcomingExamHorizonDays), 5)
	if err != nil {
		return out, err
	}

	// AdminOrderBuckets' monthStart/monthEnd only feed created_this_month and
	// completed_this_month, which this response drops. Passing the dashboard's
	// selected period would mislabel those unused counters as "this month"
	// when they're not; a real Jakarta calendar month (matching
	// admin_order.go:84) keeps them honest even though nothing here reads them.
	orderBucketsNow := time.Now().In(jakarta)
	monthStart := time.Date(orderBucketsNow.Year(), orderBucketsNow.Month(), 1, 0, 0, 0, 0, jakarta)
	monthEnd := monthStart.AddDate(0, 1, 0)

	buckets, err := s.AdminOrderBuckets(ctx, repository.OrderFilter{}, monthStart, monthEnd)
	if err != nil {
		return out, err
	}

	products, err := s.AdminTopProducts(ctx, from, to, "qty", 5)
	if err != nil {
		return out, err
	}
	tops := make([]DashboardTopProduct, 0, len(products))
	for _, p := range products {
		tops = append(tops, DashboardTopProduct{
			ProductID:      p.ProductID.String(),
			Name:           p.Name,
			ProductType:    p.ProductType,
			IsPhysical:     isPhysicalType(p.ProductType),
			QtySold:        p.QtySold,
			ProductRevenue: p.ProductRevenue,
		})
	}

	return DashboardResponse{
		Period: DashboardPeriod{
			From:   from.Format("2006-01-02"),
			To:     to.AddDate(0, 0, -1).Format("2006-01-02"),
			Bucket: bucket,
		},
		KPI: map[string]KPI{
			"revenue":        makeKPI(revenue, prevRevenue, prevRevOK),
			"order_count":    makeKPI(orders, prevOrders, prevOrdOK),
			"new_students":   makeKPI(newStud, prevNew, prevNewOK),
			"schools":        {Value: float64(schools)},
			"students_total": {Value: float64(studentsTotal)},
		},
		Series: series,
		Attention: DashboardAttention{
			NeedsConfirm:   buckets.NeedsConfirm,
			ReadyToShip:    buckets.ReadyToShip,
			ShipmentFailed: buckets.ShipmentFailed,
			ActiveSessions: activeSessions,
		},
		TopProducts:   tops,
		UpcomingExams: upcoming,
	}, nil
}
