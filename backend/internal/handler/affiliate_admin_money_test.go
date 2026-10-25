package handler_test

import (
	"context"
	"encoding/csv"
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

// affiliateAdminMoneyEnv is a private, migrated database behind a local router
// that registers only the admin money endpoints with the real JWTMiddleware +
// RBACMiddleware("affiliates:*") guards — routes.go wiring is owned by the
// parent task.
type affiliateAdminMoneyEnv struct {
	pool   *pgxpool.Pool
	e      *echo.Echo
	rdb    *redis.Client
	signer *infra.JWTSigner
}

func newAffiliateAdminMoneyEnv(t *testing.T) *affiliateAdminMoneyEnv {
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
		JWTSecret:       "affiliate-admin-money-secret",
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
	admin := e.Group("/admin", handler.JWTMiddleware(svc, signer))
	admin.POST("/affiliates/:id/adjustments", h.AdminCreateAffiliateAdjustment, handler.RBACMiddleware("affiliates:*"))
	admin.GET("/affiliates/:id/adjustments", h.AdminListAffiliateAdjustments, handler.RBACMiddleware("affiliates:*"))
	admin.GET("/affiliate-payouts", h.AdminListAffiliatePayouts, handler.RBACMiddleware("affiliates:*"))
	admin.GET("/affiliate-payouts/export", h.AdminAffiliatePayoutsExport, handler.RBACMiddleware("affiliates:*"))
	admin.POST("/affiliate-payouts/:id/decide", h.AdminDecideAffiliatePayout, handler.RBACMiddleware("affiliates:*"))
	admin.POST("/affiliate-payouts/:id/process", h.AdminProcessAffiliatePayout, handler.RBACMiddleware("affiliates:*"))

	t.Cleanup(pool.Close)
	return &affiliateAdminMoneyEnv{pool: pool, e: e, rdb: rdb, signer: signer}
}

// tokenFor mints a valid access token for a real seeded user (actor ids land
// in created_by/processed_by/actor_id FK columns) and seeds its session so
// JWTMiddleware's SessionActive check passes.
func (env *affiliateAdminMoneyEnv) tokenFor(t *testing.T, role string) string {
	t.Helper()
	var sub string
	if err := env.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, $3, 'active', false) RETURNING id`,
		"Admin "+uniqueHandlerSuffix(), "admin_"+uniqueHandlerSuffix(), role,
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

func seedAffiliateAdminMoneyAffiliator(t *testing.T, pool *pgxpool.Pool) (affiliatorID, userID uuid.UUID, code string) {
	t.Helper()
	name := "Affiliator Admin " + uniqueHandlerSuffix()
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affadm_"+uniqueHandlerSuffix(),
	).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	code = "AF" + uniqueHandlerSuffix()[:6]
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, 'active', 'v1') RETURNING id`,
		userID, code,
	).Scan(&affiliatorID); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return affiliatorID, userID, code
}

