package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// seedEarlierOrder puts a one-line order for product in the given status, the
// way an earlier checkout would have left it, without running that checkout.
func seedEarlierOrder(t *testing.T, repo *repository.Repository, student, product uuid.UUID, productType, status, cancellationReason string) {
	t.Helper()
	ctx := context.Background()
	var orderID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO orders (student_id, status, cancellation_reason, subtotal, discount, shipping_cost, total)
		 VALUES ($1, $2, NULLIF($3, ''), 50000, 0, 0, 50000) RETURNING id`, student, status, cancellationReason,
	).Scan(&orderID); err != nil {
		t.Fatalf("seed %s order: %v", status, err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
		 VALUES ($1, $2, $3, 'earlier', 50000, 1, 50000)`, orderID, product, productType,
	); err != nil {
		t.Fatalf("seed %s item: %v", status, err)
	}
}

func TestDigitalPurchase_OnlyLiveOrdersBlockTheirTargetThroughAnotherSKU(t *testing.T) {
	for _, tc := range []struct {
		name, status, reason string
		blocked              bool
	}{
		{"payment_pending", "payment_pending", "", true},
		{"paid", "paid", "", true},
		{"processing", "processing", "", true},
		{"shipped", "shipped", "", true},
		{"completed", "completed", "", true},
		{"payment_expired", "payment_expired", "", false},
		{"cancelled", "cancelled", "", false},
		{"refunded", "cancelled", "refunded", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, pay := newDigitalPurchaseService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			course := seedCourseTarget(t, repo)
			earlier := seedDigitalProduct(t, repo, "course", 50000, course)
			other := seedDigitalProduct(t, repo, "course", 40000, course)
			seedEarlierOrder(t, repo, student, earlier, "course", tc.status, tc.reason)

			if !tc.blocked {
				orderID := cartOf(t, svc, student, other)
				if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
					t.Fatalf("Checkout while the earlier order is %s: %v", tc.name, err)
				}
				if n := pay.creates.Load(); n != 1 {
					t.Errorf("gateway calls = %d, want 1", n)
				}
				return
			}

			cart, _, err := svc.MintCart(ctx, student.String())
			if err != nil {
				t.Fatalf("MintCart: %v", err)
			}
			wantConflict(t, svc.AddItem(ctx, student.String(), cart.ID.String(), other.String(), 1), model.DigitalConflictClaimed)
			forceLine(t, repo, cart.ID, other, "course", 40000)
			_, err = svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey())
			wantConflict(t, err, model.DigitalConflictClaimed)
			if got := orderStatusOf(t, repo, cart.ID); got != "cart" {
				t.Errorf("order status = %q, want cart", got)
			}
			if n := pay.creates.Load(); n != 0 {
				t.Errorf("gateway calls = %d, want 0 for the refused order", n)
			}
		})
	}
}

func TestDigitalPurchase_LiveOrderBlocksTheSameSKUAgain(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
	first := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), first.String(), checkoutKey()); err != nil {
		t.Fatalf("first Checkout: %v", err)
	}

	second, created, err := svc.MintCart(ctx, student.String())
	if err != nil || !created || second.ID == first {
		t.Fatalf("MintCart = %v, created=%v, err=%v; want a new cart after checkout", second.ID, created, err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), second.ID.String(), product.String(), 1), model.DigitalConflictClaimed)
	forceLine(t, repo, second.ID, product, "course", 50000)
	_, err = svc.Checkout(ctx, student.String(), second.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictClaimed)
	if n := pay.creates.Load(); n != 1 {
		t.Errorf("gateway calls = %d, want 1 (the first order only)", n)
	}
}

func TestDigitalPurchase_LiveExamOrderBlocksTheExamThroughAnotherSKU(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	earlier := seedDigitalProduct(t, repo, "exam", 50000, exam)
	other := seedDigitalProduct(t, repo, "exam", 40000, exam)
	seedEarlierOrder(t, repo, student, earlier, "exam", "payment_pending", "")

	cart, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	forceLine(t, repo, cart.ID, other, "exam", 40000)
	_, err = svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictClaimed)
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestDigitalPurchase_RevokedCourseCanBeBoughtAgain(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	enrollInCourse(t, repo, student, course)

	cart, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), cart.ID.String(), product.String(), 1), model.DigitalConflictOwned)

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE course_session SET status = 'revoked', revoked_at = now() WHERE student_id = $1 AND course_id = $2`, student, course,
	); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := svc.AddItem(ctx, student.String(), cart.ID.String(), product.String(), 1); err != nil {
		t.Fatalf("AddItem after revoke: %v", err)
	}
	if _, err := svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout after revoke: %v", err)
	}
}

// The guard is a read, so it is race-free only while a student has one open
// cart: checkout locks that row, and a second checkout waits on it.
func TestOpenCart_AStudentCanHoldOnlyOne(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)

	first, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	second, _, err := svc.MintCart(ctx, student.String())
	if err != nil || second.ID != first.ID {
		t.Fatalf("second MintCart = %v, %v; want the open cart %v", second.ID, err, first.ID)
	}

	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total) VALUES ($1, 'cart', 0, 0, 0, 0)`, student)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second open cart insert err = %v, want a unique violation", err)
	}
}

