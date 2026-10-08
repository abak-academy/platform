package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"akademi-bimbel/internal/metrics"
	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// comparePassword times the bcrypt comparison. Its latency is the dominant
// per-login CPU cost — the capacity model (#62/#98 §3) predicts ~234 ms/op on
// dev hardware and a hard ceiling of concurrent logins on the 2-vCPU VM, so
// the real distribution gets measured, not assumed. op distinguishes the two
// call sites: the capacity model quotes the login series only.
func comparePassword(hash, password, op string) bool {
	start := time.Now()
	ok := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	metrics.ObservePasswordVerify(op, time.Since(start))
	return ok
}

type UserRepository interface {
	Ping(ctx context.Context) error
	CreateUser(ctx context.Context, u *model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]model.User, error)
	UpdatePasswordHash(ctx context.Context, userID, hash string) error
	UpdateUserProfile(ctx context.Context, userID string, name, email, username, phone, address, targetExam *string, grade *int, dob *time.Time, applySchool bool, schoolID *string, unlistedSchoolName *string, jenjang *string, provinsiID, kotaID, kecamatanID, kodePos *string) error
	UpdateUserPhoto(ctx context.Context, userID, photoURL string) error
	ActivateUser(ctx context.Context, userID string) (bool, error)
	TombstoneUser(ctx context.Context, userID string) error
}

const minPasswordLen = 8

// OTPChallenge is the outcome of arming — or deliberately skipping — an OTP
// dispatch for a pending account.
type OTPChallenge struct {
	PendingToken string
	EmailFailed  bool // a send was attempted and failed
	OTPActive    bool // a live code already existed — nothing was sent
	ResendAfter  int  // seconds until the next send is allowed; 0 = now
}

func (s *Service) Register(ctx context.Context, email, password, name, ip string) (OTPChallenge, error) {
	email = normalizeEmail(email)
	if email == "" {
		return OTPChallenge{}, ErrInvalidCredentials
	}
	if len(password) < minPasswordLen {
		return OTPChallenge{}, ErrWeakPassword
	}
	name = strings.TrimSpace(stripFormatRunes(name))
	if name == "" {
		return OTPChallenge{}, ErrInvalidCredentials
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return OTPChallenge{}, err
	}
	if existing != nil {
		if existing.Status == "pending_verification" {
			return s.startOTPChallenge(ctx, existing, ip)
		}
		return OTPChallenge{}, ErrEmailTaken
	}

	hash, err := hashPassword(password)
	if err != nil {
		return OTPChallenge{}, err
	}
	username, err := s.generateUniqueUsername(ctx, name)
	if err != nil {
		return OTPChallenge{}, err
	}
	user := &model.User{
		Email:        &email,
		Username:     &username,
		PasswordHash: string(hash),
		Role:         RoleStudent,
		Name:         name,
		Status:       "pending_verification",
		OTPEnabled:   true,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return OTPChallenge{}, err
	}
	return s.startOTPChallenge(ctx, user, ip)
}

func (s *Service) Login(ctx context.Context, identifier, password, ip string) (accessToken, refreshToken, pendingToken string, err error) {
	user, err := s.lookupByIdentifier(ctx, identifier)
	if err != nil {
		return "", "", "", err
	}
	if user == nil {
		return "", "", "", ErrInvalidCredentials
	}
	if !comparePassword(user.PasswordHash, password, metrics.OpLogin) {
		return "", "", "", ErrInvalidCredentials
	}
	switch user.Status {
	case "active":
		accessToken, refreshToken, err = s.mintSession(ctx, user)
		return accessToken, refreshToken, "", err
	case "pending_verification":
		challenge, err := s.startOTPChallenge(ctx, user, ip)
		if err != nil {
			return "", "", "", err
		}
		return "", "", challenge.PendingToken, &VerificationPendingError{
			EmailFailed: challenge.EmailFailed,
			OTPActive:   challenge.OTPActive,
			ResendAfter: challenge.ResendAfter,
		}
	default:
		return "", "", "", ErrInvalidCredentials
	}
}

func (s *Service) lookupByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	identifier = strings.TrimSpace(identifier)
	if strings.Contains(identifier, "@") {
		return s.repo.GetUserByEmail(ctx, identifier)
	}
	user, err := s.repo.GetUserByUsername(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return s.repo.GetUserByEmail(ctx, identifier)
}

