package repository

import (
	"context"
	"time"
)

// SystemConfigRow represents a single row from the system_config table.
type SystemConfigRow struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	IsSecret  bool      `json:"is_secret"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListSystemConfig returns all rows from the system_config table.
func (r *Repository) ListSystemConfig(ctx context.Context) ([]SystemConfigRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, value, is_secret, updated_at FROM system_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []SystemConfigRow
	for rows.Next() {
		var c SystemConfigRow
		if err := rows.Scan(&c.Key, &c.Value, &c.IsSecret, &c.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// GetSystemConfigValue reads one key; found is false when the row is absent.
func (r *Repository) GetSystemConfigValue(ctx context.Context, key string) (value string, found bool, err error) {
	err = r.pool.QueryRow(ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
	if isNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// InsertSystemConfigIfAbsent stores a non-secret value only when the key has no
// row yet, so a deliberate manual choice is never overwritten.
func (r *Repository) InsertSystemConfigIfAbsent(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret, updated_at)
		 VALUES ($1, $2, false, now())
		 ON CONFLICT (key) DO NOTHING`,
		key, value,
	)
	return err
}

// UpsertSystemConfig inserts or updates a single system_config row.
func (r *Repository) UpsertSystemConfig(ctx context.Context, key, value string, isSecret bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (key) DO UPDATE SET value = $2, is_secret = $3, updated_at = now()`,
		key, value, isSecret,
	)
	return err
}
