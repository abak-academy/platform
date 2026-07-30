package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// insertBankQuestion inserts a bare bank question, letting question_number come
// from the sequence DEFAULT (NFR-4), and returns its id.
func insertBankQuestion(t *testing.T, pool *pgxpool.Pool, body string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('essay', $1, 1, 0) RETURNING id`,
		body,
	).Scan(&id))
	return id
}

// FR-3: the bank list orders newest question_number first, identically across
// repeated calls.
func TestListBankQuestions_OrdersByQuestionNumberDescending(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	first := insertBankQuestion(t, pool, "Q1")
	second := insertBankQuestion(t, pool, "Q2")
	third := insertBankQuestion(t, pool, "Q3")
	want := []uuid.UUID{third, second, first}

	for attempt := 0; attempt < 2; attempt++ {
		items, _, err := r.ListBankQuestions(ctx, QuestionFilter{Limit: 20})
		require.NoError(t, err)
		require.Len(t, items, 3)

		got := make([]uuid.UUID, 3)
		for i := 0; i < 3; i++ {
			got[i] = items[i].Question.ID
		}
		require.Equal(t, want, got, "attempt %d: bank list must order question_number DESC identically across calls", attempt)

		for i := 0; i < 2; i++ {
			require.Greater(t, items[i].Question.QuestionNumber, items[i+1].Question.QuestionNumber,
				"question_number must strictly decrease down the returned page")
		}
	}
}

// FR-4: paging with a small limit over more rows than fit on one page must walk
// next_cursor to the end, visiting every row exactly once, in descending
// question_number order — the cursor predicate must key off question_number,
// not id, or this silently drops/duplicates rows.
func TestListBankQuestions_CursorWalksToEnd(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	created := make([]uuid.UUID, 5)
	for i := 0; i < 5; i++ {
		created[i] = insertBankQuestion(t, pool, "Paged Q")
	}

	var seenIDs []uuid.UUID
	var seenNumbers []int
	cursor := ""
	for page := 0; page < 10; page++ {
		items, nextCursor, err := r.ListBankQuestions(ctx, QuestionFilter{Limit: 2, Cursor: cursor})
		require.NoError(t, err)
		require.LessOrEqual(t, len(items), 2)
		for _, item := range items {
			seenIDs = append(seenIDs, item.Question.ID)
			seenNumbers = append(seenNumbers, item.Question.QuestionNumber)
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	require.Len(t, seenIDs, 5, "walking next_cursor to the end must visit exactly the 5 created questions")

	seenSet := map[uuid.UUID]int{}
	for _, id := range seenIDs {
		seenSet[id]++
	}
	for _, id := range created {
		require.Equal(t, 1, seenSet[id], "question %s must appear exactly once across the walk", id)
	}

	for i := 0; i < len(seenNumbers)-1; i++ {
		require.Greater(t, seenNumbers[i], seenNumbers[i+1], "question_number must strictly decrease across the whole walk")
	}
}
