package handler_test

import (
	"akademi-bimbel/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// fakeRepo is an in-memory UserRepository for handler tests.
type fakeRepo struct {
	byID map[string]*model.User
	seq  int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]*model.User{}}
}

func (f *fakeRepo) Ping(_ context.Context) error { return nil }

func (f *fakeRepo) CreateUser(_ context.Context, u *model.User) error {
	f.seq++
	u.ID = fmt.Sprintf("u%d", f.seq)
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.AuthProvider == "" {
		u.AuthProvider = "password"
	}
	cp := *u
	f.byID[u.ID] = &cp
	return nil
}

func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Email != nil && *u.Email == email && u.Status != "deleted" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) GetUserByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Username != nil && *u.Username == username && u.Status != "deleted" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) GetUserByID(_ context.Context, id string) (*model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (f *fakeRepo) GetUsersByIDs(_ context.Context, ids []uuid.UUID) ([]model.User, error) {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id.String()] = true
	}
	var users []model.User
	for _, u := range f.byID {
		if u.Role != "student" {
			continue
		}
		if idSet[u.ID] {
			users = append(users, *u)
		}
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

func (f *fakeRepo) UpdatePasswordHash(_ context.Context, userID, hash string) error {
	u, ok := f.byID[userID]
	if !ok {
		return fmt.Errorf("not found")
	}
	u.PasswordHash = hash
	return nil
}

func (f *fakeRepo) UpdateUserProfile(_ context.Context, userID string, name, email, username, phone, address, targetExam *string, grade *int, dob *time.Time, applySchool bool, schoolID *string, unlistedSchoolName *string, jenjang *string, provinsiID, kotaID, kecamatanID, kodePos *string) error {
	u, ok := f.byID[userID]
	if !ok {
		return fmt.Errorf("not found")
	}
	if name != nil {
		u.Name = *name
	}
	if email != nil {
		u.Email = email
	}
	if dob != nil {
		u.DOB = dob
	}
	if username != nil {
		u.Username = username
	}
	if phone != nil {
		u.Phone = phone
	}
	if address != nil {
		u.AlamatDomisili = address
	}
	if targetExam != nil {
		u.TargetExam = targetExam
	}
	if grade != nil {
		u.Grade = grade
	}
	if applySchool {
		u.SchoolID = schoolID
		u.UnlistedSchoolName = unlistedSchoolName
	}
	if jenjang != nil {
		u.Jenjang = jenjang
	}
	if provinsiID != nil {
		u.ProvinsiID = provinsiID
	}
	if kotaID != nil {
		u.KotaID = kotaID
	}
	if kecamatanID != nil {
		u.KecamatanID = kecamatanID
	}
	if kodePos != nil {
		u.KodePos = kodePos
	}
	return nil
}

func (f *fakeRepo) UpdateUserPhoto(_ context.Context, userID, photoURL string) error {
	u, ok := f.byID[userID]
	if !ok {
		return fmt.Errorf("not found")
	}
	u.PhotoURL = &photoURL
	return nil
}

func (f *fakeRepo) ListSchools(_ context.Context) ([]*model.School, error) {
	return nil, nil
}

func (f *fakeRepo) ActivateUser(_ context.Context, userID string) (bool, error) {
	u, ok := f.byID[userID]
	if !ok {
		return false, fmt.Errorf("not found")
	}
	if u.Status != "pending_verification" {
		return false, nil
	}
	u.Status = "active"
	u.OTPEnabled = false
	return true, nil
}

func (f *fakeRepo) TombstoneUser(_ context.Context, userID string) error {
	u, ok := f.byID[userID]
	if !ok {
		return fmt.Errorf("not found")
	}
	u.Status = "deleted"
	return nil
}

func (f *fakeRepo) seed(u *model.User) {
	f.seq++
	if u.ID == "" {
		u.ID = fmt.Sprintf("seed%d", f.seq)
	}
	if u.AuthProvider == "" {
		u.AuthProvider = "password"
	}
	cp := *u
	f.byID[u.ID] = &cp
}

func strptr(s string) *string { return &s }

func mustHash(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}

type testEnv struct {
	e      *echo.Echo
	mr     *miniredis.Miniredis
	svc    *service.Service
	signer *infra.JWTSigner
	repo   *fakeRepo
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{
		JWTSecret:       "handler-test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
		GoogleClientID:  "handler-google-client",
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	repo := newFakeRepo()
	svc := service.New(repo, rdb, signer, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, cfg)

	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login, handler.LoginRateLimiter())
	auth.POST("/google", h.GoogleLogin)
	auth.POST("/otp/send", h.SendOTP)
	auth.POST("/otp/verify", h.VerifyOTP)
	auth.POST("/logout", h.Logout, handler.JWTMiddleware(svc, signer))
	auth.POST("/password/forgot", h.ForgotPassword)
	auth.POST("/password/reset", h.ResetPassword)
	auth.PATCH("/password/change", h.ChangePassword, handler.JWTMiddleware(svc, signer))
	auth.GET("/me", h.Me, handler.JWTMiddleware(svc, signer))
	auth.PATCH("/photo", h.UpdatePhoto, handler.JWTMiddleware(svc, signer))

	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(svc, signer))
	adminProducts := admin.Group("/products")
	adminProducts.GET("", h.AdminListProducts)
	adminProducts.POST("", h.AdminCreateProduct)
	adminProducts.GET("/:id", h.AdminGetProduct)
	adminProducts.PATCH("/:id", h.AdminUpdateProduct)
	adminProducts.POST("/:id/publish", h.AdminPublishProduct)
	adminProducts.DELETE("/:id", h.AdminDeleteProduct)

	return &testEnv{e: e, mr: mr, svc: svc, signer: signer, repo: repo}
}

