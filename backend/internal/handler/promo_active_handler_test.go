package handler_test

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// This file exercises the NF-2 active-public-promo listing at the HTTP
// boundary (server.RegisterRoutesForTest) — the real route table, not the
// service layer directly — so FR-9 (401 unauthenticated), FR-11 (trimmed
// DTO), and FR-12 (validate stays anonymous) are proven against production
// routing, not a hand-built subset of it.

var (
	promoActiveDBOnce sync.Once
	promoActiveDBEnv  *promoActiveTestEnv
)

type promoActiveTestEnv struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	e      *echo.Echo
	signer *infra.JWTSigner
}

func newPromoActiveDBEnv(t *testing.T) *promoActiveTestEnv {
	t.Helper()
	promoActiveDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_promo_active_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("connection string: %v", err)
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		store := repository.New(pool)

		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("miniredis: %v", err)
		}
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

		cfg := &config.Config{
			JWTSecret:       "test-secret",
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
		)
		e := echo.New()
		e.HideBanner = true
		h := handler.New(svc)
		server.RegisterRoutesForTest(e, h, svc, signer)

		promoActiveDBEnv = &promoActiveTestEnv{pool: pool, rdb: rdb, e: e, signer: signer}
	})
	if promoActiveDBEnv == nil {
		t.Fatal("promo active test env failed to initialize")
	}
	return promoActiveDBEnv
}

func promoActiveToken(t *testing.T, env *promoActiveTestEnv, userID, role string) string {
	t.Helper()
	tokenString, jti, err := env.signer.SignAccess(userID, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

func insertPromoActiveStudent(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	email := "promo-active-student-" + time.Now().Format("150405.000000") + "@test.local"
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		email, service.RoleStudent, "Promo Active Student",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

// TestListActivePublicPromos_Unauthenticated_Returns401 is FR-9.
func TestListActivePublicPromos_Unauthenticated_Returns401(t *testing.T) {
	env := newPromoActiveDBEnv(t)

	_, err := env.pool.Exec(context.Background(),
		`INSERT INTO promo_code (code, is_public, discount_percent) VALUES ($1, true, 10)`,
		"NOAUTH-LEAK-TEST",
	)
	if err != nil {
		t.Fatalf("seed public promo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/promo-codes/active", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "NOAUTH-LEAK-TEST") {
		t.Fatalf("401 response body must not contain promo code data, got %s", rec.Body.String())
	}
}

// TestListActivePublicPromos_Authenticated_TrimmedDTO is FR-10 + FR-11 at
// the HTTP boundary: an authenticated student sees only the code, and the
// serialised JSON keys exclude id/used_count/max_uses.
func TestListActivePublicPromos_Authenticated_TrimmedDTO(t *testing.T) {
	env := newPromoActiveDBEnv(t)
	studentID := insertPromoActiveStudent(t, env.pool)
	token := promoActiveToken(t, env, studentID, service.RoleStudent)

	uniqueCode := "TRIMMED-" + time.Now().Format("150405.000000")
	_, err := env.pool.Exec(context.Background(),
		`INSERT INTO promo_code (code, is_public, discount_percent, min_order_amount, max_discount_amount)
		VALUES ($1, true, 15, 50000, 20000)`,
		uniqueCode,
	)
	if err != nil {
		t.Fatalf("seed public promo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/promo-codes/active", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Real captured response — this is the shape Task 13's fixture derives from.
	t.Logf("captured GET /promo-codes/active response: %s", rec.Body.String())

	var resp struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var found map[string]json.RawMessage
	for _, item := range resp.Data {
		var code string
		if err := json.Unmarshal(item["code"], &code); err == nil && code == uniqueCode {
			found = item
			break
		}
	}
	if found == nil {
		t.Fatalf("seeded public promo %q not found in response: %s", uniqueCode, rec.Body.String())
	}

	for _, forbiddenKey := range []string{"id", "used_count", "max_uses"} {
		if _, present := found[forbiddenKey]; present {
			t.Errorf("serialised promo must not carry key %q, got keys=%v", forbiddenKey, keysOf(found))
		}
	}
	for _, requiredKey := range []string{"code", "discount_percent", "discount_amount", "min_order_amount", "max_discount_amount", "expires_at"} {
		if _, present := found[requiredKey]; !present {
			t.Errorf("serialised promo must carry key %q, got keys=%v", requiredKey, keysOf(found))
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestValidatePromo_StaysAnonymous_AfterActiveRouteRegistered is FR-12: the
// new authenticated sibling group must not leak JWTMiddleware onto the
// pre-existing anonymous POST /promo-codes/validate route.
func TestValidatePromo_StaysAnonymous_AfterActiveRouteRegistered(t *testing.T) {
	env := newPromoActiveDBEnv(t)

	code := "ANON-VALIDATE-" + time.Now().Format("150405.000000")
	_, err := env.pool.Exec(context.Background(),
		`INSERT INTO promo_code (code, discount_percent) VALUES ($1, 10)`,
		code,
	)
	if err != nil {
		t.Fatalf("seed promo: %v", err)
	}

	body := `{"code":"` + code + `","subtotal":100000,"shipping_cost":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/promo-codes/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Deliberately no Authorization header.
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /promo-codes/validate without auth: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
