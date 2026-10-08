package service

import (
	"akademi-bimbel/config"
	"context"
	"encoding/hex"
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
		OTPSecret:       "test-otp-secret",
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
		chall, err := svc.Register(ctx, "New@Example.com", "password123", "Budi", "1.1.1.1")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		if chall.PendingToken == "" {
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
		_, err := svc.Register(ctx, "taken@example.com", "password123", "Budi", "1.1.1.1")
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken, got %v", err)
		}
	})

	t.Run("weak password", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "x@example.com", "short", "Budi", "1.1.1.1")
		if !errors.Is(err, ErrWeakPassword) {
			t.Errorf("want ErrWeakPassword, got %v", err)
		}
	})

	t.Run("username generation exhaustion fails account creation", func(t *testing.T) {
		repo := &collisionRepo{fakeUserRepo: *newFakeUserRepo(), blockUntil: 99}
		svc, _ := newTestService(t, repo)
		_, err := svc.Register(ctx, "exhausted@example.com", "password123", "Budi Santoso", "1.1.1.1")
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
		svc, mr := newTestService(t, repo)
		_, err := svc.Register(ctx, "loginuser@example.com", "password123", "Login User", "1.1.1.1")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		// Register armed the 60s resend cooldown; the login OTP dispatch shares
		// the same ladder, so step past it.
		mr.FastForward(time.Minute + time.Second)
		u, _ := repo.GetUserByEmail(ctx, "loginuser@example.com")
		if u == nil || u.Username == nil {
			t.Fatal("user/username not created")
		}
		_, _, pending, err := svc.Login(ctx, *u.Username, "password123", "1.1.1.1")
		if !errors.Is(err, ErrVerificationPending) {
			t.Fatalf("Login with username: want ErrVerificationPending, got %v", err)
		}
		if pending == "" {
			t.Error("Login with username: want non-empty pending_token")
		}
		_, _, _, err = svc.Login(ctx, *u.Username, "wrongpassword", "1.1.1.1")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Login with wrong password: want ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("re-register on pending email with a live code skips the send", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		if _, err := svc.Register(ctx, "pending@example.com", "password123", "Budi", "1.1.1.1"); err != nil {
			t.Fatalf("first Register: %v", err)
		}
		codeBefore, _ := mr.Get("otp:" + "pending@example.com")

		chall, err := svc.Register(ctx, "pending@example.com", "password123", "Budi", "1.1.1.1")
		if err != nil {
			t.Fatalf("second Register: %v", err)
		}
		if !chall.OTPActive {
			t.Error("want otp_active=true while the first code is still live")
		}
		if chall.EmailFailed {
			t.Error("want email_failed=false — nothing was sent")
		}
		if chall.ResendAfter <= 0 || chall.ResendAfter > 60 {
			t.Errorf("ResendAfter = %d, want the armed 60s cooldown", chall.ResendAfter)
		}
		if codeAfter, _ := mr.Get("otp:" + "pending@example.com"); codeAfter != codeBefore {
			t.Error("the stored code must not change when no send happens")
		}
	})

	t.Run("re-register on pending email resends after rate limit expires", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		first, err := svc.Register(ctx, "pending@example.com", "password123", "Budi", "1.1.1.1")
		if err != nil {
			t.Fatalf("first Register: %v", err)
		}
		before, _ := repo.GetUserByEmail(ctx, "pending@example.com")
		mr.FastForward(time.Minute)

		second, err := svc.Register(ctx, "pending@example.com", "password123", "Budi", "1.1.1.1")
		if err != nil {
			t.Fatalf("second Register: %v", err)
		}
		if errors.Is(err, ErrEmailTaken) {
			t.Error("want no ErrEmailTaken for pending email re-register")
		}
		if !second.OTPActive {
			t.Error("want otp_active=true — the live code skips the send")
		}
		if second.PendingToken == "" {
			t.Error("want non-empty pending_token")
		}
		if second.PendingToken == first.PendingToken {
			t.Error("want a fresh pending_token, got the same one")
		}
		after, _ := repo.GetUserByEmail(ctx, "pending@example.com")
		if after == nil || before == nil || after.ID != before.ID {
			t.Errorf("want same user id, got before=%v after=%v", before, after)
		}
		if !mr.Exists("pending:" + second.PendingToken) {
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
		_, _, _, err := svc.Login(ctx, "user@example.com", "wrong", "1.1.1.1")
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
		_, _, _, err := svc.Login(ctx, "user@example.com", "password123", "1.1.1.1")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("want ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("valid active user returns tokens inline", func(t *testing.T) {
		repo := newFakeUserRepo()
		seedActive(repo, false)
		svc, _ := newTestService(t, repo)
		access, refresh, _, err := svc.Login(ctx, "user@example.com", "password123", "1.1.1.1")
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
		access, _, _, err := svc.Login(ctx, "budi", "password123", "1.1.1.1")
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
		access, refresh, pending, err := svc.Login(ctx, "pending@example.com", "password123", "1.1.1.1")
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
		_, _, pending, err := svc.Login(ctx, "pending@example.com", "wrong", "1.1.1.1")
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

	setup := func(t *testing.T) (*Service, *miniredis.Miniredis, *fakeUserRepo, string, string, *stubOTPProvider) {
		repo := newFakeUserRepo()
		svc, mr := newTestService(t, repo)
		capture := &stubOTPProvider{}
		svc.otpProvider = capture
		chall, err := svc.Register(ctx, "user@example.com", "password123", "Budi", "1.1.1.1")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		userID, _ := mr.Get("pending:" + chall.PendingToken)
		return svc, mr, repo, chall.PendingToken, userID, capture
	}

	t.Run("correct code returns tokens, clears keys and disables otp", func(t *testing.T) {
		svc, mr, repo, pending, userID, capture := setup(t)
		code := capture.codes[0]
		access, refresh, err := svc.VerifyOTP(ctx, pending, code, "1.1.1.1")
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
		svc, _, repo, pending, userID, capture := setup(t)
		code := capture.codes[0]
		repo.byID[userID].Status = "deactivated"

		access, refresh, err := svc.VerifyOTP(ctx, pending, code, "1.1.1.1")
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
		svc, _, _, pending, _, capture := setup(t)
		_ = capture
		_, _, err := svc.VerifyOTP(ctx, pending, "000000", "1.1.1.1")
		if !errors.Is(err, ErrInvalidOTP) {
			t.Errorf("want ErrInvalidOTP, got %v", err)
		}
	})

	t.Run("three wrong entries kill the code, a fresh dispatch restores the budget", func(t *testing.T) {
		svc, mr, _, pending, userID, _ := setup(t)

		for i := 1; i <= 3; i++ {
			_, _, err := svc.VerifyOTP(ctx, pending, fmt.Sprintf("wrong-%d", i), "1.1.1.1")
			if i < maxOTPVerifyAttempts && !errors.Is(err, ErrInvalidOTP) {
				t.Fatalf("wrong #%d: want ErrInvalidOTP, got %v", i, err)
			}
			if i == maxOTPVerifyAttempts && !errors.Is(err, ErrOTPAttemptsExceeded) {
				t.Fatalf("wrong #%d: want ErrOTPAttemptsExceeded, got %v", i, err)
			}
		}
		if mr.Exists("otp:" + userID) {
			t.Error("the code should be killed after three wrong entries")
		}
		if _, _, err := svc.VerifyOTP(ctx, pending, "000000", "1.1.1.1"); !errors.Is(err, ErrOTPExpired) {
			t.Errorf("post-kill verify err = %v, want ErrOTPExpired", err)
		}

		// A fresh dispatch restores the budget: one wrong entry is a plain
		// invalid OTP again, not an attempts-exceeded.
		mr.FastForward(time.Minute + time.Second) // step past the ladder cooldown
		if err := svc.SendOTP(ctx, userID, "1.1.1.1"); err != nil {
			t.Fatalf("SendOTP: %v", err)
		}
		if _, _, err := svc.VerifyOTP(ctx, pending, "000000", "1.1.1.1"); !errors.Is(err, ErrInvalidOTP) {
			t.Errorf("after re-dispatch err = %v, want ErrInvalidOTP (fresh budget)", err)
		}
	})

	t.Run("expired otp", func(t *testing.T) {
		svc, mr, _, pending, userID, _ := setup(t)
		mr.Del("otp:" + userID)
		_, _, err := svc.VerifyOTP(ctx, pending, "123456", "1.1.1.1")
		if !errors.Is(err, ErrOTPExpired) {
			t.Errorf("want ErrOTPExpired, got %v", err)
		}
	})

	t.Run("invalid pending token", func(t *testing.T) {
		svc, _, _, _, _, _ := setup(t)
		_, _, err := svc.VerifyOTP(ctx, "bogus", "123456", "1.1.1.1")
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
		_, refresh, _, err := svc.Login(ctx, "user@example.com", "password123", "1.1.1.1")
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
	access, _, _, err := svc.Login(ctx, "user@example.com", "password123", "1.1.1.1")
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

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("first SendOTP: %v", err)
	}
	if !mr.Exists("otp:u1") {
		t.Error("otp key should be set")
	}
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrOTPRateLimit) {
		t.Errorf("second SendOTP within window: want ErrOTPRateLimit, got %v", err)
	}
}

// stubOTPProvider lets OTP-send tests control the provider outcome. With
// failChannel set, only sends on that channel fail; others succeed. Successful
// sends are captured so tests can assert on the delivered codes.
type stubOTPProvider struct {
	err         error
	failChannel string
	sent        int
	codes       []string
}

func (f *stubOTPProvider) SendOTP(_ context.Context, channel, _, code string) error {
	f.sent++
	if f.err == nil || (f.failChannel != "" && channel != f.failChannel) {
		f.codes = append(f.codes, code)
		return nil
	}
	return f.err
}

func TestRegister_EmailSendFailure_NonFatal(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc, mr := newTestService(t, repo)
	otp := &stubOTPProvider{err: errors.New("451 4.7.1 Ratelimit exceeded")}
	svc.otpProvider = otp

	chall, err := svc.Register(ctx, "relaydown@example.com", "password123", "Budi", "1.1.1.1")
	if err != nil {
		t.Fatalf("Register with failing SMTP: want non-fatal, got %v", err)
	}
	if chall.PendingToken == "" {
		t.Error("want non-empty pending_token even when the send failed")
	}
	if !chall.EmailFailed {
		t.Error("want email_failed=true")
	}
	u, _ := repo.GetUserByEmail(ctx, "relaydown@example.com")
	if u == nil {
		t.Fatal("user should still be created when the send fails")
	}
	if mr.Exists("otp:" + u.ID) {
		t.Error("no live code should exist when the send failed — the undelivered code must not replace anything")
	}
	if mr.Exists("otp:new:" + u.ID) {
		t.Error("pending code key should be discarded when the send failed")
	}
	if otp.sent != 1 {
		t.Errorf("want exactly 1 send attempt, got %d", otp.sent)
	}

	// After the provider recovers, re-registering retries the send and does
	// store a live code.
	otp.err = nil
	mr.FastForward(otpSendfailPause + time.Second)
	retry, err := svc.Register(ctx, "relaydown@example.com", "password123", "Budi", "1.1.1.1")
	if err != nil {
		t.Fatalf("retry Register: %v", err)
	}
	if retry.EmailFailed {
		t.Error("retry should report email_failed=false when the send succeeds")
	}
	if !mr.Exists("otp:" + u.ID) {
		t.Error("successful retry should store a live code")
	}
}

func TestResend_EmailDeliveryFailed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, _ := newTestService(t, repo)
	svc.otpProvider = &stubOTPProvider{err: errors.New("451 4.7.1 Ratelimit exceeded")}

	// The resend endpoint surfaces the failure instead of swallowing it.
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	if !errors.Is(err, ErrEmailDeliveryFailed) {
		t.Errorf("want ErrEmailDeliveryFailed, got %v", err)
	}
}

func TestLogin_EmailFailedFlag(t *testing.T) {
	ctx := context.Background()
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
	svc.otpProvider = &stubOTPProvider{err: errors.New("smtp down")}

	_, _, pending, err := svc.Login(ctx, "pending@example.com", "password123", "1.1.1.1")
	if !errors.Is(err, ErrVerificationPending) {
		t.Fatalf("want ErrVerificationPending, got %v", err)
	}
	var vp *VerificationPendingError
	if !errors.As(err, &vp) {
		t.Fatal("want typed VerificationPendingError")
	}
	if !vp.EmailFailed {
		t.Error("want EmailFailed=true on the 403 path")
	}
	if pending == "" {
		t.Error("want non-empty pending_token even when the send failed")
	}
	if mr.Exists("otp:u-pending") {
		t.Error("no live code should exist when the send failed")
	}
}

func TestOTPSendLadder(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	// The ladder test sends 6 codes/hour; lift the hour quota so only the
	// ladder itself is exercised.
	svc.cfg.OTPQuotaHour = 12

	// cooldownAfterSend asserts the immediate-next-send Retry-After matches
	// the ladder step for the Nth send.
	cooldownAfterSend := func(t *testing.T, want time.Duration) {
		t.Helper()
		err := svc.SendOTP(ctx, "u1", "1.1.1.1")
		var rl *OTPRateLimitError
		if !errors.As(err, &rl) {
			t.Fatalf("want OTPRateLimitError, got %v", err)
		}
		if rl.RetryAfter < want-time.Second || rl.RetryAfter > want {
			t.Errorf("RetryAfter = %v, want ~%v", rl.RetryAfter, want)
		}
	}

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	cooldownAfterSend(t, time.Minute)
	mr.FastForward(time.Minute + time.Second)

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 2: %v", err)
	}
	cooldownAfterSend(t, 2*time.Minute)
	mr.FastForward(2*time.Minute + time.Second)

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 3: %v", err)
	}
	cooldownAfterSend(t, 5*time.Minute)
	mr.FastForward(5*time.Minute + time.Second)

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 4: %v", err)
	}
	cooldownAfterSend(t, 15*time.Minute)
	mr.FastForward(15*time.Minute + time.Second)

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 5: %v", err)
	}
	cooldownAfterSend(t, time.Hour)
	// The ladder window (1h) also expired, so the next cooldown resets to 60s.
	mr.FastForward(time.Hour + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 6: %v", err)
	}
	cooldownAfterSend(t, time.Minute)
}

