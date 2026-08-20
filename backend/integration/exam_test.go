package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doJSONBody(t *testing.T, env *testEnv, method, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	resp := env.doJSON(t, method, path, body, token)
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// jsonNum reads a numeric field from a decoded JSON response map, treating a missing key
// as zero. Some struct fields (e.g. model.SessionResult's score/correct_count/empty_count)
// carry `omitempty`, so a legitimately-zero value is absent from the wire payload rather
// than serialized as JSON `0`.
func jsonNum(v any) float64 {
	if v == nil {
		return 0
	}
	return v.(float64)
}

// seedTest directly inserts a test row, returns the testID.
func seedTest(t *testing.T, env *testEnv, title, subject, topic string, duration int) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		title, subject, topic, duration,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedQuestion inserts a bank question and attaches it to the test, returning the question id.
func seedQuestion(t *testing.T, env *testEnv, testID, format, body string, sortOrder int) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body)
		 VALUES ($1, $2) RETURNING id`,
		format, body,
	).Scan(&id)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2::uuid, $3)`,
		testID, id, sortOrder,
	)
	require.NoError(t, err)
	return id
}

// ----- Test CRUD -----

func TestExam_AdminCreateTest_returns_201_and_test(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"title":            "Algebra Mid-Term",
		"subject":          "math",
		"topic":            "algebra",
		"duration_minutes": 60,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	assert.NotEmpty(t, out["id"])
	assert.Equal(t, "Algebra Mid-Term", out["title"])
	assert.Equal(t, "math", out["subject"])
	assert.Equal(t, float64(60), out["duration_minutes"])
}

// TestExam_AdminValidation_rejects merges eleven single-case tests that each stood up
// their own Postgres+Redis to assert one 4xx. The rules themselves are covered as pure
// functions by service.TestValidateQuestion_*/TestValidateTest_*; what only an
// integration test can show is that ErrValidation reaches the client as 422
// validation_failed — so every case is kept, sharing one env instead of eleven.
func TestExam_AdminValidation_rejects(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")
	testID := seedTest(t, env, "X", "math", "algebra", 60)

	t.Run("create test", func(t *testing.T) {
		cases := []struct {
			name string
			body map[string]any
		}{
			{"blank title", map[string]any{"title": "   ", "subject": "math", "topic": "algebra", "duration_minutes": 60}},
			{"zero duration", map[string]any{"title": "X", "subject": "math", "topic": "algebra", "duration_minutes": 0}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", tc.body, token)
				assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity,
					"want 400 or 422, got %d body=%v", resp.StatusCode, out)
			})
		}
	})

	t.Run("create question", func(t *testing.T) {
		cases := []struct {
			name string
			body map[string]any
		}{
			{"mcq with two correct", map[string]any{
				"format": "mcq", "body": "2+2", "sort_order": 1,
				"options": []map[string]any{
					{"key": "a", "text": "4", "is_correct": true, "sort_order": 1},
					{"key": "b", "text": "5", "is_correct": true, "sort_order": 2},
				}}},
			{"mcq with zero correct", map[string]any{
				"format": "mcq", "body": "pick one", "sort_order": 1,
				"options": []map[string]any{
					{"key": "a", "text": "1", "is_correct": false, "sort_order": 1},
					{"key": "b", "text": "2", "is_correct": false, "sort_order": 2},
				}}},
			{"mcq with only one option", map[string]any{
				"format": "mcq", "body": "pick one", "sort_order": 1,
				"options": []map[string]any{
					{"key": "a", "text": "1", "is_correct": true, "sort_order": 1},
				}}},
			{"multi_answer with zero correct", map[string]any{
				"format": "multi_answer", "body": "pick any", "sort_order": 1,
				"options": []map[string]any{
					{"key": "a", "text": "x", "is_correct": false, "sort_order": 1},
					{"key": "b", "text": "y", "is_correct": false, "sort_order": 2},
				}}},
			{"short without correct answer", map[string]any{
				"format": "short", "body": "capital of France", "sort_order": 1}},
			{"fill_blank without correct answer", map[string]any{
				"format": "fill_blank", "body": "2 + 2 = ___", "sort_order": 1}},
			{"essay with options", map[string]any{
				"format": "essay", "body": "explain", "sort_order": 1,
				"options": []map[string]any{
					{"key": "a", "text": "nope", "is_correct": false, "sort_order": 1},
				}}},
			{"point_correct below 1", map[string]any{
				"format": "essay", "body": "explain gravity", "sort_order": 1, "point_correct": 0}},
			{"negative point_wrong", map[string]any{
				"format": "essay", "body": "explain gravity", "sort_order": 1, "point_wrong": -1}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", tc.body, token)
				assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body=%v", out)
				assert.Equal(t, "validation_failed", out["code"])
			})
		}
	})
}

func TestExam_AdminListTests_returns_data_and_cursor(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	for i := 0; i < 3; i++ {
		body := map[string]any{
			"title":            fmt.Sprintf("Test %d", i),
			"subject":          "math",
			"topic":            "algebra",
			"duration_minutes": 60,
		}
		resp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", body, token)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?subject=math", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, ok := out["data"].([]any)
	require.True(t, ok, "data should be array, got %T", out["data"])
	assert.Len(t, data, 3)
	assert.NotNil(t, out["next_cursor"])
}

func TestExam_AdminListTests_includes_question_count_per_row(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "Counted", "math", "algebra", 60)
	_ = seedQuestion(t, env, testID, "mcq", "q1", 1)
	_ = seedQuestion(t, env, testID, "mcq", "q2", 2)
	_ = seedQuestion(t, env, testID, "essay", "q3", 3)
	seedTest(t, env, "Empty", "math", "algebra", 60)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?subject=math", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 2)

	countedCount := float64(-1)
	emptyCount := float64(-1)
	for _, raw := range data {
		row := raw.(map[string]any)
		qc, ok := row["question_count"].(float64)
		require.True(t, ok, "row must include question_count as number; row=%v", row)
		switch row["title"].(string) {
		case "Counted":
			countedCount = qc
		case "Empty":
			emptyCount = qc
		}
	}
	assert.Equal(t, float64(3), countedCount, "test with 3 questions should report question_count=3")
	assert.Equal(t, float64(0), emptyCount, "test with 0 questions should report question_count=0")
}

func TestExam_AdminListTests_filters_by_subject(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body1 := map[string]any{"title": "A", "subject": "math", "topic": "algebra", "duration_minutes": 60}
	resp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", body1, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body2 := map[string]any{"title": "B", "subject": "biology", "topic": "cells", "duration_minutes": 30}
	resp, _ = doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", body2, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?subject=biology", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	first := data[0].(map[string]any)
	assert.Equal(t, "biology", first["subject"])
}

func TestExam_AdminGetTest_returns_TestDetail(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	_ = seedQuestion(t, env, testID, "essay", "explain", 1)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests/"+testID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	assert.NotNil(t, out["test"])
	assert.NotNil(t, out["questions"])
}

func TestExam_AdminUpdateTest_updates_fields(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "Old Title", "math", "algebra", 60)
	body := map[string]any{"title": "New Title", "duration_minutes": 90}
	resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/tests/"+testID, body, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	assert.Equal(t, "New Title", out["title"])
	assert.Equal(t, float64(90), out["duration_minutes"])
}

func TestExam_AdminDeleteTest_returns_204_and_cascades(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	qID := seedQuestion(t, env, testID, "essay", "explain", 1)

	resp := env.doJSON(t, http.MethodDelete, "/api/v1/admin/tests/"+testID, nil, token)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Post-0025: deleting a test removes the test_question links (CASCADE) but
	// leaves the bank question intact.
	ctx := context.Background()
	var qCount int
	err := env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question WHERE id = $1`, qID,
	).Scan(&qCount)
	require.NoError(t, err)
	assert.Equal(t, 1, qCount, "bank question must survive test deletion")

	var linkCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM test_question WHERE test_id = $1`, testID,
	).Scan(&linkCount)
	require.NoError(t, err)
	assert.Equal(t, 0, linkCount, "test_question links must be cascade-deleted")
}

// ----- Question CRUD -----

func TestExam_AdminCreateQuestion_mcq_with_one_correct_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":     "mcq",
		"body":       "2+2",
		"sort_order": 1,
		"options": []map[string]any{
			{"key": "a", "text": "4", "is_correct": true, "sort_order": 1},
			{"key": "b", "text": "5", "is_correct": false, "sort_order": 2},
		},
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, "mcq", q["format"])
	options := out["options"].([]any)
	assert.Len(t, options, 2)
}

