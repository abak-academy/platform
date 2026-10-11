package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0068_ExamFormatPolicy(t *testing.T) {
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0067_users_password_temporary.up.sql")
	applyMigrationFile(t, pool, "0068_exam_format_policy.up.sql")

	var examID, testID, questionID string
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO exam (title) VALUES ('m0068') RETURNING id`,
	).Scan(&examID))
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO test (title, subject, topic, duration_minutes, shared_material_html, instructions_html)
		 VALUES ('m0068', 's', 't', 1, '<p>m</p>', '<b>i</b>') RETURNING id`,
	).Scan(&testID))
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO question (format, body, point_correct, point_wrong, material_html)
		 VALUES ('essay', 'b', 1, 0, '<p>material</p>') RETURNING id`,
	).Scan(&questionID))
	_, err := pool.Exec(t.Context(),
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 0)`, examID, testID)
	require.NoError(t, err)

	columns := []struct {
		name  string
		query string
		id    string
		want  string
	}{
		{"format version defaults to legacy", `SELECT format_version::text FROM exam WHERE id = $1`, examID, "1"},
		{"navigation policy defaults to free", `SELECT navigation_policy FROM exam WHERE id = $1`, examID, "free"},
		{"question presentation defaults to one per page", `SELECT question_presentation FROM exam_test WHERE exam_id = $1`, examID, "one_per_page"},
		{"shared material round trips", `SELECT shared_material_html FROM test WHERE id = $1`, testID, "<p>m</p>"},
		{"instructions round trip", `SELECT instructions_html FROM test WHERE id = $1`, testID, "<b>i</b>"},
		{"question material round trips", `SELECT material_html FROM question WHERE id = $1`, questionID, "<p>material</p>"},
	}
	for _, tt := range columns {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			require.NoError(t, pool.QueryRow(t.Context(), tt.query, tt.id).Scan(&got))
			require.Equal(t, tt.want, got)
		})
	}

	constraints := []struct {
		name  string
		query string
		args  []any
	}{
		{"invalid navigation policy", `INSERT INTO exam (title, navigation_policy) VALUES ('bad', 'bogus')`, nil},
		{"non-positive format version", `INSERT INTO exam (title, format_version) VALUES ('bad', 0)`, nil},
		{"invalid question presentation", `INSERT INTO exam_test (exam_id, test_id, sort_order, question_presentation) VALUES ($1, $2, 1, 'bogus')`, []any{examID, testID}},
	}
	for _, tt := range constraints {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(t.Context(), tt.query, tt.args...)
			assertCheckViolation(t, err)
		})
	}

	applyMigrationFile(t, pool, "0068_exam_format_policy.down.sql")
	droppedColumns := []struct{ table, column string }{
		{"exam", "format_version"},
		{"exam", "navigation_policy"},
		{"exam_test", "question_presentation"},
		{"test", "shared_material_html"},
		{"test", "instructions_html"},
		{"question", "material_html"},
	}
	for _, tt := range droppedColumns {
		t.Run("down drops "+tt.table+"."+tt.column, func(t *testing.T) {
			var exists bool
			require.NoError(t, pool.QueryRow(t.Context(),
				`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
				tt.table, tt.column,
			).Scan(&exists))
			require.False(t, exists, "%s.%s must be dropped by down", tt.table, tt.column)
		})
	}
}
