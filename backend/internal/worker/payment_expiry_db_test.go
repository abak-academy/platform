package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestSweepStalePayments_OutcomeByOrder(t *testing.T) {
	tests := []struct {
		name          string
		bookStock     int
		withDigital   bool
		paidFirst     bool
		sweeps        int
		wantStatus    string
		wantBookStock int
	}{
		{"mixed pending order expires and restores book stock once", 5, true, false, 2, "payment_expired", 5},
		{"physical-only pending order expires and restores stock", 4, false, false, 1, "payment_expired", 4},
		{"paid order is left alone and keeps its stock taken", 5, true, true, 1, "paid", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newLifecycleEnv(t)
			ctx := context.Background()
			student := env.student(t)
			book := env.book(t, tt.bookStock)
			products := []uuid.UUID{book}
			var digital uuid.UUID
			if tt.withDigital {
				digital = env.courseProduct(t, 50000, env.course(t))
				products = append(products, digital)
			}

			orderID := env.pendingOrder(t, student, products...)
			if got := env.stock(t, book); got != tt.bookStock-2 {
				t.Fatalf("book stock after checkout = %d, want %d", got, tt.bookStock-2)
			}
			if tt.paidFirst {
				if err := env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString()); err != nil {
					t.Fatalf("webhook: %v", err)
				}
			}
			env.expireDeadline(t, orderID)

			for i := 0; i < tt.sweeps; i++ {
				env.w.sweepStalePayments(ctx)
			}

			if got := env.status(t, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
			if got := env.stock(t, book); got != tt.wantBookStock {
				t.Errorf("book stock = %d, want %d", got, tt.wantBookStock)
			}
			if tt.withDigital {
				if got := env.stock(t, digital); got != 0 {
					t.Errorf("digital product stock = %d, want 0 (digital lines never had stock taken)", got)
				}
			}
		})
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
}

func TestExpirePaymentOrder_SkipsOrderASettlementAlreadyResolved(t *testing.T) {
	env := newLifecycleEnv(t)
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.paidOrder(t, student, book, digital)
	env.expireDeadline(t, orderID)

	expired, err := env.w.expirePaymentOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("expirePaymentOrder: %v", err)
	}

	if expired {
		t.Error("expirePaymentOrder reported an expiry for an order that was already paid")
	}
	if got := env.status(t, orderID); got != "paid" {
		t.Errorf("status = %q, want paid", got)
	}
	if got := env.stock(t, book); got != 3 {
		t.Errorf("book stock = %d, want 3 (a skipped expiry restores nothing)", got)
	}
}
