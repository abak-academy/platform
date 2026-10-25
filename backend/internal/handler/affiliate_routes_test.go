package handler_test

import (
	"net/http"
	"testing"

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
