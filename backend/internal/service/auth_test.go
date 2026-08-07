package service

import (
	"akademi-bimbel/config"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	byID      map[string]*model.User
	seq       int
	createErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byID: map[string]*model.User{}}
}

func (f *fakeUserRepo) Ping(_ context.Context) error { return nil }

func (f *fakeUserRepo) CreateUser(_ context.Context, u *model.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.seq++
	u.ID = fmt.Sprintf("user-%d", f.seq)
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

func (f *fakeUserRepo) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Email != nil && strings.ToLower(*u.Email) == email && u.Status != "deleted" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeUserRepo) GetUserByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Username != nil && *u.Username == username && u.Status != "deleted" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeUserRepo) GetUserByID(_ context.Context, id string) (*model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserRepo) UpdatePasswordHash(_ context.Context, userID, hash string) error {
	u, ok := f.byID[userID]
	if !ok {
		return errors.New("not found")
	}
	u.PasswordHash = hash
	return nil
}

func (f *fakeUserRepo) UpdateUserProfile(_ context.Context, userID string, name, email, username, phone, address, targetExam *string, grade *int, dob *time.Time, applySchool bool, schoolID *string, unlistedSchoolName *string, jenjang, provinsiID, kotaID, kecamatanID, kodePos *string) error {
	u, ok := f.byID[userID]
	if !ok {
		return errors.New("not found")
	}
	if name != nil {
		u.Name = *name
	}
	if email != nil {
		u.Email = email
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

func (f *fakeUserRepo) UpdateUserPhoto(_ context.Context, userID, photoURL string) error {
	u, ok := f.byID[userID]
	if !ok {
		return errors.New("not found")
	}
	u.PhotoURL = &photoURL
	return nil
}

func (f *fakeUserRepo) ListSchools(_ context.Context) ([]*model.School, error) {
	return nil, nil
}

func (f *fakeUserRepo) ActivateUser(_ context.Context, userID string) (bool, error) {
	u, ok := f.byID[userID]
	if !ok {
		return false, errors.New("not found")
	}
	if u.Status != "pending_verification" {
		return false, nil
	}
	u.Status = "active"
	u.OTPEnabled = false
	return true, nil
}

func (f *fakeUserRepo) GetUsersByIDs(_ context.Context, ids []uuid.UUID) ([]model.User, error) {
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

func (f *fakeUserRepo) TombstoneUser(_ context.Context, userID string) error {
	u, ok := f.byID[userID]
	if !ok {
		return errors.New("not found")
	}
	u.Status = "deleted"
	return nil
}

// seed inserts a user directly, bypassing CreateUser sequencing concerns.
func (f *fakeUserRepo) seed(u *model.User) {
	f.seq++
	if u.ID == "" {
		u.ID = fmt.Sprintf("seed-%d", f.seq)
	}
	cp := *u
	f.byID[u.ID] = &cp
}

func strptr(s string) *string { return &s }

func newTestService(t *testing.T, repo UserRepository) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
		GoogleClientID:  "google-client-id",
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	svc := New(repo, rdb, signer, &NoopOTPProvider{}, &NoopEmailProvider{}, cfg)
	return svc, mr
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path creates user with username", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		pending, err := svc.Register(ctx, "New@Example.com", "password123", "Budi")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		if pending == "" {
			t.Error("want non-empty pending_token")
		}
		u, _ := repo.GetUserByEmail(ctx, "new@example.com")
		if u == nil {
			t.Fatal("user not created")
		}
		if u.Role != RoleStudent || u.Status != "pending_verification" || !u.OTPEnabled {
			t.Errorf("unexpected user defaults: role=%s status=%s otp=%v", u.Role, u.Status, u.OTPEnabled)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("password123")); err != nil {
			t.Errorf("password not hashed correctly: %v", err)
		}
		if !mr.Exists("otp:" + u.ID) {
			t.Error("otp key should be stored after register")
		}
		if u.Username == nil {
			t.Fatal("username should be set")
		}
		if !strings.HasPrefix(*u.Username, "budi") {
			t.Errorf("Username %q: want prefix 'budi'", *u.Username)
		}
		if len(*u.Username) != 8 {
			t.Errorf("Username %q: want 8 chars (4 base + 4 digits), got %d", *u.Username, len(*u.Username))
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{Email: strptr("taken@example.com"), Status: "active", Role: RoleStudent})
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "taken@example.com", "password123", "Budi")
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken, got %v", err)
		}
	})

	t.Run("weak password", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "x@example.com", "short", "Budi")
		if !errors.Is(err, ErrWeakPassword) {
			t.Errorf("want ErrWeakPassword, got %v", err)
		}
	})

	t.Run("username generation exhaustion fails account creation", func(t *testing.T) {
		repo := &collisionRepo{fakeUserRepo: *newFakeUserRepo(), blockUntil: 99}
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "exhausted@example.com", "password123", "Budi Santoso")
		if !errors.Is(err, ErrUsernameGenerationExhausted) {
			t.Errorf("want ErrUsernameGenerationExhausted, got %v", err)
		}
		u, _ := repo.GetUserByEmail(ctx, "exhausted@example.com")
		if u != nil {
			t.Error("user should not have been created when username generation failed")
		}
	})

	t.Run("can login with generated username (FR-SELFREG-03)", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "loginuser@example.com", "password123", "Login User")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		u, _ := repo.GetUserByEmail(ctx, "loginuser@example.com")
		if u == nil || u.Username == nil {
			t.Fatal("user/username not created")
		}
		_, _, pending, err := svc.Login(ctx, *u.Username, "password123")
		if !errors.Is(err, ErrVerificationPending) {
			t.Fatalf("Login with username: want ErrVerificationPending, got %v", err)
		}
		if pending == "" {
			t.Error("Login with username: want non-empty pending_token")
		}
		_, _, _, err = svc.Login(ctx, *u.Username, "wrongpassword")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Login with wrong password: want ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("re-register on pending email is rate limited", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		if _, err := svc.Register(ctx, "pending@example.com", "password123", "Budi"); err != nil {
			t.Fatalf("first Register: %v", err)
		}

		_, err := svc.Register(ctx, "pending@example.com", "password123", "Budi")
		if !errors.Is(err, ErrOTPRateLimit) {
			t.Errorf("want ErrOTPRateLimit, got %v", err)
		}
	})

	t.Run("re-register on pending email resends after rate limit expires", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		first, err := svc.Register(ctx, "pending@example.com", "password123", "Budi")
		if err != nil {
			t.Fatalf("first Register: %v", err)
		}
		before, _ := repo.GetUserByEmail(ctx, "pending@example.com")
		mr.FastForward(time.Minute)

		second, err := svc.Register(ctx, "pending@example.com", "password123", "Budi")
		if err != nil {
			t.Fatalf("second Register: %v", err)
		}
		if errors.Is(err, ErrEmailTaken) {
			t.Error("want no ErrEmailTaken for pending email re-register")
		}
		if second == "" {
			t.Error("want non-empty pending_token")
		}
		if second == first {
			t.Error("want a fresh pending_token, got the same one")
		}
		after, _ := repo.GetUserByEmail(ctx, "pending@example.com")
		if after == nil || before == nil || after.ID != before.ID {
			t.Errorf("want same user id, got before=%v after=%v", before, after)
		}
		if !mr.Exists("pending:" + second) {
			t.Error("fresh pending token should be stored in redis")
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()

	seedActive := func(repo *fakeUserRepo, otp bool) {
		repo.seed(&model.User{
			Email:        strptr("user@example.com"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "active",
			OTPEnabled:   otp,
		})
	}

	t.Run("wrong password", func(t *testing.T) {
		repo := newFakeUserRepo()
		seedActive(repo, false)
		svc, _ := newTestService(t, repo)
		_, _, _, err := svc.Login(ctx, "user@example.com", "wrong")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("want ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("inactive user", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			Email:        strptr("user@example.com"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "deactivated",
		})
		svc, _ := newTestService(t, repo)
		_, _, _, err := svc.Login(ctx, "user@example.com", "password123")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("want ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("valid active user returns tokens inline", func(t *testing.T) {
		repo := newFakeUserRepo()
		seedActive(repo, false)
		svc, _ := newTestService(t, repo)
		access, refresh, _, err := svc.Login(ctx, "user@example.com", "password123")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if access == "" || refresh == "" {
			t.Error("want non-empty tokens")
		}
	})

	t.Run("login by username", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			Username:     strptr("budi"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "active",
		})
		svc, _ := newTestService(t, repo)
		access, _, _, err := svc.Login(ctx, "budi", "password123")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if access == "" {
			t.Error("want token for username login")
		}
	})

	t.Run("pending user with correct password blocks, resends otp, returns fresh pending token", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			ID:           "u-pending",
			Email:        strptr("pending@example.com"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "pending_verification",
			OTPEnabled:   true,
		})
		svc, mr := newTestService(t, repo)
		access, refresh, pending, err := svc.Login(ctx, "pending@example.com", "password123")
		if !errors.Is(err, ErrVerificationPending) {
			t.Errorf("want ErrVerificationPending, got %v", err)
		}
		if access != "" || refresh != "" {
			t.Errorf("want no session tokens, got access=%q refresh=%q", access, refresh)
		}
		if pending == "" {
			t.Error("want non-empty fresh pending_token")
		}
		if !mr.Exists("pending:" + pending) {
			t.Error("fresh pending token should be stored in redis")
		}
		if !mr.Exists("otp:u-pending") {
			t.Error("otp should be re-dispatched to redis")
		}
	})

	t.Run("pending user with wrong password rejects without dispatching otp", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			ID:           "u-pending",
			Email:        strptr("pending@example.com"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "pending_verification",
			OTPEnabled:   true,
		})
		svc, mr := newTestService(t, repo)
		_, _, pending, err := svc.Login(ctx, "pending@example.com", "wrong")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("want ErrInvalidCredentials, got %v", err)
		}
		if pending != "" {
			t.Errorf("want no pending_token, got %q", pending)
		}
		if mr.Exists("otp:u-pending") {
			t.Error("otp should not be dispatched on wrong password")
		}
	})
}

