package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

func scopeSchoolPtr(s string) *string { return &s }

func newScopeRequest(t *testing.T, target string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestResolveStudentSchoolScope(t *testing.T) {
	tests := []struct {
		name       string
		claims     *infra.Claims
		target     string
		wantSchool string
		wantStatus int
	}{
		{
			name:   "admin_exam reads across every school",
			claims: &infra.Claims{Role: service.RoleAdminExam},
		},
		{
			name:       "admin_exam still validates an explicit school_id",
			claims:     &infra.Claims{Role: service.RoleAdminExam},
			target:     "/admin/students?school_id=not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "admin_school stays pinned to its JWT school",
			claims:     &infra.Claims{Role: service.RoleAdminSchool, SchoolID: scopeSchoolPtr("s1")},
			wantSchool: "s1",
		},
		{
			name:       "admin_school cannot widen its scope",
			claims:     &infra.Claims{Role: service.RoleAdminSchool, SchoolID: scopeSchoolPtr("s1")},
			target:     "/admin/students?school_id=s2",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "a role without a school is still refused",
			claims:     &infra.Claims{Role: service.RoleAdminStore},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(service.NewForTest(nil))
			target := tt.target
			if target == "" {
				target = "/admin/students"
			}
			c, rec := newScopeRequest(t, target)

			got, err := h.resolveStudentSchoolScope(c, tt.claims)
			if tt.wantStatus != 0 {
				if !scopeHandled(err) {
					t.Fatalf("resolveStudentSchoolScope() err = %v, want the written %d", err, tt.wantStatus)
				}
				if rec.Code != tt.wantStatus {
					t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveStudentSchoolScope() err = %v, want nil", err)
			}
			if got != tt.wantSchool {
				t.Errorf("schoolID = %q, want %q", got, tt.wantSchool)
			}
		})
	}
}

// resolveSchoolScope must keep denying admin_exam: it also guards school-bound routes.
func TestResolveSchoolScope_StillDeniesAdminExam(t *testing.T) {
	h := New(service.NewForTest(nil))
	c, rec := newScopeRequest(t, "/admin/dashboard/exam")

	if _, err := h.resolveSchoolScope(c, &infra.Claims{Role: service.RoleAdminExam}); !scopeHandled(err) {
		t.Fatalf("resolveSchoolScope(admin_exam) err = %v, want the written 403", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing school scope") {
		t.Errorf("body = %s, want missing school scope", rec.Body.String())
	}
}
