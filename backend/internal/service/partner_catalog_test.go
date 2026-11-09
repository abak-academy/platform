package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }

func TestEvaluatePartnerPromo(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	mitra := uuid.NewString()
	voucher := func(unit string, ids ...string) model.PromoCode {
		return model.PromoCode{
			ID: uuid.New(), Code: "MITRA", DiscountPercent: f(10),
			Partner: &model.PartnerVoucher{IDs: append([]string{}, ids...), QuotaUnit: unit},
		}
	}

	t.Run("who it applies to", func(t *testing.T) {
		_, err := evaluatePartnerPromo(model.PromoCode{Code: "BIASA", DiscountPercent: f(10)}, mitra, 100000, 1, now)
		require.ErrorIs(t, err, ErrInvalidPromo, "an ordinary promo is not a Mitra voucher")

		_, err = evaluatePartnerPromo(voucher("order", uuid.NewString()), mitra, 100000, 1, now)
		require.ErrorIs(t, err, ErrInvalidPromo, "a voucher made for another Mitra")

		_, err = evaluatePartnerPromo(voucher("order", mitra), mitra, 100000, 1, now)
		require.NoError(t, err, "a voucher naming this Mitra")

		_, err = evaluatePartnerPromo(voucher("order"), mitra, 100000, 1, now)
		require.NoError(t, err, "a voucher with no Mitra named is for every Mitra")
	})

	t.Run("minimum students in one order", func(t *testing.T) {
		v := voucher("order")
		v.Partner.MinStudents = 5

		_, err := evaluatePartnerPromo(v, mitra, 100000, 4, now)
		var minErr *PromoMinStudentsError
		require.ErrorIs(t, err, ErrPromoMinStudents)
		require.ErrorAs(t, err, &minErr)
		require.Equal(t, 5, minErr.Min)

		_, err = evaluatePartnerPromo(v, mitra, 100000, 5, now)
		require.NoError(t, err, "exactly the minimum is enough")

		v.Partner.MinStudents = 0
		_, err = evaluatePartnerPromo(v, mitra, 100000, 1, now)
		require.NoError(t, err, "zero means no minimum")
	})

	t.Run("expiry and quota", func(t *testing.T) {
		expired := voucher("order")
		past := now.Add(-time.Hour)
		expired.ExpiresAt = &past
		_, err := evaluatePartnerPromo(expired, mitra, 100000, 1, now)
		require.ErrorIs(t, err, ErrInvalidPromo)

		exhausted := voucher("student")
		exhausted.MaxUses, exhausted.UsedCount = i(10), 10
		_, err = evaluatePartnerPromo(exhausted, mitra, 100000, 1, now)
		require.ErrorIs(t, err, ErrInvalidPromo)

		perOrder := voucher("order")
		perOrder.MaxUses, perOrder.UsedCount = i(3), 2
		res, err := evaluatePartnerPromo(perOrder, mitra, 100000, 50, now)
		require.NoError(t, err, "an order of 50 students takes one use when uses are counted per order")
		require.Equal(t, 1, res.Uses)

		perStudent := voucher("student")
		perStudent.MaxUses, perStudent.UsedCount = i(10), 6
		_, err = evaluatePartnerPromo(perStudent, mitra, 100000, 5, now)
		var short *PromoQuotaInsufficientError
		require.ErrorAs(t, err, &short)
		require.ErrorIs(t, err, ErrPromoQuotaInsufficient)
		require.Equal(t, 4, short.Remaining, "the whole order is refused and the Mitra is told what is left")

		res, err = evaluatePartnerPromo(perStudent, mitra, 100000, 4, now)
		require.NoError(t, err)
		require.Equal(t, 4, res.Uses, "one use per student")
	})

	t.Run("the discount cap follows the quota unit", func(t *testing.T) {
		perOrder := voucher("order")
		perOrder.MaxDiscountAmount = f(30000)
		res, err := evaluatePartnerPromo(perOrder, mitra, 100000, 5, now)
		require.NoError(t, err)
		require.Equal(t, 30000.0, res.Discount, "10% of 500000 is capped once for the whole order")
		require.Equal(t, 470000.0, res.Total)

		perStudent := voucher("student")
		perStudent.MaxDiscountAmount = f(8000)
		res, err = evaluatePartnerPromo(perStudent, mitra, 100000, 5, now)
		require.NoError(t, err)
		require.Equal(t, 40000.0, res.Discount, "10% of 100000 is capped at 8000 for each of 5 students")
		require.Equal(t, 460000.0, res.Total)
	})

	t.Run("a fixed discount", func(t *testing.T) {
		perOrder := voucher("order")
		perOrder.DiscountPercent, perOrder.DiscountAmount = nil, f(25000)
		res, err := evaluatePartnerPromo(perOrder, mitra, 100000, 5, now)
		require.NoError(t, err)
		require.Equal(t, 25000.0, res.Discount)

		perStudent := voucher("student")
		perStudent.DiscountPercent, perStudent.DiscountAmount = nil, f(25000)
		res, err = evaluatePartnerPromo(perStudent, mitra, 20000, 3, now)
		require.NoError(t, err)
		require.Equal(t, 60000.0, res.Discount, "each student's discount cannot pass the unit price")
		require.Equal(t, 0.0, res.Total)
	})

	t.Run("the minimum is on the whole order", func(t *testing.T) {
		v := voucher("student")
		v.MinOrderAmount = f(300000)
		_, err := evaluatePartnerPromo(v, mitra, 100000, 2, now)
		require.ErrorIs(t, err, ErrPromoMinOrder)
		_, err = evaluatePartnerPromo(v, mitra, 100000, 3, now)
		require.NoError(t, err)
	})
}

