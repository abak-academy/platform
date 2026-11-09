package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	partnerOnlyJSON = `{"affiliate": false, "partner": {"school_ids": []}}`
	bothProgramJSON = `{"affiliate": true, "partner": {"school_ids": []}}`
)

// seedProgramAccount creates an affiliator user and its profile. An empty
// programs value leaves the column NULL: a plain Affiliator account.
func seedProgramAccount(t *testing.T, pool *pgxpool.Pool, userStatus, programStatus, code, programs, reason string) string {
	t.Helper()
	suffix := uniqueHandlerSuffix()
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled) VALUES ($1, $2, 'affiliator', $3, false) RETURNING id`,
		"Program "+suffix, "gate_"+suffix, userStatus,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var programsValue, codeValue, reasonValue any
	if programs != "" {
		programsValue = programs
	}
	if code != "" {
		codeValue = code
	}
	if reason != "" {
		reasonValue = reason
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO affiliators (user_id, referral_code, program_status, rejection_reason, terms_version, programs)
		 VALUES ($1, $2, $3, $4, 'v1', $5::jsonb)`,
		userID, codeValue, programStatus, reasonValue, programsValue); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	return userID
}

func (env *affiliateJobsEnv) tokenForUser(t *testing.T, userID, role string) string {
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

func (env *affiliateJobsEnv) setPartnerFlag(t *testing.T, value string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('partner_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`, value); err != nil {
		t.Fatalf("set partner flag: %v", err)
	}
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestPartnerGate_letsOnlyAnApprovedMitraIn(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")

	tests := []struct {
		name       string
		userStatus string
		status     string
		programs   string
		reason     string
		wantStatus int
		wantCode   string
	}{
		{"a plain Affiliator is not enrolled", "active", "active", "", "", http.StatusForbidden, "program_not_enrolled"},
		{"a pending Mitra waits for approval", "active", "pending_approval", partnerOnlyJSON, "", http.StatusForbidden, "program_pending"},
		{"a rejected Mitra sees the reason", "active", "rejected", partnerOnlyJSON, "dokumen kurang", http.StatusForbidden, "program_rejected"},
		{"an approved Mitra is in", "active", "active", partnerOnlyJSON, "", http.StatusOK, ""},
		{"an account in both programs is in", "active", "active", bothProgramJSON, "", http.StatusOK, ""},
		{"a suspended Mitra can still read", "deactivated", "suspended", partnerOnlyJSON, "", http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := seedProgramAccount(t, env.pool, tt.userStatus, tt.status, "", tt.programs, tt.reason)
			token := env.tokenForUser(t, userID, service.RoleAffiliator)

			rec := getWithToken(t, env.e, "/api/v1/partners/me", token)
			if rec.Code != tt.wantStatus {
				t.Fatalf("GET /partners/me: want %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if tt.wantCode != "" && body["code"] != tt.wantCode {
				t.Errorf("code = %v, want %s", body["code"], tt.wantCode)
			}
			if tt.reason != "" && body["reason"] != tt.reason {
				t.Errorf("reason = %v, want %q", body["reason"], tt.reason)
			}
		})
	}

	t.Run("a student and a missing token never get in", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/partners/me", env.tokenFor(t, service.RoleStudent))
		if rec.Code != http.StatusForbidden {
			t.Errorf("student: want 403, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = getWithToken(t, env.e, "/api/v1/partners/me", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("no token: want 401, got %d", rec.Code)
		}
	})
}

func TestPartnerGate_programSwitchOffAndDeactivationKeepReadsButStopWrites(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	active := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "", partnerOnlyJSON, "")

	write := func(userID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/partners/me", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.tokenForUser(t, userID, service.RoleAffiliator))
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		return rec
	}

	env.setPartnerFlag(t, "false")
	rec := write(active)
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "partner_program_off" {
		t.Errorf("write with the program off: want 403 partner_program_off, got %d %s", rec.Code, rec.Body.String())
	}
	rec = getWithToken(t, env.e, "/api/v1/partners/me", env.tokenForUser(t, active, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Fatalf("read with the program off: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec)["program_enabled"] != false {
		t.Errorf("program_enabled = %v, want false so the portal can show the banner", decodeBody(t, rec)["program_enabled"])
	}

	env.setPartnerFlag(t, "true")
	rec = write(suspended)
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "account_deactivated" {
		t.Errorf("write by a suspended account: want 403 account_deactivated, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAffiliateGate_keepsAMitraOnlyAccountOutOfTheAffiliatorRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)

	partnerOnly := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	rec := getWithToken(t, env.e, "/api/v1/affiliates/me", env.tokenForUser(t, partnerOnly, service.RoleAffiliator))
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "program_not_enrolled" {
		t.Errorf("Mitra-only on /affiliates/me: want 403 program_not_enrolled, got %d %s", rec.Code, rec.Body.String())
	}

	plain := seedProgramAccount(t, env.pool, "active", "active", "AFG"+uniqueHandlerSuffix()[:5], "", "")
	rec = getWithToken(t, env.e, "/api/v1/affiliates/me", env.tokenForUser(t, plain, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Errorf("plain Affiliator on /affiliates/me: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	pending := seedProgramAccount(t, env.pool, "active", "pending_approval", "", "", "")
	rec = getWithToken(t, env.e, "/api/v1/affiliates/me", env.tokenForUser(t, pending, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Errorf("a pending Affiliator still sees its own status: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "AFH"+uniqueHandlerSuffix()[:5], "", "")
	token := env.tokenForUser(t, suspended, service.RoleAffiliator)
	if rec := getWithToken(t, env.e, "/api/v1/affiliates/me", token); rec.Code != http.StatusOK {
		t.Errorf("a suspended Affiliator can still read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSONWithToken(t, env.e, "/api/v1/affiliates/me/payouts", token, `{}`)
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "account_deactivated" {
		t.Errorf("a suspended Affiliator cannot write: want 403 account_deactivated, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOrderRoutesAreClosedToAffiliators(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	userID := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	token := env.tokenForUser(t, userID, service.RoleAffiliator)

	for _, path := range []string{"/api/v1/orders", "/api/v1/orders/" + uuid.NewString()} {
		if rec := getWithToken(t, env.e, path, token); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s: want 403, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
	rec := postJSONWithToken(t, env.e, "/api/v1/orders/"+uuid.NewString()+"/checkout", token, `{}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("checkout: want 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMe_carriesTheProgramStatesOfAnAffiliator(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	userID := seedProgramAccount(t, env.pool, "active", "pending_approval", "", partnerOnlyJSON, "")
	rec := getWithToken(t, env.e, "/api/v1/auth/me", env.tokenForUser(t, userID, service.RoleAffiliator))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /auth/me: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	states, ok := decodeBody(t, rec)["affiliator_programs"].(map[string]any)
	if !ok {
		t.Fatalf("affiliator_programs missing: %s", rec.Body.String())
	}
	if states["partner"] != "pending" || states["affiliate"] != "not_enrolled" || states["account_active"] != true {
		t.Errorf("affiliator_programs = %v, want partner pending, affiliate not_enrolled, account_active true", states)
	}
}

func TestPublicStatus_reportsBothSwitches(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	rec := getWithToken(t, env.e, "/api/v1/affiliates/public/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["partner_enabled"] != true {
		t.Errorf("partner_enabled = %v, want true", body["partner_enabled"])
	}
	if _, ok := body["enabled"]; !ok {
		t.Errorf("enabled is missing; older clients still read it: %s", rec.Body.String())
	}
}

func TestAdminPartnerProgramRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	admin := env.tokenFor(t, service.RoleSuperAdmin)
	other := env.tokenFor(t, service.RoleAdminStore)

	plain := seedProgramAccount(t, env.pool, "active", "active", "AFI"+uniqueHandlerSuffix()[:5], "", "")
	mitra := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")

	t.Run("only a super admin can add a program", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliates/"+plain+"/programs", other, `{"program":"partner"}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("want 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a super admin adds Mitra to an Affiliator", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliates/"+plain+"/programs", admin, `{"program":"partner"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		programs, _ := decodeBody(t, rec)["programs"].(map[string]any)
		if programs["partner"] == nil || programs["affiliate"] != true {
			t.Errorf("programs = %v, want affiliate true and a partner profile", programs)
		}
	})

	t.Run("an unknown program is a bad request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, "/api/v1/admin/affiliates/"+plain+"/programs", admin, `{"program":"reseller"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the list filters by program", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/admin/affiliates?program=partner&per_page=50", admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), mitra) {
			t.Errorf("the Mitra-only account is missing from the partner list: %s", rec.Body.String())
		}
		rec = getWithToken(t, env.e, "/api/v1/admin/affiliates?program=reseller", admin)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("an unknown program filter: want 400, got %d", rec.Code)
		}
	})

	t.Run("the schools are validated and replaced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/affiliates/"+mitra+"/partner/schools", strings.NewReader(`{"school_ids":["`+uuid.NewString()+`"]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+admin)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("an unknown school: want 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the Mitra switch is its own switch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/partner-settings/program", strings.NewReader(`{"enabled":true}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+admin)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !env.svc.PartnerProgramEnabled(context.Background()) {
			t.Error("partner_enabled was not stored")
		}
		if env.svc.AffiliateProgramEnabled(context.Background()) {
			t.Error("switching the Mitra program must not switch the Affiliator program")
		}
	})
}
