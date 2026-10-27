package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func TestEvaluatePromo(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	past, future := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	pA, pB := uuid.NewString(), uuid.NewString()
	lines := []PromoLine{{ProductID: pA, Qty: 2, Amount: 200}, {ProductID: pB, Qty: 3, Amount: 300}}

	base := func(mod func(*model.PromoCode)) model.PromoCode {
		p := model.PromoCode{Code: "P", DiscountPercent: float64ptr(10)}
		if mod != nil {
			mod(&p)
		}
		return p
	}

	tests := []struct {
		name         string
		promo        model.PromoCode
		role         string
		lines        []PromoLine
		wantErr      error
		wantDiscount float64
	}{
		{"plain percent over the whole order", base(nil), RoleStudent, lines, nil, 50},
		{"nominal discount is capped at the subtotal", base(func(p *model.PromoCode) { p.DiscountPercent = nil; p.DiscountAmount = float64ptr(900) }), RoleStudent, lines, nil, 500},
		{"percent respects the cap", base(func(p *model.PromoCode) { p.MaxDiscountAmount = float64ptr(20) }), RoleStudent, lines, nil, 20},
		{"affiliator only is refused for a student", base(func(p *model.PromoCode) { p.AffiliatorOnly = true }), RoleStudent, lines, ErrInvalidPromo, 0},
		{"affiliator only is refused for an anonymous caller", base(func(p *model.PromoCode) { p.AffiliatorOnly = true }), "", lines, ErrInvalidPromo, 0},
		{"affiliator only is refused for admin_school", base(func(p *model.PromoCode) { p.AffiliatorOnly = true }), RoleAdminSchool, lines, ErrInvalidPromo, 0},
		{"affiliator only works for an affiliator", base(func(p *model.PromoCode) { p.AffiliatorOnly = true }), RoleAffiliator, lines, nil, 50},
		{"not started yet", base(func(p *model.PromoCode) { p.StartsAt = &future }), RoleStudent, lines, ErrPromoNotStarted, 0},
		{"started", base(func(p *model.PromoCode) { p.StartsAt = &past }), RoleStudent, lines, nil, 50},
		{"expired", base(func(p *model.PromoCode) { p.ExpiresAt = &past }), RoleStudent, lines, ErrInvalidPromo, 0},
		{"exhausted", base(func(p *model.PromoCode) { p.MaxUses = intptr(1); p.UsedCount = 1 }), RoleStudent, lines, ErrInvalidPromo, 0},
		{"min qty met", base(func(p *model.PromoCode) { p.MinQty = intptr(5) }), RoleStudent, lines, nil, 50},
		{"min qty missed", base(func(p *model.PromoCode) { p.MinQty = intptr(6) }), RoleStudent, lines, ErrPromoMinQty, 0},
		{"min amount missed", base(func(p *model.PromoCode) { p.MinOrderAmount = float64ptr(501) }), RoleStudent, lines, ErrPromoMinOrder, 0},
		{"product scope discounts only matching lines", base(func(p *model.PromoCode) { p.ProductIDs = []string{pA} }), RoleStudent, lines, nil, 20},
		{"product scope counts only matching qty", base(func(p *model.PromoCode) { p.ProductIDs = []string{pA}; p.MinQty = intptr(3) }), RoleStudent, lines, ErrPromoMinQty, 0},
		{"product scope counts only matching amount", base(func(p *model.PromoCode) { p.ProductIDs = []string{pA}; p.MinOrderAmount = float64ptr(250) }), RoleStudent, lines, ErrPromoMinOrder, 0},
		{"product scope with no matching line is invalid", base(func(p *model.PromoCode) { p.ProductIDs = []string{uuid.NewString()} }), RoleStudent, lines, ErrInvalidPromo, 0},
		{"unknown lines cannot satisfy a product scope", base(func(p *model.PromoCode) { p.ProductIDs = []string{pA} }), RoleStudent, []PromoLine{{Amount: 500, Unscoped: true}}, ErrInvalidPromo, 0},
		{"unknown lines cannot satisfy a min qty", base(func(p *model.PromoCode) { p.MinQty = intptr(1) }), RoleStudent, []PromoLine{{Amount: 500, Unscoped: true}}, ErrInvalidPromo, 0},
		{"unknown lines still work for a plain promo", base(nil), RoleStudent, []PromoLine{{Amount: 500, Unscoped: true}}, nil, 50},
		{"missing code is invalid", model.PromoCode{}, RoleStudent, lines, ErrInvalidPromo, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluatePromo(tt.promo, tt.role, tt.lines, 0, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("evaluatePromo() err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Discount != tt.wantDiscount {
				t.Errorf("discount = %v, want %v", got.Discount, tt.wantDiscount)
			}
		})
	}
}

