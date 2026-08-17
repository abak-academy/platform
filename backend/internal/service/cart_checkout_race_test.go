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

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// insertRaceTestProduct seeds a product row for the item-mutation-vs-checkout
// tests below, which call repository.AddItem/RemoveItem/UpdateItemQty
// directly (bypassing the service layer, same as the PatchCart race above) so
// the mutation is not stopped by the service's own pre-transaction status
// check — the point is to prove the repository's own lock holds.
func insertRaceTestProduct(t *testing.T, repo *repository.Repository, productType string, price, weightGrams int) string {
	t.Helper()
	var id string
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ($1, $2, $3, 100, 'published', $4) RETURNING id`,
		productType, "Item Race Product "+uuid.New().String(), price, weightGrams,
	).Scan(&id); err != nil {
		t.Fatalf("create product: %v", err)
	}
	return id
}

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

// TestAddItem_RepoRejectsAfterOrderLeavesCart, TestUpdateItemQty_..., and
// TestRemoveItem_... cover the item-mutation half of the same P1 finding:
// repository.AddItem/UpdateItemQty/RemoveItem must acquire the same orders
// FOR UPDATE lock as PatchCart/Checkout and refuse to touch order_item once
// the row has left 'cart' — deterministic before/after checks, same shape as
// TestPatchCart_RepoRejectsPatchAfterOrderLeavesCart above. RemoveItem is the
// one the finding calls out specifically: pre-fix it has no status predicate
// at all, so the DELETE goes through even on a payment_pending order.
func TestAddItem_RepoRejectsAfterOrderLeavesCart(t *testing.T) {
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()
	studentID := insertCheckoutStudent(t, repo, "NonCart Add Student", "noncartadd_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	before, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (before): %v", err)
	}

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("simulate concurrent checkout: %v", err)
	}

	productID := insertRaceTestProduct(t, repo, "book", 20000, 500)
	pID, err := uuid.Parse(productID)
	if err != nil {
		t.Fatalf("parse productID: %v", err)
	}

	err = repo.AddItem(ctx, orderID, model.OrderItem{
		ProductID:   pID,
		ProductType: "book",
		Name:        "Late Add Book",
		UnitPrice:   20000,
		Qty:         1,
		WeightGrams: 500,
	}, true)
	if !errors.Is(err, repository.ErrOrderNotEditable) {
		t.Fatalf("want repository.ErrOrderNotEditable, got %v", err)
	}

	after, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}
	if len(after.Items) != len(before.Items) {
		t.Errorf("item count changed on a non-cart order: before=%d after=%d", len(before.Items), len(after.Items))
	}
	if after.Subtotal != before.Subtotal || after.Total != before.Total {
		t.Errorf("totals mutated on a non-cart order: before subtotal=%v total=%v, after subtotal=%v total=%v",
			before.Subtotal, before.Total, after.Subtotal, after.Total)
	}
}

func TestUpdateItemQty_RepoRejectsAfterOrderLeavesCart(t *testing.T) {
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()
	studentID := insertCheckoutStudent(t, repo, "NonCart Qty Student", "noncartqty_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	before, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (before): %v", err)
	}
	itemID := before.Items[0].ID

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("simulate concurrent checkout: %v", err)
	}

	err = repo.UpdateItemQty(ctx, orderID, itemID, 5, false)
	if !errors.Is(err, repository.ErrOrderNotEditable) {
		t.Fatalf("want repository.ErrOrderNotEditable, got %v", err)
	}

	after, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}
	if after.Items[0].Qty != before.Items[0].Qty {
		t.Errorf("qty mutated on a non-cart order: before=%d after=%d", before.Items[0].Qty, after.Items[0].Qty)
	}
	if after.Subtotal != before.Subtotal || after.Total != before.Total {
		t.Errorf("totals mutated on a non-cart order: before subtotal=%v total=%v, after subtotal=%v total=%v",
			before.Subtotal, before.Total, after.Subtotal, after.Total)
	}
}

func TestRemoveItem_RepoRejectsAfterOrderLeavesCart(t *testing.T) {
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()
	studentID := insertCheckoutStudent(t, repo, "NonCart Remove Student", "noncartrm_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	before, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (before): %v", err)
	}
	itemID := before.Items[0].ID

	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET status = 'payment_pending' WHERE id = $1`, orderID,
	); err != nil {
		t.Fatalf("simulate concurrent checkout: %v", err)
	}

	err = repo.RemoveItem(ctx, orderID, itemID, true)
	if !errors.Is(err, repository.ErrOrderNotEditable) {
		t.Fatalf("want repository.ErrOrderNotEditable, got %v", err)
	}

	after, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}
	if len(after.Items) != len(before.Items) {
		t.Fatalf("item deleted from a non-cart order: before=%d items, after=%d items", len(before.Items), len(after.Items))
	}
	if after.Subtotal != before.Subtotal || after.Total != before.Total {
		t.Errorf("totals mutated on a non-cart order: before subtotal=%v total=%v, after subtotal=%v total=%v",
			before.Subtotal, before.Total, after.Subtotal, after.Total)
	}
}

