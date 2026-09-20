package handler_test

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"akademi-bimbel/internal/infra"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Every DB-backed env in this package used to start its own Postgres container
// and replay all migrations — thirteen containers for one `go test` run, which
// was 202s of the backend suite. They now share one container: migrations run
// once into handler_template, and each env clones its own database from it.
//
// A database per env, not a shared one, so envs stay as isolated from each
// other as separate containers made them.
const handlerTemplateDB = "handler_template"

var (
	sharedPGOnce      sync.Once
	sharedPGMaintDSN  string
	sharedPGContainer testcontainers.Container
	sharedPGErr       error
	handlerDBSeq      atomic.Int64
)

func withHandlerDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}

// newTemplatedPool returns a pool on a private, already-migrated database.
func newTemplatedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	sharedPGOnce.Do(func() {
		c, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase(handlerTemplateDB),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			sharedPGErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		sharedPGContainer = c

		templateDSN, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			sharedPGErr = fmt.Errorf("connection string: %w", err)
			return
		}
		// RunMigrations closes its own pool; CREATE DATABASE ... TEMPLATE
		// refuses while any session is still connected to the source.
		if err := infra.RunMigrations(ctx, templateDSN); err != nil {
			sharedPGErr = fmt.Errorf("run migrations: %w", err)
			return
		}
		sharedPGMaintDSN, sharedPGErr = withHandlerDatabase(templateDSN, "postgres")
	})
	if sharedPGErr != nil {
		t.Fatalf("shared handler postgres: %v", sharedPGErr)
	}

	name := fmt.Sprintf("h_%d", handlerDBSeq.Add(1))
	maint, err := pgxpool.New(ctx, sharedPGMaintDSN)
	if err != nil {
		t.Fatalf("maintenance pool: %v", err)
	}
	_, err = maint.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE "+handlerTemplateDB)
	maint.Close()
	if err != nil {
		t.Fatalf("clone template database: %v", err)
	}

	dsn, err := withHandlerDatabase(sharedPGMaintDSN, name)
	if err != nil {
		t.Fatalf("env dsn: %v", err)
	}
	pool, err := infra.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	return pool
}

// terminateSharedPG is called from TestMain once the package is done.
func terminateSharedPG() {
	if sharedPGContainer != nil {
		_ = sharedPGContainer.Terminate(context.Background())
	}
}

// closeEnv releases what a DB-backed env owns. Each argument is optional: envs
// differ in whether they use a real redis client or a miniredis.
func closeEnv(pool *pgxpool.Pool, rdb *redis.Client, mr *miniredis.Miniredis) {
	if pool != nil {
		pool.Close()
	}
	if rdb != nil {
		_ = rdb.Close()
	}
	if mr != nil {
		mr.Close()
	}
}