// TestOTPSendfailGate: three delivery failures stop sending for the remaining
// sendfail window — a 429-rate-limit pause around the relay, not the 12h
// per-channel lockout the old code used to punish the user with.
func TestOTPSendfailGate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	svc.otpProvider = &stubOTPProvider{err: errors.New("451 4.7.1 Ratelimit exceeded")}

	for i := 0; i < 3; i++ {
		if i > 0 {
			mr.FastForward(otpSendfailPause + time.Second)
		}
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
			t.Fatalf("failure send %d: want ErrEmailDeliveryFailed, got %v", i+1, err)
		}
	}
	if !mr.Exists(otpSendfailKey("u1", "email")) {
		t.Fatal("sendfail counter should exist after 3 delivery failures")
	}

	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	if !errors.Is(err, ErrOTPRateLimit) {
		t.Fatalf("want ErrOTPRateLimit gate after 3 sendfail, got %v", err)
	}
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatal("want typed OTPRateLimitError")
	}
	if rl.RetryAfter <= 0 || rl.RetryAfter > otpSendfailWindow {
		t.Errorf("RetryAfter = %v, want within the sendfail window", rl.RetryAfter)
	}
	if mr.Exists("otp:lock:u1:email") {
		t.Error("the 12h lockout key must no longer be written")
	}
}