func seedAffiliateAdminMoneyPayout(t *testing.T, pool *pgxpool.Pool, affiliatorID uuid.UUID, status string, amount float64, holder string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO affiliate_payouts (affiliator_id, amount, status, bank_name, bank_account_number, bank_account_holder)
		 VALUES ($1, $2, $3, 'BCA', '1234567890', $4) RETURNING id`,
		affiliatorID, amount, status, holder,
	).Scan(&id); err != nil {
		t.Fatalf("seed payout: %v", err)
	}
	return id
}

func TestAffiliateAdminMoneyEndpoints_authGates(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	id := affiliatorID.String()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admin/affiliates/" + id + "/adjustments"},
		{http.MethodGet, "/admin/affiliates/" + id + "/adjustments"},
		{http.MethodGet, "/admin/affiliate-payouts"},
		{http.MethodGet, "/admin/affiliate-payouts/export"},
		{http.MethodPost, "/admin/affiliate-payouts/" + id + "/decide"},
		{http.MethodPost, "/admin/affiliate-payouts/" + id + "/process"},
	}
	for _, ep := range endpoints {
		t.Run("no token 401 "+ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("want 401, got %d", rec.Code)
			}
		})
		t.Run("affiliator role 403 "+ep.method+" "+ep.path, func(t *testing.T) {
			token := env.tokenFor(t, service.RoleAffiliator)
			req := httptest.NewRequest(ep.method, ep.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("want 403, got %d", rec.Code)
			}
		})
	}
}

func TestAffiliateAdminCreateAdjustmentHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliates/"+affiliatorID.String()+"/adjustments", token,
		`{"category":"correction","amount":-25000,"reason":"koreksi refund","note":"catatan"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var adj struct {
		ID             string  `json:"id"`
		Category       string  `json:"category"`
		Amount         float64 `json:"amount"`
		ApprovalStatus *string `json:"approval_status"`
		Note           *string `json:"note"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if adj.Category != "correction" || adj.Amount != -25000 {
		t.Errorf("body = %+v", adj)
	}
	if adj.ApprovalStatus != nil {
		t.Errorf("approval_status = %v, want nil", adj.ApprovalStatus)
	}
	if adj.Note == nil || *adj.Note != "catatan" {
		t.Errorf("note = %v, want catatan", adj.Note)
	}

	var ledgerN int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_ledger_entries WHERE adjustment_id = $1 AND entry_type = 'adjustment'`,
		adj.ID,
	).Scan(&ledgerN); err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledgerN != 1 {
		t.Errorf("ledger rows = %d, want 1", ledgerN)
	}
	var auditN int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = 'affiliate.adjustment' AND target_id = $1`,
		adj.ID,
	).Scan(&auditN); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditN != 1 {
		t.Errorf("audit rows = %d, want 1", auditN)
	}
}

func TestAffiliateAdminCreateAdjustmentHandler_invalidUUID_400(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliates/not-a-uuid/adjustments", token,
		`{"category":"bonus","amount":1000,"reason":"why"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

func TestAffiliateAdminListAdjustmentsHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	for i := 0; i < 2; i++ {
		rec := postJSONWithToken(t, env.e, "/admin/affiliates/"+affiliatorID.String()+"/adjustments", token,
			`{"category":"other","amount":1000,"reason":"history seed"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed adjustment %d: want 201, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := getRequest(t, env.e, "/admin/affiliates/"+affiliatorID.String()+"/adjustments?limit=1", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 2 {
		t.Errorf("total = %d, want 2", body.Total)
	}
	if len(body.Data) != 1 {
		t.Errorf("page rows = %d, want 1", len(body.Data))
	}
}

func TestAffiliateAdminListPayoutsHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 100000, "Holder Req")
	seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "paid", 200000, "Holder Paid")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getRequest(t, env.e, "/admin/affiliate-payouts", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []struct {
			Status         string `json:"status"`
			AffiliatorName string `json:"affiliator_name"`
			AffiliatorCode string `json:"affiliator_code"`
			BankName       string `json:"bank_name"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("total = %d, want 1 (requested only by default)", body.Total)
	}
	if len(body.Data) != 1 {
		t.Fatalf("rows = %d, want 1", len(body.Data))
	}
	if body.Data[0].Status != "requested" {
		t.Errorf("status = %q, want requested", body.Data[0].Status)
	}
	if body.Data[0].AffiliatorName == "" || body.Data[0].AffiliatorCode == "" || body.Data[0].BankName == "" {
		t.Errorf("row missing joins: %+v", body.Data[0])
	}

	rec = getRequest(t, env.e, "/admin/affiliate-payouts?status=paid", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("paid filter: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || len(body.Data) != 1 || body.Data[0].Status != "paid" {
		t.Errorf("paid filter body = %+v", body)
	}
}

func TestAffiliateAdminPayoutsExportHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 150000, "=EVILHOLDER")
	seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "paid", 250000, "ShouldNotAppear")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	req := httptest.NewRequest(http.MethodGet, "/admin/affiliate-payouts/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get(echo.HeaderContentDisposition); cd != `attachment; filename="affiliate-payouts.csv"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	records, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("not parseable CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + 1 queue row", len(records))
	}
	if records[1][4] != "'=EVILHOLDER" {
		t.Errorf("holder cell = %q, want injection-neutralised '=EVILHOLDER", records[1][4])
	}
}

func TestAffiliateAdminDecidePayoutHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	payoutID := seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 300000, "Decide Holder")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliate-payouts/"+payoutID.String()+"/decide", token,
		`{"decision":"paid","proof_url":"payment_proof/`+uuid.NewString()+`/proof.jpg"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Status   string  `json:"status"`
		ProofURL *string `json:"proof_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "paid" || updated.ProofURL == nil {
		t.Errorf("body = %+v, want paid with proof", updated)
	}
	var proof string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT proof_url FROM affiliate_payouts WHERE id = $1`, payoutID,
	).Scan(&proof); err != nil {
		t.Fatalf("read payout: %v", err)
	}
	if !strings.HasPrefix(proof, "payment_proof/") {
		t.Errorf("persisted proof_url = %q", proof)
	}
}

func TestAffiliateAdminDecidePayoutHandler_reject(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	payoutID := seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 200000, "Reject Holder")
	if _, err := env.pool.Exec(context.Background(),
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, payout_id) VALUES ($1, 'payout', -200000, $2)`,
		affiliatorID, payoutID,
	); err != nil {
		t.Fatalf("seed deduction: %v", err)
	}
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliate-payouts/"+payoutID.String()+"/decide", token,
		`{"decision":"reject","reason":"rekening tidak cocok"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Status       string  `json:"status"`
		RejectReason *string `json:"reject_reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "rejected" {
		t.Errorf("status = %q, want rejected", updated.Status)
	}
	var reversalN int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_ledger_entries WHERE payout_id = $1 AND entry_type = 'payout_reversal'`,
		payoutID,
	).Scan(&reversalN); err != nil {
		t.Fatalf("count reversal: %v", err)
	}
	if reversalN != 1 {
		t.Errorf("payout_reversal rows = %d, want 1", reversalN)
	}
}

func TestAffiliateAdminDecidePayoutHandler_paidWithoutProof_400(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	payoutID := seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 100000, "Holder")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliate-payouts/"+payoutID.String()+"/decide", token,
		`{"decision":"paid","proof_url":"payment_proof/../../etc/passwd"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAffiliateAdminProcessPayoutHandler(t *testing.T) {
	env := newAffiliateAdminMoneyEnv(t)
	affiliatorID, _, _ := seedAffiliateAdminMoneyAffiliator(t, env.pool)
	payoutID := seedAffiliateAdminMoneyPayout(t, env.pool, affiliatorID, "requested", 100000, "Process Holder")
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := postJSONWithToken(t, env.e, "/admin/affiliate-payouts/"+payoutID.String()+"/process", token, `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "processing" {
		t.Errorf("status = %q, want processing", updated.Status)
	}
}
