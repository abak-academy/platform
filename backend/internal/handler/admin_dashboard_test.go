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

// TestAdminDashboardRejectsInvertedRange pins the fix for the crash where an
// inverted or empty custom period (nothing in PeriodBar stopped a super
// admin from typing `to` before `from`) made generate_series produce zero
// rows, series came back `null`, and the whole /admin page went blank on
// `d.series.map`. parseDayRange advances `to` by one day, so from=2026-08-06
// / to=2026-08-05 becomes an equal, not just inverted, pair after parsing —
// the boundary this rejects, not only strict inversion.
func TestAdminDashboardRejectsInvertedRange(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard?from=2026-08-06&to=2026-08-05", token)

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

// TestAdminDashboardDefaultPeriodEndsToday pins the no-params path, which is
// the common case: presetRange("30d") on the frontend sends neither `from`
// nor `to`. The handler must derive a midnight-aligned default `to` the way
// parseDayRange would for an explicit "today", not pass time.Now() straight
// through — otherwise the echoed period.to reads one day behind the data the
// query actually covers.
func TestAdminDashboardDefaultPeriodEndsToday(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard", token)
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

	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load Asia/Jakarta: %v", err)
	}
	wantTo := time.Now().In(jkt).Format("2006-01-02")
	if body.Period.To != wantTo {
		t.Errorf("period.to = %q, want today (%q) in Asia/Jakarta", body.Period.To, wantTo)
	}
}

// TestAdminDashboardDefaultPeriodSpansThirtyDays pins the off-by-one where the
// default window was today-30..today+1 (exclusive) — 31 days — while the UI
// preset is labelled "30 hari". period.to names the last included day, so an
// exact 30-day span is period.to - period.from == 29 days.
func TestAdminDashboardDefaultPeriodSpansThirtyDays(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard", token)
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

	from, err := time.Parse("2006-01-02", body.Period.From)
	if err != nil {
		t.Fatalf("parse period.from: %v", err)
	}
	to, err := time.Parse("2006-01-02", body.Period.To)
	if err != nil {
		t.Fatalf("parse period.to: %v", err)
	}

	gotDays := int(to.Sub(from).Hours()/24) + 1
	if gotDays != 30 {
		t.Errorf("default window spans %d days, want 30 (from=%s to=%s)", gotDays, body.Period.From, body.Period.To)
	}
}
