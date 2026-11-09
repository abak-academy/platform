package worker

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func TestOrderPaidPartnerOrder_everyParticipantGetsTheCourseAndARefundTakesItBack(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	setAffiliateWorkerFlag(t, env.repo, "true")
	env.exec(t, `INSERT INTO system_config (key, value, is_secret) VALUES ('partner_enabled', 'true', false) ON CONFLICT (key) DO UPDATE SET value = 'true'`)
	t.Cleanup(func() { env.exec(t, `DELETE FROM system_config WHERE key = 'partner_enabled'`) })

	var partnerID uuid.UUID
	if err := env.repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, role, name, status) VALUES ($1, 'affiliator', 'Mitra Worker', 'active') RETURNING id`,
		"mitra-worker-"+uniqueWorkerSuffix()+"@test.local").Scan(&partnerID); err != nil {
		t.Fatalf("seed partner: %v", err)
	}
	env.exec(t, `INSERT INTO affiliators (user_id, program_status, terms_version, programs) VALUES ($1, 'active', 'v1', '{"affiliate": false, "partner": {"school_ids": []}}')`, partnerID)

	course := env.course(t)
	product := env.courseProduct(t, 100000, course)
	admin := env.admin(t)
	if _, err := env.svc.AdminSetProductPartnerAccess(ctx, admin, product.String(), "all", nil); err != nil {
		t.Fatalf("open product: %v", err)
	}

	first, second := env.student(t), env.student(t)
	env.exec(t, `UPDATE users SET registered_by = $1 WHERE id IN ($2, $3)`, partnerID, first, second)

	// The first student also belongs to an Affiliator through a referral binding.
	affiliator := seedWorkerAffiliator(t, env.repo, "active")
	bindWorkerStudent(t, env.repo, first, affiliator)
	var boundUntil *string
	if err := env.repo.Pool().QueryRow(ctx, `SELECT expires_at::text FROM affiliate_bindings WHERE student_id = $1`, first).Scan(&boundUntil); err != nil {
		t.Fatalf("read binding: %v", err)
	}

	pct := 100.0
	voucher, err := env.svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: "WRK" + uniqueWorkerSuffix(), DiscountPercent: &pct, Partner: &model.PartnerVoucher{QuotaUnit: "student"}})
	if err != nil {
		t.Fatalf("create voucher: %v", err)
	}
	t.Cleanup(func() {
		env.exec(t, `UPDATE orders SET promo_code_id = NULL WHERE promo_code_id = $1`, voucher.ID)
		env.exec(t, `DELETE FROM promo_code WHERE id = $1`, voucher.ID)
	})

	order, err := env.svc.CreatePartnerOrder(ctx, partnerID.String(), product.String(), []string{first.String(), second.String()}, voucher.Code)
	if err != nil {
		t.Fatalf("create partner order: %v", err)
	}
	if res, err := env.svc.CheckoutPartnerOrder(ctx, partnerID.String(), order.ID.String(), "key-"+uniqueWorkerSuffix()); err != nil || !res.Free {
		t.Fatalf("checkout = %+v, %v; want a free settlement", res, err)
	}
	pollAllWithWorker(t, env, &Worker{repo: env.repo, commissionSvc: env.svc})

	for _, student := range []uuid.UUID{first, second} {
		if got := env.activeEnrollments(t, student, course); got != 1 {
			t.Errorf("student %s active enrollments = %d, want 1", student, got)
		}
	}
	if got := env.activeEnrollments(t, partnerID, course); got != 0 {
		t.Errorf("the Mitra, the buyer, was enrolled itself: %d", got)
	}
	if got := env.count(t, `SELECT count(*) FROM course_session WHERE order_id = $1`, order.ID); got != 2 {
		t.Errorf("sessions tied to the order = %d, want 2, so a refund can find them", got)
	}

	if got := env.count(t, `SELECT count(*) FROM affiliate_transactions WHERE order_id = $1`, order.ID); got != 0 {
		t.Errorf("a Mitra order created %d affiliate transactions, want none", got)
	}
	var stillBoundUntil *string
	var bindings int
	if err := env.repo.Pool().QueryRow(ctx,
		`SELECT count(*), max(expires_at::text) FROM affiliate_bindings WHERE student_id = $1 AND status = 'active' AND affiliator_id = $2`, first, affiliator,
	).Scan(&bindings, &stillBoundUntil); err != nil {
		t.Fatalf("read binding: %v", err)
	}
	if bindings != 1 {
		t.Errorf("the student's Affiliator binding = %d, want it untouched", bindings)
	}
	if (boundUntil == nil) != (stillBoundUntil == nil) || (boundUntil != nil && *boundUntil != *stillBoundUntil) {
		t.Errorf("the binding was extended by a Mitra purchase: %v -> %v", boundUntil, stillBoundUntil)
	}

	if err := env.refundErr(order.ID); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if got := env.count(t, `SELECT count(*) FROM course_session WHERE order_id = $1 AND status = 'revoked'`, order.ID); got != 2 {
		t.Errorf("revoked sessions = %d, want both participants", got)
	}
	for _, student := range []uuid.UUID{first, second} {
		if got := env.activeEnrollments(t, student, course); got != 0 {
			t.Errorf("student %s still has access after the refund", student)
		}
	}
	if got := env.count(t, `SELECT count(*) FROM affiliate_transactions WHERE order_id = $1`, order.ID); got != 0 {
		t.Errorf("a refund of a Mitra order wrote %d affiliate transactions", got)
	}
}
