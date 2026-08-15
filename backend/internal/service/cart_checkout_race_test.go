package service

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/repository"
)

// newCheckoutRaceTestService is newCheckoutTestService plus a courier spy, so
// the same service can both seed a priced courier quote via PatchCart (needs
// a matching GetRates response) and run real Checkout calls (needs redis +
// a payment client).
func newCheckoutRaceTestService(t *testing.T) (*Service, *repository.Repository) {
	t.Helper()
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	spy := &recordingLogisticsClient{rate: CourierRate{Courier: "JNE", Service: "REG", Price: 18000, CourierCode: "jne", ServiceCode: "reg"}}
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, spy, nil, nil, nil)
	return svc, repo
}

// TestPatchCart_RepoRejectsPatchAfterOrderLeavesCart covers the P1 finding's
// first half: a PATCH that lands after a concurrent checkout has already
// moved the order out of cart must not mutate it. repository.PatchCart is
// exercised directly (not through the service) because service.PatchCart's
// own pre-check already refuses a snapshot read as non-cart — the bug this
// pins is the repository UPDATE having no status predicate of its own, which
// only matters when the row changes status *after* the service's read and
// *before* the UPDATE executes, exactly the window a slow Save Address
// request racing a fast Checkout opens.
func TestPatchCart_RepoRejectsPatchAfterOrderLeavesCart(t *testing.T) {
	svc, repo := newDestinationChangeTestService(t)
	ctx := context.Background()
	studentID := insertCheckoutStudent(t, repo, "NonCart Patch Student", "noncartpatch_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	before, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (before): %v", err)
	}

	// Simulate a checkout that has already won the race and flipped the order
	// to payment_pending.
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("simulate concurrent checkout: %v", err)
	}

	err = repo.PatchCart(ctx, orderID, repository.OrderPatch{
		SelectedCourier: "JNT",
		SelectedService: "EXPRESS",
		ShippingCost:    99000,
		Total:           999000,
	})
	if !errors.Is(err, repository.ErrOrderNotEditable) {
		t.Fatalf("want repository.ErrOrderNotEditable, got %v", err)
	}

	after, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}
	if after.ShippingCost != before.ShippingCost {
		t.Errorf("shipping_cost mutated on a non-cart order: before=%v after=%v", before.ShippingCost, after.ShippingCost)
	}
	if after.Total != before.Total {
		t.Errorf("total mutated on a non-cart order: before=%v after=%v", before.Total, after.Total)
	}
	if after.SelectedCourier != before.SelectedCourier {
		t.Errorf("selected_courier mutated on a non-cart order: before=%q after=%q", before.SelectedCourier, after.SelectedCourier)
	}
}

// TestCheckout_ConcurrentPatchCartRace covers the P1 finding's second half:
// checkout and address invalidation (PatchCart) must not decide off two
// independent snapshots. For each of n independently-seeded physical carts,
// one goroutine races svc.Checkout against another racing repo.PatchCart
// (voiding the courier/shipping quote), both released off a shared start
// gate so the two operations genuinely overlap in Postgres.
//
// The two operations are mutually exclusive by construction once both sides
// hold the row lock correctly (Checkout's FOR UPDATE re-read, PatchCart's
// status='cart' predicate): whichever commits first determines the other's
// outcome, in either order —
//   - if the void commits first, Checkout's locked re-read sees the voided
//     courier and must reject with ErrShippingRequired, leaving the order in
//     'cart' (rolled back, unmutated by checkout);
//   - if Checkout's transaction commits first, the order is no longer
//     'cart', so PatchCart's own predicate then rejects the void.
//
// So exactly one of {checkout succeeded, void succeeded} must be true for
// every trial, and the persisted row must agree with whichever one won. That
// invariant is what the old code (validating a pre-transaction snapshot,
// PatchCart with no status predicate) could not hold: both could "win"
// independently, letting checkout create a payment from a quote that was
// already voided, or letting a late PATCH mutate an order checkout had
// already moved to payment_pending.
func TestCheckout_ConcurrentPatchCartRace(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()

	const n = 8

	type pair struct {
		orderID   uuid.UUID
		studentID string
	}
	pairs := make([]pair, n)
	for i := 0; i < n; i++ {
		studentID := insertCheckoutStudent(t, repo, "Checkout Race Student", "checkoutrace_")
		orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")
		pairs[i] = pair{orderID: orderID, studentID: studentID}
	}

	var ready sync.WaitGroup
	ready.Add(n * 2)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n * 2)

	checkoutErrs := make([]error, n)
	voidErrs := make([]error, n)

	for i, p := range pairs {
		i, p := i, p

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			current, gerr := repo.GetOrderByID(ctx, p.orderID)
			if gerr != nil {
				voidErrs[i] = gerr
				return
			}
			voidErrs[i] = repo.PatchCart(ctx, p.orderID, repository.OrderPatch{
				ShippingAddress:    current.ShippingAddress,
				SelectedCourier:    "",
				SelectedService:    "",
				PromoCodeID:        current.PromoCodeID,
				Discount:           current.Discount,
				ShippingCost:       0,
				Total:              current.Subtotal - current.Discount,
				ProvinceID:         current.ProvinceID,
				CityID:             current.CityID,
				DistrictID:         current.DistrictID,
				KodePos:            current.KodePos,
				IsEstimate:         false,
				CourierCode:        nil,
				CourierServiceCode: nil,
			})
		}()

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			_, cerr := svc.Checkout(ctx, p.studentID, p.orderID.String(), "checkout-race-key-"+p.orderID.String())
			checkoutErrs[i] = cerr
		}()
	}

	ready.Wait()
	close(start)
	done.Wait()

	for i, p := range pairs {
		got, err := repo.GetOrderByID(ctx, p.orderID)
		if err != nil {
			t.Fatalf("trial %d: GetOrderByID: %v", i, err)
		}

		checkoutWon := checkoutErrs[i] == nil
		voidWon := voidErrs[i] == nil

		if checkoutWon == voidWon {
			t.Errorf("trial %d: want exactly one of {checkout, void} to win, got checkoutErr=%v voidErr=%v", i, checkoutErrs[i], voidErrs[i])
			continue
		}

		if checkoutWon {
			if got.SelectedCourier == "" || got.ShippingCost <= 0 {
				t.Errorf("trial %d: checkout succeeded but persisted courier/shipping_cost was voided (courier=%q cost=%v) — checkout must have used a stale pre-lock snapshot", i, got.SelectedCourier, got.ShippingCost)
			}
			if got.Status == "cart" {
				t.Errorf("trial %d: checkout succeeded but order status is still cart", i)
			}
		} else {
			if !errors.Is(checkoutErrs[i], ErrShippingRequired) {
				t.Errorf("trial %d: void won but checkout error is %v, want ErrShippingRequired", i, checkoutErrs[i])
			}
			if got.Status != "cart" {
				t.Errorf("trial %d: void won but order status is %q, want cart (checkout must roll back)", i, got.Status)
			}
			if got.SelectedCourier != "" {
				t.Errorf("trial %d: void won but selected_courier is %q, want empty", i, got.SelectedCourier)
			}
		}
	}
}