// TestOTPSendfailPause: a single delivery failure arms a short fixed pause the
// user can retry after, regardless of ladder position.
func TestOTPSendfailPause(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	svc.otpProvider = &stubOTPProvider{err: errors.New("smtp down")}

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
		t.Fatalf("want ErrEmailDeliveryFailed, got %v", err)
	}

	// Immediate retry: still inside the pause.
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	if !errors.Is(err, ErrOTPRateLimit) {
		t.Fatalf("want ErrOTPRateLimit during the pause, got %v", err)
	}
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatal("want typed OTPRateLimitError")
	}
	if rl.RetryAfter < otpSendfailPause-time.Second || rl.RetryAfter > otpSendfailPause {
		t.Errorf("RetryAfter = %v, want ~%v", rl.RetryAfter, otpSendfailPause)
	}

	// Past the pause the retry goes through.
	mr.FastForward(otpSendfailPause + time.Second)
	otpErr := &stubOTPProvider{}
	svc.otpProvider = otpErr
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Errorf("retry after the pause: want nil, got %v", err)
	}
}

func TestLogin_NoSendWhenCodeStillAlive(t *testing.T) {
	ctx := context.Background()
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
	otp := &stubOTPProvider{}
	svc.otpProvider = otp
	// A live code from an earlier dispatch — e.g. the email is still in flight.
	mr.Set("otp:u-pending", "123456")

	_, _, pending, err := svc.Login(ctx, "pending@example.com", "password123", "1.1.1.1")

	if !errors.Is(err, ErrVerificationPending) {
		t.Fatalf("want ErrVerificationPending, got %v", err)
	}
	var vp *VerificationPendingError
	if !errors.As(err, &vp) {
		t.Fatal("want typed VerificationPendingError")
	}
	if !vp.OTPActive {
		t.Error("want OTPActive=true — a live code skips the send")
	}
	if vp.EmailFailed {
		t.Error("want EmailFailed=false — nothing was sent")
	}
	if pending == "" {
		t.Error("want a fresh pending token even without a send")
	}
	if otp.sent != 0 {
		t.Errorf("provider sends = %d, want 0", otp.sent)
	}
}

