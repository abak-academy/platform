package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// courseTestEnv sets up Echo with full routes registered via server.RegisterRoutesForTest.
func newCourseTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)

	// Rebuild echo with full route set so course routes exist.
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, handler.New(env.svc), env.svc, env.signer)
	env.e = e
	return env
}

func adminStoreToken(t *testing.T, env *testEnv) string {
	t.Helper()
	env.repo.seed(&model.User{
		ID:     "admin_store_user",
		Email:  strptr("admin_store@example.com"),
		Role:   service.RoleAdminStore,
		Status: "active",
	})
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	tok, jti, _ := env.signer.SignAccess("admin_store_user", service.RoleAdminStore, nil, []string{})
	rdb.Set(context.Background(), "session:access:"+jti, "admin_store_user", 15*time.Minute)
	return tok
}

// FR3: GET /api/v1/admin/courses/:id route is registered (no panic on 401).
func TestAdminGetCourse_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/courses/some-id", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// FR5: DELETE /api/v1/admin/courses/:id route is registered (no panic on 401).
func TestAdminDeleteCourse_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/courses/some-id", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// FR19: GET /api/v1/courses requires auth (returns 401 without token).
func TestStudentListCourses_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// FR20: GET /api/v1/courses/:id requires auth.
func TestStudentGetCourse_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/courses/some-id", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// FR6: POST /admin/products with type=course and empty course_ids returns 422 with code="course_required".
func TestAdminCreateProduct_CourseType_EmptyCourseIDs_Returns422CourseRequired(t *testing.T) {
	env := newCourseTestEnv(t)
	tok := adminStoreToken(t, env)

	body := map[string]interface{}{
		"type":       "course",
		"name":       "Math Bundle",
		"price":      50000,
		"course_ids": []string{},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["code"] != "course_required" {
		t.Errorf("want code='course_required', got %v", resp["code"])
	}
}

// FR6: POST /admin/products with type=course and no course_ids key also returns 422.
func TestAdminCreateProduct_CourseType_NoCourseIDs_Returns422(t *testing.T) {
	env := newCourseTestEnv(t)
	tok := adminStoreToken(t, env)

	body := map[string]interface{}{
		"type":  "course",
		"name":  "Math Bundle",
		"price": 50000,
		// course_ids absent
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["code"] != "course_required" {
		t.Errorf("want code='course_required', got %v", resp["code"])
	}
}

// FR2: GET /api/v1/admin/courses route exists and requires auth.
func TestAdminListCourses_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/courses", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestAdminUpdateProduct_CourseIDs_Returns400(t *testing.T) {
	env := newCourseTestEnv(t)
	tok := adminStoreToken(t, env)
	for _, ids := range [][]string{{"00000000-0000-0000-0000-000000000001"}, {}} {
		body, err := json.Marshal(map[string]any{"course_ids": ids})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/unused", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	}
}
