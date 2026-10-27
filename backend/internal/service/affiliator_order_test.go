package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newAffiliatorOrderService(t *testing.T, repo *repository.Repository) *Service {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{JWTSecret: "affiliator-order-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 168 * time.Hour}
	return NewWithStore(repo, repo, rdb, infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL),
		&NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, &NoopLogisticsClient{}, nil, cfg, nil)
}

func TestAffiliatorOrder_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newAffiliatorOrderService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()

	school := createTestSchool(t, svc)
	otherSchool := createTestSchool(t, svc)
	acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
		Email: "ord-" + uniqueSuffix() + "@test.com", Name: "Aff", Password: "password123", SchoolIDs: []string{school},
	})
	if err != nil {
		t.Fatalf("CreateAffiliator: %v", err)
	}
	newStudent := func(schoolID string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ('Siswa', $1, 'student', 'active', $2, '', 'sma') RETURNING id`,
			schoolID, "ordstu_"+uniqueSuffix(),
		).Scan(&id); err != nil {
			t.Fatalf("insert student: %v", err)
		}
		return id
	}
	ids := func(in ...uuid.UUID) []string {
		out := make([]string, len(in))
		for i, id := range in {
			out[i] = id.String()
		}
		return out
	}
	newPromo := func(t *testing.T, mod func(*model.PromoCode)) model.PromoCode {
		t.Helper()
		p := model.PromoCode{Code: "AFF" + uniqueSuffix(), DiscountPercent: float64ptr(10), AffiliatorOnly: true, MinQty: intptr(2)}
		if mod != nil {
			mod(&p)
		}
		created, err := svc.AdminCreatePromoCode(ctx, p)
		if err != nil {
			t.Fatalf("AdminCreatePromoCode: %v", err)
		}
		return created
	}

	examTarget := seedExamTarget(t, repo)
	examProduct := seedDigitalProduct(t, repo, "exam", 50000, examTarget)
	var courseTarget uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'Guru') RETURNING id`,
		"Course "+uuid.NewString(),
	).Scan(&courseTarget); err != nil {
		t.Fatalf("create course: %v", err)
	}
	courseProduct := seedDigitalProduct(t, repo, "course", 30000, courseTarget)

	t.Run("product list holds exam and course products only", func(t *testing.T) {
		list, err := svc.ListAffiliatorProducts(ctx)
		if err != nil {
			t.Fatalf("ListAffiliatorProducts: %v", err)
		}
		found := map[string]bool{}
		for _, p := range list {
			if p.Type != "exam" && p.Type != "course" {
				t.Errorf("unexpected product type %q", p.Type)
			}
			found[p.ID] = true
		}
		if !found[examProduct.String()] || !found[courseProduct.String()] {
			t.Errorf("list is missing the seeded exam or course product")
		}
	})

	t.Run("preview excludes owners and applies the promo to the net new count", func(t *testing.T) {
		a, b, c := newStudent(school), newStudent(school), newStudent(school)
		registerForExam(t, repo, c, examTarget, "registered")
		promo := newPromo(t, nil)

		p, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(a, b, c)}, promo.Code)
		if err != nil {
			t.Fatalf("PreviewAffiliatorOrder: %v", err)
		}
		if p.NetNewCount != 2 || len(p.Excluded) != 1 || p.Excluded[0].StudentID != c.String() || p.Excluded[0].Reason != "already_owned" {
			t.Errorf("preview = %+v, want 2 net new and the registered student excluded", p)
		}
		if p.Subtotal != 100000 || p.Discount != 10000 || p.Total != 90000 {
			t.Errorf("amounts = %v/%v/%v, want 100000/10000/90000", p.Subtotal, p.Discount, p.Total)
		}

		_, err = svc.PreviewAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(a)}, promo.Code)
		if !errors.Is(err, ErrPromoMinQty) {
			t.Errorf("one student under min_qty 2: err = %v, want ErrPromoMinQty", err)
		}
	})

	t.Run("course ownership is an active enrollment", func(t *testing.T) {
		a, b := newStudent(school), newStudent(school)
		enrollInCourse(t, repo, a, courseTarget)
		p, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, school, courseProduct.String(), ParticipantSelector{StudentIDs: ids(a, b)}, "")
		if err != nil {
			t.Fatalf("PreviewAffiliatorOrder: %v", err)
		}
		if p.NetNewCount != 1 || len(p.Excluded) != 1 || p.Excluded[0].StudentID != a.String() {
			t.Errorf("preview = %+v, want the enrolled student excluded", p)
		}
	})

	t.Run("refuses a foreign school, foreign students, other product types and a student promo role", func(t *testing.T) {
		own, foreign := newStudent(school), newStudent(otherSchool)
		if _, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, otherSchool, examProduct.String(), ParticipantSelector{StudentIDs: ids(foreign)}, ""); !errors.Is(err, ErrAffiliatorSchoolNotAllowed) {
			t.Errorf("foreign school: err = %v, want ErrAffiliatorSchoolNotAllowed", err)
		}
		if _, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(own, foreign)}, ""); !errors.Is(err, ErrCrossSchoolStudent) {
			t.Errorf("foreign student in own school: err = %v, want ErrCrossSchoolStudent", err)
		}
		merch, err := svc.CreateProduct(ctx, model.Product{Type: "merchandise", Name: "Kaos " + uniqueSuffix(), Price: 1000, Stock: 5, Status: "published"}, RoleSuperAdmin)
		if err != nil {
			t.Fatalf("CreateProduct: %v", err)
		}
		if _, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, school, merch.ID, ParticipantSelector{StudentIDs: ids(own)}, ""); !errors.Is(err, ErrAffiliatorProductType) {
			t.Errorf("physical product: err = %v, want ErrAffiliatorProductType", err)
		}
		if _, err := svc.PreviewAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(own)}, "NOPE"+uniqueSuffix()); !errors.Is(err, ErrInvalidPromo) {
			t.Errorf("unknown promo: err = %v, want ErrInvalidPromo", err)
		}
	})

	t.Run("create stores buyer, participants and promo; a new order supersedes the old one", func(t *testing.T) {
		a, b := newStudent(school), newStudent(school)
		promo := newPromo(t, nil)
		first, err := svc.CreateAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(a, b)}, promo.Code)
		if err != nil {
			t.Fatalf("CreateAffiliatorOrder: %v", err)
		}
		if first.StudentID.String() != acc.ID || first.Status != "cart" || first.Subtotal != 100000 || first.Discount != 10000 || first.Total != 90000 {
			t.Errorf("order = %+v, want affiliator-owned cart 100000 - 10000 = 90000", first)
		}
		if first.PromoCodeID == nil || *first.PromoCodeID != promo.ID {
			t.Errorf("promo_code_id = %v, want %s", first.PromoCodeID, promo.ID)
		}
		if len(first.Items) != 1 || first.Items[0].Qty != 2 {
			t.Errorf("items = %+v, want one line of qty 2", first.Items)
		}
		parts, _ := repo.GetOrderParticipants(ctx, first.ID)
		if len(parts) != 2 {
			t.Errorf("participants = %v, want the two students", parts)
		}

		second, err := svc.CreateAffiliatorOrder(ctx, acc.ID, school, courseProduct.String(), ParticipantSelector{StudentIDs: ids(a)}, "")
		if err != nil {
			t.Fatalf("second CreateAffiliatorOrder: %v", err)
		}
		old, _ := repo.GetOrderByID(ctx, first.ID)
		if old.Status != "cancelled" || old.CancellationReason != "superseded" || second.Status != "cart" {
			t.Errorf("old=%q/%q new=%q, want old cancelled as superseded and new a cart", old.Status, old.CancellationReason, second.Status)
		}
	})

	t.Run("create refuses when nobody is left to buy for", func(t *testing.T) {
		a := newStudent(school)
		registerForExam(t, repo, a, examTarget, "registered")
		if _, err := svc.CreateAffiliatorOrder(ctx, acc.ID, school, examProduct.String(), ParticipantSelector{StudentIDs: ids(a)}, ""); !errors.Is(err, ErrZeroNetNewParticipants) {
			t.Errorf("err = %v, want ErrZeroNetNewParticipants", err)
		}
	})

	t.Run("checkout counts the promo once and refuses an order whose promo no longer holds", func(t *testing.T) {
		a, b := newStudent(school), newStudent(school)
		promo := newPromo(t, nil)
		order, err := svc.CreateAffiliatorOrder(ctx, acc.ID, school, courseProduct.String(), ParticipantSelector{StudentIDs: ids(a, b)}, promo.Code)
		if err != nil {
			t.Fatalf("CreateAffiliatorOrder: %v", err)
		}
		if _, err := svc.CheckoutAffiliatorOrder(ctx, uuid.NewString(), order.ID.String(), "k-"+uniqueSuffix()); !errors.Is(err, ErrOrderNotFound) {
			t.Errorf("other buyer: err = %v, want ErrOrderNotFound", err)
		}

		if _, err := repo.Pool().Exec(ctx, `UPDATE promo_code SET expires_at = now() - interval '1 hour' WHERE id = $1`, promo.ID); err != nil {
			t.Fatalf("expire promo: %v", err)
		}
		if _, err := svc.CheckoutAffiliatorOrder(ctx, acc.ID, order.ID.String(), "k-"+uniqueSuffix()); !errors.Is(err, ErrInvalidPromo) {
			t.Errorf("expired promo: err = %v, want ErrInvalidPromo", err)
		}
		if still, _ := repo.GetOrderByID(ctx, order.ID); still.Status != "cart" {
			t.Errorf("status after refused checkout = %q, want cart", still.Status)
		}

		if _, err := repo.Pool().Exec(ctx, `UPDATE promo_code SET expires_at = NULL WHERE id = $1`, promo.ID); err != nil {
			t.Fatalf("restore promo: %v", err)
		}
		key := "k-" + uniqueSuffix()
		res, err := svc.CheckoutAffiliatorOrder(ctx, acc.ID, order.ID.String(), key)
		if err != nil || res.GatewayRef == "" {
			t.Fatalf("CheckoutAffiliatorOrder: %+v, %v", res, err)
		}
		if _, err := svc.CheckoutAffiliatorOrder(ctx, acc.ID, order.ID.String(), key); err != nil {
			t.Errorf("replay with the same key: %v, want the cached result", err)
		}
		got, _ := repo.GetPromoByID(ctx, promo.ID)
		if got.UsedCount != 1 {
			t.Errorf("used_count = %d, want 1", got.UsedCount)
		}
	})
}
