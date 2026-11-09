package service

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type partnerOrderEnv struct {
	svc     *Service
	repo    *repository.Repository
	actor   string
	partner string
	schoolA string
	schoolB string
}

func newPartnerOrderEnv(t *testing.T) *partnerOrderEnv {
	t.Helper()
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret) VALUES ('partner_enabled', 'true', false) ON CONFLICT (key) DO UPDATE SET value = 'true'`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM system_config WHERE key = 'partner_enabled'`) })

	actor := seedAffiliateAdminActor(t, repo, "po-actor-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })
	env := &partnerOrderEnv{svc: svc, repo: repo, actor: actor, schoolA: createTestSchool(t, svc), schoolB: createTestSchool(t, svc)}
	env.partner = seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	_, err = svc.AdminSetPartnerSchools(ctx, actor, env.partner, []string{env.schoolA})
	require.NoError(t, err)
	return env
}

// courseProduct creates a published course product linked to one course.
func (e *partnerOrderEnv) courseProduct(t *testing.T, price int) (productID, courseID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, e.repo.Pool().QueryRow(ctx,
		`INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'Guru') RETURNING id`, "Kelas "+uniqueSuffix(),
	).Scan(&courseID))
	require.NoError(t, e.repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, status) VALUES ('course', $1, $2, 'published') RETURNING id`, "Produk Kelas "+uniqueSuffix(), price,
	).Scan(&productID))
	_, err := e.repo.Pool().Exec(ctx, `INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`, productID, courseID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = e.repo.Pool().Exec(ctx, `DELETE FROM course_session WHERE course_id = $1`, courseID)
		_, _ = e.repo.Pool().Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, productID)
	})
	return productID, courseID
}

func (e *partnerOrderEnv) open(t *testing.T, productID uuid.UUID) {
	t.Helper()
	_, err := e.svc.AdminSetProductPartnerAccess(context.Background(), e.actor, productID.String(), "all", nil)
	require.NoError(t, err)
}

func (e *partnerOrderEnv) voucher(t *testing.T, percent float64, unit string, maxUses *int) model.PromoCode {
	t.Helper()
	p, err := e.svc.AdminCreatePromoCode(context.Background(), model.PromoCode{
		Code: "PO" + uniqueSuffix(), DiscountPercent: &percent, MaxUses: maxUses,
		Partner: &model.PartnerVoucher{QuotaUnit: unit},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = e.repo.Pool().Exec(context.Background(), `UPDATE orders SET promo_code_id = NULL WHERE promo_code_id = $1`, p.ID)
		_, _ = e.repo.Pool().Exec(context.Background(), `DELETE FROM promo_code WHERE id = $1`, p.ID)
	})
	return p
}

func (e *partnerOrderEnv) student(t *testing.T, name, schoolID, registeredBy string) string {
	t.Helper()
	return seedStudent(t, e.repo, name+" "+uniqueSuffix(), schoolID, registeredBy, true)
}

func (e *partnerOrderEnv) enroll(t *testing.T, studentID string, courseID uuid.UUID) {
	t.Helper()
	_, err := e.repo.Pool().Exec(context.Background(),
		`INSERT INTO course_session (student_id, course_id, status, source) VALUES ($1, $2, 'active', 'order')`, studentID, courseID)
	require.NoError(t, err)
}

func TestPreviewPartnerOrder_whoAndWhatAMitraMayOrder(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	product, course := env.courseProduct(t, 100000)
	env.open(t, product)

	inSchool := env.student(t, "Ani", env.schoolA, "")
	mine := env.student(t, "Budi", "", env.partner)
	mineElsewhere := env.student(t, "Cici", env.schoolB, env.partner)
	outside := env.student(t, "Dedi", env.schoolB, "")
	otherMitrasNoSchool := env.student(t, "Eko", "", seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String())

	t.Run("the allowed school and the students the Mitra registered", func(t *testing.T) {
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{inSchool, mine, mineElsewhere}, "")
		require.NoError(t, err)
		require.Equal(t, 3, preview.NetNewCount)
		require.Equal(t, 300000.0, preview.Subtotal)
		require.Equal(t, 300000.0, preview.Total)
		require.Empty(t, preview.Excluded)
	})

	t.Run("one student out of scope refuses the whole request", func(t *testing.T) {
		for _, bad := range []string{outside, otherMitrasNoSchool, uuid.NewString()} {
			_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{inSchool, bad}, "")
			require.ErrorIs(t, err, ErrPartnerStudentNotAllowed, bad)
		}
	})

	t.Run("a student who already owns the product is left out and not charged", func(t *testing.T) {
		owner := env.student(t, "Fani", env.schoolA, "")
		env.enroll(t, owner, course)
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{inSchool, owner}, "")
		require.NoError(t, err)
		require.Equal(t, 1, preview.NetNewCount)
		require.Equal(t, 100000.0, preview.Subtotal)
		require.Len(t, preview.Excluded, 1)
		require.Equal(t, owner, preview.Excluded[0].StudentID)
		require.Equal(t, "already_owned", preview.Excluded[0].Reason)

		_, err = env.svc.CreatePartnerOrder(ctx, env.partner, product.String(), []string{owner}, "")
		require.ErrorIs(t, err, ErrZeroNetNewParticipants)
	})

	t.Run("a revoked enrollment does not count as owning", func(t *testing.T) {
		refunded := env.student(t, "Gita", env.schoolA, "")
		env.enroll(t, refunded, course)
		_, err := env.repo.Pool().Exec(ctx, `UPDATE course_session SET status = 'revoked' WHERE student_id = $1`, refunded)
		require.NoError(t, err)
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{refunded}, "")
		require.NoError(t, err)
		require.Equal(t, 1, preview.NetNewCount, "after a refund the student can be bought for again")
	})

	t.Run("bad requests", func(t *testing.T) {
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), nil, "")
		require.ErrorIs(t, err, ErrEmptySelector)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{inSchool, inSchool}, "")
		require.ErrorIs(t, err, ErrDuplicateParticipant)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{"nope"}, "")
		require.ErrorIs(t, err, ErrInvalidUUID)
	})
}

func TestPreviewPartnerOrder_productRules(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	student := env.student(t, "Ani", env.schoolA, "")

	t.Run("a product nobody opened to a Mitra is refused", func(t *testing.T) {
		closed, _ := env.courseProduct(t, 100000)
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, closed.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductNotAllowed)
	})

	t.Run("a product opened to another Mitra only", func(t *testing.T) {
		product, _ := env.courseProduct(t, 100000)
		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		_, err := env.svc.AdminSetProductPartnerAccess(ctx, env.actor, product.String(), "selected", []string{other})
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductNotAllowed)
	})

	t.Run("only exam and course products", func(t *testing.T) {
		var book uuid.UUID
		require.NoError(t, env.repo.Pool().QueryRow(ctx,
			`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, 5, 'published', 300) RETURNING id`, "Buku "+uniqueSuffix(),
		).Scan(&book))
		env.open(t, book)
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, book.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductType)
	})

	t.Run("a draft product does not exist for a Mitra", func(t *testing.T) {
		draft, _ := env.courseProduct(t, 100000)
		env.open(t, draft)
		_, err := env.repo.Pool().Exec(ctx, `UPDATE product SET status = 'draft' WHERE id = $1`, draft)
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, draft.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrProductNotFound)
	})

	t.Run("a suspended account and a switched-off program cannot start an order", func(t *testing.T) {
		product, _ := env.courseProduct(t, 100000)
		env.open(t, product)

		suspended := seedProgramAccount(t, env.repo, "deactivated", "suspended", "", partnerOnlyPrograms).String()
		_, err := env.svc.PreviewPartnerOrder(ctx, suspended, product.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrAccountDeactivated)

		_, err = env.repo.Pool().Exec(ctx, `UPDATE system_config SET value = 'false' WHERE key = 'partner_enabled'`)
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProgramOff)
	})
}

func TestPreviewPartnerOrder_voucher(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	product, _ := env.courseProduct(t, 100000)
	env.open(t, product)
	a, b := env.student(t, "Ani", env.schoolA, ""), env.student(t, "Budi", env.schoolA, "")

	perStudent := env.voucher(t, 10, "student", nil)
	preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{a, b}, perStudent.Code)
	require.NoError(t, err)
	require.Equal(t, 200000.0, preview.Subtotal)
	require.Equal(t, 20000.0, preview.Discount)
	require.Equal(t, 180000.0, preview.Total)
	require.Equal(t, perStudent.Code, preview.PromoCode)

	t.Run("an ordinary promo does not apply to a Mitra order", func(t *testing.T) {
		pct := 50.0
		ordinary, err := env.svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "ORD" + uniqueSuffix(), DiscountPercent: &pct, IsPublic: true})
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = env.repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, ordinary.ID) })
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{a}, ordinary.Code)
		require.ErrorIs(t, err, ErrInvalidPromo)
	})

	t.Run("a voucher for another Mitra does not apply", func(t *testing.T) {
		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		pct := 10.0
		v, err := env.svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "OTH" + uniqueSuffix(), DiscountPercent: &pct, Partner: &model.PartnerVoucher{IDs: []string{other}, QuotaUnit: "order"}})
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = env.repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, v.ID) })
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{a}, v.Code)
		require.ErrorIs(t, err, ErrInvalidPromo)
	})

	t.Run("an order bigger than the quota left is refused, with what is left", func(t *testing.T) {
		limited := env.voucher(t, 10, "student", i(1))
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, product.String(), []string{a, b}, limited.Code)
		var short *PromoQuotaInsufficientError
		require.ErrorAs(t, err, &short)
		require.Equal(t, 1, short.Remaining)
	})
}

func TestCreateAndCheckoutPartnerOrder(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	product, _ := env.courseProduct(t, 100000)
	env.open(t, product)
	a, b := env.student(t, "Ani", env.schoolA, ""), env.student(t, "Budi", "", env.partner)
	full := env.voucher(t, 100, "student", i(10))

	order, err := env.svc.CreatePartnerOrder(ctx, env.partner, product.String(), []string{a, b}, full.Code)
	require.NoError(t, err)
	require.Equal(t, env.partner, order.StudentID.String(), "the Mitra is the buyer")
	require.Equal(t, "cart", order.Status)
	require.Len(t, order.Items, 1)
	require.Equal(t, 2, order.Items[0].Qty, "qty is the seat count")
	require.Equal(t, 200000.0, order.Subtotal)
	require.Equal(t, 200000.0, order.Discount)
	require.Equal(t, 0.0, order.Total)
	require.NotNil(t, order.PromoCodeID)

	participants, err := env.repo.GetOrderParticipants(ctx, order.ID)
	require.NoError(t, err)
	require.Len(t, participants, 2)

	usedCount := func() int {
		var n int
		require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT used_count FROM promo_code WHERE id = $1`, full.ID).Scan(&n))
		return n
	}
	require.Equal(t, 0, usedCount(), "creating an order does not use the voucher yet")

	t.Run("another Mitra cannot pay it or read it", func(t *testing.T) {
		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		_, err := env.svc.CheckoutPartnerOrder(ctx, other, order.ID.String(), "key-"+uniqueSuffix())
		require.ErrorIs(t, err, ErrOrderNotFound)
		_, err = env.svc.GetPartnerOrder(ctx, other, order.ID.String())
		require.ErrorIs(t, err, ErrOrderNotFound)
	})

	t.Run("a second order replaces the unpaid first one", func(t *testing.T) {
		second, err := env.svc.CreatePartnerOrder(ctx, env.partner, product.String(), []string{a}, "")
		require.NoError(t, err)
		var status string
		require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&status))
		require.Equal(t, "cancelled", status)

		order = second
	})

	t.Run("a fully discounted order is paid at once and takes one voucher use per student", func(t *testing.T) {
		withVoucher, err := env.svc.CreatePartnerOrder(ctx, env.partner, product.String(), []string{a, b}, full.Code)
		require.NoError(t, err)
		res, err := env.svc.CheckoutPartnerOrder(ctx, env.partner, withVoucher.ID.String(), "key-"+uniqueSuffix())
		require.NoError(t, err)
		require.True(t, res.Free)
		var status string
		require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, withVoucher.ID).Scan(&status))
		require.Equal(t, "paid", status)
		require.Equal(t, 2, usedCount(), "two students, two uses")

		detail, err := env.svc.GetPartnerOrder(ctx, env.partner, withVoucher.ID.String())
		require.NoError(t, err)
		require.Len(t, detail.Students, 2)

		list, _, err := env.svc.ListPartnerOrders(ctx, env.partner, "", 20)
		require.NoError(t, err)
		require.NotEmpty(t, list)
	})
}

