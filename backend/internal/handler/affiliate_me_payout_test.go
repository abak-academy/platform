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

// affiliatePayoutEnv wires the /affiliates/me payout routes with
// the real JWT and AffiliatorOnlyMiddleware, one migrated database per test.
type affiliatePayoutEnv struct {
	pool   *pgxpool.Pool
	e      *echo.Echo
	signer *infra.JWTSigner
	rdb    *redis.Client
}

func newAffiliatePayoutEnv(t *testing.T) *affiliatePayoutEnv {
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
		JWTSecret:       "affiliate-payout-test-secret",
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
	me := v1.Group("/affiliates/me", handler.JWTMiddleware(svc, signer), handler.AffiliatorOnlyMiddleware())
	me.POST("/payouts", h.AffiliateMeRequestPayout)
	me.GET("/payouts", h.AffiliateMePayouts)
	me.GET("/adjustments", h.AffiliateMeAdjustments)

	t.Cleanup(pool.Close)
	return &affiliatePayoutEnv{pool: pool, e: e, signer: signer, rdb: rdb}
}

func (env *affiliatePayoutEnv) tokenFor(t *testing.T, userID, role string) string {
	t.Helper()
	tok, jti, err := env.signer.SignAccess(userID, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, userID, time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return tok
}

func seedAffiliatePayoutOwner(t *testing.T, pool *pgxpool.Pool, name string) affiliateMeOwner {
	t.Helper()
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affpay_"+uniqueHandlerSuffix(),
	).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	code := "AF" + uniqueHandlerSuffix()[:6]
	var affID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version) VALUES ($1, $2, 'active', 'v1') RETURNING user_id`,
		userID, code,
	).Scan(&affID); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return affiliateMeOwner{userID: userID, affID: affID, code: code}
}

func seedAffiliatePayoutBank(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE affiliators SET bank_account = '{"bank_name":"BCA","account_number":"1234567890","account_holder":"Budi Santoso"}' WHERE user_id = $1`,
		affiliatorID); err != nil {
		t.Fatalf("seed bank: %v", err)
	}
}

func seedAffiliatePayoutSaldo(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID, amount float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, detail)
		 VALUES ($1, 'adjustment', 'posted', $2, now(), '{"category":"other"}')`,
		affiliatorID, amount); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
}

func seedAffiliatePayoutRow(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID, status string, amount float64, requestedAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, created_at, detail)
		 VALUES ($1, 'payout', $2, -$3::numeric, $4, $4,
		     jsonb_build_object('bank_account', jsonb_build_object(
		         'bank_name', 'BCA', 'account_number', '1234567890', 'account_holder', $5::text))) RETURNING id`,
		affiliatorID, status, amount, requestedAt, "Budi Santoso",
	).Scan(&id); err != nil {
		t.Fatalf("seed payout: %v", err)
	}
	return id
}

func TestAffiliateMePayoutHandler_roleGates(t *testing.T) {
	env := newAffiliatePayoutEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Pay Gate "+uniqueHandlerSuffix())

	tests := []struct {
		name  string
		role  string
		token bool
		want  int
	}{
		{"super_admin is forbidden", service.RoleSuperAdmin, true, http.StatusForbidden},
		{"admin_store is forbidden", service.RoleAdminStore, true, http.StatusForbidden},
		{"student is forbidden", service.RoleStudent, true, http.StatusForbidden},
		{"missing token is unauthorized", "", false, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := ""
			if tt.token {
				token = env.tokenFor(t, uuid.NewString(), tt.role)
			}
			for _, tc := range []struct{ method, path string }{
				{http.MethodPost, "/api/v1/affiliates/me/payouts"},
				{http.MethodGet, "/api/v1/affiliates/me/payouts"},
				{http.MethodGet, "/api/v1/affiliates/me/adjustments"},
			} {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				if tc.method == http.MethodPost {
					req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				}
				rec := httptest.NewRecorder()
				env.e.ServeHTTP(rec, req)
				if rec.Code != tt.want {
					t.Errorf("%s %s as %q: want %d, got %d: %s", tc.method, tc.path, tt.role, tt.want, rec.Code, rec.Body.String())
				}
			}
		})
	}
	_ = owner
}

