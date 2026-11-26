package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0076_TestimonialsUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0075_event.up.sql")

	applyMigrationFile(t, pool, "0076_testimonials.up.sql")
	requireColumnExists(t, pool, "testimonial", "consent_note", true)
	_, err := pool.Exec(ctx, `INSERT INTO testimonial (name, role_label) VALUES ('A', 'SMP')`)
	require.NoError(t, err)

	applyMigrationFile(t, pool, "0076_testimonials.down.sql")
	var tables int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'testimonial'`).Scan(&tables))
	require.Zero(t, tables)

	applyMigrationFile(t, pool, "0076_testimonials.up.sql")
	requireColumnExists(t, pool, "testimonial", "photo_key", true)
}
