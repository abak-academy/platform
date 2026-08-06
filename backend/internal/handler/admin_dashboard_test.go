package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// adminDashboardTestEnv wires the real Postgres fixture (shipHandlerRepoFixture,
// defined in admin_order_handler_test.go) behind the full production route set,
// so RBACMiddleware("revenue:read") and JWTMiddleware run for real rather than
// being bypassed by calling the handler method directly.
type adminDashboardTestEnv struct {
	e      *echo.Echo
	rdb    *redis.Client
	signer *infra.JWTSigner
}

func newAdminDashboardTestEnv(t *testing.T) *adminDashboardTestEnv {
	t.Helper()
	repo := shipHandlerRepoFixture(t)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	signer := infra.NewJWTSigner("dashboard-handler-test-secret", time.Hour)
	svc := service.NewWithStore(repo, repo, rdb, signer, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, nil, nil)
	h := handler.New(svc)

	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, signer)

	return &adminDashboardTestEnv{e: e, rdb: rdb, signer: signer}
}

// tokenFor mints a valid access token for role and seeds its session in Redis
// so JWTMiddleware's SessionActive check passes.
func (env *adminDashboardTestEnv) tokenFor(t *testing.T, role string) string {
	t.Helper()
	sub := "dashboard-test-" + role
	tok, jti, err := env.signer.SignAccess(sub, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, sub, time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return tok
}

func TestAdminDashboardRejectsAdminStore(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleAdminStore)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard", token)

	// admin_store has no revenue:read, deliberately — see the 2026-08-04 split.
	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", rec.Code)
	}
}

func TestAdminDashboardAllowsSuperAdmin(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"period", "kpi", "series", "attention", "top_products", "upcoming_exams"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing %q", key)
		}
	}
}

func TestAdminDashboardRejectsBadDate(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard?from=08-2026", token)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestAdminDashboardRejectsUnknownBucket(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard?bucket=fortnight", token)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 — bucket is interpolated into SQL", rec.Code)
	}
}

func TestAdminDashboardEchoesRequestedPeriod(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard?from=2026-07-01&to=2026-07-31", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Period struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"period"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// `to` is exclusive internally; the echo must name the last day included,
	// or the page's period label reads one day later than the data.
	if body.Period.From != "2026-07-01" || body.Period.To != "2026-07-31" {
		t.Errorf("period = %s..%s, want 2026-07-01..2026-07-31", body.Period.From, body.Period.To)
	}
}