func TestVerifyOTP(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) (*Service, *miniredis.Miniredis, *fakeUserRepo, string, string) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		pending, err := svc.Register(ctx, "user@example.com", "password123", "Budi")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		userID, _ := mr.Get("pending:" + pending)
		return svc, mr, repo, pending, userID
	}

	t.Run("correct code returns tokens, clears keys and disables otp", func(t *testing.T) {
		svc, mr, repo, pending, userID := setup(t)
		code, _ := mr.Get("otp:" + userID)
		access, refresh, err := svc.VerifyOTP(ctx, pending, code)
		if err != nil {
			t.Fatalf("VerifyOTP: %v", err)
		}
		if access == "" || refresh == "" {
			t.Error("want tokens")
		}
		if mr.Exists("otp:"+userID) || mr.Exists("pending:"+pending) {
			t.Error("otp/pending keys should be deleted after verify")
		}
		u, _ := repo.GetUserByID(ctx, userID)
		if u == nil || u.OTPEnabled {
			t.Error("otp should be disabled after verification")
		}
		if u == nil || u.Status != "active" {
			t.Errorf("want status active after verification, got %v", u)
		}
	})

	t.Run("deactivated user cannot be activated by a pending otp", func(t *testing.T) {
		svc, mr, repo, pending, userID := setup(t)
		code, _ := mr.Get("otp:" + userID)
		repo.byID[userID].Status = "deactivated"

		access, refresh, err := svc.VerifyOTP(ctx, pending, code)
		if !errors.Is(err, ErrInvalidPendingToken) {
			t.Errorf("want ErrInvalidPendingToken, got %v", err)
		}
		if access != "" || refresh != "" {
			t.Errorf("want no session tokens, got access=%q refresh=%q", access, refresh)
		}
		u, _ := repo.GetUserByID(ctx, userID)
		if u == nil || u.Status != "deactivated" {
			t.Errorf("want status deactivated, got %v", u)
		}
	})

	t.Run("wrong code", func(t *testing.T) {
		svc, _, _, pending, _ := setup(t)
		_, _, err := svc.VerifyOTP(ctx, pending, "000000")
		if !errors.Is(err, ErrInvalidOTP) {
			t.Errorf("want ErrInvalidOTP, got %v", err)
		}
	})

	t.Run("expired otp", func(t *testing.T) {
		svc, mr, _, pending, userID := setup(t)
		mr.Del("otp:" + userID)
		_, _, err := svc.VerifyOTP(ctx, pending, "123456")
		if !errors.Is(err, ErrOTPExpired) {
			t.Errorf("want ErrOTPExpired, got %v", err)
		}
	})

	t.Run("invalid pending token", func(t *testing.T) {
		svc, _, _, _, _ := setup(t)
		_, _, err := svc.VerifyOTP(ctx, "bogus", "123456")
		if !errors.Is(err, ErrInvalidPendingToken) {
			t.Errorf("want ErrInvalidPendingToken, got %v", err)
		}
	})
}

func TestRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("valid token rotates", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			ID:           "u1",
			Email:        strptr("user@example.com"),
			PasswordHash: mustHashStd("password123"),
			Role:         RoleStudent,
			Status:       "active",
		})
		svc, mr := newTestService(t, repo)
		_, refresh, _, err := svc.Login(ctx, "user@example.com", "password123")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		newAccess, newRefresh, err := svc.Refresh(ctx, refresh)
		if err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if newAccess == "" || newRefresh == "" {
			t.Error("want new tokens")
		}
		if mr.Exists("session:refresh:" + refresh) {
			t.Error("old refresh key should be deleted")
		}
		if !mr.Exists("session:refresh:" + newRefresh) {
			t.Error("new refresh key should exist")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		_, _, err := svc.Refresh(ctx, "nope")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("want ErrInvalidRefreshToken, got %v", err)
		}
	})
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("user@example.com"),
		PasswordHash: mustHashStd("password123"),
		Role:         RoleStudent,
		Status:       "active",
	})
	svc, mr := newTestService(t, repo)
	access, _, _, err := svc.Login(ctx, "user@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := infra.NewJWTSigner("test-secret", 15*time.Minute).ParseAccess(access)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	jti := claims.ID
	if !mr.Exists("session:access:" + jti) {
		t.Fatal("access session should exist")
	}
	if err := svc.Logout(ctx, jti, ""); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if mr.Exists("session:access:" + jti) {
		t.Error("access session should be deleted")
	}
	if err := svc.Logout(ctx, jti, ""); err != nil {
		t.Errorf("second Logout should be idempotent, got %v", err)
	}
}

