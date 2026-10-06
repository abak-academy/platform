package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/repository"
)

// lifecyclePayments is a gateway fake whose signature always verifies, whose
// reconcile answer and creation failures are switchable, and which counts
// CreatePayment calls.
type lifecyclePayments struct {
	countingPaymentClient
	paid       atomic.Bool
	failCreate atomic.Bool
	onCreate   func()

	// expiresAfter, when set, is how long from now the gateway claims its link
	// lasts; lastExpiresIn records what ABAK asked for.
	expiresAfter  time.Duration
	lastExpiresIn atomic.Int64
	lastExpiresAt atomic.Value
}

func (p *lifecyclePayments) CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error) {
	if p.onCreate != nil {
		p.onCreate()
	}
	if p.failCreate.Load() {
		p.creates.Add(1)
		return PaymentResponse{}, errors.New("gateway down")
	}
	p.lastExpiresIn.Store(int64(req.ExpiresIn))
	p.lastExpiresAt.Store(req.ExpiresAt)
	resp, err := p.countingPaymentClient.CreatePayment(ctx, req)
	if err == nil {
		resp.SnapToken = "token-" + req.OrderID
		resp.PaymentURL = "https://payment.test/" + req.OrderID
	}
	if err == nil && p.expiresAfter > 0 {
		resp.ExpiresAt = time.Now().Add(p.expiresAfter)
	}
	return resp, err
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
			if got := latePaymentFlags(t, repo, orderID); got != 1 {
				t.Errorf("late payment flags = %d, want exactly 1 across repeated deliveries", got)
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
		wantFlags  int
	}{
		{name: "gateway paid on an expired order is refused and flagged once", actorTag: "reconcileguard_", expire: true, wantErr: ErrOrderNotPayable, wantStatus: "payment_expired", wantEvents: 0, wantFlags: 1},
		{name: "repeat on a paid order does not emit again", actorTag: "reconcilerepeat_", wantStatus: "paid", wantEvents: 1, wantFlags: 0},
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
			if got := latePaymentFlags(t, repo, orderID); got != tt.wantFlags {
				t.Errorf("late payment flags = %d, want %d", got, tt.wantFlags)
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

type settleVia string

const (
	viaWebhook   settleVia = "webhook"
	viaReconcile settleVia = "reconcile"
	viaConfirm   settleVia = "confirm"
)

func settle(t *testing.T, svc *Service, actor string, orderID uuid.UUID, via settleVia, txID, key string) error {
	t.Helper()
	ctx := context.Background()
	switch via {
	case viaWebhook:
		return svc.HandlePaymentWebhook(ctx, settlementBody(orderID, txID), "sig", key)
	case viaReconcile:
		return svc.AdminReconcileOrder(ctx, actor, orderID.String(), "reconcile-"+uniqueSuffix())
	case viaConfirm:
		return svc.AdminConfirmOrder(ctx, actor, orderID.String(), "confirm-"+uniqueSuffix(), "payment_proof/"+actor+"/"+uniqueSuffix()+".jpg")
	}
	t.Fatalf("unknown entry point %q", via)
	return nil
}

func errMatches(err, want error) bool {
	if want == nil {
		return err == nil
	}
	return errors.Is(err, want)
}

func TestSettlementPastDeadlineBeforeSweep_ExpiresOrderInsteadOfFulfilling(t *testing.T) {
	tests := []struct {
		name       string
		via        settleVia
		calls      int
		wantErr    error
		wantStatus string
		wantFlags  int
		wantStock  int
		sweepAfter bool
	}{
		{"webhook is acknowledged, expires the order and flags once", viaWebhook, 2, nil, "payment_expired", 1, 5, true},
		{"reconcile is refused, expires the order and flags once", viaReconcile, 2, ErrOrderNotPayable, "payment_expired", 1, 5, true},
		{"admin confirm is refused and leaves the order pending", viaConfirm, 1, ErrOrderNotPayable, "payment_pending", 0, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			ctx := context.Background()
			student := newDigitalStudent(t, repo)
			actor := insertAdminActor(t, repo, "pastdeadline_")
			orderID, book := pendingMixedOrder(t, svc, repo, student)
			elapseDeadline(t, repo, orderID)
			pay.paid.Store(true)
			methodBefore, proofBefore, _ := manualConfirmWrites(t, repo, orderID)

			for i := 0; i < tt.calls; i++ {
				if err := settle(t, svc, actor, orderID, tt.via, "tx-past", uuid.NewString()); !errMatches(err, tt.wantErr) {
					t.Fatalf("%s #%d err = %v, want %v", tt.via, i, err, tt.wantErr)
				}
			}

			if got := orderStatusOf(t, repo, orderID); got != tt.wantStatus {
				t.Errorf("status = %q, want %s: no grace after the deadline", got, tt.wantStatus)
			}
			if got := orderPaidEvents(t, repo, orderID); got != 0 {
				t.Errorf("OrderPaid events = %d, want 0", got)
			}
			if got := latePaymentFlags(t, repo, orderID); got != tt.wantFlags {
				t.Errorf("late payment flags = %d, want %d", got, tt.wantFlags)
			}
			if got := stockOf(t, repo, book); got != tt.wantStock {
				t.Errorf("book stock = %d, want %d", got, tt.wantStock)
			}
			method, proof, audits := manualConfirmWrites(t, repo, orderID)
			if method != methodBefore || proof != proofBefore || audits != 0 {
				t.Errorf("manual confirm writes = %q, %q, %d audits; want unchanged %q, %q and 0", method, proof, audits, methodBefore, proofBefore)
			}

			if !tt.sweepAfter {
				return
			}
			tx, err := repo.BeginTx(ctx)
			if err != nil {
				t.Fatalf("BeginTx: %v", err)
			}
			expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
			_ = tx.Rollback(ctx)
			if err != nil || expired {
				t.Errorf("sweeper transition afterwards = %v, %v; want false, nil (no second restock)", expired, err)
			}
			if got := stockOf(t, repo, book); got != tt.wantStock {
				t.Errorf("book stock after sweep = %d, want %d", got, tt.wantStock)
			}
		})
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

func TestSettlementBeforeDeadline_IsFulfilledAndNotFlagged(t *testing.T) {
	tests := []struct {
		name     string
		via      settleVia
		deadline string
	}{
		{"webhook with the full payment window", viaWebhook, ""},
		{"webhook seconds before the deadline", viaWebhook, "30 seconds"},
		{"reconcile seconds before the deadline", viaReconcile, "30 seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			actor := insertAdminActor(t, repo, "beforedeadline_")
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			if tt.deadline != "" {
				setDeadline(t, repo, orderID, tt.deadline)
			}
			pay.paid.Store(true)

			if err := settle(t, svc, actor, orderID, tt.via, "tx-ok", uuid.NewString()); err != nil {
				t.Fatalf("%s: %v", tt.via, err)
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

// Reconcile flags under the gateway reference (it cannot learn the transaction id) and must not double-count a webhook flag.
func TestLatePayment_OneFlagPerPaymentAcrossWebhookAndReconcile(t *testing.T) {
	type step struct {
		via  settleVia
		txID string
	}
	tests := []struct {
		name         string
		steps        []step
		wantRefs     []string
		wantNotified string
	}{
		{
			name:     "reconcile after webhook does not flag the same payment twice",
			steps:    []step{{viaWebhook, "tx-a"}, {viaReconcile, ""}, {viaReconcile, ""}},
			wantRefs: []string{"tx-a"},
		},
		{
			name:         "webhook after reconcile attributes the existing flag",
			steps:        []step{{viaReconcile, ""}, {viaWebhook, "tx-b"}, {viaWebhook, "tx-b"}},
			wantRefs:     []string{"tx-b"},
			wantNotified: "tx-b",
		},
		{
			name:     "distinct transactions are flagged separately",
			steps:    []step{{viaWebhook, "tx-one"}, {viaWebhook, "tx-two"}, {viaWebhook, "tx-one"}, {viaReconcile, ""}},
			wantRefs: []string{"tx-one", "tx-two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
			ctx := context.Background()

			for i, s := range tt.steps {
				var want error
				if s.via == viaReconcile {
					want = ErrOrderNotPayable
				}
				if err := settle(t, svc, actor, orderID, s.via, s.txID, uuid.NewString()); !errMatches(err, want) {
					t.Fatalf("step #%d %s err = %v, want %v", i, s.via, err, want)
				}
			}

			if refs := lateFlagReferences(t, repo, orderID); !slices.Equal(refs, tt.wantRefs) {
				t.Fatalf("late flag references = %v, want %v", refs, tt.wantRefs)
			}
			if tt.wantNotified == "" {
				return
			}
			var notified string
			if err := repo.Pool().QueryRow(ctx,
				`SELECT payload->'notification'->>'transaction_id' FROM webhook_log
				 WHERE event_type = 'payment_late' AND gateway_ref = $1`, orderID.String(),
			).Scan(&notified); err != nil {
				t.Fatalf("read flag: %v", err)
			}
			if notified != tt.wantNotified {
				t.Errorf("flag carries notification transaction %q, want %s so an operator can refund it", notified, tt.wantNotified)
			}
		})
	}
}

func TestLatePayment_FailedFlagWriteIsNotAcknowledgedAndFulfilsNothing(t *testing.T) {
	tests := []struct {
		name         string
		via          settleVia
		wantAfterErr error
	}{
		{"webhook is not acknowledged, so the gateway redelivers", viaWebhook, nil},
		{"reconcile returns the failure, not a clean refusal", viaReconcile, ErrOrderNotPayable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, orderID, actor := expiredOrderWithLateGatewayPayment(t)
			unblock := blockLateFlagWrites(t, repo, orderID)
			key := uuid.NewString()

			err := settle(t, svc, actor, orderID, tt.via, "tx-late", key)
			if err == nil || errors.Is(err, ErrOrderNotPayable) {
				t.Fatalf("%s err = %v, want the flag write failure, not an acknowledgement or a clean refusal", tt.via, err)
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
			if err := settle(t, svc, actor, orderID, tt.via, "tx-late", key); !errMatches(err, tt.wantAfterErr) {
				t.Fatalf("%s after the fault err = %v, want %v", tt.via, err, tt.wantAfterErr)
			}
			if got := latePaymentFlags(t, repo, orderID); got != 1 {
				t.Errorf("late payment flags after the fault = %d, want 1", got)
			}
		})
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

// blockLateFlagWrites makes one order's flag insert fail like a database fault and returns the undo.
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

// mixedCart builds a cart of 2 books plus a paid course without checking out.
func mixedCart(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) (orderID, book uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, 5, 'published', 300) RETURNING id`,
		"Book "+uuid.NewString(),
	).Scan(&book); err != nil {
		t.Fatalf("create book: %v", err)
	}
	course := seedCourseTarget(t, repo)
	digital := seedDigitalProduct(t, repo, "course", 50000, course)
	order, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, student.String(), order.ID.String(), book.String(), 2); err != nil {
		t.Fatalf("AddItem book: %v", err)
	}
	if err := svc.AddItem(ctx, student.String(), order.ID.String(), digital.String(), 1); err != nil {
		t.Fatalf("AddItem course: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET shipping_cost = 18000, selected_courier = 'JNE' WHERE id = $1`, order.ID); err != nil {
		t.Fatalf("set shipping: %v", err)
	}
	return order.ID, book
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
