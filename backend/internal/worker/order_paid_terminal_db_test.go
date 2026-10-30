package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

func (e *lifecycleEnv) paidOrder(t *testing.T, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := e.pendingOrder(t, student, products...)
	if err := e.svc.HandlePaymentWebhook(context.Background(), e.settlementPayload(orderID), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	return orderID
}

func (e *lifecycleEnv) refundErr(orderID uuid.UUID) error {
	var actor uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Refund Actor', 'super_admin', 'active', $1, '') RETURNING id`, "refund_"+uuid.NewString()[:10],
	).Scan(&actor); err != nil {
		return fmt.Errorf("insert actor: %w", err)
	}
	if err := e.svc.AdminRefundOrder(context.Background(), actor.String(), orderID.String(), "refund_proof/"+uuid.NewString()+".png"); err != nil {
		return fmt.Errorf("refund: %w", err)
	}
	return nil
}

func (e *lifecycleEnv) refund(t *testing.T, orderID uuid.UUID) {
	t.Helper()
	if err := e.refundErr(orderID); err != nil {
		t.Fatal(err)
	}
}

// pollAllErr drains the outbox and reports failures instead of touching
// *testing.T, so a goroutine other than the test's can run it. The database is
// shared across tests, so events queued by earlier tests come first and one
// poll claims only ten.
func (e *lifecycleEnv) pollAllErr(ctx context.Context) error {
	for i := 0; i < 50; i++ {
		var n int
		if err := e.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&n); err != nil {
			return fmt.Errorf("count pending outbox: %w", err)
		}
		if n == 0 {
			return nil
		}
		e.w.pollOutbox(ctx)
	}
	return fmt.Errorf("outbox did not drain after 50 polls")
}

func (e *lifecycleEnv) pollAll(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := e.pollAllErr(ctx); err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
}

// waitForWaitersBehind polls until a backend is blocked, directly or through
// another waiter, by holder's transaction, so the test knows the worker reached
// the order row lock from observed database state instead of a guessed delay.
func waitForWaitersBehind(t *testing.T, e *lifecycleEnv, holder pgx.Tx, want int) {
	t.Helper()
	ctx := context.Background()
	var holderPID int
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatalf("read holder pid: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		n := e.count(t,
			`WITH RECURSIVE blocked AS (
			     SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))
			     UNION
			     SELECT a.pid FROM pg_stat_activity a JOIN blocked b ON b.pid = ANY(pg_blocking_pids(a.pid))
			 ) SELECT count(*) FROM blocked`, holderPID)
		if n >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiters behind the refund = %d, want %d: the worker never blocked on the order row", n, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (e *lifecycleEnv) unprocessedEvents(t *testing.T, orderID uuid.UUID) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND processed_at IS NULL`, orderID)
}

func (e *lifecycleEnv) enrollmentCourse(t *testing.T, orderID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`SELECT pc.course_id FROM order_item oi JOIN product_course pc ON pc.product_id = oi.product_id
		 WHERE oi.order_id = $1 AND oi.product_type = 'course'`, orderID,
	).Scan(&id); err != nil {
		t.Fatalf("order course: %v", err)
	}
	return id
}