func TestSendOTP(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:         "u1",
		Email:      strptr("user@example.com"),
		Role:       RoleStudent,
		Status:     "active",
		OTPEnabled: true,
	})
	svc, mr := newTestService(t, repo)

	if err := svc.SendOTP(ctx, "u1"); err != nil {
		t.Fatalf("first SendOTP: %v", err)
	}
	if !mr.Exists("otp:u1") {
		t.Error("otp key should be set")
	}
	if err := svc.SendOTP(ctx, "u1"); !errors.Is(err, ErrOTPRateLimit) {
		t.Errorf("second SendOTP within window: want ErrOTPRateLimit, got %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	ctx := context.Background()

	t.Run("valid token and otp updates password", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			ID:           "u1",
			Email:        strptr("user@example.com"),
			PasswordHash: mustHashStd("oldpassword"),
			Role:         RoleStudent,
			Status:       "active",
		})
		svc, mr := newTestService(t, repo)
		mr.Set("reset:tok", "u1:654321")
		if err := svc.ResetPassword(ctx, "tok", "654321", "brandnewpass"); err != nil {
			t.Fatalf("ResetPassword: %v", err)
		}
		u, _ := repo.GetUserByID(ctx, "u1")
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("brandnewpass")); err != nil {
			t.Errorf("password not updated: %v", err)
		}
		if mr.Exists("reset:tok") {
			t.Error("reset token should be deleted")
		}
	})

	t.Run("wrong otp", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), PasswordHash: mustHashStd("oldpassword"), Role: RoleStudent, Status: "active"})
		svc, mr := newTestService(t, repo)
		mr.Set("reset:tok", "u1:654321")
		if err := svc.ResetPassword(ctx, "tok", "000000", "brandnewpass"); !errors.Is(err, ErrInvalidResetToken) {
			t.Errorf("want ErrInvalidResetToken, got %v", err)
		}
	})

	t.Run("invalid reset token", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		if err := svc.ResetPassword(ctx, "missing", "654321", "brandnewpass"); !errors.Is(err, ErrInvalidResetToken) {
			t.Errorf("want ErrInvalidResetToken, got %v", err)
		}
	})
}

