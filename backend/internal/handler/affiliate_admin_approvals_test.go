package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// affiliateAdminEnv is a private, migrated database per test behind the real
// JWT + RBAC middleware: the route's super_admin gate is the point, so the
// tests drive a real router rather than calling handler methods directly.
type affiliateAdminEnv struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	e      *echo.Echo
	signer *infra.JWTSigner
}

// newAffiliateAdminRouter wires the admin affiliate routes the same way
// routes.go's /admin group does: JWT on the group, RBACMiddleware("affiliates:*")
// on the affiliate subgroup.
func newAffiliateAdminRouter(t *testing.T, h *handler.Handler, svc *service.Service, signer *infra.JWTSigner) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(svc, signer))
	affiliates := admin.Group("/affiliates")
	affiliates.Use(handler.RBACMiddleware("affiliates:*"))
	affiliates.GET("", h.AdminListAffiliates)
	affiliates.POST("/:id/approval", h.AdminDecideAffiliateApproval)
	affiliates.PATCH("/:id/status", h.AdminChangeAffiliateStatus)
	affiliates.POST("/:id/freeze", h.AdminFreezeAffiliateBalance)
	affiliates.POST("/:id/unfreeze", h.AdminUnfreezeAffiliateBalance)
	affiliates.POST("/:id/level", h.AdminChangeAffiliateLevel)
	return e
}

func newAffiliateAdminEnv(t *testing.T) *affiliateAdminEnv {
	t.Helper()
	pool := newTemplatedPool(t)
	store := repository.New(pool)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		JWTSecret:       "affiliate-admin-test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	svc := service.NewWithStore(
		store, store, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, cfg, nil,
	)
	h := handler.New(svc)
	e := newAffiliateAdminRouter(t, h, svc, signer)

	t.Cleanup(pool.Close)
	return &affiliateAdminEnv{pool: pool, rdb: rdb, e: e, signer: signer}
}

// tokenFor mints a valid access token for the role and seeds its session so
// JWTMiddleware's SessionActive check passes.
func (env *affiliateAdminEnv) tokenFor(t *testing.T, role string) string {
	t.Helper()
	tok, jti, err := env.signer.SignAccess(uuid.NewString(), role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, "1", time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return tok
}

// seedAffiliateAdminRow inserts a user + affiliator row with the given status
// and returns the affiliator id and code.
func seedAffiliateAdminRow(t *testing.T, pool *pgxpool.Pool, status string) (id, code string) {
	t.Helper()
	suffix := uniqueHandlerSuffix()
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, email, username, role, status) VALUES ($1, $2, $3, 'affiliator', 'active') RETURNING id`,
		"Affiliator "+suffix, "affadmin-"+suffix+"@test.local", "affadmin_"+suffix,
	).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	code = "AF" + suffix[:6]
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, $3, 'v1') RETURNING id`,
		userID, code, status,
	).Scan(&id); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return id, code
}

func TestAffiliateAdminHandlerRBACGate(t *testing.T) {
	env := newAffiliateAdminEnv(t)
	seedAffiliateAdminRow(t, env.pool, "pending_approval")

	t.Run("a missing token is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/affiliates", nil)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin_store is forbidden", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleAdminStore)
		rec := getWithToken(t, env.e, "/api/v1/admin/affiliates", token)
		if rec.Code != http.StatusForbidden {
			t.Errorf("want 403, got %d: %s", rec.Code, rec.Body.String())
		}
		token = env.tokenFor(t, service.RoleAdminStore)
		rec = postJSONWithToken(t, env.e, "/api/v1/admin/affiliates/"+uuid.NewString()+"/approval", token, `{"decision":"approve"}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("approval: want 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("super_admin may list", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := getWithToken(t, env.e, "/api/v1/admin/affiliates", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Total < 1 || len(body.Data) < 1 {
			t.Fatalf("list is empty: %s", rec.Body.String())
		}
		if body.Data[0].Name == "" {
			t.Errorf("admin list rows must carry the owner's name")
		}
	})
}

func TestAffiliateAdminHandlerValidation400s(t *testing.T) {
	env := newAffiliateAdminEnv(t)
	id, _ := seedAffiliateAdminRow(t, env.pool, "pending_approval")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"approval decision is unknown", http.MethodPost, "/approval", `{"decision":"maybe"}`},
		{"approval body is missing", http.MethodPost, "/approval", ``},
		{"status value is unknown", http.MethodPatch, "/status", `{"status":"bogus","reason":"alasan"}`},
		{"suspend needs days", http.MethodPatch, "/status", `{"status":"suspended","reason":"alasan"}`},
		{"freeze needs days", http.MethodPost, "/freeze", `{"reason":"alasan"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/admin/affiliates/" + id + tt.path
			var rec *httptest.ResponseRecorder
			if tt.method == http.MethodPost {
				rec = postJSONWithToken(t, env.e, path, token, tt.body)
			} else {
				req := httptest.NewRequest(tt.method, path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				rec = httptest.NewRecorder()
				env.e.ServeHTTP(rec, req)
			}
			if rec.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAffiliateAdminHandlerApprovalGeneratesCode(t *testing.T) {
	env := newAffiliateAdminEnv(t)
	id, seededCode := seedAffiliateAdminRow(t, env.pool, "pending_approval")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliates/"+id+"/approval", token, `{"decision":"approve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var aff struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &aff); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if aff.Status != "active" {
		t.Errorf("status = %q, want active", aff.Status)
	}
	if len(aff.Code) != 8 {
		t.Fatalf("generated code %q is not 8 chars", aff.Code)
	}
	if strings.ContainsAny(aff.Code, "O0I1") {
		t.Errorf("code %q carries an ambiguous glyph", aff.Code)
	}
	if aff.Code == seededCode {
		t.Errorf("approval must replace the placeholder code")
	}

	var persistedStatus, persistedCode string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT status, code FROM affiliators WHERE id = $1`, id).Scan(&persistedStatus, &persistedCode); err != nil {
		t.Fatalf("reload affiliator: %v", err)
	}
	if persistedStatus != "active" || persistedCode != aff.Code {
		t.Errorf("persisted status/code = %s/%s, want active/%s", persistedStatus, persistedCode, aff.Code)
	}

	var auditNo int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliator' AND target_id = $1 AND action = 'affiliate.approval'`,
		id).Scan(&auditNo); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditNo != 1 {
		t.Errorf("affiliate.approval audit rows = %d, want 1", auditNo)
	}
}

func TestAffiliateAdminHandlerListQFilter(t *testing.T) {
	env := newAffiliateAdminEnv(t)
	id, code := seedAffiliateAdminRow(t, env.pool, "active")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/affiliates?q="+code, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || len(body.Data) != 1 || body.Data[0].ID != id {
		t.Errorf("q filter returned total=%d rows=%d, want exactly the seeded affiliator: %s", body.Total, len(body.Data), rec.Body.String())
	}
}