func TestOTPSendfail_ResetOnSuccess(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	otp := &stubOTPProvider{err: errors.New("451 4.7.1 Ratelimit exceeded")}
	svc.otpProvider = otp

	// Two failures, one success, two more failures — the success clears the
	// counter, so the second pair does not trip the gate.
	for i := 0; i < 2; i++ {
		if i > 0 {
			mr.FastForward(otpSendfailPause + time.Second)
		}
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
			t.Fatalf("initial failure %d: want ErrEmailDeliveryFailed, got %v", i+1, err)
		}
	}
	otp.err = nil
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("recovery send: %v", err)
	}
	if mr.Exists(otpSendfailKey("u1", "email")) {
		t.Error("successful send should clear the sendfail counter")
	}

	// The recovery send armed its ladder cooldown — step past it, then start
	// counting failures again from zero.
	otp.err = errors.New("451 4.7.1 Ratelimit exceeded")
	mr.FastForward(5 * time.Minute)
	for i := 0; i < 2; i++ {
		mr.FastForward(otpSendfailPause + time.Second)
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
			t.Fatalf("post-recovery failure %d: want ErrEmailDeliveryFailed, got %v", i+1, err)
		}
	}

	// The third failure since recovery lands (sendfail reaches 3). The gate
	// stops the fourth.
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
		t.Errorf("third failure: want ErrEmailDeliveryFailed, got %v", err)
	}
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrOTPRateLimit) {
		t.Errorf("fourth attempt: want ErrOTPRateLimit gate, got %v", err)
	}
}

