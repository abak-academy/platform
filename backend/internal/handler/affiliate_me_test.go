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

// affiliateMeEnv wires the /affiliates/me group with the real JWT and
// AffiliatorOnlyMiddleware so role gating and privacy scoping run for real.
type affiliateMeEnv struct {
	pool   *pgxpool.Pool
	e      *echo.Echo
	signer *infra.JWTSigner
	rdb    *redis.Client
}

func newAffiliateMeEnv(t *testing.T) *affiliateMeEnv {
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
		JWTSecret:       "affiliate-me-test-secret",
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
	me.GET("", h.AffiliateMe)
	me.PATCH("/bank", h.AffiliateMeUpdateBank)
	me.GET("/bindings", h.AffiliateMeBindings)
	me.GET("/commissions", h.AffiliateMeCommissions)
	me.GET("/ledger", h.AffiliateMeLedger)
	me.GET("/stats", h.AffiliateMeStats)

	t.Cleanup(pool.Close)
	return &affiliateMeEnv{pool: pool, e: e, signer: signer, rdb: rdb}
}

// tokenFor mints an access token for a real user id and seeds its session so
// JWTMiddleware's SessionActive check passes.
func (env *affiliateMeEnv) tokenFor(t *testing.T, userID, role string) string {
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

type affiliateMeOwner struct {
	userID string
	affID  uuid.UUID
	code   string
}

func seedAffiliateMeOwner(t *testing.T, pool *pgxpool.Pool, name string) affiliateMeOwner {
	t.Helper()
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affme_"+uniqueHandlerSuffix(),
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

func seedAffiliateMeStudent(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'student', 'active', false) RETURNING id`,
		name, "stdme_"+uniqueHandlerSuffix(),
	).Scan(&id); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	return id
}

func seedAffiliateMeBinding(t *testing.T, pool *pgxpool.Pool, affiliatorID, studentID uuid.UUID) time.Time {
	t.Helper()
	startedAt := time.Now().UTC().AddDate(0, 0, -10)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, ended_at)
		 VALUES ($1, $2, 'active', 'link', $3, $4)`,
		affiliatorID, studentID, startedAt, startedAt.AddDate(0, 3, 0),
	); err != nil {
		t.Fatalf("seed binding: %v", err)
	}
	return startedAt
}

func seedAffiliateMePaidOrder(t *testing.T, pool *pgxpool.Pool, studentID string, subtotal, discount float64) {
	t.Helper()
	paidAt := time.Now().UTC().AddDate(0, 0, -5)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, paid_at)
		 VALUES ($1, 'paid', $2, $3, 0, $4, $5)`,
		studentID, subtotal, discount, subtotal-discount, paidAt,
	); err != nil {
		t.Fatalf("seed order: %v", err)
	}
}

func TestAffiliateMeHandler_affiliatorSeesOwnProfile(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliateMeOwner(t, env.pool, "Affiliator Profile "+uniqueHandlerSuffix())

	rec := getWithToken(t, env.e, "/api/v1/affiliates/me", env.tokenFor(t, owner.userID, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /affiliates/me: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code         string  `json:"code"`
		ReferralLink string  `json:"referral_link"`
		Level        string  `json:"level"`
		Status       string  `json:"status"`
		BankName     *string `json:"bank_name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	if body.Code != owner.code {
		t.Errorf("code = %q, want %q", body.Code, owner.code)
	}
	if body.ReferralLink != "/r/"+owner.code {
		t.Errorf("referral_link = %q, want %q", body.ReferralLink, "/r/"+owner.code)
	}
	if body.Status != "active" {
		t.Errorf("status = %q, want active", body.Status)
	}
	if body.BankName != nil {
		t.Errorf("bank_name = %v, want nil for a fresh profile", *body.BankName)
	}
}