// TestGoogleLogin_InvalidToken covers FR-13: bad id_token → ErrInvalidToken.
func TestGoogleLogin_InvalidToken(t *testing.T) {
	// Stand up a fake tokeninfo server that returns 400.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	// Redirect http.DefaultClient to the fake server by overriding transport.
	orig := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: ts.URL, transport: http.DefaultTransport}
	defer func() { http.DefaultTransport = orig }()

	repo := newFakeUserRepo()
	svc, _ := newTestService(t, repo)

	_, _, err := svc.GoogleLogin(context.Background(), "bogus-id-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken, got %v", err)
	}
}

// redirectTransport rewrites all requests to a fixed base URL (fake server).
type redirectTransport struct {
	target    string
	transport http.RoundTripper
}

func (r *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := r.target + req.URL.RequestURI()
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	return r.transport.RoundTrip(newReq)
}

// TestLogout_WithRefreshToken covers FR-17: logout also deletes the refresh token when supplied.
func TestLogout_WithRefreshToken(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("user@example.com"),
		PasswordHash: mustHashStd("password123"),
		Role:         RoleStudent,
		Status:       "active",
	})
	svc, mr := newTestService(t, repo)

	access, refresh, _, err := svc.Login(ctx, "user@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	claims, err := infra.NewJWTSigner("test-secret", 15*time.Minute).ParseAccess(access)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	jti := claims.ID

	if !mr.Exists("session:refresh:" + refresh) {
		t.Fatal("refresh key should exist before logout")
	}

	if err := svc.Logout(ctx, jti, refresh); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if mr.Exists("session:access:" + jti) {
		t.Error("access session should be deleted")
	}
	if mr.Exists("session:refresh:" + refresh) {
		t.Error("refresh session should be deleted when refresh_token supplied")
	}
}

// TestResetPassword_RevokesAllSessions covers FR-20: password reset invalidates ALL user sessions.
func TestResetPassword_RevokesAllSessions(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("user@example.com"),
		PasswordHash: mustHashStd("oldpassword"),
		Role:         RoleStudent,
		Status:       "active",
	})
	svc, mr := newTestService(t, repo)

	// Mint a session so we have live access+refresh keys.
	access, refresh, _, err := svc.Login(ctx, "user@example.com", "oldpassword")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := infra.NewJWTSigner("test-secret", 15*time.Minute).ParseAccess(access)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	jti := claims.ID

	// Confirm keys exist before reset.
	if !mr.Exists("session:access:" + jti) {
		t.Fatal("access key should exist before reset")
	}
	if !mr.Exists("session:refresh:" + refresh) {
		t.Fatal("refresh key should exist before reset")
	}

	// Trigger reset.
	mr.Set("reset:tok", "u1:654321")
	if err := svc.ResetPassword(ctx, "tok", "654321", "brandnewpass"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	// Both session keys must be gone.
	if mr.Exists("session:access:" + jti) {
		t.Error("access session should be invalidated after password reset")
	}
	if mr.Exists("session:refresh:" + refresh) {
		t.Error("refresh session should be invalidated after password reset")
	}
}