func TestExam_AdminCreateQuestion_multi_answer_with_two_correct_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":     "multi_answer",
		"body":       "pick any",
		"sort_order": 1,
		"options": []map[string]any{
			{"key": "a", "text": "x", "is_correct": true, "sort_order": 1},
			{"key": "b", "text": "y", "is_correct": true, "sort_order": 2},
		},
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, "multi_answer", q["format"])
	options := out["options"].([]any)
	assert.Len(t, options, 2)
}

func TestExam_AdminCreateQuestion_short_with_correct_answer_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":         "short",
		"body":           "capital of France",
		"sort_order":     1,
		"correct_answer": "Paris",
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, "short", q["format"])
	assert.Equal(t, "Paris", q["correct_answer"])
}

func TestExam_AdminCreateQuestion_fill_blank_with_correct_answer_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":         "fill_blank",
		"body":           "2 + 2 = ___",
		"sort_order":     1,
		"correct_answer": "4",
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, "fill_blank", q["format"])
	assert.Equal(t, "4", q["correct_answer"])
}

func TestExam_AdminPatchQuestion_nonexistent_id_returns_404(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"format":     "essay",
		"body":       "doesn't matter",
		"sort_order": 1,
	}
	// Use a non-zero UUID so the service takes the UPDATE branch (not the CREATE-on-uuid.Nil branch).
	// This exercises the UpdateQuestionTx not-found path → ErrQuestionNotFound → 404.
	fakeQuestionID := "11111111-1111-1111-1111-111111111111"
	resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/questions/"+fakeQuestionID, body, token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "body=%v", out)
	assert.Equal(t, "question_not_found", out["code"])
}

func TestExam_AdminCreateQuestion_essay_accepts_no_options_no_correct_answer_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":     "essay",
		"body":       "explain gravity",
		"sort_order": 1,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, "essay", q["format"])
	options := out["options"].([]any)
	assert.Len(t, options, 0)
}

func TestExam_AdminCreateQuestion_roundtrips_nondefault_points(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":        "essay",
		"body":          "explain gravity",
		"sort_order":    1,
		"point_correct": 5,
		"point_wrong":   3,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, float64(5), q["point_correct"])
	assert.Equal(t, float64(3), q["point_wrong"])

	resp2, out2 := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests/"+testID+"/questions", nil, token)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%v", out2)
	data := out2["data"].([]any)
	require.Len(t, data, 1)
	readQ := data[0].(map[string]any)["question"].(map[string]any)
	assert.Equal(t, float64(5), readQ["point_correct"])
	assert.Equal(t, float64(3), readQ["point_wrong"])
}

func TestExam_AdminCreateQuestion_defaults_points_when_omitted(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":     "essay",
		"body":       "explain gravity",
		"sort_order": 1,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, float64(1), q["point_correct"])
	assert.Equal(t, float64(0), q["point_wrong"])
}

// FR-16/FR-18: fractional point_correct is accepted end to end (handler no longer
// coerces to int) and echoes back unchanged.
func TestExam_AdminCreateQuestion_fractional_point_correct_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":        "essay",
		"body":          "explain gravity",
		"sort_order":    1,
		"point_correct": 1.5,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, 1.5, q["point_correct"])
}

// FR-16/FR-18: same for point_wrong.
func TestExam_AdminCreateQuestion_fractional_point_wrong_returns_201(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	body := map[string]any{
		"format":      "essay",
		"body":        "explain gravity",
		"sort_order":  1,
		"point_wrong": 0.5,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q := out["question"].(map[string]any)
	assert.Equal(t, 0.5, q["point_wrong"])
}

func TestExam_AdminUpdateQuestion_replaces_options_atomically(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	ctx := context.Background()
	qID := seedQuestion(t, env, testID, "mcq", "2+2", 1)
	_, err := env.pool.Exec(ctx,
		`INSERT INTO question_option (question_id, key, text, is_correct, sort_order)
		 VALUES ($1, 'a', '4', true, 1), ($1, 'b', '5', false, 2)`,
		qID,
	)
	require.NoError(t, err)

	// PATCH with completely different option set — old options gone, new options present
	body := map[string]any{
		"format":     "mcq",
		"body":       "3+3",
		"sort_order": 1,
		"options": []map[string]any{
			{"key": "x", "text": "6", "is_correct": true, "sort_order": 1},
			{"key": "y", "text": "7", "is_correct": false, "sort_order": 2},
		},
	}
	resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/questions/"+qID, body, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	options := out["options"].([]any)
	require.Len(t, options, 2)
	keys := []string{}
	for _, o := range options {
		keys = append(keys, o.(map[string]any)["key"].(string))
	}
	assert.Contains(t, keys, "x")
	assert.Contains(t, keys, "y")
	assert.NotContains(t, keys, "a")
}

// Task 6 relaxed the old "attached to any test" refusal: a question attached
// only to a draft test (not part of any exam, let alone a live one) is now
// deletable, and the test_question join row goes with it via ON DELETE CASCADE.
func TestExam_AdminDeleteQuestion_attachedToDraftTestOnly_returns_204(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	qID := seedQuestion(t, env, testID, "essay", "explain", 1)

	resp, out := doJSONBody(t, env, http.MethodDelete, "/api/v1/admin/questions/"+qID, nil, token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, "body=%v", out)

	ctx := context.Background()
	var exists bool
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.False(t, exists)
}

func TestExam_AdminCreateBankQuestion_returns_201_and_no_attachment(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"format":        "essay",
		"body":          "explain gravity",
		"point_correct": 2,
		"point_wrong":   0,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	q, ok := out["question"].(map[string]any)
	require.True(t, ok, "expected nested question object, got %T", out["question"])
	assert.NotEmpty(t, q["id"])
	assert.Equal(t, "essay", q["format"])
	assert.Equal(t, "explain gravity", q["body"])
	assert.Equal(t, float64(2), q["point_correct"])

	qID := q["id"].(string)
	ctx := context.Background()
	var count int
	err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM test_question WHERE question_id = $1`, qID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestExam_AdminDeleteQuestion_unattached_returns_204(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	ctx := context.Background()
	var qID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('essay', 'standalone', 1, 0) RETURNING id`,
	).Scan(&qID)
	require.NoError(t, err)

	resp := env.doJSON(t, http.MethodDelete, "/api/v1/admin/questions/"+qID, nil, token)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestExam_AdminListBankQuestions_returns_rows_and_cursor(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	ctx := context.Background()
	var topicID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO exam_topic (name, subject) VALUES ($1, 'math') RETURNING id`,
		"Algebra "+fmt.Sprintf("%d", time.Now().UnixNano()),
	).Scan(&topicID)
	require.NoError(t, err)

	uniqueToken := fmt.Sprintf("banklist-%d", time.Now().UnixNano())
	var mcqID, essayID string
	err = env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body, topic_id, point_correct, point_wrong) VALUES ('mcq', $1, $2, 1, 0) RETURNING id`,
		uniqueToken+" mcq", topicID,
	).Scan(&mcqID)
	require.NoError(t, err)
	err = env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body, topic_id, point_correct, point_wrong) VALUES ('essay', $1, $2, 1, 0) RETURNING id`,
		uniqueToken+" essay", topicID,
	).Scan(&essayID)
	require.NoError(t, err)

	testID := seedTest(t, env, "X", "math", "algebra", 60)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2::uuid, 1)`,
		testID, mcqID,
	)
	require.NoError(t, err)

	// Filter by format.
	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions?format=mcq", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	assert.Len(t, data, 1)
	row := data[0].(map[string]any)
	question := row["question"].(map[string]any)
	assert.Equal(t, mcqID, question["id"])
	assert.Equal(t, float64(1), row["attached_count"])

	// Filter by topic_id.
	resp, out = doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions?topic_id="+topicID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data = out["data"].([]any)
	assert.Len(t, data, 2)

	// Search by body.
	resp, out = doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions?search="+url.QueryEscape(uniqueToken+" essay"), nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data = out["data"].([]any)
	assert.Len(t, data, 1)
	row = data[0].(map[string]any)
	question = row["question"].(map[string]any)
	assert.Equal(t, essayID, question["id"])

	// Cursor pagination with limit 1.
	resp, out = doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions?search="+url.QueryEscape(uniqueToken)+"&limit=1", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data = out["data"].([]any)
	require.Len(t, data, 1)
	page1ID := data[0].(map[string]any)["question"].(map[string]any)["id"].(string)
	assert.NotEmpty(t, out["next_cursor"])
	cursor := out["next_cursor"].(string)

	resp, out = doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions?search="+url.QueryEscape(uniqueToken)+"&limit=1&cursor="+cursor, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data = out["data"].([]any)
	require.Len(t, data, 1)
	page2ID := data[0].(map[string]any)["question"].(map[string]any)["id"].(string)
	assert.NotEqual(t, page1ID, page2ID)
	// Both seeded questions appear across the two pages.
	assert.True(t, (page1ID == mcqID || page2ID == mcqID) && (page1ID == essayID || page2ID == essayID))
}