func TestPartnerVoucherAdmin(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	mitra := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	plain := seedProgramAccount(t, repo, "active", "active", "AFV"+uniqueSuffix()[:5], "").String()
	code := func() string { return "MITRA" + uniqueSuffix() }
	cleanup := func(id uuid.UUID) {
		t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, id) })
	}

	t.Run("a Mitra voucher keeps its unit and Mitra and is never public", func(t *testing.T) {
		created, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{
			Code: code(), DiscountPercent: f(15), IsPublic: true,
			Partner: &model.PartnerVoucher{IDs: []string{mitra, mitra}, QuotaUnit: "student"},
		})
		require.NoError(t, err)
		cleanup(created.ID)
		require.NotNil(t, created.Partner)
		require.Equal(t, []string{mitra}, created.Partner.IDs, "duplicates are dropped")
		require.Equal(t, "student", created.Partner.QuotaUnit)
		require.False(t, created.IsPublic, "a Mitra voucher is never listed publicly")

		again, err := repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.Equal(t, created.Partner, again.Partner)
	})

	t.Run("the minimum students is stored, and cannot be negative", func(t *testing.T) {
		created, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{
			Code: code(), DiscountPercent: f(10),
			Partner: &model.PartnerVoucher{QuotaUnit: "order", MinStudents: 5},
		})
		require.NoError(t, err)
		cleanup(created.ID)
		again, err := repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.Equal(t, 5, again.Partner.MinStudents)

		_, err = svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(10), Partner: &model.PartnerVoucher{QuotaUnit: "order", MinStudents: -1}})
		require.ErrorIs(t, err, ErrValidation)

		require.NoError(t, svc.AdminUpdatePromoCode(ctx, created.ID.String(), nil, nil, false, false, &PartnerVoucherInput{Enabled: true, QuotaUnit: "order", MinStudents: 8}))
		again, err = repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.Equal(t, 8, again.Partner.MinStudents)
	})

	t.Run("the quota unit has no default", func(t *testing.T) {
		_, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{}})
		require.ErrorIs(t, err, ErrValidation)
	})

	t.Run("only Mitra accounts can be named", func(t *testing.T) {
		_, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{IDs: []string{plain}, QuotaUnit: "order"}})
		require.ErrorIs(t, err, ErrValidation, "an Affiliator-only account is not a Mitra")
		_, err = svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{IDs: []string{"nope"}, QuotaUnit: "order"}})
		require.ErrorIs(t, err, ErrInvalidUUID)
	})

	t.Run("it cannot also be a promo Affiliators share", func(t *testing.T) {
		_, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), AffiliatorOnly: true, Partner: &model.PartnerVoucher{QuotaUnit: "order"}})
		require.ErrorIs(t, err, ErrValidation)
	})

	t.Run("an update that omits the setting leaves it alone", func(t *testing.T) {
		created, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{QuotaUnit: "order"}})
		require.NoError(t, err)
		cleanup(created.ID)

		require.NoError(t, svc.AdminUpdatePromoCode(ctx, created.ID.String(), i(5), nil, true, false, nil))
		got, err := repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.NotNil(t, got.Partner, "an old client that never heard of the setting must not turn the voucher into a public promo")
		require.Equal(t, "order", got.Partner.QuotaUnit)
		require.Equal(t, 5, *got.MaxUses)

		require.NoError(t, svc.AdminUpdatePromoCode(ctx, created.ID.String(), nil, nil, false, false, &PartnerVoucherInput{Enabled: true, IDs: []string{mitra}, QuotaUnit: "student"}))
		got, err = repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.Equal(t, &model.PartnerVoucher{IDs: []string{mitra}, QuotaUnit: "student"}, got.Partner)

		require.NoError(t, svc.AdminUpdatePromoCode(ctx, created.ID.String(), nil, nil, false, false, &PartnerVoucherInput{Enabled: false}))
		got, err = repo.GetPromoByCode(ctx, created.Code)
		require.NoError(t, err)
		require.Nil(t, got.Partner, "switching it off makes the promo ordinary again")
	})

	t.Run("students and the public checks never see it", func(t *testing.T) {
		created, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(50), IsPublic: true, Partner: &model.PartnerVoucher{QuotaUnit: "order"}})
		require.NoError(t, err)
		cleanup(created.ID)

		_, err = svc.ValidatePromo(ctx, created.Code, 100000, 0)
		require.ErrorIs(t, err, ErrInvalidPromo, "a student who learns the code still cannot use it")

		public, err := svc.ListActivePublicPromos(ctx)
		require.NoError(t, err)
		for _, p := range public {
			require.NotEqual(t, created.Code, p.Code)
		}
	})

	t.Run("a Mitra sees the vouchers for everyone and for itself, not for others", func(t *testing.T) {
		other := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
		forAll, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{QuotaUnit: "order"}})
		require.NoError(t, err)
		forMe, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{IDs: []string{mitra}, QuotaUnit: "order"}})
		require.NoError(t, err)
		forOther, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), Partner: &model.PartnerVoucher{IDs: []string{other}, QuotaUnit: "order"}})
		require.NoError(t, err)
		expiredAt := time.Now().Add(-time.Hour)
		expired, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), ExpiresAt: &expiredAt, Partner: &model.PartnerVoucher{QuotaUnit: "order"}})
		require.NoError(t, err)
		shared, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code(), DiscountPercent: f(5), AffiliatorOnly: true})
		require.NoError(t, err)
		for _, p := range []model.PromoCode{forAll, forMe, forOther, expired, shared} {
			cleanup(p.ID)
		}

		got, err := svc.ListPartnerVouchers(ctx, mitra)
		require.NoError(t, err)
		codes := map[string]bool{}
		for _, p := range got {
			codes[p.Code] = true
		}
		require.True(t, codes[forAll.Code])
		require.True(t, codes[forMe.Code])
		require.False(t, codes[forOther.Code], "another Mitra's voucher")
		require.False(t, codes[expired.Code], "an expired voucher")
		require.False(t, codes[shared.Code], "a promo meant for students is not a voucher for the Mitra")
	})
}