func TestOrderPaid_RefundWinsAndGrantsNoAccess(t *testing.T) {
	tests := []struct {
		name       string
		iterations int
		cancel     func(t *testing.T, env *lifecycleEnv, student uuid.UUID) (orderID, course uuid.UUID)
	}{
		{
			name:       "refunded before the event is polled",
			iterations: 1,
			cancel: func(t *testing.T, env *lifecycleEnv, student uuid.UUID) (uuid.UUID, uuid.UUID) {
				book := env.book(t, 5)
				digital := env.courseProduct(t, 50000, env.course(t))

				orderID := env.paidOrder(t, student, book, digital)
				course := env.enrollmentCourse(t, orderID)
				env.refund(t, orderID)

				env.pollAll(t, context.Background())
				return orderID, course
			},
		},
		{
			// The refund holds the order row while the worker picks up the event: the
			// worker must wait for it and then see the cancelled order, not fulfil on the
			// paid status it read earlier.
			name:       "worker waits for a refund holding the order row",
			iterations: 1,
			cancel: func(t *testing.T, env *lifecycleEnv, student uuid.UUID) (uuid.UUID, uuid.UUID) {
				ctx := context.Background()
				digital := env.courseProduct(t, 50000, env.course(t))

				orderID := env.paidOrder(t, student, digital)
				course := env.enrollmentCourse(t, orderID)

				refundTx, err := env.repo.BeginTx(ctx)
				if err != nil {
					t.Fatalf("begin: %v", err)
				}
				defer refundTx.Rollback(ctx)
				if _, err := refundTx.Exec(ctx, `SELECT 1 FROM orders WHERE id = $1 FOR UPDATE`, orderID); err != nil {
					t.Fatalf("lock order: %v", err)
				}

				pollDone := make(chan error, 1)
				go func() {
					pollDone <- env.pollAllErr(ctx)
				}()
				waitForWaitersBehind(t, env, refundTx, 1)

				if _, err := refundTx.Exec(ctx, `UPDATE orders SET status = 'cancelled', cancellation_reason = 'refunded' WHERE id = $1`, orderID); err != nil {
					t.Fatalf("cancel: %v", err)
				}
				if err := refundTx.Commit(ctx); err != nil {
					t.Fatalf("commit: %v", err)
				}
				if err := <-pollDone; err != nil {
					t.Fatalf("poll outbox: %v", err)
				}
				return orderID, course
			},
		},
		{
			name:       "poll and refund racing",
			iterations: 6,
			cancel: func(t *testing.T, env *lifecycleEnv, student uuid.UUID) (uuid.UUID, uuid.UUID) {
				ctx := context.Background()
				digital := env.courseProduct(t, 50000, env.course(t))
				orderID := env.paidOrder(t, student, digital)
				course := env.enrollmentCourse(t, orderID)

				var wg sync.WaitGroup
				refundDone := make(chan error, 1)
				pollDone := make(chan error, 1)
				wg.Add(2)
				go func() {
					defer wg.Done()
					pollDone <- env.pollAllErr(ctx)
				}()
				go func() {
					defer wg.Done()
					refundDone <- env.refundErr(orderID)
				}()
				wg.Wait()
				if err := <-pollDone; err != nil {
					t.Fatalf("poll outbox: %v", err)
				}
				if err := <-refundDone; err != nil {
					t.Fatal(err)
				}
				return orderID, course
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newLifecycleEnv(t)
			student := env.student(t)

			for i := 0; i < tt.iterations; i++ {
				orderID, course := tt.cancel(t, env, student)

				if got := env.activeEnrollments(t, student, course); got != 0 {
					t.Errorf("iteration %d: active enrollments = %d, want 0 for a refunded order", i, got)
				}
				if got := env.status(t, orderID); got != "cancelled" {
					t.Errorf("iteration %d: status = %q, want cancelled (not processing or completed)", i, got)
				}
				if got := env.unprocessedEvents(t, orderID); got != 0 {
					t.Errorf("iteration %d: unprocessed outbox events = %d, want 0 (the stale event is consumed, not retried forever)", i, got)
				}
			}
		})
	}
}