// startOTPChallenge arms (or deliberately skips) an OTP dispatch. A live code
// means the previous dispatch is still inside the user's mailbox window:
// re-sending would burn relay quota and trip the ladder for nothing, so only
// the pending token is re-armed and the user is told a code is already
// waiting. The throttle gates sends — never verification.
func (s *Service) startOTPChallenge(ctx context.Context, user *model.User, ip string) (OTPChallenge, error) {
	if exists, err := s.rdb.Exists(ctx, "otp:"+user.ID).Result(); err == nil && exists > 0 {
		pendingToken, err := s.storePendingToken(ctx, user.ID)
		if err != nil {
			return OTPChallenge{}, err
		}
		return OTPChallenge{
			PendingToken: pendingToken,
			OTPActive:    true,
			ResendAfter:  s.resendAfter(ctx, user.ID),
		}, nil
	}

	channel, destination := PreferredOTPChannel(deref(user.Phone), deref(user.Email))
	keys := loginOTPSendKeys(user.ID, channel, ip)
	if err := s.claimOTPSend(ctx, keys); err != nil {
		if errors.Is(err, ErrOTPRateLimit) {
			s.recordOTPGateReject(ctx, keys)
		}
		return OTPChallenge{}, err
	}
	pendingToken, err := s.storePendingToken(ctx, user.ID)
	if err != nil {
		return OTPChallenge{}, err
	}
	code, err := genOTP()
	if err != nil {
		return OTPChallenge{}, err
	}
	stored := s.hashOTP(otpPurposeLogin, code)
	// The new code waits in a pending key so a provider failure cannot destroy
	// the last delivered code.
	if err := s.rdb.Set(ctx, otpNewKey(user.ID), stored, s.cfg.OTPTTL).Err(); err != nil {
		return OTPChallenge{}, err
	}
	// A failed provider send is not fatal: the account stays pending and the
	// caller reports email_failed so the client can offer the resend path.
	if err := s.otpProvider.SendOTP(ctx, channel, destination, code); err != nil {
		s.rdb.Del(ctx, otpNewKey(user.ID))
		s.rollbackOTPClaim(ctx, keys)
		s.recordOTPSendFailure(ctx, user.ID, channel)
		metrics.ObserveOTPSend("delivery_failed")
		return OTPChallenge{PendingToken: pendingToken, EmailFailed: true, ResendAfter: s.resendAfter(ctx, user.ID)}, nil
	}
	s.promoteOTP(ctx, user.ID, channel, stored)
	metrics.ObserveOTPSend("sent")
	return OTPChallenge{PendingToken: pendingToken, ResendAfter: s.resendAfter(ctx, user.ID)}, nil
}

func (s *Service) storePendingToken(ctx context.Context, userID string) (string, error) {
	pendingToken := newToken(userID)
	if err := s.rdb.Set(ctx, "pending:"+pendingToken, userID, s.cfg.OTPTTL).Err(); err != nil {
		return "", err
	}
	return pendingToken, nil
}

// resendAfter reports the remaining send cooldown in whole seconds; 0 when the
// next send is free.
func (s *Service) resendAfter(ctx context.Context, userID string) int {
	ttl, err := s.rdb.TTL(ctx, otpCooldownKey(userID)).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return int(math.Ceil(ttl.Seconds()))
}

// dispatchOTP arms a fresh code in the pending key and sends it, treating
// provider failure as ErrEmailDeliveryFailed. Used by the resend endpoint,
// which surfaces the failure; the register/login paths swallow it inside
// startOTPChallenge instead. The previous code stays verifiable until the
// provider accepts the new one.
func (s *Service) dispatchOTP(ctx context.Context, user *model.User, keys otpSendKeys, ip string) error {
	code, err := genOTP()
	if err != nil {
		return err
	}
	stored := s.hashOTP(otpPurposeLogin, code)
	if err := s.rdb.Set(ctx, otpNewKey(user.ID), stored, s.cfg.OTPTTL).Err(); err != nil {
		return err
	}
	channel, destination := PreferredOTPChannel(deref(user.Phone), deref(user.Email))
	if err := s.otpProvider.SendOTP(ctx, channel, destination, code); err != nil {
		s.rdb.Del(ctx, otpNewKey(user.ID))
		s.rollbackOTPClaim(ctx, keys)
		s.recordOTPSendFailure(ctx, user.ID, channel)
		metrics.ObserveOTPSend("delivery_failed")
		return &EmailDeliveryError{RetryAfter: s.otpSendfailPause(), Err: err}
	}
	s.promoteOTP(ctx, user.ID, channel, stored)
	metrics.ObserveOTPSend("sent")
	return nil
}