func postJSON(t *testing.T, e *echo.Echo, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func getWithToken(t *testing.T, e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// --- Test cases ---

func TestRegisterHandler_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	rec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "budi@example.com",
		"password": "strongpass123",
		"name":     "Budi",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["otp_required"] != true {
		t.Errorf("want otp_required=true, got %v", resp["otp_required"])
	}
	if resp["pending_token"] == "" || resp["pending_token"] == nil {
		t.Error("want non-empty pending_token")
	}
}

func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		Email:        strptr("taken@example.com"),
		PasswordHash: mustHash("whatever"),
		Role:         service.RoleStudent,
		Status:       "active",
	})
	rec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "taken@example.com",
		"password": "strongpass123",
		"name":     "Budi",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "email_taken" {
		t.Errorf("want code=email_taken, got %v", resp["code"])
	}
}

func TestLoginHandler_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("user@example.com"),
		PasswordHash: mustHash("password123"),
		Role:         service.RoleStudent,
		Status:       "active",
	})
	rec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": "user@example.com",
		"password":   "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["access_token"] == "" || resp["access_token"] == nil {
		t.Error("want access_token")
	}
	if resp["refresh_token"] == "" || resp["refresh_token"] == nil {
		t.Error("want refresh_token")
	}
	user, _ := resp["user"].(map[string]any)
	if user == nil {
		t.Fatal("want user in response")
	}
	if ap, _ := user["auth_provider"].(string); ap != "password" {
		t.Errorf("user.auth_provider: want 'password' (default), got '%s'", ap)
	}
	if _, ok := user["school_id"]; !ok {
		t.Error("user.school_id missing from response")
	}
	if _, ok := user["grade"]; !ok {
		t.Error("user.grade missing from response")
	}
}

func TestGoogleLoginHandler_ResponseIncludesProfileGateFields(t *testing.T) {
	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"aud":            "handler-google-client",
			"email":          "google@example.com",
			"email_verified": "true",
			"name":           "Google Student",
		})
	}))
	t.Cleanup(tokenInfo.Close)

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = googleTokenInfoTransport{fakeURL: tokenInfo.URL}
	t.Cleanup(func() { http.DefaultClient.Transport = originalTransport })

	env := newTestEnv(t)
	schoolID := "school-db"
	grade := 12
	env.repo.seed(&model.User{
		ID:           "google-user",
		Email:        strptr("google@example.com"),
		Role:         service.RoleStudent,
		Status:       "active",
		AuthProvider: "google",
		SchoolID:     &schoolID,
		Grade:        &grade,
	})

	rec := postJSON(t, env.e, "/api/v1/auth/google", map[string]string{
		"id_token": "google-id-token",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("want user object, got %T", resp["user"])
	}
	if got := user["auth_provider"]; got != "google" {
		t.Errorf("auth_provider: want google, got %v", got)
	}
	if got := user["school_id"]; got != schoolID {
		t.Errorf("school_id: want %s, got %v", schoolID, got)
	}
	if got := user["grade"]; got != float64(grade) {
		t.Errorf("grade: want %d, got %v", grade, got)
	}
}