// waitForWaitersBehind polls until at least want backends are blocked, directly
// or through another waiter, by holder's transaction, so the test knows the
// checkouts reached the cart row lock from observed database state.
func waitForWaitersBehind(t *testing.T, repo *repository.Repository, holder pgx.Tx, want int) {
	t.Helper()
	ctx := context.Background()
	var holderPID int
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatalf("read holder pid: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		var n int
		if err := repo.Pool().QueryRow(ctx,
			`WITH RECURSIVE blocked AS (
			     SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))
			     UNION
			     SELECT a.pid FROM pg_stat_activity a JOIN blocked b ON b.pid = ANY(pg_blocking_pids(a.pid))
			 ) SELECT count(*) FROM blocked`, holderPID,
		).Scan(&n); err != nil {
			t.Fatalf("read waiters: %v", err)
		}
		if n >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiters behind the holder = %d, want %d: checkout never blocked on the cart row", n, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Serialization proof only: the loser fails on the cart's status, so this does
// not exercise the guard; the guard is covered by the MintCart test below.
func TestDigitalPurchase_ConcurrentCheckoutsForOneStudentCreateOneOrderAndOnePayment(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
	cartID := cartOf(t, svc, student, product)

	holder, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT 1 FROM orders WHERE id = $1 FOR UPDATE`, cartID); err != nil {
		t.Fatalf("lock cart: %v", err)
	}

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.Checkout(ctx, student.String(), cartID.String(), checkoutKey())
			done <- err
		}()
	}
	waitForWaitersBehind(t, repo, holder, 2)
	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("release cart: %v", err)
	}

	var succeeded, notEditable int
	for i := 0; i < 2; i++ {
		switch err := <-done; {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrOrderNotEditable):
			notEditable++
		default:
			t.Errorf("unexpected Checkout error: %v", err)
		}
	}
	if succeeded != 1 || notEditable != 1 {
		t.Errorf("succeeded=%d notEditable=%d, want 1 and 1", succeeded, notEditable)
	}
	if n := pendingOrdersOf(t, repo, student); n != 1 {
		t.Errorf("pending orders = %d, want 1", n)
	}
	if n := pay.creates.Load(); n != 1 {
		t.Errorf("gateway calls = %d, want 1", n)
	}
}

// The holder stands in for a checkout transaction: it has moved the cart to
// payment_pending and not committed. A concurrent MintCart must wait on that
// row and then get a new cart, which only the guard can refuse for the target.
func TestDigitalPurchase_MintCartWaitsForCheckoutThenGuardRefusesTheSameTarget(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
	first := cartOf(t, svc, student, product)

	holder, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx,
		`UPDATE orders SET status = 'payment_pending', payment_expires_at = now() + interval '1 hour' WHERE id = $1`, first,
	); err != nil {
		t.Fatalf("move cart to payment_pending: %v", err)
	}

	type minted struct {
		order   model.Order
		created bool
		err     error
	}
	done := make(chan minted, 1)
	go func() {
		order, created, err := svc.MintCart(ctx, student.String())
		done <- minted{order, created, err}
	}()
	waitForWaitersBehind(t, repo, holder, 1)
	select {
	case m := <-done:
		t.Fatalf("MintCart returned %v before the checkout committed", m.order.ID)
	default:
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	m := <-done
	if m.err != nil || !m.created || m.order.ID == first {
		t.Fatalf("MintCart = %v, created=%v, err=%v; want a new cart once the order committed", m.order.ID, m.created, m.err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), m.order.ID.String(), product.String(), 1), model.DigitalConflictClaimed)
	forceLine(t, repo, m.order.ID, product, "course", 50000)
	_, err = svc.Checkout(ctx, student.String(), m.order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictClaimed)
	if got := orderStatusOf(t, repo, m.order.ID); got != "cart" {
		t.Errorf("second order status = %q, want cart", got)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}
