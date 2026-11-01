package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func retryKey() string { return "retry-key-" + uniqueSuffix() }

func TestRetryPayment_RefusesWhenTheTargetBecameUnavailable(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		arrange   func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "exam the student was granted meanwhile",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				exam := seedExamTarget(t, repo)
				product := seedDigitalProduct(t, repo, "exam", 50000, exam)
				orderID := pendingOrderFor(t, svc, student, product)
				registerForExam(t, repo, student, exam, "registered")
				return orderID
			},
			assertErr: func(t *testing.T, err error) { wantConflict(t, err, model.DigitalConflictOwned) },
		},
		{
			name: "order whose target another live order holds",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				course := seedCourseTarget(t, repo)
				holder := seedDigitalProduct(t, repo, "course", 50000, course)
				sameCourse := seedDigitalProduct(t, repo, "course", 40000, course)
				pendingOrderFor(t, svc, student, holder)

				loser := cartOf(t, svc, student)
				forceLine(t, repo, loser, sameCourse, "course", 40000)
				if _, err := repo.Pool().Exec(ctx,
					`UPDATE orders SET status = 'payment_pending', total = 40000 WHERE id = $1`, loser,
				); err != nil {
					t.Fatalf("make second pending order: %v", err)
				}
				return loser
			},
			assertErr: func(t *testing.T, err error) { wantConflict(t, err, model.DigitalConflictClaimed) },
		},
		{
			name: "order with a line that no longer links anything",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				courseA, courseB := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
				productA := seedDigitalProduct(t, repo, "course", 50000, courseA)
				productB := seedDigitalProduct(t, repo, "course", 40000, courseB)
				pending := pendingOrderFor(t, svc, student, productA, productB)
				if _, err := repo.Pool().Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, productB); err != nil {
					t.Fatalf("unlink product: %v", err)
				}
				return pending
			},
			assertErr: func(t *testing.T, err error) {
				if !errors.Is(err, ErrDigitalProductNoTargets) {
					t.Fatalf("err = %v, want ErrDigitalProductNoTargets", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID := tt.arrange(t, svc, repo, student)
			before := pay.creates.Load()
			statusBefore := orderStatusOf(t, repo, orderID)

			_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

			tt.assertErr(t, err)
			if got := orderStatusOf(t, repo, orderID); got != statusBefore {
				t.Errorf("status = %q, want unchanged %q", got, statusBefore)
			}
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls = %d, want 0", got)
			}
		})
	}
}

func TestRetryPayment_ProceedsWhenTheRecheckHasNothingToRefuse(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		arrange func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID
	}{
		{
			name: "bulk order the buyer already owns the exam of",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				participant := newDigitalStudent(t, repo)
				exam := seedExamTarget(t, repo)
				product := seedDigitalProduct(t, repo, "exam", 50000, exam)
				orderID := pendingOrderFor(t, svc, student, product)
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, orderID, participant); err != nil {
					t.Fatalf("seed participant: %v", err)
				}
				registerForExam(t, repo, student, exam, "registered")
				return orderID
			},
		},
		{
			name: "order whose only live claim on its targets is itself",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				productA := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
				productB := seedDigitalProduct(t, repo, "course", 40000, seedCourseTarget(t, repo))
				return pendingOrderFor(t, svc, student, productA, productB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID := tt.arrange(t, svc, repo, student)
			before := pay.creates.Load()

			if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey()); err != nil {
				t.Fatalf("RetryPayment: %v", err)
			}
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls = %d, want 0", got)
			}
		})
	}
}

func pendingOrderFor(t *testing.T, svc *Service, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := cartOf(t, svc, student, products...)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey(), ""); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID
}

func leaveTwentySecondsOfWindow(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	setDeadline(t, repo, orderID, "20 seconds")
}

func leaveTwoMinutesThirtySecondsOfWindow(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	setDeadline(t, repo, orderID, "2 minutes 30 seconds")
}

func expireByTheSweeper(t *testing.T, _ *Service, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	expireOrderNow(t, repo, orderID)
}

func settleByWebhook(t *testing.T, svc *Service, _ *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	if err := svc.HandlePaymentWebhook(context.Background(), settlementBody(orderID, "tx-"+uniqueSuffix()), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
}

func TestRetryPayment_OrderPastItsWindowIsRefusedWithoutGatewayCall(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		pastWindow func(t *testing.T, repo *repository.Repository, orderID uuid.UUID)
		wantErr    error
		wantStatus string
	}{
		{"order the sweeper already expired is never reopened", expireOrderNow, ErrPaymentExpired, "payment_expired"},
		{"elapsed deadline is refused before the sweeper runs, and retry does not expire the order itself", elapseDeadline, ErrPaymentExpired, "payment_pending"},
		{"20 seconds left is refused as too close", leaveTwentySecondsOfWindow, ErrPaymentDeadlineTooClose, "payment_pending"},
		{"2m30s left, inside the session margin plus a minute, is refused as too close", leaveTwoMinutesThirtySecondsOfWindow, ErrPaymentDeadlineTooClose, "payment_pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			tt.pastWindow(t, repo, orderID)
			deadlineBefore := *deadlineOf(t, repo, orderID)
			before := pay.creates.Load()

			_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := orderStatusOf(t, repo, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
			if got := *deadlineOf(t, repo, orderID); !got.Equal(deadlineBefore) {
				t.Errorf("deadline = %v, want unchanged %v", got, deadlineBefore)
			}
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls = %d, want 0", got)
			}
		})
	}
}

