package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// These tests run at the migration boundary: they seed rows that predate
// migration 0049, apply the real .up.sql, and assert on what the backfill
// produced — following migration_0037_0039_test.go (NFR-3).

// seedPre0049Question inserts a bank-only question (topic_id and correct_answer
// both optional) using only the columns that exist before 0049.
func seedPre0049Question(t *testing.T, pool *pgxpool.Pool, format, body string, correctAnswer *string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO question (format, body, correct_answer, point_correct, point_wrong)
		 VALUES ($1, $2, $3, 1, 0) RETURNING id`,
		format, body, correctAnswer,
	).Scan(&id))
	return id
}

func seedPre0049QuestionBlank(t *testing.T, pool *pgxpool.Pool, questionID uuid.UUID, blankIndex int, correctAnswer string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO question_blank (question_id, blank_index, correct_answer) VALUES ($1, $2, $3)`,
		questionID, blankIndex, correctAnswer,
	)
	require.NoError(t, err)
}

func TestMigration0049_QuestionNumberBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0046_papua_2022_regions.up.sql")

	answer := "jakarta"
	scalar := seedPre0049Question(t, pool, "short", "Capital of Indonesia?", &answer)
	multiBlank := seedPre0049Question(t, pool, "multi_blank", "Fill: __ is the capital, founded in __", nil)
	seedPre0049QuestionBlank(t, pool, multiBlank, 1, "jakarta")
	seedPre0049QuestionBlank(t, pool, multiBlank, 2, "1945")
	bare := seedPre0049Question(t, pool, "essay", "Explain photosynthesis", nil)

	applyMigrationFile(t, pool, "0049_question_authoring.up.sql")

	questionNumber := func(id uuid.UUID) int {
		t.Helper()
		var n *int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT question_number FROM question WHERE id = $1`, id,
		).Scan(&n))
		require.NotNil(t, n, "every pre-existing question must be backfilled")
		return *n
	}

	nums := map[uuid.UUID]int{
		scalar:     questionNumber(scalar),
		multiBlank: questionNumber(multiBlank),
		bare:       questionNumber(bare),
	}
	seen := map[int]bool{}
	for id, n := range nums {
		require.False(t, seen[n], "question_number must be distinct across seeded rows (question %s got %d again)", id, n)
		seen[n] = true
	}
	maxBackfilled := 0
	for _, n := range nums {
		if n > maxBackfilled {
			maxBackfilled = n
		}
	}

	// FR-2: a raw insert naming no question_number gets one from the DEFAULT,
	// strictly above every backfilled value.
	var fresh *int
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		 VALUES ('essay', 'Post-migration question', 1, 0) RETURNING question_number`,
	).Scan(&fresh))
	require.NotNil(t, fresh, "the column DEFAULT must assign a number")
	require.Greater(t, *fresh, maxBackfilled, "the sequence must continue above every backfilled question_number")

	// Uniqueness is enforced.
	_, err := pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong, question_number)
		 VALUES ('essay', 'Dup', 1, 0, $1)`, maxBackfilled,
	)
	require.Error(t, err, "a duplicate question_number must be rejected")
}

func TestMigration0049_AcceptedAnswerBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0046_papua_2022_regions.up.sql")

	answer := "42"
	scalar := seedPre0049Question(t, pool, "short", "Answer to everything?", &answer)
	noAnswer := seedPre0049Question(t, pool, "essay", "No scalar answer here", nil)
	multiBlank := seedPre0049Question(t, pool, "multi_blank", "Fill: __ and __", nil)
	seedPre0049QuestionBlank(t, pool, multiBlank, 1, "jakarta")
	seedPre0049QuestionBlank(t, pool, multiBlank, 2, "1945")

	applyMigrationFile(t, pool, "0049_question_authoring.up.sql")

	// The scalar correct_answer produced exactly one question-level row.
	rows, err := pool.Query(ctx,
		`SELECT blank_index, answer_index, answer FROM question_accepted_answer WHERE question_id = $1`, scalar,
	)
	require.NoError(t, err)
	type row struct {
		blankIndex, answerIndex int
		answer                  string
	}
	var scalarRows []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.blankIndex, &r.answerIndex, &r.answer))
		scalarRows = append(scalarRows, r)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Len(t, scalarRows, 1, "the scalar correct_answer must produce exactly one accepted-answer row")
	require.Equal(t, 0, scalarRows[0].blankIndex, "question-level accepted answers use blank_index 0")
	require.Equal(t, 1, scalarRows[0].answerIndex)
	require.Equal(t, "42", scalarRows[0].answer)

	// A question with no scalar correct_answer produces no accepted-answer row.
	var noAnswerCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question_accepted_answer WHERE question_id = $1`, noAnswer,
	).Scan(&noAnswerCount))
	require.Equal(t, 0, noAnswerCount)

	// Each question_blank row produced exactly one accepted-answer row at its blank's index.
	blankAnswer := func(blankIndex int) string {
		t.Helper()
		var a string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT answer FROM question_accepted_answer WHERE question_id = $1 AND blank_index = $2 AND answer_index = 1`,
			multiBlank, blankIndex,
		).Scan(&a))
		return a
	}
	require.Equal(t, "jakarta", blankAnswer(1))
	require.Equal(t, "1945", blankAnswer(2))

	var multiBlankCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question_accepted_answer WHERE question_id = $1`, multiBlank,
	).Scan(&multiBlankCount))
	require.Equal(t, 2, multiBlankCount, "one accepted-answer row per question_blank row, no more")

	// Cascade: deleting the question removes its accepted-answer rows.
	_, err = pool.Exec(ctx, `DELETE FROM question WHERE id = $1`, scalar)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question_accepted_answer WHERE question_id = $1`, scalar,
	).Scan(&noAnswerCount))
	require.Equal(t, 0, noAnswerCount, "question_accepted_answer must cascade on question delete")
}

func TestMigration0049_PointsAndFormatConstraints(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0046_papua_2022_regions.up.sql")
	applyMigrationFile(t, pool, "0049_question_authoring.up.sql")

	// Fractional points: 0.5 succeeds, 0 is rejected by the widened CHECK.
	_, err := pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('short', 'Half point', 0.5, 0)`,
	)
	require.NoError(t, err, "point_correct = 0.5 must be accepted after widening to NUMERIC")

	_, err = pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('short', 'Zero point', 0, 0)`,
	)
	require.Error(t, err, "point_correct = 0 must be rejected by the CHECK")

	// true_false is accepted; an arbitrary format is not.
	_, err = pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('true_false', 'T/F question', 1, 0)`,
	)
	require.NoError(t, err, "format = 'true_false' must be accepted")

	_, err = pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('nonsense', 'Bad format', 1, 0)`,
	)
	require.Error(t, err, "format = 'nonsense' must be rejected")
}

func TestMigration0049_UpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0046_papua_2022_regions.up.sql")

	answer := "seed-answer"
	q := seedPre0049Question(t, pool, "short", "Seed question", &answer)
	multiBlank := seedPre0049Question(t, pool, "multi_blank", "Seed blanks", nil)
	seedPre0049QuestionBlank(t, pool, multiBlank, 1, "one")

	applyMigrationFile(t, pool, "0049_question_authoring.up.sql")
	applyMigrationFile(t, pool, "0049_question_authoring.down.sql")

	assertColumnMissing := func(table, column string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
			table, column,
		).Scan(&exists))
		require.False(t, exists, "%s.%s must be dropped by down", table, column)
	}
	assertColumnMissing("question", "question_number")

	var tableExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'question_accepted_answer')`,
	).Scan(&tableExists))
	require.False(t, tableExists, "question_accepted_answer must be dropped by down")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'question_statement')`,
	).Scan(&tableExists))
	require.False(t, tableExists, "question_statement must be dropped by down")

	var seqExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = 'question_number_seq')`,
	).Scan(&seqExists))
	require.False(t, seqExists, "down must drop question_number_seq")

	// Pre-existing question rows and their data survive the round trip.
	var preservedAnswer *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT correct_answer FROM question WHERE id = $1`, q,
	).Scan(&preservedAnswer))
	require.NotNil(t, preservedAnswer)
	require.Equal(t, answer, *preservedAnswer)

	var blankCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question_blank WHERE question_id = $1`, multiBlank,
	).Scan(&blankCount))
	require.Equal(t, 1, blankCount, "question_blank rows must survive the round trip")

	// The old format CHECK is back: 'true_false' is rejected, 'multi_blank' still works.
	_, err := pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('true_false', 'Should fail', 1, 0)`,
	)
	require.Error(t, err, "after down, 'true_false' must be rejected again")

	// up must succeed again cleanly (NFR-2).
	applyMigrationFile(t, pool, "0049_question_authoring.up.sql")

	var n *int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT question_number FROM question WHERE id = $1`, q,
	).Scan(&n))
	require.NotNil(t, n, "the second up must re-backfill question_number")

	_, err = pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('true_false', 'Should succeed', 1, 0)`,
	)
	require.NoError(t, err, "after the second up, 'true_false' must be accepted again")
}
