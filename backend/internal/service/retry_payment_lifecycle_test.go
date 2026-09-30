package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func retryKey() string { return "retry-key-" + uniqueSuffix() }

func gatewayRefOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) string {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.GatewayRef
}

func TestRetryPayment_ExpiredOrderIsRefusedWithoutGatewayCall(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, orderID)
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("err = %v, want ErrPaymentExpired", err)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired (never reopened)", got)
	}
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
	if got := liveClaimsOf(t, repo, student); got != 0 {
		t.Errorf("live claims = %d, want 0", got)
	}
}

func TestRetryPayment_ElapsedDeadlineIsRefusedEvenBeforeTheSweeperRuns(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET payment_expires_at = now() - interval '1 second' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("elapse deadline: %v", err)
	}
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("err = %v, want ErrPaymentExpired", err)
	}
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_pending" {
		t.Errorf("status = %q, want payment_pending (retry does not expire orders itself)", got)
	}
}

func TestRetryPayment_CachedResponseDoesNotBypassOrderState(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	key := retryKey()

	if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key); err != nil {
		t.Fatalf("first retry: %v", err)
	}
	if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key); err != nil {
		t.Fatalf("replayed retry on an unchanged order: %v", err)
	}
	created := pay.creates.Load()

	expireOrderNow(t, repo, orderID)
	if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key); !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("replayed retry on an expired order: err = %v, want ErrPaymentExpired", err)
	}

	paidID, _, _ := pendingCourseOrder(t, svc, repo, newDigitalStudent(t, repo))
	paidStudent := studentOf(t, repo, paidID)
	paidKey := retryKey()
	if _, err := svc.RetryPayment(ctx, paidStudent.String(), paidID.String(), paidKey); err != nil {
		t.Fatalf("retry before payment: %v", err)
	}
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(paidID, "tx-cache"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if _, err := svc.RetryPayment(ctx, paidStudent.String(), paidID.String(), paidKey); !errors.Is(err, ErrOrderNotEditable) {
		t.Fatalf("replayed retry on a paid order: err = %v, want ErrOrderNotEditable", err)
	}
	if got := pay.creates.Load(); got != created+2 {
		t.Errorf("gateway calls = %d, want %d (the paid order's checkout and first retry only)", got, created+2)
	}
}

func studentOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) uuid.UUID {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.StudentID
}

func TestRetryPayment_RecoversGatewayCreationFailureAndKeepsClaims(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	orderID := cartOf(t, svc, student, product)
	pay.failCreate.Store(true)
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err == nil {
		t.Fatal("Checkout with a failing gateway succeeded, want an error")
	}
	pay.failCreate.Store(false)
	if got := gatewayRefOf(t, repo, orderID); got != "" {
		t.Fatalf("gateway ref after failed checkout = %q, want none", got)
	}

	result, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())
	if err != nil {
		t.Fatalf("RetryPayment: %v", err)
	}
	if result.GatewayRef == "" || gatewayRefOf(t, repo, orderID) != result.GatewayRef {
		t.Errorf("order gateway ref = %q, want the retry's %q", gatewayRefOf(t, repo, orderID), result.GatewayRef)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_pending" {
		t.Errorf("status = %q, want payment_pending", got)
	}
	if got := liveClaimsOf(t, repo, student); got != 1 {
		t.Errorf("live claims = %d, want 1", got)
	}
}

func TestRetryPayment_RefusesExamTheStudentWasGrantedMeanwhile(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	product := seedDigitalProduct(t, repo, "exam", 50000, exam)
	orderID := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	registerForExam(t, repo, student, exam, "registered")
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

	wantConflict(t, err, model.DigitalConflictOwned)
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func TestRetryPayment_RefusesLegacyOrderWhoseTargetAnotherOrderHolds(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	holder := seedDigitalProduct(t, repo, "course", 50000, course)
	sameCourse := seedDigitalProduct(t, repo, "course", 40000, course)
	pendingCourseOrderFor(t, svc, student, holder)

	loser := cartOf(t, svc, student)
	forceLine(t, repo, loser, sameCourse, "course", 40000)
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending', total = 40000 WHERE id = $1`, loser,
	); err != nil {
		t.Fatalf("make legacy pending order: %v", err)
	}
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), loser.String(), retryKey())

	wantConflict(t, err, model.DigitalConflictClaimed)
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