// TestGoogleLoginHandler_NonStudentEmail_409 covers FR-2's wire contract: an
// existing non-student email gets 403 with code=google_not_student, not just
// a service-level sentinel error.
func TestGoogleLoginHandler_NonStudentEmail_409(t *testing.T) {
	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"aud":            "handler-google-client",
			"email":          "admin-not-student@example.com",
			"email_verified": "true",
			"name":           "Admin Person",
		})
	}))
	t.Cleanup(tokenInfo.Close)

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = googleTokenInfoTransport{fakeURL: tokenInfo.URL}
	t.Cleanup(func() { http.DefaultClient.Transport = originalTransport })

	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:           "admin-user",
		Email:        strptr("admin-not-student@example.com"),
		Role:         service.RoleAdminStore,
		Status:       "active",
		AuthProvider: "password",
	})

	rec := postJSON(t, env.e, "/api/v1/auth/google", map[string]string{
		"id_token": "google-id-token",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["code"] != "google_not_student" {
		t.Errorf("code: want google_not_student, got %v", resp["code"])
	}
}

func TestVerifyOTPHandler_HappyPath(t *testing.T) {
	env := newTestEnv(t)

	// Register to create an OTP challenge.
	regRec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "otp@example.com",
		"password": "password123",
		"name":     "OTP User",
	})
	if regRec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", regRec.Code, regRec.Body.String())
	}
	var regResp map[string]any
	json.NewDecoder(regRec.Body).Decode(&regResp)
	pendingToken := regResp["pending_token"].(string)

	// Resolve userID from pending token and read OTP from miniredis.
	userID, _ := env.mr.Get("pending:" + pendingToken)
	if userID == "" {
		t.Fatal("pending token not stored in redis")
	}
	otpCode, _ := env.mr.Get("otp:" + userID)
	if otpCode == "" {
		t.Fatal("otp not stored in redis")
	}

	rec := postJSON(t, env.e, "/api/v1/auth/otp/verify", map[string]string{
		"pending_token": pendingToken,
		"code":          otpCode,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["access_token"] == "" || resp["access_token"] == nil {
		t.Error("want access_token")
	}
	if resp["refresh_token"] == "" || resp["refresh_token"] == nil {
		t.Error("want refresh_token")
	}
}

func TestMeHandler_WithoutToken(t *testing.T) {
	env := newTestEnv(t)
	rec := getWithToken(t, env.e, "/api/v1/auth/me", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "unauthorized" {
		t.Errorf("want code=unauthorized, got %v", resp["code"])
	}
}

// TestRegisterHandler_WeakPassword covers FR-3: weak password → 400 invalid_request.
func TestRegisterHandler_WeakPassword(t *testing.T) {
	env := newTestEnv(t)
	rec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "budi@example.com",
		"password": "short",
		"name":     "Budi",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("want code=invalid_request, got %v", resp["code"])
	}
}

// TestLoginHandler_RateLimit covers FR-29: >10 login attempts/min/IP → 429 rate_limited.
func TestLoginHandler_RateLimit(t *testing.T) {
	env := newTestEnv(t)
	body := map[string]string{"identifier": "x@example.com", "password": "wrongpass"}
	var lastCode int
	for i := 0; i < 12; i++ {
		rec := postJSON(t, env.e, "/api/v1/auth/login", body)
		lastCode = rec.Code
		if rec.Code == http.StatusTooManyRequests {
			var resp map[string]any
			json.NewDecoder(rec.Body).Decode(&resp)
			if resp["code"] != "rate_limited" {
				t.Errorf("want code=rate_limited, got %v", resp["code"])
			}
			return
		}
	}
	t.Errorf("want 429 after 11th attempt, last code=%d", lastCode)
}

func TestMeHandler_ValidToken(t *testing.T) {
	env := newTestEnv(t)
	email := "me@example.com"
	jwtSchoolID := "school-jwt"
	env.repo.seed(&model.User{
		ID:           "u1",
		Email:        &email,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleStudent,
		Status:       "active",
		SchoolID:     &jwtSchoolID,
	})

	// Login to get access token
	loginRec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": email,
		"password":   "password123",
	})
	var loginResp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&loginResp)
	accessToken, ok := loginResp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("want access_token in login response, got %v", loginResp)
	}
	claims, err := env.signer.ParseAccess(accessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.SchoolID == nil || *claims.SchoolID != jwtSchoolID {
		t.Fatalf("JWT school_id: want %s, got %v", jwtSchoolID, claims.SchoolID)
	}
	dbSchoolID := "school-db"
	env.repo.byID["u1"].SchoolID = &dbSchoolID

	rec := getWithToken(t, env.e, "/api/v1/auth/me", accessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["email"] != email {
		t.Errorf("want email=%s, got %v", email, resp["email"])
	}
	if resp["role"] == nil {
		t.Error("want role in response")
	}
	if ap, _ := resp["auth_provider"].(string); ap != "password" {
		t.Errorf("auth_provider: want 'password' (DB truth), got '%s'", ap)
	}
	if got := resp["school_id"]; got != dbSchoolID {
		t.Errorf("school_id: want DB value %s, got %v", dbSchoolID, got)
	}
}

type googleTokenInfoTransport struct {
	fakeURL string
}

func (t googleTokenInfoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "googleapis.com") {
		fakeReq, err := http.NewRequestWithContext(req.Context(), req.Method, t.fakeURL, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultTransport.RoundTrip(fakeReq)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// TestLoginHandler_UnverifiedUser covers FR-5: login for a pending_verification
// user returns 403 verification_pending with otp_required and a pending_token,
// not the generic invalid_credentials error.
func TestLoginHandler_UnverifiedUser(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("pending@example.com"),
		PasswordHash: mustHash("password123"),
		Role:         service.RoleStudent,
		Status:       "pending_verification",
	})
	rec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": "pending@example.com",
		"password":   "password123",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "verification_pending" {
		t.Errorf("want code=verification_pending, got %v", resp["code"])
	}
	if resp["otp_required"] != true {
		t.Errorf("want otp_required=true, got %v", resp["otp_required"])
	}
	if resp["pending_token"] == "" || resp["pending_token"] == nil {
		t.Error("want non-empty pending_token")
	}
	if resp["id"] != "pending@example.com" {
		t.Errorf("want id=pending@example.com, got %v", resp["id"])
	}
}

// TestRegisterHandler_ResendOnPendingEmail covers FR-6: re-registering with an
// email that already has a pending_verification account resends the OTP and
// returns 201 with a fresh pending_token, not 409 email_taken.
func TestRegisterHandler_ResendOnPendingEmail(t *testing.T) {
	env := newTestEnv(t)
	firstRec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "resend@example.com",
		"password": "strongpass123",
		"name":     "Budi",
	})
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	var firstResp map[string]any
	json.NewDecoder(firstRec.Body).Decode(&firstResp)
	firstToken := firstResp["pending_token"]
	env.mr.FastForward(time.Minute)

	rec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "resend@example.com",
		"password": "strongpass123",
		"name":     "Budi",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["otp_required"] != true {
		t.Errorf("want otp_required=true, got %v", resp["otp_required"])
	}
	if resp["pending_token"] == "" || resp["pending_token"] == nil {
		t.Error("want non-empty pending_token")
	}
	if resp["pending_token"] == firstToken {
		t.Error("want a fresh pending_token on resend, got same as first")
	}
}

