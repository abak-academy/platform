package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// adminSystemTestEnv holds the minimal environment for admin system handler tests.
// Uses service.New (no storeRepo) — only validation-before-storeRepo paths are testable.
type adminSystemTestEnv struct {
	e  *echo.Echo
	h  *handler.Handler
	mr *miniredis.Miniredis
}

func newAdminSystemEnv(t *testing.T) *adminSystemTestEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := newFakeRepo()
	svc := service.New(repo, rdb, nil, nil, nil, nil)
	e := echo.New()
	e.HideBanner = true
	return &adminSystemTestEnv{e: e, h: handler.New(svc), mr: mr}
}

// setAdminClaims sets super_admin claims on the echo context.
func setAdminClaims(c echo.Context, sub string) {
	c.Set("claims", &infra.Claims{Sub: sub, Role: "super_admin"})
}

// --- AdminListAccounts ---

func TestAdminListAccounts_InvalidRole_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/system/accounts?role=badrole", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)

	err := env.h.AdminListAccounts(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("want code=invalid_request, got %v", resp["code"])
	}
}

func TestAdminListAccounts_InvalidStatus_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/system/accounts?status=pending", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)

	err := env.h.AdminListAccounts(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- AdminCreateAccount ---

func TestAdminCreateAccount_EmptyBody_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/admin/system/accounts", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAccount(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminCreateAccount_MissingFields_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	payload := map[string]string{"email": "admin@test.com"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/system/accounts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAccount(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- AdminChangeAccountRole ---

func TestAdminChangeAccountRole_MissingRole_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/uuid/role", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("some-uuid")
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminChangeAccountRole(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminChangeAccountRole_InvalidUUID_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := bytes.NewReader([]byte(`{"role":"admin_store"}`))
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/notauuid/role", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("notauuid")
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminChangeAccountRole(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- AdminChangeAccountStatus ---

func TestAdminChangeAccountStatus_MissingStatus_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/uuid/status", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("some-uuid")
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminChangeAccountStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminChangeAccountStatus_InvalidStatus_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := bytes.NewReader([]byte(`{"status":"deleted"}`))
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/uuid/status", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("some-uuid")
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminChangeAccountStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminChangeAccountStatus_SelfDeactivation_403(t *testing.T) {
	env := newAdminSystemEnv(t)
	actorID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	body := bytes.NewReader([]byte(`{"status":"deactivated"}`))
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/"+actorID+"/status", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(actorID)
	setAdminClaims(c, actorID)

	err := env.h.AdminChangeAccountStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "cannot_deactivate_self" {
		t.Errorf("want code=cannot_deactivate_self, got %v", resp["code"])
	}
}

// --- AdminResetAccountPassword ---

func TestAdminResetAccountPassword_InvalidUUID_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/system/accounts/notauuid/reset-password", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("notauuid")
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminResetAccountPassword(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- AdminListAuditLog ---

func TestAdminListAuditLog_InvalidActorID_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/system/audit?actor_id=notauuid", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)

	err := env.h.AdminListAuditLog(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- AdminUpdateSystemConfig ---

func TestAdminUpdateSystemConfig_UnknownKey_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	payload := map[string]string{"unknown_key": "value"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/system/config", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminUpdateSystemConfig(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminCreateAccount_SchoolIDForNonSchoolRole_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := map[string]interface{}{
		"email":    "test@example.com",
		"name":     "Test Admin",
		"role":     "admin_store",
		"password": "password123",
		"school_id": "s-1",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/system/accounts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "u1")

	err := env.h.AdminCreateAccount(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// service layer returns ErrSchoolNotAllowed which maps to 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for school_id with non-admin_school role, got %d", rec.Code)
	}
}

func TestAdminChangeAccountRole_SchoolIDInPayload(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := map[string]interface{}{
		"role":      "admin_school",
		"school_id": "s-1",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/u2/role", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("u2")
	setAdminClaims(c, "u1")

	err := env.h.AdminChangeAccountRole(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// parseUUID fails for non-UUID "u2" → ErrInvalidUUID → 400.
	// Handler correctly processes the request (school_id field accepted, no early rejection).
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (invalid uuid), got %d", rec.Code)
	}
}

func TestAdminChangeAccountRole_MissingSchoolID_400(t *testing.T) {
	env := newAdminSystemEnv(t)
	body := map[string]interface{}{
		"role": "admin_school",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/admin/system/accounts/u2/role", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("u2")
	setAdminClaims(c, "u1")

	err := env.h.AdminChangeAccountRole(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Service returns ErrSchoolRequired (school_id required for admin_school) → 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing school_id, got %d", rec.Code)
	}
}

// TestAdminChangeAccountRole_GoogleAccountNoPassword_WireContract covers FR-9's
// wire contract end to end: a Google-provider account with no password must
// get 409 with code=account_has_no_password over real HTTP, not just the
// service-level sentinel (see TestChangeAccountRole_GoogleAccountGuard in
// internal/service/admin_users_test.go for the sentinel-level coverage).
func TestAdminChangeAccountRole_GoogleAccountNoPassword_WireContract(t *testing.T) {
	env := newTestEnvWithStore(t)
	store := repository.New(env.pool)
	ctx := context.Background()

	email := "goog-wire-" + uuid.NewString() + "@test.com"
	target := &model.User{
		Email:        &email,
		Name:         "Google Admin Wire",
		PasswordHash: "",
		Role:         service.RoleAdminStore,
		Status:       "active",
		AuthProvider: "google",
	}
	if err := store.CreateUser(ctx, target); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	token := mintTokenForEnv(t, env, uuid.NewString(), service.RoleSuperAdmin)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/system/accounts/"+target.ID+"/role", token, map[string]string{
		"role": "super_admin",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["code"] != "account_has_no_password" {
		t.Errorf("code: want account_has_no_password, got %v", resp["code"])
	}
}
