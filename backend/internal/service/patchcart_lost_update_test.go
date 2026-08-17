package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// staleOrderPatchFrom builds a repository.OrderPatch exactly the way
// service.PatchCart's own PatchCart would from stale — a read taken before a
// concurrent item mutation commits — carrying stale's own updated_at as the
// optimistic-concurrency token. This is the shape a "keep everything,
// nothing courier-related in this PATCH" request racing a concurrent item
// mutation would produce.
func staleOrderPatchFrom(stale model.Order) repository.OrderPatch {
	updatedAt := stale.UpdatedAt
	return repository.OrderPatch{
		ShippingAddress:    stale.ShippingAddress,
		SelectedCourier:    stale.SelectedCourier,
		SelectedService:    stale.SelectedService,
		CourierCode:        stale.CourierCode,
		CourierServiceCode: stale.CourierServiceCode,
		IsEstimate:         stale.IsEstimate,
		PromoCodeID:        stale.PromoCodeID,
		Discount:           stale.Discount,
		ShippingCost:       stale.ShippingCost,
		Total:              stale.Subtotal - stale.Discount + stale.ShippingCost,
		ProvinceID:         stale.ProvinceID,
		CityID:             stale.CityID,
		DistrictID:         stale.DistrictID,
		KodePos:            stale.KodePos,
		ExpectedUpdatedAt:  &updatedAt,
	}
}

// TestPatchCart_LostUpdate_ConcurrentDigitalAddItem pins the P1 finding's
// worst case: a digital AddItem commits between PatchCart's read and its
// write. AddItem correctly recomputes subtotal and total together; a
// PatchCart built from the pre-add read must not be allowed to overwrite
// total alone with a figure derived from the stale subtotal. Pre-fix,
// repository.PatchCart has no optimistic-concurrency check, so this stale
// write succeeds and total ends up below the persisted (post-add) subtotal.
func TestPatchCart_LostUpdate_ConcurrentDigitalAddItem(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Lost Update Add Student", "lostupdadd_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	stale, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (stale): %v", err)
	}

	digitalProductID := insertRaceTestProduct(t, repo, "course", 25000, 0)
	pID, err := uuid.Parse(digitalProductID)
	if err != nil {
		t.Fatalf("parse digitalProductID: %v", err)
	}
	if err := repo.AddItem(ctx, orderID, model.OrderItem{
		ProductID:   pID,
		ProductType: "course",
		Name:        "Concurrent Digital Add",
		UnitPrice:   25000,
		Qty:         1,
	}, false); err != nil {
		t.Fatalf("concurrent AddItem: %v", err)
	}

	patchErr := repo.PatchCart(ctx, orderID, staleOrderPatchFrom(stale))

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}

	if len(got.Items) != 2 {
		t.Fatalf("want 2 items (physical + concurrently-added digital), got %d", len(got.Items))
	}
	wantSubtotal := 0.0
	for _, item := range got.Items {
		wantSubtotal += item.Jumlah
	}
	if got.Subtotal != wantSubtotal {
		t.Fatalf("persisted subtotal %v does not match sum of items %v", got.Subtotal, wantSubtotal)
	}
	if got.Total != got.Subtotal-got.Discount+got.ShippingCost {
		t.Errorf("lost update: total %v != subtotal(%v) - discount(%v) + shipping(%v)",
			got.Total, got.Subtotal, got.Discount, got.ShippingCost)
	}
	if !errors.Is(patchErr, repository.ErrOrderChanged) {
		t.Errorf("want repository.ErrOrderChanged from a PatchCart built off a stale read, got %v", patchErr)
	}
}

