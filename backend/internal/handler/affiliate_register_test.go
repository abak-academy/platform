package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"akademi-bimbel/internal/service"
)

func TestRegisterAffiliatorHandler(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	const path = "/api/v1/auth/affiliators/register"
	body := func(email, terms string) string {
		return `{"email":"` + email + `","password":"password123","name":"Budi Santoso","phone":"081234567890","terms_version":"` + terms + `"}`
	}

	t.Run("closed while the feature flag is off", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, path, "", body("closed@example.com", "v1"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	setAffiliateFlag(t, env.pool, "true")

	t.Run("creates a pending user and a codeless pending profile", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, path, "", body("new.aff@example.com", "v1"))
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			PendingToken string `json:"pending_token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.PendingToken == "" {
			t.Fatalf("want a pending token, got %s (%v)", rec.Body.String(), err)
		}
		var role, userStatus, phone, affStatus, terms string
		var code *string
		if err := env.pool.QueryRow(context.Background(),
			`SELECT u.role, u.status, u.phone, a.program_status, a.terms_version, a.referral_code
			 FROM users u JOIN affiliators a ON a.user_id = u.id WHERE u.email = 'new.aff@example.com'`,
		).Scan(&role, &userStatus, &phone, &affStatus, &terms, &code); err != nil {
			t.Fatalf("read back: %v", err)
		}
		if role != "affiliator" || userStatus != "pending_verification" || phone != "081234567890" ||
			affStatus != "pending_approval" || terms != "v1" || code != nil {
			t.Errorf("unexpected rows: %s %s %s %s %s %v", role, userStatus, phone, affStatus, terms, code)
		}
	})

	t.Run("a repeat while pending re-issues the challenge without a second profile", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, path, "", body("new.aff@example.com", "v1"))
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var n int
		if err := env.pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM affiliators a JOIN users u ON u.id = a.user_id WHERE u.email = 'new.aff@example.com'`).Scan(&n); err != nil || n != 1 {
			t.Errorf("profiles = %d (%v), want 1", n, err)
		}
	})

	t.Run("an email already used by another account is taken", func(t *testing.T) {
		if _, err := env.pool.Exec(context.Background(),
			`INSERT INTO users (name, username, email, role, status, otp_enabled)
			 VALUES ('Sekolah', 'sekolah_adm', 'school@example.com', 'admin_school', 'active', false)`); err != nil {
			t.Fatalf("seed: %v", err)
		}
		rec := postJSONWithToken(t, env.e, path, "", body("school@example.com", "v1"))
		if rec.Code != http.StatusConflict {
			t.Errorf("want 409, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing terms is a bad request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, path, "", body("noterms@example.com", ""))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the admin list shows the pending profile with an empty code", func(t *testing.T) {
		token := env.tokenFor(t, service.RoleSuperAdmin)
		rec := getWithToken(t, env.e, "/api/v1/admin/affiliates?status=pending_approval", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the daily cap answers 429", func(t *testing.T) {
		if _, err := env.pool.Exec(context.Background(),
			`INSERT INTO affiliate_settings (id, daily_registration_cap)
			 VALUES (1, (SELECT count(*) FROM users WHERE role = 'affiliator' AND created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'Asia/Jakarta'))
			 ON CONFLICT (id) DO UPDATE SET daily_registration_cap = EXCLUDED.daily_registration_cap`); err != nil {
			t.Fatalf("set cap: %v", err)
		}
		rec := postJSONWithToken(t, env.e, path, "", body("capped@example.com", "v1"))
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("want 429, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
