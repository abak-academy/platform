package repository

import (
	"context"
	"io/fs"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	dbmigrations "akademi-bimbel/db"
)

// newMigration0025Pool starts an ephemeral Postgres container and returns a
// connected pool. The caller must not run infra.RunMigrations because the test
// needs to stop at 0024, seed pre-0025 data, then apply 0025 itself.
func newMigration0025Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

func applyMigrationFile(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()
	ctx := context.Background()
	sql, err := fs.ReadFile(dbmigrations.FS, "migrations/"+filename)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
}

// applyMigrationsUpTo applies all *.up.sql files whose name is <= stopAt
// (lexicographic order matches the zero-padded numeric prefixes).
func applyMigrationsUpTo(t *testing.T, pool *pgxpool.Pool, stopAt string) {
	t.Helper()
	entries, err := fs.Glob(dbmigrations.FS, "migrations/*.up.sql")
	require.NoError(t, err)
	sort.Strings(entries)
	for _, path := range entries {
		filename := path[len("migrations/"):]
		if filename > stopAt {
			break
		}
		applyMigrationFile(t, pool, filename)
	}
}

// seedPre0025State creates the scenario the data migration must preserve:
//   - two tests sharing the same (subject, topic) to test topic dedupe;
//   - one test with a different subject/topic;
//   - five questions across those tests with explicit sort_order;
//   - one exam_session_answer row to prove down migration does not touch history.
func seedPre0025State(t *testing.T, pool *pgxpool.Pool) (userID, sessionID uuid.UUID, testIDs []uuid.UUID, questionIDs []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		"migration-0025@test.local", "student", "Migration Test",
	).Scan(&id)
	require.NoError(t, err)
	userID = id

	var examID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, $2) RETURNING id`,
		"Migration Exam", "draft",
	).Scan(&examID)
	require.NoError(t, err)

	testRows := []struct {
		title, subject, topic string
	}{
		{"Math A", "math", "algebra"},
		{"Math B", "math", "algebra"},
		{"Physics C", "physics", "mechanics"},
	}
	for _, r := range testRows {
		err := pool.QueryRow(ctx,
			`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, $2, $3, 60) RETURNING id`,
			r.title, r.subject, r.topic,
		).Scan(&id)
		require.NoError(t, err)
		testIDs = append(testIDs, id)
	}

	// Question order per test matters for FR-6.
	questionInputs := []struct {
		testIdx    int
		sortOrder  int
		body       string
		format     string
	}{
		{0, 2, "QA2", "mcq"},
		{0, 1, "QA1", "mcq"},
		{1, 1, "QB1", "mcq"},
		{2, 1, "QC1", "mcq"},
		{1, 2, "QB2", "mcq"},
	}
	for _, q := range questionInputs {
		err := pool.QueryRow(ctx,
			`INSERT INTO question (test_id, format, body, sort_order, point_correct, point_wrong)
			VALUES ($1, $2, $3, $4, 4, 0) RETURNING id`,
			testIDs[q.testIdx], q.format, q.body, q.sortOrder,
		).Scan(&id)
		require.NoError(t, err)
		questionIDs = append(questionIDs, id)
	}

	var registrationID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		userID, examID, "tok-"+uuid.NewString(),
	).Scan(&registrationID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		registrationID, userID, examID, time.Now(),
	).Scan(&sessionID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO exam_session_answer (session_id, question_id, answer, saved_at)
		VALUES ($1, $2, $3, $4)`,
		sessionID, questionIDs[0], "A", time.Now(),
	)
	require.NoError(t, err)

	// Link all three tests to the exam so the session tree is consistent.
	for i, testID := range testIDs {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
			examID, testID, i+1,
		)
		require.NoError(t, err)
	}

	return userID, sessionID, testIDs, questionIDs
}