func TestAffiliateMePayoutHandler_noLeakBetweenAffiliators(t *testing.T) {
	env := newAffiliatePayoutEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	ownerA := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Pay A "+uniqueHandlerSuffix())
	ownerB := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Pay B "+uniqueHandlerSuffix())

	seedAffiliatePayoutBank(t, env.pool, ownerA.affID)
	seedAffiliatePayoutSaldo(t, env.pool, ownerA.affID, 200000)
	seedAffiliatePayoutRow(t, env.pool, ownerA.affID, "requested", 100000, time.Now().UTC())

	tokenB := env.tokenFor(t, ownerB.userID, service.RoleAffiliator)

	payouts := getWithToken(t, env.e, "/api/v1/affiliates/me/payouts", tokenB)
	if payouts.Code != http.StatusOK {
		t.Fatalf("GET payouts as B: want 200, got %d: %s", payouts.Code, payouts.Body.String())
	}
	var payoutPage struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(payouts.Body.Bytes(), &payoutPage); err != nil {
		t.Fatalf("unmarshal payouts B: %v", err)
	}
	if payoutPage.Total != 0 || len(payoutPage.Data) != 0 {
		t.Errorf("payouts for B = %v, want empty (A's rows are private)", payoutPage)
	}

	adjustments := getWithToken(t, env.e, "/api/v1/affiliates/me/adjustments", tokenB)
	if adjustments.Code != http.StatusOK {
		t.Fatalf("GET adjustments as B: want 200, got %d: %s", adjustments.Code, adjustments.Body.String())
	}
}

func TestAffiliateMePayoutHandler_validationErrors(t *testing.T) {
	env := newAffiliatePayoutEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Pay Bad "+uniqueHandlerSuffix())
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"negative amount is rejected", http.MethodPost, "/api/v1/affiliates/me/payouts", `{"amount":-1}`},
		{"zero amount is rejected", http.MethodPost, "/api/v1/affiliates/me/payouts", `{"amount":0}`},
		{"broken json is rejected", http.MethodPost, "/api/v1/affiliates/me/payouts", `{"amount":`},
		{"page 0 is rejected", http.MethodGet, "/api/v1/affiliates/me/payouts?page=0", ""},
		{"bad page on adjustments is rejected", http.MethodGet, "/api/v1/affiliates/me/adjustments?page=many", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s: want 400, got %d: %s", tt.method, tt.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAffiliateMePayoutHandler_requestPayoutHappyPath(t *testing.T) {
	env := newAffiliatePayoutEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Pay Ok "+uniqueHandlerSuffix())
	seedAffiliatePayoutBank(t, env.pool, owner.affID)
	seedAffiliatePayoutSaldo(t, env.pool, owner.affID, 150000)
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/affiliates/me/payouts", strings.NewReader(``))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST payouts: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var payout struct {
		ID     uuid.UUID `json:"id"`
		Amount float64   `json:"amount"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payout); err != nil {
		t.Fatalf("unmarshal payout: %v", err)
	}
	if payout.ID == uuid.Nil || payout.Status != "requested" || payout.Amount != 150000 {
		t.Errorf("payout body = %+v, want full saldo requested", payout)
	}

	var ledgerAmount float64
	var payoutID, actorID uuid.UUID
	if err := env.pool.QueryRow(context.Background(),
		`SELECT amount, id, actor_id FROM affiliate_transactions
		 WHERE affiliator_id = $1 AND type = 'payout'`, owner.affID,
	).Scan(&ledgerAmount, &payoutID, &actorID); err != nil {
		t.Fatalf("read ledger row: %v", err)
	}
	if ledgerAmount != -150000 || payoutID != payout.ID || actorID.String() != owner.userID {
		t.Errorf("ledger row = amount %v payout %v actor %v, want -150000 %v %v",
			ledgerAmount, payoutID, actorID, payout.ID, owner.userID)
	}

	var audits int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = 'affiliate.payout_requested' AND target_id = $1 AND actor_id = $2`,
		payout.ID.String(), owner.userID,
	).Scan(&audits); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if audits != 1 {
		t.Errorf("audit rows = %d, want 1", audits)
	}

	list := getWithToken(t, env.e, "/api/v1/affiliates/me/payouts", token)
	if list.Code != http.StatusOK {
		t.Fatalf("GET payouts: want 200, got %d: %s", list.Code, list.Body.String())
	}
	var page struct {
		Data             []map[string]any `json:"data"`
		Saldo            float64          `json:"saldo"`
		MinPayout        float64          `json:"min_payout"`
		PayoutDayOfMonth *int             `json:"payout_day_of_month"`
		Total            int              `json:"total"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal payout page: %v", err)
	}
	if page.Total != 1 || len(page.Data) != 1 {
		t.Errorf("payout page = %+v, want the one requested payout", page)
	}
	if page.Saldo != 0 {
		t.Errorf("saldo = %v, want 0 after the full-saldo request", page.Saldo)
	}
	if page.MinPayout <= 0 {
		t.Errorf("min_payout = %v, want the effective minimum", page.MinPayout)
	}
	if page.PayoutDayOfMonth != nil {
		t.Errorf("payout_day_of_month = %v, want unset by default", *page.PayoutDayOfMonth)
	}
}
