package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/repository"
)

// lifecyclePayments is a gateway fake whose signature always verifies, whose
// reconcile answer is switchable, and which counts CreatePayment calls.
type lifecyclePayments struct {
	countingPaymentClient
	paid atomic.Bool
}

func (p *lifecyclePayments) QueryStatus(context.Context, string) (PaymentStatus, error) {
	return PaymentStatus{Paid: p.paid.Load()}, nil
}

func (p *lifecyclePayments) VerifySignature([]byte, string) bool { return true }

func newLifecycleService(t *testing.T) (*Service, *repository.Repository, *lifecyclePayments) {
	t.Helper()
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	pay := &lifecyclePayments{}
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, pay, &NoopLogisticsClient{}, nil, nil, nil)
	return svc, repo, pay
}

// pendingCourseOrder checks out a paid-course cart so the order is
// payment_pending.
func pendingCourseOrder(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) (orderID, product, course uuid.UUID) {
	t.Helper()
	course = seedCourseTarget(t, repo)
	product = seedDigitalProduct(t, repo, "course", 50000, course)
	orderID = cartOf(t, svc, student, product)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID, product, course
}

func expireOrderNow(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET payment_expires_at = now() - interval '1 minute' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("elapse deadline: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = 'payment_expired' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("expire order: %v", err)
	}
}

func settlementBody(orderID uuid.UUID, txID string) []byte {
	b, _ := json.Marshal(map[string]string{
		"transaction_status": "settlement",
		"order_id":           orderID.String(),
		"transaction_id":     txID,
		"payment_type":       "bank_transfer",
	})
	return b
}

func orderPaidEvents(t *testing.T, repo *repository.Repository, orderID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'OrderPaid'`, orderID,
	).Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}

func webhookLogRows(t *testing.T, repo *repository.Repository, orderID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM webhook_log WHERE event_type = 'payment_success' AND gateway_ref = $1`, orderID.String(),
	).Scan(&n); err != nil {
		t.Fatalf("count webhook_log: %v", err)
	}
	return n
}

func TestWebhook_SettlementOnTerminalOrderIsAcknowledgedAndNotFulfilled(t *testing.T) {
	tests := []struct {
		name        string
		txID        string
		park        func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID)
		deliveries  int
		wantStatus  string
		wantEvents  int
		wantLogRows int
	}{
		{
			name: "expired order",
			txID: "tx-late",
			park: func(t *testing.T, _ *Service, repo *repository.Repository, orderID uuid.UUID) {
				expireOrderNow(t, repo, orderID)
			},
			deliveries:  2,
			wantStatus:  "payment_expired",
			wantEvents:  0,
			wantLogRows: 2,
		},
		{
			name: "refunded order",
			txID: "tx-1",
			park: func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID) {
				ctx := context.Background()
				if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-1"), "sig", uuid.NewString()); err != nil {
					t.Fatalf("webhook: %v", err)
				}
				actor := seedRevokeActor(t, repo)
				if err := svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"); err != nil {
					t.Fatalf("refund: %v", err)
				}
			},
			deliveries:  1,
			wantStatus:  "cancelled",
			wantEvents:  1,
			wantLogRows: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			tt.park(t, svc, repo, orderID)

			for i := 0; i < tt.deliveries; i++ {
				if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, tt.txID), "sig", uuid.NewString()); err != nil {
					t.Fatalf("webhook #%d must be acknowledged, got %v", i, err)
				}
			}

			if got := orderStatusOf(t, repo, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %s", got, tt.wantStatus)
			}
			if got := orderPaidEvents(t, repo, orderID); got != tt.wantEvents {
				t.Errorf("OrderPaid events = %d, want %d", got, tt.wantEvents)
			}
			if got := webhookLogRows(t, repo, orderID); got != tt.wantLogRows {
				t.Errorf("webhook_log rows = %d, want %d: every delivery must leave a durable row", got, tt.wantLogRows)
			}
		})
	}
}