// The sweeper can overwrite a status a webhook just committed as paid; the money
// is in, so the event must still grant access, and the status only moves forward.
func TestOrderPaid_GrantsCourseAndOnlyMovesStatusForward(t *testing.T) {
	tests := []struct {
		name       string
		withBook   bool
		start      string
		wantStatus string
	}{
		{"paid digital only ends completed", false, "paid", "completed"},
		{"paid mixed with a book ends processing", true, "paid", "processing"},
		{"expired digital only ends completed", false, "expired", "completed"},
		{"expired mixed with a book ends processing", true, "expired", "processing"},
		{"shipped mixed with a book stays shipped", true, "shipped", "shipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newLifecycleEnv(t)
			ctx := context.Background()
			student := env.student(t)
			digital := env.courseProduct(t, 50000, env.course(t))
			products := []uuid.UUID{digital}
			items := `{"product_id":"` + digital.String() + `","product_type":"course","qty":1}`
			if tt.withBook {
				book := env.book(t, 5)
				products = []uuid.UUID{book, digital}
				items = `{"product_id":"` + book.String() + `","product_type":"book","qty":1},` + items
			}

			var orderID uuid.UUID
			if tt.start == "expired" {
				orderID = env.pendingOrder(t, student, products...)
				env.expireDeadline(t, orderID)
				env.w.sweepStalePayments(ctx)
				if got := env.status(t, orderID); got != "payment_expired" {
					t.Fatalf("status = %q, want payment_expired before the event runs", got)
				}
				env.exec(t,
					`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) VALUES ('order', $1, 'OrderPaid', $2::jsonb)`,
					orderID, `{"order_id":"`+orderID.String()+`","items":[`+items+`]}`,
				)
			} else {
				orderID = env.paidOrder(t, student, products...)
			}
			if tt.start == "shipped" {
				if err := env.repo.SetShipped(ctx, orderID, "RESI-"+uuid.NewString()[:8]); err != nil {
					t.Fatalf("SetShipped: %v", err)
				}
			}
			course := env.enrollmentCourse(t, orderID)

			env.pollAll(t, ctx)

			if got := env.activeEnrollments(t, student, course); got != 1 {
				t.Errorf("active enrollments = %d, want 1: a settled payment must grant access whatever state the order is in", got)
			}
			if got := env.status(t, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %s: the event must not leave an order payment_expired or walk shipped back", got, tt.wantStatus)
			}
			if got := env.unprocessedEvents(t, orderID); got != 0 {
				t.Errorf("unprocessed outbox events = %d, want 0", got)
			}
		})
	}
}

func TestOrderPaid_ReplayedEventDoesNotRefulfilOrMoveStatus(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.paidOrder(t, student, book, digital)
	course := env.enrollmentCourse(t, orderID)
	env.pollAll(t, ctx)

	env.exec(t,
		`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) VALUES ('order', $1, 'OrderPaid', $2::jsonb)`,
		orderID, `{"order_id":"`+orderID.String()+`","items":[{"product_id":"`+digital.String()+`","product_type":"course","qty":1}]}`,
	)
	env.pollAll(t, ctx)
	if got := env.status(t, orderID); got != "processing" {
		t.Errorf("status after replay = %q, want processing", got)
	}
	if got := env.count(t, `SELECT count(*) FROM course_session WHERE student_id = $1 AND course_id = $2`, student, course); got != 1 {
		t.Errorf("course sessions after replay = %d, want 1", got)
	}
}

func TestOrderPaid_FulfilledTargetIsOwnedUntilRefundedThenCanBeRepurchased(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	course := env.course(t)
	digital := env.courseProduct(t, 50000, course)
	other := env.courseProduct(t, 40000, course)

	orderID := env.paidOrder(t, student, digital)
	env.pollAll(t, ctx)

	cart, _, err := env.svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if got := env.status(t, orderID); got != "completed" {
		t.Fatalf("status = %q, want completed before the repurchase attempt", got)
	}
	err = env.svc.AddItem(ctx, student.String(), cart.ID.String(), other.String(), 1)
	var conflict *service.DigitalTargetConflictError
	if !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Reason != model.DigitalConflictOwned {
		t.Fatalf("AddItem while the access is live: err = %v, want one %q conflict", err, model.DigitalConflictOwned)
	}

	env.refund(t, orderID)
	if err := env.svc.AddItem(ctx, student.String(), cart.ID.String(), other.String(), 1); err != nil {
		t.Fatalf("AddItem after refund: %v", err)
	}
	if _, err := env.svc.Checkout(ctx, student.String(), cart.ID.String(), "repurchase-"+uuid.NewString(), ""); err != nil {
		t.Fatalf("Checkout after refund: %v", err)
	}
	if got := env.status(t, cart.ID); got != "payment_pending" {
		t.Errorf("repurchase status = %q, want payment_pending", got)
	}
	if got := env.status(t, orderID); got != "cancelled" {
		t.Errorf("refunded order status = %q, want cancelled", got)
	}
}