// TestCheckout_ConcurrentAddItemRace, TestCheckout_ConcurrentUpdateItemQtyRace,
// and TestCheckout_ConcurrentRemoveItemRace cover the item-mutation half of
// the same concurrency invariant as TestCheckout_ConcurrentPatchCartRace
// above: checkout and a racing item mutation must not decide off two
// independent snapshots. Each racing mutation is built to be adversarial to
// checkout the same way the PatchCart void is — AddItem/RemoveItem clear the
// shipping quote (clearShipping=true) so a checkout that observes the
// cleared quote must reject with ErrShippingRequired; UpdateItemQty bumps a
// digital item's qty past its qty-1 limit so a checkout that observes the
// bumped qty must reject with ErrDigitalQtyLimit. Either way exactly one of
// {checkout, mutation} must win, and the persisted order's subtotal must
// always equal the sum of its persisted order_item rows — no torn state
// where a payment is created for an amount that no longer matches the items.
func TestCheckout_ConcurrentAddItemRace(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()

	const n = 8

	type pair struct {
		orderID   uuid.UUID
		studentID string
		productID uuid.UUID
	}
	pairs := make([]pair, n)
	for i := 0; i < n; i++ {
		studentID := insertCheckoutStudent(t, repo, "Checkout AddItem Race Student", "checkoutaddrace_")
		orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")
		productID := insertRaceTestProduct(t, repo, "book", 15000, 300)
		pID, err := uuid.Parse(productID)
		if err != nil {
			t.Fatalf("parse productID: %v", err)
		}
		pairs[i] = pair{orderID: orderID, studentID: studentID, productID: pID}
	}

	var ready sync.WaitGroup
	ready.Add(n * 2)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n * 2)

	checkoutErrs := make([]error, n)
	addErrs := make([]error, n)

	for i, p := range pairs {
		i, p := i, p

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			addErrs[i] = repo.AddItem(ctx, p.orderID, model.OrderItem{
				ProductID:   p.productID,
				ProductType: "book",
				Name:        "Race Add Book",
				UnitPrice:   15000,
				Qty:         1,
				WeightGrams: 300,
			}, true)
		}()

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			_, cerr := svc.Checkout(ctx, p.studentID, p.orderID.String(), "checkout-add-race-key-"+p.orderID.String())
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
		addWon := addErrs[i] == nil

		if checkoutWon == addWon {
			t.Errorf("trial %d: want exactly one of {checkout, add} to win, got checkoutErr=%v addErr=%v", i, checkoutErrs[i], addErrs[i])
			continue
		}

		if checkoutWon {
			if !errors.Is(addErrs[i], repository.ErrOrderNotEditable) {
				t.Errorf("trial %d: checkout won but add error is %v, want repository.ErrOrderNotEditable", i, addErrs[i])
			}
			if len(got.Items) != 1 {
				t.Errorf("trial %d: checkout won but item count is %d, want 1 (add must not have applied)", i, len(got.Items))
			}
			if got.Status == "cart" {
				t.Errorf("trial %d: checkout succeeded but order status is still cart", i)
			}
		} else {
			if !errors.Is(checkoutErrs[i], ErrShippingRequired) {
				t.Errorf("trial %d: add won but checkout error is %v, want ErrShippingRequired", i, checkoutErrs[i])
			}
			if got.Status != "cart" {
				t.Errorf("trial %d: add won but order status is %q, want cart (checkout must roll back)", i, got.Status)
			}
			if len(got.Items) != 2 {
				t.Errorf("trial %d: add won but item count is %d, want 2", i, len(got.Items))
			}
		}

		expectedSubtotal := 0.0
		for _, item := range got.Items {
			expectedSubtotal += item.Jumlah
		}
		if got.Subtotal != expectedSubtotal {
			t.Errorf("trial %d: persisted subtotal %v does not match sum of persisted items %v — torn state", i, got.Subtotal, expectedSubtotal)
		}
		if got.Total != expectedSubtotal-got.Discount+got.ShippingCost {
			t.Errorf("trial %d: persisted total %v does not match subtotal-discount+shipping (%v) — torn state",
				i, got.Total, expectedSubtotal-got.Discount+got.ShippingCost)
		}
	}
}

