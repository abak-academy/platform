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
// order: past the deadline it leaves one flag that names the gateway's
// transaction id, inside the window it fulfils exactly once.
func TestReconcileConcurrentWithWebhook(t *testing.T) {
	tests := []struct {
		name             string
		withBook         bool
		pastDeadline     bool
		wantReconcileErr error
		wantStatus       string
		wantEvents       int
		wantRefs         int
	}{
		{"past deadline flags the settlement once", true, true, service.ErrOrderNotPayable, "payment_expired", 0, 1},
		{"inside window fulfils once", false, false, nil, "paid", 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newLifecycleEnv(t)
			ctx := context.Background()
			student := env.student(t)
			actor := env.admin(t)
			env.pay.paid.Store(true)

			for i := 0; i < 8; i++ {
				products := []uuid.UUID{env.courseProduct(t, 50000, env.course(t))}
				var book uuid.UUID
				if tt.withBook {
					book = env.book(t, 5)
					products = append(products, book)
				}
				orderID := env.pendingOrder(t, student, products...)
				if tt.pastDeadline {
					env.expireDeadline(t, orderID)
				}

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
				if !errors.Is(reconcileErr, tt.wantReconcileErr) {
					t.Errorf("iteration %d: reconcile = %v, want %v", i, reconcileErr, tt.wantReconcileErr)
				}
				if status := env.status(t, orderID); status != tt.wantStatus {
					t.Errorf("iteration %d: status = %q, want %q", i, status, tt.wantStatus)
				}
				if tt.withBook {
					if got := env.stock(t, book); got != 5 {
						t.Errorf("iteration %d: book stock = %d, want 5 (restored once)", i, got)
					}
				}
				if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID); got != tt.wantEvents {
					t.Errorf("iteration %d: OrderPaid events = %d, want %d", i, got, tt.wantEvents)
				}
				refs := env.lateFlagReferences(t, orderID)
				if len(refs) != tt.wantRefs {
					t.Errorf("iteration %d: late flags = %v, want %d", i, refs, tt.wantRefs)
				}
				for _, ref := range refs {
					if want := "tx-" + orderID.String(); ref != want {
						t.Errorf("iteration %d: late flag reference = %q, want %q", i, ref, want)
					}
				}
			}
		})
	}
}