// promoteOTP publishes a successfully sent code hash: it replaces the live
// code with a full TTL, resets the verification budget (a fresh code means a
// fresh budget), and clears the sendfail streak.
func (s *Service) promoteOTP(ctx context.Context, userID, channel, stored string) error {
	if err := s.rdb.Set(ctx, "otp:"+userID, stored, s.cfg.OTPTTL).Err(); err != nil {
		return err
	}
	s.rdb.Del(ctx, otpNewKey(userID), otpVerifyKey(userID), otpSendfailKey(userID, channel))
	return nil
}

func (s *Service) ResolveUserFromPendingToken(ctx context.Context, pendingToken string) (string, error) {
	userID, err := s.rdb.Get(ctx, "pending:"+pendingToken).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrInvalidPendingToken
	}
	return userID, err
}

func (s *Service) ResolveUserFromEmail(ctx context.Context, email string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", ErrUserNotFound
	}
	return user.ID, nil
}

func (s *Service) SendOTP(ctx context.Context, userID, ip string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	channel, _ := PreferredOTPChannel(deref(user.Phone), deref(user.Email))
	keys := loginOTPSendKeys(userID, channel, ip)
	if err := s.claimOTPSend(ctx, keys); err != nil {
		if errors.Is(err, ErrOTPRateLimit) {
			s.recordOTPGateReject(ctx, keys)
		}
		return err
	}
	return s.dispatchOTP(ctx, user, keys, ip)
}

// otpResendLadder arms the cooldown after each successful send: 60s after the
// first, then 2m, 5m, 15m, and a 1h cap for every further send (konsep §7,
// free tier starting at 60s). otpLadderWindow is the fixed window the level
// count lives in — after an hour without a send it expires and the ladder
// resets. otpSendfailWindow bounds consecutive delivery failures: once
// maxOTPFailures fail inside one window, sends pause until it expires.
var (
	otpResendLadder  = [5]time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	otpLadderWindow  = time.Hour
	otpSendfailPause = 30 * time.Second
	otpSendfailMax   = 3
	// Sends gating on otpSendfailWindow: otpSendfailMax delivery failures
	// inside one window stop sending until the window expires.
	otpSendfailWindow = time.Hour
	// Wrong verification entries are budgeted per code: three strikes kill the
	// code so an online guesser can't grind through the 6-digit space inside
	// its TTL.
	maxOTPVerifyAttempts = 3
)

func otpCooldownKey(userID string) string          { return "otp:cooldown:" + userID }
func otpCountKey(userID string) string             { return "otp:count:" + userID }
func otpVerifyKey(userID string) string            { return "otp:verify:" + userID }
func otpNewKey(userID string) string               { return "otp:new:" + userID }
func otpSendfailKey(userID, channel string) string { return "otp:sendfail:" + userID + ":" + channel }

// recordOTPSendFailure bumps the channel's delivery-failure counter and
// replaces the cooldown with the fixed short pause — provider failure delays
// the next retry by otpSendfailPause, never the ladder.
func (s *Service) recordOTPSendFailure(ctx context.Context, userID, channel string) {
	n, err := s.rdb.Incr(ctx, otpSendfailKey(userID, channel)).Result()
	if err == nil && n == 1 {
		s.rdb.Expire(ctx, otpSendfailKey(userID, channel), otpSendfailWindow)
	}
	s.rdb.Set(ctx, otpCooldownKey(userID), "1", otpSendfailPause)
}

