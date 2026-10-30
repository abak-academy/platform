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
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// affiliateJobsEnv is a private, migrated database behind the production route
// set, so the real JWTMiddleware + RBACMiddleware("affiliates:*") guards run.
type affiliateJobsEnv struct {
	pool   *pgxpool.Pool
	svc    *service.Service
	e      *echo.Echo
	rdb    *redis.Client
	signer *infra.JWTSigner
}

func newAffiliateJobsEnv(t *testing.T) *affiliateJobsEnv {
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
		JWTSecret:       "affiliate-jobs-test-secret",
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

	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, signer)

	t.Cleanup(pool.Close)
	return &affiliateJobsEnv{pool: pool, svc: svc, e: e, rdb: rdb, signer: signer}
}

// tokenFor mints a valid access token for the role and seeds its session so
// JWTMiddleware's SessionActive check passes.
func (env *affiliateJobsEnv) tokenFor(t *testing.T, role string) string {
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

func TestAffiliateJobsRunAndDetailHandlerRBAC(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	id, code := seedAffiliateAdminRow(t, env.pool, "active")

	t.Run("a missing token is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/affiliate-jobs/run", strings.NewReader(`{"scope":"bindings"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("run: want 401, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = getWithToken(t, env.e, "/api/v1/admin/affiliates/"+id, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("detail: want 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin_store is forbidden on run and detail", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleAdminStore)
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliate-jobs/run", token, `{"scope":"bindings"}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("run: want 403, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = getWithToken(t, env.e, "/api/v1/admin/affiliates/"+id, token)
		if rec.Code != http.StatusForbidden {
			t.Errorf("detail: want 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("super_admin runs the bindings job", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliate-jobs/run", token, `{"scope":"bindings"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var res service.AffiliateJobResult
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if res.Scope != "bindings" {
			t.Errorf("scope = %q, want bindings", res.Scope)
		}
	})

	t.Run("super_admin reads the detail", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := getWithToken(t, env.e, "/api/v1/admin/affiliates/"+id, token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Affiliate struct {
				ID   string `json:"id"`
				Code string `json:"code"`
			} `json:"affiliate"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Affiliate.ID != id || body.Affiliate.Code != code || body.Name == "" {
			t.Errorf("detail body incomplete: %s", rec.Body.String())
		}
	})

	t.Run("unknown scope is a bad request", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliate-jobs/run", token, `{"scope":"bogus"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid quarter is a bad request once the feature is on", func(t *testing.T) {
		if _, err := env.pool.Exec(context.Background(),
			`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', 'true', false)
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`); err != nil {
			t.Fatalf("enable flag: %v", err)
		}
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliate-jobs/run", token, `{"scope":"levels","quarter":"2026-Q9"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