func TestAffiliateMeHandler_rejectsNonAffiliatorRoles(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	seedAffiliateMeOwner(t, env.pool, "Affiliator Gate "+uniqueHandlerSuffix())

	tests := []struct {
		name  string
		role  string
		token bool
		want  int
	}{
		{"student token is forbidden", service.RoleStudent, true, http.StatusForbidden},
		{"super_admin token is forbidden", service.RoleSuperAdmin, true, http.StatusForbidden},
		{"admin_school token is forbidden", service.RoleAdminSchool, true, http.StatusForbidden},
		{"missing token is unauthorized", "", false, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := ""
			if tt.token {
				token = env.tokenFor(t, uuid.NewString(), tt.role)
			}
			rec := getWithToken(t, env.e, "/api/v1/affiliates/me", token)
			if rec.Code != tt.want {
				t.Errorf("GET /affiliates/me as %q: want %d, got %d: %s", tt.role, tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAffiliateMeHandler_noLeakBetweenAffiliators(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	ownerA := seedAffiliateMeOwner(t, env.pool, "Affiliator A "+uniqueHandlerSuffix())
	ownerB := seedAffiliateMeOwner(t, env.pool, "Affiliator B "+uniqueHandlerSuffix())

	studentA := seedAffiliateMeStudent(t, env.pool, "Budi Santoso")
	seedAffiliateMeBinding(t, env.pool, ownerA.affID, uuid.MustParse(studentA))
	seedAffiliateMePaidOrder(t, env.pool, studentA, 100000, 10000)

	studentB := seedAffiliateMeStudent(t, env.pool, "Citra Melati")
	seedAffiliateMeBinding(t, env.pool, ownerB.affID, uuid.MustParse(studentB))
	seedAffiliateMePaidOrder(t, env.pool, studentB, 500000, 0)

	tokenA := env.tokenFor(t, ownerA.userID, service.RoleAffiliator)
	tokenB := env.tokenFor(t, ownerB.userID, service.RoleAffiliator)

	recA := getWithToken(t, env.e, "/api/v1/affiliates/me/bindings", tokenA)
	if recA.Code != http.StatusOK {
		t.Fatalf("GET bindings as A: want 200, got %d: %s", recA.Code, recA.Body.String())
	}
	var bindingsA struct {
		Data []struct {
			Name       string  `json:"name"`
			TotalSpend float64 `json:"total_spend"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(recA.Body.Bytes(), &bindingsA); err != nil {
		t.Fatalf("unmarshal bindings A: %v", err)
	}
	if bindingsA.Total != 1 || len(bindingsA.Data) != 1 {
		t.Fatalf("bindings for A: total=%d len=%d, want 1", bindingsA.Total, len(bindingsA.Data))
	}
	if bindingsA.Data[0].Name != "Budi S." {
		t.Errorf("name = %q, want privacy-formatted %q", bindingsA.Data[0].Name, "Budi S.")
	}
	if bindingsA.Data[0].TotalSpend != 90000 {
		t.Errorf("total_spend = %v, want 90000", bindingsA.Data[0].TotalSpend)
	}

	privacyKeys := map[string]bool{
		"email": true, "phone": true, "address": true, "dob": true, "grade": true, "alamat_domisili": true,
	}
	checkPrivacy := func(label string, body []byte) {
		t.Helper()
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("%s: unmarshal body: %v", label, err)
		}
		var walk func(m map[string]any, path string)
		walk = func(m map[string]any, path string) {
			for key, value := range m {
				if privacyKeys[strings.ToLower(key)] {
					t.Errorf("%s: forbidden PII key %q at %s", label, key, path)
				}
				if nested, ok := value.(map[string]any); ok {
					walk(nested, path+"."+key)
				}
			}
		}
		walk(payload, "")
	}
	checkPrivacy("bindings A", recA.Body.Bytes())

	recB := getWithToken(t, env.e, "/api/v1/affiliates/me/bindings", tokenB)
	if recB.Code != http.StatusOK {
		t.Fatalf("GET bindings as B: want 200, got %d: %s", recB.Code, recB.Body.String())
	}
	var bindingsB struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recB.Body.Bytes(), &bindingsB); err != nil {
		t.Fatalf("unmarshal bindings B: %v", err)
	}
	if len(bindingsB.Data) != 1 || bindingsB.Data[0]["name"] != "Citra M." {
		t.Fatalf("bindings for B = %v, want only B's own student", bindingsB.Data)
	}
	checkPrivacy("bindings B", recB.Body.Bytes())

	statsA := getWithToken(t, env.e, "/api/v1/affiliates/me/stats", tokenA)
	if statsA.Code != http.StatusOK {
		t.Fatalf("GET stats as A: want 200, got %d: %s", statsA.Code, statsA.Body.String())
	}
	var stats struct {
		Registrations  int `json:"registrations"`
		ActiveBindings int `json:"active_bindings"`
		TotalStudents  int `json:"total_students"`
	}
	if err := json.Unmarshal(statsA.Body.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshal stats A: %v", err)
	}
	if stats.Registrations != 1 || stats.ActiveBindings != 1 || stats.TotalStudents != 1 {
		t.Errorf("stats for A = %+v, want single-student counts", stats)
	}

	commissionsA := getWithToken(t, env.e, "/api/v1/affiliates/me/commissions", tokenA)
	if commissionsA.Code != http.StatusOK {
		t.Fatalf("GET commissions as A: want 200, got %d: %s", commissionsA.Code, commissionsA.Body.String())
	}
	var commissions struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(commissionsA.Body.Bytes(), &commissions); err != nil {
		t.Fatalf("unmarshal commissions A: %v", err)
	}
	if commissions.Total != 0 {
		t.Errorf("commissions for A = %d, want 0 (no commissions seeded for A)", commissions.Total)
	}
}

func TestAffiliateMeBindingsHandler_privacyPayloadShape(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliateMeOwner(t, env.pool, "Affiliator Shape "+uniqueHandlerSuffix())
	student := seedAffiliateMeStudent(t, env.pool, "Budi Santoso Putra")
	seedAffiliateMeBinding(t, env.pool, owner.affID, uuid.MustParse(student))
	seedAffiliateMePaidOrder(t, env.pool, student, 250000, 50000)

	rec := getWithToken(t, env.e, "/api/v1/affiliates/me/bindings", env.tokenFor(t, owner.userID, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET bindings: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data    []map[string]any `json:"data"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
		Total   int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("data = %v, want 1 binding", body.Data)
	}
	item := body.Data[0]
	if item["name"] != "Budi S. P." {
		t.Errorf("name = %v, want Budi S. P.", item["name"])
	}
	for _, key := range []string{"email", "phone", "address", "student_email", "student_phone"} {
		if _, ok := item[key]; ok {
			t.Errorf("binding payload has forbidden key %q", key)
		}
	}
	for _, key := range []string{"binding_id", "name", "registered_at", "status", "source", "started_at", "ended_at", "total_spend"} {
		if _, ok := item[key]; !ok {
			t.Errorf("binding payload missing key %q", key)
		}
	}
	if _, ok := item["student_id"]; ok {
		t.Errorf("binding payload exposes student_id; name and dates are the agreed identity")
	}
	if body.PerPage != 20 || body.Page != 1 || body.Total != 1 {
		t.Errorf("page meta = %+v, want page 1 per_page 20 total 1", body)
	}
}

func TestAffiliateMeUpdateBankHandler(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliateMeOwner(t, env.pool, "Affiliator Bank "+uniqueHandlerSuffix())
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/affiliates/me/bank",
		strings.NewReader(`{"bank_name":"BCA","bank_account_number":"1234567890","bank_account_holder":"Budi Santoso"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH bank: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var bank struct {
		BankName          string `json:"bank_name"`
		BankAccountNumber string `json:"bank_account_number"`
		BankAccountHolder string `json:"bank_account_holder"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bank); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bank.BankName != "BCA" || bank.BankAccountNumber != "1234567890" || bank.BankAccountHolder != "Budi Santoso" {
		t.Errorf("bank response = %+v", bank)
	}

	var persisted [3]string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT bank_name, bank_account_number, bank_account_holder FROM affiliators WHERE id = $1`,
		owner.affID,
	).Scan(&persisted[0], &persisted[1], &persisted[2]); err != nil {
		t.Fatalf("read bank: %v", err)
	}
	for i, want := range []string{"BCA", "1234567890", "Budi Santoso"} {
		if persisted[i] != want {
			t.Errorf("persisted bank field %d = %q, want %q", i, persisted[i], want)
		}
	}

	var audits int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = 'affiliate.bank_update' AND target_id = $1 AND actor_id = $2`,
		owner.affID.String(), owner.userID,
	).Scan(&audits); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if audits != 1 {
		t.Errorf("audit rows = %d, want 1", audits)
	}

	missing := httptest.NewRequest(http.MethodPatch, "/api/v1/affiliates/me/bank",
		strings.NewReader(`{"bank_name":"BCA","bank_account_number":"1234567890"}`))
	missing.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	missing.Header.Set("Authorization", "Bearer "+token)
	recMissing := httptest.NewRecorder()
	env.e.ServeHTTP(recMissing, missing)
	if recMissing.Code != http.StatusBadRequest {
		t.Errorf("PATCH bank missing holder: want 400, got %d: %s", recMissing.Code, recMissing.Body.String())
	}
	var errBody map[string]any
	if err := json.Unmarshal(recMissing.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if errBody["code"] != "invalid_request" {
		t.Errorf("error code = %v, want invalid_request", errBody["code"])
	}
}

func TestAffiliateMeListHandlers_rejectBadQueryParams(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliateMeOwner(t, env.pool, "Affiliator Query "+uniqueHandlerSuffix())
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	tests := []struct {
		name string
		path string
	}{
		{"page 0 is rejected", "/api/v1/affiliates/me/bindings?page=0"},
		{"non-numeric page is rejected", "/api/v1/affiliates/me/commissions?page=many"},
		{"bad date format is rejected", "/api/v1/affiliates/me/commissions?from=08-2026"},
		{"unknown product type is rejected", "/api/v1/affiliates/me/commissions?product_type=mystery"},
		{"unknown ledger order is rejected", "/api/v1/affiliates/me/ledger?order=sideways"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := getWithToken(t, env.e, tt.path, token)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s: want 400, got %d: %s", tt.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAffiliateMeListHandlers_clampPerPage(t *testing.T) {
	env := newAffiliateMeEnv(t)
	setAffiliateFlag(t, env.pool, "true")
	owner := seedAffiliateMeOwner(t, env.pool, "Affiliator Clamp "+uniqueHandlerSuffix())
	token := env.tokenFor(t, owner.userID, service.RoleAffiliator)

	rec := getWithToken(t, env.e, "/api/v1/affiliates/me/bindings?per_page=100", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET bindings: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		PerPage int `json:"per_page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.PerPage != 50 {
		t.Errorf("per_page = %d, want clamped to 50", body.PerPage)
	}
}
