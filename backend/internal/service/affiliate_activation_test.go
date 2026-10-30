package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newAffiliateActivationService builds a real-DB service whose OTP provider
// records codes and whose JWT signer mints real sessions, so the
// Register → VerifyOTP and Google registration paths run end to end.
func newAffiliateActivationService(t *testing.T) (*Service, *repository.Repository, *RecordingOTPProvider) {
	t.Helper()
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{
		JWTSecret:       "affiliate-activation-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
		OTPSecret:       "affiliate-activation-otp-secret",
		GoogleClientID:  "google-client-id",
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	otp := NewRecordingOTPProvider()
	svc := NewWithStore(repo, repo, rdb, signer, otp, &NoopEmailProvider{}, nil, nil, nil, cfg, nil)
	return svc, repo, otp
}

// setAffiliateFlag writes the feature flag and deletes the key when the test
// ends, so the shared fixture never leaks an on/off state to later tests.
func setAffiliateFlag(t *testing.T, repo *repository.Repository, value string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`, value,
	); err != nil {
		t.Fatalf("set affiliate flag: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM system_config WHERE key = 'affiliate_enabled'`)
	})
}

func registerPendingStudent(t *testing.T, svc *Service, otp *RecordingOTPProvider, email string) (pendingToken, userID string) {
	t.Helper()
	ctx := context.Background()
	chall, err := svc.Register(ctx, email, "password123", "Ani", "1.1.1.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(otp.Codes) != 1 {
		t.Fatalf("OTP codes delivered = %d, want 1", len(otp.Codes))
	}
	id, err := svc.ResolveUserFromPendingToken(ctx, chall.PendingToken)
	if err != nil {
		t.Fatalf("resolve pending token: %v", err)
	}
	return chall.PendingToken, id
}

func seedPostLaunchAffiliateSettings(t *testing.T, repo *repository.Repository) {
	t.Helper()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
}

func TestVerifyOTP_ManualCodeCreatesBinding(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-votp-manual-"+uniqueSuffix()+"@test.local", "", "active")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-manual-"+uniqueSuffix()+"@test.local")
	access, refresh, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{ManualCode: owner.code})
	if err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatalf("want session tokens after activation")
	}
	binding := getActiveAffiliateBinding(t, repo, userID)
	if binding == nil {
		t.Fatal("want an active binding after OTP activation with a manual referral code")
	}
	if binding.AffiliatorID != owner.affiliatorID {
		t.Errorf("binding affiliator = %v, want %v", binding.AffiliatorID, owner.affiliatorID)
	}
	if binding.Source != affiliateSourceManualCode {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceManualCode)
	}
}

func TestVerifyOTP_CookieCodeCreatesLinkBinding(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-votp-cookie-"+uniqueSuffix()+"@test.local", "", "active")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-cookie-"+uniqueSuffix()+"@test.local")
	if _, _, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{CookieCode: owner.code}); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	binding := getActiveAffiliateBinding(t, repo, userID)
	if binding == nil {
		t.Fatal("want an active binding after OTP activation with a cookie code")
	}
	if binding.AffiliatorID != owner.affiliatorID {
		t.Errorf("binding affiliator = %v, want %v", binding.AffiliatorID, owner.affiliatorID)
	}
	if binding.Source != affiliateSourceLink {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceLink)
	}
}

func TestVerifyOTP_ManualCodeWinsOverCookie(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	manual := seedAffiliatorOwner(t, repo, "aff-votp-mwin-"+uniqueSuffix()+"@test.local", "", "active")
	cookie := seedAffiliatorOwner(t, repo, "aff-votp-cwin-"+uniqueSuffix()+"@test.local", "", "active")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-mwin-"+uniqueSuffix()+"@test.local")
	if _, _, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{ManualCode: manual.code, CookieCode: cookie.code}); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	binding := getActiveAffiliateBinding(t, repo, userID)
	if binding == nil {
		t.Fatal("want an active binding when manual and cookie codes are both sent")
	}
	if binding.AffiliatorID != manual.affiliatorID {
		t.Errorf("binding affiliator = %v, want the manual-code owner %v", binding.AffiliatorID, manual.affiliatorID)
	}
	if binding.Source != affiliateSourceManualCode {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceManualCode)
	}
	if n := countAffiliateBindings(t, repo, userID); n != 1 {
		t.Errorf("bindings created = %d, want 1", n)
	}
}

