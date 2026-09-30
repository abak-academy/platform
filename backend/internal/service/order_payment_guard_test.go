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

// TODO: uncertain: this payload shape is hand-written, not captured from the provider.
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

func manualConfirmWrites(t *testing.T, repo *repository.Repository, orderID uuid.UUID) (method, proof string, audits int) {
	t.Helper()
	ctx := context.Background()
	if err := repo.Pool().QueryRow(ctx,
		`SELECT COALESCE(payment_method, ''), COALESCE(payment_proof_url, '') FROM orders WHERE id = $1`, orderID,
	).Scan(&method, &proof); err != nil {
		t.Fatalf("read manual payment fields: %v", err)
	}
	if err := repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = 'order.confirm'`, orderID.String(),
	).Scan(&audits); err != nil {
		t.Fatalf("count confirm audits: %v", err)
	}
	return method, proof, audits
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
	tests := []struct {
		name string
		// seed leaves one order in a state that cannot be confirmed and returns it.
		seed  func(t *testing.T, svc *Service, repo *repository.Repository) uuid.UUID
		check func(t *testing.T, repo *repository.Repository, orderID uuid.UUID)
	}{
		{
			name: "expired",
			seed: func(t *testing.T, svc *Service, repo *repository.Repository) uuid.UUID {
				orderID, _, _ := pendingCourseOrder(t, svc, repo, newDigitalStudent(t, repo))
				expireOrderNow(t, repo, orderID)
				return orderID
			},
			check: func(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
				if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
					t.Errorf("status = %q, want payment_expired", got)
				}
				if got := orderPaidEvents(t, repo, orderID); got != 0 {
					t.Errorf("OrderPaid events = %d, want 0", got)
				}
			},
		},
		{
			name: "paid",
			seed: func(t *testing.T, svc *Service, repo *repository.Repository) uuid.UUID {
				orderID, _, _ := pendingCourseOrder(t, svc, repo, newDigitalStudent(t, repo))
				if err := svc.HandlePaymentWebhook(context.Background(), settlementBody(orderID, "tx-paid"), "sig", uuid.NewString()); err != nil {
					t.Fatalf("webhook: %v", err)
				}
				return orderID
			},
			check: func(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
				if got := orderPaidEvents(t, repo, orderID); got != 1 {
					t.Errorf("OrderPaid events = %d, want the webhook's 1 only", got)
				}
			},
		},
		{
			name: "cart",
			seed: func(t *testing.T, svc *Service, repo *repository.Repository) uuid.UUID {
				return cartOf(t, svc, newDigitalStudent(t, repo))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			ctx := context.Background()
			actor := insertAdminActor(t, repo, "confirmguard_")
			orderID := tt.seed(t, svc, repo)
			statusBefore := orderStatusOf(t, repo, orderID)
			methodBefore, proofBefore, _ := manualConfirmWrites(t, repo, orderID)

			proof := "payment_proof/" + actor + "/" + uniqueSuffix() + ".jpg"
			if err := svc.AdminConfirmOrder(ctx, actor, orderID.String(), "confirm-"+uniqueSuffix(), proof); !errors.Is(err, ErrOrderNotPayable) {
				t.Errorf("err = %v, want ErrOrderNotPayable", err)
			}

			if got := orderStatusOf(t, repo, orderID); got != statusBefore {
				t.Errorf("status = %q, want unchanged %q", got, statusBefore)
			}
			method, gotProof, audits := manualConfirmWrites(t, repo, orderID)
			if method != methodBefore {
				t.Errorf("payment_method = %q, want unchanged %q", method, methodBefore)
			}
			if gotProof != proofBefore || gotProof == proof {
				t.Errorf("payment_proof_url = %q, want unchanged %q and not the refused proof", gotProof, proofBefore)
			}
			if audits != 0 {
				t.Errorf("order.confirm audit rows = %d, want 0", audits)
			}
			if tt.check != nil {
				tt.check(t, repo, orderID)
			}
		})
	}
}

func TestAdminReconcileOrder_OutcomeDependsOnTheOrderStatusUnderItsLock(t *testing.T) {
	tests := []struct {
		name       string
		actorTag   string
		expire     bool
		wantErr    error
		wantStatus string
		wantEvents int
	}{
		{name: "gateway paid on an expired order is refused", actorTag: "reconcileguard_", expire: true, wantErr: ErrOrderNotPayable, wantStatus: "payment_expired", wantEvents: 0},
		{name: "repeat on a paid order does not emit again", actorTag: "reconcilerepeat_", wantStatus: "paid", wantEvents: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			actor := insertAdminActor(t, repo, tt.actorTag)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			if tt.expire {
				expireOrderNow(t, repo, orderID)
			}
			pay.paid.Store(true)

			for i := 0; i < 2; i++ {
				err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix())
				if tt.wantErr == nil && err != nil {
					t.Fatalf("reconcile #%d: %v", i, err)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("reconcile #%d err = %v, want %v", i, err, tt.wantErr)
				}
			}

			if got := orderStatusOf(t, repo, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %s", got, tt.wantStatus)
			}
			if got := orderPaidEvents(t, repo, orderID); got != tt.wantEvents {
				t.Errorf("OrderPaid events = %d, want %d", got, tt.wantEvents)
			}
		})
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

func TestWebhook_SettlementOnAnOrderThatCannotTakeItIsFlaggedAndNotFulfilled(t *testing.T) {
	tests := []struct {
		name       string
		txID       string
		arrange    func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID)
		deliveries int
		wantStatus string
		wantEvents int
		wantRows   int
	}{
		{
			name: "order the sweeper expired, settlement delivered twice",
			txID: "tx-late",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID) {
				expireOrderNow(t, repo, orderID)
			},
			deliveries: 2,
			wantStatus: "payment_expired",
			wantEvents: 0,
			wantRows:   2,
		},
		{
			name: "order refunded after its settlement, settlement redelivered",
			txID: "tx-1",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, orderID uuid.UUID) {
				if err := svc.HandlePaymentWebhook(context.Background(), settlementBody(orderID, "tx-1"), "sig", uuid.NewString()); err != nil {
					t.Fatalf("webhook: %v", err)
				}
				actor := seedRevokeActor(t, repo)
				if err := svc.AdminRefundOrder(context.Background(), actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png"); err != nil {
					t.Fatalf("refund: %v", err)
				}
			},
			deliveries: 1,
			wantStatus: "cancelled",
			wantEvents: 1,
			wantRows:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			tt.arrange(t, svc, repo, orderID)

			for i := 0; i < tt.deliveries; i++ {
				if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, tt.txID), "sig", uuid.NewString()); err != nil {
					t.Fatalf("late webhook #%d must be acknowledged, got %v", i, err)
				}
			}

			if got := orderStatusOf(t, repo, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
			if got := orderPaidEvents(t, repo, orderID); got != tt.wantEvents {
				t.Errorf("OrderPaid events = %d, want %d", got, tt.wantEvents)
			}
			if got := webhookLogRows(t, repo, orderID); got != tt.wantRows {
				t.Errorf("webhook_log rows = %d, want %d: each late settlement must leave a durable row", got, tt.wantRows)
			}
			if got := latePaymentFlags(t, repo, orderID); got != 1 {
				t.Errorf("late payment flags = %d, want exactly 1 across repeated deliveries", got)
			}
		})
	}
}

func TestAdminReconcileOrder_GatewayPaidOnExpiredOrderIsFlaggedNotMarkedPaid(t *testing.T) {
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
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Errorf("late payment flags = %d, want 1 (durably flagged once)", got)
	}
}

func TestWebhook_SettlementPastDeadlineBeforeSweepExpiresOrderInsteadOfFulfilling(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, book := pendingMixedOrder(t, svc, repo, student)
	elapseDeadline(t, repo, orderID)

	for i := 0; i < 2; i++ {
		if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-past"), "sig", uuid.NewString()); err != nil {
			t.Fatalf("webhook #%d must be acknowledged, got %v", i, err)
		}
	}

	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired: no grace after the deadline", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 0 {
		t.Errorf("OrderPaid events = %d, want 0", got)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Errorf("late payment flags = %d, want 1", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5 (restored once)", got)
	}

	tx, _ := repo.BeginTx(ctx)
	expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
	_ = tx.Rollback(ctx)
	if err != nil || expired {
		t.Errorf("sweeper transition afterwards = %v, %v; want false, nil (no second restock)", expired, err)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock after sweep = %d, want 5", got)
	}
}

func TestWebhook_LateFlagCarriesOrderAndTransactionContext(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	elapseDeadline(t, repo, orderID)

	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-ctx"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	var reference, status, source string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT payload->>'reference', payload->>'order_status', payload->>'source'
		 FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String(),
	).Scan(&reference, &status, &source); err != nil {
		t.Fatalf("read late flag: %v", err)
	}
	if reference != "tx-ctx" || status != "payment_expired" || source != "webhook" {
		t.Errorf("flag = reference %q, status %q, source %q; want tx-ctx, payment_expired, webhook", reference, status, source)
	}
}

func TestAdminReconcileOrder_GatewayPaidPastDeadlineExpiresOrderAndFlags(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "reconcilepast_")
	orderID, book := pendingMixedOrder(t, svc, repo, student)
	elapseDeadline(t, repo, orderID)
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
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Errorf("late payment flags = %d, want 1", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5 (restored once)", got)
	}
}

func TestAdminConfirmOrder_RejectsPendingOrderPastDeadlineBeforeSweep(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "confirmpast_")
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	elapseDeadline(t, repo, orderID)

	err := svc.AdminConfirmOrder(ctx, actor, orderID.String(), "confirm-"+uniqueSuffix(), "payment_proof/"+actor+"/"+uniqueSuffix()+".jpg")

	if !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("err = %v, want ErrOrderNotPayable", err)
	}
	if got := orderStatusOf(t, repo, orderID); got == "paid" {
		t.Error("order past its deadline was marked paid by admin confirm")
	}
	if got := orderPaidEvents(t, repo, orderID); got != 0 {
		t.Errorf("OrderPaid events = %d, want 0", got)
	}
}

func TestWebhook_SettlementBeforeDeadlineIsFulfilledAndNotFlagged(t *testing.T) {
	tests := []struct {
		name     string
		deadline string
	}{
		{"order with its full payment window", ""},
		{"order seconds before its deadline", "30 seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			if tt.deadline != "" {
				setDeadline(t, repo, orderID, tt.deadline)
			}

			if err := svc.HandlePaymentWebhook(context.Background(), settlementBody(orderID, "tx-ok"), "sig", uuid.NewString()); err != nil {
				t.Fatalf("webhook: %v", err)
			}

			if got := orderStatusOf(t, repo, orderID); got != "paid" {
				t.Errorf("status = %q, want paid", got)
			}
			if got := orderPaidEvents(t, repo, orderID); got != 1 {
				t.Errorf("OrderPaid events = %d, want 1", got)
			}
			if got := latePaymentFlags(t, repo, orderID); got != 0 {
				t.Errorf("late payment flags = %d, want 0", got)
			}
		})
	}
}

func TestAdminReconcileOrder_GatewayPaidSecondsBeforeDeadlineIsFulfilledAndNotFlagged(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "reconcileedge_")
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	setDeadline(t, repo, orderID, "30 seconds")
	pay.paid.Store(true)

	if err := svc.AdminReconcileOrder(context.Background(), actor, orderID.String(), "reconcile-"+uniqueSuffix()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := orderStatusOf(t, repo, orderID); got != "paid" {
		t.Errorf("status = %q, want paid", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 1 {
		t.Errorf("OrderPaid events = %d, want 1", got)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 0 {
		t.Errorf("late payment flags = %d, want 0", got)
	}
}

func TestLatePayment_ReconcileAfterWebhookDoesNotFlagTheSamePaymentTwice(t *testing.T) {
	svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-a"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix()); !errors.Is(err, ErrOrderNotPayable) {
			t.Fatalf("reconcile #%d err = %v, want ErrOrderNotPayable", i, err)
		}
	}

	if refs := lateFlagReferences(t, repo, orderID); len(refs) != 1 || refs[0] != "tx-a" {
		t.Errorf("late flag references = %v, want only [tx-a]", refs)
	}
}

func TestLatePayment_WebhookAfterReconcileAttributesTheExistingFlag(t *testing.T) {
	svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	if err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix()); !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("reconcile err = %v, want ErrOrderNotPayable", err)
	}

	for i := 0; i < 2; i++ {
		if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-b"), "sig", uuid.NewString()); err != nil {
			t.Fatalf("webhook #%d: %v", i, err)
		}
	}

	refs := lateFlagReferences(t, repo, orderID)
	if len(refs) != 1 || refs[0] != "tx-b" {
		t.Fatalf("late flag references = %v, want the reconcile flag attributed to [tx-b]", refs)
	}
	var source, notified string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT payload->>'source', payload->'notification'->>'transaction_id' FROM webhook_log
		 WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String(),
	).Scan(&source, &notified); err != nil {
		t.Fatalf("read flag: %v", err)
	}
	if notified != "tx-b" {
		t.Errorf("flag carries notification transaction %q, want tx-b so an operator can refund it", notified)
	}
}

func TestLatePayment_DistinctTransactionsAreFlaggedSeparately(t *testing.T) {
	svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	for _, tx := range []string{"tx-one", "tx-two", "tx-one"} {
		if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, tx), "sig", uuid.NewString()); err != nil {
			t.Fatalf("webhook %s: %v", tx, err)
		}
	}
	if err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix()); !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("reconcile err = %v, want ErrOrderNotPayable", err)
	}

	if refs := lateFlagReferences(t, repo, orderID); len(refs) != 2 || refs[0] != "tx-one" || refs[1] != "tx-two" {
		t.Errorf("late flag references = %v, want [tx-one tx-two]", refs)
	}
}

// blockLateFlagWrites makes the flag insert for one order fail, the way a
// database fault would, and returns the undo.

func TestWebhook_FailedLateFlagWriteIsNotAcknowledgedAndFulfilsNothing(t *testing.T) {
	svc, repo, orderID, _ := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	unblock := blockLateFlagWrites(t, repo, orderID)
	key := uuid.NewString()

	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-late"), "sig", key); err == nil {
		t.Fatal("webhook acknowledged a late settlement whose flag could not be written; the gateway would never redeliver it")
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 0 {
		t.Errorf("OrderPaid events = %d, want 0", got)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 0 {
		t.Errorf("late payment flags = %d, want 0 while the write fails", got)
	}

	unblock()
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-late"), "sig", key); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Errorf("late payment flags after redelivery = %d, want 1", got)
	}
}

func TestAdminReconcileOrder_FailedLateFlagWriteReturnsErrorAndFulfilsNothing(t *testing.T) {
	svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	unblock := blockLateFlagWrites(t, repo, orderID)

	err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix())
	if err == nil || errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("reconcile err = %v, want the flag write failure, not a clean refusal", err)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := orderPaidEvents(t, repo, orderID); got != 0 {
		t.Errorf("OrderPaid events = %d, want 0", got)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 0 {
		t.Errorf("late payment flags = %d, want 0 while the write fails", got)
	}

	unblock()
	if err := svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix()); !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("reconcile after the fault err = %v, want ErrOrderNotPayable", err)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Errorf("late payment flags = %d, want 1", got)
	}
}

func TestAdminRefundOrder_RefusesFlaggedExpiredOrder(t *testing.T) {
	svc, repo, orderID, _ := expiredOrderWithLateGatewayPayment(t)
	ctx := context.Background()
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-late"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Fatalf("late payment flags = %d, want 1", got)
	}

	actor := seedRevokeActor(t, repo)
	err := svc.AdminRefundOrder(ctx, actor.String(), orderID.String(), "refund_proof/"+uniqueSuffix()+".png")
	if !errors.Is(err, ErrOrderNotRefundable) {
		t.Fatalf("refund err = %v, want ErrOrderNotRefundable: the app holds no payment to reverse", err)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
}

func TestLatePayment_FlaggedOrderTargetCanBeBoughtAgain(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, product, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, orderID)
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-late"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if got := latePaymentFlags(t, repo, orderID); got != 1 {
		t.Fatalf("late payment flags = %d, want 1", got)
	}

	fresh := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), fresh.String(), checkoutKey()); err != nil {
		t.Fatalf("checkout of the flagged order's target: %v", err)
	}
	if got := orderStatusOf(t, repo, fresh); got != "payment_pending" {
		t.Errorf("fresh order status = %q, want payment_pending", got)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("flagged order status = %q, want payment_expired (final)", got)
	}
}

func blockLateFlagWrites(t *testing.T, repo *repository.Repository, orderID uuid.UUID) (unblock func()) {
	t.Helper()
	ctx := context.Background()
	name := "block_late_" + strings.ReplaceAll(orderID.String(), "-", "")[:12]
	for _, stmt := range []string{
		`CREATE FUNCTION ` + name + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late flag write blocked'; END $$`,
		`CREATE TRIGGER ` + name + ` BEFORE INSERT ON webhook_log FOR EACH ROW
		 WHEN (NEW.event_type = 'payment_late' AND NEW.gateway_ref = '` + orderID.String() + `') EXECUTE FUNCTION ` + name + `()`,
	} {
		if _, err := repo.Pool().Exec(ctx, stmt); err != nil {
			t.Fatalf("block flag writes: %v", err)
		}
	}
	undo := func() {
		_, _ = repo.Pool().Exec(ctx, `DROP TRIGGER IF EXISTS `+name+` ON webhook_log`)
		_, _ = repo.Pool().Exec(ctx, `DROP FUNCTION IF EXISTS `+name+`()`)
	}
	t.Cleanup(undo)
	return undo
}