func TestOTPSendfail_PerChannelIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{
		ID:    "u1",
		Email: strptr("user@example.com"),
		Phone: strptr("+628123456789"),
		Role:  RoleStudent, Status: "active", OTPEnabled: true,
	})
	svc, mr := newTestService(t, repo)
	// The user has a phone, so PreferredOTPChannel resolves to WhatsApp —
	// fail only that channel and leave email healthy.
	svc.otpProvider = &stubOTPProvider{err: errors.New("gateway down"), failChannel: "whatsapp"}

	for i := 0; i < 3; i++ {
		if i > 0 {
			mr.FastForward(otpSendfailPause + time.Second)
		}
		// The register/login path swallows the send failure, so drive the
		// failures through the resend endpoint which surfaces them.
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
			t.Fatalf("whatsapp failure %d: want ErrEmailDeliveryFailed, got %v", i+1, err)
		}
	}
	if !mr.Exists(otpSendfailKey("u1", "whatsapp")) {
		t.Error("whatsapp sendfail counter should exist after 3 failures")
	}
	if mr.Exists(otpSendfailKey("u1", "email")) {
		t.Error("email channel must not be flagged by whatsapp failures")
	}
	// Step past the pause the last failure armed. The email channel's claim
	// then passes while whatsapp stays gated.
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.claimOTPSend(ctx, loginOTPSendKeys("u1", "email", "1.1.1.1")); err != nil {
		t.Errorf("email claim after whatsapp sendfail: want nil, got %v", err)
	}
	if err := svc.claimOTPSend(ctx, loginOTPSendKeys("u1", "whatsapp", "1.1.1.1")); !errors.Is(err, ErrOTPRateLimit) {
		t.Errorf("whatsapp claim: want ErrOTPRateLimit, got %v", err)
	}
}

