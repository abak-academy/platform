package handler_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// seedPositionExam builds one exam with one test of 3 MC questions, a
// checked-in in_progress session for a fresh student, and returns the
// session ID plus the ordered question IDs (sort_order 1..3).
func seedPositionExam(t *testing.T, env *testEnvWithStore) (sessionID, studentID uuid.UUID, questionIDs []uuid.UUID) {
	t.Helper()

	studentID = seedUser(t, env.pool, "student", "Position Student")
	testID := seedTest(t, env.pool)
	q1 := seedMCQuestion(t, env.pool, testID, "Q1", 1, 1)
	q2 := seedMCQuestion(t, env.pool, testID, "Q2", 1, 2)
	q3 := seedMCQuestion(t, env.pool, testID, "Q3", 1, 3)
	examID := seedExam(t, env.pool, "Position Exam", false, "hidden", "")
	seedExamTest(t, env.pool, examID, testID, 1)
	regID := seedRegistration(t, env.pool, studentID, examID)
	sessionID = seedMonitorSession(t, env.pool, regID, studentID, examID, time.Now(), "in_progress", nil, nil)

	return sessionID, studentID, []uuid.UUID{q1, q2, q3}
}

// TestExamSaveAnswers_WithPosition_PersistsAtomically proves FR-35: a save
// carrying current_position writes the position in the same transaction as
// the answers, both landing durably in one PATCH.
func TestExamSaveAnswers_WithPosition_PersistsAtomically(t *testing.T) {
	env := newTestEnvWithStore(t)
	sessionID, studentID, questionIDs := seedPositionExam(t, env)
	token := mintTokenForEnv(t, env, studentID.String(), "student")

	position := 2
	body := map[string]any{
		"answers": []map[string]any{
			{"question_id": questionIDs[0].String(), "answer": "a"},
		},
		"current_position": position,
	}
	rec := patchJSONRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/answers", token, body)
	if rec.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var gotPosition *int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT current_position FROM exam_session WHERE id = $1`, sessionID,
	).Scan(&gotPosition); err != nil {
		t.Fatalf("select current_position: %v", err)
	}
	if gotPosition == nil || *gotPosition != position {
		t.Errorf("current_position: want %d, got %v", position, gotPosition)
	}

	var gotAnswer *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT answer FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
		sessionID, questionIDs[0],
	).Scan(&gotAnswer); err != nil {
		t.Fatalf("select answer: %v", err)
	}
	if gotAnswer == nil || *gotAnswer != "a" {
		t.Errorf("answer: want %q, got %v", "a", gotAnswer)
	}
}

// TestExamSaveAnswers_OutOfRangePosition_Returns400AndPersistsNeither proves
// FR-35's guard: a position outside [0, total_questions) is rejected before
// any write, so neither the answer nor the position land — an advisory field
// must never be able to corrupt a save it rides along with.
func TestExamSaveAnswers_OutOfRangePosition_Returns400AndPersistsNeither(t *testing.T) {
	env := newTestEnvWithStore(t)
	sessionID, studentID, questionIDs := seedPositionExam(t, env)
	token := mintTokenForEnv(t, env, studentID.String(), "student")

	body := map[string]any{
		"answers": []map[string]any{
			{"question_id": questionIDs[0].String(), "answer": "a"},
		},
		// total questions is 3 (valid range [0,3)); 3 is out of range.
		"current_position": 3,
	}
	rec := patchJSONRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/answers", token, body)
	if rec.Code != 400 {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_position" {
		t.Errorf("code: want invalid_position, got %v", resp["code"])
	}

	var gotPosition *int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT current_position FROM exam_session WHERE id = $1`, sessionID,
	).Scan(&gotPosition); err != nil {
		t.Fatalf("select current_position: %v", err)
	}
	if gotPosition != nil {
		t.Errorf("current_position: want nil (untouched), got %v", *gotPosition)
	}

	var count int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM exam_session_answer WHERE session_id = $1`, sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("count answers: %v", err)
	}
	if count != 0 {
		t.Errorf("answers persisted: want 0, got %d", count)
	}
}

// TestExamReconnect_ResponseBody_CarriesPositionAtDocumentedKey proves FR-36:
// after a position-bearing save, GET /exam/sessions/:id surfaces that
// position at the documented "current_position" JSON key. Asserted against
// the marshalled response body (a generic map), not a Go struct field, per
// this project's wire-shape-divergence history.
func TestExamReconnect_ResponseBody_CarriesPositionAtDocumentedKey(t *testing.T) {
	env := newTestEnvWithStore(t)
	sessionID, studentID, questionIDs := seedPositionExam(t, env)
	token := mintTokenForEnv(t, env, studentID.String(), "student")

	position := 1
	saveBody := map[string]any{
		"answers": []map[string]any{
			{"question_id": questionIDs[0].String(), "answer": "a"},
		},
		"current_position": position,
	}
	saveRec := patchJSONRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/answers", token, saveBody)
	if saveRec.Code != 200 {
		t.Fatalf("save want 200, got %d body=%s", saveRec.Code, saveRec.Body.String())
	}

	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String(), token)
	if rec.Code != 200 {
		t.Fatalf("reconnect want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	got, ok := resp["current_position"]
	if !ok {
		t.Fatalf("response body missing current_position key; body=%v", resp)
	}
	if got != float64(position) {
		t.Errorf("current_position: want %v, got %v (type %T)", float64(position), got, got)
	}
}
