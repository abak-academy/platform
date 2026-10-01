package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigration0068_ExamFormatPolicy proves the paired up/down migration adds the
// six format-policy columns with their defaults and constraints, and removes them
// cleanly on down.
func TestMigration0068_ExamFormatPolicy(t *testing.T) {
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0067_users_password_temporary.up.sql")
	applyMigrationFile(t, pool, "0068_exam_format_policy.up.sql")

	// exam defaults.
	var formatVersion int
	var navigationPolicy string
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO exam (title) VALUES ('m0068') RETURNING format_version, navigation_policy`,
	).Scan(&formatVersion, &navigationPolicy))
	require.Equal(t, 1, formatVersion)
	require.Equal(t, "free", navigationPolicy)

	// test columns exist and round-trip.
	var testID string
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO test (title, subject, topic, duration_minutes, shared_material_html, instructions_html)
		 VALUES ('m0068', 's', 't', 1, '<p>m</p>', '<b>i</b>') RETURNING id`,
	).Scan(&testID))
	var shared, instructions string
	require.NoError(t, pool.QueryRow(t.Context(),
		`SELECT shared_material_html, instructions_html FROM test WHERE id = $1`, testID,
	).Scan(&shared, &instructions))
	require.Equal(t, "<p>m</p>", shared)
	require.Equal(t, "<b>i</b>", instructions)

	// question material_html exists.
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO question (format, body, point_correct, point_wrong, material_html)
		 VALUES ('essay', 'b', 1, 0, '<p>material</p>') RETURNING id`,
	).Scan(&testID))
	var material string
	require.NoError(t, pool.QueryRow(t.Context(),
		`SELECT material_html FROM question WHERE body = 'b'`,
	).Scan(&material))
	require.Equal(t, "<p>material</p>", material)

	// exam_test default presentation.
	var examID string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT id FROM exam WHERE title = 'm0068'`).Scan(&examID))
	var qTestID string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT id FROM test WHERE title = 'm0068'`).Scan(&qTestID))
	var presentation string
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 0) RETURNING question_presentation`,
		examID, qTestID,
	).Scan(&presentation))
	require.Equal(t, "one_per_page", presentation)

	// CHECK constraints reject out-of-range values.
	_, err := pool.Exec(t.Context(), `INSERT INTO exam (title, navigation_policy) VALUES ('bad', 'bogus')`)
	require.Error(t, err)
	_, err = pool.Exec(t.Context(), `INSERT INTO exam (title, format_version) VALUES ('bad', 0)`)
	require.Error(t, err)
	_, err = pool.Exec(t.Context(),
		`INSERT INTO exam_test (exam_id, test_id, sort_order, question_presentation) VALUES ($1, $2, 1, 'bogus')`,
		examID, qTestID)
	require.Error(t, err)

	// Down removes every column again.
	applyMigrationFile(t, pool, "0068_exam_format_policy.down.sql")
	for _, col := range []struct{ table, column string }{
		{"exam", "format_version"},
		{"exam", "navigation_policy"},
		{"exam_test", "question_presentation"},
		{"test", "shared_material_html"},
		{"test", "instructions_html"},
		{"question", "material_html"},
	} {
		var exists bool
		require.NoError(t, pool.QueryRow(t.Context(),
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
			col.table, col.column,
		).Scan(&exists))
		require.False(t, exists, "%s.%s must be dropped by down", col.table, col.column)
	}
}
