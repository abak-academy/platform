package service

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func TestAffiliatorDashboard_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newAffiliatorOrderService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()

	school := createTestSchool(t, svc)
	acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
		Email: "dash-" + uniqueSuffix() + "@test.com", Name: "Aff", Password: "password123", SchoolIDs: []string{school},
	})
	if err != nil {
		t.Fatalf("CreateAffiliator: %v", err)
	}
	newStudent := func() uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ('Siswa', $1, 'student', 'active', $2, '', 'sma') RETURNING id`,
			school, "dashstu_"+uniqueSuffix(),
		).Scan(&id); err != nil {
			t.Fatalf("insert student: %v", err)
		}
		return id
	}
	s1, s2, s3 := newStudent(), newStudent(), newStudent()
	product := seedDigitalProduct(t, repo, "exam", 50000, seedExamTarget(t, repo))

	t.Run("empty dashboard is zero with every student unpurchased", func(t *testing.T) {
		d, err := svc.AffiliatorDashboard(ctx, acc.ID, nil, nil)
		if err != nil {
			t.Fatalf("AffiliatorDashboard: %v", err)
		}
		if d.Totals.PaidOrders != 0 || d.Totals.PendingOrders != 0 || len(d.Monthly) != 0 || len(d.TopProducts) != 0 {
			t.Errorf("totals = %+v, want zeros", d.Totals)
		}
		if len(d.Schools) != 1 || d.Schools[0].StudentCount != 3 || d.UnpurchasedStudents != 3 {
			t.Errorf("schools=%+v unpurchased=%d, want 3 students all unpurchased", d.Schools, d.UnpurchasedStudents)
		}
	})

	created, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{
		Code: "DASH" + uniqueSuffix(), DiscountPercent: float64ptr(10), AffiliatorOnly: true,
	})
	if err != nil {
		t.Fatalf("AdminCreatePromoCode: %v", err)
	}
	promo := created.Code
	order, err := svc.CreateAffiliatorOrder(ctx, acc.ID, product.String(), []string{s1.String(), s2.String()}, promo)
	if err != nil {
		t.Fatalf("CreateAffiliatorOrder: %v", err)
	}

	t.Run("a cart does not count as a purchase", func(t *testing.T) {
		d, _ := svc.AffiliatorDashboard(ctx, acc.ID, nil, nil)
		if d.UnpurchasedStudents != 3 || d.Totals.PendingOrders != 0 {
			t.Errorf("unpurchased=%d pending=%d, want 3 and 0 while still a cart", d.UnpurchasedStudents, d.Totals.PendingOrders)
		}
		rows, _, err := svc.ListAffiliatorOrders(ctx, acc.ID, "", 20, "")
		if err != nil || len(rows) != 0 {
			t.Errorf("orders = %+v, %v, want none (carts are hidden)", rows, err)
		}
	})

	if _, err := svc.CheckoutAffiliatorOrder(ctx, acc.ID, order.ID.String(), "dash-"+uniqueSuffix()); err != nil {
		t.Fatalf("CheckoutAffiliatorOrder: %v", err)
	}

	t.Run("pending order covers its students but is not revenue yet", func(t *testing.T) {
		d, _ := svc.AffiliatorDashboard(ctx, acc.ID, nil, nil)
		if d.Totals.PendingOrders != 1 || d.Totals.PaidOrders != 0 || d.UnpurchasedStudents != 1 {
			t.Errorf("totals=%+v unpurchased=%d, want 1 pending, 0 paid, 1 left", d.Totals, d.UnpurchasedStudents)
		}
		mine, _, _, err := svc.ListAffiliatorStudents(ctx, acc.ID, "", false, true, "", "", 100, "", nil, "")
		if err != nil || len(mine) != 1 || mine[0].ID != s3.String() {
			t.Errorf("not purchased = %+v, %v, want only the third student", mine, err)
		}
	})

	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = 'paid', paid_at = now() WHERE id = $1`, order.ID); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	t.Run("a paid order shows in totals, trend and best sellers", func(t *testing.T) {
		d, err := svc.AffiliatorDashboard(ctx, acc.ID, nil, nil)
		if err != nil {
			t.Fatalf("AffiliatorDashboard: %v", err)
		}
		if d.Totals.PaidOrders != 1 || d.Totals.PaidUnits != 2 || d.Totals.PaidAmount != 90000 || d.Totals.DiscountTotal != 10000 || d.Totals.PendingOrders != 0 {
			t.Errorf("totals = %+v, want 1 order, 2 units, 90000 paid, 10000 discount", d.Totals)
		}
		if len(d.Monthly) != 1 || d.Monthly[0].Orders != 1 || d.Monthly[0].Units != 2 || d.Monthly[0].Amount != 90000 {
			t.Errorf("monthly = %+v, want one month with the order", d.Monthly)
		}
		if len(d.TopProducts) != 1 || d.TopProducts[0].Units != 2 || d.TopProducts[0].SchoolID != school || d.TopProducts[0].ProductID != product.String() {
			t.Errorf("top products = %+v, want 2 units of the product at the school", d.TopProducts)
		}
	})

	t.Run("a refunded order stops counting", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = 'cancelled', cancellation_reason = 'refunded', cancelled_at = now() WHERE id = $1`, order.ID); err != nil {
			t.Fatalf("refund: %v", err)
		}
		d, _ := svc.AffiliatorDashboard(ctx, acc.ID, nil, nil)
		if d.Totals.PaidOrders != 0 || d.Totals.PaidAmount != 0 || len(d.TopProducts) != 0 || d.UnpurchasedStudents != 3 {
			t.Errorf("totals=%+v top=%+v unpurchased=%d, want the refund to remove the sale and free the students", d.Totals, d.TopProducts, d.UnpurchasedStudents)
		}
	})

	t.Run("order list carries product, seats, school and promo", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = 'paid', cancellation_reason = NULL, cancelled_at = NULL WHERE id = $1`, order.ID); err != nil {
			t.Fatalf("restore: %v", err)
		}
		rows, _, err := svc.ListAffiliatorOrders(ctx, acc.ID, "paid", 20, "")
		if err != nil || len(rows) != 1 {
			t.Fatalf("orders = %+v, %v, want one paid order", rows, err)
		}
		o := rows[0]
		if o.ParticipantCount != 2 || o.ProductType != "exam" || o.Total != 90000 || o.PromoCode == nil || o.SchoolName == nil {
			t.Errorf("order row = %+v, want 2 seats, exam, 90000, promo and school set", o)
		}
		if other, _, _ := svc.ListAffiliatorOrders(ctx, acc.ID, "payment_pending", 20, ""); len(other) != 0 {
			t.Errorf("status filter returned %+v, want none", other)
		}
	})

	t.Run("admin monitoring shows the same figures", func(t *testing.T) {
		list, _, _, err := svc.ListAffiliators(ctx, "", "", 100, "")
		if err != nil {
			t.Fatalf("ListAffiliators: %v", err)
		}
		var row *AffiliatorResponse
		for i := range list {
			if list[i].ID == acc.ID {
				row = &list[i]
			}
		}
		if row == nil || row.StudentCount != 3 || row.OrderCount != 1 || row.PaidAmount != 90000 {
			t.Fatalf("list row = %+v, want 3 students, 1 order, 90000 paid", row)
		}
		detail, err := svc.GetAffiliatorDetail(ctx, acc.ID)
		if err != nil || detail.Totals.PaidOrders != 1 || len(detail.Recent) != 1 || len(detail.Affiliator.Schools) != 1 {
			t.Errorf("detail = %+v, %v, want the paid order and the school", detail, err)
		}
		if _, err := svc.GetAffiliatorDetail(ctx, uuid.NewString()); !errors.Is(err, ErrAffiliatorNotFound) {
			t.Errorf("unknown affiliator: err = %v, want ErrAffiliatorNotFound", err)
		}
	})
}
