package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"akademi-bimbel/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type courierQuoteClient struct {
	NoopLogisticsClient
	req   ShippingQuoteRequest
	rates []CourierRate
	err   error
}

func (f *courierQuoteClient) GetRates(_ context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	f.req = req
	return f.rates, f.err
}

func courierTestRates() []CourierRate {
	return []CourierRate{
		{Courier: "JNE", Service: "REG", CourierCode: "jne", ServiceCode: "reg", Price: 10000},
		{Courier: "JNE", Service: "YES", CourierCode: "jne", ServiceCode: "yes", Price: 20000},
		{Courier: "JNE", Service: "OKE", CourierCode: "jne", ServiceCode: "oke", Price: 8000},
		{CourierCode: "jne", ServiceCode: "jtr", Price: 5000},
		{CourierCode: "lion", ServiceCode: "reg_pack", Price: 11000},
		{CourierCode: "anteraja", ServiceCode: "reg", Price: 12000},
	}
}

func TestShippingCourierConfigReadFailureUsesDefaults(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	fake := &courierQuoteClient{rates: courierTestRates()}
	svc := &Service{storeRepo: repository.New(pool)}
	var client LogisticsClient = fake
	svc.logistics.Store(&client)
	got, err := svc.GetShippingRates(context.Background(), ShippingQuoteRequest{DestinationPostalCode: "12345", Couriers: []string{"tiki"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fake.req.Couriers, []string{"jne"}) {
		t.Errorf("couriers=%v want [jne]", fake.req.Couriers)
	}
	if !reflect.DeepEqual(got, courierTestRates()[:3]) {
		t.Errorf("default rates=%+v", got)
	}
}

func TestShippingCourierSavedConfig(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_ = repo.UpsertSystemConfig(ctx, "biteship_services", "", false)
		_ = repo.UpsertSystemConfig(ctx, "shipping_fallback_flat_rate", "", false)
	})
	if err := repo.UpsertSystemConfig(ctx, "shipping_fallback_flat_rate", "15000", false); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, config string
		couriers     []string
		indices      []int
	}{
		{"absent", "", []string{"jne"}, []int{0, 1, 2}},
		{"empty", "", []string{"jne"}, []int{0, 1, 2}},
		{"lion enabled oke disabled", "lion:reg_pack,jne:yes,jne:reg", []string{"jne", "lion"}, []int{0, 1, 4}},
		{"intersection empty", "tiki:reg", []string{"tiki"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "absent" {
				if _, err := repo.Pool().Exec(ctx, `DELETE FROM system_config WHERE key = 'biteship_services'`); err != nil {
					t.Fatal(err)
				}
			} else if err := repo.UpsertSystemConfig(ctx, "biteship_services", tt.config, false); err != nil {
				t.Fatal(err)
			}
			fake := &courierQuoteClient{rates: courierTestRates()}
			svc := &Service{storeRepo: repo}
			var client LogisticsClient = fake
			svc.logistics.Store(&client)
			got, err := svc.GetShippingRates(ctx, ShippingQuoteRequest{DestinationPostalCode: "12345"})
			if tt.indices == nil {
				if !errors.Is(err, ErrShippingRouteUnserved) {
					t.Fatalf("want route unserved, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fake.req.Couriers, tt.couriers) {
				t.Errorf("couriers=%v want %v", fake.req.Couriers, tt.couriers)
			}
			var want []CourierRate
			for _, i := range tt.indices {
				want = append(want, fake.rates[i])
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("rates=%+v want %+v", got, want)
			}
		})
	}
}

func TestPatchCart_DisabledCourierSelection(t *testing.T) {
	fake := &courierQuoteClient{rates: courierTestRates()}
	svc, repo := newShipOrderTestService(t, fake)
	ctx := context.Background()
	if err := repo.UpsertSystemConfig(ctx, "biteship_services", "jne:reg", false); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSystemConfig(ctx, "shipping_fallback_flat_rate", "15000", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = repo.UpsertSystemConfig(ctx, "biteship_services", "", false)
		_ = repo.UpsertSystemConfig(ctx, "shipping_fallback_flat_rate", "", false)
	})
	id := createShippableOrder(t, svc, repo, "cart", true)
	order, err := repo.GetOrderByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	province, city, district, postal := "93", "9301", "930101", "12345"
	err = svc.PatchCart(ctx, order.StudentID.String(), id.String(), CartPatch{Courier: "JNE", Service: "OKE", ProvinceID: &province, CityID: &city, DistrictID: &district, KodePos: &postal})
	if !errors.Is(err, ErrInvalidCourierSelection) {
		t.Fatalf("want ErrInvalidCourierSelection, got %v", err)
	}
}