// Post-0025 sort_order lives on test_question and is assigned automatically when a
// question is attached, so a duplicate sort_order conflict can no longer occur
// through this endpoint. The old uq_question_order constraint is gone.

// ----- RBAC enforcement -----

func TestExam_NonAdminRole_gets_403_on_tests_endpoints(t *testing.T) {
	env := newTestEnv(t)
	studentID := seedUser(t, env, "student", "active", false)
	token := authToken(t, env, studentID, "student")

	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests",
		map[string]any{"title": "X", "subject": "math", "topic": "algebra", "duration_minutes": 60}, token)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "body=%v", out)
	assert.Equal(t, "forbidden", out["code"])

	// FR-9: q is a narrowing predicate only — it grants nothing.
	resp2, out2 := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?q=x", nil, token)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode, "body=%v", out2)
	assert.Equal(t, "forbidden", out2["code"])
}

func TestExam_NonAdminRole_gets_403_on_questions_endpoints(t *testing.T) {
	env := newTestEnv(t)
	testID := seedTest(t, env, "X", "math", "algebra", 60)
	qID := seedQuestion(t, env, testID, "essay", "explain", 1)

	studentID := seedUser(t, env, "student", "active", false)
	studentToken := authToken(t, env, studentID, "student")

	body := map[string]any{"format": "essay", "body": "x", "sort_order": 1}
	resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/questions/"+qID, body, studentToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "body=%v", out)
	assert.Equal(t, "forbidden", out["code"])
}

// ----- Exam CRUD (Slice 2) -----

