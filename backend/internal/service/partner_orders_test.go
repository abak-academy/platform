package service

import (
	"context"
	"errors"
	"testing"
	"time"

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
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{inSchool, mine, mineElsewhere}, "")
		require.NoError(t, err)
		require.Len(t, preview.Items, 1)
		require.Equal(t, 3, preview.Items[0].NetNewCount)
		require.Equal(t, 300000.0, preview.Subtotal)
		require.Equal(t, 300000.0, preview.Total)
		require.Empty(t, preview.Items[0].Excluded)
	})

	t.Run("one student out of scope refuses the whole request", func(t *testing.T) {
		for _, bad := range []string{outside, otherMitrasNoSchool, uuid.NewString()} {
			_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{inSchool, bad}, "")
			require.ErrorIs(t, err, ErrPartnerStudentNotAllowed, bad)
		}
	})

	t.Run("a student who already owns the product is left out and not charged", func(t *testing.T) {
		owner := env.student(t, "Fani", env.schoolA, "")
		env.enroll(t, owner, course)
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{inSchool, owner}, "")
		require.NoError(t, err)
		require.Len(t, preview.Items, 1)
		require.Equal(t, 1, preview.Items[0].NetNewCount)
		require.Equal(t, 100000.0, preview.Subtotal)
		require.Len(t, preview.Items[0].Excluded, 1)
		require.Equal(t, owner, preview.Items[0].Excluded[0].StudentID)
		require.Equal(t, "already_owned", preview.Items[0].Excluded[0].Reason)

		_, err = env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, []string{owner}, "")
		require.ErrorIs(t, err, ErrZeroNetNewParticipants)
	})

	t.Run("a revoked enrollment does not count as owning", func(t *testing.T) {
		refunded := env.student(t, "Gita", env.schoolA, "")
		env.enroll(t, refunded, course)
		_, err := env.repo.Pool().Exec(ctx, `UPDATE course_session SET status = 'revoked' WHERE student_id = $1`, refunded)
		require.NoError(t, err)
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{refunded}, "")
		require.NoError(t, err)
		require.Equal(t, 1, preview.Items[0].NetNewCount, "after a refund the student can be bought for again")
	})

	t.Run("bad requests", func(t *testing.T) {
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, nil, "")
		require.ErrorIs(t, err, ErrEmptySelector)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{inSchool, inSchool}, "")
		require.ErrorIs(t, err, ErrDuplicateParticipant)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{"nope"}, "")
		require.ErrorIs(t, err, ErrInvalidUUID)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, nil, []string{inSchool}, "")
		require.ErrorIs(t, err, ErrEmptySelector, "no product chosen")
	})
}

// A Mitra buying two products at once: ownership is resolved per product, so
// a student already owning one product is only dropped from that product's
// line and still served the other.
func TestPreviewAndCreatePartnerOrder_multiProduct(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	productA, courseA := env.courseProduct(t, 100000)
	productB, _ := env.courseProduct(t, 50000)
	env.open(t, productA)
	env.open(t, productB)

	ani := env.student(t, "Ani", env.schoolA, "")
	budi := env.student(t, "Budi", env.schoolA, "")
	env.enroll(t, ani, courseA) // Ani already owns product A, not B.

	t.Run("preview excludes a student from only the product they already own", func(t *testing.T) {
		preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{productA.String(), productB.String()}, []string{ani, budi}, "")
		require.NoError(t, err)
		require.Len(t, preview.Items, 2)
		byProduct := map[string]PartnerOrderItemPreview{}
		for _, item := range preview.Items {
			byProduct[item.ProductID] = item
		}
		require.Equal(t, 1, byProduct[productA.String()].NetNewCount, "only Budi is net-new on A")
		require.Len(t, byProduct[productA.String()].Excluded, 1)
		require.Equal(t, ani, byProduct[productA.String()].Excluded[0].StudentID)
		require.Equal(t, 2, byProduct[productB.String()].NetNewCount, "both are net-new on B")
		require.Empty(t, byProduct[productB.String()].Excluded)
		require.Equal(t, 100000.0+50000.0*2, preview.Subtotal, "A charges Budi once, B charges both")
		require.Len(t, preview.Students, 2, "Ani is still served (by B), so still a served student")
	})

	t.Run("create: one order, one order_item per product, Ani still gets B", func(t *testing.T) {
		order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{productA.String(), productB.String()}, []string{ani, budi}, "")
		require.NoError(t, err)
		require.Len(t, order.Items, 2, "one order_item per product that has someone net-new")

		var itemA, itemB model.OrderItem
		for _, item := range order.Items {
			switch item.ProductID.String() {
			case productA.String():
				itemA = item
			case productB.String():
				itemB = item
			}
		}
		require.Equal(t, 1, itemA.Qty, "only Budi on A")
		require.Equal(t, 2, itemB.Qty, "both on B")

		aniOnA, err := env.repo.GetOrderItemParticipants(ctx, itemA.ID)
		require.NoError(t, err)
		require.NotContains(t, aniOnA, uuid.MustParse(ani), "Ani already owns A, so she is not a participant on A's item")

		aniOnB, err := env.repo.GetOrderItemParticipants(ctx, itemB.ID)
		require.NoError(t, err)
		require.Contains(t, aniOnB, uuid.MustParse(ani), "Ani is still served by B")

		all, err := env.repo.GetOrderParticipants(ctx, order.ID)
		require.NoError(t, err)
		require.Len(t, all, 2, "both students are participants somewhere on the order")
	})

	t.Run("a product every selected student already owns is dropped, the order still succeeds for the rest", func(t *testing.T) {
		order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{productA.String(), productB.String()}, []string{ani}, "")
		require.NoError(t, err)
		require.Len(t, order.Items, 1, "A is dropped entirely — Ani is its only candidate and already owns it")
		require.Equal(t, productB.String(), order.Items[0].ProductID.String())
	})

	t.Run("every product already owned by every selected student fails the whole order", func(t *testing.T) {
		_, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{productA.String()}, []string{ani}, "")
		require.ErrorIs(t, err, ErrZeroNetNewParticipants)
	})

	t.Run("the order detail lists a student once even though they are a participant on two items", func(t *testing.T) {
		order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{productA.String(), productB.String()}, []string{budi}, "")
		require.NoError(t, err)
		require.Len(t, order.Items, 2, "Budi owns neither product yet, so both items survive")

		detail, err := env.svc.GetPartnerOrder(ctx, env.partner, order.ID.String())
		require.NoError(t, err)
		require.Len(t, detail.Students, 1, "Budi is a participant on both items but must appear once")
		require.Equal(t, budi, detail.Students[0].ID.String())
	})
}

