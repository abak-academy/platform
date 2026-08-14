package service

import (
	"context"
	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// seedPhysicalCartWithCourierQuote mints a cart, adds one physical item, and
// drives a real courier selection through PatchCart so the order persists a
// non-empty selected_courier/selected_service, resolved courier codes, and a
// priced shipping_cost at the given destination — the pre-image every test in
// this file starts from.
func seedPhysicalCartWithCourierQuote(t *testing.T, svc *Service, repo *repository.Repository, studentID, provinceID, cityID, districtID, kodePos string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ('book', $1, 50000, 10, 'published', 500) RETURNING id`,
		"Destination Change Book "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	err = svc.PatchCart(ctx, studentID, order.ID.String(), CartPatch{
		Courier:    "JNE",
		Service:    "REG",
		ProvinceID: &provinceID,
		CityID:     &cityID,
		DistrictID: &districtID,
		KodePos:    &kodePos,
	})
	if err != nil {
		t.Fatalf("PatchCart (seed courier quote): %v", err)
	}

	seeded, err := repo.GetOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID (seed check): %v", err)
	}
	if seeded.SelectedCourier != "JNE" || seeded.ShippingCost <= 0 {
		t.Fatalf("seed: want selected_courier=JNE and shipping_cost>0, got %q / %v", seeded.SelectedCourier, seeded.ShippingCost)
	}

	return order.ID
}

func newDestinationChangeTestService(t *testing.T) (*Service, *repository.Repository) {
	t.Helper()
	_, repo := newRealDBService(t)
	spy := &recordingLogisticsClient{rate: CourierRate{Courier: "JNE", Service: "REG", Price: 18000, CourierCode: "jne", ServiceCode: "reg", IsEstimate: true}}
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, nil, spy, nil, nil, nil)
	return svc, repo
}

// TestPatchCart_KodePosChangeVoidsQuote covers FR-7: an address-only PATCH
// (patch.Courier == "") that changes kode_pos against the persisted
// destination must void the stale quote.
func TestPatchCart_KodePosChangeVoidsQuote(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Kodepos Change Student", "kpchg_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	newKodePos := "54321"
	if err := svc.PatchCart(ctx, studentID, orderID.String(), CartPatch{
		KodePos: &newKodePos,
	}); err != nil {
		t.Fatalf("PatchCart (kode_pos change): %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.SelectedCourier != "" {
		t.Errorf("want selected_courier \"\", got %q", got.SelectedCourier)
	}
	if got.SelectedService != "" {
		t.Errorf("want selected_service \"\", got %q", got.SelectedService)
	}
	if got.CourierCode != nil {
		t.Errorf("want courier_code NULL, got %v", *got.CourierCode)
	}
	if got.CourierServiceCode != nil {
		t.Errorf("want courier_service_code NULL, got %v", *got.CourierServiceCode)
	}
	if got.IsEstimate {
		t.Error("want is_estimate false")
	}
	if got.ShippingCost != 0 {
		t.Errorf("want shipping_cost 0, got %v", got.ShippingCost)
	}
	if got.Total != got.Subtotal-got.Discount {
		t.Errorf("want total == subtotal - discount (%v), got %v", got.Subtotal-got.Discount, got.Total)
	}
}

// TestPatchCart_ProvinceCityDistrictChangeVoidsQuote covers the case the old
// guard (kode_pos only) missed: province_id/city_id/district_id changing with
// kode_pos unchanged must also void the quote.
func TestPatchCart_ProvinceCityDistrictChangeVoidsQuote(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Province Change Student", "provchg_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	newProvince, newCity, newDistrict := "94", "9401", "940101"
	if err := svc.PatchCart(ctx, studentID, orderID.String(), CartPatch{
		ProvinceID: &newProvince,
		CityID:     &newCity,
		DistrictID: &newDistrict,
	}); err != nil {
		t.Fatalf("PatchCart (province/city/district change): %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.SelectedCourier != "" {
		t.Errorf("want selected_courier \"\", got %q", got.SelectedCourier)
	}
	if got.SelectedService != "" {
		t.Errorf("want selected_service \"\", got %q", got.SelectedService)
	}
	if got.CourierCode != nil {
		t.Errorf("want courier_code NULL, got %v", *got.CourierCode)
	}
	if got.CourierServiceCode != nil {
		t.Errorf("want courier_service_code NULL, got %v", *got.CourierServiceCode)
	}
	if got.IsEstimate {
		t.Error("want is_estimate false")
	}
	if got.ShippingCost != 0 {
		t.Errorf("want shipping_cost 0, got %v", got.ShippingCost)
	}
	if got.Total != got.Subtotal-got.Discount {
		t.Errorf("want total == subtotal - discount (%v), got %v", got.Subtotal-got.Discount, got.Total)
	}
}

// TestPatchCart_IdenticalDestinationPatchPreservesQuote covers FR-8: resending
// the identical four destination values must not void a valid quote. This is
// the test that stops the fix from over-firing.
func TestPatchCart_IdenticalDestinationPatchPreservesQuote(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Identical Destination Student", "identdst_")

	provinceID, cityID, districtID, kodePos := "93", "9301", "930101", "12345"
	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, provinceID, cityID, districtID, kodePos)

	if err := svc.PatchCart(ctx, studentID, orderID.String(), CartPatch{
		ProvinceID: &provinceID,
		CityID:     &cityID,
		DistrictID: &districtID,
		KodePos:    &kodePos,
	}); err != nil {
		t.Fatalf("PatchCart (identical destination): %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.SelectedCourier != "JNE" {
		t.Errorf("want selected_courier to survive a no-op destination patch, got %q", got.SelectedCourier)
	}
	if got.SelectedService != "REG" {
		t.Errorf("want selected_service to survive a no-op destination patch, got %q", got.SelectedService)
	}
	if got.CourierCode == nil || *got.CourierCode != "jne" {
		t.Errorf("want courier_code to survive a no-op destination patch, got %v", got.CourierCode)
	}
	if got.CourierServiceCode == nil || *got.CourierServiceCode != "reg" {
		t.Errorf("want courier_service_code to survive a no-op destination patch, got %v", got.CourierServiceCode)
	}
	if !got.IsEstimate {
		t.Error("want is_estimate to survive a no-op destination patch as true, the seeded quote's value")
	}
	if got.ShippingCost != 18000 {
		t.Errorf("want shipping_cost 18000 to survive a no-op destination patch, got %v", got.ShippingCost)
	}
}

// TestPatchCart_OmittedDestinationFieldsPreserveQuote covers FR-9: a PATCH
// that omits destination fields entirely (all four nil) counts as unchanged
// and preserves the quote.
func TestPatchCart_OmittedDestinationFieldsPreserveQuote(t *testing.T) {
	ctx := context.Background()
	svc, repo := newDestinationChangeTestService(t)
	studentID := insertCheckoutStudent(t, repo, "Omitted Destination Student", "omitdst_")

	orderID := seedPhysicalCartWithCourierQuote(t, svc, repo, studentID, "93", "9301", "930101", "12345")

	if err := svc.PatchCart(ctx, studentID, orderID.String(), CartPatch{
		ShippingAddress: []byte(`{"street":"Jl. Contoh No. 9"}`),
	}); err != nil {
		t.Fatalf("PatchCart (destination omitted): %v", err)
	}

	got, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.SelectedCourier != "JNE" {
		t.Errorf("want selected_courier to survive an omitted-destination patch, got %q", got.SelectedCourier)
	}
	if got.ShippingCost != 18000 {
		t.Errorf("want shipping_cost 18000 to survive an omitted-destination patch, got %v", got.ShippingCost)
	}
}
