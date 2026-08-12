package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"akademi-bimbel/internal/service"
)

func TestExamDashboardAllowsAdminExam(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleAdminExam)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/exam", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"active_sessions", "upcoming_exams", "counts", "recent_violations"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing %q", key)
		}
	}

	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object: %T", body["counts"])
	}
	for _, key := range []string{"questions", "tests", "exams", "courses"} {
		if _, ok := counts[key]; !ok {
			t.Errorf("counts missing %q", key)
		}
	}
}

func TestExamDashboardAllowsSuperAdmin(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/exam", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestExamDashboardRejectsRolesWithoutSessions(t *testing.T) {
	for _, role := range []string{service.RoleAdminStore, service.RoleAdminSchool} {
		t.Run(role, func(t *testing.T) {
			env := newAdminDashboardTestEnv(t)
			token := env.tokenFor(t, role)

			rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/exam", token)

			if rec.Code != http.StatusForbidden {
				t.Errorf("got %d, want 403", rec.Code)
			}
		})
	}
}

func TestExamDashboardRouteDoesNotWidenTheSuperAdminDashboard(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleAdminExam)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard", token)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403 — admin_exam has no revenue:read", rec.Code)
	}
}

func TestSchoolDashboardAllowsAdminSchool(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleAdminSchool)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/school", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{
		"counts", "orderable_exam_count", "latest_bulk_order", "recent_results",
	} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing %q", key)
		}
	}

	// An admin who has never placed a bulk order gets an explicit null, not a
	// zero-valued object the page would render as a real order.
	if body["latest_bulk_order"] != nil {
		t.Errorf("latest_bulk_order = %v, want null for a school with no orders", body["latest_bulk_order"])
	}
}

func TestSchoolDashboardRejectsRolesWithoutStudents(t *testing.T) {
	for _, role := range []string{service.RoleAdminStore, service.RoleAdminExam} {
		t.Run(role, func(t *testing.T) {
			env := newAdminDashboardTestEnv(t)
			token := env.tokenFor(t, role)

			rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/school", token)

			if rec.Code != http.StatusForbidden {
				t.Errorf("got %d, want 403", rec.Code)
			}
		})
	}
}

func TestSchoolDashboardAllowsSuperAdmin(t *testing.T) {
	env := newAdminDashboardTestEnv(t)
	token := env.tokenFor(t, service.RoleSuperAdmin)

	rec := getWithToken(t, env.e, "/api/v1/admin/dashboard/school", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}
}