func TestGetShippingRates_Degraded(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	prior := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prior) })
	tests := []struct {
		name      string
		clientErr error
		rates     []CourierRate
		wantErr   error
		cause     string
	}{
		{"client error", errors.New(strings.Repeat("x", 350)), nil, ErrShippingTemporarilyUnavailable, "client_error"},
		{"empty rates", nil, nil, ErrShippingRouteUnserved, "route_unserved"},
		{"all filtered", nil, []CourierRate{{CourierCode: "tiki", ServiceCode: "reg", Price: 15000}}, ErrShippingRouteUnserved, "route_unserved"},
		{"origin unset", ErrShippingOriginUnset, nil, ErrShippingUnavailable, "origin_unset"},
		{"auth rejected", errors.Join(ErrShippingAuthRejected, errors.New("raw secret response body")), nil, ErrShippingUnavailable, "auth_rejected"},
		{"noop", ErrShippingUnavailable, nil, ErrShippingUnavailable, "client_noop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			fake := &courierQuoteClient{rates: tt.rates, err: tt.clientErr}
			svc := &Service{storeRepo: repository.New(pool)}
			var client LogisticsClient = fake
			svc.logistics.Store(&client)
			got, err := svc.GetShippingRates(context.Background(), ShippingQuoteRequest{DestinationPostalCode: "12345"})
			if !errors.Is(err, tt.wantErr) || len(got) != 0 {
				t.Errorf("rates=%+v err=%v, want no rates and %v", got, err, tt.wantErr)
			}
			var record map[string]any
			if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record["cause"] != tt.cause {
				t.Errorf("cause=%v want %s", record["cause"], tt.cause)
			}
			if tt.cause == "client_error" {
				if record["err"] != strings.Repeat("x", 300) {
					t.Errorf("err=%v, want truncated client error", record["err"])
				}
			} else if _, ok := record["err"]; ok {
				t.Errorf("unexpected error text: %s", buf.String())
			}
			if strings.Contains(buf.String(), "raw secret response body") {
				t.Error("auth response body leaked")
			}
		})
	}
}

func TestGetShippingRates_LegacyFlatRowCannotPriceFailure(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	if err := repo.UpsertSystemConfig(ctx, "shipping_fallback_flat_rate", "15000", false); err != nil {
		t.Fatal(err)
	}
	fake := &courierQuoteClient{err: errors.New("upstream down")}
	svc := &Service{storeRepo: repo}
	var client LogisticsClient = fake
	svc.logistics.Store(&client)
	rates, err := svc.GetShippingRates(ctx, ShippingQuoteRequest{DestinationPostalCode: "12345"})
	if !errors.Is(err, ErrShippingTemporarilyUnavailable) || len(rates) != 0 {
		t.Fatalf("rates=%+v err=%v", rates, err)
	}
}

func TestPatchCart_RealRateClearsEstimate(t *testing.T) {
	fake := &courierQuoteClient{rates: courierTestRates()}
	svc, repo := newShipOrderTestService(t, fake)
	ctx := context.Background()
	id := createShippableOrder(t, svc, repo, "cart", true)
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET is_estimate = true WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	order, err := repo.GetOrderByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	province, city, district, postal := "93", "9301", "930101", "12345"
	if err := svc.PatchCart(ctx, order.StudentID.String(), id.String(), CartPatch{Courier: "JNE", Service: "REG", ProvinceID: &province, CityID: &city, DistrictID: &district, KodePos: &postal}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetOrderByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsEstimate || got.ShippingCost != 10000 || got.CourierCode == nil || *got.CourierCode != "jne" {
		t.Fatalf("real quote not persisted: %+v", got)
	}
}