func TestVerifyOTP_FlagOffIgnoresReferral(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "false")
	owner := seedAffiliatorOwner(t, repo, "aff-votp-off-"+uniqueSuffix()+"@test.local", "", "active")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-off-"+uniqueSuffix()+"@test.local")
	if _, _, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{ManualCode: owner.code}); err != nil {
		t.Fatalf("VerifyOTP with flag off must still succeed: %v", err)
	}
	if n := countAffiliateBindings(t, repo, userID); n != 0 {
		t.Errorf("bindings created with flag off = %d, want 0", n)
	}
}

func TestVerifyOTP_BindingFailureKeepsActivation(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-votp-bad-"+uniqueSuffix()+"@test.local", "", "active")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-bad-"+uniqueSuffix()+"@test.local")
	// Break the binding insert without touching the activation path: the
	// affiliator's row vanishes between ResolveBinding and CreateBinding.
	if _, err := repo.Pool().Exec(ctx, `UPDATE affiliators SET program_status = 'suspended' WHERE user_id = $1`, owner.affiliatorID); err != nil {
		t.Fatalf("suspend affiliator: %v", err)
	}
	access, refresh, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{ManualCode: owner.code})
	if err != nil {
		t.Fatalf("activation must not fail when binding fails: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("want session tokens even when the binding path fails")
	}
	var status string
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, userID).Scan(&status); err != nil {
		t.Fatalf("read user: %v", err)
	}
	if status != "active" {
		t.Errorf("user status after failed binding = %q, want active", status)
	}
	if n := countAffiliateBindings(t, repo, userID); n != 0 {
		t.Errorf("bindings created = %d, want 0", n)
	}
}

func TestVerifyOTP_UnknownCodeStillActivatesWithoutBinding(t *testing.T) {
	svc, repo, otp := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")

	pending, userID := registerPendingStudent(t, svc, otp, "student-votp-unknown-"+uniqueSuffix()+"@test.local")
	if _, _, err := svc.VerifyOTP(ctx, pending, otp.Codes[0], "1.1.1.1", AffiliateReferral{ManualCode: "NOPE0000"}); err != nil {
		t.Fatalf("VerifyOTP with unknown code must still succeed: %v", err)
	}
	if n := countAffiliateBindings(t, repo, userID); n != 0 {
		t.Errorf("bindings created from an unknown code = %d, want 0", n)
	}
}

func TestGoogleLogin_BindsStudentAtRegistration(t *testing.T) {
	svc, repo, _ := newAffiliateActivationService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-google-"+uniqueSuffix()+"@test.local", "", "active")

	email := "gstudent-" + uniqueSuffix() + "@test.local"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(googleTokenInfo{
			Aud:           "google-client-id",
			Email:         email,
			EmailVerified: "true",
			Name:          "Google Student",
		})
	}))
	defer ts.Close()
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = &googleTokenInfoTransport{fakeURL: ts.URL}
	defer func() { http.DefaultClient.Transport = orig }()

	access, _, err := svc.GoogleLogin(ctx, "fake-id-token", AffiliateReferral{ManualCode: owner.code})
	if err != nil {
		t.Fatalf("GoogleLogin: %v", err)
	}
	if access == "" {
		t.Fatal("want an access token")
	}
	user, err := repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		t.Fatalf("google user must exist: %v", err)
	}
	binding := getActiveAffiliateBinding(t, repo, user.ID)
	if binding == nil {
		t.Fatal("want a binding created at google registration")
	}
	if binding.AffiliatorID != owner.affiliatorID {
		t.Errorf("binding affiliator = %v, want %v", binding.AffiliatorID, owner.affiliatorID)
	}
	if binding.Source != affiliateSourceManualCode {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceManualCode)
	}
}
