package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

func TestStudentDashboard_Unauthorized(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.GET("/dashboard", h.StudentDashboard)

	rec := getWithToken(t, env.e, "/api/v1/students/dashboard", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "unauthorized" {
		t.Errorf("want code=unauthorized, got %v", resp["code"])
	}
}

func TestStudentProfile_Unauthorized(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.GET("/profile", h.StudentProfile)

	rec := getWithToken(t, env.e, "/api/v1/students/profile", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "unauthorized" {
		t.Errorf("want code=unauthorized, got %v", resp["code"])
	}
}

func TestStudentProfile_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "u1",
		Email:  strptr("student@example.com"),
		Name:   "Student One",
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.GET("/profile", h.StudentProfile)

	// Login to get token
	loginRec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": "student@example.com",
		"password":   "wrongpass", // bcrypt doesn't match
	})
	_ = loginRec

	// Login should fail due to password mismatch; let's register a proper user
	// Instead, seed with password hash so login works
	env2 := newTestEnv(t)
	env2.repo.seed(&model.User{
		ID:           "u2",
		Email:        strptr("test@example.com"),
		PasswordHash: mustHash("password123"),
		Name:         "Test User",
		Role:         service.RoleStudent,
		Status:       "active",
	})
	h2 := handler.New(env2.svc)
	v1b := env2.e.Group("/api/v1")
	students2 := v1b.Group("/students")
	students2.Use(handler.JWTMiddleware(env2.svc, env2.signer))
	students2.GET("/profile", h2.StudentProfile)

	loginRec2 := postJSON(t, env2.e, "/api/v1/auth/login", map[string]string{
		"identifier": "test@example.com",
		"password":   "password123",
	})
	if loginRec2.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d body=%s", loginRec2.Code, loginRec2.Body.String())
	}
	var loginResp map[string]any
	json.NewDecoder(loginRec2.Body).Decode(&loginResp)
	token, _ := loginResp["access_token"].(string)
	if token == "" {
		t.Fatal("no access_token in login response")
	}

	rec := getWithToken(t, env2.e, "/api/v1/students/profile", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["email"] != "test@example.com" {
		t.Errorf("email: want test@example.com, got %v", resp["email"])
	}
	if resp["name"] != "Test User" {
		t.Errorf("name: want Test User, got %v", resp["name"])
	}
}

func TestListSchools(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	env.e.GET("/api/v1/schools", h.ListSchools)

	rec := getWithToken(t, env.e, "/api/v1/schools", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp []any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode schools: %v", err)
	}
	if resp != nil {
		t.Error("schools: want null (empty repo), got non-nil")
	}
}

