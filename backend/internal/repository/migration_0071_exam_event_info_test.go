package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var examEventInfoColumns = []string{"whatsapp_group_url", "additional_rules_note", "additional_rules_url"}

func requireExamEventInfoColumns(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	for _, col := range examEventInfoColumns {
		requireColumnExists(t, pool, "exam", col, want)
	}
}

func TestMigration0071_ExamEventInfoRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0070_affiliate.up.sql")
	requireExamEventInfoColumns(t, pool, false)

	var examID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, timer_mode, duration_minutes) VALUES ('old exam', 'overall', 60) RETURNING id`,
	).Scan(&examID))

	applyMigrationFile(t, pool, "0071_exam_event_info.up.sql")
	requireExamEventInfoColumns(t, pool, true)
	for _, col := range examEventInfoColumns {
		var nullable, dataType string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_nullable, data_type FROM information_schema.columns WHERE table_name = 'exam' AND column_name = $1`, col,
		).Scan(&nullable, &dataType))
		require.Equal(t, "YES", nullable, col)
		require.Equal(t, "text", dataType, col)
	}
	var wa, note, url *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT whatsapp_group_url, additional_rules_note, additional_rules_url FROM exam WHERE id = $1`, examID,
	).Scan(&wa, &note, &url))
	require.Nil(t, wa)
	require.Nil(t, note)
	require.Nil(t, url)

	applyMigrationFile(t, pool, "0071_exam_event_info.down.sql")
	requireExamEventInfoColumns(t, pool, false)

	applyMigrationFile(t, pool, "0071_exam_event_info.up.sql")
	requireExamEventInfoColumns(t, pool, true)
}
