package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

type countingPaymentClient struct {
	creates atomic.Int32
}

func (c *countingPaymentClient) CreatePayment(_ context.Context, req PaymentRequest) (PaymentResponse, error) {
	c.creates.Add(1)
	return PaymentResponse{GatewayRef: "count-" + req.OrderID, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (c *countingPaymentClient) QueryStatus(context.Context, string) (PaymentStatus, error) {
	return PaymentStatus{}, nil
}

func (c *countingPaymentClient) VerifySignature([]byte, string) bool { return false }

func newDigitalPurchaseService(t *testing.T) (*Service, *repository.Repository, *countingPaymentClient) {
	t.Helper()
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	pay := &countingPaymentClient{}
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, pay, &NoopLogisticsClient{}, nil, nil, nil)
	return svc, repo, pay
}

func newDigitalStudent(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	id := insertCheckoutStudent(t, repo, "Digital Student", "digstu_")
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE users SET unlisted_school_name = 'SMA Test', grade = 12, dob = '2008-01-01' WHERE id = $1`, id,
	); err != nil {
		t.Fatalf("set biodata: %v", err)
	}
	return uuid.MustParse(id)
}

func seedCourseTarget(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO course (title) VALUES ($1) RETURNING id`, "Course "+uuid.NewString(),
	).Scan(&id); err != nil {
		t.Fatalf("create course: %v", err)
	}
	return id
}

func seedExamTarget(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	return uuid.MustParse(createTestExamForBulk(t, repo))
}

func seedDigitalProduct(t *testing.T, repo *repository.Repository, productType string, price int, targets ...uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ($1, $2, $3, 0, 'published') RETURNING id`,
		productType, "Digital "+uuid.NewString(), price,
	).Scan(&id); err != nil {
		t.Fatalf("create product: %v", err)
	}
	link := `INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`
	if productType == "exam" {
		link = `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`
	}
	for _, target := range targets {
		if _, err := repo.Pool().Exec(ctx, link, id, target); err != nil {
			t.Fatalf("link product: %v", err)
		}
	}
	return id
}

func enrollInCourse(t *testing.T, repo *repository.Repository, studentID, courseID uuid.UUID) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO course_session (student_id, course_id, status, source) VALUES ($1, $2, 'active', 'admin')`,
		studentID, courseID,
	); err != nil {
		t.Fatalf("enroll: %v", err)
	}
}

func registerForExam(t *testing.T, repo *repository.Repository, studentID, examID uuid.UUID, status string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, $4)`,
		studentID, examID, "tok-"+uuid.NewString(), status,
	); err != nil {
		t.Fatalf("register: %v", err)
	}
}

// cartOf builds the student's cart through the service, so it stops at the
// first early-validation rejection.
func cartOf(t *testing.T, svc *Service, studentID uuid.UUID, productIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	order, _, err := svc.MintCart(context.Background(), studentID.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	for _, p := range productIDs {
		if err := svc.AddItem(context.Background(), studentID.String(), order.ID.String(), p.String(), 1); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
	}
	return order.ID
}

// forceLine puts a line in the cart through the repository, skipping
// AddItem's early check, to prove checkout alone still rejects it.
func forceLine(t *testing.T, repo *repository.Repository, orderID, productID uuid.UUID, productType string, unitPrice float64) {
	t.Helper()
	if err := repo.AddItem(context.Background(), orderID, model.OrderItem{
		ProductID: productID, ProductType: productType, Name: "forced", UnitPrice: unitPrice, Qty: 1,
	}, false); err != nil {
		t.Fatalf("repo.AddItem: %v", err)
	}
}

func checkoutKey() string { return "digital-key-" + uniqueSuffix() }

func orderStatusOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) string {
	t.Helper()
	var status string
	if err := repo.Pool().QueryRow(context.Background(), `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

