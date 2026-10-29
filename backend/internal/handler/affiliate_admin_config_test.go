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

// affiliateConfigEnv is a private, migrated database per test behind the real
// JWT + RBAC middleware: the five admin config surfaces are gated by
// RBACMiddleware("affiliates:*"), so the tests drive a real router.
type affiliateConfigEnv struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	e      *echo.Echo
	signer *infra.JWTSigner
}

// newAffiliateAdminConfigRouter wires the admin config routes the way
// routes.go's /admin group does: JWT on the group, RBACMiddleware("affiliates:*")
// in front of the affiliate config surfaces.
func newAffiliateAdminConfigRouter(t *testing.T, h *handler.Handler, svc *service.Service, signer *infra.JWTSigner) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(svc, signer))
	cfg := admin.Group("", handler.RBACMiddleware("affiliates:*"))
	cfg.GET("/affiliate-settings", h.AdminGetAffiliateSettings)
	cfg.PUT("/affiliate-settings", h.AdminUpdateAffiliateSettings)
	cfg.GET("/affiliate-product-overrides", h.AdminListAffiliateOverrides)
	cfg.GET("/affiliate-product-overrides/products", h.AdminSearchAffiliateOverrideProducts)
	cfg.PUT("/affiliate-product-overrides/:product_id", h.AdminUpsertAffiliateOverride)
	cfg.DELETE("/affiliate-product-overrides/:product_id", h.AdminDeleteAffiliateOverride)
	cfg.GET("/affiliates/:id/commissions", h.AdminListAffiliateCommissions)
	cfg.GET("/affiliates/:id/commissions/export", h.AdminAffiliateCommissionsExport)
	cfg.GET("/affiliate-reports/summary", h.AdminAffiliateReportSummary)
	cfg.GET("/affiliate-reports/liability", h.AdminAffiliateReportLiability)
	cfg.GET("/affiliate-reports/orders", h.AdminListAffiliateReportOrders)
	cfg.GET("/affiliate-reports/orders/export", h.AdminAffiliateReportOrdersExport)
	return e
}

func newAffiliateAdminConfigEnv(t *testing.T) *affiliateConfigEnv {
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
		JWTSecret:       "affiliate-config-test-secret",
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
	e := newAffiliateAdminConfigRouter(t, h, svc, signer)

	t.Cleanup(pool.Close)
	return &affiliateConfigEnv{pool: pool, rdb: rdb, e: e, signer: signer}
}

// tokenFor mints a valid access token for a real seeded user (settings PUT
// writes updated_by with an FK to users) and seeds its session so
// JWTMiddleware's SessionActive check passes.
func (env *affiliateConfigEnv) tokenFor(t *testing.T, role string) string {
	t.Helper()
	var sub string
	if err := env.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, $3, 'active', false) RETURNING id`,
		"Config Admin "+uniqueHandlerSuffix(), "affcfgtok_"+uniqueHandlerSuffix(), role,
	).Scan(&sub); err != nil {
		t.Fatalf("seed actor user: %v", err)
	}
	tok, jti, err := env.signer.SignAccess(sub, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, sub, time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return tok
}

func putJSONWithToken(t *testing.T, e interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func deleteWithToken(t *testing.T, e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

const validAffiliateSettingsBody = `{
	"cookie_days": 45,
	"binding_months": 3,
	"min_extend_amount": 0,
	"digital_wait_days": 7,
	"physical_wait_days": 14,
	"min_payout": 100000,
	"payout_day_of_month": null,
	"adjustment_threshold": 1000000,
	"adjustment_threshold_enabled": false,
	"freeze_max_days": 14,
	"soon_end_days": 30,
	"launch_date": null,
	"eval_day": 8,
	"silver_threshold": 5000000,
	"gold_threshold": 15000000,
	"hold_thresholds": {"bronze": 1000000, "silver": null},
	"daily_registration_cap": null,
	"levels": [
		{"level": "silver", "commission_rule": {"exam": {"type": "percent", "value": 12}}, "binding_months": 6, "digital_wait_days": 3, "min_payout": 50000}
	]
}`

func seedConfigOverrideProduct(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO product (name, type, price, status) VALUES ($1, 'book', 60000, 'published') RETURNING id`,
		"Config Handler Product "+uniqueHandlerSuffix(),
	).Scan(&id); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return id
}