// Migration 0068 can claim only one target of a legacy bundle and record the
// other as a backfill conflict; the order stays unsnapshotted, so a retry must
// still check the target it lost, not just the one it owns.
func TestRetryPayment_LegacyBundleWithOnlyOneTargetClaimedStillChecksTheLostOne(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	kept, lost := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	bundle := seedDigitalProduct(t, repo, "course", 90000, kept, lost)
	single := seedDigitalProduct(t, repo, "course", 40000, lost)
	legacy := pendingCourseOrderFor(t, svc, student, bundle)

	var winner, winnerItem uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, checked_out_at)
		 VALUES ($1, 'paid', 40000, 0, 0, 40000, now()) RETURNING id`, student,
	).Scan(&winner); err != nil {
		t.Fatalf("seed winning order: %v", err)
	}
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
		 VALUES ($1, $2, 'course', 'x', 40000, 1, 40000) RETURNING id`, winner, single,
	).Scan(&winnerItem); err != nil {
		t.Fatalf("seed winning item: %v", err)
	}
	var legacyItem uuid.UUID
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM order_item WHERE order_id = $1`, legacy).Scan(&legacyItem); err != nil {
		t.Fatalf("read legacy item: %v", err)
	}
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM order_digital_target WHERE order_id = $1 AND target_id = $2`, []any{legacy, lost}},
		{`UPDATE orders SET digital_targets_snapshot_at = NULL WHERE id = $1`, []any{legacy}},
		{`INSERT INTO order_digital_target (order_id, order_item_id, student_id, target_kind, target_id)
		  VALUES ($1, $2, $3, 'course', $4)`, []any{winner, winnerItem, student, lost}},
		{`INSERT INTO order_digital_target_backfill_conflict (order_id, order_item_id, student_id, target_kind, target_id, winning_order_id)
		  VALUES ($1, $2, $3, 'course', $4, $5)`, []any{legacy, legacyItem, student, lost, winner}},
	} {
		if _, err := repo.Pool().Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("simulate partial backfill: %v", err)
		}
	}
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), legacy.String(), retryKey())

	conflict := wantConflict(t, err, model.DigitalConflictClaimed)
	for _, c := range conflict.Conflicts {
		if c.TargetID == kept {
			t.Errorf("conflicts = %+v: the order's own claim on %v is its reservation, not a conflict", conflict.Conflicts, kept)
		}
	}
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func unsnapshotLegacyOrder(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM order_digital_target WHERE order_id = $1`, orderID); err != nil {
		t.Fatalf("drop claims: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET digital_targets_snapshot_at = NULL WHERE id = $1`, orderID); err != nil {
		t.Fatalf("drop snapshot: %v", err)
	}
}

func TestRetryPayment_LegacyOrderWithALineThatNoLongerLinksAnythingIsRefused(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	courseA, courseB := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	productA := seedDigitalProduct(t, repo, "course", 50000, courseA)
	productB := seedDigitalProduct(t, repo, "course", 40000, courseB)
	legacy := pendingCourseOrderFor(t, svc, student, productA, productB)
	unsnapshotLegacyOrder(t, repo, legacy)
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, productB); err != nil {
		t.Fatalf("unlink product: %v", err)
	}
	before := pay.creates.Load()

	_, err := svc.RetryPayment(ctx, student.String(), legacy.String(), retryKey())

	if !errors.Is(err, ErrDigitalProductNoTargets) {
		t.Fatalf("err = %v, want ErrDigitalProductNoTargets", err)
	}
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func TestRetryPayment_LegacyOrderWhoseLinesAllResolveCanStillRetry(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	productA := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
	productB := seedDigitalProduct(t, repo, "course", 40000, seedCourseTarget(t, repo))
	legacy := pendingCourseOrderFor(t, svc, student, productA, productB)
	unsnapshotLegacyOrder(t, repo, legacy)
	before := pay.creates.Load()

	if _, err := svc.RetryPayment(ctx, student.String(), legacy.String(), retryKey()); err != nil {
		t.Fatalf("RetryPayment: %v", err)
	}
	if got := pay.creates.Load() - before; got != 1 {
		t.Errorf("gateway calls = %d, want 1", got)
	}
}

func pendingCourseOrderFor(t *testing.T, svc *Service, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := cartOf(t, svc, student, products...)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID
}

func TestRetryPayment_OrderExpiringDuringGatewayCallKeepsItsOldPaymentState(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	originalRef := gatewayRefOf(t, repo, orderID)
	pay.onCreate = func() {
		pay.onCreate = nil
		expireOrderNow(t, repo, orderID)
	}

	_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

	if err == nil {
		t.Fatal("retry succeeded for an order that expired while the gateway was being called")
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired (never resurrected)", got)
	}
	if got := gatewayRefOf(t, repo, orderID); got != originalRef {
		t.Errorf("gateway ref = %q, want unchanged %q", got, originalRef)
	}
	if got := liveClaimsOf(t, repo, student); got != 0 {
		t.Errorf("live claims = %d, want 0", got)
	}
}

func TestRetryPayment_FreshCheckoutAfterExpiryStartsFromANewCart(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, product, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, orderID)

	fresh := cartOf(t, svc, student, product)
	if fresh == orderID {
		t.Fatal("fresh cart reused the expired order")
	}
	if _, err := svc.Checkout(ctx, student.String(), fresh.String(), checkoutKey()); err != nil {
		t.Fatalf("fresh checkout: %v", err)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("old order status = %q, want payment_expired", got)
	}
}