func liveClaimsOf(t *testing.T, repo *repository.Repository, studentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM order_digital_target WHERE student_id = $1 AND released_at IS NULL`, studentID,
	).Scan(&n); err != nil {
		t.Fatalf("count claims: %v", err)
	}
	return n
}

func wantConflict(t *testing.T, err error, reason string) *DigitalTargetConflictError {
	t.Helper()
	if !errors.Is(err, ErrDigitalTargetConflict) {
		t.Fatalf("err = %v, want ErrDigitalTargetConflict", err)
	}
	var conflictErr *DigitalTargetConflictError
	if !errors.As(err, &conflictErr) || len(conflictErr.Conflicts) == 0 {
		t.Fatalf("err = %#v, want *DigitalTargetConflictError with conflicts", err)
	}
	for _, c := range conflictErr.Conflicts {
		if c.Reason == reason {
			return conflictErr
		}
	}
	t.Fatalf("conflicts = %+v, want one with reason %q", conflictErr.Conflicts, reason)
	return nil
}

func TestDigitalPurchase_OwnedCourse_RejectedAtCartAndCheckout(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	enrollInCourse(t, repo, student, course)

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), order.ID.String(), product.String(), 1), model.DigitalConflictOwned)

	forceLine(t, repo, order.ID, product, "course", 50000)
	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictOwned)

	if got := orderStatusOf(t, repo, order.ID); got != "cart" {
		t.Errorf("order status = %q, want cart (checkout rolled back)", got)
	}
	if n := liveClaimsOf(t, repo, student); n != 0 {
		t.Errorf("live claims = %d, want 0", n)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestDigitalPurchase_ExamRegistration_BlocksUnlessRevoked(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	product := seedDigitalProduct(t, repo, "exam", 50000, exam)
	registerForExam(t, repo, student, exam, "registered")

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	forceLine(t, repo, order.ID, product, "exam", 50000)
	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictOwned)
	if n := pay.creates.Load(); n != 0 {
		t.Fatalf("gateway calls = %d, want 0 while registration is live", n)
	}

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE exam_registration SET status = 'revoked', revoked_at = now() WHERE student_id = $1 AND exam_id = $2`, student, exam,
	); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout after revoke: %v", err)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Errorf("live claims = %d, want 1", n)
	}
}

func TestDigitalPurchase_PendingOrderBlocksDifferentSKUForSameTarget(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	first := seedDigitalProduct(t, repo, "course", 50000, course)
	second := seedDigitalProduct(t, repo, "course", 40000, course)

	firstOrder := cartOf(t, svc, student, first)
	if _, err := svc.Checkout(ctx, student.String(), firstOrder.String(), checkoutKey()); err != nil {
		t.Fatalf("first Checkout: %v", err)
	}

	secondOrder, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), secondOrder.ID.String(), second.String(), 1), model.DigitalConflictClaimed)

	forceLine(t, repo, secondOrder.ID, second, "course", 40000)
	_, err = svc.Checkout(ctx, student.String(), secondOrder.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictClaimed)

	if n := pay.creates.Load(); n != 1 {
		t.Errorf("gateway calls = %d, want 1 (first order only)", n)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Errorf("live claims = %d, want 1", n)
	}
}

func TestDigitalPurchase_BundleWithOneOwnedTargetIsRejectedWhole(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	owned, other := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	bundle := seedDigitalProduct(t, repo, "course", 90000, owned, other)
	enrollInCourse(t, repo, student, owned)

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	forceLine(t, repo, order.ID, bundle, "course", 90000)
	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	conflictErr := wantConflict(t, err, model.DigitalConflictOwned)
	if len(conflictErr.Conflicts) != 1 || conflictErr.Conflicts[0].TargetID != owned || conflictErr.Conflicts[0].ProductID != bundle {
		t.Errorf("conflicts = %+v, want only the owned target %v of bundle %v", conflictErr.Conflicts, owned, bundle)
	}
	if n := liveClaimsOf(t, repo, student); n != 0 {
		t.Errorf("live claims = %d, want 0: no partial bundle reservation", n)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestDigitalPurchase_TwoLinesSharingATargetAreRejected(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	shared, extra := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	single := seedDigitalProduct(t, repo, "course", 50000, shared)
	bundle := seedDigitalProduct(t, repo, "course", 90000, shared, extra)

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, student.String(), order.ID.String(), single.String(), 1); err != nil {
		t.Fatalf("first AddItem: %v", err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), order.ID.String(), bundle.String(), 1), model.DigitalConflictDuplicateInCart)

	forceLine(t, repo, order.ID, bundle, "course", 90000)
	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictDuplicateInCart)
	if n := liveClaimsOf(t, repo, student); n != 0 {
		t.Errorf("live claims = %d, want 0", n)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestDigitalPurchase_ConflictInMixedCartRollsBackStockAndPayment(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	digital := seedDigitalProduct(t, repo, "course", 50000, course)
	enrollInCourse(t, repo, student, course)

	var bookID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, 5, 'published', 300) RETURNING id`,
		"Book "+uuid.NewString(),
	).Scan(&bookID); err != nil {
		t.Fatalf("create book: %v", err)
	}

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, student.String(), order.ID.String(), bookID.String(), 2); err != nil {
		t.Fatalf("AddItem book: %v", err)
	}
	forceLine(t, repo, order.ID, digital, "course", 50000)
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET shipping_cost = 18000, selected_courier = 'JNE' WHERE id = $1`, order.ID,
	); err != nil {
		t.Fatalf("set shipping: %v", err)
	}

	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictOwned)

	var stock int
	if err := repo.Pool().QueryRow(ctx, `SELECT stock FROM product WHERE id = $1`, bookID).Scan(&stock); err != nil {
		t.Fatalf("read stock: %v", err)
	}
	if stock != 5 {
		t.Errorf("book stock = %d, want 5 (decrement rolled back)", stock)
	}
	if got := orderStatusOf(t, repo, order.ID); got != "cart" {
		t.Errorf("order status = %q, want cart", got)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestDigitalPurchase_FreeOrderClaimsAndBlocksAnotherSKU(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	free := seedDigitalProduct(t, repo, "course", 0, course)
	otherFree := seedDigitalProduct(t, repo, "course", 0, course)

	result, err := svc.Checkout(ctx, student.String(), cartOf(t, svc, student, free).String(), checkoutKey())
	if err != nil || !result.Free {
		t.Fatalf("free Checkout = %+v, %v; want Free", result, err)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Fatalf("live claims = %d, want 1 kept by the paid free order", n)
	}

	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	forceLine(t, repo, order.ID, otherFree, "course", 0)
	_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictClaimed)
	if got := orderStatusOf(t, repo, order.ID); got != "cart" {
		t.Errorf("order status = %q, want cart", got)
	}
}

