package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// PrintGetCertificateData tests (FR-18, FR-19, FR-21, NFR-S2, Task 7)
// ---------------------------------------------------------------------------

func TestPrintGetCertificateData_NoToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+uuid.NewString(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPrintGetCertificateData_GarbageToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+uuid.NewString()+"?token=not-a-real-token", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPrintGetCertificateData_ExpiredToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)
	sessionID := uuid.NewString()

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCertificate, sessionID)
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}
	env.mr.FastForward(61 * time.Second)

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+sessionID+"?token="+token, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestPrintGetCertificateData_SecondRedemption_Returns401 proves FR-14: a
// print token is consumed by RedeemPrintToken as soon as it validates, before
// GetCertificatePrintData even runs — so the second call must 401 regardless
// of whether the session backing the first call actually resolved.
func TestPrintGetCertificateData_SecondRedemption_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)
	sessionID := uuid.NewString()

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCertificate, sessionID)
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	first := getRequest(t, env.e, "/api/v1/print/certificates/"+sessionID+"?token="+token, "")
	if first.Code == http.StatusUnauthorized {
		t.Fatalf("first redemption should not be 401 (token was valid), got %d body=%s", first.Code, first.Body.String())
	}

	second := getRequest(t, env.e, "/api/v1/print/certificates/"+sessionID+"?token="+token, "")
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 on second redemption, got %d body=%s", second.Code, second.Body.String())
	}
}

// TestPrintGetCertificateData_NormalUserBearerToken_Returns401 proves FR-21:
// this route sits outside every JWT-authenticated group, so a normal user
// access token is never accepted in place of a print token.
func TestPrintGetCertificateData_NormalUserBearerToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Print Auth Student")
	bearer := mintTokenForEnv(t, env, student.String(), service.RoleStudent)

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+uuid.NewString(), bearer)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestPrintGetCertificateData_ResultGateDenied_Returns403 proves FR-19: the
// gate is re-applied independently of the mint-time check. "modern" is the
// only built-in template that stamps score/rank tokens (certificate_layout.go),
// so a "hidden" result_config denies it.
func TestPrintGetCertificateData_ResultGateDenied_Returns403(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Gate Denied Student")
	examID := seedExam(t, env.pool, "Gate Denied Exam", false, "hidden", "modern")
	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 80, &submittedAt)

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCertificate, sessionID.String())
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+sessionID.String()+"?token="+token, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, has := resp["values"]; has {
		t.Errorf("expected no field values on a gate-denied response, got %v", resp["values"])
	}
	if _, has := resp["certificate_number"]; has {
		t.Errorf("expected no certificate_number on a gate-denied response, got %v", resp["certificate_number"])
	}
}

// TestPrintGetCertificateData_ValidToken_ReturnsFieldValuesAndCertificateNumber
// proves the "done when" bar for Task 7: the response body — decoded as a raw
// map, not a Go struct — carries the certificate number and the full
// field-value map at the documented JSON key positions ("certificate_number"
// top-level and "values.*"), computed from the real session/exam records.
func TestPrintGetCertificateData_ValidToken_ReturnsFieldValuesAndCertificateNumber(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Print Success Student")
	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "1+1", 1, 1)
	examID := seedExam(t, env.pool, "Print Success Exam", false, "score_only", "modern")
	seedExamTest(t, env.pool, examID, testID, 1)
	regID := seedRegistration(t, env.pool, student, examID)
	// AllocateCertificateNumber requires a non-NULL participant_number.
	if _, err := env.pool.Exec(context.Background(), `UPDATE exam_registration SET participant_number = 1 WHERE id = $1`, regID); err != nil {
		t.Fatalf("set participant_number: %v", err)
	}
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 1, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCertificate, sessionID.String())
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/print/certificates/"+sessionID.String()+"?token="+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	certNumber, _ := resp["certificate_number"].(string)
	if certNumber == "" {
		t.Fatalf(`resp["certificate_number"] = %v, want a non-empty string`, resp["certificate_number"])
	}

	values, ok := resp["values"].(map[string]any)
	if !ok {
		t.Fatalf(`resp["values"] is not an object: %T (%v)`, resp["values"], resp["values"])
	}
	if values["student_name"] != "Print Success Student" {
		t.Errorf(`values["student_name"] = %v, want "Print Success Student"`, values["student_name"])
	}
	if values["exam_title"] != "Print Success Exam" {
		t.Errorf(`values["exam_title"] = %v, want "Print Success Exam"`, values["exam_title"])
	}
	if values["certificate_number"] != certNumber {
		t.Errorf(`values["certificate_number"] = %v, want %q (must match the top-level certificate_number)`, values["certificate_number"], certNumber)
	}
	if values["score"] != "1" {
		t.Errorf(`values["score"] = %v, want "1"`, values["score"])
	}

	layout, ok := resp["layout"].(map[string]any)
	if !ok {
		t.Fatalf(`resp["layout"] is not an object: %T`, resp["layout"])
	}
	if _, has := layout["fields"]; !has {
		t.Errorf("layout missing fields")
	}

	if bg, _ := resp["background_url"].(string); bg == "" {
		t.Errorf(`resp["background_url"] is empty, want a browser-loadable URL`)
	}
}