func TestExam_AdminCreateExam_doesNotCreateLinkedProduct(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"title":             "Tryout Akbar",
		"timer_mode":        "per_test",
		"is_free":           true,
		"requires_checkin":  false,
		"allow_leaderboard": true,
		"randomize":         false,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)

	examID, ok := out["id"].(string)
	require.True(t, ok, "expected a flat exam object with id, got %T", out["id"])
	assert.NotEmpty(t, examID)
	assert.Nil(t, out["product"], "CreateExam must not create or return a linked product")

	ctx := context.Background()
	var productCount int
	err := env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM product_exam WHERE exam_id = $1`, examID,
	).Scan(&productCount)
	require.NoError(t, err)
	assert.Equal(t, 0, productCount, "no product_exam row should exist until an admin explicitly attaches the exam to a product")
}

func TestExam_AdminListExams_returns_data_and_cursor(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	createdIDs := map[string]bool{}
	for i := 0; i < 3; i++ {
		body := map[string]any{
			"title":      fmt.Sprintf("Paket %d", i),
			"timer_mode": "per_test",
			"is_free":    true,
		}
		resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams", body, token)
		require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
		createdIDs[out["id"].(string)] = true
	}

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	assert.GreaterOrEqual(t, len(data), 3, "list should include all 3 created exams")
	for _, raw := range data {
		row := raw.(map[string]any)
		assert.True(t, createdIDs[row["id"].(string)], "listed exam %v should be one of the 3 we created", row["id"])
	}
}

func TestExam_AdminGetExam_detail_with_and_without_tests(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	emptyBody := map[string]any{"title": "Empty Paket", "timer_mode": "per_test", "is_free": true}
	emptyResp, emptyOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams", emptyBody, token)
	require.Equal(t, http.StatusCreated, emptyResp.StatusCode)
	emptyExamID := emptyOut["id"].(string)

	emptyGet, emptyDetail := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+emptyExamID, nil, token)
	require.Equal(t, http.StatusOK, emptyGet.StatusCode, "body=%v", emptyDetail)
	testsAny, ok := emptyDetail["tests"].([]any)
	require.True(t, ok, "tests should be array, got %T", emptyDetail["tests"])
	assert.Len(t, testsAny, 0)
	assert.Equal(t, "Empty Paket", emptyDetail["title"])

	withTestResp, withTestOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Full Paket", "timer_mode": "per_test", "is_free": true}, token)
	require.Equal(t, http.StatusCreated, withTestResp.StatusCode)
	withExamID := withTestOut["id"].(string)

	testB := seedTest(t, env, "B-Test", "math", "algebra", 30)
	testA := seedTest(t, env, "A-Test", "math", "algebra", 30)
	_ = seedQuestion(t, env, testA, "essay", "a", 1)

	putBody := []string{testB, testA}
	putResp := env.doJSON(t, http.MethodPut, "/api/v1/admin/exams/"+withExamID+"/tests", putBody, token)
	require.Equal(t, http.StatusNoContent, putResp.StatusCode)
	putResp.Body.Close()

	detailResp, detail := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+withExamID, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode, "body=%v", detail)
	detailTests := detail["tests"].([]any)
	require.Len(t, detailTests, 2)
	first := detailTests[0].(map[string]any)["test"].(map[string]any)
	second := detailTests[1].(map[string]any)["test"].(map[string]any)
	assert.Equal(t, testB, first["id"], "sort_order 0 should be testB")
	assert.Equal(t, testA, second["id"], "sort_order 1 should be testA")
	assert.Equal(t, float64(0), first["question_count"], "testB has no seeded questions")
	assert.Equal(t, float64(1), second["question_count"], "testA has 1 seeded question")
}

func TestExam_AdminUpdateExam_overlays_fields(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	createResp, createOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Old", "timer_mode": "per_test", "is_free": true}, token)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	examID := createOut["id"].(string)

	scheduled := time.Date(2030, 1, 15, 10, 0, 0, 0, time.UTC)
	patchBody := map[string]any{
		"title":            "New Title",
		"scheduled_at":     scheduled.Format(time.RFC3339),
		"timer_mode":       "overall",
		"duration_minutes": 60,
	}
	patchResp, patchOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/exams/"+examID, patchBody, token)
	require.Equal(t, http.StatusOK, patchResp.StatusCode, "body=%v", patchOut)
	assert.Equal(t, "New Title", patchOut["title"])
	assert.Equal(t, "overall", patchOut["timer_mode"])
	assert.Equal(t, float64(60), patchOut["duration_minutes"])

	getResp, getOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	require.Equal(t, http.StatusOK, getResp.StatusCode, "body=%v", getOut)
	assert.Equal(t, "New Title", getOut["title"])
	assert.Equal(t, "overall", getOut["timer_mode"])
	assert.Equal(t, float64(60), getOut["duration_minutes"])
}

func TestExam_AdminUpdateExam_partialPatch_preservesBooleans(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	createResp, createOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{
			"title":             "Booleans",
			"is_free":           true,
			"requires_checkin":  true,
			"allow_leaderboard": true,
			"randomize":         true,
		}, token)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "body=%v", createOut)
	examID := createOut["id"].(string)

	patchResp, patchOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/exams/"+examID,
		map[string]any{"title": "Booleans v2"}, token)
	require.Equal(t, http.StatusOK, patchResp.StatusCode, "body=%v", patchOut)

	_, detail := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	assert.Equal(t, "Booleans v2", detail["title"])
	assert.Equal(t, true, detail["is_free"], "is_free must survive a PATCH that omits it")
	assert.Equal(t, true, detail["requires_checkin"], "requires_checkin must survive a PATCH that omits it")
	assert.Equal(t, true, detail["allow_leaderboard"], "allow_leaderboard must survive a PATCH that omits it")
	assert.Equal(t, true, detail["randomize"], "randomize must survive a PATCH that omits it")
}

func TestExam_AdminUpdateExam_ignoresLifecycleAndBundleFields(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	createResp, createOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Lifecycle"}, token)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "body=%v", createOut)
	examID := createOut["id"].(string)

	patchResp, patchOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/exams/"+examID,
		map[string]any{
			"status":              "published",
			"bundle_url":          "https://evil.example/bundle.json",
			"bundle_generated_at": time.Now().Format(time.RFC3339),
		}, token)
	require.Equal(t, http.StatusOK, patchResp.StatusCode, "body=%v", patchOut)

	_, detail := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	assert.Equal(t, "draft", detail["status"], "status is a lifecycle field — PATCH must not flip it")
	assert.Nil(t, detail["bundle_url"], "bundle_url is system-managed — PATCH must not set it")
	assert.Nil(t, detail["bundle_generated_at"], "bundle_generated_at is system-managed — PATCH must not set it")
}

func TestExam_AdminReplaceExamTests_declarative_and_bad_id_leaves_rows_intact(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	createResp, createOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Replaceable", "timer_mode": "per_test", "is_free": true}, token)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	examID := createOut["id"].(string)

	t1 := seedTest(t, env, "T1", "math", "algebra", 30)
	t2 := seedTest(t, env, "T2", "math", "algebra", 30)
	t3 := seedTest(t, env, "T3", "math", "algebra", 30)
	fakeID := "11111111-2222-3333-4444-555555555555"

	putResp := env.doJSON(t, http.MethodPut, "/api/v1/admin/exams/"+examID+"/tests",
		[]string{t1, t2}, token)
	require.Equal(t, http.StatusNoContent, putResp.StatusCode)
	putResp.Body.Close()

	detailResp, detail := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode, "body=%v", detail)
	cur := detail["tests"].([]any)
	require.Len(t, cur, 2)

	putResp2 := env.doJSON(t, http.MethodPut, "/api/v1/admin/exams/"+examID+"/tests",
		[]string{t3}, token)
	require.Equal(t, http.StatusNoContent, putResp2.StatusCode)
	putResp2.Body.Close()

	_, detail2 := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	cur2 := detail2["tests"].([]any)
	require.Len(t, cur2, 1, "after replace down to 1 test_id, list should be exactly 1")
	assert.Equal(t, t3, cur2[0].(map[string]any)["test"].(map[string]any)["id"])

	badResp, badOut := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/exams/"+examID+"/tests",
		[]string{fakeID}, token)
	assert.True(t, badResp.StatusCode == http.StatusBadRequest || badResp.StatusCode == http.StatusNotFound,
		"want 400 or 404, got %d body=%v", badResp.StatusCode, badOut)

	_, detailAfter := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID, nil, token)
	curAfter := detailAfter["tests"].([]any)
	require.Len(t, curAfter, 1, "bad test_id must not mutate exam_test rows")
	assert.Equal(t, t3, curAfter[0].(map[string]any)["test"].(map[string]any)["id"])
}

func TestExam_Product_UpdatePriceViaGenericProductEndpoint(t *testing.T) {
	env := newTestEnv(t)
	examAdminID := seedUser(t, env, "admin_exam", "active", false)
	examToken := authToken(t, env, examAdminID, "admin_exam")
	storeAdminID := seedUser(t, env, "admin_store", "active", false)
	storeToken := authToken(t, env, storeAdminID, "admin_store")

	createExamResp, createExamOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Pricing Paket", "timer_mode": "per_test", "is_free": false}, examToken)
	require.Equal(t, http.StatusCreated, createExamResp.StatusCode)
	examID := createExamOut["id"].(string)

	createProductResp, createProductOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/products",
		map[string]any{"type": "exam", "name": "Pricing Paket", "price": 0, "exam_ids": []string{examID}}, storeToken)
	require.Equal(t, http.StatusCreated, createProductResp.StatusCode, "body=%v", createProductOut)
	productID := createProductOut["id"].(string)

	// The admin Product form always submits the full editable field set (matches
	// ProductModal's actual behavior) — status is required alongside price.
	priceResp, priceOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/products/"+productID,
		map[string]any{"name": "Pricing Paket", "price": 50000, "status": "draft"}, storeToken)
	require.Equal(t, http.StatusOK, priceResp.StatusCode, "body=%v", priceOut)
	assert.Equal(t, float64(50000), priceOut["price"])

	ctx := context.Background()
	var dbPrice int64
	err := env.pool.QueryRow(ctx, `SELECT price FROM product WHERE id = $1`, productID).Scan(&dbPrice)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), dbPrice, "product.price in DB must equal 50000")
}

func TestExam_Product_PartialPatchOmittingStatus_PreservesStatus(t *testing.T) {
	env := newTestEnv(t)
	examAdminID := seedUser(t, env, "admin_exam", "active", false)
	examToken := authToken(t, env, examAdminID, "admin_exam")
	storeAdminID := seedUser(t, env, "admin_store", "active", false)
	storeToken := authToken(t, env, storeAdminID, "admin_store")

	createExamResp, createExamOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Partial Patch", "timer_mode": "per_test", "is_free": false}, examToken)
	require.Equal(t, http.StatusCreated, createExamResp.StatusCode)
	examID := createExamOut["id"].(string)

	createProductResp, createProductOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/products",
		map[string]any{"type": "exam", "name": "Partial Patch", "price": 0, "exam_ids": []string{examID}}, storeToken)
	require.Equal(t, http.StatusCreated, createProductResp.StatusCode, "body=%v", createProductOut)
	productID := createProductOut["id"].(string)
	require.Equal(t, "draft", createProductOut["status"])

	// A legitimate partial PATCH omitting status entirely must not clobber it.
	priceResp, priceOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/products/"+productID,
		map[string]any{"price": 75000}, storeToken)
	require.Equal(t, http.StatusOK, priceResp.StatusCode, "body=%v", priceOut)
	assert.Equal(t, float64(75000), priceOut["price"])
	assert.Equal(t, "draft", priceOut["status"])

	ctx := context.Background()
	var dbPrice int64
	var dbStatus string
	err := env.pool.QueryRow(ctx, `SELECT price, status FROM product WHERE id = $1`, productID).Scan(&dbPrice, &dbStatus)
	require.NoError(t, err)
	assert.Equal(t, int64(75000), dbPrice, "product.price in DB must equal 75000")
	assert.Equal(t, "draft", dbStatus, "product.status in DB must remain 'draft'")
}

func TestExam_Product_PublishViaGenericProductEndpoint(t *testing.T) {
	env := newTestEnv(t)
	examAdminID := seedUser(t, env, "admin_exam", "active", false)
	examToken := authToken(t, env, examAdminID, "admin_exam")
	storeAdminID := seedUser(t, env, "admin_store", "active", false)
	storeToken := authToken(t, env, storeAdminID, "admin_store")

	createExamResp, createExamOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/exams",
		map[string]any{"title": "Publishable", "timer_mode": "overall", "duration_minutes": 90, "is_free": false}, examToken)
	require.Equal(t, http.StatusCreated, createExamResp.StatusCode)
	examID := createExamOut["id"].(string)

	createProductResp, createProductOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/products",
		map[string]any{"type": "exam", "name": "Publishable", "price": 25000, "exam_ids": []string{examID}}, storeToken)
	require.Equal(t, http.StatusCreated, createProductResp.StatusCode, "body=%v", createProductOut)
	productID := createProductOut["id"].(string)

	pubResp, pubOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/products/"+productID+"/publish",
		nil, storeToken)
	require.Equal(t, http.StatusOK, pubResp.StatusCode, "body=%v", pubOut)

	ctx := context.Background()
	var dbStatus string
	err := env.pool.QueryRow(ctx, `SELECT status FROM product WHERE id = $1`, productID).Scan(&dbStatus)
	require.NoError(t, err)
	assert.Equal(t, "published", dbStatus, "product.status in DB must equal 'published'")
}

// ----- Slice 5 (Task 8): result endpoint + essay grading, end-to-end -----
//
// AdminCreateExam is a known, pre-existing, out-of-scope defect on this branch (empty
// req.ResultConfig binds to "" and violates the migration-0015 CHECK(result_config IN (...))
// constraint on INSERT, 500ing every one of the 7 TestExam_Admin{CreateExam,ListExams,GetExam,
// UpdateExam,ReplaceExamTests}_* and TestExam_Product_{UpdatePrice,Publish}ViaGenericProductEndpoint
// tests above). The tests below seed `exam`/`exam_test`/`exam_registration` directly via SQL (same
// pattern seedTest/seedQuestion already use) to sidestep that defect entirely and exercise only the
// Slice 5 student session + result + admin grading endpoints, which do not go through AdminCreateExam.

// seedQuestionWithPoints inserts a question row with explicit point_correct/point_wrong
// (seedQuestion always takes the column defaults of 1/0) and returns its ID.
func seedQuestionWithPoints(t *testing.T, env *testEnv, testID, format, body string, sortOrder, pointCorrect, pointWrong int) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		format, body, pointCorrect, pointWrong,
	).Scan(&id)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2::uuid, $3)`,
		testID, id, sortOrder,
	)
	require.NoError(t, err)
	return id
}