// TestPatchCart_LostUpdate_ConcurrentUpdateItemQty covers the qty-change
// mutation kind: a concurrent qty bump on the cart's only physical item
// changes its weight, so the mutation clears the shipping quote alongside
// the totals. A PatchCart built from before the bump must not be allowed to
// restore the stale (pre-bump-weight) shipping quote on top of the new qty.
func TestPatchCart_LostUpdate_ConcurrentUpdateItemQty(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Lost Update Qty Student", "lostupdqty_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	stale, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (stale): %v", err)
	}
	itemID := stale.Items[0].ID

	if err := repo.UpdateItemQty(ctx, orderID, itemID, 3, true); err != nil {
		t.Fatalf("concurrent UpdateItemQty: %v", err)
	}

	patchErr := repo.PatchCart(ctx, orderID, staleOrderPatchFrom(stale))

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}

	if got.Items[0].Qty != 3 {
		t.Fatalf("want qty 3 to survive, got %d", got.Items[0].Qty)
	}
	if got.Total != got.Subtotal-got.Discount+got.ShippingCost {
		t.Errorf("lost update: total %v != subtotal(%v) - discount(%v) + shipping(%v)",
			got.Total, got.Subtotal, got.Discount, got.ShippingCost)
	}
	if got.ShippingCost != 0 || got.SelectedCourier != "" {
		t.Errorf("lost update: stale PatchCart restored a shipping quote priced for the old weight (courier=%q cost=%v)",
			got.SelectedCourier, got.ShippingCost)
	}
	if !errors.Is(patchErr, repository.ErrOrderChanged) {
		t.Errorf("want repository.ErrOrderChanged from a PatchCart built off a stale read, got %v", patchErr)
	}
}

// TestPatchCart_LostUpdate_ConcurrentRemoveItem covers the remove mutation
// kind: removing one of two physical items changes total weight, so the
// mutation clears the shipping quote too. A PatchCart built from before the
// remove must not be allowed to restore the stale shipping quote.
func TestPatchCart_LostUpdate_ConcurrentRemoveItem(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Lost Update Remove Student", "lostupdrm_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	secondProductID := insertRaceTestProduct(t, repo, "book", 30000, 700)
	if err := svc.AddItem(ctx, studentID, orderID.String(), secondProductID, 1); err != nil {
		t.Fatalf("seed second physical item: %v", err)
	}
	// Re-quote so the seeded quote reflects both items' combined weight
	// before the "concurrent" remove below.
	if err := svc.PatchCart(ctx, studentID, orderID.String(), CartPatch{
		Courier:    "JNE",
		Service:    "REG",
		ProvinceID: strPtr("93"),
		CityID:     strPtr("9301"),
		DistrictID: strPtr("930101"),
		KodePos:    strPtr("12345"),
	}); err != nil {
		t.Fatalf("re-quote after seeding second item: %v", err)
	}

	stale, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (stale): %v", err)
	}
	if len(stale.Items) != 2 {
		t.Fatalf("seed: want 2 items, got %d", len(stale.Items))
	}
	removeItemID := stale.Items[0].ID

	if err := repo.RemoveItem(ctx, orderID, removeItemID, true); err != nil {
		t.Fatalf("concurrent RemoveItem: %v", err)
	}

	patchErr := repo.PatchCart(ctx, orderID, staleOrderPatchFrom(stale))

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID (after): %v", err)
	}

	if len(got.Items) != 1 {
		t.Fatalf("want 1 item after concurrent remove, got %d", len(got.Items))
	}
	if got.Total != got.Subtotal-got.Discount+got.ShippingCost {
		t.Errorf("lost update: total %v != subtotal(%v) - discount(%v) + shipping(%v)",
			got.Total, got.Subtotal, got.Discount, got.ShippingCost)
	}
	if got.ShippingCost != 0 || got.SelectedCourier != "" {
		t.Errorf("lost update: stale PatchCart restored a shipping quote priced for a removed item's weight (courier=%q cost=%v)",
			got.SelectedCourier, got.ShippingCost)
	}
	if !errors.Is(patchErr, repository.ErrOrderChanged) {
		t.Errorf("want repository.ErrOrderChanged from a PatchCart built off a stale read, got %v", patchErr)
	}
}