func TestWebhook_RepeatedSettlementEmitsOneOrderPaid(t *testing.T) {
	tests := []struct {
		name       string
		txID       string
		deliveries int
		concurrent bool
	}{
		{"sequential redeliveries", "tx-dup", 3, false},
		{"simultaneous deliveries", "tx-race", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)

			errs := make([]error, tt.deliveries)
			deliver := func(i int) {
				errs[i] = svc.HandlePaymentWebhook(ctx, settlementBody(orderID, tt.txID), "sig", uuid.NewString())
			}
			if tt.concurrent {
				var wg sync.WaitGroup
				for i := 0; i < tt.deliveries; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						deliver(i)
					}(i)
				}
				wg.Wait()
			} else {
				for i := 0; i < tt.deliveries; i++ {
					deliver(i)
				}
			}

			for i, err := range errs {
				if err != nil {
					t.Errorf("delivery #%d: %v", i, err)
				}
			}
			if got := orderStatusOf(t, repo, orderID); got != "paid" {
				t.Errorf("status = %q, want paid", got)
			}
			if got := orderPaidEvents(t, repo, orderID); got != 1 {
				t.Errorf("OrderPaid events = %d, want 1", got)
			}
			if got := webhookLogRows(t, repo, orderID); got != tt.deliveries {
				t.Errorf("webhook_log rows = %d, want %d: every delivery is logged, including repeats on a settled order", got, tt.deliveries)
			}
		})
	}
}

func TestWebhook_UnknownOrderIsNotFound(t *testing.T) {
	svc, _, _ := newLifecycleService(t)
	err := svc.HandlePaymentWebhook(context.Background(), settlementBody(uuid.New(), "tx-none"), "sig", uuid.NewString())
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestAdminConfirmOrder_RefusesOrdersThatAreNotAwaitingPayment(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "confirmguard_")

	expiredID, _, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, expiredID)
	paidID, _, _ := pendingCourseOrder(t, svc, repo, newDigitalStudent(t, repo))
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(paidID, "tx-paid"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	cartID := cartOf(t, svc, newDigitalStudent(t, repo))

	for name, orderID := range map[string]uuid.UUID{"expired": expiredID, "paid": paidID, "cart": cartID} {
		proof := "payment_proof/" + actor + "/" + uniqueSuffix() + ".jpg"
		err := svc.AdminConfirmOrder(ctx, actor, orderID.String(), "confirm-"+uniqueSuffix(), proof)
		if !errors.Is(err, ErrOrderNotPayable) {
			t.Errorf("%s: err = %v, want ErrOrderNotPayable", name, err)
		}
	}
	if got := orderStatusOf(t, repo, expiredID); got != "payment_expired" {
		t.Errorf("expired order status = %q, want payment_expired", got)
	}
	if got := orderPaidEvents(t, repo, expiredID); got != 0 {
		t.Errorf("expired order OrderPaid events = %d, want 0", got)
	}
	if got := orderPaidEvents(t, repo, paidID); got != 1 {
		t.Errorf("paid order OrderPaid events = %d, want the webhook's 1 only", got)
	}
}

func TestAdminReconcileOrder_GatewayPaidOnExpiredOrderIsRefusedNotMarkedPaid(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "reconcileguard_")
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, orderID)
	pay.paid.Store(true)

	for i := 0; i < 2; i++ {
		err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix())
		if !errors.Is(err, ErrOrderNotPayable) {
			t.Fatalf("reconcile #%d err = %v, want ErrOrderNotPayable", i, err)
		}
	}

	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 0 {
		t.Errorf("OrderPaid events = %d, want 0", got)
	}
}

func TestAdminReconcileOrder_RepeatOnPaidOrderDoesNotEmitAgain(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "reconcilerepeat_")
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	pay.paid.Store(true)

	for i := 0; i < 2; i++ {
		if err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix()); err != nil {
			t.Fatalf("reconcile #%d: %v", i, err)
		}
	}

	if got := orderStatusOf(t, repo, orderID); got != "paid" {
		t.Errorf("status = %q, want paid", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 1 {
		t.Errorf("OrderPaid events = %d, want 1", got)
	}
}

func TestAdminRefundOrder_ConcurrentRefundsApplyOnce(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-refund"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	actor := seedRevokeActor(t, repo)

	const callers = 4
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png")
		}(i)
	}
	wg.Wait()

	succeeded := 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !errors.Is(err, ErrOrderNotRefundable):
			t.Errorf("loser err = %v, want ErrOrderNotRefundable", err)
		}
	}
	if succeeded != 1 {
		t.Errorf("successful refunds = %d, want exactly 1", succeeded)
	}
	var audits int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = 'order.refund'`, orderID.String(),
	).Scan(&audits); err != nil {
		t.Fatalf("count refund audits: %v", err)
	}
	if audits != 1 {
		t.Errorf("refund audit rows = %d, want 1", audits)
	}
}