// seedMCQOptions inserts two options ("a", "b") for a mcq question; correctKey marks which
// option key is_correct=true.
func seedMCQOptions(t *testing.T, env *testEnv, questionID, correctKey string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO question_option (question_id, key, text, is_correct, sort_order)
		 VALUES ($1, 'a', 'Option A', $2, 1), ($1, 'b', 'Option B', $3, 2)`,
		questionID, correctKey == "a", correctKey == "b",
	)
	require.NoError(t, err)
}

// seedExam inserts an exam row directly via SQL (bypassing the broken AdminCreateExam —
// see comment above), giving the caller full control over result_config/result_release_at.
func seedExam(t *testing.T, env *testEnv, title, resultConfig string, resultReleaseAt *time.Time) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO exam (title, is_free, requires_checkin, timer_mode, result_config, result_release_at)
		 VALUES ($1, true, false, 'per_test', $2, $3) RETURNING id`,
		title, resultConfig, resultReleaseAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// linkExamTest inserts an exam_test join row.
func linkExamTest(t *testing.T, env *testEnv, examID, testID string, sortOrder int) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
		examID, testID, sortOrder,
	)
	require.NoError(t, err)
}

// seedExamRegistration inserts an exam_registration row for a student, returns registrationID.
func seedExamRegistration(t *testing.T, env *testEnv, studentID, examID string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	token := fmt.Sprintf("tok-%d", time.Now().UnixNano())
	err := env.pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, token,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// startSubmitSession drives the student session flow over HTTP (start -> save answers ->
// submit) and returns the sessionID.
func startSubmitSession(t *testing.T, env *testEnv, studentToken, registrationID string, answers []map[string]any) string {
	t.Helper()
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/exam/sessions",
		map[string]any{"registration_id": registrationID}, studentToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, "start session body=%v", out)
	sessionID, ok := out["session_id"].(string)
	require.True(t, ok, "expected session_id string, got %T", out["session_id"])

	saveResp, saveOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/exam/sessions/"+sessionID+"/answers",
		map[string]any{"answers": answers}, studentToken)
	require.Equal(t, http.StatusOK, saveResp.StatusCode, "save answers body=%v", saveOut)

	submitResp, submitOut := doJSONBody(t, env, http.MethodPost, "/api/v1/exam/sessions/"+sessionID+"/submit", nil, studentToken)
	require.Equal(t, http.StatusOK, submitResp.StatusCode, "submit body=%v", submitOut)
	require.Equal(t, "submitted", submitOut["status"])

	return sessionID
}

