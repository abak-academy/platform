package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newTestDeps(t *testing.T) (*infra.JWTSigner, *service.Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	signer := infra.NewJWTSigner("test-secret", time.Hour)
	svc := service.NewForTest(rdb)
	return signer, svc, mr
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	signer, svc, _ := newTestDeps(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	err := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	signer, svc, _ := newTestDeps(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage.token.here")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	err := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_RevokedSession(t *testing.T) {
	signer, svc, _ := newTestDeps(t)

	tokenStr, _, err := signer.SignAccess("user1", service.RoleStudent, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	err = mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 (revoked), got %d", rec.Code)
	}
}

func TestJWTMiddleware_InsufficientRole(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("user1", service.RoleStudent, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "user1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("questions:read")

	chain := mw(rbac(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestJWTMiddleware_MustChangePasswordGate(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("user1", service.RoleStudent, nil, nil, true)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "user1")

	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	e := echo.New()
	e.GET("/api/v1/admin/students", ok, JWTMiddleware(svc, signer))
	e.GET("/api/v1/auth/me", ok, JWTMiddleware(svc, signer))
	e.PATCH("/api/v1/auth/password/change", ok, JWTMiddleware(svc, signer))
	e.POST("/api/v1/auth/logout", ok, JWTMiddleware(svc, signer))

	t.Run("blocks other routes with password_change_required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "password_change_required") {
			t.Errorf("want password_change_required code, got %s", rec.Body.String())
		}
	})

	t.Run("allows me, password change and logout", func(t *testing.T) {
		for _, tc := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/me"},
			{http.MethodPatch, "/api/v1/auth/password/change"},
			{http.MethodPost, "/api/v1/auth/logout"},
		} {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s %s: want 200, got %d", tc.method, tc.path, rec.Code)
			}
		}
	})

	t.Run("token without flag is unaffected", func(t *testing.T) {
		plainToken, plainJTI, err := signer.SignAccess("user2", service.RoleStudent, nil, nil)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		mr.Set("session:access:"+plainJTI, "user2")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students", nil)
		req.Header.Set("Authorization", "Bearer "+plainToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rec.Code)
		}
	})
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("user1", service.RoleAdminExam, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "user1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var gotClaims *infra.Claims
	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("questions:read")

	chain := mw(rbac(func(c echo.Context) error {
		gotClaims = ClaimsFromContext(c)
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if gotClaims == nil {
		t.Error("ClaimsFromContext returned nil, want non-nil claims")
	}
	if gotClaims != nil && gotClaims.Sub != "user1" {
		t.Errorf("claims.Sub = %q, want %q", gotClaims.Sub, "user1")
	}
}

// FR-35: a student JWT (no "uploads:write" capability) hitting an
// uploads:write-gated route is rejected 403, same as questions:*/tests:*.
func TestRBACMiddleware_UploadsWrite_StudentForbidden(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("student1", service.RoleStudent, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "student1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("uploads:write")

	chain := mw(rbac(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// FR-35 (positive case): admin_exam carries "uploads:write" (rbac.go's
// roleCapabilities map) and passes the same gate.
func TestRBACMiddleware_UploadsWrite_AdminExamAllowed(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("admin1", service.RoleAdminExam, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "admin1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("uploads:write")

	chain := mw(rbac(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// PR review P1: admin_school needs GET /admin/exams and GET /admin/exams/:id
// (the sibling read-only route group, gated on "products(exam):read") to use
// the Registrations tab on the exam detail page.
func TestRBACMiddleware_ProductsExamRead_AdminSchoolAllowed(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("school1", service.RoleAdminSchool, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "school1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("products(exam):read")

	chain := mw(rbac(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// PR review P1 (negative case): admin_school must NOT gain exam write access
// — only the scoped read capability was granted, matching the review's
// explicit instruction not to grant products(exam):write.
func TestRBACMiddleware_ProductsExamWrite_AdminSchoolForbidden(t *testing.T) {
	signer, svc, mr := newTestDeps(t)

	tokenStr, jti, err := signer.SignAccess("school1", service.RoleAdminSchool, nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	mr.Set("session:access:"+jti, "school1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := JWTMiddleware(svc, signer)
	rbac := RBACMiddleware("products(exam):write")

	chain := mw(rbac(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}))
	err = chain(c)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}
