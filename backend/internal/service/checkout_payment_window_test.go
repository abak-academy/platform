package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/repository"
)

func deadlineOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) *time.Time {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.PaymentExpiresAt
}

func setDeadline(t *testing.T, repo *repository.Repository, orderID uuid.UUID, in string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE orders SET payment_expires_at = now() + $2::interval WHERE id = $1`, orderID, in,
	); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
}

func sweepOrder(t *testing.T, repo *repository.Repository, orderID uuid.UUID) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
	if err != nil {
		t.Fatalf("ExpirePaymentOrderTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return expired
}

func TestCheckout_GivesThePendingOrderABoundedDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil {
		t.Fatal("payment_expires_at is NULL after checkout")
	}
	if left := time.Until(*deadline); left < 23*time.Hour || left > 24*time.Hour+time.Minute {
		t.Errorf("time left = %v, want about 24h", left)
	}
	if asked := time.Duration(pay.lastExpiresIn.Load()); asked > 24*time.Hour || asked < 23*time.Hour {
		t.Errorf("gateway asked to keep the link for %v, want no more than the order window", asked)
	}
}

func TestCheckout_GatewayFailureStillExpiresAndFreesStock(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, book := mixedCart(t, svc, repo, student)
	pay.failCreate.Store(true)

	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err == nil {
		t.Fatal("Checkout with a failing gateway succeeded")
	}
	if deadlineOf(t, repo, orderID) == nil {
		t.Fatal("payment_expires_at is NULL after a gateway failure: the sweeper can never free this order")
	}
	if got := stockOf(t, repo, book); got != 3 {
		t.Fatalf("stock after checkout = %d, want 3", got)
	}

	elapseDeadline(t, repo, orderID)
	if !sweepOrder(t, repo, orderID) {
		t.Fatal("sweeper transition did not expire the order")
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5", got)
	}
}

func TestRetryPayment_NeverExtendsTheOrderDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	setDeadline(t, repo, orderID, "2 hours")
	original := *deadlineOf(t, repo, orderID)
	pay.expiresAfter = 72 * time.Hour

	for i := 0; i < 3; i++ {
		result, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())
		if err != nil {
			t.Fatalf("retry #%d: %v", i, err)
		}
		if got := *deadlineOf(t, repo, orderID); !got.Equal(original) {
			t.Errorf("retry #%d moved the deadline from %v to %v", i, original, got)
		}
		if result.PaymentExpiresAt.After(original) {
			t.Errorf("retry #%d advertises %v, past the order deadline %v", i, result.PaymentExpiresAt, original)
		}
		if asked := time.Duration(pay.lastExpiresIn.Load()); asked > 2*time.Hour {
			t.Errorf("retry #%d asked the gateway for %v, more than the 2h left", i, asked)
		}
	}
}

func TestRetryPayment_RefusedWithLessThanAMinuteLeft(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	setDeadline(t, repo, orderID, "20 seconds")
	before := pay.creates.Load()

	_, err := svc.RetryPayment(context.Background(), student.String(), orderID.String(), retryKey())

	if !errors.Is(err, ErrPaymentDeadlineTooClose) {
		t.Fatalf("err = %v, want ErrPaymentDeadlineTooClose", err)
	}
	if got := pay.creates.Load() - before; got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func TestRetryPayment_OrderWithoutDeadlineGetsOneOnceAndKeepsIt(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET payment_expires_at = NULL, gateway_ref = NULL WHERE id = $1`, orderID); err != nil {
		t.Fatalf("simulate a pre-deadline order: %v", err)
	}

	if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey()); err != nil {
		t.Fatalf("first retry: %v", err)
	}
	first := deadlineOf(t, repo, orderID)
	if first == nil {
		t.Fatal("retry left the order without a deadline")
	}
	if left := time.Until(*first); left < 23*time.Hour || left > 24*time.Hour+time.Minute {
		t.Errorf("time left = %v, want about 24h", left)
	}
	if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey()); err != nil {
		t.Fatalf("second retry: %v", err)
	}
	if got := *deadlineOf(t, repo, orderID); !got.Equal(*first) {
		t.Errorf("second retry moved the deadline from %v to %v", *first, got)
	}
}

func TestCheckout_OrderThatExpiresDuringTheGatewayCallGetsNoPaymentLink(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, book := mixedCart(t, svc, repo, student)
	pay.onCreate = func() {
		pay.onCreate = nil
		elapseDeadline(t, repo, orderID)
		sweepOrder(t, repo, orderID)
	}

	result, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey())

	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("err = %v, want ErrPaymentExpired", err)
	}
	if result.GatewayRef != "" || result.PaymentURL != "" || result.SnapToken != "" {
		t.Errorf("result = %+v, want no payment link for an expired order", result)
	}
	if got := gatewayRefOf(t, repo, orderID); got != "" {
		t.Errorf("gateway ref = %q, want none attached to an expired order", got)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5", got)
	}
}