func TestMigration0025_QuestionBank(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Bring the DB to exactly the 0024 schema, then seed pre-0025 data.
	applyMigrationsUpTo(t, pool, "0024_auth_provider.up.sql")
	_, _, testIDs, questionIDs := seedPre0025State(t, pool)

	// Capture pre-migration counts.
	var preQuestionCount, preAnswerCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM question`).Scan(&preQuestionCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM exam_session_answer`).Scan(&preAnswerCount))

	// Apply 0025 up.
	applyMigrationFile(t, pool, "0025_question_bank.up.sql")

	// FR-4: distinct (subject, topic) pairs deduped into exam_topic.
	var topicCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM exam_topic`).Scan(&topicCount))
	require.Equal(t, 2, topicCount, "expected two distinct topics: (math,algebra) and (physics,mechanics)")

	var algebraTopicID, mechanicsTopicID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM exam_topic WHERE subject = $1 AND name = $2`, "math", "algebra",
	).Scan(&algebraTopicID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM exam_topic WHERE subject = $1 AND name = $2`, "physics", "mechanics",
	).Scan(&mechanicsTopicID))

	// FR-5/FR-6: one test_question row per question, preserving per-test order.
	var joinCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM test_question`).Scan(&joinCount))
	require.Equal(t, preQuestionCount, joinCount, "every question must map to exactly one join row")

	for i, qid := range questionIDs {
		var testID uuid.UUID
		var sortOrder int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT test_id, sort_order FROM test_question WHERE question_id = $1`, qid,
		).Scan(&testID, &sortOrder))

		switch i {
		case 0:
			require.Equal(t, testIDs[0], testID)
			require.Equal(t, 2, sortOrder)
		case 1:
			require.Equal(t, testIDs[0], testID)
			require.Equal(t, 1, sortOrder)
		case 2:
			require.Equal(t, testIDs[1], testID)
			require.Equal(t, 1, sortOrder)
		case 3:
			require.Equal(t, testIDs[2], testID)
			require.Equal(t, 1, sortOrder)
		case 4:
			require.Equal(t, testIDs[1], testID)
			require.Equal(t, 2, sortOrder)
		}
	}

	// Verify per-test ordering matches the old question.test_id/sort_order grouping.
	for _, testID := range testIDs {
		rows, err := pool.Query(ctx,
			`SELECT question_id, sort_order FROM test_question WHERE test_id = $1 ORDER BY sort_order`,
			testID,
		)
		require.NoError(t, err)
		var prevOrder int = -1
		for rows.Next() {
			var qid uuid.UUID
			var order int
			require.NoError(t, rows.Scan(&qid, &order))
			require.Greater(t, order, prevOrder, "sort_order must be strictly increasing per test")
			prevOrder = order
		}
		require.NoError(t, rows.Err())
		rows.Close()
	}

	// FR-5: topic_id backfilled from the question's former test.
	var topicID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT topic_id FROM question WHERE id = $1`, questionIDs[0],
	).Scan(&topicID))
	require.Equal(t, algebraTopicID, topicID)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT topic_id FROM question WHERE id = $1`, questionIDs[3],
	).Scan(&topicID))
	require.Equal(t, mechanicsTopicID, topicID)

	// Prove the new schema allows a bank-only question (zero test attachments).
	_, err := pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('mcq', 'Bank-only', 4, 0)`,
	)
	require.NoError(t, err)

	// Attach an existing question to a second test so down's deterministic rule is exercised.
	_, err = pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 99)`,
		testIDs[2], questionIDs[0],
	)
	require.NoError(t, err)

	var expectedTestID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT test_id FROM test_question WHERE question_id = $1 ORDER BY test_id ASC LIMIT 1`, questionIDs[0],
	).Scan(&expectedTestID))

	// Apply 0025 down.
	applyMigrationFile(t, pool, "0025_question_bank.down.sql")

	// FR-7: row counts preserved; history untouched.
	var postQuestionCount, postAnswerCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM question`).Scan(&postQuestionCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM exam_session_answer`).Scan(&postAnswerCount))
	require.Equal(t, preQuestionCount+1, postQuestionCount, "all questions including the bank-only one must survive down")
	require.Equal(t, preAnswerCount, postAnswerCount, "exam_session_answer rows must be untouched")

	// FR-7: deterministic lowest-test_id backfill.
	var backfilledTestID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT test_id FROM question WHERE id = $1`, questionIDs[0],
	).Scan(&backfilledTestID))
	require.Equal(t, expectedTestID, backfilledTestID)

	// Old schema columns are restored.
	var hasTestID, hasSortOrder, hasTopicID bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'question' AND column_name = 'test_id')`,
	).Scan(&hasTestID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'question' AND column_name = 'sort_order')`,
	).Scan(&hasSortOrder))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'question' AND column_name = 'topic_id')`,
	).Scan(&hasTopicID))
	require.True(t, hasTestID, "question.test_id must be restored by down")
	require.True(t, hasSortOrder, "question.sort_order must be restored by down")
	require.False(t, hasTopicID, "question.topic_id must be dropped by down")

	// uq_question_order must be restored — and must tolerate the bank-only
	// question's NULL test_id (Postgres treats NULLs as distinct in a UNIQUE
	// constraint), so this insert must succeed even though a NULL-test_id row
	// (the bank-only question) already exists.
	_, err = pool.Exec(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('mcq', 'Another bank-only', 4, 0)`,
	)
	require.NoError(t, err, "a second bank-only (NULL test_id) question must not violate uq_question_order")

	var uniqueConstraintExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = 'uq_question_order')`,
	).Scan(&uniqueConstraintExists))
	require.True(t, uniqueConstraintExists, "uq_question_order must be restored by down")

	// FK's ON DELETE CASCADE must be restored: deleting a test cascades to its
	// (backfilled) questions, matching the pre-0025 contract.
	var deleteRule string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT rc.delete_rule
		FROM information_schema.referential_constraints rc
		JOIN information_schema.table_constraints tc ON tc.constraint_name = rc.constraint_name
		WHERE tc.table_name = 'question' AND tc.constraint_type = 'FOREIGN KEY'`,
	).Scan(&deleteRule))
	require.Equal(t, "CASCADE", deleteRule, "question.test_id FK must restore ON DELETE CASCADE")
}
