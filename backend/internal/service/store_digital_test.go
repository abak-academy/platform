package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
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

func TestDigitalPurchase_SnapshotKeepsCheckoutTargetsAfterProductLinkEdit(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	original, replacement := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, original)

	orderID := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, product); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`, product, replacement); err != nil {
		t.Fatalf("relink: %v", err)
	}

	snapshot, err := repo.GetOrderDigitalSnapshot(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderDigitalSnapshot: %v", err)
	}
	if !snapshot.Taken || len(snapshot.Targets) != 1 || snapshot.Targets[0].TargetID != original || snapshot.Targets[0].ProductID != product {
		t.Fatalf("snapshot = %+v, want the original course %v under product %v", snapshot, original, product)
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

	snapshot, err := repo.GetOrderDigitalSnapshot(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderDigitalSnapshot: %v", err)
	}
	if snapshot.Taken || len(snapshot.Targets) != 0 {
		t.Errorf("snapshot = %+v, want none for a physical-only order", snapshot)
	}
}

// The claim primitive is what a second order of the same student (a payment
// retry, or a cart minted the instant the first checkout commits) must win
// through: whichever transaction commits first holds the target, and the
// loser is told which targets it lost instead of erroring.
func TestDigitalPurchase_ConcurrentClaimsForOneTargetHaveExactlyOneWinner(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()

	const trials = 6
	for i := 0; i < trials; i++ {
		student := newDigitalStudent(t, repo)
		course := seedCourseTarget(t, repo)
		product := seedDigitalProduct(t, repo, "course", 50000, course)
		rival := seedDigitalProduct(t, repo, "course", 40000, course)

		cartID := cartOf(t, svc, student, product)
		var rivalOrder, rivalItem uuid.UUID
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
			 VALUES ($1, 'payment_expired', 40000, 0, 0, 40000) RETURNING id`, student,
		).Scan(&rivalOrder); err != nil {
			t.Fatalf("insert rival order: %v", err)
		}
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
			 VALUES ($1, $2, 'course', 'rival', 40000, 1, 40000) RETURNING id`, rivalOrder, rival,
		).Scan(&rivalItem); err != nil {
			t.Fatalf("insert rival item: %v", err)
		}

		before := pay.creates.Load()
		start := make(chan struct{})
		var wg sync.WaitGroup
		var checkoutWon, claimWon atomic.Bool
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if _, err := svc.Checkout(ctx, student.String(), cartID.String(), checkoutKey()); err == nil {
				checkoutWon.Store(true)
			} else if !errors.Is(err, ErrDigitalTargetConflict) {
				t.Errorf("Checkout: unexpected error %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			tx, err := repo.BeginTx(ctx)
			if err != nil {
				t.Errorf("begin: %v", err)
				return
			}
			defer tx.Rollback(ctx)
			lost, err := repo.ClaimDigitalTargetsTx(ctx, tx, rivalOrder, student, []model.DigitalTarget{
				{OrderItemID: rivalItem, ProductID: rival, Kind: model.DigitalTargetCourse, TargetID: course},
			})
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if len(lost) == 0 {
				if err := tx.Commit(ctx); err != nil {
					t.Errorf("commit: %v", err)
					return
				}
				claimWon.Store(true)
			}
		}()
		close(start)
		wg.Wait()

		if checkoutWon.Load() == claimWon.Load() {
			t.Fatalf("trial %d: checkoutWon=%v claimWon=%v, want exactly one winner", i, checkoutWon.Load(), claimWon.Load())
		}
		if n := liveClaimsOf(t, repo, student); n != 1 {
			t.Fatalf("trial %d: live claims = %d, want 1", i, n)
		}
		wantCalls := before
		if checkoutWon.Load() {
			wantCalls++
		}
		if got := pay.creates.Load(); got != wantCalls {
			t.Fatalf("trial %d: gateway calls = %d, want %d (loser never reaches the gateway)", i, got, wantCalls)
		}
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