func TestStudentUpdateProfile_Unauthorized(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.PATCH("/profile", h.StudentUpdateProfile)

	rec := doPatchJSON(t, env.e, "/api/v1/students/profile", map[string]string{"name": "New Name"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestStudentUpdateProfile_InvalidBody(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:           "u-update",
		Email:        strptr("update@example.com"),
		PasswordHash: mustHash("password123"),
		Name:         "Update User",
		Role:         service.RoleStudent,
		Status:       "active",
	})
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.PATCH("/profile", h.StudentUpdateProfile)

	// First login to get a valid token
	loginRec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": "update@example.com",
		"password":   "password123",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d", loginRec.Code)
	}
	var loginResp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&loginResp)
	token, _ := loginResp["access_token"].(string)
	if token == "" {
		t.Fatal("no access_token in login response")
	}

	// Non-JSON body with valid token
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/profile", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid body, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePhoto_Unauthorized(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.PATCH("/photo", h.UpdatePhoto)

	rec := doPatchJSON(t, env.e, "/api/v1/students/photo", map[string]string{"photo_url": "http://example.com/photo.jpg"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestGeneratePresignUploadURL_Unauthorized(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	rec := getWithToken(t, env.e, "/api/v1/uploads/presign?filename=test.jpg", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestGeneratePresignUploadURL_MissingFilename(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	// Login to get token
	env.repo.seed(&model.User{
		ID:           "u-presign",
		Email:        strptr("presign@example.com"),
		PasswordHash: mustHash("password123"),
		Name:         "Presign User",
		Role:         service.RoleStudent,
		Status:       "active",
	})
	loginRec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": "presign@example.com",
		"password":   "password123",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d", loginRec.Code)
	}
	var loginResp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&loginResp)
	token, _ := loginResp["access_token"].(string)

	// Missing filename
	rec := getWithToken(t, env.e, "/api/v1/uploads/presign", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing filename, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGeneratePresignUploadURL_BogusKind_400(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	token := loginForPresignTest(t, env, "kind-bogus")

	rec := getWithToken(t, env.e, "/api/v1/uploads/presign?filename=test.jpg&kind=bogus", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for bogus kind, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, hasKey := resp["key"]; hasKey {
		t.Errorf("want no key in response for rejected kind, got %v", resp)
	}
}

func TestGeneratePresignUploadURL_NoKind_KeyUnderAvatars(t *testing.T) {
	env := newStorageBackedTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	token := loginForPresignTest(t, env, "kind-default")

	rec := getWithToken(t, env.e, "/api/v1/uploads/presign?filename=test.jpg", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	key, _ := resp["key"].(string)
	if !strings.HasPrefix(key, "avatars/") {
		t.Errorf("want key under avatars/, got %q", key)
	}
}

func TestGeneratePresignUploadURL_KindAvatar_KeyUnderAvatars(t *testing.T) {
	env := newStorageBackedTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	token := loginForPresignTest(t, env, "kind-avatar-explicit")

	rec := getWithToken(t, env.e, "/api/v1/uploads/presign?filename=test.jpg&kind=avatar", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	key, _ := resp["key"].(string)
	if !strings.HasPrefix(key, "avatars/") {
		t.Errorf("want key under avatars/, got %q", key)
	}
}

func TestGeneratePresignUploadURL_KindProduct_KeyUnderProduct(t *testing.T) {
	env := newStorageBackedTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(env.svc, env.signer))
	uploads.GET("/presign", h.GeneratePresignUploadURL)

	token := loginForPresignTest(t, env, "kind-product")

	rec := getWithToken(t, env.e, "/api/v1/uploads/presign?filename=test.jpg&kind=product", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	key, _ := resp["key"].(string)
	if !strings.HasPrefix(key, "product/") {
		t.Errorf("want key under product/, got %q", key)
	}
}

// loginForPresignTest seeds a student user and logs in, returning an access token.
func loginForPresignTest(t *testing.T, env *testEnv, idSuffix string) string {
	t.Helper()
	email := idSuffix + "@example.com"
	env.repo.seed(&model.User{
		ID:           "u-" + idSuffix,
		Email:        strptr(email),
		PasswordHash: mustHash("password123"),
		Name:         "Presign User",
		Role:         service.RoleStudent,
		Status:       "active",
	})
	loginRec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": email,
		"password":   "password123",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d", loginRec.Code)
	}
	var loginResp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&loginResp)
	token, _ := loginResp["access_token"].(string)
	return token
}

// newStorageBackedTestEnv mirrors newTestEnv but wires a real minio client
// (Region set explicitly, so PUT presigning never dials out — see
// avatar_proxy_test.go for the same trick) so presign tests can assert on the
// actual returned key's prefix instead of only checking the status code.
func newStorageBackedTestEnv(t *testing.T) *testEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{
		JWTSecret:               "handler-test-secret",
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         168 * time.Hour,
		OTPTTL:                  5 * time.Minute,
		GoogleClientID:          "handler-google-client",
		ObjectStorageBucketName: "bucket",
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	repo := newFakeRepo()
	storage, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	svc := service.NewWithStore(repo, nil, rdb, signer, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, nil, nil, storage, cfg)

	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	auth := v1.Group("/auth")
	auth.POST("/login", h.Login, handler.LoginRateLimiter())

	return &testEnv{e: e, mr: mr, svc: svc, signer: signer, repo: repo}
}

// doPatchJSON sends a PATCH request with JSON body.
func doPatchJSON(t *testing.T, e *echo.Echo, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