// ---------------------------------------------------------------------------
// PrintGetCardData tests (FR-20, FR-21, NFR-S2, Task 7)
// ---------------------------------------------------------------------------

func TestPrintGetCardData_NoToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)

	rec := getRequest(t, env.e, "/api/v1/print/cards/"+uuid.NewString(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPrintGetCardData_WrongKindToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)
	regID := uuid.NewString()

	// Minted for the certificate kind, presented to the card route (FR-15).
	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCertificate, regID)
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/print/cards/"+regID+"?token="+token, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPrintGetCardData_ValidToken_ReturnsCardFields(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Print Card Student")
	examID := seedExam(t, env.pool, "Print Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCard, regID.String())
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/print/cards/"+regID.String()+"?token="+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["student_name"] != "Print Card Student" {
		t.Errorf(`resp["student_name"] = %v, want "Print Card Student"`, resp["student_name"])
	}
	if resp["exam_title"] != "Print Card Exam" {
		t.Errorf(`resp["exam_title"] = %v, want "Print Card Exam"`, resp["exam_title"])
	}
	checkInCode, _ := resp["check_in_code"].(string)
	if checkInCode == "" {
		t.Errorf(`resp["check_in_code"] is empty, want the registration token`)
	}
}

// TestPrintGetCardData_ValidToken_ReturnsFieldParityWithBuildCardHTML proves
// Task 25: the four values buildCardHTML (backend/internal/service/card_html.go)
// showed before Task 12 switched GetExamCard to the print route — tenant
// name, tenant logo, student photo, and the check-in footer note — are back
// in the print-data response, checked against the real response body.
func TestPrintGetCardData_ValidToken_ReturnsFieldParityWithBuildCardHTML(t *testing.T) {
	env := newTestEnvWithStoreAndStorage(t)
	student := seedUser(t, env.pool, "student", "Print Card Parity Student")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE users SET photo_url = $1 WHERE id = $2`, "avatars/print-card-parity.png", student); err != nil {
		t.Fatalf("set photo_url: %v", err)
	}
	examID := seedExam(t, env.pool, "Print Card Parity Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET requires_checkin = true, check_in_window_minutes = 15 WHERE id = $1`, examID); err != nil {
		t.Fatalf("set requires_checkin: %v", err)
	}
	regID := seedRegistration(t, env.pool, student, examID)

	if _, err := env.svc.UpdateSystemConfig(context.Background(), student.String(), map[string]string{
		"app_name":     "Bimbel Prima",
		"app_logo_url": "https://cdn.example.com/logo.png",
	}); err != nil {
		t.Fatalf("UpdateSystemConfig: %v", err)
	}

	token, err := env.svc.MintPrintToken(context.Background(), service.PrintTokenKindCard, regID.String())
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/print/cards/"+regID.String()+"?token="+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["tenant_name"] != "Bimbel Prima" {
		t.Errorf(`resp["tenant_name"] = %v, want "Bimbel Prima"`, resp["tenant_name"])
	}
	if resp["tenant_logo_url"] != "https://cdn.example.com/logo.png" {
		t.Errorf(`resp["tenant_logo_url"] = %v, want the configured external logo URL unchanged`, resp["tenant_logo_url"])
	}
	photoURL, _ := resp["photo_url"].(string)
	if photoURL == "" || !strings.Contains(photoURL, "print-card-parity.png") {
		t.Errorf(`resp["photo_url"] = %v, want a browser-loadable presigned URL for the stored avatar key`, resp["photo_url"])
	}
	footerNote, _ := resp["footer_note"].(string)
	if !strings.Contains(footerNote, "15") {
		t.Errorf(`resp["footer_note"] = %v, want the check-in window copy mentioning 15 menit`, footerNote)
	}
}
