package adapter

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

// TestBiteshipSandboxCreateAndFetchOrder is the Level 3 sandbox proof: create
// one sandbox order, re-fetch it, then re-create with the same reference_id
// to exercise the duplicate-detection path (FR-C-7). Skipped unless
// BITESHIP_SANDBOX_KEY is set, so it never runs in CI. Same shape as the
// local Gotenberg proof already used in this repo — no tunnel, no staging.
//
//	BITESHIP_SANDBOX_KEY=biteship_test_xxx \
//	  go test ./internal/adapter/ -run TestBiteshipSandboxCreateAndFetchOrder -v
func TestBiteshipSandboxCreateAndFetchOrder(t *testing.T) {
	apiKey := os.Getenv("BITESHIP_SANDBOX_KEY")
	if apiKey == "" {
		t.Skip("BITESHIP_SANDBOX_KEY not set; this test needs a real Biteship sandbox key")
	}

	// Sandbox is key-selected, not host-selected — the base URL stays the
	// production host per the design's decision.
	client := NewBiteshipClient(nil, apiKey, "https://api.biteship.com", &http.Client{Timeout: 10 * time.Second})

	req := service.CreateShipmentRequest{
		ReferenceID:             fmt.Sprintf("sandbox-proof-%d", time.Now().UnixNano()),
		OriginContactName:       "Budi Test",
		OriginContactPhone:      "081200000000",
		OriginAddress:           "Jl. Contoh No. 1",
		OriginPostalCode:        "12440",
		DestinationContactName:  "Siti Test",
		DestinationContactPhone: "081300000000",
		DestinationAddress:      "Jl. Contoh No. 2",
		DestinationPostalCode:   "12240",
		CourierCode:             "jne",
		ServiceCode:             "reg",
		Items: []service.ShipmentItem{
			{Name: "Buku Ujian Test", Value: 50000, Quantity: 1, WeightGrams: 500},
		},
	}

	created, err := client.CreateOrder(t.Context(), req)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if created.BiteshipOrderID == "" {
		t.Fatal("expected a biteship order id, got empty string")
	}
	t.Logf("created sandbox order %s, waybill=%q status=%q", created.BiteshipOrderID, created.WaybillID, created.Status)

	fetched, err := client.GetOrder(t.Context(), created.BiteshipOrderID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if fetched.BiteshipOrderID != created.BiteshipOrderID {
		t.Errorf("GetOrder returned order id %q, want %q", fetched.BiteshipOrderID, created.BiteshipOrderID)
	}

	_, err = client.CreateOrder(t.Context(), req)
	var alreadyBooked *service.ShipmentAlreadyBookedError
	if !errors.As(err, &alreadyBooked) {
		t.Fatalf("expected retry with same reference_id to return *service.ShipmentAlreadyBookedError, got %v", err)
	}
	if alreadyBooked.ExistingBiteshipOrderID == "" {
		t.Error("expected ShipmentAlreadyBookedError to carry the echoed existing biteship order id, got empty string")
	}
	t.Logf("duplicate retry correctly echoed existing order id %q", alreadyBooked.ExistingBiteshipOrderID)
}
