package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Whichever of webhook and sweeper takes the order lock first must decide the
// outcome; the loser may never leave a half-applied mix of the two.
func TestSweepStalePayments_ConcurrentWebhookNeverMixesPaidAndExpired(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)

	for i := 0; i < 8; i++ {
		book := env.book(t, 5)
		digital := env.courseProduct(t, 50000, env.course(t))
		orderID := env.pendingOrder(t, student, book, digital)
		env.expireDeadline(t, orderID)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString())
		}()
		go func() {
			defer wg.Done()
			env.w.sweepStalePayments(ctx)
		}()
		wg.Wait()

		outbox := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID)
		switch status := env.status(t, orderID); status {
		case "paid":
			if got := env.stock(t, book); got != 3 {
				t.Errorf("iteration %d: paid order book stock = %d, want 3", i, got)
			}
			if got := env.liveClaims(t, orderID); got != 1 {
				t.Errorf("iteration %d: paid order live claims = %d, want 1", i, got)
			}
			if outbox != 1 {
				t.Errorf("iteration %d: paid order OrderPaid events = %d, want 1", i, outbox)
			}
		case "payment_expired":
			if got := env.stock(t, book); got != 5 {
				t.Errorf("iteration %d: expired order book stock = %d, want 5", i, got)
			}
			if got := env.liveClaims(t, orderID); got != 0 {
				t.Errorf("iteration %d: expired order live claims = %d, want 0", i, got)
			}
			if outbox != 0 {
				t.Errorf("iteration %d: expired order OrderPaid events = %d, want 0", i, outbox)
			}
		default:
			t.Errorf("iteration %d: status = %q, want paid or payment_expired", i, status)
		}
	}
}
