package worker

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/service"
)

func (e *lifecycleEnv) lateFlagReferences(t *testing.T, orderID uuid.UUID) []string {
	t.Helper()
	rows, err := e.repo.Pool().Query(context.Background(),
		`SELECT payload->>'reference' FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String())
	if err != nil {
		t.Fatalf("read late flags: %v", err)
	}
	defer rows.Close()
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			t.Fatalf("scan: %v", err)
		}
		refs = append(refs, ref)
	}
	return refs
}

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

		var webhookErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			webhookErr = env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString())
		}()
		go func() {
			defer wg.Done()
			env.w.sweepStalePayments(ctx)
		}()
		wg.Wait()

		if webhookErr != nil {
			t.Errorf("iteration %d: webhook = %v, want it acknowledged", i, webhookErr)
		}
		if status := env.status(t, orderID); status != "payment_expired" {
			t.Errorf("iteration %d: status = %q, want payment_expired", i, status)
		}
		if got := env.stock(t, book); got != 5 {
			t.Errorf("iteration %d: book stock = %d, want 5 (restored once)", i, got)
		}
		if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID); got != 0 {
			t.Errorf("iteration %d: OrderPaid events = %d, want 0", i, got)
		}
		if got := env.count(t, `SELECT count(*) FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String()); got != 1 {
			t.Errorf("iteration %d: late flags = %d, want exactly 1", i, got)
		}
	}
}

// One settlement reported by both the webhook and an admin reconcile, in either
// order, leaves one flag that names the gateway's transaction id.
func TestReconcileConcurrentWithWebhookPastDeadlineFlagsTheSettlementOnce(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	actor := env.admin(t)
	env.pay.paid.Store(true)

	for i := 0; i < 8; i++ {
		book := env.book(t, 5)
		digital := env.courseProduct(t, 50000, env.course(t))
		orderID := env.pendingOrder(t, student, book, digital)
		env.expireDeadline(t, orderID)

		var webhookErr, reconcileErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			webhookErr = env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString())
		}()
		go func() {
			defer wg.Done()
			reconcileErr = env.svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uuid.NewString())
		}()
		wg.Wait()

		if webhookErr != nil {
			t.Errorf("iteration %d: webhook = %v, want it acknowledged", i, webhookErr)
		}
		if !errors.Is(reconcileErr, service.ErrOrderNotPayable) {
			t.Errorf("iteration %d: reconcile = %v, want ErrOrderNotPayable", i, reconcileErr)
		}
		if status := env.status(t, orderID); status != "payment_expired" {
			t.Errorf("iteration %d: status = %q, want payment_expired", i, status)
		}
		if got := env.stock(t, book); got != 5 {
			t.Errorf("iteration %d: book stock = %d, want 5 (restored once)", i, got)
		}
		if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID); got != 0 {
			t.Errorf("iteration %d: OrderPaid events = %d, want 0", i, got)
		}
		want := "tx-" + orderID.String()
		if refs := env.lateFlagReferences(t, orderID); len(refs) != 1 || refs[0] != want {
			t.Errorf("iteration %d: late flag references = %v, want exactly [%s]", i, refs, want)
		}
	}
}

func TestReconcileConcurrentWithWebhookInsideWindowFulfilsOnce(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	actor := env.admin(t)
	env.pay.paid.Store(true)

	for i := 0; i < 8; i++ {
		orderID := env.pendingOrder(t, student, env.courseProduct(t, 50000, env.course(t)))

		var webhookErr, reconcileErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			webhookErr = env.svc.HandlePaymentWebhook(ctx, env.settlementPayload(orderID), "sig", uuid.NewString())
		}()
		go func() {
			defer wg.Done()
			reconcileErr = env.svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uuid.NewString())
		}()
		wg.Wait()

		if webhookErr != nil || reconcileErr != nil {
			t.Errorf("iteration %d: webhook = %v, reconcile = %v, want both nil", i, webhookErr, reconcileErr)
		}
		if status := env.status(t, orderID); status != "paid" {
			t.Errorf("iteration %d: status = %q, want paid", i, status)
		}
		if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID); got != 1 {
			t.Errorf("iteration %d: OrderPaid events = %d, want 1", i, got)
		}
		if refs := env.lateFlagReferences(t, orderID); len(refs) != 0 {
			t.Errorf("iteration %d: late flags = %v, want none", i, refs)
		}
	}
}
