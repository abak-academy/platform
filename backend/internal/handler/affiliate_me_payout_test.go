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

// affiliatePayoutEnv wires the /affiliates/me payout and violation routes with
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
	me.GET("/violations", h.AffiliateMeViolations)
	me.POST("/violations/:id/appeal", h.AffiliateMeAppealViolation)

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
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, 'active', 'v1') RETURNING id`,
		userID, code,
	).Scan(&affID); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return affiliateMeOwner{userID: userID, affID: affID, code: code}
}

func seedAffiliatePayoutBank(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE affiliators SET bank_name = 'BCA', bank_account_number = '1234567890', bank_account_holder = 'Budi Santoso' WHERE id = $1`,
		affiliatorID); err != nil {
		t.Fatalf("seed bank: %v", err)
	}
}

func seedAffiliatePayoutSaldo(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID, amount float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount) VALUES ($1, 'commission', $2)`,
		affiliatorID, amount); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
}

func seedAffiliatePayoutRow(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID, status string, amount float64, requestedAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliate_payouts (affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder, requested_at)
		 VALUES ($1, $2, $3, 'BCA', '1234567890', 'Budi Santoso', $4) RETURNING id`,
		affiliatorID, amount, status, requestedAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed payout: %v", err)
	}
	return id
}

func seedAffiliatePayoutDecider(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'super_admin', 'active', false) RETURNING id`,
		name, "paydec_"+uniqueHandlerSuffix(),
	).Scan(&id); err != nil {
		t.Fatalf("seed decider: %v", err)
	}
	return uuid.MustParse(id)
}

func seedAffiliatePayoutViolation(t *testing.T, pool *pgxpool.Pool, affiliatorID, deciderID uuid.UUID, status, appealStatus string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliate_violations (affiliator_id, category, description, sanction, status, decided_by, decided_at, appeal_status, created_at)
		 VALUES ($1, 'self_purchase', 'pesan sendiri', 'warning', $2, $3, $4, $5, $4) RETURNING id`,
		affiliatorID, status, deciderID, createdAt, appealStatus,
	).Scan(&id); err != nil {
		t.Fatalf("seed violation: %v", err)
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
				{http.MethodGet, "/api/v1/affiliates/me/violations"},
				{http.MethodPost, "/api/v1/affiliates/me/violations/" + uuid.NewString() + "/appeal"},
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

	decider := seedAffiliatePayoutDecider(t, env.pool, "Admin Pay Privacy "+uniqueHandlerSuffix())
	seedAffiliatePayoutBank(t, env.pool, ownerA.affID)
	seedAffiliatePayoutSaldo(t, env.pool, ownerA.affID, 200000)
	seedAffiliatePayoutRow(t, env.pool, ownerA.affID, "requested", 100000, time.Now().UTC())
	seedAffiliatePayoutViolation(t, env.pool, ownerA.affID, decider, "resolved", "none", time.Now().UTC())

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

	violations := getWithToken(t, env.e, "/api/v1/affiliates/me/violations", tokenB)
	if violations.Code != http.StatusOK {
		t.Fatalf("GET violations as B: want 200, got %d: %s", violations.Code, violations.Body.String())
	}
	var violationPage struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(violations.Body.Bytes(), &violationPage); err != nil {
		t.Fatalf("unmarshal violations B: %v", err)
	}
	if violationPage.Total != 0 {
		t.Errorf("violations for B = %d, want 0", violationPage.Total)
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
		{"bad page on violations is rejected", http.MethodGet, "/api/v1/affiliates/me/violations?page=-3", ""},
		{"bad violation id is rejected", http.MethodPost, "/api/v1/affiliates/me/violations/nope/appeal", `{"text":"saya keberatan"}`},
		{"empty appeal text is rejected", http.MethodPost, "/api/v1/affiliates/me/violations/" + uuid.NewString() + "/appeal", `{"text":"   "}`},
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
		`SELECT amount, payout_id, actor_id FROM affiliate_ledger_entries
		 WHERE affiliator_id = $1 AND entry_type = 'payout'`, owner.affID,
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

func TestAffiliateMeAppealHandler_recordsPendingAppeal(t *testing.T) {
	env := newAffiliatePayoutEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliatePayoutOwner(t, env.pool, "Affiliator Appeal Ok "+uniqueHandlerSuffix())
	decider := seedAffiliatePayoutDecider(t, env.pool, "Admin Appeal Ok "+uniqueHandlerSuffix())
	violationID := seedAffiliatePayoutViolation(t, env.pool, owner.affID, decider, "resolved", "none", time.Now().UTC())
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	body := `{"text":"Saya keberatan, bukan pembelian sendiri."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/affiliates/me/violations/"+violationID.String()+"/appeal", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST appeal: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		AppealStatus string `json:"appeal_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal appeal: %v", err)
	}
	if updated.AppealStatus != "pending" {
		t.Errorf("appeal_status = %q, want pending", updated.AppealStatus)
	}

	var persisted string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT appeal_status FROM affiliate_violations WHERE id = $1`, violationID,
	).Scan(&persisted); err != nil {
		t.Fatalf("read violation: %v", err)
	}
	if persisted != "pending" {
		t.Errorf("persisted appeal_status = %q, want pending", persisted)
	}
}
