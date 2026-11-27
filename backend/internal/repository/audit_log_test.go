package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Compile-time check: *Repository must implement all audit log methods.
var _ interface {
	ListAuditLog(context.Context, AuditLogFilter) ([]AuditLogRow, int, error)
	InsertAuditLogMeta(context.Context, pgx.Tx, *string, string, string, string, map[string]any) error
} = (*Repository)(nil)