func TestRetryPayment_CachedResponseDoesNotBypassOrderState(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		change      func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID)
		wantErr     error
		wantSameRef bool
	}{
		{"unchanged order replays the first response", nil, nil, true},
		{"expired order is refused", expireByTheSweeper, ErrPaymentExpired, false},
		{"paid order is refused", settleByWebhook, ErrOrderNotEditable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, newDigitalStudent(t, repo))
			student := studentOf(t, repo, orderID)
			clearPaymentForRetry(t, svc, repo, orderID)
			key := retryKey()
			first, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key)
			if err != nil {
				t.Fatalf("first retry: %v", err)
			}
			if tt.change != nil {
				tt.change(t, svc, repo, orderID)
			}
			before := pay.creates.Load()

			replay, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("replay err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantSameRef && replay.GatewayRef != first.GatewayRef {
				t.Errorf("replay ref = %q, want the first call's %q", replay.GatewayRef, first.GatewayRef)
			}
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls during replay = %d, want 0", got)
			}
		})
	}
}

func TestRetryPayment_OneKeyUsedOnTwoOrdersNeverHandsOutTheOtherOrdersReference(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderA, _, _ := pendingCourseOrder(t, svc, repo, student)
	orderB, _, _ := pendingCourseOrder(t, svc, repo, student)
	clearPaymentForRetry(t, svc, repo, orderA)
	key := retryKey()

	first, err := svc.RetryPayment(ctx, student.String(), orderA.String(), key)
	if err != nil {
		t.Fatalf("retry A: %v", err)
	}
	got, err := svc.RetryPayment(ctx, student.String(), orderB.String(), key)
	if err != nil {
		t.Fatalf("retry B with A's key: %v", err)
	}
	if got.GatewayRef == first.GatewayRef || got.GatewayRef != gatewayRefOf(t, repo, orderB) {
		t.Errorf("order B got reference %q, want its own %q and not %q", got.GatewayRef, gatewayRefOf(t, repo, orderB), first.GatewayRef)
	}
}

func TestRetryPayment_OrderExpiringDuringGatewayCallKeepsItsOldPaymentState(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	clearPaymentForRetry(t, svc, repo, orderID)
	originalRef := gatewayRefOf(t, repo, orderID)
	pay.onCreate = func() {
		pay.onCreate = nil
		expireOrderNow(t, repo, orderID)
	}

	_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("err = %v, want ErrPaymentExpired for an order that expired while the gateway was being called", err)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired (never resurrected)", got)
	}
	if got := gatewayRefOf(t, repo, orderID); got != originalRef {
		t.Errorf("gateway ref = %q, want unchanged %q", got, originalRef)
	}
}

func gatewayRefOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) string {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.GatewayRef
}

func studentOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) uuid.UUID {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.StudentID
}