func TestPreviewPartnerOrder_productRules(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	student := env.student(t, "Ani", env.schoolA, "")

	t.Run("a product nobody opened to a Mitra is refused", func(t *testing.T) {
		closed, _ := env.courseProduct(t, 100000)
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{closed.String()}, []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductNotAllowed)
	})

	t.Run("a product opened to another Mitra only", func(t *testing.T) {
		product, _ := env.courseProduct(t, 100000)
		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		_, err := env.svc.AdminSetProductPartnerAccess(ctx, env.actor, product.String(), "selected", []string{other})
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductNotAllowed)
	})

	t.Run("only exam and course products", func(t *testing.T) {
		var book uuid.UUID
		require.NoError(t, env.repo.Pool().QueryRow(ctx,
			`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, 5, 'published', 300) RETURNING id`, "Buku "+uniqueSuffix(),
		).Scan(&book))
		env.open(t, book)
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{book.String()}, []string{student}, "")
		require.ErrorIs(t, err, ErrPartnerProductType)
	})

	t.Run("a draft product does not exist for a Mitra", func(t *testing.T) {
		draft, _ := env.courseProduct(t, 100000)
		env.open(t, draft)
		_, err := env.repo.Pool().Exec(ctx, `UPDATE product SET status = 'draft' WHERE id = $1`, draft)
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{draft.String()}, []string{student}, "")
		require.ErrorIs(t, err, ErrProductNotFound)
	})

	t.Run("a suspended account and a switched-off program cannot start an order", func(t *testing.T) {
		product, _ := env.courseProduct(t, 100000)
		env.open(t, product)

		suspended := seedProgramAccount(t, env.repo, "deactivated", "suspended", "", partnerOnlyPrograms).String()
		_, err := env.svc.PreviewPartnerOrder(ctx, suspended, []string{product.String()}, []string{student}, "")
		require.ErrorIs(t, err, ErrAccountDeactivated)

		_, err = env.repo.Pool().Exec(ctx, `UPDATE system_config SET value = 'false' WHERE key = 'partner_enabled'`)
		require.NoError(t, err)
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{student}, "")
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
	preview, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{a, b}, perStudent.Code)
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
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{a}, ordinary.Code)
		require.ErrorIs(t, err, ErrInvalidPromo)
	})

	t.Run("a voucher for another Mitra does not apply", func(t *testing.T) {
		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		pct := 10.0
		v, err := env.svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "OTH" + uniqueSuffix(), DiscountPercent: &pct, Partner: &model.PartnerVoucher{IDs: []string{other}, QuotaUnit: "order"}})
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = env.repo.Pool().Exec(ctx, `DELETE FROM promo_code WHERE id = $1`, v.ID) })
		_, err = env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{a}, v.Code)
		require.ErrorIs(t, err, ErrInvalidPromo)
	})

	t.Run("an order bigger than the quota left is refused, with what is left", func(t *testing.T) {
		limited := env.voucher(t, 10, "student", i(1))
		_, err := env.svc.PreviewPartnerOrder(ctx, env.partner, []string{product.String()}, []string{a, b}, limited.Code)
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

	order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, []string{a, b}, full.Code)
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
		second, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, []string{a}, "")
		require.NoError(t, err)
		var status string
		require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&status))
		require.Equal(t, "cancelled", status)

		order = second
	})

	t.Run("a fully discounted order is paid at once and takes one voucher use per student", func(t *testing.T) {
		withVoucher, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, []string{a, b}, full.Code)
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
		require.Equal(t, full.Code, detail.PromoCode, "the detail names the voucher the order used")

		list, _, err := env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{}, 1, 20)
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
			order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, students, v.Code)
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

