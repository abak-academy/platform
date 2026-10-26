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

	for _, path := range []string{"bindings", "commissions", "ledger", "payouts", "adjustments", "violations"} {
		rec := getWithToken(t, env.e, "/api/v1/affiliates/me/"+path, affToken)
		assertDataIsArray(t, "affiliator "+path, rec.Body.String(), rec.Code)
	}

	adminPaths := []string{
		"/api/v1/admin/affiliates?q=tidak-ada-yang-cocok",
		"/api/v1/admin/affiliates/" + affID + "/adjustments",
		"/api/v1/admin/affiliate-payouts",
		"/api/v1/admin/affiliate-flags",
		"/api/v1/admin/affiliate-appeals",
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