// TestChangePassword_RevokesOtherSessionsKeepsCaller covers FR-29..FR-30 and
// FR-33: a successful password change kills every OTHER session for the
// user, keeps the caller's own session alive, and does so by removing
// revoked jtis individually from user_access_sessions:<uid> rather than
// deleting the index set wholesale. Runs for both a student and an admin
// role (FR-33: no role gates this behaviour).
func TestChangePassword_RevokesOtherSessionsKeepsCaller(t *testing.T) {
	ctx := context.Background()
	signer := infra.NewJWTSigner("test-secret", 15*time.Minute)

	for _, role := range []string{RoleStudent, "admin_store"} {
		t.Run(role, func(t *testing.T) {
			repo := newFakeUserRepo()
			repo.seed(&model.User{
				ID:           "u1",
				Email:        strptr("user@example.com"),
				PasswordHash: mustHashStd("oldpassword"),
				Role:         role,
				Status:       "active",
			})
			svc, mr := newTestService(t, repo)

			accessA, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword")
			if err != nil {
				t.Fatalf("login A: %v", err)
			}
			claimsA, err := signer.ParseAccess(accessA)
			if err != nil {
				t.Fatalf("parse A: %v", err)
			}
			jtiA := claimsA.ID

			accessB, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword")
			if err != nil {
				t.Fatalf("login B: %v", err)
			}
			claimsB, err := signer.ParseAccess(accessB)
			if err != nil {
				t.Fatalf("parse B: %v", err)
			}
			jtiB := claimsB.ID

			if err := svc.ChangePassword(ctx, "u1", "oldpassword", "brandnewpass", jtiA); err != nil {
				t.Fatalf("ChangePassword: %v", err)
			}

			// Assertion 1: the other session is dead.
			if svc.SessionActive(ctx, jtiB) {
				t.Error("want jtiB session dead after password change")
			}
			// Assertion 2: the caller survives.
			if !svc.SessionActive(ctx, jtiA) {
				t.Error("want jtiA (caller) session to survive")
			}
			// Assertion 3: the index set was not deleted wholesale — jtiA is
			// still a member, jtiB is not. Checked directly against
			// miniredis, not via SessionActive, since that's the assertion
			// a Del-the-set bug would still pass.
			members, err := mr.SMembers("user_access_sessions:u1")
			if err != nil {
				t.Fatalf("SMembers: %v", err)
			}
			foundA, foundB := false, false
			for _, m := range members {
				if m == jtiA {
					foundA = true
				}
				if m == jtiB {
					foundB = true
				}
			}
			if !foundA {
				t.Error("want jtiA still a member of user_access_sessions index")
			}
			if foundB {
				t.Error("want jtiB removed from user_access_sessions index")
			}
		})
	}
}

// TestChangePassword_FailedChangeRevokesNothing covers FR-32: a failed
// change (wrong current_password) revokes no session and writes no new hash.
func TestChangePassword_FailedChangeRevokesNothing(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:           "u1",
		Email:        strptr("user@example.com"),
		PasswordHash: mustHashStd("oldpassword"),
		Role:         RoleStudent,
		Status:       "active",
	})
	svc, _ := newTestService(t, repo)
	signer := infra.NewJWTSigner("test-secret", 15*time.Minute)

	accessA, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword")
	if err != nil {
		t.Fatalf("login A: %v", err)
	}
	claimsA, err := signer.ParseAccess(accessA)
	if err != nil {
		t.Fatalf("parse A: %v", err)
	}
	accessB, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword")
	if err != nil {
		t.Fatalf("login B: %v", err)
	}
	claimsB, err := signer.ParseAccess(accessB)
	if err != nil {
		t.Fatalf("parse B: %v", err)
	}

	err = svc.ChangePassword(ctx, "u1", "wrongpassword", "brandnewpass", claimsA.ID)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}

	if !svc.SessionActive(ctx, claimsA.ID) {
		t.Error("failed change must not revoke the caller's session")
	}
	if !svc.SessionActive(ctx, claimsB.ID) {
		t.Error("failed change must not revoke the other session")
	}
	u, _ := repo.GetUserByID(ctx, "u1")
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("oldpassword")) != nil {
		t.Error("failed change must not alter the password hash")
	}
}

// mustHashStd hashes outside a *testing.T context (used in closures/seed).
func mustHashStd(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}
