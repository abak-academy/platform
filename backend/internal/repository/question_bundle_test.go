package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedQuestionBundleTest(t *testing.T) (*Repository, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := newGradingTestPool(t)
	testID := insertGradingTest(t, pool)
	questionID := insertGradingEssayQuestion(t, pool, testID, "Owner invalidation question", 1, 1)
	return New(pool), testID, questionID
}

func TestMigration0063StoresQuestionBundlesOnTest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	repo, _, _ := seedQuestionBundleTest(t)
	ctx := context.Background()

	var legacyTableExists bool
	require.NoError(t, repo.pool.QueryRow(ctx,
		`SELECT to_regclass('public.question_bundle') IS NOT NULL`,
	).Scan(&legacyTableExists))
	require.False(t, legacyTableExists, "0063 must not create a standalone request table")

	for _, column := range []string{
		"question_naskah_key", "question_naskah_generated_at",
		"question_kunci_key", "question_kunci_generated_at", "question_bundle_revision",
	} {
		var testColumnExists, examColumnExists bool
		require.NoError(t, repo.pool.QueryRow(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'test' AND column_name = $1
			)`, column,
		).Scan(&testColumnExists))
		require.Truef(t, testColumnExists, "test.%s must exist", column)
		require.NoError(t, repo.pool.QueryRow(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'exam' AND column_name = $1
			)`, column,
		).Scan(&examColumnExists))
		require.Falsef(t, examColumnExists, "exam.%s must not exist", column)
	}
}

func TestQuestionBundleReadyWriteDoesNotResurrectAnInvalidatedArtifact(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	repo, testID, _ := seedQuestionBundleTest(t)
	ctx := context.Background()

	owner, err := repo.GetQuestionBundleOwner(ctx, testID, "naskah")
	require.NoError(t, err)
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.ClearQuestionBundleKeysByTestTx(ctx, tx, testID))
	require.NoError(t, tx.Commit(ctx))

	written, err := repo.SetQuestionBundleReadyIfCurrent(ctx, testID, "naskah", "question-bundles/tests/stale.pdf", owner.Revision)
	require.NoError(t, err)
	require.False(t, written, "an invalidation after render start must fence out the stale PDF")

	current, err := repo.GetQuestionBundleOwner(ctx, testID, "naskah")
	require.NoError(t, err)
	require.Nil(t, current.ObjectKey)
	require.Greater(t, current.Revision, owner.Revision)
}

func TestQuestionBundleOwnerStateRoundTripsAndOverwritesDeterministicKey(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	repo, testID, _ := seedQuestionBundleTest(t)
	ctx := context.Background()
	key := "question-bundles/tests/" + testID.String() + "/naskah-r0.pdf"

	require.NoError(t, repo.SetQuestionBundleReady(ctx, testID, "naskah", key))
	state, err := repo.GetQuestionBundleOwner(ctx, testID, "naskah")
	require.NoError(t, err)
	require.NotNil(t, state.ObjectKey)
	require.Equal(t, key, *state.ObjectKey)
	require.NotNil(t, state.GeneratedAt)

	require.NoError(t, repo.SetQuestionBundleReady(ctx, testID, "naskah", key))
	overwritten, err := repo.GetQuestionBundleOwner(ctx, testID, "naskah")
	require.NoError(t, err)
	require.Equal(t, key, *overwritten.ObjectKey)
	require.False(t, overwritten.GeneratedAt.Before(*state.GeneratedAt))
}

func TestQuestionBundleInvalidationGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	t.Run("test edit clears the test", func(t *testing.T) {
		repo, testID, _ := seedQuestionBundleTest(t)
		seedQuestionBundleKeys(t, repo, testID)
		tx, err := repo.BeginTx(context.Background())
		require.NoError(t, err)
		require.NoError(t, repo.ClearQuestionBundleKeysByTestTx(context.Background(), tx, testID))
		require.NoError(t, tx.Commit(context.Background()))
		assertQuestionBundleKeys(t, repo, testID, false)
	})

	t.Run("question edit clears every containing test", func(t *testing.T) {
		repo, testID, questionID := seedQuestionBundleTest(t)
		secondTestID := insertGradingTest(t, repo.pool)
		_, err := repo.pool.Exec(context.Background(),
			`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 0)`,
			secondTestID, questionID,
		)
		require.NoError(t, err)
		seedQuestionBundleKeys(t, repo, testID, secondTestID)
		tx, err := repo.BeginTx(context.Background())
		require.NoError(t, err)
		require.NoError(t, repo.ClearQuestionBundleKeysByQuestionTx(context.Background(), tx, questionID))
		require.NoError(t, tx.Commit(context.Background()))
		assertQuestionBundleKeys(t, repo, testID, false)
		assertQuestionBundleKeys(t, repo, secondTestID, false)
	})
}

func seedQuestionBundleKeys(t *testing.T, repo *Repository, testIDs ...uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	for _, testID := range testIDs {
		for _, variant := range []string{"naskah", "kunci"} {
			key := "question-bundles/tests/" + testID.String() + "/" + variant + ".pdf"
			require.NoError(t, repo.SetQuestionBundleReady(ctx, testID, variant, key))
		}
	}
}

func assertQuestionBundleKeys(t *testing.T, repo *Repository, testID uuid.UUID, wantPresent bool) {
	t.Helper()
	for _, variant := range []string{"naskah", "kunci"} {
		state, err := repo.GetQuestionBundleOwner(context.Background(), testID, variant)
		require.NoError(t, err)
		if wantPresent {
			require.NotNilf(t, state.ObjectKey, "test %s key must be preserved", variant)
			require.NotNilf(t, state.GeneratedAt, "test %s timestamp must be preserved", variant)
		} else {
			require.Nilf(t, state.ObjectKey, "test %s key must be invalidated", variant)
			require.Nilf(t, state.GeneratedAt, "test %s timestamp must be invalidated", variant)
		}
	}
}