// TestResend_FailedSendKeepsPreviousCode: a failed resend must not destroy the
// still-delivered code. The new code lives in a pending key until the provider
// accepts the send; only then does it replace the live one.
func TestResend_FailedSendKeepsPreviousCode(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc, mr := newTestService(t, repo)
	otp := &stubOTPProvider{}
	svc.otpProvider = otp

	chall, err := svc.Register(ctx, "pending@example.com", "password123", "Budi", "1.1.1.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	uid, _ := mr.Get("pending:" + chall.PendingToken)
	code1 := otp.codes[0]
	storedBefore, _ := mr.Get("otp:" + uid)

	otp.err = errors.New("smtp down")
	mr.FastForward(time.Minute + time.Second)
	if err := svc.SendOTP(ctx, uid, "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
		t.Fatalf("resend: want ErrEmailDeliveryFailed, got %v", err)
	}
	if live, _ := mr.Get("otp:" + uid); live != storedBefore {
		t.Errorf("stored value changed after a failed resend: %q -> %q", storedBefore, live)
	}
	if mr.Exists("otp:new:" + uid) {
		t.Error("pending code key must be discarded when the send fails")
	}

	// Recovery: the resend succeeds, promotes a fresh code, and only that
	// code verifies afterwards.
	otp.err = nil
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.SendOTP(ctx, uid, "1.1.1.1"); err != nil {
		t.Fatalf("resend after recovery: %v", err)
	}
	code2 := otp.codes[len(otp.codes)-1]
	if code2 == code1 {
		t.Error("resend must generate a fresh code")
	}
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, code1, "1.1.1.1"); !errors.Is(err, ErrInvalidOTP) {
		t.Errorf("old code after successful resend: %v, want ErrInvalidOTP", err)
	}
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, code2, "1.1.1.1"); err != nil {
		t.Errorf("new code after successful resend: %v, want nil", err)
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
		if err := svc.ResetPassword(ctx, "tok", "654321", "brandnewpass", "1.1.1.1"); err != nil {
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
		if err := svc.ResetPassword(ctx, "tok", "000000", "brandnewpass", "1.1.1.1"); !errors.Is(err, ErrInvalidResetToken) {
			t.Errorf("want ErrInvalidResetToken, got %v", err)
		}
	})

	t.Run("invalid reset token", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc, _ := newTestService(t, repo)
		if err := svc.ResetPassword(ctx, "missing", "654321", "brandnewpass", "1.1.1.1"); !errors.Is(err, ErrInvalidResetToken) {
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

	access, refresh, _, err := svc.Login(ctx, "user@example.com", "password123", "1.1.1.1")
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
	access, refresh, _, err := svc.Login(ctx, "user@example.com", "oldpassword", "1.1.1.1")
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
	if err := svc.ResetPassword(ctx, "tok", "654321", "brandnewpass", "1.1.1.1"); err != nil {
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

			accessA, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword", "1.1.1.1")
			if err != nil {
				t.Fatalf("login A: %v", err)
			}
			claimsA, err := signer.ParseAccess(accessA)
			if err != nil {
				t.Fatalf("parse A: %v", err)
			}
			jtiA := claimsA.ID

			accessB, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword", "1.1.1.1")
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

	accessA, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword", "1.1.1.1")
	if err != nil {
		t.Fatalf("login A: %v", err)
	}
	claimsA, err := signer.ParseAccess(accessA)
	if err != nil {
		t.Fatalf("parse A: %v", err)
	}
	accessB, _, _, err := svc.Login(ctx, "user@example.com", "oldpassword", "1.1.1.1")
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

// --- OTP send quotas (T2: layered limits, konsep-otp.md §6) ---

func seedActiveOTP(t *testing.T) (*Service, *miniredis.Miniredis, *fakeUserRepo) {
	t.Helper()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "active", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	return svc, mr, repo
}

func TestOTPQuotaHourPerTarget(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)

	for i := 0; i < 3; i++ {
		if i > 0 {
			mr.FastForward(2*time.Minute + time.Second)
		}
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	if got, _ := mr.Get("otp:hr:u1"); got != "3" {
		t.Fatalf("otp:hr:u1 = %q, want 3", got)
	}
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("send 4: want OTPRateLimitError, got %v", err)
	}
	if rl.RetryAfter < 15*time.Minute {
		t.Errorf("RetryAfter = %v, want the hour-window remainder", rl.RetryAfter)
	}
	if got, _ := mr.Get("otp:hr:u1"); got != "3" {
		t.Errorf("denied send consumed the quota: hr=%q", got)
	}
}

func TestOTPQuotaDayPerTarget(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	svc.cfg.OTPQuotaDay = 2

	for i := 0; i < 2; i++ {
		if i > 0 {
			mr.FastForward(2*time.Minute + time.Second)
		}
		if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("send 3: want OTPRateLimitError, got %v", err)
	}
	if rl.RetryAfter < 22*time.Hour {
		t.Errorf("RetryAfter = %v, want the day-window remainder", rl.RetryAfter)
	}
}

func TestOTPQuotaPerIP(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	svc.cfg.OTPQuotaIPHour = 2

	for i := 0; i < 2; i++ {
		if i > 0 {
			mr.FastForward(2*time.Minute + time.Second)
		}
		if err := svc.SendOTP(ctx, "u1", "9.9.9.9"); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	if got, _ := mr.Get("otp:ip:9.9.9.9"); got != "2" {
		t.Fatalf("otp:ip:9.9.9.9 = %q, want 2", got)
	}
	err := svc.SendOTP(ctx, "u1", "9.9.9.9")
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("send 3: want OTPRateLimitError, got %v", err)
	}
	// Clear the ladder cooldown the last success armed, then try a different
	// source IP — it is unaffected.
	mr.FastForward(2*time.Minute + time.Second)
	if err := svc.SendOTP(ctx, "u1", "7.7.7.7"); err != nil {
		t.Errorf("send from a fresh IP: want nil, got %v", err)
	}
}

func TestOTPQuotaGateRejectionsNotConsumed(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	svc.cfg.OTPQuotaIPHour = 1

	if err := svc.SendOTP(ctx, "u1", "9.9.9.9"); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	mr.FastForward(2*time.Minute + time.Second)

	for i := 0; i < 2; i++ {
		err := svc.SendOTP(ctx, "u1", "9.9.9.9")
		if !errors.Is(err, ErrOTPRateLimit) {
			t.Fatalf("rejected attempt %d: want OTPRateLimitError, got %v", i+1, err)
		}
		mr.FastForward(2*time.Minute + time.Second)
	}
	if got, _ := mr.Get("otp:hr:u1"); got != "1" {
		t.Errorf("hr counter after 2 rejections = %q, want 1 (rejections consume nothing)", got)
	}
	if got, _ := mr.Get("otp:count:u1"); got != "1" {
		t.Errorf("level after 2 rejections = %q, want 1 (rejections do not climb the ladder)", got)
	}
	if got, _ := mr.Get("otp:abuse:u1"); got != "2" {
		t.Errorf("otp:abuse:u1 = %q, want 2", got)
	}
}

func TestOTPQuotaWindowsDoNotRefresh(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	svc.cfg.OTPQuotaHour = 2

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	mr.FastForward(2*time.Minute + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("send 2: %v", err)
	}
	ttl1 := mr.TTL("otp:hr:u1")
	if ttl1 <= 0 {
		t.Fatalf("hr window TTL = %v, want set", ttl1)
	}
	mr.FastForward(5 * time.Second)
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	var rl *OTPRateLimitError
	if errors.As(err, &rl) {
		ttl2 := mr.TTL("otp:hr:u1")
		if d := time.Duration(ttl1-5*time.Second) - time.Duration(ttl2); d < -time.Second || d > time.Second {
			t.Errorf("window TTL moved from %v (after 5s) to %v — a fixed window must not extend on requests/denials", ttl1-5*time.Second, ttl2)
		}
		return
	}
	t.Fatalf("send 3: want the hr quota denial, got %v", err)
}

func TestOTPQuotaFailureDoesNotConsume(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	otp := &stubOTPProvider{err: errors.New("smtp down")}
	svc.otpProvider = otp

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); !errors.Is(err, ErrEmailDeliveryFailed) {
		t.Fatalf("failed send: want ErrEmailDeliveryFailed, got %v", err)
	}
	otp.err = nil
	mr.FastForward(otpSendfailPause + time.Second)
	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err != nil {
		t.Fatalf("recovery send: %v", err)
	}
	if got, _ := mr.Get("otp:hr:u1"); got != "1" {
		t.Errorf("hr counter after failed+successful send = %q, want 1 — the failed claim must roll back", got)
	}
	if got, _ := mr.Get("otp:count:u1"); got != "1" {
		t.Errorf("level = %q, want 1 — a failed claim must not climb the ladder", got)
	}
	// The successful send stands at the FIRST ladder step.
	err := svc.SendOTP(ctx, "u1", "1.1.1.1")
	var rl *OTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("next send: want OTPRateLimitError, got %v", err)
	}
	if rl.RetryAfter < time.Minute-time.Second || rl.RetryAfter > time.Minute {
		t.Errorf("next-send RetryAfter = %v, want ~%v", rl.RetryAfter, time.Minute)
	}
}

