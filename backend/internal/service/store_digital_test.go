package service

import (
	"context"
	"errors"
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

func pendingOrdersOf(t *testing.T, repo *repository.Repository, studentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM orders WHERE student_id = $1 AND status = 'payment_pending'`, studentID,
	).Scan(&n); err != nil {
		t.Fatalf("count pending orders: %v", err)
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

type digitalEnv struct {
	svc     *Service
	repo    *repository.Repository
	student uuid.UUID
}

type ownedSetup struct {
	product uuid.UUID
	target  uuid.UUID
	kind    string
	price   float64
}

func revokeCourseSession(t *testing.T, repo *repository.Repository, studentID, courseID uuid.UUID) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE course_session SET status = 'revoked', revoked_at = now() WHERE student_id = $1 AND course_id = $2`, studentID, courseID,
	); err != nil {
		t.Fatalf("revoke course: %v", err)
	}
}

func revokeExamRegistration(t *testing.T, repo *repository.Repository, studentID, examID uuid.UUID) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE exam_registration SET status = 'revoked', revoked_at = now() WHERE student_id = $1 AND exam_id = $2`, studentID, examID,
	); err != nil {
		t.Fatalf("revoke exam: %v", err)
	}
}

func TestDigitalPurchase_CheckoutByTargetOwnership(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		arrange      func(t *testing.T, e digitalEnv) ownedSetup
		wantConflict string
	}{
		{
			name: "owned course",
			arrange: func(t *testing.T, e digitalEnv) ownedSetup {
				course := seedCourseTarget(t, e.repo)
				enrollInCourse(t, e.repo, e.student, course)
				return ownedSetup{product: seedDigitalProduct(t, e.repo, "course", 50000, course), target: course, kind: "course", price: 50000}
			},
			wantConflict: model.DigitalConflictOwned,
		},
		{
			name: "live exam registration",
			arrange: func(t *testing.T, e digitalEnv) ownedSetup {
				exam := seedExamTarget(t, e.repo)
				registerForExam(t, e.repo, e.student, exam, "registered")
				return ownedSetup{product: seedDigitalProduct(t, e.repo, "exam", 50000, exam), target: exam, kind: "exam", price: 50000}
			},
			wantConflict: model.DigitalConflictOwned,
		},
		{
			name: "revoked course session",
			arrange: func(t *testing.T, e digitalEnv) ownedSetup {
				course := seedCourseTarget(t, e.repo)
				enrollInCourse(t, e.repo, e.student, course)
				revokeCourseSession(t, e.repo, e.student, course)
				return ownedSetup{product: seedDigitalProduct(t, e.repo, "course", 50000, course), target: course, kind: "course", price: 50000}
			},
		},
		{
			name: "revoked exam registration",
			arrange: func(t *testing.T, e digitalEnv) ownedSetup {
				exam := seedExamTarget(t, e.repo)
				registerForExam(t, e.repo, e.student, exam, "registered")
				revokeExamRegistration(t, e.repo, e.student, exam)
				return ownedSetup{product: seedDigitalProduct(t, e.repo, "exam", 50000, exam), target: exam, kind: "exam", price: 50000}
			},
		},
		{
			name: "bundle with one owned target is rejected whole",
			arrange: func(t *testing.T, e digitalEnv) ownedSetup {
				owned, other := seedCourseTarget(t, e.repo), seedCourseTarget(t, e.repo)
				enrollInCourse(t, e.repo, e.student, owned)
				return ownedSetup{product: seedDigitalProduct(t, e.repo, "course", 90000, owned, other), target: owned, kind: "course", price: 90000}
			},
			wantConflict: model.DigitalConflictOwned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newDigitalPurchaseService(t)
			student := newDigitalStudent(t, repo)
			setup := tt.arrange(t, digitalEnv{svc: svc, repo: repo, student: student})

			cart, _, err := svc.MintCart(ctx, student.String())
			if err != nil {
				t.Fatalf("MintCart: %v", err)
			}
			blocked := tt.wantConflict != ""
			err = svc.AddItem(ctx, student.String(), cart.ID.String(), setup.product.String(), 1)
			wantStatus, wantGateway, wantPending := "payment_pending", int32(1), 1
			if blocked {
				wantConflict(t, err, tt.wantConflict)
				forceLine(t, repo, cart.ID, setup.product, setup.kind, setup.price)
				wantStatus, wantGateway, wantPending = "cart", 0, 0
			} else if err != nil {
				t.Fatalf("AddItem: %v", err)
			}

			_, err = svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey())
			if blocked {
				conflictErr := wantConflict(t, err, tt.wantConflict)
				if len(conflictErr.Conflicts) != 1 || conflictErr.Conflicts[0].TargetID != setup.target || conflictErr.Conflicts[0].ProductID != setup.product {
					t.Errorf("conflicts = %+v, want only target %v of product %v", conflictErr.Conflicts, setup.target, setup.product)
				}
			} else if err != nil {
				t.Fatalf("Checkout: %v", err)
			}
			if got := orderStatusOf(t, repo, cart.ID); got != wantStatus {
				t.Errorf("order status = %q, want %s", got, wantStatus)
			}
			if n := pay.creates.Load(); n != wantGateway {
				t.Errorf("gateway calls = %d, want %d", n, wantGateway)
			}
			if n := pendingOrdersOf(t, repo, student); n != wantPending {
				t.Errorf("pending orders = %d, want %d", n, wantPending)
			}
		})
	}
}

func TestDigitalPurchase_RevokingAnExamRegistrationUnblocksTheRefusedOrder(t *testing.T) {
	ctx := context.Background()
	svc, repo, pay := newDigitalPurchaseService(t)
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	registerForExam(t, repo, student, exam, "registered")
	product := seedDigitalProduct(t, repo, "exam", 50000, exam)

	cart, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	wantConflict(t, svc.AddItem(ctx, student.String(), cart.ID.String(), product.String(), 1), model.DigitalConflictOwned)
	forceLine(t, repo, cart.ID, product, "exam", 50000)

	_, err = svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey())
	conflictErr := wantConflict(t, err, model.DigitalConflictOwned)
	if len(conflictErr.Conflicts) != 1 || conflictErr.Conflicts[0].TargetID != exam || conflictErr.Conflicts[0].ProductID != product {
		t.Errorf("conflicts = %+v, want only target %v of product %v", conflictErr.Conflicts, exam, product)
	}
	if got := orderStatusOf(t, repo, cart.ID); got != "cart" {
		t.Errorf("order status after refusal = %q, want cart", got)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls after refusal = %d, want 0", n)
	}
	if n := pendingOrdersOf(t, repo, student); n != 0 {
		t.Errorf("pending orders after refusal = %d, want 0", n)
	}

	revokeExamRegistration(t, repo, student, exam)

	if _, err := svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout after revoke: %v", err)
	}
	if got := orderStatusOf(t, repo, cart.ID); got != "payment_pending" {
		t.Errorf("order status = %q, want payment_pending", got)
	}
	if n := pay.creates.Load(); n != 1 {
		t.Errorf("gateway calls = %d, want 1", n)
	}
	if n := pendingOrdersOf(t, repo, student); n != 1 {
		t.Errorf("pending orders = %d, want 1", n)
	}
}

func TestDigitalPurchase_TargetRepeatedAcrossLinesIsRefused(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	shared, extra := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
	single := seedDigitalProduct(t, repo, "course", 50000, shared)
	bundle := seedDigitalProduct(t, repo, "course", 90000, shared, extra)

	cart := cartOf(t, svc, student, single)
	wantConflict(t, svc.AddItem(ctx, student.String(), cart.String(), bundle.String(), 1), model.DigitalConflictDuplicateInCart)
	forceLine(t, repo, cart, bundle, "course", 90000)
	_, err := svc.Checkout(ctx, student.String(), cart.String(), checkoutKey())
	wantConflict(t, err, model.DigitalConflictDuplicateInCart)

	if got := orderStatusOf(t, repo, cart); got != "cart" {
		t.Errorf("order status = %q, want cart", got)
	}
	if n := pay.creates.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
	if n := pendingOrdersOf(t, repo, student); n != 0 {
		t.Errorf("pending orders = %d, want 0", n)
	}
}

func TestDigitalPurchase_ConflictInMixedCartRollsBackStockAndPayment(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		owned  bool
	}{
		{"owned target", model.DigitalConflictOwned, true},
		{"target repeated across lines", model.DigitalConflictDuplicateInCart, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newDigitalPurchaseService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			course := seedCourseTarget(t, repo)
			digital := seedDigitalProduct(t, repo, "course", 50000, course)
			sameTarget := seedDigitalProduct(t, repo, "course", 40000, course)
			if tt.owned {
				enrollInCourse(t, repo, student, course)
			}

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
			if !tt.owned {
				forceLine(t, repo, order.ID, sameTarget, "course", 40000)
			}
			if _, err := repo.Pool().Exec(ctx,
				`UPDATE orders SET shipping_cost = 18000, selected_courier = 'JNE' WHERE id = $1`, order.ID,
			); err != nil {
				t.Fatalf("set shipping: %v", err)
			}

			_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
			wantConflict(t, err, tt.reason)

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
		})
	}
}

func TestDigitalPurchase_PaidFreeOrderBlocksAnotherSKUBeforeAndAfterFulfilment(t *testing.T) {
	tests := []struct {
		name         string
		fulfil       bool
		wantConflict string
	}{
		{"before the order is fulfilled", false, model.DigitalConflictClaimed},
		{"after the order is fulfilled", true, model.DigitalConflictOwned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			if tt.fulfil {
				if _, err := repo.Pool().Exec(ctx,
					`UPDATE orders SET status = 'completed' WHERE student_id = $1 AND status = 'paid'`, student,
				); err != nil {
					t.Fatalf("complete order: %v", err)
				}
				enrollInCourse(t, repo, student, course)
			}

			order, _, err := svc.MintCart(ctx, student.String())
			if err != nil {
				t.Fatalf("MintCart: %v", err)
			}
			forceLine(t, repo, order.ID, otherFree, "course", 0)
			_, err = svc.Checkout(ctx, student.String(), order.ID.String(), checkoutKey())
			wantConflict(t, err, tt.wantConflict)

			if got := orderStatusOf(t, repo, order.ID); got != "cart" {
				t.Errorf("order status = %q, want cart", got)
			}
		})
	}
}

func TestDigitalPurchase_OtherStudentsAreUnaffected(t *testing.T) {
	svc, repo, pay := newDigitalPurchaseService(t)
	ctx := context.Background()
	first, second := newDigitalStudent(t, repo), newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)

	firstOrder := cartOf(t, svc, first, product)
	if _, err := svc.Checkout(ctx, first.String(), firstOrder.String(), checkoutKey()); err != nil {
		t.Fatalf("first student Checkout: %v", err)
	}
	secondOrder := cartOf(t, svc, second, product)
	if _, err := svc.Checkout(ctx, second.String(), secondOrder.String(), checkoutKey()); err != nil {
		t.Fatalf("second student Checkout: %v", err)
	}

	if got := orderStatusOf(t, repo, firstOrder); got != "payment_pending" {
		t.Errorf("first order status = %q, want payment_pending", got)
	}
	if got := orderStatusOf(t, repo, secondOrder); got != "payment_pending" {
		t.Errorf("second order status = %q, want payment_pending", got)
	}
	if n := pay.creates.Load(); n != 2 {
		t.Errorf("gateway calls = %d, want 2 (one per student)", n)
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