func (s *Service) VerifyOTP(ctx context.Context, pendingToken, code, ip string) (accessToken, refreshToken string, err error) {
	userID, err := s.rdb.Get(ctx, "pending:"+pendingToken).Result()
	if errors.Is(err, redis.Nil) {
		return "", "", ErrInvalidPendingToken
	}
	if err != nil {
		return "", "", err
	}

	// The gate script raises the attempt budget BEFORE the caller compares,
	// so parallel wrong guesses cannot share one budget (konsep §5).
	stored, attempts, deny := s.runOTPVerifyGate(ctx, userID, ip)
	if deny != nil {
		metrics.ObserveOTPVerify("denied")
		return "", "", deny
	}
	if stored == "" {
		metrics.ObserveOTPVerify("invalid")
		return "", "", ErrOTPExpired
	}
	if !otpEqualMAC(stored, s.hashOTP(otpPurposeLogin, code)) {
		// Wrong entries are budgeted per code: maxOTPVerifyAttempts strikes
		// kill the code so an online guesser can't grind through the 6-digit
		// space inside its TTL. The hour/day/IP vfail counters survive a
		// resend and bound total guessing.
		s.recordOTPVerifyFailure(ctx, userID, ip)
		if attempts >= int64(s.otpAttempts()) {
			s.rdb.Del(ctx, "otp:"+userID)
			metrics.ObserveOTPVerify("attempts_exceeded")
			return "", "", ErrOTPAttemptsExceeded
		}
		metrics.ObserveOTPVerify("invalid")
		return "", "", ErrInvalidOTP
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if user == nil {
		return "", "", ErrUserNotFound
	}
	activated, err := s.repo.ActivateUser(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if !activated {
		return "", "", ErrInvalidPendingToken
	}
	metrics.ObserveOTPVerify("ok")
	// Success clears the OTP and every gate state it accumulated (konsep §5).
	vk := loginOTPVerifyKeys(userID, ip)
	s.rdb.Del(ctx,
		"otp:"+userID, "pending:"+pendingToken, otpVerifyKey(userID), otpNewKey(userID),
		otpCooldownKey(userID), otpCountKey(userID),
		vk.vfailHour, vk.vfailDay, vk.vfailIP,
	)
	return s.mintSession(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (newAccessToken, newRefreshToken string, err error) {
	userID, err := s.rdb.Get(ctx, "session:refresh:"+refreshToken).Result()
	if errors.Is(err, redis.Nil) {
		return "", "", ErrInvalidRefreshToken
	}
	if err != nil {
		return "", "", err
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if user == nil || user.Status != "active" {
		return "", "", ErrInvalidRefreshToken
	}

	s.rdb.Del(ctx, "session:refresh:"+refreshToken)
	return s.mintSession(ctx, user)
}

func (s *Service) Logout(ctx context.Context, jti, refreshToken string) error {
	userID, _ := s.rdb.Get(ctx, "session:access:"+jti).Result()
	if err := s.rdb.Del(ctx, "session:access:"+jti).Err(); err != nil {
		return err
	}
	if refreshToken != "" {
		s.rdb.Del(ctx, "session:refresh:"+refreshToken)
	}
	if userID != "" {
		s.rdb.SRem(ctx, "user_access_sessions:"+userID, jti)
		if refreshToken != "" {
			s.rdb.SRem(ctx, "user_refresh_sessions:"+userID, refreshToken)
		}
	}
	return nil
}

func (s *Service) Me(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *Service) ForgotPassword(ctx context.Context, email, ip string) error {
	email = normalizeEmail(email)
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		// No enumeration: pretend success when the email is unknown.
		return nil
	}
	keys := passwordResetSendKeys(email, ip)
	if err := s.claimOTPSend(ctx, keys); err != nil {
		if errors.Is(err, ErrOTPRateLimit) {
			s.recordOTPGateReject(ctx, keys)
		}
		return err
	}

	code, err := genOTP()
	if err != nil {
		return err
	}
	token := newToken(user.ID)
	stored := s.hashOTP(otpPurposeReset, code)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, "reset:"+token, map[string]interface{}{
		"uid":      user.ID,
		"hash":     stored,
		"attempts": 0,
	})
	pipe.Expire(ctx, "reset:"+token, s.cfg.OTPTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	body := fmt.Sprintf("Your password reset code is %s. Token: %s", code, token)
	if err := s.emailProvider.SendEmail(ctx, deref(user.Email), "Password reset", body); err != nil {
		s.rollbackOTPClaim(ctx, keys)
		s.recordOTPSendFailureOn(ctx, keys)
		metrics.ObserveOTPSend("delivery_failed")
		return &EmailDeliveryError{RetryAfter: s.otpSendfailPause(), Err: err}
	}
	s.rdb.Del(ctx, keys.sendfail)
	metrics.ObserveOTPSend("sent")
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token, otp, newPassword, ip string) error {
	// The gate script raises the attempt budget before the hash comparison,
	// so parallel guesses share one budget (konsep §5), and the reset vfail
	// budgets survive across reset tokens.
	userID, stored, attempts, deny := s.runResetVerifyGate(ctx, token, ip)
	if deny != nil {
		metrics.ObserveOTPVerify("denied")
		return deny
	}
	if userID == "" {
		metrics.ObserveOTPVerify("invalid")
		return ErrInvalidResetToken
	}
	if !otpEqualMAC(stored, s.hashOTP(otpPurposeReset, otp)) {
		s.recordResetVerifyFailure(ctx, userID, ip)
		if attempts >= int64(s.otpAttempts()) {
			s.rdb.Del(ctx, "reset:"+token)
		}
		return ErrInvalidResetToken
	}
	if len(newPassword) < minPasswordLen {
		return ErrWeakPassword
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePasswordHash(ctx, userID, string(hash)); err != nil {
		return err
	}
	// Success clears the token and the accumulated wrong-entry budget.
	vk := resetVerifyKeys(userID, ip)
	s.rdb.Del(ctx, "reset:"+token, vk.vfailHour, vk.vfailDay, vk.vfailIP)
	s.revokeAllSessions(ctx, userID)
	return nil
}

// revokeAllSessions kills every session for userID. With exceptJTI supplied,
// the matching access jti is spared and stays a member of
// user_access_sessions:<uid> (removed jtis are SRem'd individually, not
// deleted via a wholesale Del on the index set) — otherwise a later
// ResetPassword/AdminResetAccountPassword would fail to find and kill the
// surviving session. Refresh tokens have no link back to an access jti
// (mintSession stores them in separate, unpaired sets), so every refresh
// token is always revoked, the caller's included (FR-31).
func (s *Service) revokeAllSessions(ctx context.Context, userID string, exceptJTI ...string) {
	var keep string
	if len(exceptJTI) > 0 {
		keep = exceptJTI[0]
	}

	jtis, _ := s.rdb.SMembers(ctx, "user_access_sessions:"+userID).Result()
	for _, jti := range jtis {
		if jti == keep {
			continue
		}
		s.rdb.Del(ctx, "session:access:"+jti)
		s.rdb.SRem(ctx, "user_access_sessions:"+userID, jti)
	}
	if keep == "" {
		s.rdb.Del(ctx, "user_access_sessions:"+userID)
	}

	s.revokeRefreshSessions(ctx, userID)
}

func (s *Service) revokeRefreshSessions(ctx context.Context, userID string) {
	refreshTokens, _ := s.rdb.SMembers(ctx, "user_refresh_sessions:"+userID).Result()
	for _, rt := range refreshTokens {
		s.rdb.Del(ctx, "session:refresh:"+rt)
	}
	s.rdb.Del(ctx, "user_refresh_sessions:"+userID)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword, callerJTI string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if !comparePassword(user.PasswordHash, currentPassword, metrics.OpChangePassword) {
		return ErrInvalidCredentials
	}
	if len(newPassword) < minPasswordLen {
		return ErrWeakPassword
	}
	if newPassword == currentPassword {
		return ErrWeakPassword
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePasswordHash(ctx, userID, string(hash)); err != nil {
		return err
	}
	s.revokeAllSessions(ctx, userID, callerJTI)
	return nil
}

func (s *Service) SessionActive(ctx context.Context, jti string) bool {
	exists, _ := s.rdb.Exists(ctx, "session:access:"+jti).Result()
	return exists > 0
}

func (s *Service) mintSession(ctx context.Context, user *model.User) (accessToken, refreshToken string, err error) {
	caps := Capabilities(user.Role)
	tokenString, jti, err := s.jwtSigner.SignAccess(user.ID, user.Role, user.SchoolID, caps)
	if err != nil {
		return "", "", err
	}
	if err := s.rdb.Set(ctx, "session:access:"+jti, user.ID, s.cfg.AccessTokenTTL).Err(); err != nil {
		return "", "", err
	}
	refreshToken = fmt.Sprintf("%s-%d", user.ID, time.Now().UnixNano())
	if err := s.rdb.Set(ctx, "session:refresh:"+refreshToken, user.ID, s.cfg.RefreshTokenTTL).Err(); err != nil {
		return "", "", err
	}
	s.rdb.SAdd(ctx, "user_access_sessions:"+user.ID, jti)
	s.rdb.SAdd(ctx, "user_refresh_sessions:"+user.ID, refreshToken)
	return tokenString, refreshToken, nil
}

// hashOTP binds the code to its purpose and the server secret (konsep-otp.md
// §11): what sits in Redis is the hex HMAC, never the raw 6 digits.
const (
	otpPurposeLogin = "login"
	otpPurposeReset = "reset"
)

func (s *Service) hashOTP(purpose, code string) string {
	var secret []byte
	if s.cfg != nil {
		secret = []byte(s.cfg.OTPSecret)
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// otpEqualMAC compares two stored/derived HMAC hexes in constant time.
func otpEqualMAC(storedHash, submittedHash string) bool {
	return hmac.Equal([]byte(storedHash), []byte(submittedHash))
}

func genOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func newToken(userID string) string {
	return fmt.Sprintf("%s-%d", userID, time.Now().UnixNano())
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