func TestDigitalPurchase_OtherStudentsAreUnaffected(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	first, second := newDigitalStudent(t, repo), newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)

	if _, err := svc.Checkout(ctx, first.String(), cartOf(t, svc, first, product).String(), checkoutKey()); err != nil {
		t.Fatalf("first student Checkout: %v", err)
	}
	if _, err := svc.Checkout(ctx, second.String(), cartOf(t, svc, second, product).String(), checkoutKey()); err != nil {
		t.Fatalf("second student Checkout: %v", err)
	}
}

func TestDigitalPurchase_PhysicalOnlyOrderTakesNoSnapshot(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)

	var bookID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, 5, 'published', 300) RETURNING id`,
		"Book "+uuid.NewString(),
	).Scan(&bookID); err != nil {
		t.Fatalf("create book: %v", err)
	}
	orderID := cartOf(t, svc, student, bookID)
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET shipping_cost = 18000, selected_courier = 'JNE' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("set shipping: %v", err)
	}
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	snapshot, err := repo.GetOrderDigitalSnapshot(ctx, nil, orderID)
	if err != nil {
		t.Fatalf("GetOrderDigitalSnapshot: %v", err)
	}
	if snapshot.Taken || len(snapshot.Targets) != 0 {
		t.Errorf("snapshot = %+v, want none for a physical-only order", snapshot)
	}
}

// waitForWaitersBehind polls until at least want backends are blocked, directly
// or through another waiter, by holder's transaction. Scoping the wait to the
// holder means an unrelated lock waiter in the shared database cannot satisfy
// it, and a checkout that never reaches the lock fails the test instead of
// slipping past the ordering it describes.
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
			t.Fatalf("waiters behind the holder = %d, want %d: checkout never blocked on the rival's uncommitted claim", n, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// openRivalClaim holds an uncommitted claim on course for the student, the way a
// second order of theirs (a payment retry) would mid-transaction. The returned
// tx must be committed or rolled back by the caller.
func openRivalClaim(t *testing.T, repo *repository.Repository, student, course uuid.UUID) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	rivalProduct := seedDigitalProduct(t, repo, "course", 40000, course)
	var rivalOrder, rivalItem uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'payment_expired', 40000, 0, 0, 40000) RETURNING id`, student,
	).Scan(&rivalOrder); err != nil {
		t.Fatalf("insert rival order: %v", err)
	}
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
		 VALUES ($1, $2, 'course', 'rival', 40000, 1, 40000) RETURNING id`, rivalOrder, rivalProduct,
	).Scan(&rivalItem); err != nil {
		t.Fatalf("insert rival item: %v", err)
	}
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin rival tx: %v", err)
	}
	lost, err := repo.ClaimDigitalTargetsTx(ctx, tx, rivalOrder, student, []model.DigitalTarget{
		{OrderItemID: rivalItem, ProductID: rivalProduct, Kind: model.DigitalTargetCourse, TargetID: course},
	})
	if err != nil || len(lost) != 0 {
		tx.Rollback(ctx)
		t.Fatalf("rival claim = lost %v, err %v; want a clean uncommitted claim", lost, err)
	}
	return tx
}

// The claim is the unique index, not the eligibility read: Checkout's read
// cannot see the rival's uncommitted claim, so only blocking on the index
// insert and then losing when the rival commits keeps the target single-owner.
func TestDigitalPurchase_CheckoutBlocksOnUncommittedRivalClaimAndLosesWhenItCommits(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	cartID := cartOf(t, svc, student, product)

	rival := openRivalClaim(t, repo, student, course)
	defer rival.Rollback(ctx)

	done := make(chan error, 1)
	go func() {
		_, err := svc.Checkout(ctx, student.String(), cartID.String(), checkoutKey())
		done <- err
	}()

	waitForWaitersBehind(t, repo, rival, 1)
	select {
	case err := <-done:
		t.Fatalf("Checkout returned %v before the rival claim resolved", err)
	default:
	}
	if err := rival.Commit(ctx); err != nil {
		t.Fatalf("commit rival: %v", err)
	}

	wantConflict(t, <-done, model.DigitalConflictClaimed)
	if got := orderStatusOf(t, repo, cartID); got != "cart" {
		t.Errorf("order status = %q, want cart", got)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Errorf("live claims = %d, want 1 (the rival's)", n)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0 for the loser", n)
	}
}

// When the rival rolls back, the blocked checkout wins. A second submit of the
// same cart waits on the order row and must not create a second claim or payment.
func TestDigitalPurchase_CheckoutWinsWhenRivalRollsBackAndDoubleSubmitPaysOnce(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	cartID := cartOf(t, svc, student, product)

	rival := openRivalClaim(t, repo, student, course)
	defer rival.Rollback(ctx)

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.Checkout(ctx, student.String(), cartID.String(), checkoutKey())
			done <- err
		}()
	}
	waitForWaitersBehind(t, repo, rival, 2)
	if err := rival.Rollback(ctx); err != nil {
		t.Fatalf("rollback rival: %v", err)
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
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Errorf("live claims = %d, want 1", n)
	}
	if n := pay.creates.Load(); n != 1 {
		t.Errorf("gateway calls = %d, want 1", n)
	}
}

func TestDigitalPurchase_ReleasedClaimAllowsRepurchase(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)

	orderID := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := repo.ReleaseDigitalTargetsTx(ctx, tx, orderID, "test"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if _, err := svc.Checkout(ctx, student.String(), cartOf(t, svc, student, product).String(), checkoutKey()); err != nil {
		t.Fatalf("repurchase after release: %v", err)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Errorf("live claims = %d, want 1 (the new order)", n)
	}
}

func TestDigitalPurchase_RefundReleasesClaimForRepurchase(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 0, course)

	orderID := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if n := liveClaimsOf(t, repo, student); n != 1 {
		t.Fatalf("live claims = %d, want 1 before refund", n)
	}

	actor := seedRevokeActor(t, repo)
	if err := svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"); err != nil {
		t.Fatalf("AdminRefundOrder: %v", err)
	}
	if n := liveClaimsOf(t, repo, student); n != 0 {
		t.Errorf("live claims = %d, want 0 after refund", n)
	}

	if _, err := svc.Checkout(ctx, student.String(), cartOf(t, svc, student, product).String(), checkoutKey()); err != nil {
		t.Fatalf("repurchase after refund: %v", err)
	}
}

func TestDigitalPurchase_ProductWithNoLinkedTargetsIsNotCharged(t *testing.T) {
	for _, productType := range []string{"course", "exam"} {
		t.Run(productType, func(t *testing.T) {
			svc, repo, pay := newDigitalPurchaseService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			unlinked := seedDigitalProduct(t, repo, productType, 50000)

			orderID := cartOf(t, svc, student, unlinked)
			_, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey())
			if !errors.Is(err, ErrDigitalProductNoTargets) {
				t.Fatalf("Checkout err = %v, want ErrDigitalProductNoTargets", err)
			}
			if got := orderStatusOf(t, repo, orderID); got != "cart" {
				t.Errorf("order status = %q, want cart", got)
			}
			if n := pay.creates.Load(); n != 0 {
				t.Errorf("gateway calls = %d, want 0", n)
			}
			if snapshot, err := repo.GetOrderDigitalSnapshot(ctx, nil, orderID); err != nil || snapshot.Taken {
				t.Errorf("snapshot = %+v, err %v; want none", snapshot, err)
			}
		})
	}
}

// linkFreshCourse gives a course product something to grant; checkout refuses
// digital products that link nothing.
func linkFreshCourse(t *testing.T, repo *repository.Repository, productID string) {
	t.Helper()
	course := seedCourseTarget(t, repo)
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`, productID, course,
	); err != nil {
		t.Fatalf("link course: %v", err)
	}
}