// On a multi-product order, only the item whose participant actually became
// an owner blocks checkout — the other item's own participants are checked
// against its own product, independently.
func TestCheckoutPartnerOrder_revalidatesEachItemIndependently(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	productA, courseA := env.courseProduct(t, 100000)
	productB, _ := env.courseProduct(t, 50000)
	env.open(t, productA)
	env.open(t, productB)
	ani := env.student(t, "Ani", env.schoolA, "")
	budi := env.student(t, "Budi", env.schoolA, "")

	order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{productA.String(), productB.String()}, []string{ani, budi}, "")
	require.NoError(t, err)

	// Ani buys product A herself before the Mitra pays — only A's item is affected.
	env.enroll(t, ani, courseA)

	_, err = env.svc.CheckoutPartnerOrder(ctx, env.partner, order.ID.String(), "key-"+uniqueSuffix())
	var owns *ParticipantAlreadyOwnsError
	require.ErrorAs(t, err, &owns)
	require.Len(t, owns.Students, 1, "only Ani, only on A")
	require.Equal(t, ani, owns.Students[0].ID.String())

	var status string
	require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&status))
	require.Equal(t, "cart", status, "the whole order stays unpaid, not partially settled")
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
		itemID, err := env.repo.InsertOrderItemTx(ctx, tx, order.ID, model.OrderItem{ProductID: product, ProductType: "course", Name: "Kelas", UnitPrice: 0, Qty: 3})
		require.NoError(t, err)
		require.NoError(t, env.repo.InsertOrderItemParticipantsTx(ctx, tx, order.ID, itemID, participants))
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

func TestListPartnerOrders_pagesAndFilters(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	first, _ := env.courseProduct(t, 100000)
	second, _ := env.courseProduct(t, 50000)
	env.open(t, first)
	env.open(t, second)
	ani := env.student(t, "Ani", env.schoolA, "")
	budi := env.student(t, "Budi", env.schoolA, "")

	one, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{first.String()}, []string{ani}, "")
	require.NoError(t, err)
	two, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{second.String()}, []string{budi}, "")
	require.NoError(t, err)
	var firstName string
	require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT name FROM product WHERE id = $1`, first).Scan(&firstName))

	t.Run("a page carries the total the filters match", func(t *testing.T) {
		list, total, err := env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, list, 1)
		require.Equal(t, two.ID, list[0].ID, "newest first")
		require.NotEmpty(t, list[0].Items)

		list, _, err = env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{}, 2, 1)
		require.NoError(t, err)
		require.Equal(t, one.ID, list[0].ID)
	})

	t.Run("a search finds a product on the order or a student it covers", func(t *testing.T) {
		list, total, err := env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{Q: firstName}, 1, 20)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, one.ID, list[0].ID)

		list, total, err = env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{Q: "Budi"}, 1, 20)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, two.ID, list[0].ID)
	})

	t.Run("status and date narrow the list, and another Mitra's orders never show", func(t *testing.T) {
		_, total, err := env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{Status: "paid"}, 1, 20)
		require.NoError(t, err)
		require.Zero(t, total)

		tomorrow := time.Now().Add(24 * time.Hour)
		_, total, err = env.svc.ListPartnerOrders(ctx, env.partner, repository.PartnerOrderFilter{From: &tomorrow}, 1, 20)
		require.NoError(t, err)
		require.Zero(t, total)

		other := seedProgramAccount(t, env.repo, "active", "active", "", partnerOnlyPrograms).String()
		_, total, err = env.svc.ListPartnerOrders(ctx, other, repository.PartnerOrderFilter{}, 1, 20)
		require.NoError(t, err)
		require.Zero(t, total)
	})
}
