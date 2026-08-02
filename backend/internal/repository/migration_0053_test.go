package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0053_ExamEndScreen proves 0053 adds the two nullable
// end-screen columns to exam (FR-38/FR-39) and that the down migration drops
// exactly those two columns without touching any other exam data.
func TestMigration0053_ExamEndScreen(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0052_exam_session_position.up.sql")

	assertColumn := func(column string, want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'exam' AND column_name = $1)`,
			column,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}
	assertNullable := func(column string, msg string) {
		t.Helper()
		var nullable string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'exam' AND column_name = $1`,
			column,
		).Scan(&nullable))
		require.Equal(t, "YES", nullable, msg)
	}

	newColumns := []string{"end_screen_image_url", "end_screen_promo_text"}
	for _, col := range newColumns {
		assertColumn(col, false, col+" must not exist before 0053")
	}

	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, $2) RETURNING id`,
		"Migration 0053 Exam", "draft",
	).Scan(&examID))

	applyMigrationFile(t, pool, "0053_exam_end_screen.up.sql")

	for _, col := range newColumns {
		assertColumn(col, true, col+" must exist after 0053")
		assertNullable(col, col+" must be nullable")
	}

	_, err := pool.Exec(ctx,
		`UPDATE exam SET end_screen_image_url = $1, end_screen_promo_text = $2 WHERE id = $3`,
		"https://cdn.example.com/end-screen.png", "Thanks for taking the exam!", examID,
	)
	require.NoError(t, err)

	applyMigrationFile(t, pool, "0053_exam_end_screen.down.sql")

	for _, col := range newColumns {
		assertColumn(col, false, col+" must be dropped by down")
	}

	var titleIntact string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title FROM exam WHERE id = $1`, examID,
	).Scan(&titleIntact))
	require.Equal(t, "Migration 0053 Exam", titleIntact, "down must not touch any other exam column")
}
