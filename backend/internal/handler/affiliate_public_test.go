package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	v1.POST("/affiliates/clicks", h.AffiliateRecordClick, handler.AffiliateClickRateLimiter())

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
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, $3, 'v1')`,
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
	}{
		{"active affiliator with flag on is valid", "active", strPtrP("true"), keepCode, true, true},
		{"unknown code is valid:false without error", "active", strPtrP("true"), noCode, false, false},
		{"pending affiliator is valid:false", "pending_approval", strPtrP("true"), keepCode, false, false},
		{"flag off ignores an active code", "active", strPtrP("false"), keepCode, false, false},
		{"unset flag is default off", "active", nil, keepCode, false, false},
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
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}
			if body.Valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", body.Valid, tt.wantValid)
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

func TestAffiliateRecordClickHandler(t *testing.T) {
	tests := []struct {
		name       string
		flag       *string
		code       func(affiliatorCode string) string
		wantRecord bool
	}{
		{"flag on and active affiliator records click", strPtrP("true"), keepCode, true},
		{"flag off is a no-op", strPtrP("false"), keepCode, false},
		{"unknown code records nothing", strPtrP("true"), noCode, false},
		{"unset flag is default off", nil, keepCode, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newAffiliatePublicEnv(t)
			code, _ := seedAffiliator(t, env.pool, "active")
			if tt.flag != nil {
				setAffiliateFlag(t, env.pool, *tt.flag)
			}

			payload := fmt.Sprintf(`{"code":%q,"user_agent":"Mozilla/5.0 Test"}`, tt.code(code))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/affiliates/clicks", strings.NewReader(payload))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.RemoteAddr = "198.51.100.10:54321"
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("POST clicks: want 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Recorded bool `json:"recorded"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}
			if body.Recorded != tt.wantRecord {
				t.Errorf("recorded = %v, want %v", body.Recorded, tt.wantRecord)
			}

			var n int
			q := env.pool.QueryRow(context.Background(), `SELECT count(*) FROM affiliate_clicks WHERE user_agent = 'Mozilla/5.0 Test'`)
			if err := q.Scan(&n); err != nil {
				t.Fatalf("count clicks: %v", err)
			}
			if tt.wantRecord && n != 1 {
				t.Errorf("affiliate_clicks rows = %d, want 1", n)
			}
			if !tt.wantRecord && n != 0 {
				t.Errorf("affiliate_clicks rows = %d, want 0", n)
			}
		})
	}
}

func TestAffiliateRecordClick_sameDayRepeatIsDeduped(t *testing.T) {
	env := newAffiliatePublicEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	code, _ := seedAffiliator(t, env.pool, "active")

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/affiliates/clicks", strings.NewReader(fmt.Sprintf(`{"code":%q,"user_agent":"dedup-ua"}`, code)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.RemoteAddr = "198.51.100.99:5555"
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("click %d: want 200, got %d", i, rec.Code)
		}
		var body struct {
			Recorded bool `json:"recorded"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
		}
		want := i == 0
		if body.Recorded != want {
			t.Errorf("click %d recorded = %v, want %v", i, body.Recorded, want)
		}
	}

	var n int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_clicks WHERE user_agent = 'dedup-ua'`).Scan(&n); err != nil {
		t.Fatalf("count clicks: %v", err)
	}
	if n != 1 {
		t.Errorf("affiliate_clicks rows = %d, want 1 after same-day dedupe", n)
	}
}

func TestAffiliateClickRateLimiter_returns429(t *testing.T) {
	env := newAffiliatePublicEnv(t)
	setAffiliateFlag(t, env.pool, "true")

	var rec *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/affiliates/clicks", strings.NewReader(`{"code":"AFTEST01","user_agent":"ua"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.RemoteAddr = "198.51.100.77:1234"
		rec = httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th click: want 429, got %d: %s", rec.Code, rec.Body.String())
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