func TestCheckoutPartnerOrder_revalidatesWhatChangedSinceTheOrderWasCreated(t *testing.T) {
	ctx := context.Background()

	// each case builds a fresh order, changes one thing, and expects the checkout
	// to be refused before it touches the order or the voucher.
	tests := []struct {
		name   string
		change func(t *testing.T, env *partnerOrderEnv, product, course uuid.UUID, students []string, v model.PromoCode)
		check  func(t *testing.T, err error)
	}{
		{"the product is closed to the Mitra", func(t *testing.T, env *partnerOrderEnv, product, _ uuid.UUID, _ []string, _ model.PromoCode) {
			_, err := env.svc.AdminSetProductPartnerAccess(ctx, env.actor, product.String(), "none", nil)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrPartnerProductNotAllowed) }},
		{"a student leaves the allowed schools", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, _ model.PromoCode) {
			_, err := env.svc.AdminSetPartnerSchools(ctx, env.actor, env.partner, nil)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrPartnerStudentNotAllowed) }},
		{"a student has bought the product meanwhile", func(t *testing.T, env *partnerOrderEnv, _, course uuid.UUID, students []string, _ model.PromoCode) {
			env.enroll(t, students[0], course)
		}, func(t *testing.T, err error) {
			var owns *ParticipantAlreadyOwnsError
			require.ErrorAs(t, err, &owns)
			require.ErrorIs(t, err, ErrParticipantAlreadyOwns)
			require.Len(t, owns.Students, 1, "the Mitra is told which student")
		}},
		{"the voucher expired", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, v model.PromoCode) {
			_, err := env.repo.Pool().Exec(ctx, `UPDATE promo_code SET expires_at = now() - interval '1 hour' WHERE id = $1`, v.ID)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrInvalidPromo) }},
		{"the voucher changed its discount", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, v model.PromoCode) {
			_, err := env.repo.Pool().Exec(ctx, `UPDATE promo_code SET discount_percent = 40 WHERE id = $1`, v.ID)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrInvalidPromo) }},
		{"another order used up the quota", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, v model.PromoCode) {
			_, err := env.repo.Pool().Exec(ctx, `UPDATE promo_code SET used_count = max_uses WHERE id = $1`, v.ID)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrInvalidPromo) }},
		{"the program is switched off", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, _ model.PromoCode) {
			_, err := env.repo.Pool().Exec(ctx, `UPDATE system_config SET value = 'false' WHERE key = 'partner_enabled'`)
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrPartnerProgramOff) }},
		{"the account is suspended", func(t *testing.T, env *partnerOrderEnv, _, _ uuid.UUID, _ []string, _ model.PromoCode) {
			_, err := env.svc.ChangeAffiliateStatus(ctx, env.actor, env.partner, "suspended", "uji")
			require.NoError(t, err)
		}, func(t *testing.T, err error) { require.ErrorIs(t, err, ErrAccountDeactivated) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newPartnerOrderEnv(t)
			product, course := env.courseProduct(t, 100000)
			env.open(t, product)
			students := []string{env.student(t, "Ani", env.schoolA, ""), env.student(t, "Budi", env.schoolA, "")}
			v := env.voucher(t, 10, "student", i(10))
			order, err := env.svc.CreatePartnerOrder(ctx, env.partner, product.String(), students, v.Code)
			require.NoError(t, err)

			tt.change(t, env, product, course, students, v)

			_, err = env.svc.CheckoutPartnerOrder(ctx, env.partner, order.ID.String(), "key-"+uniqueSuffix())
			require.Error(t, err)
			tt.check(t, err)

			var status string
			var used int
			require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&status))
			require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT used_count FROM promo_code WHERE id = $1`, v.ID).Scan(&used))
			require.Equal(t, "cart", status, "a refused checkout leaves the order as it was")
			if tt.name != "another order used up the quota" {
				require.Equal(t, 0, used, "and the voucher unused")
			}
		})
	}
}

func TestCheckout_aParticipantOrderMayHaveManySeatsButASelfOrderMayNot(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	product, _ := env.courseProduct(t, 0)
	env.open(t, product)
	buyer := env.student(t, "Pembeli", env.schoolA, "")

	cart := func(participants []uuid.UUID) uuid.UUID {
		tx, err := env.repo.BeginTx(ctx)
		require.NoError(t, err)
		order, err := env.repo.CreateOrderTx(ctx, tx, uuid.MustParse(buyer))
		require.NoError(t, err)
		require.NoError(t, env.repo.InsertOrderItemTx(ctx, tx, order.ID, model.OrderItem{ProductID: product, ProductType: "course", Name: "Kelas", UnitPrice: 0, Qty: 3}))
		require.NoError(t, env.repo.InsertOrderParticipantsTx(ctx, tx, order.ID, participants))
		require.NoError(t, tx.Commit(ctx))
		return order.ID
	}

	selfOrder := cart(nil)
	_, err := env.svc.Checkout(ctx, buyer, selfOrder.String(), "key-"+uniqueSuffix(), "")
	require.True(t, errors.Is(err, ErrDigitalQtyLimit), "a digital line bought for oneself is still one seat: %v", err)

	_, err = env.repo.Pool().Exec(ctx, `UPDATE orders SET status = 'cancelled' WHERE id = $1`, selfOrder)
	require.NoError(t, err)

	three := []uuid.UUID{uuid.MustParse(env.student(t, "A", env.schoolA, "")), uuid.MustParse(env.student(t, "B", env.schoolA, "")), uuid.MustParse(env.student(t, "C", env.schoolA, ""))}
	seats := cart(three)
	res, err := env.svc.Checkout(ctx, buyer, seats.String(), "key-"+uniqueSuffix(), "")
	require.NoError(t, err, "with participants qty is the seat count")
	require.True(t, res.Free)
}