func TestRetryPayment_SessionStates(t *testing.T) {
	type prepareFunc func(t *testing.T, svc *Service, repo *repository.Repository, pay *lifecyclePayments, student uuid.UUID) (uuid.UUID, CheckoutResult)
	tests := []struct {
		name        string
		prepare     prepareFunc
		wantErr     error
		wantCreates int32
	}{
		{
			name: "checkout stored the session, so retry hands it back without a gateway call",
			prepare: func(t *testing.T, svc *Service, repo *repository.Repository, _ *lifecyclePayments, student uuid.UUID) (uuid.UUID, CheckoutResult) {
				orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
				first, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey(), "")
				if err != nil {
					t.Fatal(err)
				}
				return orderID, first
			},
		},
		{
			name: "order has a reference but its session is gone, so retry is refused",
			prepare: func(t *testing.T, svc *Service, repo *repository.Repository, _ *lifecyclePayments, student uuid.UUID) (uuid.UUID, CheckoutResult) {
				orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
				if err := svc.rdb.Del(context.Background(), "payment:session:"+orderID.String()).Err(); err != nil {
					t.Fatal(err)
				}
				return orderID, CheckoutResult{}
			},
			wantErr: ErrPaymentSessionUnavailable,
		},
		{
			name: "checkout failed before a session existed, so retry creates and stores one",
			prepare: func(t *testing.T, svc *Service, repo *repository.Repository, pay *lifecyclePayments, student uuid.UUID) (uuid.UUID, CheckoutResult) {
				orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
				pay.failCreate.Store(true)
				if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey(), ""); err == nil {
					t.Fatal("checkout should fail")
				}
				pay.failCreate.Store(false)
				return orderID, CheckoutResult{}
			},
			wantCreates: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			orderID, first := tt.prepare(t, svc, repo, pay, student)
			before, err := repo.GetOrderByID(ctx, orderID)
			if err != nil {
				t.Fatal(err)
			}
			calls := pay.creates.Load()
			key := retryKey()

			got, err := svc.RetryPayment(ctx, student.String(), orderID.String(), key)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if delta := pay.creates.Load() - calls; delta != tt.wantCreates {
				t.Errorf("gateway calls = %d, want %d", delta, tt.wantCreates)
			}
			cacheKey := "idempotency:retry:" + student.String() + ":" + orderID.String() + ":" + key
			if cached := svc.rdb.Exists(ctx, cacheKey).Val() != 0; cached != (tt.wantCreates > 0) {
				t.Errorf("retry idempotency cache written = %v, want %v", cached, tt.wantCreates > 0)
			}
			after, err := repo.GetOrderByID(ctx, orderID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != nil {
				if got != (CheckoutResult{}) {
					t.Errorf("refused retry returned %+v", got)
				}
				if after.GatewayRef != before.GatewayRef || after.Status != before.Status || !after.PaymentExpiresAt.Equal(*before.PaymentExpiresAt) || !after.UpdatedAt.Equal(before.UpdatedAt) {
					t.Error("refused retry changed the order")
				}
				return
			}

			if got.GatewayRef == "" || got.SnapToken == "" || got.PaymentURL == "" {
				t.Fatalf("retry returned no link: %+v", got)
			}
			if first.GatewayRef != "" && (got.GatewayRef != first.GatewayRef || got.SnapToken != first.SnapToken || got.PaymentURL != first.PaymentURL || !got.PaymentExpiresAt.Equal(first.PaymentExpiresAt)) {
				t.Fatalf("retry = %+v, want checkout session %+v", got, first)
			}
			if after.GatewayRef != got.GatewayRef {
				t.Errorf("attached reference = %q, want %q", after.GatewayRef, got.GatewayRef)
			}
			stored, ok := svc.loadPaymentSession(ctx, orderID.String())
			if !ok || stored.GatewayRef != got.GatewayRef || stored.SnapToken != got.SnapToken || stored.PaymentURL != got.PaymentURL || !stored.ExpiresAt.Equal(got.PaymentExpiresAt) {
				t.Errorf("stored session = %+v, %v; want %+v", stored, ok, got)
			}
			if ttl := svc.rdb.TTL(ctx, "payment:session:"+orderID.String()).Val(); ttl <= 0 || ttl > time.Until(got.PaymentExpiresAt)+time.Second {
				t.Errorf("session TTL = %v, expiry = %v", ttl, got.PaymentExpiresAt)
			}
			again, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())
			if err != nil || again.GatewayRef != got.GatewayRef || pay.creates.Load()-calls != tt.wantCreates {
				t.Errorf("second retry = %+v, %v; gateway calls %d, want %d", again, err, pay.creates.Load()-calls, tt.wantCreates)
			}
		})
	}
}

func TestRetryPayment_StoredSessionDoesNotBypassValidation(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		elapsed      bool
		otherStudent bool
		want         error
	}{
		{"expired", "payment_expired", false, false, ErrPaymentExpired},
		{"elapsed", "", true, false, ErrPaymentExpired},
		{"paid", "paid", false, false, ErrOrderNotEditable},
		{"cancelled", "cancelled", false, false, ErrOrderNotEditable},
		{"another student", "", false, true, ErrOrderNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			if _, ok := svc.loadPaymentSession(ctx, orderID.String()); !ok {
				t.Fatal("checkout session missing")
			}
			if tt.status != "" {
				if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = $2 WHERE id = $1`, orderID, tt.status); err != nil {
					t.Fatal(err)
				}
			}
			if tt.elapsed {
				elapseDeadline(t, repo, orderID)
			}
			if tt.otherStudent {
				student = newDigitalStudent(t, repo)
			}
			before := pay.creates.Load()
			got, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())
			if !errors.Is(err, tt.want) || got != (CheckoutResult{}) {
				t.Fatalf("retry = %+v, %v; want no session and %v", got, err, tt.want)
			}
			if pay.creates.Load() != before {
				t.Fatal("refused retry called gateway")
			}
		})
	}
}

func clearPaymentForRetry(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET gateway_ref = NULL WHERE id = $1`, orderID); err != nil {
		t.Fatal(err)
	}
	if err := svc.rdb.Del(ctx, "payment:session:"+orderID.String()).Err(); err != nil {
		t.Fatal(err)
	}
}
