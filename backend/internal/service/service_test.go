package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"akademi-bimbel/internal/model"
)

// pingFailRepo overrides Ping to simulate a broken postgres connection.
type pingFailRepo struct {
	fakeUserRepo
}

func (f *pingFailRepo) Ping(_ context.Context) error {
	return errors.New("connection refused")
}

func TestHealth_AllUp(t *testing.T) {
	repo := newFakeUserRepo()
	svc, _ := newTestService(t, repo)

	h := svc.Health(context.Background())
	if h.Status != "ok" {
		t.Errorf("Status: want ok, got %s", h.Status)
	}
	if h.Postgres != "ok" {
		t.Errorf("Postgres: want ok, got %s", h.Postgres)
	}
	if h.Redis != "ok" {
		t.Errorf("Redis: want ok, got %s", h.Redis)
	}
}

func TestHealth_PostgresDown(t *testing.T) {
	svc, _ := newTestService(t, &pingFailRepo{})

	h := svc.Health(context.Background())
	if h.Status != "degraded" {
		t.Errorf("Status: want degraded, got %s", h.Status)
	}
	if h.Postgres != "down" {
		t.Errorf("Postgres: want down, got %s", h.Postgres)
	}
	if h.Redis != "ok" {
		t.Errorf("Redis: want ok, got %s", h.Redis)
	}
}

func TestHealth_RedisDown(t *testing.T) {
	repo := newFakeUserRepo()
	svc, mr := newTestService(t, repo)
	mr.Close() // force redis failures

	h := svc.Health(context.Background())
	if h.Status != "degraded" {
		t.Errorf("Status: want degraded, got %s", h.Status)
	}
	if h.Postgres != "ok" {
		t.Errorf("Postgres: want ok, got %s", h.Postgres)
	}
	if h.Redis != "down" {
		t.Errorf("Redis: want down, got %s", h.Redis)
	}
}

func TestParseAccess_ValidToken(t *testing.T) {
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:           "u-parse",
		Email:        strptr("parse@example.com"),
		PasswordHash: mustHashStd("password123"),
		Role:         RoleStudent,
		Status:       "active",
	})
	svc, _ := newTestService(t, repo)

	// Mint a token via the service's own login flow to get a valid JWT.
	access, _, _, err := svc.Login(context.Background(), "parse@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if access == "" {
		t.Fatal("empty access token")
	}

	claims, err := svc.ParseAccess(access)
	if err != nil {
		t.Fatalf("ParseAccess: want nil err, got %v", err)
	}
	if claims.Sub != "u-parse" {
		t.Errorf("Sub: want u-parse, got %s", claims.Sub)
	}
}

func TestParseAccess_InvalidToken(t *testing.T) {
	repo := newFakeUserRepo()
	svc, _ := newTestService(t, repo)

	_, err := svc.ParseAccess("this.is.not.a.valid.token")
	if err == nil {
		t.Error("ParseAccess: want error for invalid token, got nil")
	}
}

func TestNewForTest(t *testing.T) {
	svc := NewForTest(nil)
	if svc == nil {
		t.Fatal("NewForTest: got nil")
	}
}

func TestNew(t *testing.T) {
	svc, _ := newTestService(t, newFakeUserRepo())
	if svc == nil {
		t.Fatal("New: got nil service")
	}
}

// TestReloadLogisticsClient_ConcurrentWithGetShippingRates drives admin config
// reloads and quote requests against the same Service concurrently. Run with
// -race: before logistics was held behind atomic.Pointer, this reliably
// tripped the race detector because ReloadLogisticsClient wrote s.logistics
// while GetShippingRates read it from another goroutine.
func TestReloadLogisticsClient_ConcurrentWithGetShippingRates(t *testing.T) {
	_, repo := newRealDBService(t)
	// The noop client now errors (see shipping_rates.go), so GetShippingRates
	// always falls through to the configured flat rate — needs a real repo.
	if err := repo.UpsertSystemConfig(context.Background(), "shipping_fallback_flat_rate", "12000", false); err != nil {
		t.Fatalf("seed shipping_fallback_flat_rate: %v", err)
	}

	svc := &Service{storeRepo: repo}
	var initial LogisticsClient = &NoopLogisticsClient{}
	svc.logistics.Store(&initial)
	svc.SetReloadLogisticsFn(func(ctx context.Context) LogisticsClient {
		return &NoopLogisticsClient{}
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			svc.ReloadLogisticsClient(ctx)
		}()
		go func() {
			defer wg.Done()
			if _, err := svc.GetShippingRates(ctx, ShippingQuoteRequest{DestinationPostalCode: "12345", WeightGrams: 500}); err != nil {
				t.Errorf("GetShippingRates: %v", err)
			}
		}()
	}
	wg.Wait()
}
