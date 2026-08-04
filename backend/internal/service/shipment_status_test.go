package service

import (
	"context"
	"slices"
	"testing"

	"akademi-bimbel/internal/repository"
)

func TestShipmentStatusSpellings_CoversBothWireFormats(t *testing.T) {
	got := shipmentStatusSpellings("courier_not_found")
	for _, want := range []string{"courier_not_found", "courierNotFound"} {
		if !slices.Contains(got, want) {
			t.Errorf("want %q among %v", want, got)
		}
	}
}

func TestShipmentStatusSpellings_SingleWordHasOneForm(t *testing.T) {
	if got := shipmentStatusSpellings("returned"); len(got) != 1 || got[0] != "returned" {
		t.Errorf(`want exactly ["returned"], got %v`, got)
	}
}

// A healthy status leaking into the filter would badge every in-flight parcel
// as broken, which is worse than not having the filter at all.
func TestShipmentFailureStatusValues_ExcludesHealthyAndOnHold(t *testing.T) {
	values := ShipmentFailureStatusValues()
	for _, healthy := range []string{"confirmed", "allocated", "picked", "in_transit", "dropping_off", "delivered", "on_hold"} {
		if slices.Contains(values, healthy) {
			t.Errorf("%q must not be treated as a failed shipment", healthy)
		}
	}
}

func TestShipmentFailureStatusValues_CarriesEverySpelling(t *testing.T) {
	values := ShipmentFailureStatusValues()
	for _, want := range []string{
		"courier_not_found", "courierNotFound",
		"return_in_transit", "returnInTransit",
		"rejected", "cancelled", "returned", "disposed",
	} {
		if !slices.Contains(values, want) {
			t.Errorf("want %q among %v", want, values)
		}
	}
}

// TestAdminListOrders_ShipmentFailedFilter proves the filter an admin needs to
// find parcels that died: orders.status still reads "shipped" for both of
// these, so without this the broken one is indistinguishable in the listing.
//
// The failed order is stored camelCase on purpose — that is the spelling the
// docs use and the one a naive filter would miss.
func TestAdminListOrders_ShipmentFailedFilter(t *testing.T) {
	svc, repo := newShippingWebhookTestService(t, &fakeShipWebhookLogistics{})
	ctx := context.Background()

	failed := seedShippedOrder(t, svc, repo, "biteship-filter-failed")
	healthy := seedShippedOrder(t, svc, repo, "biteship-filter-healthy")
	if err := repo.SetShipmentStatus(ctx, failed, "courierNotFound"); err != nil {
		t.Fatalf("set failed status: %v", err)
	}
	if err := repo.SetShipmentStatus(ctx, healthy, "in_transit"); err != nil {
		t.Fatalf("set healthy status: %v", err)
	}

	orders, _, err := svc.AdminListOrders(ctx, repository.OrderFilter{
		ShipmentStatusIn: ShipmentFailureStatusValues(),
		Limit:            100,
	})
	if err != nil {
		t.Fatalf("AdminListOrders: %v", err)
	}

	seen := map[string]bool{}
	for _, o := range orders {
		seen[o.ID.String()] = true
	}
	if !seen[failed.String()] {
		t.Error("an order whose courier was never found must appear under the failed filter")
	}
	if seen[healthy.String()] {
		t.Error("an in-transit order must not appear under the failed filter")
	}
}