func TestCheckout_ConcurrentUpdateItemQtyRace(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()

	const n = 8

	type pair struct {
		orderID   uuid.UUID
		studentID string
		itemID    uuid.UUID
	}
	pairs := make([]pair, n)
	for i := 0; i < n; i++ {
		studentID := insertCheckoutStudent(t, repo, "Checkout Qty Race Student", "checkoutqtyrace_")
		productID := insertRaceTestProduct(t, repo, "course", 25000, 0)

		order, _, err := svc.MintCart(ctx, studentID)
		if err != nil {
			t.Fatalf("MintCart: %v", err)
		}
		if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		seeded, err := repo.GetOrderByID(ctx, order.ID)
		if err != nil {
			t.Fatalf("GetOrderByID (seed check): %v", err)
		}
		pairs[i] = pair{orderID: order.ID, studentID: studentID, itemID: seeded.Items[0].ID}
	}

	var ready sync.WaitGroup
	ready.Add(n * 2)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n * 2)

	checkoutErrs := make([]error, n)
	updateErrs := make([]error, n)

	for i, p := range pairs {
		i, p := i, p

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			updateErrs[i] = repo.UpdateItemQty(ctx, p.orderID, p.itemID, 2, false)
		}()

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			_, cerr := svc.Checkout(ctx, p.studentID, p.orderID.String(), "checkout-qty-race-key-"+p.orderID.String())
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
		updateWon := updateErrs[i] == nil

		if checkoutWon == updateWon {
			t.Errorf("trial %d: want exactly one of {checkout, update} to win, got checkoutErr=%v updateErr=%v", i, checkoutErrs[i], updateErrs[i])
			continue
		}

		if checkoutWon {
			if !errors.Is(updateErrs[i], repository.ErrOrderNotEditable) {
				t.Errorf("trial %d: checkout won but update error is %v, want repository.ErrOrderNotEditable", i, updateErrs[i])
			}
			if got.Items[0].Qty != 1 {
				t.Errorf("trial %d: checkout won but qty is %d, want 1 (update must not have applied)", i, got.Items[0].Qty)
			}
			if got.Status == "cart" {
				t.Errorf("trial %d: checkout succeeded but order status is still cart", i)
			}
		} else {
			if !errors.Is(checkoutErrs[i], ErrDigitalQtyLimit) {
				t.Errorf("trial %d: update won but checkout error is %v, want ErrDigitalQtyLimit", i, checkoutErrs[i])
			}
			if got.Status != "cart" {
				t.Errorf("trial %d: update won but order status is %q, want cart (checkout must roll back)", i, got.Status)
			}
			if got.Items[0].Qty != 2 {
				t.Errorf("trial %d: update won but qty is %d, want 2", i, got.Items[0].Qty)
			}
		}

		expectedSubtotal := 0.0
		for _, item := range got.Items {
			expectedSubtotal += item.Jumlah
		}
		if got.Subtotal != expectedSubtotal {
			t.Errorf("trial %d: persisted subtotal %v does not match sum of persisted items %v — torn state", i, got.Subtotal, expectedSubtotal)
		}
	}
}

