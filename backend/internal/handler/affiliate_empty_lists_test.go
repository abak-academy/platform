package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

// A Go nil slice marshals to null, and the web tables call rows.length on it, so
// every list an affiliator or admin can open must answer [] when it is empty.
func assertDataIsArray(t *testing.T, name, body string, code int) {
	t.Helper()
	if code != http.StatusOK {
		t.Errorf("%s: want 200, got %d: %s", name, code, body)
		return
	}
	var parsed struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Errorf("%s: decode: %v", name, err)
		return
	}
	if string(parsed.Data) == "null" || len(parsed.Data) == 0 {
		t.Errorf("%s: data must be [] when empty, got %q in %s", name, parsed.Data, body)
	}
}

func TestEmptyListsAnswerAnEmptyArray(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	ctx := context.Background()
	setAffiliateFlag(t, env.pool, "true")

	affID, _ := seedAffiliateAdminRow(t, env.pool, "active")
	var userID, adminID string
	if err := env.pool.QueryRow(ctx, `SELECT user_id FROM affiliators WHERE id = $1`, affID).Scan(&userID); err != nil {
		t.Fatalf("affiliator user: %v", err)
	}
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ('Empty Lists Admin', $1, 'super_admin', 'active', false) RETURNING id`,
		"empty_admin_"+uniqueHandlerSuffix(),
	).Scan(&adminID); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	token := func(sub, role string) string {
		tok, jti, err := env.signer.SignAccess(sub, role, nil, []string{})
		if err != nil {
			t.Fatalf("SignAccess: %v", err)
		}
		if err := env.rdb.Set(ctx, "session:access:"+jti, "1", time.Hour).Err(); err != nil {
			t.Fatalf("seed session: %v", err)
		}
		return tok
	}
	affToken := token(userID, service.RoleAffiliator)
	adminToken := token(adminID, service.RoleSuperAdmin)

	for _, path := range []string{"bindings", "commissions", "ledger", "payouts", "adjustments"} {
		rec := getWithToken(t, env.e, "/api/v1/affiliates/me/"+path, affToken)
		assertDataIsArray(t, "affiliator "+path, rec.Body.String(), rec.Code)
	}

	adminPaths := []string{
		"/api/v1/admin/affiliates?q=tidak-ada-yang-cocok",
		"/api/v1/admin/affiliates/" + affID + "/adjustments",
		"/api/v1/admin/affiliate-payouts",
		"/api/v1/admin/affiliate-product-overrides",
	}
	for _, path := range adminPaths {
		rec := getWithToken(t, env.e, path, adminToken)
		assertDataIsArray(t, "admin "+path, rec.Body.String(), rec.Code)
	}

	rec := getWithToken(t, env.e, "/api/v1/admin/affiliate-reports/summary?from=2000-01-01&to=2000-01-02", adminToken)
	if rec.Code != http.StatusOK {
		t.Errorf("report summary: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAffiliateMeListQueryParams(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	ctx := context.Background()
	setAffiliateFlag(t, env.pool, "true")
	affID, _ := seedAffiliateAdminRow(t, env.pool, "active")
	var userID string
	if err := env.pool.QueryRow(ctx, `SELECT user_id FROM affiliators WHERE id = $1`, affID).Scan(&userID); err != nil {
		t.Fatalf("affiliator user: %v", err)
	}
	tok, jti, err := env.signer.SignAccess(userID, service.RoleAffiliator, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(ctx, "session:access:"+jti, "1", time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	cases := []struct {
		path string
		want int
	}{
		{"ledger?type=commission&q=sari&from=2026-01-01&to=2026-12-31", http.StatusOK},
		{"ledger?type=bogus", http.StatusBadRequest},
		{"commissions?q=x&status=held", http.StatusOK},
		{"commissions?status=bogus", http.StatusBadRequest},
		{"bindings?type=link&q=budi", http.StatusOK},
		{"bindings?type=bogus", http.StatusBadRequest},
		{"payouts?status=paid", http.StatusOK},
		{"payouts?status=bogus", http.StatusBadRequest},
		{"adjustments?type=bonus", http.StatusOK},
		{"adjustments?type=bogus", http.StatusBadRequest},
		{"ledger?from=not-a-date", http.StatusBadRequest},
	}
	for _, tc := range cases {
		rec := getWithToken(t, env.e, "/api/v1/affiliates/me/"+tc.path, tok)
		if rec.Code != tc.want {
			t.Errorf("GET %s: want %d, got %d: %s", tc.path, tc.want, rec.Code, rec.Body.String())
		}
	}
}
