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

func TestOrderPaid_RefundedBeforeFulfilmentGrantsNoAccessAndStaysCancelled(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.paidOrder(t, student, book, digital)
	course := env.enrollmentCourse(t, orderID)
	env.refund(t, orderID)

	env.pollAll(t, ctx)

	if got := env.activeEnrollments(t, student, course); got != 0 {
		t.Errorf("active enrollments = %d, want 0 for a refunded order", got)
	}
	if got := env.status(t, orderID); got != "cancelled" {
		t.Errorf("status = %q, want cancelled (not processing or completed)", got)
	}
	if got := env.unprocessedEvents(t, orderID); got != 0 {
		t.Errorf("unprocessed outbox events = %d, want 0 (the stale event is consumed, not retried forever)", got)
	}
}

func TestOrderPaid_EventForExpiredOrderGrantsNothing(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.pendingOrder(t, student, digital)
	course := env.enrollmentCourse(t, orderID)
	env.expireDeadline(t, orderID)
	env.w.sweepStalePayments(ctx)
	env.exec(t,
		`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) VALUES ('order', $1, 'OrderPaid', $2::jsonb)`,
		orderID, `{"order_id":"`+orderID.String()+`","items":[{"product_id":"`+digital.String()+`","product_type":"course","qty":1}]}`,
	)

	env.pollAll(t, ctx)

	if got := env.activeEnrollments(t, student, course); got != 0 {
		t.Errorf("active enrollments = %d, want 0 for an expired order", got)
	}
	if got := env.status(t, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := env.unprocessedEvents(t, orderID); got != 0 {
		t.Errorf("unprocessed outbox events = %d, want 0", got)
	}
}

// The refund holds the order row while the worker picks up the event: the
// worker must wait for it and then see the cancelled order, not fulfil on the
// paid status it read earlier.
func TestOrderPaid_WaitsForRefundHoldingTheOrderAndGrantsNothing(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
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
	<-pollDone

	if got := env.activeEnrollments(t, student, course); got != 0 {
		t.Errorf("active enrollments = %d, want 0: worker fulfilled an order refunded while it was handling the event", got)
	}
	if got := env.status(t, orderID); got != "cancelled" {
		t.Errorf("status = %q, want cancelled", got)
	}
}

func TestOrderPaid_PaidOrderIsFulfilledOnce(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.paidOrder(t, student, book, digital)
	course := env.enrollmentCourse(t, orderID)
	env.pollAll(t, ctx)

	if got := env.activeEnrollments(t, student, course); got != 1 {
		t.Errorf("active enrollments = %d, want 1", got)
	}
	if got := env.status(t, orderID); got != "processing" {
		t.Errorf("status = %q, want processing (mixed order waits for shipping)", got)
	}

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

func TestOrderPaid_ConcurrentRefundNeverLeavesAccessOnACancelledOrder(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)

	for i := 0; i < 6; i++ {
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
			t.Fatalf("iteration %d: poll outbox: %v", i, err)
		}
		if err := <-refundDone; err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}

		if got := env.status(t, orderID); got != "cancelled" {
			t.Errorf("iteration %d: status = %q, want cancelled", i, got)
		}
		if got := env.activeEnrollments(t, student, course); got != 0 {
			t.Errorf("iteration %d: active enrollments = %d, want 0 on a refunded order", i, got)
		}
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

	if _, _, err := env.svc.MintCart(ctx, student.String()); err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	cart, _, _ := env.svc.MintCart(ctx, student.String())
	if got := env.status(t, orderID); got != "completed" {
		t.Fatalf("status = %q, want completed before the repurchase attempt", got)
	}
	err := env.svc.AddItem(ctx, student.String(), cart.ID.String(), other.String(), 1)
	var conflict *service.DigitalTargetConflictError
	if !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Reason != model.DigitalConflictOwned {
		t.Fatalf("AddItem while the access is live: err = %v, want one %q conflict", err, model.DigitalConflictOwned)
	}

	env.refund(t, orderID)
	if err := env.svc.AddItem(ctx, student.String(), cart.ID.String(), other.String(), 1); err != nil {
		t.Fatalf("AddItem after refund: %v", err)
	}
	if _, err := env.svc.Checkout(ctx, student.String(), cart.ID.String(), "repurchase-"+uuid.NewString()); err != nil {
		t.Fatalf("Checkout after refund: %v", err)
	}
}

func TestOrderPaid_ShippedMixedOrderStillGetsItsCourseAndStaysShipped(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	student := env.student(t)
	book := env.book(t, 5)
	digital := env.courseProduct(t, 50000, env.course(t))

	orderID := env.paidOrder(t, student, book, digital)
	course := env.enrollmentCourse(t, orderID)
	if err := env.repo.SetShipped(ctx, orderID, "RESI-"+uuid.NewString()[:8]); err != nil {
		t.Fatalf("SetShipped: %v", err)
	}

	env.pollAll(t, ctx)

	if got := env.activeEnrollments(t, student, course); got != 1 {
		t.Errorf("active enrollments = %d, want 1: a shipped order still owes its course", got)
	}
	if got := env.status(t, orderID); got != "shipped" {
		t.Errorf("status = %q, want shipped (not walked back to processing)", got)
	}
	if got := env.unprocessedEvents(t, orderID); got != 0 {
		t.Errorf("unprocessed outbox events = %d, want 0", got)
	}
}