func TestCheckout_ConcurrentRemoveItemRace(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()

	const n = 8

	type pair struct {
		orderID    uuid.UUID
		studentID  string
		removeItem uuid.UUID
	}
	pairs := make([]pair, n)
	for i := 0; i < n; i++ {
		studentID := insertCheckoutStudent(t, repo, "Checkout Remove Race Student", "checkoutrmrace_")

		productAID := insertRaceTestProduct(t, repo, "book", 20000, 500)
		productBID := insertRaceTestProduct(t, repo, "book", 30000, 700)

		order, _, err := svc.MintCart(ctx, studentID)
		if err != nil {
			t.Fatalf("MintCart: %v", err)
		}
		if err := svc.AddItem(ctx, studentID, order.ID.String(), productAID, 1); err != nil {
			t.Fatalf("AddItem A: %v", err)
		}
		if err := svc.AddItem(ctx, studentID, order.ID.String(), productBID, 1); err != nil {
			t.Fatalf("AddItem B: %v", err)
		}
		if err := svc.PatchCart(ctx, studentID, order.ID.String(), CartPatch{
			Courier:    "JNE",
			Service:    "REG",
			ProvinceID: strPtr("93"),
			CityID:     strPtr("9301"),
			DistrictID: strPtr("930101"),
			KodePos:    strPtr("12345"),
		}); err != nil {
			t.Fatalf("PatchCart (seed courier quote): %v", err)
		}

		seeded, err := repo.GetOrderByID(ctx, order.ID)
		if err != nil {
			t.Fatalf("GetOrderByID (seed check): %v", err)
		}
		if seeded.SelectedCourier == "" || seeded.ShippingCost <= 0 {
			t.Fatalf("seed: want selected_courier set and shipping_cost>0, got %q / %v", seeded.SelectedCourier, seeded.ShippingCost)
		}
		if len(seeded.Items) != 2 {
			t.Fatalf("seed: want 2 items, got %d", len(seeded.Items))
		}

		pairs[i] = pair{orderID: order.ID, studentID: studentID, removeItem: seeded.Items[0].ID}
	}

	var ready sync.WaitGroup
	ready.Add(n * 2)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n * 2)

	checkoutErrs := make([]error, n)
	removeErrs := make([]error, n)

	for i, p := range pairs {
		i, p := i, p

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			removeErrs[i] = repo.RemoveItem(ctx, p.orderID, p.removeItem, true)
		}()

		go func() {
			defer done.Done()
			ready.Done()
			<-start
			_, cerr := svc.Checkout(ctx, p.studentID, p.orderID.String(), "checkout-remove-race-key-"+p.orderID.String())
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
		removeWon := removeErrs[i] == nil

		if checkoutWon == removeWon {
			t.Errorf("trial %d: want exactly one of {checkout, remove} to win, got checkoutErr=%v removeErr=%v", i, checkoutErrs[i], removeErrs[i])
			continue
		}

		if checkoutWon {
			if !errors.Is(removeErrs[i], repository.ErrOrderNotEditable) {
				t.Errorf("trial %d: checkout won but remove error is %v, want repository.ErrOrderNotEditable", i, removeErrs[i])
			}
			if len(got.Items) != 2 {
				t.Errorf("trial %d: checkout won but item count is %d, want 2 (remove must not have applied)", i, len(got.Items))
			}
			if got.Status == "cart" {
				t.Errorf("trial %d: checkout succeeded but order status is still cart", i)
			}
		} else {
			if !errors.Is(checkoutErrs[i], ErrShippingRequired) {
				t.Errorf("trial %d: remove won but checkout error is %v, want ErrShippingRequired", i, checkoutErrs[i])
			}
			if got.Status != "cart" {
				t.Errorf("trial %d: remove won but order status is %q, want cart (checkout must roll back)", i, got.Status)
			}
			if len(got.Items) != 1 {
				t.Errorf("trial %d: remove won but item count is %d, want 1", i, len(got.Items))
			}
		}

		expectedSubtotal := 0.0
		for _, item := range got.Items {
			expectedSubtotal += item.Jumlah
		}
		if got.Subtotal != expectedSubtotal {
			t.Errorf("trial %d: persisted subtotal %v does not match sum of persisted items %v — torn state", i, got.Subtotal, expectedSubtotal)
		}
	}
}
