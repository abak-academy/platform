package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

// ---------------------------------------------------------------------------
// AdminUpdateExam end-screen round-trip tests (Task 20, FR-38/FR-39)
// ---------------------------------------------------------------------------

func TestAdminUpdateExam_EndScreen_RoundTripsAtDocumentedKeys(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin End Screen")

	examID := seedExam(t, env.pool, "End Screen Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{
			"end_screen_image_url":  "https://cdn.example.com/end-screen.png",
			"end_screen_promo_text": "Thanks for taking the exam! Enroll in our next class.",
		},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := body["end_screen_image_url"]; got != "https://cdn.example.com/end-screen.png" {
		t.Errorf("response end_screen_image_url: want the uploaded URL, got %v", got)
	}
	if got := body["end_screen_promo_text"]; got != "Thanks for taking the exam! Enroll in our next class." {
		t.Errorf("response end_screen_promo_text: want the promo text, got %v", got)
	}

	// Verify actual persistence, not just an echoed request body.
	var imageURL, promoText *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT end_screen_image_url, end_screen_promo_text FROM exam WHERE id = $1`, examID,
	).Scan(&imageURL, &promoText); err != nil {
		t.Fatalf("query end screen columns: %v", err)
	}
	if imageURL == nil || *imageURL != "https://cdn.example.com/end-screen.png" {
		t.Errorf("persisted end_screen_image_url: want the uploaded URL, got %v", imageURL)
	}
	if promoText == nil || *promoText != "Thanks for taking the exam! Enroll in our next class." {
		t.Errorf("persisted end_screen_promo_text: want the promo text, got %v", promoText)
	}
}

func TestAdminUpdateExam_EndScreen_ExplicitNullClears(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Clear End Screen")

	examID := seedExam(t, env.pool, "Clear End Screen Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET end_screen_image_url = $1, end_screen_promo_text = $2 WHERE id = $3`,
		"https://cdn.example.com/old.png", "Old promo", examID,
	); err != nil {
		t.Fatalf("seed end screen columns: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"end_screen_image_url": nil, "end_screen_promo_text": nil},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var imageURL, promoText *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT end_screen_image_url, end_screen_promo_text FROM exam WHERE id = $1`, examID,
	).Scan(&imageURL, &promoText); err != nil {
		t.Fatalf("query end screen columns: %v", err)
	}
	if imageURL != nil {
		t.Errorf("end_screen_image_url: want cleared (nil), got %v", *imageURL)
	}
	if promoText != nil {
		t.Errorf("end_screen_promo_text: want cleared (nil), got %v", *promoText)
	}
}

func TestAdminUpdateExam_EndScreen_OmittedFieldsPreserveExisting(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preserve End Screen")

	examID := seedExam(t, env.pool, "Preserve End Screen Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET end_screen_image_url = $1, end_screen_promo_text = $2 WHERE id = $3`,
		"https://cdn.example.com/keep.png", "Keep this promo", examID,
	); err != nil {
		t.Fatalf("seed end screen columns: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	// An unrelated-field PATCH that omits both end-screen keys must preserve them.
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var imageURL, promoText *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT end_screen_image_url, end_screen_promo_text FROM exam WHERE id = $1`, examID,
	).Scan(&imageURL, &promoText); err != nil {
		t.Fatalf("query end screen columns: %v", err)
	}
	if imageURL == nil || *imageURL != "https://cdn.example.com/keep.png" {
		t.Errorf("end_screen_image_url: want preserved, got %v", imageURL)
	}
	if promoText == nil || *promoText != "Keep this promo" {
		t.Errorf("end_screen_promo_text: want preserved, got %v", promoText)
	}
}

// ---------------------------------------------------------------------------
// StudentGetSessionResult end-screen propagation (FR-38/FR-39)
// ---------------------------------------------------------------------------

// TestStudentGetSessionResult_EndScreenConfigured_CarriesImageAndPromo proves
// FR-38 end-to-end through the real result endpoint: a submitted session of
// an exam with both end-screen fields set carries them in the response, even
// while the result itself stays gated ("hidden") — the end screen is shown
// on submit regardless of result-visibility config.
func TestStudentGetSessionResult_EndScreenConfigured_CarriesImageAndPromo(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student End Screen")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "End Screen Result Exam", false, "hidden", "classic")
	setExamCertificateEnabled(t, env.pool, examID, false)
	seedExamTest(t, env.pool, examID, testID, 1)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET end_screen_image_url = $1, end_screen_promo_text = $2 WHERE id = $3`,
		"https://cdn.example.com/end-screen.png", "Thanks for taking the exam!", examID,
	); err != nil {
		t.Fatalf("set end screen columns: %v", err)
	}

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 1, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/result", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["state"] != "hidden" {
		t.Fatalf("state: want hidden (result_config=hidden gates it), got %v", resp["state"])
	}
	if got := resp["end_screen_image_url"]; got != "https://cdn.example.com/end-screen.png" {
		t.Errorf("end_screen_image_url: want the configured URL, got %v", got)
	}
	if got := resp["end_screen_promo_text"]; got != "Thanks for taking the exam!" {
		t.Errorf("end_screen_promo_text: want the configured text, got %v", got)
	}
}

// TestStudentGetSessionResult_EndScreenNotConfigured_OmitsFields proves FR-39:
// an exam with no end-screen content configured carries neither key (they are
// omitempty, unlike certificate_url which is always present).
func TestStudentGetSessionResult_EndScreenNotConfigured_OmitsFields(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student No End Screen")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "No End Screen Result Exam", false, "hidden", "classic")
	setExamCertificateEnabled(t, env.pool, examID, false)
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 1, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/result", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := resp["end_screen_image_url"]; ok {
		t.Errorf("end_screen_image_url: want key absent when unconfigured, got %v", resp["end_screen_image_url"])
	}
	if _, ok := resp["end_screen_promo_text"]; ok {
		t.Errorf("end_screen_promo_text: want key absent when unconfigured, got %v", resp["end_screen_promo_text"])
	}
}
