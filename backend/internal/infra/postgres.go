package infra

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultMaxConns replaces pgx's unchosen max(4, NumCPU). Sized for vm-app: 2
// vCPU, two Go processes (api, worker) each holding a pool, PgBouncer in
// transaction pooling in front. 10 per process = 20 client connections toward
// PgBouncer, comfortably under a stock default_pool_size of 20 per user/db pair
// while being 2.5x the accidental 4.
const defaultMaxConns int32 = 10

// poolConfig overrides any default_query_exec_mode in the DSN: named prepared
// statements don't survive PgBouncer transaction pooling, and cache_describe is
// the only pooler-safe mode that still resolves []uuid.UUID and jsonb arguments.
//
// maxConns precedence: maxConns >= 1 always wins. maxConns == 0 defers to the
// DSN's own pool_max_conns when the DSN carried one, else falls back to
// defaultMaxConns. pgxpool.ParseConfig always leaves cfg.MaxConns populated
// (from the DSN or its own computed default), so an explicit DSN value can't be
// told apart from the computed one by inspecting cfg alone — it must be detected
// by inspecting the parsed DSN's runtime params before pgxpool consumes them.
func poolConfig(dsn string, maxConns int32) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe

	switch {
	case maxConns >= 1:
		cfg.MaxConns = maxConns
	case dsnHasPoolMaxConns(dsn):
		// keep whatever pgxpool.ParseConfig already computed from the DSN
	default:
		cfg.MaxConns = defaultMaxConns
	}

	return cfg, nil
}

func dsnHasPoolMaxConns(dsn string) bool {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return false
	}
	_, ok := connConfig.Config.RuntimeParams["pool_max_conns"]
	return ok
}

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return NewPoolWithMaxConns(ctx, dsn, 0)
}

func NewPoolWithMaxConns(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(dsn, maxConns)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