func TestPartnerProductAccess(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "pa-actor-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })
	mitraA := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	mitraB := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()

	course := func() uuid.UUID {
		id := seedCommissionProduct(t, repo, "course")
		t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM product WHERE id = $1`, id) })
		return id
	}
	listed := func(partner string) map[uuid.UUID]bool {
		products, err := svc.ListPartnerProducts(ctx, partner)
		require.NoError(t, err)
		out := map[uuid.UUID]bool{}
		for _, p := range products {
			out[uuid.MustParse(p.ID)] = true
		}
		return out
	}

	none, all, selected := course(), course(), course()
	got, err := svc.AdminGetProductPartnerAccess(ctx, none.String())
	require.NoError(t, err)
	require.Equal(t, model.PartnerScopeNone, got.Scope, "a new product is not opened to anyone")

	_, err = svc.AdminSetProductPartnerAccess(ctx, actor, all.String(), "all", nil)
	require.NoError(t, err)
	_, err = svc.AdminSetProductPartnerAccess(ctx, actor, selected.String(), "selected", []string{mitraA})
	require.NoError(t, err)

	forA, forB := listed(mitraA), listed(mitraB)
	require.False(t, forA[none] || forB[none], "scope none shows to nobody")
	require.True(t, forA[all] && forB[all], "scope all shows to every Mitra")
	require.True(t, forA[selected], "scope selected shows to the Mitra named")
	require.False(t, forB[selected], "and not to the others")

	open, err := repo.ProductOpenToPartner(ctx, selected, mitraB)
	require.NoError(t, err)
	require.False(t, open)
	open, err = repo.ProductOpenToPartner(ctx, selected, mitraA)
	require.NoError(t, err)
	require.True(t, open)

	t.Run("saving the product form leaves the access alone", func(t *testing.T) {
		product, err := repo.GetProductByID(ctx, selected.String())
		require.NoError(t, err)
		product.Name = "Renamed " + uniqueSuffix()
		require.NoError(t, repo.UpdateProduct(ctx, selected.String(), product))
		after, err := svc.AdminGetProductPartnerAccess(ctx, selected.String())
		require.NoError(t, err)
		require.Equal(t, model.PartnerScopeSelected, after.Scope)
		require.Equal(t, []string{mitraA}, after.IDs)
	})

	t.Run("an exam product with no open exam stays out even for scope all", func(t *testing.T) {
		exam := seedCommissionProduct(t, repo, "exam")
		t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM product WHERE id = $1`, exam) })
		_, err := svc.AdminSetProductPartnerAccess(ctx, actor, exam.String(), "all", nil)
		require.NoError(t, err)
		require.False(t, listed(mitraA)[exam])
	})

	t.Run("a draft product stays out", func(t *testing.T) {
		draft := course()
		_, err := svc.AdminSetProductPartnerAccess(ctx, actor, draft.String(), "all", nil)
		require.NoError(t, err)
		_, err = repo.Pool().Exec(ctx, `UPDATE product SET status = 'draft' WHERE id = $1`, draft)
		require.NoError(t, err)
		require.False(t, listed(mitraA)[draft])
	})

	t.Run("setting none closes it again", func(t *testing.T) {
		_, err := svc.AdminSetProductPartnerAccess(ctx, actor, all.String(), "none", []string{mitraA})
		require.NoError(t, err)
		require.False(t, listed(mitraA)[all])
		var stored *string
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT partner_access::text FROM product WHERE id = $1`, all).Scan(&stored))
		require.Nil(t, stored, "none is stored as no value at all")
	})

	t.Run("validation", func(t *testing.T) {
		p := course()
		_, err := svc.AdminSetProductPartnerAccess(ctx, actor, p.String(), "selected", nil)
		require.ErrorIs(t, err, ErrValidation, "selected needs a Mitra")
		_, err = svc.AdminSetProductPartnerAccess(ctx, actor, p.String(), "everyone", nil)
		require.ErrorIs(t, err, ErrValidation)
		plain := seedProgramAccount(t, repo, "active", "active", "AFW"+uniqueSuffix()[:5], "").String()
		_, err = svc.AdminSetProductPartnerAccess(ctx, actor, p.String(), "selected", []string{plain})
		require.ErrorIs(t, err, ErrValidation, "only a Mitra account can be chosen")
		_, err = svc.AdminSetProductPartnerAccess(ctx, actor, uuid.NewString(), "all", nil)
		require.ErrorIs(t, err, ErrProductNotFound)

		var audited int
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = 'product.partner_access_set'`, selected).Scan(&audited))
		require.Equal(t, 1, audited)
	})
}

