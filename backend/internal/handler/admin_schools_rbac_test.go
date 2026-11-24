package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"

	"github.com/labstack/echo/v4"
)

// newSchoolAdminRouter mirrors routes.go's adminSchoolsCreate + adminSchoolsWrite
// groups: POST "" accepts schools:create (Feature 2), while PUT/PATCH stay
// schools:write-only.
func newSchoolAdminRouter(t *testing.T, h *handler.Handler) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true

	create := e.Group("/admin/schools")
	create.Use(handler.RBACMiddleware("schools:create"))
	create.POST("", h.AdminCreateSchool)

	write := e.Group("/admin/schools")
	write.Use(handler.RBACMiddleware("schools:write"))
	write.PUT("/:id", h.AdminUpdateSchool)
	write.PATCH("/:id", h.AdminChangeSchoolStatus)

	return e
}

// TestAdminSchoolRoutes_AdminExamAdminStore_CreateYesWriteNo proves Feature 2's
// split: admin_exam/admin_store pass RBAC on create (reaching the handler's own
// 400 for missing fields) but are still rejected on update/status-change.
func TestAdminSchoolRoutes_AdminExamAdminStore_CreateYesWriteNo(t *testing.T) {
	for _, role := range []string{"admin_exam", "admin_store"} {
		t.Run(role+"/create_past_rbac_400", func(t *testing.T) {
			env := newAdminSystemEnv(t)
			e := newSchoolAdminRouter(t, env.h)
			e.Use(claimsInjector(&infra.Claims{Sub: "u1", Role: role}))

			req := httptest.NewRequest(http.MethodPost, "/admin/schools", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400 (past RBAC, missing fields), got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(role+"/update_403", func(t *testing.T) {
			env := newAdminSystemEnv(t)
			e := newSchoolAdminRouter(t, env.h)
			e.Use(claimsInjector(&infra.Claims{Sub: "u1", Role: role}))

			req := httptest.NewRequest(http.MethodPut, "/admin/schools/s1", strings.NewReader(`{"name":"X"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(role+"/status_change_403", func(t *testing.T) {
			env := newAdminSystemEnv(t)
			e := newSchoolAdminRouter(t, env.h)
			e.Use(claimsInjector(&infra.Claims{Sub: "u1", Role: role}))

			req := httptest.NewRequest(http.MethodPatch, "/admin/schools/s1", strings.NewReader(`{"status":"deactivated"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminSchoolRoutes_AdminSchoolAndStudent_403Everywhere pins the unchanged
// side of Feature 2: roles with neither schools:create nor schools:write stay
// rejected on all three endpoints.
func TestAdminSchoolRoutes_AdminSchoolAndStudent_403Everywhere(t *testing.T) {
	schoolID := "11111111-1111-1111-1111-111111111111"
	roles := map[string]*infra.Claims{
		"admin_school": {Sub: "as1", Role: "admin_school", SchoolID: &schoolID},
		"student":      {Sub: "st1", Role: "student"},
	}
	targets := []struct {
		name, method, path, body string
	}{
		{"create", http.MethodPost, "/admin/schools", `{}`},
		{"update", http.MethodPut, "/admin/schools/s1", `{"name":"X"}`},
		{"status_change", http.MethodPatch, "/admin/schools/s1", `{"status":"deactivated"}`},
	}

	for role, claims := range roles {
		for _, target := range targets {
			t.Run(role+"/"+target.name, func(t *testing.T) {
				env := newAdminSystemEnv(t)
				e := newSchoolAdminRouter(t, env.h)
				e.Use(claimsInjector(claims))

				req := httptest.NewRequest(target.method, target.path, strings.NewReader(target.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// TestAdminSchoolRoutes_SuperAdmin_PassesRBACEverywhere proves super_admin is
// unaffected by the Feature 2 split: its "*" satisfies both schools:create and
// schools:write, so every route reaches the handler's own validation.
func TestAdminSchoolRoutes_SuperAdmin_PassesRBACEverywhere(t *testing.T) {
	targets := []struct {
		name, method, path, body string
	}{
		{"create", http.MethodPost, "/admin/schools", `{}`},
		{"update", http.MethodPut, "/admin/schools/s1", `{"name":" --- "}`},
		{"status_change", http.MethodPatch, "/admin/schools/s1", `{"status":"pending"}`},
	}

	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			env := newAdminSystemEnv(t)
			e := newSchoolAdminRouter(t, env.h)
			e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

			req := httptest.NewRequest(target.method, target.path, strings.NewReader(target.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400 (past RBAC), got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
