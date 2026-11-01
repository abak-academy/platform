package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

func TestAffiliateRouteGates(t *testing.T) {
	env := newAffiliateJobsEnv(t)

	cases := []struct {
		name  string
		path  string
		role  string
		token bool
		want  int
	}{
		{"me needs a token", "/api/v1/affiliates/me", "", false, http.StatusUnauthorized},
		{"me rejects student", "/api/v1/affiliates/me", service.RoleStudent, true, http.StatusForbidden},
		{"me rejects super_admin", "/api/v1/affiliates/me", service.RoleSuperAdmin, true, http.StatusForbidden},
		{"admin list rejects admin_store", "/api/v1/admin/affiliates", service.RoleAdminStore, true, http.StatusForbidden},
		{"admin list rejects affiliator", "/api/v1/admin/affiliates", service.RoleAffiliator, true, http.StatusForbidden},
		{"payout export rejects admin_exam", "/api/v1/admin/affiliate-payouts/export", service.RoleAdminExam, true, http.StatusForbidden},
		{"settings rejects admin_school", "/api/v1/admin/affiliate-settings", service.RoleAdminSchool, true, http.StatusForbidden},
		{"admin list allows super_admin", "/api/v1/admin/affiliates", service.RoleSuperAdmin, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := ""
			if tc.token {
				token = env.tokenFor(t, tc.role)
			}
			rec := getWithToken(t, env.e, tc.path, token)
			if rec.Code != tc.want {
				t.Errorf("GET %s as %q: want %d, got %d: %s", tc.path, tc.role, tc.want, rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("public validate needs no token", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/affiliates/public/validate?code=ABCDEFGH", "")
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound && rec.Body.Len() == 0 {
			t.Errorf("public validate must be routed without JWT, got %d", rec.Code)
		}
	})
}

func TestMintCartReportsAffiliateCodeEligibility(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	ctx := context.Background()

	var studentID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ('Cart Student', $1, 'student', 'active', false) RETURNING id`,
		"cart_"+uniqueHandlerSuffix(),
	).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	tok, jti, err := env.signer.SignAccess(studentID, service.RoleStudent, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(ctx, "session:access:"+jti, "1", time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	eligible := func() bool {
		rec := postJSONWithToken(t, env.e, "/api/v1/orders", tok, `{}`)
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("mint cart: %d %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		v, ok := body["affiliate_code_eligible"].(bool)
		if !ok {
			t.Fatalf("affiliate_code_eligible missing from %s", rec.Body.String())
		}
		if _, ok := body["id"]; !ok {
			t.Fatalf("order fields must stay flat next to the flag: %s", rec.Body.String())
		}
		return v
	}

	if eligible() {
		t.Error("flag off: want not eligible")
	}

	setAffiliateFlag(t, env.pool, "true")
	if _, err := env.pool.Exec(ctx,
		`INSERT INTO affiliate_settings (id, launch_date) VALUES (1, now() - interval '1 day')
		 ON CONFLICT (id) DO UPDATE SET launch_date = now() - interval '1 day'`); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE users SET created_at = now() WHERE id = $1`, studentID); err != nil {
		t.Fatalf("touch student: %v", err)
	}
	if !eligible() {
		t.Error("flag on, post-launch, unbound: want eligible")
	}
}
