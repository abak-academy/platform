package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigration0080_CompetitionResultUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0079_promo_active.up.sql")

	requireTableExists := func(table string, want bool) {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM information_schema.tables WHERE table_name = $1`, table).Scan(&n))
		require.Equal(t, want, n == 1, table)
	}
	requireTableExists("exam_external_round", true)
	requireTableExists("competition_result", false)

	applyMigrationFile(t, pool, "0080_competition_result.up.sql")
	requireTableExists("exam_external_round", false)
	requireTableExists("exam_external_result", false)
	requireTableExists("competition_result", true)
	requireTableExists("competition_result_row", true)

	var productID, examID, regID, resultID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price, status) VALUES ('exam', 'Edition', 0, 'published') RETURNING id`).Scan(&productID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ('E') RETURNING id`).Scan(&examID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number) VALUES ($1, $2, $3, 1) RETURNING id`,
		seedMigrationStudent(t, pool, "Siswa"), examID, "tok-"+uuid.NewString()).Scan(&regID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO competition_result (product_id) VALUES ($1) RETURNING id`, productID).Scan(&resultID))
	_, err := pool.Exec(ctx, `INSERT INTO competition_result (product_id) VALUES ($1)`, productID)
	require.Error(t, err, "one result set per product")
	_, err = pool.Exec(ctx,
		`INSERT INTO competition_result_row (result_id, registration_id, score, rank) VALUES ($1, $2, 1, 0)`, resultID, regID)
	require.Error(t, err, "rank >= 1")
	_, err = pool.Exec(ctx,
		`INSERT INTO competition_result_row (result_id, registration_id, score, certificate_url) VALUES ($1, $2, 1, 'http://x')`, resultID, regID)
	require.Error(t, err, "certificate_url must be https")
	_, err = pool.Exec(ctx,
		`INSERT INTO competition_result_row (result_id, registration_id, score, rank) VALUES ($1, $2, 90.5, 1)`, resultID, regID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, productID)
	require.NoError(t, err)
	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM competition_result_row`).Scan(&rows))
	require.Zero(t, rows, "deleting the product cascades its result set and rows")

	applyMigrationFile(t, pool, "0080_competition_result.down.sql")
	requireTableExists("competition_result", false)
	requireTableExists("competition_result_row", false)
	requireTableExists("exam_external_round", true)
	requireTableExists("exam_external_result", true)
	requireColumnExists := func(table, column string) {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, table, column).Scan(&n))
		require.Equal(t, 1, n, table+"."+column)
	}
	requireColumnExists("exam_external_round", "label")
	requireColumnExists("exam_external_round", "label_en")
	requireColumnExists("exam_external_result", "round_id")

	applyMigrationFile(t, pool, "0080_competition_result.up.sql")
	requireTableExists("competition_result_row", true)
	requireTableExists("exam_external_result", false)
}
