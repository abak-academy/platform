package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// realDBOnce/realDBSvc back newRealDBService: storeRepo is *repository.Repository
// (a concrete type, not an interface), so school/student Service methods that
// touch it cannot be exercised against a fake — a real Postgres instance is the
// only way to invoke them at all. One container is shared across every test in
// this package that needs it; callers must use unique codes/NIS/names (see
// uniqueSuffix) since rows are never reset between tests.
var (
	realDBOnce  sync.Once
	realDBSvc   *Service
	realDBRepo  *repository.Repository
	realDBRedis *miniredis.Miniredis
)

func newRealDBService(t *testing.T) (*Service, *repository.Repository) {
	t.Helper()
	realDBOnce.Do(func() {
		ctx := context.Background()
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("connection string: %v", err)
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		pool, err := infra.NewPool(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		repo := repository.New(pool)
		redisServer, err := miniredis.Run()
		if err != nil {
			t.Fatalf("start miniredis: %v", err)
		}
		rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		realDBRepo = repo
		realDBRedis = redisServer
		realDBSvc = NewWithStore(repo, repo, rdb,
			infra.NewJWTSigner("test-secret", 15*time.Minute),
			&NoopOTPProvider{}, &NoopEmailProvider{},
			nil, nil, nil,
			&config.Config{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 168 * time.Hour},
			nil)
	})
	if realDBSvc == nil {
		t.Fatal("real db service failed to initialize")
	}
	return realDBSvc, realDBRepo
}

// uniqueSuffix returns a short unique token for building school codes/NIS/names
// that won't collide with other tests sharing the same real-DB fixture.
func uniqueSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
}