func TestEvaluatePromo_totalKeepsWholeOrderAndShipping(t *testing.T) {
	pA, pB := uuid.NewString(), uuid.NewString()
	promo := model.PromoCode{Code: "P", DiscountPercent: float64ptr(10), ProductIDs: []string{pA}}
	got, err := evaluatePromo(promo, RoleStudent, []PromoLine{{ProductID: pA, Qty: 1, Amount: 200}, {ProductID: pB, Qty: 1, Amount: 300}}, 50, time.Now())
	if err != nil {
		t.Fatalf("evaluatePromo: %v", err)
	}
	if got.Discount != 20 || got.Total != 530 {
		t.Errorf("discount=%v total=%v, want 20 and 530 (500 - 20 + 50)", got.Discount, got.Total)
	}
}

func TestPromoAdmin_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	create := func(t *testing.T, mod func(*model.PromoCode)) (model.PromoCode, error) {
		t.Helper()
		p := model.PromoCode{Code: "PROMO" + uniqueSuffix(), DiscountPercent: float64ptr(10)}
		if mod != nil {
			mod(&p)
		}
		return svc.AdminCreatePromoCode(ctx, p)
	}

	t.Run("create rejects invalid definitions", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		past := time.Now().Add(-time.Hour)
		cases := map[string]func(*model.PromoCode){
			"both discounts":         func(p *model.PromoCode) { p.DiscountAmount = float64ptr(5) },
			"no discount":            func(p *model.PromoCode) { p.DiscountPercent = nil },
			"percent over 100":       func(p *model.PromoCode) { p.DiscountPercent = float64ptr(101) },
			"negative min qty":       func(p *model.PromoCode) { p.MinQty = intptr(-1) },
			"start after expiry":     func(p *model.PromoCode) { p.StartsAt = &future; p.ExpiresAt = &past },
			"affiliator only public": func(p *model.PromoCode) { p.AffiliatorOnly = true; p.IsPublic = true },
			"unknown product":        func(p *model.PromoCode) { p.ProductIDs = []string{uuid.NewString()} },
		}
		for name, mod := range cases {
			if _, err := create(t, mod); !errors.Is(err, ErrValidation) {
				t.Errorf("%s: err = %v, want ErrValidation", name, err)
			}
		}
		if _, err := create(t, func(p *model.PromoCode) { p.ProductIDs = []string{"nope"} }); !errors.Is(err, ErrInvalidUUID) {
			t.Errorf("bad product id: err = %v, want ErrInvalidUUID", err)
		}
	})

	t.Run("new fields round trip through create, get and update", func(t *testing.T) {
		start := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
		created, err := create(t, func(p *model.PromoCode) {
			p.AffiliatorOnly = true
			p.StartsAt = &start
			p.MinQty = intptr(3)
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if !created.AffiliatorOnly || created.MinQty == nil || *created.MinQty != 3 || created.StartsAt == nil || len(created.ProductIDs) != 0 {
			t.Fatalf("created = %+v, want the new fields stored and no product scope", created)
		}

		upd := created
		upd.MinQty = intptr(10)
		upd.DiscountPercent = nil
		upd.DiscountAmount = float64ptr(5000)
		if err := svc.AdminUpdatePromoCode(ctx, created.ID.String(), upd); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := repo.GetPromoByCode(ctx, created.Code)
		if err != nil {
			t.Fatalf("GetPromoByCode: %v", err)
		}
		if got.MinQty == nil || *got.MinQty != 10 || got.DiscountAmount == nil || *got.DiscountAmount != 5000 || got.DiscountPercent != nil {
			t.Errorf("after update = %+v, want min_qty 10 and nominal 5000", got)
		}
		if got.Code != created.Code || got.UsedCount != created.UsedCount {
			t.Errorf("update changed code or used_count: %+v", got)
		}
	})

	t.Run("product scope round trips for existing products", func(t *testing.T) {
		prod, err := svc.CreateProduct(ctx, model.Product{Type: "merchandise", Name: "Scope " + uniqueSuffix(), Price: 1000, Stock: 5, Status: "published"}, RoleSuperAdmin)
		if err != nil {
			t.Fatalf("CreateProduct: %v", err)
		}
		id := prod.ID
		created, err := create(t, func(p *model.PromoCode) { p.ProductIDs = []string{id, " " + id + " "} })
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetPromoByCode(ctx, created.Code)
		if err != nil {
			t.Fatalf("GetPromoByCode: %v", err)
		}
		if len(got.ProductIDs) != 1 || got.ProductIDs[0] != id {
			t.Errorf("product_ids = %v, want [%s]", got.ProductIDs, id)
		}
		line := []PromoLine{{ProductID: id, Qty: 1, Amount: 1000}, {ProductID: uuid.NewString(), Qty: 1, Amount: 9000}}
		v, err := svc.ValidatePromoLines(ctx, created.Code, RoleStudent, line, 0)
		if err != nil || v.Discount != 100 {
			t.Errorf("validate = %+v, %v, want discount 100 on the scoped product only", v, err)
		}
	})

	t.Run("affiliator promos stay out of the public list and show in the affiliator list", func(t *testing.T) {
		aff, err := create(t, func(p *model.PromoCode) { p.AffiliatorOnly = true })
		if err != nil {
			t.Fatalf("create affiliator promo: %v", err)
		}
		pub, err := create(t, func(p *model.PromoCode) { p.IsPublic = true })
		if err != nil {
			t.Fatalf("create public promo: %v", err)
		}
		future := time.Now().Add(time.Hour)
		notYet, err := create(t, func(p *model.PromoCode) { p.AffiliatorOnly = true; p.StartsAt = &future })
		if err != nil {
			t.Fatalf("create future promo: %v", err)
		}

		has := func(list []model.PromoCode, code string) bool {
			for _, p := range list {
				if p.Code == code {
					return true
				}
			}
			return false
		}
		public, _ := svc.ListActivePublicPromos(ctx)
		if has(public, aff.Code) || !has(public, pub.Code) {
			t.Errorf("public list wrong: affiliator promo present=%v, public promo present=%v", has(public, aff.Code), has(public, pub.Code))
		}
		mine, _ := svc.ListActiveAffiliatorPromos(ctx)
		if !has(mine, aff.Code) || has(mine, pub.Code) || has(mine, notYet.Code) {
			t.Errorf("affiliator list wrong: want only %s", aff.Code)
		}
	})

	t.Run("validate enforces the role end to end", func(t *testing.T) {
		aff, err := create(t, func(p *model.PromoCode) { p.AffiliatorOnly = true })
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		line := []PromoLine{{ProductID: uuid.NewString(), Qty: 1, Amount: 1000}}
		if _, err := svc.ValidatePromoLines(ctx, aff.Code, RoleStudent, line, 0); !errors.Is(err, ErrInvalidPromo) {
			t.Errorf("student: err = %v, want ErrInvalidPromo", err)
		}
		if _, err := svc.ValidatePromo(ctx, aff.Code, 1000, 0); !errors.Is(err, ErrInvalidPromo) {
			t.Errorf("anonymous ValidatePromo: err = %v, want ErrInvalidPromo", err)
		}
		v, err := svc.ValidatePromoLines(ctx, aff.Code, RoleAffiliator, line, 0)
		if err != nil || v.Discount != 100 || v.Code != aff.Code {
			t.Errorf("affiliator: %+v, %v, want discount 100", v, err)
		}
	})
}