// loginToken logs identifier/password in against env and returns the access
// token, failing the test on any error along the way.
func loginToken(t *testing.T, env *testEnv, identifier, password string) string {
	t.Helper()
	rec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": identifier,
		"password":   password,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: want 200, got %d body=%s", identifier, rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	token, _ := resp["access_token"].(string)
	if token == "" {
		t.Fatalf("login %s: want non-empty access_token", identifier)
	}
	return token
}

// TestMeHandler_PhotoURL covers FR-13: GET /auth/me gains photo_url (empty
// string when NULL) while every field already present keeps its name, type
// and value — asserted as a full key set, not just the new key.
func TestMeHandler_PhotoURL(t *testing.T) {
	env := newTestEnv(t)

	t.Run("NULL photo_url returns empty string and full key set", func(t *testing.T) {
		email := "nophoto@example.com"
		env.repo.seed(&model.User{
			ID:           "u-nophoto",
			Email:        &email,
			PasswordHash: mustHash("password123"),
			Role:         service.RoleStudent,
			Status:       "active",
		})
		token := loginToken(t, env, email, "password123")

		rec := getWithToken(t, env.e, "/api/v1/auth/me", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["photo_url"] != "" {
			t.Errorf("want empty photo_url for NULL, got %v", resp["photo_url"])
		}
		for _, key := range []string{"id", "email", "username", "name", "role", "school_id", "auth_provider", "status", "photo_url"} {
			if _, ok := resp[key]; !ok {
				t.Errorf("want key %q in /auth/me response, missing", key)
			}
		}
	})

	t.Run("set photo_url is returned verbatim", func(t *testing.T) {
		email := "withphoto@example.com"
		photo := "avatars/u-withphoto/photo.png"
		env.repo.seed(&model.User{
			ID:           "u-withphoto",
			Email:        &email,
			PasswordHash: mustHash("password123"),
			Role:         service.RoleStudent,
			Status:       "active",
			PhotoURL:     &photo,
		})
		token := loginToken(t, env, email, "password123")

		rec := getWithToken(t, env.e, "/api/v1/auth/me", token)
		var resp map[string]any
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["photo_url"] != photo {
			t.Errorf("want photo_url=%s, got %v", photo, resp["photo_url"])
		}
	})
}

// TestLoginHandler_ResponseIncludesPhotoURL covers FR-14: the login
// response's user object carries photo_url.
func TestLoginHandler_ResponseIncludesPhotoURL(t *testing.T) {
	env := newTestEnv(t)
	email := "photouser@example.com"
	photo := "avatars/u1/photo.png"
	env.repo.seed(&model.User{
		ID:           "u1",
		Email:        &email,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleStudent,
		Status:       "active",
		PhotoURL:     &photo,
	})

	rec := postJSON(t, env.e, "/api/v1/auth/login", map[string]string{
		"identifier": email,
		"password":   "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	user, _ := resp["user"].(map[string]any)
	if user == nil {
		t.Fatal("want user in response")
	}
	if user["photo_url"] != photo {
		t.Errorf("user.photo_url: want %s, got %v", photo, user["photo_url"])
	}
}

// TestUpdateOwnPhoto_Success covers FR-15: any authenticated role, including
// admin_store, can PATCH /auth/photo with a valid avatar key.
func TestUpdateOwnPhoto_Success(t *testing.T) {
	env := newTestEnv(t)
	email := "store-admin@example.com"
	env.repo.seed(&model.User{
		ID:           "u-store",
		Email:        &email,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleAdminStore,
		Status:       "active",
	})
	token := loginToken(t, env, email, "password123")

	rec := doPatchJSON(t, env.e, "/api/v1/auth/photo", map[string]string{
		"photo_url": "avatars/u-store/new.png",
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestUpdateOwnPhoto_InvalidURL covers FR-16: a photo_url that does not
// resolve to an uploaded avatar key is rejected with 422 validation_failed.
func TestUpdateOwnPhoto_InvalidURL(t *testing.T) {
	env := newTestEnv(t)
	email := "store-admin2@example.com"
	env.repo.seed(&model.User{
		ID:           "u-store2",
		Email:        &email,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleAdminStore,
		Status:       "active",
	})
	token := loginToken(t, env, email, "password123")

	rec := doPatchJSON(t, env.e, "/api/v1/auth/photo", map[string]string{
		"photo_url": "https://evil.example.com/x.png",
	}, token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "validation_failed" {
		t.Errorf("want code=validation_failed, got %v", resp["code"])
	}
}

// TestStudentsPhoto_RoleGate covers FR-17: PATCH /students/photo is
// unaffected by the new /auth/photo route — it still succeeds for a student
// token and still 403s for an admin token via StudentOnlyMiddleware.
func TestStudentsPhoto_RoleGate(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	v1 := env.e.Group("/api/v1")
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(env.svc, env.signer))
	students.Use(handler.StudentOnlyMiddleware())
	students.PATCH("/photo", h.UpdatePhoto)

	studentEmail := "student-photo@example.com"
	env.repo.seed(&model.User{
		ID:           "u-student",
		Email:        &studentEmail,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleStudent,
		Status:       "active",
	})
	adminEmail := "admin-photo@example.com"
	env.repo.seed(&model.User{
		ID:           "u-admin",
		Email:        &adminEmail,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleAdminStore,
		Status:       "active",
	})

	studentToken := loginToken(t, env, studentEmail, "password123")
	adminToken := loginToken(t, env, adminEmail, "password123")

	rec := doPatchJSON(t, env.e, "/api/v1/students/photo", map[string]string{
		"photo_url": "avatars/u-student/photo.png",
	}, studentToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for student, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doPatchJSON(t, env.e, "/api/v1/students/photo", map[string]string{
		"photo_url": "avatars/u-admin/photo.png",
	}, adminToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for admin, got %d body=%s", rec.Code, rec.Body.String())
	}
}
