package infra

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	dbmigrations "akademi-bimbel/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open migration pool: %w", err)
	}
	defer pool.Close()

	// Two api replicas run this at startup (main.go), so serialize the whole
	// run with a session-level advisory lock or concurrent runners race the
	// same migration — seen as 23505 on pg_extension_name_index from 0066's
	// CREATE EXTENSION.
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer lockConn.Release()

	const lockKey = `hashtext('akademi-bimbel-migrations')`
	if _, err = lockConn.Exec(ctx, `SELECT pg_advisory_lock(`+lockKey+`)`); err != nil {
		return fmt.Errorf("take migration advisory lock: %w", err)
	}
	defer func() {
		// Unlock must run before Release (LIFO) so the lock never stays on a
		// returned pooled connection, and must ignore a cancelled ctx.
		if _, err := lockConn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(`+lockKey+`)`); err != nil {
			slog.ErrorContext(ctx, "release migration advisory lock", "err", err)
		}
	}()

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.Glob(dbmigrations.FS, "migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(entries)

	for _, path := range entries {
		filename := path[len("migrations/"):]

		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`,
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(dbmigrations.FS, path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", filename, err)
		}

		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}

		if _, err = tx.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, filename,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}

		slog.InfoContext(ctx, "migration applied", "file", filename)
	}

	return nil
}
