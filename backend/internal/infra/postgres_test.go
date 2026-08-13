package infra

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPoolConfig_ForcesCacheDescribeMode(t *testing.T) {
	cfg, err := poolConfig("postgres://user:pass@localhost:5432/db", 0)
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheDescribe {
		t.Fatalf("DefaultQueryExecMode = %v, want %v", cfg.ConnConfig.DefaultQueryExecMode, pgx.QueryExecModeCacheDescribe)
	}
}

func TestPoolConfig_ForcesCacheDescribeOverDSNOverride(t *testing.T) {
	cfg, err := poolConfig("postgres://user:pass@localhost:5432/db?default_query_exec_mode=cache_statement", 0)
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheDescribe {
		t.Fatalf("DefaultQueryExecMode = %v, want %v (code must win over DSN)", cfg.ConnConfig.DefaultQueryExecMode, pgx.QueryExecModeCacheDescribe)
	}
}

func TestPoolConfig_MalformedDSN(t *testing.T) {
	cfg, err := poolConfig("://not-a-valid-dsn", 0)
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if cfg != nil {
		t.Fatalf("expected nil config on error, got %+v", cfg)
	}
}

func TestPoolConfig_MaxConns(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		maxConns int32
		want     int32
	}{
		{
			name:     "explicit N applied",
			dsn:      "postgres://user:pass@localhost:5432/db",
			maxConns: 7,
			want:     7,
		},
		{
			name:     "zero falls back to defaultMaxConns",
			dsn:      "postgres://user:pass@localhost:5432/db",
			maxConns: 0,
			want:     defaultMaxConns,
		},
		{
			name:     "zero preserves DSN pool_max_conns",
			dsn:      "postgres://user:pass@localhost:5432/db?pool_max_conns=25",
			maxConns: 0,
			want:     25,
		},
		{
			name:     "explicit N wins over DSN pool_max_conns",
			dsn:      "postgres://user:pass@localhost:5432/db?pool_max_conns=25",
			maxConns: 7,
			want:     7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := poolConfig(tt.dsn, tt.maxConns)
			if err != nil {
				t.Fatalf("poolConfig: %v", err)
			}
			if cfg.MaxConns != tt.want {
				t.Fatalf("MaxConns = %d, want %d", cfg.MaxConns, tt.want)
			}
			if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheDescribe {
				t.Fatalf("DefaultQueryExecMode = %v, want %v", cfg.ConnConfig.DefaultQueryExecMode, pgx.QueryExecModeCacheDescribe)
			}
		})
	}
}