func TestAffiliateAdminConfigHandlerRBAC(t *testing.T) {
	env := newAffiliateAdminConfigEnv(t)
	seedAffiliateAdminRow(t, env.pool, "active")
	productID := seedConfigOverrideProduct(t, env.pool)
	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	surfaces := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"settings read", http.MethodGet, "/api/v1/admin/affiliate-settings", ""},
		{"settings write", http.MethodPut, "/api/v1/admin/affiliate-settings", validAffiliateSettingsBody},
		{"overrides read", http.MethodGet, "/api/v1/admin/affiliate-product-overrides", ""},
		{"overrides product search", http.MethodGet, "/api/v1/admin/affiliate-product-overrides/products?q=a", ""},
		{"overrides write", http.MethodPut, "/api/v1/admin/affiliate-product-overrides/" + productID.String(), `{"type":"percent","value":10}`},
		{"overrides delete", http.MethodDelete, "/api/v1/admin/affiliate-product-overrides/" + productID.String(), ""},
		{"reports summary", http.MethodGet, "/api/v1/admin/affiliate-reports/summary?from=" + from + "&to=" + to, ""},
		{"reports orders", http.MethodGet, "/api/v1/admin/affiliate-reports/orders", ""},
		{"reports orders export", http.MethodGet, "/api/v1/admin/affiliate-reports/orders/export", ""},
		{"reports liability", http.MethodGet, "/api/v1/admin/affiliate-reports/liability", ""},
	}

	t.Run("a missing token is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/affiliate-settings", nil)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin_store is forbidden on the five surfaces", func(t *testing.T) {
		for _, surface := range surfaces {
			t.Run(surface.name, func(t *testing.T) {
				token := env.tokenFor(t, service.RoleAdminStore)
				var rec *httptest.ResponseRecorder
				switch surface.method {
				case http.MethodGet:
					rec = getWithToken(t, env.e, surface.path, token)
				case http.MethodPut:
					rec = putJSONWithToken(t, env.e, surface.path, token, surface.body)
				case http.MethodDelete:
					rec = deleteWithToken(t, env.e, surface.path, token)
				default:
					rec = postJSONWithToken(t, env.e, surface.path, token, surface.body)
				}
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s %s: want 403, got %d: %s", surface.method, surface.path, rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("super_admin reads and writes the surfaces", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)

		rec := getWithToken(t, env.e, "/api/v1/admin/affiliate-settings", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET settings: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var got struct {
			Settings struct {
				CookieDays int `json:"cookie_days"`
			} `json:"settings"`
			Levels []struct {
				Level string `json:"level"`
			} `json:"levels"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode settings: %v", err)
		}
		if len(got.Levels) != 3 {
			t.Errorf("GET settings levels = %d rows, want the three level rows", len(got.Levels))
		}

		rec = putJSONWithToken(t, env.e, "/api/v1/admin/affiliate-settings", token, validAffiliateSettingsBody)
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT settings: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var putBack struct {
			Settings struct {
				CookieDays int `json:"cookie_days"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &putBack); err != nil {
			t.Fatalf("decode put settings: %v", err)
		}
		if putBack.Settings.CookieDays != 45 {
			t.Errorf("PUT settings readback cookie_days = %d, want 45", putBack.Settings.CookieDays)
		}

		rec = getWithToken(t, env.e, "/api/v1/admin/affiliate-product-overrides", token)
		if rec.Code != http.StatusOK {
			t.Errorf("GET overrides: want 200, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = putJSONWithToken(t, env.e, "/api/v1/admin/affiliate-product-overrides/"+productID.String(), token, `{"type":"percent","value":10}`)
		if rec.Code != http.StatusOK {
			t.Errorf("PUT override: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var ovr struct {
			Type  string  `json:"type"`
			Value float64 `json:"value"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &ovr); err != nil {
			t.Fatalf("decode override: %v", err)
		}
		if ovr.Type != "percent" || ovr.Value != 10 {
			t.Errorf("override = %+v, want percent 10", ovr)
		}

		rec = deleteWithToken(t, env.e, "/api/v1/admin/affiliate-product-overrides/"+productID.String(), token)
		if rec.Code != http.StatusOK {
			t.Errorf("DELETE override: want 200, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/summary?from="+from+"&to="+to, token)
		if rec.Code != http.StatusOK {
			t.Errorf("GET reports summary: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var summary struct {
			Data   []json.RawMessage `json:"data"`
			Totals json.RawMessage   `json:"totals"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
			t.Fatalf("decode summary: %v", err)
		}
		if summary.Totals == nil {
			t.Errorf("summary response lacks the totals row: %s", rec.Body.String())
		}

		rec = getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/liability", token)
		if rec.Code != http.StatusOK {
			t.Errorf("GET reports liability: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("an invalid settings body is rejected", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := putJSONWithToken(t, env.e, "/api/v1/admin/affiliate-settings", token,
			strings.Replace(validAffiliateSettingsBody, `"cookie_days": 45`, `"cookie_days": 0`, 1))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("PUT settings cookie_days 0: want 422, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAffiliateAdminCommissionsEndpoint(t *testing.T) {
	env := newAffiliateAdminConfigEnv(t)
	affID, _ := seedAffiliateAdminRow(t, env.pool, "active")
	base := "/api/v1/admin/affiliates/" + affID + "/commissions"

	if rec := getWithToken(t, env.e, base, env.tokenFor(t, service.RoleAdminStore)); rec.Code != http.StatusForbidden {
		t.Errorf("admin_store: want 403, got %d", rec.Code)
	}

	token := env.tokenFor(t, service.RoleSuperAdmin)
	rec := getWithToken(t, env.e, base+"?status=all&per_page=5", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET commissions: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Data    []json.RawMessage `json:"data"`
		Total   int               `json:"total"`
		PerPage int               `json:"per_page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Data == nil || page.PerPage != 5 {
		t.Errorf("page = %+v, want an empty array (not null) and per_page 5", page)
	}

	rec = getWithToken(t, env.e, base+"/export", token)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
		t.Errorf("export: code %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := getWithToken(t, env.e, "/api/v1/admin/affiliates/"+uuid.NewString()+"/commissions", token); rec.Code != http.StatusNotFound {
		t.Errorf("unknown affiliator: want 404, got %d", rec.Code)
	}
	if rec := getWithToken(t, env.e, base+"?from=nope", token); rec.Code != http.StatusBadRequest {
		t.Errorf("bad date: want 400, got %d", rec.Code)
	}
}

func TestAffiliateAdminReportOrdersEndpoint(t *testing.T) {
	env := newAffiliateAdminConfigEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/orders?status=all&per_page=5", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET report orders: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Data    []json.RawMessage `json:"data"`
		PerPage int               `json:"per_page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Data == nil || page.PerPage != 5 {
		t.Errorf("page = %+v, want an array (not null) and per_page 5", page)
	}
	rec = getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/orders/export", token)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
		t.Errorf("export: code %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/orders?from=nope", token); rec.Code != http.StatusBadRequest {
		t.Errorf("bad date: want 400, got %d", rec.Code)
	}
}