func TestIncrementPromoUsesTx_neverPassesMaxUses(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	var promoID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO promo_code (code, discount_percent, max_uses, used_count) VALUES ($1, 10, 3, 1) RETURNING id`, "QUOTA"+uniqueSuffix(),
	).Scan(&promoID))
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, promoID) })
	used := func() int {
		var n int
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT used_count FROM promo_code WHERE id = $1`, promoID).Scan(&n))
		return n
	}
	take := func(n int) error {
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		if err := repo.IncrementPromoUsesTx(ctx, tx, promoID, n); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		return tx.Commit(ctx)
	}

	err := take(3)
	require.ErrorIs(t, err, repository.ErrPromoQuotaExceeded, "3 more would pass max_uses of 3 from 1")
	require.Equal(t, 1, used(), "a refused take changes nothing")
	require.NoError(t, take(2), "taking exactly what is left is fine")
	require.Equal(t, 3, used())

	// Two checkouts racing for the last use: one wins, one is refused.
	_, err = repo.Pool().Exec(ctx, `UPDATE promo_code SET used_count = 2 WHERE id = $1`, promoID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make([]error, 6)
	for n := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[n] = take(1)
		}()
	}
	wg.Wait()
	wins := 0
	for _, e := range results {
		if e == nil {
			wins++
		} else {
			require.True(t, errors.Is(e, repository.ErrPromoQuotaExceeded), "unexpected error %v", e)
		}
	}
	require.Equal(t, 1, wins, "only one checkout may take the last use")
	require.Equal(t, 3, used())
}

func TestPromoUses_countsSeatsOnlyForAPerStudentMitraVoucher(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	mk := func(partner *model.PartnerVoucher) uuid.UUID {
		p, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "USES" + uniqueSuffix(), DiscountPercent: f(10), Partner: partner})
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, p.ID) })
		return p.ID
	}
	items := []model.OrderItem{{Qty: 3}}

	perStudent, perOrder, ordinary := mk(&model.PartnerVoucher{QuotaUnit: "student"}), mk(&model.PartnerVoucher{QuotaUnit: "order"}), mk(nil)
	for name, tc := range map[string]struct {
		id   uuid.UUID
		want int
	}{"per student": {perStudent, 3}, "per order": {perOrder, 1}, "ordinary": {ordinary, 1}} {
		got, err := svc.promoUses(ctx, model.Order{PromoCodeID: &tc.id, Items: items})
		require.NoError(t, err, name)
		require.Equal(t, tc.want, got, name)
	}
	got, err := svc.promoUses(ctx, model.Order{Items: items})
	require.NoError(t, err)
	require.Equal(t, 0, got, "no promo, no uses")
}