func elapseDeadline(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE orders SET payment_expires_at = now() - interval '1 second' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("elapse deadline: %v", err)
	}
}

func expiredOrderWithLateGatewayPayment(t *testing.T) (*Service, *repository.Repository, uuid.UUID, string) {
	t.Helper()
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	actor := insertAdminActor(t, repo, "lateflag_")
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
	expireOrderNow(t, repo, orderID)
	pay.paid.Store(true)
	return svc, repo, orderID, actor
}

// Reconcile cannot learn the gateway's transaction id, so it flags under the
// order's gateway reference; that must not double-count a payment the webhook
// already flagged under its transaction id.

func lateFlagReferences(t *testing.T, repo *repository.Repository, orderID uuid.UUID) []string {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT payload->>'reference' FROM webhook_log
		 WHERE event_type = 'payment_late' AND gateway_ref = $1 ORDER BY payload->>'reference'`, orderID.String())
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

func latePaymentFlags(t *testing.T, repo *repository.Repository, orderID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM webhook_log WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String(),
	).Scan(&n); err != nil {
		t.Fatalf("count late flags: %v", err)
	}
	return n
}

func pendingMixedOrder(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) (orderID, book uuid.UUID) {
	t.Helper()
	orderID, book = mixedCart(t, svc, repo, student)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID, book
}

func stockOf(t *testing.T, repo *repository.Repository, productID uuid.UUID) int {
	t.Helper()
	var n int
	if err := repo.Pool().QueryRow(context.Background(), `SELECT stock FROM product WHERE id = $1`, productID).Scan(&n); err != nil {
		t.Fatalf("stock: %v", err)
	}
	return n
}

// mixedCart builds a cart of 2 books plus a paid course without checking out.

