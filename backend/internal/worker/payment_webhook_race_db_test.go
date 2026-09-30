package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Past the deadline there is no grace: whichever of webhook and sweeper takes
// the order lock first expires it exactly once, and the settlement is flagged,
// never fulfilled.
func TestSweepStalePayments_ConcurrentWebhookPastDeadlineAlwaysExpiresOnce(t *testing.T) {
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

		if status := env.status(t, orderID); status != "payment_expired" {
			t.Errorf("iteration %d: status = %q, want payment_expired", i, status)
		}
		if got := env.stock(t, book); got != 5 {
			t.Errorf("iteration %d: book stock = %d, want 5 (restored once)", i, got)
		}
		if got := env.liveClaims(t, orderID); got != 0 {
			t.Errorf("iteration %d: live claims = %d, want 0", i, got)
		}
		if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID); got != 0 {
			t.Errorf("iteration %d: OrderPaid events = %d, want 0", i, got)
		}
		if got := env.count(t, `SELECT count(*) FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String()); got > 1 {
			t.Errorf("iteration %d: late flags = %d, want at most 1", i, got)
		}
	}
}