// TestExam_EssayFlow_EndToEnd_GradingAndResultGating drives the full student+admin flow for
// a session with one wrong mcq and one ungraded essay: register/start/submit (asserting the
// essay's graded_at stays NULL post-submit — PG note 1 regression), the admin grading queue +
// essays read, grading the essay (asserting the session total is recomputed by folding raw
// persisted per-answer scores and re-clamped at 0, not just added on top of the already-
// clamped submit-time total), the result endpoint's grading->result gate transition, the
// score_pembahasan breakdown/pembahasan visibility, and the ownership 404.
func TestExam_EssayFlow_EndToEnd_GradingAndResultGating(t *testing.T) {
	env := newTestEnv(t)

	adminID := seedUser(t, env, "admin_exam", "active", false)
	adminToken := authToken(t, env, adminID, "admin_exam")
	studentID := seedUser(t, env, "student", "active", false)
	studentToken := authToken(t, env, studentID, "student")

	testID := seedTest(t, env, "Essay Mix", "bahasa", "menulis", 60)
	mcqQID := seedQuestionWithPoints(t, env, testID, "mcq", "2+2", 1, 1, 8)
	seedMCQOptions(t, env, mcqQID, "a")
	essayQID := seedQuestionWithPoints(t, env, testID, "essay", "Explain gravity", 2, 5, 0)

	examID := seedExam(t, env, "Essay Paket", "score_pembahasan", nil)
	linkExamTest(t, env, examID, testID, 0)
	regID := seedExamRegistration(t, env, studentID, examID)

	sessionID := startSubmitSession(t, env, studentToken, regID, []map[string]any{
		{"question_id": mcqQID, "answer": "b"}, // wrong: correct key is "a"
		{"question_id": essayQID, "answer": "my essay answer text"},
	})

	ctx := context.Background()

	t.Run("submit leaves the essay ungraded (PG note 1 fix) and grades the mcq", func(t *testing.T) {
		var essayAnswer string
		var essayGradedAt *time.Time
		err := env.pool.QueryRow(ctx,
			`SELECT answer, graded_at FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
			sessionID, essayQID,
		).Scan(&essayAnswer, &essayGradedAt)
		require.NoError(t, err)
		assert.Equal(t, "my essay answer text", essayAnswer)
		assert.Nil(t, essayGradedAt, "essay graded_at must stay NULL right after submit")

		var mcqScore float64
		var mcqCorrect bool
		var mcqGradedAt *time.Time
		err = env.pool.QueryRow(ctx,
			`SELECT score, is_correct, graded_at FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
			sessionID, mcqQID,
		).Scan(&mcqScore, &mcqCorrect, &mcqGradedAt)
		require.NoError(t, err)
		assert.Equal(t, -8.0, mcqScore, "wrong mcq stores the raw negative point_wrong magnitude, unclamped")
		assert.False(t, mcqCorrect)
		assert.NotNil(t, mcqGradedAt, "objective answers are graded at submit time")
	})

	t.Run("result is gated to grading while the essay is ungraded", func(t *testing.T) {
		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, studentToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "grading", out["state"])
	})

	t.Run("admin grading queue lists the session with its ungraded essay count", func(t *testing.T) {
		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams/"+examID+"/grading", nil, adminToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		data, ok := out["data"].([]any)
		require.True(t, ok, "expected data array, got %T", out["data"])
		require.Len(t, data, 1)
		row := data[0].(map[string]any)
		assert.Equal(t, sessionID, row["session_id"])
		assert.Equal(t, studentID, row["student_id"])
		assert.Equal(t, float64(1), row["ungraded_essay_count"])
	})

	t.Run("admin session essays read returns the ungraded essay with question metadata", func(t *testing.T) {
		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/sessions/"+sessionID+"/essays", nil, adminToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		data, ok := out["data"].([]any)
		require.True(t, ok, "expected data array, got %T", out["data"])
		require.Len(t, data, 1)
		row := data[0].(map[string]any)
		assert.Equal(t, essayQID, row["question_id"])
		assert.Equal(t, "Explain gravity", row["body"])
		assert.Equal(t, "my essay answer text", row["answer"])
		assert.Equal(t, float64(5), row["point_correct"])
		assert.Nil(t, row["score"])
		assert.Nil(t, row["graded_at"])
	})

	t.Run("grading the essay recomputes the session total by folding raw scores and re-clamps", func(t *testing.T) {
		resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/sessions/"+sessionID+"/grade",
			map[string]any{"question_id": essayQID, "score": 2, "comment": "Needs more detail"}, adminToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "ok", out["status"])
		// Raw persisted scores are mcq=-8 and essay=2 -> fold = -6 -> clamp(0, -6) = 0.
		// A buggy "add on top of the already-clamped submit total" (0 + 2 = 2) would fail this.
		assert.Equal(t, float64(0), out["score"], "recompute must fold raw persisted scores, not add onto the clamped submit total")

		var gradedAt *time.Time
		var score float64
		var comment string
		var gradedBy string
		err := env.pool.QueryRow(ctx,
			`SELECT graded_at, score, grader_comment, graded_by FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
			sessionID, essayQID,
		).Scan(&gradedAt, &score, &comment, &gradedBy)
		require.NoError(t, err)
		assert.NotNil(t, gradedAt, "essay graded_at must be set after grading")
		assert.Equal(t, 2.0, score)
		assert.Equal(t, "Needs more detail", comment)
		assert.Equal(t, adminID, gradedBy)

		var sessScore float64
		err = env.pool.QueryRow(ctx, `SELECT score FROM exam_session WHERE id = $1`, sessionID).Scan(&sessScore)
		require.NoError(t, err)
		assert.Equal(t, 0.0, sessScore, "exam_session.score must persist the recomputed clamped total")
	})

	t.Run("result transitions to result state with score_pembahasan breakdown and pembahasan", func(t *testing.T) {
		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, studentToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "result", out["state"])
		assert.Equal(t, "score_pembahasan", out["result_config"])
		// score/correct_count/empty_count carry `omitempty` on model.SessionResult, so a
		// legitimate zero value is absent from the wire payload rather than JSON `0`.
		assert.Equal(t, float64(0), jsonNum(out["score"]))
		assert.Equal(t, float64(0), jsonNum(out["correct_count"]))
		assert.Equal(t, float64(1), out["wrong_count"])
		assert.Equal(t, float64(0), jsonNum(out["empty_count"]))
		assert.Equal(t, float64(1), out["rank"], "only fully-graded submitted session for this exam")

		breakdown, ok := out["breakdown"].([]any)
		require.True(t, ok, "expected breakdown array, got %T", out["breakdown"])
		require.Len(t, breakdown, 1)
		row := breakdown[0].(map[string]any)
		assert.Equal(t, testID, row["test_id"])
		assert.Equal(t, -6.0, row["earned"], "breakdown earned is the raw unclamped fold (mcq -8 + essay 2)")
		assert.Equal(t, float64(6), row["max"], "max sums point_correct across the test (mcq 1 + essay 5)")

		pembahasan, ok := out["pembahasan"].([]any)
		require.True(t, ok, "expected pembahasan array, got %T", out["pembahasan"])
		require.Len(t, pembahasan, 1, "essay is excluded from pembahasan, only the mcq remains")
		pRow := pembahasan[0].(map[string]any)
		assert.Equal(t, mcqQID, pRow["question_id"])
		assert.Equal(t, "b", pRow["your_answer"])
		assert.Equal(t, "a", pRow["correct_answer"])
		assert.Equal(t, false, pRow["is_correct"])
	})

	t.Run("result is 404 session_not_found for a different student", func(t *testing.T) {
		otherStudentID := seedUser(t, env, "student", "active", false)
		otherToken := authToken(t, env, otherStudentID, "student")

		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, otherToken)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "session_not_found", out["code"])
	})
}

// TestExam_Result_ScoreOnly_HidesBreakdownAndPembahasan covers FR-S5-22: score_only never
// includes breakdown/pembahasan, only score/counts/rank.
func TestExam_Result_ScoreOnly_HidesBreakdownAndPembahasan(t *testing.T) {
	env := newTestEnv(t)

	studentID := seedUser(t, env, "student", "active", false)
	studentToken := authToken(t, env, studentID, "student")

	testID := seedTest(t, env, "Score Only", "math", "aljabar", 30)
	mcqQID := seedQuestionWithPoints(t, env, testID, "mcq", "2+2", 1, 3, 1)
	seedMCQOptions(t, env, mcqQID, "a")

	examID := seedExam(t, env, "Score Only Paket", "score_only", nil)
	linkExamTest(t, env, examID, testID, 0)
	regID := seedExamRegistration(t, env, studentID, examID)

	sessionID := startSubmitSession(t, env, studentToken, regID, []map[string]any{
		{"question_id": mcqQID, "answer": "a"}, // correct
	})

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, studentToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	assert.Equal(t, "result", out["state"])
	assert.Equal(t, "score_only", out["result_config"])
	assert.Equal(t, float64(3), out["score"])
	assert.Equal(t, float64(1), out["correct_count"])
	assert.Equal(t, float64(1), out["rank"])

	_, hasBreakdown := out["breakdown"]
	assert.False(t, hasBreakdown, "score_only must not include a breakdown key")
	_, hasPembahasan := out["pembahasan"]
	assert.False(t, hasPembahasan, "score_only must not include a pembahasan key")
}

// TestExam_Result_HiddenAndLocked_Gates covers FR-S5-21 gates 1 and 3: hidden always wins,
// and a future result_release_at locks an otherwise-visible result.
func TestExam_Result_HiddenAndLocked_Gates(t *testing.T) {
	env := newTestEnv(t)

	testID := seedTest(t, env, "Gates", "math", "aljabar", 30)
	mcqQID := seedQuestionWithPoints(t, env, testID, "mcq", "2+2", 1, 1, 0)
	seedMCQOptions(t, env, mcqQID, "a")

	t.Run("hidden", func(t *testing.T) {
		studentID := seedUser(t, env, "student", "active", false)
		studentToken := authToken(t, env, studentID, "student")

		examID := seedExam(t, env, "Hidden Paket", "hidden", nil)
		linkExamTest(t, env, examID, testID, 0)
		regID := seedExamRegistration(t, env, studentID, examID)

		sessionID := startSubmitSession(t, env, studentToken, regID, []map[string]any{
			{"question_id": mcqQID, "answer": "a"},
		})

		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, studentToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "hidden", out["state"])
	})

	t.Run("locked", func(t *testing.T) {
		studentID := seedUser(t, env, "student", "active", false)
		studentToken := authToken(t, env, studentID, "student")

		releaseAt := time.Now().Add(24 * time.Hour)
		examID := seedExam(t, env, "Locked Paket", "score_only", &releaseAt)
		linkExamTest(t, env, examID, testID, 0)
		regID := seedExamRegistration(t, env, studentID, examID)

		sessionID := startSubmitSession(t, env, studentToken, regID, []map[string]any{
			{"question_id": mcqQID, "answer": "a"},
		})

		resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+sessionID+"/result", nil, studentToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
		assert.Equal(t, "locked", out["state"])
		assert.NotEmpty(t, out["result_release_at"])
	})
}

// TestExam_Result_Rank_MultipleSessionsWithTies covers FR-S5-18 end-to-end: rank is 1 +
// count of strictly-higher fully-graded submitted sessions for the same exam, and ties share
// a rank.
func TestExam_Result_Rank_MultipleSessionsWithTies(t *testing.T) {
	env := newTestEnv(t)

	testID := seedTest(t, env, "Ranked", "math", "aljabar", 30)
	q1 := seedQuestionWithPoints(t, env, testID, "mcq", "q1", 1, 1, 0)
	seedMCQOptions(t, env, q1, "a")
	q2 := seedQuestionWithPoints(t, env, testID, "mcq", "q2", 2, 1, 0)
	seedMCQOptions(t, env, q2, "a")

	examID := seedExam(t, env, "Ranked Paket", "score_only", nil)
	linkExamTest(t, env, examID, testID, 0)

	newSession := func(bothCorrect bool) (string, string) {
		studentID := seedUser(t, env, "student", "active", false)
		studentToken := authToken(t, env, studentID, "student")
		regID := seedExamRegistration(t, env, studentID, examID)
		secondAnswer := "b"
		if bothCorrect {
			secondAnswer = "a"
		}
		sessionID := startSubmitSession(t, env, studentToken, regID, []map[string]any{
			{"question_id": q1, "answer": "a"},
			{"question_id": q2, "answer": secondAnswer},
		})
		return sessionID, studentToken
	}

	topSession, topToken := newSession(true)    // score 2
	midSessionB, midTokenB := newSession(false) // score 1
	midSessionC, midTokenC := newSession(false) // score 1, tied with B
	_ = midSessionB
	_ = midSessionC

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+topSession+"/result", nil, topToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	assert.Equal(t, float64(2), out["score"])
	assert.Equal(t, float64(1), out["rank"], "top scorer has no strictly-higher sessions")

	respB, outB := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+midSessionB+"/result", nil, midTokenB)
	require.Equal(t, http.StatusOK, respB.StatusCode, "body=%v", outB)
	assert.Equal(t, float64(1), outB["score"])
	assert.Equal(t, float64(2), outB["rank"], "one strictly-higher session (top scorer)")

	respC, outC := doJSONBody(t, env, http.MethodGet, "/api/v1/exam/sessions/"+midSessionC+"/result", nil, midTokenC)
	require.Equal(t, http.StatusOK, respC.StatusCode, "body=%v", outC)
	assert.Equal(t, float64(1), outC["score"])
	assert.Equal(t, float64(2), outC["rank"], "tied with session B, shares the same rank")
}

// TestExam_MultiBlankQuestion_persist_and_read tests that multi_blank questions with blanks
// are created via HTTP, persisted correctly, and read back with all blanks intact.
func TestExam_MultiBlankQuestion_persist_and_read(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	// Create a multi_blank question with 2 blanks via bank question endpoint
	body := map[string]any{
		"format": "multi_blank",
		"body":   "Ibu kota Indonesia adalah {{1}}, didirikan tahun {{2}}.",
		"blanks": []map[string]any{
			{"index": 1, "correct_answer": "Jakarta"},
			{"index": 2, "correct_answer": "1945"},
		},
		"point_correct": 2,
		"point_wrong":   0,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)

	// Extract the question ID from response
	questionData := out["question"].(map[string]any)
	questionID := questionData["id"].(string)
	assert.NotEmpty(t, questionID)

	// Verify blanks are in the response
	blanksData := out["blanks"].([]any)
	require.Len(t, blanksData, 2, "should have 2 blanks")

	blank1 := blanksData[0].(map[string]any)
	assert.Equal(t, float64(1), blank1["index"])
	assert.Equal(t, "Jakarta", blank1["correct_answer"])

	blank2 := blanksData[1].(map[string]any)
	assert.Equal(t, float64(2), blank2["index"])
	assert.Equal(t, "1945", blank2["correct_answer"])

	// Create a test and attach the question to it
	testBody := map[string]any{
		"title":            "Integration Test",
		"subject":          "geography",
		"topic":            "basic",
		"duration_minutes": 30,
	}
	testResp, testOut := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests", testBody, token)
	require.Equal(t, http.StatusCreated, testResp.StatusCode)
	testID := testOut["id"].(string)

	// Attach the question to the test
	attachBody := map[string]any{
		"question_id": questionID,
	}
	attachResp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions/attach", attachBody, token)
	require.Equal(t, http.StatusNoContent, attachResp.StatusCode)

	// Read the test detail and verify blanks are returned
	detailResp, detailOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests/"+testID, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)

	questionsData := detailOut["questions"].([]any)
	require.Len(t, questionsData, 1)

	questionDetail := questionsData[0].(map[string]any)
	detailedQuestion := questionDetail["question"].(map[string]any)
	assert.Equal(t, "multi_blank", detailedQuestion["format"])

	// Verify blanks in detail read
	returnedBlanks := questionDetail["blanks"].([]any)
	require.Len(t, returnedBlanks, 2, "blanks should be returned in test detail")

	rb1 := returnedBlanks[0].(map[string]any)
	assert.Equal(t, float64(1), rb1["index"])
	assert.Equal(t, "Jakarta", rb1["correct_answer"])

	rb2 := returnedBlanks[1].(map[string]any)
	assert.Equal(t, float64(2), rb2["index"])
	assert.Equal(t, "1945", rb2["correct_answer"])
}

// TestExam_QuestionAudioURL_persists_and_reads_back covers a regression: audio_url was
// correctly written by CreateQuestionTx/UpdateQuestionTx but ListQuestions/ListBankQuestions
// never selected/scanned it back, so every read path (bank list, test detail) silently
// dropped a saved audio_url.
func TestExam_QuestionAudioURL_persists_and_reads_back(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"format":         "short",
		"body":           "What is the capital of Indonesia?",
		"correct_answer": "Jakarta",
		"audio_url":      "https://example.com/clip.mp3",
		"point_correct":  1,
		"point_wrong":    0,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)

	questionData := out["question"].(map[string]any)
	questionID := questionData["id"].(string)
	assert.Equal(t, "https://example.com/clip.mp3", questionData["audio_url"], "create response should echo audio_url")

	// Bank list read path (ListBankQuestions).
	listResp, listOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions", nil, token)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	items := listOut["data"].([]any)
	var found map[string]any
	for _, item := range items {
		q := item.(map[string]any)["question"].(map[string]any)
		if q["id"] == questionID {
			found = q
			break
		}
	}
	require.NotNil(t, found, "created question should appear in bank list")
	assert.Equal(t, "https://example.com/clip.mp3", found["audio_url"], "bank list should return audio_url")

	// Test-detail read path (ListQuestions via GetTestDetail).
	testID := seedTest(t, env, "Audio URL Test", "geography", "capitals", 30)
	attachResp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions/attach", map[string]any{"question_id": questionID}, token)
	require.Equal(t, http.StatusNoContent, attachResp.StatusCode)

	detailResp, detailOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests/"+testID, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)
	questionsData := detailOut["questions"].([]any)
	require.Len(t, questionsData, 1)
	detailedQuestion := questionsData[0].(map[string]any)["question"].(map[string]any)
	assert.Equal(t, "https://example.com/clip.mp3", detailedQuestion["audio_url"], "test detail should return audio_url")
}

// TestExam_TrueFalseStatements_persist_and_read_back covers the same regression shape as
// audio_url above, for true_false statements (FR-29): the child rows are written by
// CreateQuestionTx/UpdateQuestionTx, but each read path has to hydrate them separately.
// Dropping the hydration in ListQuestions leaves the admin test-detail page with a
// statement-less true_false question while the bank list still looks correct.
func TestExam_TrueFalseStatements_persist_and_read_back(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	body := map[string]any{
		"format": "true_false",
		"body":   "Decide whether each statement is true.",
		"statements": []map[string]any{
			{"index": 1, "body": "Water boils at 100C at sea level.", "is_true": true},
			{"index": 2, "body": "The sun orbits the earth.", "is_true": false},
			{"index": 3, "body": "Jakarta is the capital of Indonesia.", "is_true": true},
		},
		"point_correct": 1,
		"point_wrong":   0.5,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)

	questionData := out["question"].(map[string]any)
	questionID := questionData["id"].(string)

	assertStatements := func(t *testing.T, q map[string]any, where string) {
		t.Helper()
		raw, ok := q["statements"]
		require.NotNil(t, raw, "%s should return statements", where)
		sts, ok := raw.([]any)
		require.True(t, ok, "%s statements should be an array", where)
		require.Len(t, sts, 3, "%s should return all 3 statements", where)

		// Index order and the answer key must both survive the round trip.
		wantBodies := []string{
			"Water boils at 100C at sea level.",
			"The sun orbits the earth.",
			"Jakarta is the capital of Indonesia.",
		}
		wantTruth := []bool{true, false, true}
		for i, s := range sts {
			st := s.(map[string]any)
			assert.Equal(t, float64(i+1), st["index"], "%s statement %d index", where, i+1)
			assert.Equal(t, wantBodies[i], st["body"], "%s statement %d body", where, i+1)
			assert.Equal(t, wantTruth[i], st["is_true"], "%s statement %d is_true", where, i+1)
		}
	}

	assertStatements(t, questionData, "create response")

	// Bank list read path (ListBankQuestions).
	listResp, listOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/questions", nil, token)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	items := listOut["data"].([]any)
	var found map[string]any
	for _, item := range items {
		q := item.(map[string]any)["question"].(map[string]any)
		if q["id"] == questionID {
			found = q
			break
		}
	}
	require.NotNil(t, found, "created question should appear in bank list")
	assertStatements(t, found, "bank list")

	// Test-detail read path (ListQuestions via GetTestDetail).
	testID := seedTest(t, env, "True False Test", "science", "general", 30)
	attachResp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions/attach", map[string]any{"question_id": questionID}, token)
	require.Equal(t, http.StatusNoContent, attachResp.StatusCode)

	detailResp, detailOut := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests/"+testID, nil, token)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)
	questionsData := detailOut["questions"].([]any)
	require.Len(t, questionsData, 1)
	detailedQuestion := questionsData[0].(map[string]any)["question"].(map[string]any)
	assertStatements(t, detailedQuestion, "test detail")
}

// TestExam_MultiBlankQuestion_non_multi_blank_has_empty_blanks tests that non-multi_blank
// questions return an empty blanks slice, not nil.
func TestExam_MultiBlankQuestion_non_multi_blank_has_empty_blanks(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	testID := seedTest(t, env, "Test", "math", "algebra", 30)

	// Create a regular MCQ question
	body := map[string]any{
		"format": "mcq",
		"body":   "What is 2+2?",
		"options": []map[string]any{
			{"key": "a", "text": "3", "is_correct": false, "sort_order": 0},
			{"key": "b", "text": "4", "is_correct": true, "sort_order": 1},
		},
		"point_correct": 1,
		"point_wrong":   0,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/tests/"+testID+"/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify blanks is an empty array, not nil
	blanks := out["blanks"]
	require.NotNil(t, blanks, "blanks should not be nil")
	blanksArray := blanks.([]any)
	assert.Len(t, blanksArray, 0, "non-multi_blank should have empty blanks array")
}

// TestExam_MultiBlankQuestion_update_reduces_blanks tests that updating a multi_blank
// question to have fewer blanks removes stale rows.
func TestExam_MultiBlankQuestion_update_reduces_blanks(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	// Create a multi_blank question with 3 blanks
	body := map[string]any{
		"format": "multi_blank",
		"body":   "A {{1}} B {{2}} C {{3}}",
		"blanks": []map[string]any{
			{"index": 1, "correct_answer": "one"},
			{"index": 2, "correct_answer": "two"},
			{"index": 3, "correct_answer": "three"},
		},
		"point_correct": 3,
		"point_wrong":   0,
	}
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/questions", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	questionID := out["question"].(map[string]any)["id"].(string)

	// Verify initial 3 blanks
	initialBlanks := out["blanks"].([]any)
	require.Len(t, initialBlanks, 3)

	// Update the question to have only 2 blanks
	updateBody := map[string]any{
		"format": "multi_blank",
		"body":   "A {{1}} B {{2}}",
		"blanks": []map[string]any{
			{"index": 1, "correct_answer": "uno"},
			{"index": 2, "correct_answer": "dos"},
		},
		"point_correct": 2,
		"point_wrong":   0,
	}
	updateResp, updateOut := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/questions/"+questionID, updateBody, token)
	require.Equal(t, http.StatusOK, updateResp.StatusCode)

	// Verify updated blanks - should have exactly 2
	updatedBlanks := updateOut["blanks"].([]any)
	require.Len(t, updatedBlanks, 2, "should have reduced to 2 blanks after update")

	// Verify the values are the new ones
	blank1 := updatedBlanks[0].(map[string]any)
	assert.Equal(t, "uno", blank1["correct_answer"])

	blank2 := updatedBlanks[1].(map[string]any)
	assert.Equal(t, "dos", blank2["correct_answer"])
}

// ----- q/status filters + registration_count (PR #104 follow-up, FR-1..FR-10, FR-22..FR-27) -----

func TestExam_AdminListTests_q_filters_by_title_substring(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	seedTest(t, env, "Aljabar Dasar", "math", "algebra", 60)
	seedTest(t, env, "Sel dan Jaringan", "biology", "cells", 30)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?q=Aljabar", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "Aljabar Dasar", data[0].(map[string]any)["title"])
}

// TestExam_AdminListTests_q_finds_a_row_beyond_the_default_limit is FR-4: the
// title predicate must apply before LIMIT, or a marker beyond the default
// page of 20 could never be found without a client-side scan of everything.
func TestExam_AdminListTests_q_finds_a_row_beyond_the_default_limit(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	ids := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		ids = append(ids, seedTest(t, env, fmt.Sprintf("Plain %d", i), "math", "algebra", 60))
	}
	sort.Strings(ids)
	marker := fmt.Sprintf("Marker-%d", time.Now().UnixNano())
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `UPDATE test SET title = $1 WHERE id = $2`, marker, ids[24])
	require.NoError(t, err)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?q="+marker, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, ids[24], data[0].(map[string]any)["id"])
}

func TestExam_AdminListTests_q_composes_with_subject(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	seedTest(t, env, "Aljabar Dasar", "math", "algebra", 60)   // matches both
	seedTest(t, env, "Aljabar Lanjut", "biology", "cells", 60) // matches q, not subject
	seedTest(t, env, "Geometri Dasar", "math", "geometry", 60) // matches subject, not q

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests?subject=math&q=Aljabar", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "Aljabar Dasar", data[0].(map[string]any)["title"])
}

func TestExam_AdminListTests_absent_q_returns_the_unfiltered_set(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	seedTest(t, env, "A", "math", "algebra", 60)
	seedTest(t, env, "B", "biology", "cells", 30)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/tests", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	assert.GreaterOrEqual(t, len(data), 2)
}

func TestExam_AdminListExams_q_filters_by_title_substring(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	examA := seedExam(t, env, "Tryout Matematika", "score_pembahasan", nil)
	seedExam(t, env, "Tryout Bahasa", "score_pembahasan", nil)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams?q=Matematika", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, examA, data[0].(map[string]any)["id"])
}

func TestExam_AdminListExams_status_filters_by_status(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	draftID := seedExam(t, env, "Draft Paket", "score_pembahasan", nil)
	publishedID := seedExam(t, env, "Published Paket", "score_pembahasan", nil)
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `UPDATE exam SET status = 'published' WHERE id = $1`, publishedID)
	require.NoError(t, err)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams?status=draft", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, draftID, data[0].(map[string]any)["id"])

	respAll, outAll := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, token)
	require.Equal(t, http.StatusOK, respAll.StatusCode, "body=%v", outAll)
	assert.GreaterOrEqual(t, len(outAll["data"].([]any)), 2, "absent status returns every status")
}

func TestExam_AdminListExams_q_and_status_compose(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	matchBoth := seedExam(t, env, "Tryout Fisika", "score_pembahasan", nil)
	matchQOnly := seedExam(t, env, "Tryout Fisika Draft", "score_pembahasan", nil)
	matchStatusOnly := seedExam(t, env, "Tryout Kimia", "score_pembahasan", nil)
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `UPDATE exam SET status = 'published' WHERE id IN ($1, $2)`, matchBoth, matchStatusOnly)
	require.NoError(t, err)
	_ = matchQOnly

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams?q=Fisika&status=published", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	data := out["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, matchBoth, data[0].(map[string]any)["id"])
}

func TestExam_AdminListExams_registration_count_counts_registrations(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	examWithRegs := seedExam(t, env, "Paket Ramai", "score_pembahasan", nil)
	examEmpty := seedExam(t, env, "Paket Sepi", "score_pembahasan", nil)

	for i := 0; i < 3; i++ {
		studentID := seedUser(t, env, "student", "active", false)
		seedExamRegistration(t, env, studentID, examWithRegs)
	}

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	counts := map[string]float64{}
	for _, raw := range out["data"].([]any) {
		row := raw.(map[string]any)
		counts[row["id"].(string)] = row["registration_count"].(float64)
	}
	assert.Equal(t, float64(3), counts[examWithRegs])
	assert.Equal(t, float64(0), counts[examEmpty])
}

// TestExam_AdminListExams_registration_count_is_school_scoped is the FR-25 gate:
// an admin_school must never learn another school's registration volume.
func TestExam_AdminListExams_registration_count_is_school_scoped(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	adminToken := authToken(t, env, adminID, "admin_exam")

	schoolA := seedE4School(t, env, "School A", "sch-a-104", []string{"SMA"})
	schoolB := seedE4School(t, env, "School B", "sch-b-104", []string{"SMA"})

	examID := seedExam(t, env, "Paket Lintas Sekolah", "score_pembahasan", nil)

	for i := 0; i < 2; i++ {
		studentID := seedE4Student(t, env, "Student A", &schoolA, nil, "SMA")
		seedExamRegistration(t, env, studentID, examID)
	}
	studentB := seedE4Student(t, env, "Student B", &schoolB, nil, "SMA")
	seedExamRegistration(t, env, studentB, examID)

	schoolAdminID := seedUser(t, env, "admin_school", "active", false)
	schoolAdminToken := authTokenWithSchool(t, env, schoolAdminID, "admin_school", schoolA)

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, schoolAdminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
	row := findExamRow(t, out, examID)
	require.NotNil(t, row, "admin_school should still see the exam itself")
	assert.Equal(t, float64(2), row["registration_count"], "admin_school must see only its own school's registrations")

	respGlobal, outGlobal := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, adminToken)
	require.Equal(t, http.StatusOK, respGlobal.StatusCode, "body=%v", outGlobal)
	rowGlobal := findExamRow(t, outGlobal, examID)
	require.NotNil(t, rowGlobal)
	assert.Equal(t, float64(3), rowGlobal["registration_count"], "super_admin/admin_exam sees the total")
}

// TestExam_AdminListExams_adminExam_is_not_school_scoped is the regression guard
// for hazard 1: admin_exam has a nil claims.SchoolID and must get 200, not 403.
func TestExam_AdminListExams_adminExam_is_not_school_scoped(t *testing.T) {
	env := newTestEnv(t)
	adminID := seedUser(t, env, "admin_exam", "active", false)
	token := authToken(t, env, adminID, "admin_exam")

	resp, out := doJSONBody(t, env, http.MethodGet, "/api/v1/admin/exams", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "admin_exam has a nil SchoolID and must not be 403'd, body=%v", out)
	assert.NotNil(t, out["data"])
}

func findExamRow(t *testing.T, out map[string]any, examID string) map[string]any {
	t.Helper()
	for _, raw := range out["data"].([]any) {
		row := raw.(map[string]any)
		if row["id"].(string) == examID {
			return row
		}
	}
	return nil
}
