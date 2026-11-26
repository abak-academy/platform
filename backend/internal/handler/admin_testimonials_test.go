package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

func testimonialCall(t *testing.T, env *adminProductDBTestEnv, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	rd := strings.NewReader("")
	if body != nil {
		rd = strings.NewReader(mustJSON(t, body))
	}
	req := httptest.NewRequest(method, "/api/v1/admin/testimonials"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func testimonialSuperToken(t *testing.T, env *adminProductDBTestEnv) (token, userID string) {
	t.Helper()
	email := "tm-" + uuid.NewString()[:8] + "@test.com"
	u := &model.User{Email: &email, Name: "Super", PasswordHash: "x", Role: service.RoleSuperAdmin, Status: "active", AuthProvider: "google"}
	if err := repository.New(env.pool).CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return mintProductToken(t, env, u.ID, service.RoleSuperAdmin), u.ID
}

func TestAdminTestimonials_OnlySuperAdminPassesRBAC(t *testing.T) {
	env := newGroupingDBEnv(t)
	absent := uuid.NewString()
	calls := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "", nil},
		{http.MethodPost, "", map[string]string{"name": "x", "role_label": "y"}},
		{http.MethodPatch, "/" + absent, map[string]string{"name": "x"}},
		{http.MethodDelete, "/" + absent, nil},
	}
	for _, role := range []string{service.RoleAdminExam, service.RoleAdminStore, service.RoleAdminSchool, service.RoleAffiliator, service.RoleStudent} {
		token := mintProductToken(t, env, uuid.NewString(), role)
		for _, c := range calls {
			if rec := testimonialCall(t, env, c.method, c.path, token, c.body); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s %s = %d, want 403", role, c.method, c.path, rec.Code)
			}
		}
	}
	if rec := testimonialCall(t, env, http.MethodGet, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
	token, _ := testimonialSuperToken(t, env)
	if rec := testimonialCall(t, env, http.MethodGet, "", token, nil); rec.Code != http.StatusOK {
		t.Errorf("super_admin GET = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
}

func TestAdminTestimonials_Lifecycle(t *testing.T) {
	env := newGroupingDBEnv(t)
	token, userID := testimonialSuperToken(t, env)

	rec := testimonialCall(t, env, http.MethodPost, "", token, map[string]any{
		"name": "Ayu", "role_label": "SMA 1", "consent_by": uuid.NewString(), "is_visible": false,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	created := decodeMap(t, rec)
	id := created["id"].(string)
	if created["consent_by"] != nil || created["is_visible"] != false || created["photo_key"] != nil {
		t.Errorf("unexpected create body: %v", created)
	}

	rec = testimonialCall(t, env, http.MethodPatch, "/"+id, token, map[string]any{"is_visible": true})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("visible without parts = %d %s", rec.Code, rec.Body.String())
	}
	body := decodeMap(t, rec)
	if body["code"] != "validation_failed" {
		t.Errorf("code = %v", body["code"])
	}
	want := []any{"has_consent", "consent_note", "content.id.quote", "content.en.quote", "photo_key"}
	got, _ := body["missing"].([]any)
	if len(got) != len(want) {
		t.Fatalf("missing = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("missing[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	rec = testimonialCall(t, env, http.MethodPatch, "/"+id, token, map[string]any{
		"has_consent": true, "consent_note": "WA", "photo_key": "question/a.jpg", "is_visible": true,
		"content": map[string]any{"id": map[string]any{"quote": "Mantap"}, "en": map[string]any{"quote": "Great", "achievements": []string{"Gold"}}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", rec.Code, rec.Body.String())
	}
	pub := decodeMap(t, rec)
	if pub["is_visible"] != true || pub["consent_by"] != userID || pub["consent_by_name"] != "Super" {
		t.Errorf("published body: %v", pub)
	}

	rec = testimonialCall(t, env, http.MethodPatch, "/"+id, token, map[string]any{"photo_key": nil})
	if got := decodeMap(t, rec); rec.Code != http.StatusOK || got["is_visible"] != false || got["photo_key"] != nil {
		t.Errorf("photo null: %d %v", rec.Code, got)
	}

	for _, bad := range []map[string]any{
		{"photo_key": "avatars/x.jpg"},
		{"content": map[string]any{"fr": map[string]any{"quote": "x"}}},
	} {
		if rec := testimonialCall(t, env, http.MethodPatch, "/"+id, token, bad); rec.Code != http.StatusUnprocessableEntity || decodeMap(t, rec)["code"] != "validation_failed" {
			t.Errorf("%v = %d %s, want 422 validation_failed", bad, rec.Code, rec.Body.String())
		}
	}

	if rec := testimonialCall(t, env, http.MethodDelete, "/"+id, token, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if rec := testimonialCall(t, env, http.MethodDelete, "/"+id, token, nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete again = %d, want 404", rec.Code)
	}
	if rec := testimonialCall(t, env, http.MethodPatch, "/"+uuid.NewString(), token, map[string]any{"name": "x"}); rec.Code != http.StatusNotFound {
		t.Errorf("patch unknown = %d, want 404", rec.Code)
	}
}