func TestCheckout_AdvertisedExpiryNeverExceedsTheOrderDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	pay.expiresAfter = 72 * time.Hour
	course := seedCourseTarget(t, repo)
	orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, course))

	result, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey())
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil || result.PaymentExpiresAt.After(*deadline) {
		t.Errorf("advertised expiry %v, order deadline %v: the link must not outlive the order", result.PaymentExpiresAt, deadline)
	}
}

func TestCheckout_CachedReplayNeverOffersAnExpiredOrAnotherStudentsOrder(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, course))
	key := checkoutKey()

	first, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	replay, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
	if err != nil || replay.GatewayRef != first.GatewayRef {
		t.Fatalf("replay on an unchanged order = %+v, %v; want the same ref", replay, err)
	}

	if _, err := svc.Checkout(ctx, newDigitalStudent(t, repo).String(), orderID.String(), key); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("another student replaying the key: err = %v, want ErrOrderNotFound", err)
	}

	expireOrderNow(t, repo, orderID)
	result, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("replay on an expired order: err = %v, want ErrPaymentExpired", err)
	}
	if result.GatewayRef != "" {
		t.Errorf("replay returned ref %q for an expired order", result.GatewayRef)
	}
}

func TestCheckout_OneKeyUsedOnTwoOrdersNeverHandsOutTheOtherOrdersReference(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderA := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
	key := checkoutKey()
	first, err := svc.Checkout(ctx, student.String(), orderA.String(), key)
	if err != nil {
		t.Fatalf("Checkout A: %v", err)
	}
	orderB, _, _ := pendingCourseOrder(t, svc, repo, student)

	got, err := svc.Checkout(ctx, student.String(), orderB.String(), key)

	if got.GatewayRef != "" || got.GatewayRef == first.GatewayRef {
		t.Errorf("order B was handed reference %q, which belongs to order A", got.GatewayRef)
	}
	if !errors.Is(err, ErrOrderNotEditable) {
		t.Errorf("err = %v, want ErrOrderNotEditable: order B is already checked out", err)
	}
}

func TestCheckout_OneKeyUsedByTwoStudentsNeverHandsOutTheOtherStudentsReference(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	studentA := newDigitalStudent(t, repo)
	orderA := cartOf(t, svc, studentA, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
	studentB := newDigitalStudent(t, repo)
	orderB, _, _ := pendingCourseOrder(t, svc, repo, studentB)
	key := checkoutKey()
	first, err := svc.Checkout(ctx, studentA.String(), orderA.String(), key)
	if err != nil {
		t.Fatalf("Checkout A: %v", err)
	}

	got, err := svc.Checkout(ctx, studentB.String(), orderB.String(), key)

	if got.GatewayRef != "" || got.GatewayRef == first.GatewayRef {
		t.Errorf("student B was handed reference %q, which belongs to student A's order", got.GatewayRef)
	}
	if !errors.Is(err, ErrOrderNotEditable) {
		t.Errorf("err = %v, want ErrOrderNotEditable: order B is already checked out", err)
	}
}

func TestCheckout_FreeOrderReplayStillReturnsFreeWithoutGateway(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 0, course))
	key := checkoutKey()

	for i := 0; i < 2; i++ {
		result, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
		if err != nil || !result.Free {
			t.Fatalf("free checkout #%d = %+v, %v; want Free", i, result, err)
		}
	}
	if got := pay.creates.Load(); got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func askedDeadline(t *testing.T, pay *lifecyclePayments) time.Time {
	t.Helper()
	got, _ := pay.lastExpiresAt.Load().(time.Time)
	return got
}

// The gateway session is pinned to the order's own database deadline, not to a
// duration counted from whenever the customer later charges.
func TestCheckout_AsksTheGatewayToEndAtTheOrderDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil {
		t.Fatal("payment_expires_at is NULL after checkout")
	}
	if got := askedDeadline(t, pay); !got.Equal(*deadline) {
		t.Errorf("gateway asked to end at %v, order deadline is %v", got, *deadline)
	}
}

func TestRetryPayment_AsksTheGatewayToEndAtTheOriginalDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	setDeadline(t, repo, orderID, "2 hours 30 seconds")
	original := *deadlineOf(t, repo, orderID)

	for i := 0; i < 2; i++ {
		if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey()); err != nil {
			t.Fatalf("retry #%d: %v", i, err)
		}
		if got := askedDeadline(t, pay); !got.Equal(original) {
			t.Errorf("retry #%d asked the gateway to end at %v, original deadline is %v", i, got, original)
		}
	}
}
