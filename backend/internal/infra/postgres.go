package infra

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// poolConfig overrides any default_query_exec_mode in the DSN: named prepared
// statements don't survive PgBouncer transaction pooling, and cache_describe is
// the only pooler-safe mode that still resolves []uuid.UUID and jsonb arguments.
func poolConfig(dsn string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe
	return cfg, nil
}

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(dsn)
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
