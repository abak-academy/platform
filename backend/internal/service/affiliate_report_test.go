package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

func TestAffiliateReports_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newAffiliatorOrderService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()
	tag := "Rpt" + uniqueSuffix()

	newSchool := func(label string) string {
		t.Helper()
		code := strings.ToLower(tag) + label
		sch, err := svc.CreateSchool(ctx, "School "+tag+" "+label, code, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return sch.ID
	}
	s1, s2 := newSchool("S1"), newSchool("S2")
	newAff := func(label string, schools ...string) string {
		t.Helper()
		acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
			Email: strings.ToLower(tag) + "-" + strings.ToLower(label) + "@test.com", Name: tag + " " + label, Password: "password123", SchoolIDs: schools,
		})
		if err != nil {
			t.Fatalf("CreateAffiliator: %v", err)
		}
		return acc.ID
	}
	a1, a2, a3 := newAff("A1", s1, s2), newAff("A2", s1), newAff("A3", s2)

	newStudent := func(school string) string {
		t.Helper()
		var id uuid.UUID
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang) VALUES ('Siswa', $1, 'student', 'active', $2, '', 'sma') RETURNING id`,
			school, "rptstu_"+uniqueSuffix()).Scan(&id); err != nil {
			t.Fatalf("insert student: %v", err)
		}
		return id.String()
	}
	s1a, s1b, s1c, s1d := newStudent(s1), newStudent(s1), newStudent(s1), newStudent(s1)
	s2a, _ := newStudent(s2), newStudent(s2)

	p1 := seedDigitalProduct(t, repo, "exam", 100000, seedExamTarget(t, repo)).String()
	var courseTarget uuid.UUID
	if err := repo.Pool().QueryRow(ctx, `INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'Guru') RETURNING id`, "RptCourse "+tag).Scan(&courseTarget); err != nil {
		t.Fatalf("create course: %v", err)
	}
	p2 := seedDigitalProduct(t, repo, "course", 75000, courseTarget).String()

	promo, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "RPT" + strings.ToUpper(tag), DiscountPercent: float64ptr(10), AffiliatorOnly: true})
	if err != nil {
		t.Fatalf("AdminCreatePromoCode: %v", err)
	}
	if _, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "RPTPLAIN" + strings.ToUpper(tag), DiscountPercent: float64ptr(5)}); err != nil {
		t.Fatalf("AdminCreatePromoCode plain: %v", err)
	}

	place := func(aff, school, product string, students []string, promoCode string) uuid.UUID {
		t.Helper()
		o, err := svc.CreateAffiliatorOrder(ctx, aff, school, product, ParticipantSelector{StudentIDs: students}, promoCode)
		if err != nil {
			t.Fatalf("CreateAffiliatorOrder: %v", err)
		}
		if _, err := svc.CheckoutAffiliatorOrder(ctx, aff, o.ID.String(), "rpt-"+uniqueSuffix()); err != nil {
			t.Fatalf("CheckoutAffiliatorOrder: %v", err)
		}
		return o.ID
	}
	setOrder := func(id uuid.UUID, sql string, args ...any) {
		t.Helper()
		if _, err := repo.Pool().Exec(ctx, "UPDATE orders SET "+sql+" WHERE id = $1", append([]any{id}, args...)...); err != nil {
			t.Fatalf("update order: %v", err)
		}
	}

	o1 := place(a1, s1, p1, []string{s1a, s1b}, promo.Code)
	setOrder(o1, `status = 'paid', paid_at = '2019-03-10 12:00+07', created_at = '2019-03-09 12:00+07'`)
	o2 := place(a1, s2, p2, []string{s2a}, "")
	setOrder(o2, `created_at = '2019-03-12 12:00+07'`)
	o3 := place(a2, s1, p1, []string{s1c}, "")
	setOrder(o3, `status = 'paid', paid_at = '2019-04-05 12:00+07', created_at = '2019-04-04 12:00+07'`)
	o4 := place(a2, s1, p2, []string{s1d}, "")
	setOrder(o4, `status = 'payment_expired', created_at = '2019-03-20 12:00+07'`)
	o5 := place(a1, s1, p2, []string{s1a}, "")
	setOrder(o5, `status = 'cancelled', cancellation_reason = 'refunded', paid_at = '2019-03-15 12:00+07', created_at = '2019-03-14 12:00+07'`)

	wib := time.FixedZone("WIB", 7*3600)
	march := repository.AffiliateReportQuery{
		From: ptrTime(time.Date(2019, 3, 1, 0, 0, 0, 0, wib)),
		To:   ptrTime(time.Date(2019, 4, 1, 0, 0, 0, 0, wib)),
		Q:    tag,
	}

	t.Run("per affiliator: sales, backlog, expired and the inactive filter", func(t *testing.T) {
		rows, total, err := svc.AffiliatorReport(ctx, march)
		if err != nil {
			t.Fatalf("AffiliatorReport: %v", err)
		}
		if total != 3 || len(rows) != 3 {
			t.Fatalf("rows=%d total=%d, want the 3 affiliators", len(rows), total)
		}
		by := map[string]repository.AffiliatorReportRow{}
		for _, r := range rows {
			by[r.AffiliatorID] = r
		}
		r1, r2, r3 := by[a1], by[a2], by[a3]
		if r1.PaidOrders != 1 || r1.PaidUnits != 2 || r1.PaidAmount != 180000 || r1.DiscountTotal != 20000 {
			t.Errorf("A1 sales = %+v, want 1 order, 2 units, 180000 paid, 20000 discount (the refund and April are excluded)", r1)
		}
		if r1.PendingOrders != 1 || r1.PendingAmount != 75000 || r1.OrdersInRange != 3 || r1.SchoolCount != 2 || r1.StudentCount != 6 || r1.LastOrderAt == nil {
			t.Errorf("A1 backlog = %+v, want 1 pending of 75000, 3 orders in March, 2 schools, 6 students", r1)
		}
		if r2.PaidOrders != 0 || r2.ExpiredOrders != 1 || r2.OrdersInRange != 1 || r2.StudentCount != 4 {
			t.Errorf("A2 = %+v, want nothing paid in March (its sale is April), 1 expired, 1 order in March", r2)
		}
		if r3.OrdersInRange != 0 || r3.LastOrderAt != nil {
			t.Errorf("A3 = %+v, want no activity", r3)
		}
		if rows[0].AffiliatorID != a1 {
			t.Errorf("first row = %s, want the best seller A1 (%s)", rows[0].AffiliatorID, a1)
		}

		inactive := march
		inactive.InactiveOnly = true
		only, total, _ := svc.AffiliatorReport(ctx, inactive)
		if total != 1 || len(only) != 1 || only[0].AffiliatorID != a3 {
			t.Errorf("inactive = %+v (total %d), want only A3", only, total)
		}

		april := repository.AffiliateReportQuery{From: ptrTime(time.Date(2019, 4, 1, 0, 0, 0, 0, wib)), To: ptrTime(time.Date(2019, 5, 1, 0, 0, 0, 0, wib)), Q: tag}
		apr, _, _ := svc.AffiliatorReport(ctx, april)
		var a2Apr repository.AffiliatorReportRow
		for _, r := range apr {
			if r.AffiliatorID == a2 {
				a2Apr = r
			}
		}
		if a2Apr.PaidOrders != 1 || a2Apr.PaidAmount != 100000 {
			t.Errorf("A2 in April = %+v, want its 100000 sale there", a2Apr)
		}
	})

	t.Run("paging returns the next row and the same total", func(t *testing.T) {
		q := march
		q.Limit, q.Offset = 1, 1
		rows, total, err := svc.AffiliatorReport(ctx, q)
		if err != nil || len(rows) != 1 || total != 3 {
			t.Errorf("page = %+v total=%d err=%v, want the second of 3", rows, total, err)
		}
	})

	t.Run("per school: coverage ignores the period, sales follow it", func(t *testing.T) {
		rows, total, err := svc.AffiliateSchoolReport(ctx, march)
		if err != nil || total != 2 || len(rows) != 2 {
			t.Fatalf("rows=%+v total=%d err=%v, want both schools", rows, total, err)
		}
		by := map[string]repository.SchoolReportRow{}
		for _, r := range rows {
			by[r.SchoolID] = r
		}
		r1, r2 := by[s1], by[s2]
		if r1.Affiliators != 2 || r1.Students != 4 || r1.CoveredStudent != 3 || r1.Unpurchased != 1 {
			t.Errorf("S1 coverage = %+v, want 2 affiliators, 4 students, 3 covered (a, b, c), 1 not", r1)
		}
		if r1.PaidOrders != 1 || r1.PaidUnits != 2 || r1.PaidAmount != 180000 || r1.DiscountTotal != 20000 {
			t.Errorf("S1 sales = %+v, want the March order only", r1)
		}
		if r2.Affiliators != 2 || r2.Students != 2 || r2.CoveredStudent != 1 || r2.Unpurchased != 1 || r2.PaidOrders != 0 {
			t.Errorf("S2 = %+v, want 1 of 2 covered by the pending order and no sale", r2)
		}
	})

	t.Run("best sellers per school", func(t *testing.T) {
		rows, total, err := svc.AffiliateProductReport(ctx, march)
		if err != nil {
			t.Fatalf("AffiliateProductReport: %v", err)
		}
		if total != 1 || len(rows) != 1 || rows[0].SchoolID != s1 || rows[0].ProductID != p1 || rows[0].Units != 2 || rows[0].Orders != 1 {
			t.Errorf("rows = %+v, want 2 units of the exam at S1 from one order", rows)
		}
	})

	t.Run("promo usage lists affiliator promos and promos used on affiliator orders only", func(t *testing.T) {
		q := march
		q.Q = strings.ToUpper(tag)
		rows, total, err := svc.AffiliatePromoReport(ctx, q)
		if err != nil {
			t.Fatalf("AffiliatePromoReport: %v", err)
		}
		if total != 1 || len(rows) != 1 || rows[0].Code != promo.Code {
			t.Fatalf("rows = %+v, want only the affiliator promo (the unused plain one is left out)", rows)
		}
		if r := rows[0]; !r.AffiliatorOnly || !r.ActiveNow || r.Orders != 1 || r.DiscountTotal != 20000 || r.UsedCount != 1 {
			t.Errorf("promo row = %+v, want active, 1 order, 20000 discount, used once", r)
		}
	})

	t.Run("trend sums the month across affiliators", func(t *testing.T) {
		rows, err := svc.AffiliateTrendReport(ctx, march)
		if err != nil {
			t.Fatalf("AffiliateTrendReport: %v", err)
		}
		if len(rows) != 1 || rows[0].Month != "2019-03" || rows[0].Orders != 1 || rows[0].Units != 2 || rows[0].Amount != 180000 || rows[0].Discount != 20000 || rows[0].Affiliators != 1 {
			t.Errorf("trend = %+v, want one month 2019-03 with the one paid order", rows)
		}
	})

	t.Run("export renders CSV with a BOM, one row per record and neutralised cells", func(t *testing.T) {
		data, err := svc.ExportAffiliateReport(ctx, "affiliators", march)
		if err != nil {
			t.Fatalf("ExportAffiliateReport: %v", err)
		}
		if !strings.HasPrefix(string(data), "\xEF\xBB\xBF") {
			t.Error("export lacks the UTF-8 BOM")
		}
		lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(data), "\xEF\xBB\xBF")), "\n")
		if len(lines) != 4 || !strings.HasPrefix(lines[0], "nama,email,status") || !strings.Contains(lines[1], tag+" A1") {
			t.Errorf("export = %q, want a header and 3 rows with A1 first", lines)
		}
		for _, name := range AffiliateReportNames {
			if _, err := svc.ExportAffiliateReport(ctx, name, march); err != nil {
				t.Errorf("export %s: %v", name, err)
			}
		}
		if _, err := svc.ExportAffiliateReport(ctx, "nope", march); !errors.Is(err, ErrUnknownAffiliateReport) {
			t.Errorf("unknown report: err = %v, want ErrUnknownAffiliateReport", err)
		}
	})
}

func ptrTime(t time.Time) *time.Time { return &t }
