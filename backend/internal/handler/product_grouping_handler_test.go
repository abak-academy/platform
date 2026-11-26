package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var (
	groupingDBOnce sync.Once
	groupingDBEnv  *adminProductDBTestEnv
)

// newGroupingDBEnv serves the full route table over a real database, so RBAC is the production middleware.
func newGroupingDBEnv(t *testing.T) *adminProductDBTestEnv {
	t.Helper()
	groupingDBOnce.Do(func() {
		pool := newTemplatedPool(t)
		store := repository.New(pool)
		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("miniredis: %v", err)
		}
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		cfg := &config.Config{JWTSecret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 168 * time.Hour, OTPTTL: 5 * time.Minute}
		signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
		svc := service.NewWithStore(store, store, rdb, signer, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
			&service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, cfg, nil)
		e := echo.New()
		e.HideBanner = true
		server.RegisterRoutesForTest(e, handler.New(svc), svc, signer)
		groupingDBEnv = &adminProductDBTestEnv{pool: pool, rdb: rdb, e: e, signer: signer, mr: mr}
	})
	if groupingDBEnv == nil {
		t.Fatal("grouping test env failed to initialize")
	}
	return groupingDBEnv
}

func groupingCall(t *testing.T, env *adminProductDBTestEnv, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != nil {
		rd = strings.NewReader(mustJSON(t, body))
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, "/api/v1/admin/product-groupings"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

func handlerPrefix() string {
	b := []byte(strings.ReplaceAll(uuid.NewString(), "-", "")[:6])
	for i, c := range b {
		if c >= 'a' {
			b[i] = 'A' + (c - 'a') + 10
		} else {
			b[i] = 'A' + (c - '0')
		}
	}
	return string(b)
}

func createEvent(t *testing.T, env *adminProductDBTestEnv, token string) map[string]any {
	t.Helper()
	rec := groupingCall(t, env, http.MethodPost, "", token, map[string]any{"name": "Event " + uuid.NewString()[:8], "participant_prefix": handlerPrefix()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create event: %d %s", rec.Code, rec.Body.String())
	}
	return decodeMap(t, rec)
}

func TestAdminProductGroupings_RBACPerRole(t *testing.T) {
	env := newGroupingDBEnv(t)
	cases := []struct {
		role string
		want int
	}{
		{service.RoleSuperAdmin, http.StatusCreated},
		{service.RoleAdminStore, http.StatusCreated},
		{service.RoleAdminExam, http.StatusCreated},
		{service.RoleAdminSchool, http.StatusForbidden},
		{service.RoleAffiliator, http.StatusForbidden},
		{service.RoleStudent, http.StatusForbidden},
	}
	for _, c := range cases {
		token := mintProductToken(t, env, uuid.NewString(), c.role)
		post := groupingCall(t, env, http.MethodPost, "", token, map[string]string{"name": "Rbac " + uuid.NewString()[:8]})
		if post.Code != c.want {
			t.Errorf("%s POST = %d, want %d (%s)", c.role, post.Code, c.want, post.Body.String())
		}
		list := groupingCall(t, env, http.MethodGet, "", token, nil)
		wantList := http.StatusOK
		if c.want == http.StatusForbidden {
			wantList = http.StatusForbidden
		}
		if list.Code != wantList {
			t.Errorf("%s GET = %d, want %d", c.role, list.Code, wantList)
		}
	}
	if rec := groupingCall(t, env, http.MethodPost, "", "", map[string]string{"name": "x"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token POST = %d, want 401", rec.Code)
	}
}

func TestAdminProductGroupings_CreateAndDetailShape(t *testing.T) {
	env := newGroupingDBEnv(t)
	token := mintProductToken(t, env, uuid.NewString(), service.RoleAdminExam)
	suffix := uuid.NewString()[:8]

	rec := groupingCall(t, env, http.MethodPost, "", token, map[string]any{"name": "Olimpiade Kita " + suffix, "short_name": "Kita", "scope": "national", "tone": "teal"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	created := decodeMap(t, rec)
	if created["slug"] != "olimpiade-kita-"+suffix || created["is_visible"] != false || created["status"] != "draft" || created["tone"] != "teal" || created["scope"] != "national" {
		t.Fatalf("created = %v", created)
	}
	if created["participant_prefix"] != "KITA" && created["participant_prefix"] != nil {
		t.Fatalf("participant_prefix = %v, want KITA (or NULL when another event already took it)", created["participant_prefix"])
	}

	detail := decodeMap(t, groupingCall(t, env, http.MethodGet, "/"+created["id"].(string), token, nil))
	for _, k := range []string{"schedule", "en_complete", "products", "content", "participant_prefix", "slug"} {
		if _, ok := detail[k]; !ok {
			t.Errorf("detail lacks %q: %v", k, detail)
		}
	}
	if detail["en_complete"] != false {
		t.Errorf("en_complete = %v for an empty event, want false", detail["en_complete"])
	}
	if s, _ := detail["schedule"].([]any); s == nil || len(s) != 0 {
		t.Errorf("schedule = %v, want []", detail["schedule"])
	}
}

func TestAdminProductGroupings_PatchValidationAndConflict(t *testing.T) {
	env := newGroupingDBEnv(t)
	token := mintProductToken(t, env, uuid.NewString(), service.RoleSuperAdmin)
	g := createEvent(t, env, token)
	other := createEvent(t, env, token)
	path := "/" + g["id"].(string)

	for _, bad := range []string{"abcd", "A", "ABCDEFG", "AB12"} {
		rec := groupingCall(t, env, http.MethodPatch, path, token, map[string]any{"participant_prefix": bad})
		if rec.Code != http.StatusUnprocessableEntity || decodeMap(t, rec)["code"] != "validation_failed" {
			t.Errorf("prefix %q = %d %s, want 422 validation_failed", bad, rec.Code, rec.Body.String())
		}
	}
	rec := groupingCall(t, env, http.MethodPatch, path, token, map[string]any{"participant_prefix": other["participant_prefix"]})
	if rec.Code != http.StatusConflict || decodeMap(t, rec)["code"] != "grouping_prefix_taken" {
		t.Errorf("duplicate prefix = %d %s, want 409 grouping_prefix_taken", rec.Code, rec.Body.String())
	}
	rec = groupingCall(t, env, http.MethodPatch, path, token, map[string]any{"content": map[string]any{"fr": map[string]any{}}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown locale = %d %s, want 422", rec.Code, rec.Body.String())
	}
	rec = groupingCall(t, env, http.MethodPatch, path, token, map[string]any{"schedule": []map[string]any{{"label": "X", "starts_on": "2026-12-05", "ends_on": "2026-12-01"}}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("ends before starts = %d, want 422", rec.Code)
	}
}

func TestAdminProductGroupings_PatchVisibilityReturnsMissingList(t *testing.T) {
	env := newGroupingDBEnv(t)
	token := mintProductToken(t, env, uuid.NewString(), service.RoleAdminExam)
	g := createEvent(t, env, token)
	path := "/" + g["id"].(string)

	rec := groupingCall(t, env, http.MethodPatch, path, token, map[string]any{
		"status": "published", "is_visible": true,
		"content":  map[string]any{"id": map[string]any{"tagline": "T", "story": []string{"S"}}, "en": map[string]any{"tagline": "T", "story": []string{}}},
		"schedule": []map[string]any{{"label": "Babak", "starts_on": "2026-12-01"}},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d %s, want 422", rec.Code, rec.Body.String())
	}
	body := decodeMap(t, rec)
	missing, _ := body["missing"].([]any)
	if body["code"] != "validation_failed" || len(missing) != 2 || missing[0] != "en.story" || missing[1] != "schedule[0].label_en" {
		t.Fatalf("body = %v, want code validation_failed and missing [en.story schedule[0].label_en]", body)
	}

	rec = groupingCall(t, env, http.MethodPatch, path, token, map[string]any{
		"status": "published", "is_visible": true,
		"content":  map[string]any{"id": map[string]any{"tagline": "T", "story": []string{"S"}}, "en": map[string]any{"tagline": "T", "story": []string{"S"}}},
		"schedule": []map[string]any{{"label": "Babak", "label_en": "Round", "starts_on": "2026-12-01"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("complete patch = %d %s", rec.Code, rec.Body.String())
	}
	detail := decodeMap(t, rec)
	if detail["is_visible"] != true || detail["en_complete"] != true {
		t.Fatalf("detail = %v, want visible and en_complete", detail)
	}
	if s, _ := detail["schedule"].([]any); len(s) != 1 || s[0].(map[string]any)["label_en"] != "Round" {
		t.Fatalf("schedule = %v", detail["schedule"])
	}

	rec = groupingCall(t, env, http.MethodPatch, path, token, map[string]any{"status": "draft"})
	if rec.Code != http.StatusOK || decodeMap(t, rec)["is_visible"] != false {
		t.Fatalf("draft patch = %d %s, want is_visible false", rec.Code, rec.Body.String())
	}
}

func TestAdminProductGroupings_DeleteWithMembersConflicts(t *testing.T) {
	env := newGroupingDBEnv(t)
	token := mintProductToken(t, env, uuid.NewString(), service.RoleAdminStore)
	g := createEvent(t, env, token)
	path := "/" + g["id"].(string)

	var productID string
	if err := env.pool.QueryRow(t.Context(), `INSERT INTO product (type, name, price, status) VALUES ('book', $1, 1, 'published') RETURNING id`, "P "+uuid.NewString()).Scan(&productID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if rec := groupingCall(t, env, http.MethodPut, path+"/products", token, map[string]any{"product_ids": []string{productID}}); rec.Code != http.StatusOK {
		t.Fatalf("PUT products = %d %s", rec.Code, rec.Body.String())
	} else if p, _ := decodeMap(t, rec)["products"].([]any); len(p) != 1 {
		t.Fatalf("PUT response products = %v", p)
	}

	rec := groupingCall(t, env, http.MethodDelete, path, token, nil)
	if rec.Code != http.StatusConflict || decodeMap(t, rec)["code"] != "grouping_has_products" {
		t.Fatalf("delete with members = %d %s, want 409 grouping_has_products", rec.Code, rec.Body.String())
	}
	groupingCall(t, env, http.MethodPut, path+"/products", token, map[string]any{"product_ids": []string{}})
	if rec := groupingCall(t, env, http.MethodDelete, path, token, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete empty event = %d, want 204", rec.Code)
	}
}
