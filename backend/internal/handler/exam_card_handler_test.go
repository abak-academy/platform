package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/service"
)

// registerStudentCardRoute adds the student card-download endpoint under
// /api/v1/exam, protected by JWTMiddleware only (no RBAC) — mirrors
// registerStudentLeaderboardRoute.
func registerStudentCardRoute(t *testing.T, env *testEnv, h *handler.Handler) {
	t.Helper()
	v1 := env.e.Group("/api/v1")
	exam := v1.Group("/exam")
	exam.Use(handler.JWTMiddleware(env.svc, env.signer))
	exam.GET("/registrations/:id/card", h.StudentGetExamCard)
}

// ---------------------------------------------------------------------------
// StudentGetExamCard tests (FR-30)
// ---------------------------------------------------------------------------

func TestStudentGetExamCard_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerStudentCardRoute(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/exam/registrations/00000000-0000-0000-0000-000000000000/card", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStudentGetExamCard_NotOwned_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	owner := seedUser(t, env.pool, "student", "Card Owner")
	other := seedUser(t, env.pool, "student", "Other Card Student")

	examID := seedExam(t, env.pool, "Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, owner, examID)

	token := mintTokenForEnv(t, env, other.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String()+"/card", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "registration_not_found" {
		t.Errorf("code: want registration_not_found, got %v", resp["code"])
	}
}

// TestStudentGetExamCard_CachedCard_RedirectsToPresignedURL covers the success
// path FR-30 specifies: a registration whose card_key is already set is served
// by signing a fresh time-limited GET and redirecting to it — the API never
// streams the PDF bytes itself. This became testable once the cache-hit path
// stopped calling GetObject: presigning is a pure local computation with an
// explicit region, so it needs no reachable object store.
func TestStudentGetExamCard_CachedCard_RedirectsToPresignedURL(t *testing.T) {
	env := newTestEnvWithStoreAndStorage(t)
	owner := seedUser(t, env.pool, "student", "Cached Card Owner")
	examID := seedExam(t, env.pool, "Cached Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, owner, examID)

	const cardKey = "cards/cached-card.pdf"
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, cardKey, regID,
	); err != nil {
		t.Fatalf("set card_key: %v", err)
	}

	token := mintTokenForEnv(t, env, owner.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String()+"/card", token)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 redirect to the presigned object, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "X-Amz-Signature") {
		t.Errorf("Location must be a presigned URL, got %q", loc)
	}
	if !strings.Contains(loc, cardKey) {
		t.Errorf("Location must point at the stored card key %q, got %q", cardKey, loc)
	}
	// The download filename travels on the presigned request, since the client
	// now fetches an opaque object key rather than an API route.
	if !strings.Contains(loc, "response-content-disposition") {
		t.Errorf("presigned URL must carry the download filename, got %q", loc)
	}
	if rec.Body.Len() > 0 && strings.Contains(rec.Body.String(), "%PDF-") {
		t.Error("the API must not stream the PDF bytes; it must redirect")
	}
}
