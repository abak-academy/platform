package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

// registerAdminSystemConfigRoute wires the config PUT against the real store so
// the cache-invalidation side effect is observable in Postgres.
func registerAdminSystemConfigRoute(t *testing.T, env *testEnvWithStore, h *handler.Handler) {
	t.Helper()
	v1 := env.e.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(env.svc, env.signer))
	admin.PUT("/system/config", h.AdminUpdateSystemConfig)
}

// TestUpdateSystemConfig_CardContactChange_InvalidatesCachedCards is the second
// half of the PR #87 P1: the card's contact bar comes from system_config, and a
// cached card PDF is presigned without re-rendering, so a contact edit that did
// not drop the cache would never reach an already-downloaded card.
func TestUpdateSystemConfig_CardContactChange_InvalidatesCachedCards(t *testing.T) {
	env := newTestEnvWithStore(t)
	h := handler.New(env.svc)
	registerAdminSystemConfigRoute(t, env, h)

	admin := seedUser(t, env.pool, "super_admin", "Config Admin")
	student := seedUser(t, env.pool, "student", "Config Card Holder")
	examID := seedExam(t, env.pool, "Config Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, "cards/cfg.pdf", regID,
	); err != nil {
		t.Fatalf("seed card_key: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/system/config", token,
		map[string]string{"app_contact_phone": "0899-0000-1111"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var cardKey *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_key FROM exam_registration WHERE id = $1`, regID,
	).Scan(&cardKey); err != nil {
		t.Fatalf("query card_key: %v", err)
	}
	if cardKey != nil {
		t.Errorf("card_key: want cleared after a contact change, got %q", *cardKey)
	}
}

// A config save that touches no card-visible key must leave cached cards alone —
// the page PUTs every key on every save, so this is the common case.
func TestUpdateSystemConfig_NonCardKeyChange_KeepsCachedCards(t *testing.T) {
	env := newTestEnvWithStore(t)
	h := handler.New(env.svc)
	registerAdminSystemConfigRoute(t, env, h)

	admin := seedUser(t, env.pool, "super_admin", "Config Admin Two")
	student := seedUser(t, env.pool, "student", "Config Card Holder Two")
	examID := seedExam(t, env.pool, "Config Keep Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, "cards/keepcfg.pdf", regID,
	); err != nil {
		t.Fatalf("seed card_key: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/system/config", token,
		map[string]string{"exam_platform": "cbt.example.id"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var cardKey *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_key FROM exam_registration WHERE id = $1`, regID,
	).Scan(&cardKey); err != nil {
		t.Fatalf("query card_key: %v", err)
	}
	if cardKey == nil || *cardKey != "cards/keepcfg.pdf" {
		t.Errorf("card_key: want preserved, got %v", cardKey)
	}
}

// Re-saving the SAME card value must not invalidate either — otherwise every
// config save silently forces a Gotenberg re-render for every registration.
func TestUpdateSystemConfig_UnchangedCardValue_KeepsCachedCards(t *testing.T) {
	env := newTestEnvWithStore(t)
	h := handler.New(env.svc)
	registerAdminSystemConfigRoute(t, env, h)

	admin := seedUser(t, env.pool, "super_admin", "Config Admin Three")
	student := seedUser(t, env.pool, "student", "Config Card Holder Three")
	examID := seedExam(t, env.pool, "Config Same Card Exam", false, "hidden", "classic")
	regID := seedRegistration(t, env.pool, student, examID)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)
	if rec := putJSONRequest(t, env.e, "/api/v1/admin/system/config", token,
		map[string]string{"app_contact_phone": "0899-2222-3333"},
	); rec.Code != http.StatusOK {
		t.Fatalf("seed config: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`, "cards/same.pdf", regID,
	); err != nil {
		t.Fatalf("seed card_key: %v", err)
	}

	if rec := putJSONRequest(t, env.e, "/api/v1/admin/system/config", token,
		map[string]string{"app_contact_phone": "0899-2222-3333"},
	); rec.Code != http.StatusOK {
		t.Fatalf("re-save: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var cardKey *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT card_key FROM exam_registration WHERE id = $1`, regID,
	).Scan(&cardKey); err != nil {
		t.Fatalf("query card_key: %v", err)
	}
	if cardKey == nil || *cardKey != "cards/same.pdf" {
		t.Errorf("card_key: re-saving the same value must not invalidate, got %v", cardKey)
	}
}

func TestAdminGetSystemConfig_StaleFlatRow(t *testing.T) {
	env := newTestEnvWithStore(t)
	repo := repository.New(env.pool)
	if err := repo.UpsertSystemConfig(context.Background(), "shipping_fallback_flat_rate", "50000", false); err != nil {
		t.Fatal(err)
	}
	h := handler.New(env.svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/config", nil)
	rec := httptest.NewRecorder()
	if err := h.AdminGetSystemConfig(env.e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var config map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	if _, ok := config["shipping_fallback_flat_rate"]; ok {
		t.Fatal("removed key returned")
	}
	if _, ok := config["biteship_services"]; !ok {
		t.Fatal("active shipping config missing")
	}
}
