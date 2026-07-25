package infra

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPoolConfig_ForcesCacheDescribeMode(t *testing.T) {
	cfg, err := poolConfig("postgres://user:pass@localhost:5432/db")
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheDescribe {
		t.Fatalf("DefaultQueryExecMode = %v, want %v", cfg.ConnConfig.DefaultQueryExecMode, pgx.QueryExecModeCacheDescribe)
	}
}

func TestPoolConfig_ForcesCacheDescribeOverDSNOverride(t *testing.T) {
	cfg, err := poolConfig("postgres://user:pass@localhost:5432/db?default_query_exec_mode=cache_statement")
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheDescribe {
		t.Fatalf("DefaultQueryExecMode = %v, want %v (code must win over DSN)", cfg.ConnConfig.DefaultQueryExecMode, pgx.QueryExecModeCacheDescribe)
	}
}

func TestPoolConfig_MalformedDSN(t *testing.T) {
	cfg, err := poolConfig("://not-a-valid-dsn")
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if cfg != nil {
		t.Fatalf("expected nil config on error, got %+v", cfg)
	}
}