// --- OTP verify hardening (T3: vfail budgets, fail-closed) ---

func TestOTPVerify_VfailSurvivesResend(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	otp := &stubOTPProvider{}
	svc.otpProvider = otp
	svc.cfg.OTPVfailHour = 3
	svc.cfg.OTPVfailDay = 30

	uid := "u1"
	pending := func() string { return mustPending(mr, uid) }

	// Three wrong entries kill the code and raise vfail to 3.
	pendingToken := pending()
	mr.Set("otp:u1", "111111")
	for i := 1; i <= 3; i++ {
		_, _, err := svc.VerifyOTP(ctx, pendingToken, fmt.Sprintf("wrong-%d", i), "1.1.1.1")
		if i < 3 && !errors.Is(err, ErrInvalidOTP) {
			t.Fatalf("wrong #%d: want ErrInvalidOTP, got %v", i, err)
		}
		if i == 3 && !errors.Is(err, ErrOTPAttemptsExceeded) {
			t.Fatalf("wrong #3: want ErrOTPAttemptsExceeded, got %v", i)
		}
	}
	if got, _ := mr.Get("otp:vfail:u1"); got != "3" {
		t.Fatalf("otp:vfail:u1 = %q, want 3", got)
	}

	// A successful resend resets the per-code budget but NOT the vfail gate.
	mr.FastForward(2*time.Minute + time.Second)
	if err := svc.SendOTP(ctx, uid, "1.1.1.1"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	code2 := otp.codes[len(otp.codes)-1]
	pendingToken = pending()
	// Even the CORRECT new code is gated by the surviving vfail counter —
	// and the gate must not consume the code.
	_, _, err := svc.VerifyOTP(ctx, pendingToken, code2, "1.1.1.1")
	if !errors.Is(err, ErrOTPRateLimit) {
		t.Fatalf("gated verify: want ErrOTPRateLimit, got %v", err)
	}
	if !mr.Exists("otp:u1") {
		t.Error("the gate must not consume the code")
	}
}

func TestOTPVerify_VfailPerIPShared(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("u1@example.com"), PasswordHash: mustHashStd("password123"), Role: RoleStudent, Status: "pending_verification", OTPEnabled: true})
	repo.seed(&model.User{ID: "u2", Email: strptr("u2@example.com"), PasswordHash: mustHashStd("password123"), Role: RoleStudent, Status: "pending_verification", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	svc.cfg.OTPVfailIPHour = 2

	otp := &stubOTPProvider{}
	svc.otpProvider = otp
	svc.SendOTP(ctx, "u1", "9.9.9.9")
	svc.SendOTP(ctx, "u2", "7.7.7.7")
	code1 := otp.codes[0]
	code2 := otp.codes[1]
	tok1 := mustPending(mr, "u1")
	tok2 := mustPending(mr, "u2")

	// Two wrong entries from one IP raise the per-IP counter to the limit.
	for i := 0; i < 2; i++ {
		if _, _, err := svc.VerifyOTP(ctx, tok1, fmt.Sprintf("bad-%d", i), "9.9.9.9"); !errors.Is(err, ErrInvalidOTP) {
			t.Fatalf("wrong #%d: want ErrInvalidOTP, got %v", i+1, err)
		}
	}
	if got, _ := mr.Get("otp:vfail:ip:9.9.9.9"); got != "2" {
		t.Fatalf("otp:vfail:ip:9.9.9.9 = %q, want 2", got)
	}
	// Another user with a CORRECT code, same IP: gated.
	if _, _, err := svc.VerifyOTP(ctx, tok2, code2, "9.9.9.9"); !errors.Is(err, ErrOTPRateLimit) {
		t.Fatalf("second user verify from flagged IP: want ErrOTPRateLimit, got %v", err)
	}
	// A different IP is unaffected.
	if _, _, err := svc.VerifyOTP(ctx, tok2, code2, "3.3.3.3"); err != nil {
		t.Fatalf("second user verify from a fresh IP: want nil, got %v", err)
	}
	_ = code1
}

func TestOTPVerify_SuccessClearsGateState(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := seedActiveOTP(t)
	otp := &stubOTPProvider{}
	svc.otpProvider = otp

	// Register arms cooldown+level; the code verifies — all gate state must go.
	chall, err := svc.Register(ctx, "gatestate@example.com", "password123", "Budi", "1.1.1.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	uid, _ := mr.Get("pending:" + chall.PendingToken)
	if got, _ := mr.Get("otp:count:" + uid); got == "" {
		t.Fatal("setup: level should exist after a send")
	}
	// One wrong entry first, so vfail has state to clear.
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, "000000", "1.1.1.1"); !errors.Is(err, ErrInvalidOTP) {
		t.Fatalf("wrong entry: %v", err)
	}
	code := otp.codes[0]
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, code, "1.1.1.1"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, key := range []string{
		"otp:cooldown:" + uid, "otp:count:" + uid, "otp:verify:" + uid,
		"otp:vfail:" + uid, "otp:vfail:day:" + uid,
	} {
		if mr.Exists(key) {
			t.Errorf("%s should be cleared by a successful verification", key)
		}
	}
}

func TestOTPVerify_FailClosedRedisDown(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	repo.seed(&model.User{ID: "u1", Email: strptr("user@example.com"), Role: RoleStudent, Status: "pending_verification", OTPEnabled: true})
	svc, mr := newTestService(t, repo)
	mr.Close()

	if err := svc.SendOTP(ctx, "u1", "1.1.1.1"); err == nil {
		t.Error("SendOTP with dead redis must fail closed")
	}
	if _, err := svc.Register(ctx, "down@example.com", "password123", "Budi", "1.1.1.1"); err == nil {
		t.Error("Register with dead redis must fail closed, not silently send")
	}
}

func mustPending(mr *miniredis.Miniredis, uid string) string {
	tok := "ptok-" + uid
	mr.Set("pending:"+tok, uid)
	return tok
}

// --- T4: OTP codes are stored as HMAC hex, never plaintext ---

func TestOTPStoredHashed_NotPlaintext(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc, mr := newTestService(t, repo)
	otp := &stubOTPProvider{}
	svc.otpProvider = otp

	chall, err := svc.Register(ctx, "hash@example.com", "password123", "Budi", "1.1.1.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	u, _ := repo.GetUserByEmail(ctx, "hash@example.com")
	stored, _ := mr.Get("otp:" + u.ID)
	if stored == otp.codes[0] {
		t.Error("plaintext code must not be stored in redis")
	}
	if len(stored) != 64 {
		t.Errorf("stored value = %q (len %d), want 64 hex chars (HMAC-SHA256)", stored, len(stored))
	}
	if _, err := hex.DecodeString(stored); err != nil {
		t.Errorf("stored value is not hex: %v", err)
	}
	// The hash still verifies: the captured code works, a wrong one does not.
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, "000000", "1.1.1.1"); !errors.Is(err, ErrInvalidOTP) {
		t.Errorf("wrong code: want ErrInvalidOTP, got %v", err)
	}
	if _, _, err := svc.VerifyOTP(ctx, chall.PendingToken, otp.codes[0], "1.1.1.1"); err != nil {
		t.Errorf("captured code: want nil, got %v", err)
	}
}
