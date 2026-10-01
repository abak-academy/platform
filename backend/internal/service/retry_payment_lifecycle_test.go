package service

import (
	"context"
	"errors"
	"testing"

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
			if got := pay.creates.Load() - before; got != 1 {
				t.Errorf("gateway calls = %d, want 1", got)
			}
		})
	}
}

func pendingOrderFor(t *testing.T, svc *Service, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := cartOf(t, svc, student, products...)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID
}

func leaveTwentySecondsOfWindow(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	setDeadline(t, repo, orderID, "20 seconds")
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
		{"less than a minute left is refused as too close", leaveTwentySecondsOfWindow, ErrPaymentDeadlineTooClose, "payment_pending"},
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
