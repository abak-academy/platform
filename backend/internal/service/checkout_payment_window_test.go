package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/repository"
)

func deadlineOf(t *testing.T, repo *repository.Repository, orderID uuid.UUID) *time.Time {
	t.Helper()
	order, err := repo.GetOrderByID(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	return order.PaymentExpiresAt
}

func setDeadline(t *testing.T, repo *repository.Repository, orderID uuid.UUID, in string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`UPDATE orders SET payment_expires_at = now() + $2::interval WHERE id = $1`, orderID, in,
	); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
}

func sweepOrder(t *testing.T, repo *repository.Repository, orderID uuid.UUID) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	expired, err := repo.ExpirePaymentOrderTx(ctx, tx, orderID)
	if err != nil {
		t.Fatalf("ExpirePaymentOrderTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return expired
}

func TestCheckout_GivesThePendingOrderABoundedDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil {
		t.Fatal("payment_expires_at is NULL after checkout")
	}
	if left := time.Until(*deadline); left < 19*time.Minute || left > 21*time.Minute {
		t.Errorf("time left = %v, want about 20m", left)
	}
	if asked := time.Duration(pay.lastExpiresIn.Load()); asked > 20*time.Minute-paymentSessionMargin || asked < 17*time.Minute {
		t.Errorf("gateway asked to keep the link for %v, want no more than the order window less the %v margin", asked, paymentSessionMargin)
	}
}

func TestCheckout_GatewayFailureStillExpiresAndFreesStock(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, book := mixedCart(t, svc, repo, student)
	pay.failCreate.Store(true)

	if _, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey()); err == nil {
		t.Fatal("Checkout with a failing gateway succeeded")
	}
	if deadlineOf(t, repo, orderID) == nil {
		t.Fatal("payment_expires_at is NULL after a gateway failure: the sweeper can never free this order")
	}
	if got := stockOf(t, repo, book); got != 3 {
		t.Fatalf("stock after checkout = %d, want 3", got)
	}

	elapseDeadline(t, repo, orderID)
	if !sweepOrder(t, repo, orderID) {
		t.Fatal("sweeper transition did not expire the order")
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5", got)
	}
}

// A legacy order's session end comes from the app clock before the gateway call, its deadline from the database clock after it.
const legacyClockSkew = time.Second

