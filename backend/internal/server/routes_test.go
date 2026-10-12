package server

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

// TestRegisterRoutes_BulkExamOrderRoutesRegistered asserts that the 4
// bulk-exam-order routes are present in the production registerRoutes
// function. Blocker 1 from review_result.json: the routes exist on the
// Handler but were missing from the production routes.go — only registered
// in the test helper, masking the production gap.
func TestRegisterRoutes_BulkExamOrderRoutesRegistered(t *testing.T) {
	signer, svc, _ := newTestDeps(t)
	e := echo.New()
	e.HideBanner = true
	h := handler.New(svc)

	RegisterRoutesForTest(e, h, svc, signer)

	want := map[string]string{
		http.MethodGet:  "/api/v1/admin/bulk-exam-orders/exams",
		http.MethodPost: "/api/v1/admin/bulk-exam-orders",
	}
	optionalWant := []string{
		"/api/v1/admin/bulk-exam-orders/preview",
		"/api/v1/admin/bulk-exam-orders/:id/checkout",
	}

	registered := map[string]bool{}
	for _, r := range e.Routes() {
		registered[r.Method+"\x00"+r.Path] = true
	}

	for method, path := range want {
		key := method + "\x00" + path
		if !registered[key] {
			t.Errorf("route %s %s not registered", method, path)
		}
	}

	// For the parameterized routes, the path Echo stores is the pattern
	// (e.g. "/api/v1/admin/bulk-exam-orders/:id/checkout"). Collect all
	// registered paths under /admin/bulk-exam-orders and confirm the
	// expected set.
	gotPaths := map[string]bool{}
	for _, r := range e.Routes() {
		if r.Path == "/api/v1/admin/bulk-exam-orders/exams" ||
			r.Path == "/api/v1/admin/bulk-exam-orders/preview" ||
			r.Path == "/api/v1/admin/bulk-exam-orders/:id/checkout" {
			gotPaths[r.Path] = true
		}
	}

	missing := []string{}
	for _, p := range optionalWant {
		if !gotPaths[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("expected bulk-exam-order route(s) not registered: %v", missing)
	}
}

// TestRegisterRoutes_ExamGrantBulkRoutesRegistered asserts that the two
// exam-grant bulk routes (presign + enqueue) are present in the production
// registerRoutes function, under the existing super-admin-only
// /admin/exam-grants group.
func TestRegisterRoutes_ExamGrantBulkRoutesRegistered(t *testing.T) {
	signer, svc, _ := newTestDeps(t)
	e := echo.New()
	e.HideBanner = true
	h := handler.New(svc)

	RegisterRoutesForTest(e, h, svc, signer)

	want := map[string]string{
		http.MethodPost: "/api/v1/admin/exam-grants/bulk/presign",
	}
	registered := map[string]bool{}
	for _, r := range e.Routes() {
		registered[r.Method+"\x00"+r.Path] = true
	}
	for method, path := range want {
		key := method + "\x00" + path
		if !registered[key] {
			t.Errorf("route %s %s not registered", method, path)
		}
	}

	gotPaths := map[string]bool{}
	for _, r := range e.Routes() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/admin/exam-grants/bulk" {
			gotPaths[r.Path] = true
		}
	}
	if !gotPaths["/api/v1/admin/exam-grants/bulk"] {
		t.Errorf("route POST /api/v1/admin/exam-grants/bulk not registered")
	}
}

// TestRegisterRoutes_ExamRevokeRoutesRegistered asserts the three exam-revoke
// routes (manual + CSV presign + CSV enqueue) are present in the production
// registerRoutes function, under the super-admin-only /admin/exam-grants group.
func TestRegisterRoutes_ExamRevokeRoutesRegistered(t *testing.T) {
	signer, svc, _ := newTestDeps(t)
	e := echo.New()
	e.HideBanner = true
	h := handler.New(svc)

	RegisterRoutesForTest(e, h, svc, signer)

	want := []string{
		"/api/v1/admin/exam-grants/revoke",
		"/api/v1/admin/exam-grants/revoke/bulk/presign",
		"/api/v1/admin/exam-grants/revoke/bulk",
	}
	registered := map[string]bool{}
	for _, r := range e.Routes() {
		if r.Method == http.MethodPost {
			registered[r.Path] = true
		}
	}
	for _, path := range want {
		if !registered[path] {
			t.Errorf("route POST %s not registered", path)
		}
	}
}

func TestRegisterRoutes_ResultsWorkspaceRoutesRegistered(t *testing.T) {
	signer, svc, _ := newTestDeps(t)
	e := echo.New()
	e.HideBanner = true
	h := handler.New(svc)

	RegisterRoutesForTest(e, h, svc, signer)

	want := []string{
		"/api/v1/admin/exams/:id/results-workspace",
		"/api/v1/admin/exams/:id/results-workspace/:registration_id/attempts",
		"/api/v1/admin/exams/:id/results-workspace/sessions/:session_id",
	}
	registered := map[string]bool{}
	for _, r := range e.Routes() {
		if r.Method == http.MethodGet {
			registered[r.Path] = true
		}
	}
	for _, path := range want {
		if !registered[path] {
			t.Errorf("route GET %s not registered", path)
		}
	}
}

func TestRegisterRoutes_StudentPasswordGate(t *testing.T) {
	tests := []struct {
		role string
		want int
	}{
		{service.RoleStudent, http.StatusForbidden},
		{service.RoleAdminStore, http.StatusForbidden},
		{service.RoleAdminSchool, http.StatusForbidden},
		{service.RoleAdminExam, http.StatusBadRequest},
		{service.RoleSuperAdmin, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			signer, svc, mr := newTestDeps(t)
			e := echo.New()
			e.HideBanner = true
			RegisterRoutesForTest(e, handler.New(svc), svc, signer)

			token, jti, err := signer.SignAccess("user1", tt.role, nil, nil)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			mr.Set("session:access:"+jti, "user1")

			req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/00000000-0000-0000-0000-000000000001/password", strings.NewReader("{}"))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("PATCH password as %s: got %d, want %d (body %s)", tt.role, rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
