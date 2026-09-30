package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestSweepStalePayments_ExpiredMixedOrderReleasesClaimsAndRestoresPhysicalStockOnce(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	course := env.course(t)
	digital := env.courseProduct(t, 50000, course)
	book := env.book(t, 5)

	orderID := env.pendingOrder(t, student, book, digital)
	if got := env.stock(t, book); got != 3 {
		t.Fatalf("book stock after checkout = %d, want 3", got)
	}
	if got := env.liveClaims(t, orderID); got != 1 {
		t.Fatalf("live claims after checkout = %d, want 1", got)
	}
	env.expireDeadline(t, orderID)

	env.w.sweepStalePayments(ctx)
	env.w.sweepStalePayments(ctx)

	if got := env.status(t, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := env.liveClaims(t, orderID); got != 0 {
		t.Errorf("live claims = %d, want 0 after expiry", got)
	}
	if got := env.stock(t, book); got != 5 {
		t.Errorf("book stock = %d, want 5 (restored exactly once)", got)
	}
	if got := env.stock(t, digital); got != 0 {
		t.Errorf("digital product stock = %d, want 0 (digital lines never had stock taken)", got)
	}
}

func TestSweepStalePayments_ExpiredOrderFreesTargetForFreshCheckout(t *testing.T) {
	env := newLifecycleEnv(t)
	student := env.student(t)
	course := env.course(t)
	digital := env.courseProduct(t, 50000, course)

	orderID := env.pendingOrder(t, student, digital)
	env.expireDeadline(t, orderID)
	env.w.sweepStalePayments(context.Background())

	fresh := env.pendingOrder(t, student, digital)
	if fresh == orderID {
		t.Fatal("fresh checkout reused the expired order")
	}
	if got := env.status(t, orderID); got != "payment_expired" {
		t.Errorf("old order status = %q, want payment_expired (final)", got)
	}
	if got := env.liveClaims(t, fresh); got != 1 {
		t.Errorf("fresh order live claims = %d, want 1", got)
	}
}

func TestSweepStalePayments_PhysicalOnlyOrderStillRestoresStock(t *testing.T) {
	env := newLifecycleEnv(t)
	student := env.student(t)
	book := env.book(t, 4)

	orderID := env.pendingOrder(t, student, book)
	env.expireDeadline(t, orderID)
	env.w.sweepStalePayments(context.Background())

	if got := env.status(t, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := env.stock(t, book); got != 4 {
		t.Errorf("book stock = %d, want 4", got)
	}
}

func TestSweepStalePayments_ConcurrentSweepsRestoreStockOnce(t *testing.T) {
	env := newLifecycleEnv(t)
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.pendingOrder(t, student, book, digital)
	env.expireDeadline(t, orderID)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.w.sweepStalePayments(context.Background())
		}()
	}
	wg.Wait()

	if got := env.stock(t, book); got != 5 {
		t.Errorf("book stock = %d, want 5 (restored once)", got)
	}
	if got := env.stock(t, digital); got != 0 {
		t.Errorf("digital stock = %d, want 0", got)
	}
	if got := env.liveClaims(t, orderID); got != 0 {
		t.Errorf("live claims = %d, want 0", got)
	}
}

func TestSweepStalePayments_LeavesPaidOrderAndItsClaimAlone(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.pendingOrder(t, student, digital)
	if err := env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	env.expireDeadline(t, orderID)
	env.w.sweepStalePayments(ctx)

	if got := env.status(t, orderID); got != "paid" {
		t.Errorf("status = %q, want paid", got)
	}
	if got := env.liveClaims(t, orderID); got != 1 {
		t.Errorf("live claims = %d, want 1 (paid claim kept)", got)
	}
}
