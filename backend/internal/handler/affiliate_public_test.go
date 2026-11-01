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

// affiliatePublicEnv is a private, migrated database per test: the affiliate
// flag and settings are singleton/global state, so each test asserts
// flag-off and flag-on behavior against its own copy.
type affiliatePublicEnv struct {
	pool *pgxpool.Pool
	e    *echo.Echo
}

func newAffiliatePublicEnv(t *testing.T) *affiliatePublicEnv {
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
		JWTSecret:       "affiliate-test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	svc := service.NewWithStore(
		store, store, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, cfg,
		nil,
	)
	h := handler.New(svc)

	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	v1.GET("/affiliates/public/validate", h.AffiliateValidateCode)
	v1.GET("/affiliates/public/status", h.AffiliateProgramStatus)
	v1.GET("/affiliates/public/validate-limited", h.AffiliateValidateCode, handler.AffiliateRateLimiter())

	t.Cleanup(pool.Close)
	return &affiliatePublicEnv{pool: pool, e: e}
}

func setAffiliateFlag(t *testing.T, pool *pgxpool.Pool, value string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`,
		value); err != nil {
		t.Fatalf("set affiliate flag: %v", err)
	}
}

// seedAffiliator inserts a user + affiliator row with the given status and a
// unique code; every test runs on its own database clone, so no row cleanup
// is needed.
func seedAffiliator(t *testing.T, pool *pgxpool.Pool, status string) (code, name string) {
	t.Helper()
	name = "Affiliator " + uniqueHandlerSuffix()
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affpub_"+uniqueHandlerSuffix(),
	).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	code = strings.ToUpper("AF" + uniqueHandlerSuffix()[:6])
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version) VALUES ($1, $2, $3, 'v1')`,
		userID, code, status); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return code, name
}

func uniqueHandlerSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

func TestAffiliateValidateCodeHandler(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		flag      *string
		code      func(affiliatorCode string) string
		wantValid bool
		wantName  bool
		wantDays  int
	}{
		{"active affiliator with flag on is valid", "active", strPtrP("true"), keepCode, true, true, 30},
		{"unknown code is valid:false without error", "active", strPtrP("true"), noCode, false, false, 30},
		{"pending affiliator is valid:false", "pending_approval", strPtrP("true"), keepCode, false, false, 30},
		{"flag off ignores an active code", "active", strPtrP("false"), keepCode, false, false, 0},
		{"unset flag is default off", "active", nil, keepCode, false, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newAffiliatePublicEnv(t)
			code, name := seedAffiliator(t, env.pool, tt.status)
			if tt.flag != nil {
				setAffiliateFlag(t, env.pool, *tt.flag)
			}

			q := "/api/v1/affiliates/public/validate?code=" + tt.code(code)
			req := httptest.NewRequest(http.MethodGet, q, nil)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: want 200, got %d: %s", q, rec.Code, rec.Body.String())
			}
			var body struct {
				Valid          bool   `json:"valid"`
				AffiliatorName string `json:"affiliator_name"`
				CookieDays     int    `json:"cookie_days"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}
			if body.Valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", body.Valid, tt.wantValid)
			}
			if body.CookieDays != tt.wantDays {
				t.Errorf("cookie_days = %d, want %d", body.CookieDays, tt.wantDays)
			}
			if tt.wantName && body.AffiliatorName != name {
				t.Errorf("affiliator_name = %q, want %q", body.AffiliatorName, name)
			}
			if !tt.wantName && body.AffiliatorName != "" {
				t.Errorf("affiliator_name = %q, want empty", body.AffiliatorName)
			}
		})
	}
}

func keepCode(code string) string { return code }
func noCode(string) string        { return "UNKNOWN1" }

func strPtrP(s string) *string { return &s }

func TestAffiliateRateLimiter_returns429(t *testing.T) {
	env := newAffiliatePublicEnv(t)
	setAffiliateFlag(t, env.pool, "true")

	var rec *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/affiliates/public/validate-limited?code=AFTEST01", nil)
		req.RemoteAddr = "198.51.100.77:1234"
		rec = httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: want 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	if body["code"] != "rate_limited" {
		t.Errorf("code = %q, want rate_limited", body["code"])
	}
}

func TestAffiliateProgramStatusHandler(t *testing.T) {
	for _, tt := range []struct {
		flag string
		want bool
	}{{"true", true}, {"false", false}} {
		t.Run("flag "+tt.flag, func(t *testing.T) {
			env := newAffiliatePublicEnv(t)
			setAffiliateFlag(t, env.pool, tt.flag)

			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/affiliates/public/status", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal %q: %v", rec.Body.String(), err)
			}
			if body.Enabled != tt.want {
				t.Errorf("enabled = %v, want %v", body.Enabled, tt.want)
			}
		})
	}
}