func TestRetryPayment_KeepsTheOrdersDeadline(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		deadline string
		maxAsked time.Duration
		exactAsk bool
		skew     time.Duration
	}{
		{"existing deadline is never extended, even by a gateway offering 72h", "2 hours", 2*time.Hour - paymentSessionMargin, true, 0},
		{"deadline a couple of minutes past the margin still gets a session", "4 minutes", 4*time.Minute - paymentSessionMargin, true, 0},
		{"order without a deadline gets one on the first retry and keeps it", "", 20*time.Minute - paymentSessionMargin, false, legacyClockSkew},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID, _, _ := pendingCourseOrder(t, svc, repo, student)
			if tt.deadline != "" {
				setDeadline(t, repo, orderID, tt.deadline)
			} else if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET payment_expires_at = NULL, gateway_ref = NULL WHERE id = $1`, orderID); err != nil {
				t.Fatalf("simulate a pre-deadline order: %v", err)
			}
			clearPaymentForRetry(t, svc, repo, orderID)
			original := deadlineOf(t, repo, orderID)
			pay.expiresAfter = 72 * time.Hour

			for i := 0; i < 3; i++ {
				result, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())
				if err != nil {
					t.Fatalf("retry #%d: %v", i, err)
				}
				got := deadlineOf(t, repo, orderID)
				if got == nil {
					t.Fatalf("retry #%d left the order without a deadline", i)
				}
				if original == nil {
					if left := time.Until(*got); left < 19*time.Minute || left > 21*time.Minute {
						t.Errorf("retry #%d: time left = %v, want about 20m", i, left)
					}
					original = got
				}
				if !got.Equal(*original) {
					t.Errorf("retry #%d moved the deadline from %v to %v", i, *original, *got)
				}
				if result.PaymentExpiresAt.After(*original) {
					t.Errorf("retry #%d advertises %v, past the order deadline %v", i, result.PaymentExpiresAt, *original)
				}
				if askedFor := time.Duration(pay.lastExpiresIn.Load()); askedFor > tt.maxAsked {
					t.Errorf("retry #%d asked the gateway for %v, more than the %v left", i, askedFor, tt.maxAsked)
				}
				asked := askedDeadline(t, pay)
				wantEnd := original.Add(-paymentSessionMargin)
				if tt.exactAsk && !asked.Equal(wantEnd) {
					t.Errorf("retry #%d asked the gateway to end at %v, want the deadline %v less the %v margin", i, asked, *original, paymentSessionMargin)
				}
				if asked.After(wantEnd.Add(tt.skew)) {
					t.Errorf("retry #%d asked the gateway to end at %v, later than %v (deadline %v less the %v margin)", i, asked, wantEnd, *original, paymentSessionMargin)
				}
			}
		})
	}
}

func TestCheckout_OrderThatExpiresDuringTheGatewayCallGetsNoPaymentLink(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	orderID, book := mixedCart(t, svc, repo, student)
	pay.onCreate = func() {
		pay.onCreate = nil
		elapseDeadline(t, repo, orderID)
		sweepOrder(t, repo, orderID)
	}

	result, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey())

	if !errors.Is(err, ErrPaymentExpired) {
		t.Fatalf("err = %v, want ErrPaymentExpired", err)
	}
	if result.GatewayRef != "" || result.PaymentURL != "" || result.SnapToken != "" {
		t.Errorf("result = %+v, want no payment link for an expired order", result)
	}
	if got := gatewayRefOf(t, repo, orderID); got != "" {
		t.Errorf("gateway ref = %q, want none attached to an expired order", got)
	}
	if got := orderStatusOf(t, repo, orderID); got != "payment_expired" {
		t.Errorf("status = %q, want payment_expired", got)
	}
	if got := stockOf(t, repo, book); got != 5 {
		t.Errorf("book stock = %d, want 5", got)
	}
}

func TestCheckout_AdvertisedExpiryNeverExceedsTheOrderDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	pay.expiresAfter = 72 * time.Hour
	course := seedCourseTarget(t, repo)
	orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, course))

	result, err := svc.Checkout(ctx, student.String(), orderID.String(), checkoutKey())
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil || result.PaymentExpiresAt.After(*deadline) {
		t.Errorf("advertised expiry %v, order deadline %v: the link must not outlive the order", result.PaymentExpiresAt, deadline)
	}
}

func TestCheckout_CachedReplayNeverOffersAnExpiredOrAnotherStudentsOrder(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		change       func(t *testing.T, repo *repository.Repository, orderID uuid.UUID)
		otherStudent bool
		wantErr      error
		wantSameRef  bool
	}{
		{"unchanged order replays the first response", nil, false, nil, true},
		{"another student replaying the key", nil, true, ErrOrderNotFound, false},
		{"expired order is refused with no reference", expireOrderNow, false, ErrPaymentExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
			key := checkoutKey()
			first, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
			if err != nil {
				t.Fatalf("Checkout: %v", err)
			}
			if tt.change != nil {
				tt.change(t, repo, orderID)
			}
			caller := student
			if tt.otherStudent {
				caller = newDigitalStudent(t, repo)
			}
			before := pay.creates.Load()

			replay, err := svc.Checkout(ctx, caller.String(), orderID.String(), key)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("replay err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantSameRef && replay.GatewayRef != first.GatewayRef {
				t.Errorf("replay ref = %q, want the first call's %q", replay.GatewayRef, first.GatewayRef)
			}
			if !tt.wantSameRef && replay.GatewayRef != "" {
				t.Errorf("replay returned ref %q, want none", replay.GatewayRef)
			}
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls during replay = %d, want 0", got)
			}
		})
	}
}

func TestCheckout_ReusedKeyNeverHandsOutAnotherOrdersReference(t *testing.T) {
	tests := []struct {
		name        string
		sameStudent bool
	}{
		{"one key used on two orders of one student", true},
		{"one key used by two students", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newLifecycleService(t)
			ctx := context.Background()
			studentA := newDigitalStudent(t, repo)
			orderA := cartOf(t, svc, studentA, seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)))
			key := checkoutKey()
			first, err := svc.Checkout(ctx, studentA.String(), orderA.String(), key)
			if err != nil {
				t.Fatalf("Checkout A: %v", err)
			}
			studentB := studentA
			if !tt.sameStudent {
				studentB = newDigitalStudent(t, repo)
			}
			orderB, _, _ := pendingCourseOrder(t, svc, repo, studentB)

			got, err := svc.Checkout(ctx, studentB.String(), orderB.String(), key)

			if got.GatewayRef != "" || got.GatewayRef == first.GatewayRef {
				t.Errorf("order B was handed reference %q, which belongs to order A", got.GatewayRef)
			}
			if !errors.Is(err, ErrOrderNotEditable) {
				t.Errorf("err = %v, want ErrOrderNotEditable: order B is already checked out", err)
			}
		})
	}
}

func TestCheckout_FreeOrderReplayStillReturnsFreeWithoutGateway(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	orderID := cartOf(t, svc, student, seedDigitalProduct(t, repo, "course", 0, course))
	key := checkoutKey()

	for i := 0; i < 2; i++ {
		result, err := svc.Checkout(ctx, student.String(), orderID.String(), key)
		if err != nil || !result.Free {
			t.Fatalf("free checkout #%d = %+v, %v; want Free", i, result, err)
		}
	}
	if got := pay.creates.Load(); got != 0 {
		t.Errorf("gateway calls = %d, want 0", got)
	}
}

func askedDeadline(t *testing.T, pay *lifecyclePayments) time.Time {
	t.Helper()
	got, _ := pay.lastExpiresAt.Load().(time.Time)
	return got
}

// The gateway session is pinned to the order's own database deadline less the
// session margin, not to a duration counted from whenever the customer later charges.
func TestCheckout_AsksTheGatewayToEndAtTheOrderDeadline(t *testing.T) {
	svc, repo, pay := newLifecycleService(t)
	student := newDigitalStudent(t, repo)
	orderID, _, _ := pendingCourseOrder(t, svc, repo, student)

	deadline := deadlineOf(t, repo, orderID)
	if deadline == nil {
		t.Fatal("payment_expires_at is NULL after checkout")
	}
	if got, want := askedDeadline(t, pay), deadline.Add(-paymentSessionMargin); !got.Equal(want) {
		t.Errorf("gateway asked to end at %v, want %v (order deadline %v less the %v margin)", got, want, *deadline, paymentSessionMargin)
	}
}
