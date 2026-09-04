package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
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
	ListSchools(ctx context.Context) ([]*model.School, error)
	ActivateUser(ctx context.Context, userID string) (bool, error)
	TombstoneUser(ctx context.Context, userID string) error
}

const minPasswordLen = 8

// ValidateRequestTempPassword enforces the minimum length for the optional
// request-level temp password on student bulk register.
func ValidateRequestTempPassword(password string) error {
	if len(password) < minPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

func (s *Service) Register(ctx context.Context, email, password, name string) (pendingToken string, err error) {
	email = normalizeEmail(email)
	if email == "" {
		return "", ErrInvalidCredentials
	}
	if len(password) < minPasswordLen {
		return "", ErrWeakPassword
	}
	if strings.TrimSpace(name) == "" {
		return "", ErrInvalidCredentials
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if existing != nil {
		if existing.Status == "pending_verification" {
			return s.startRegistrationOTPChallenge(ctx, existing)
		}
		return "", ErrEmailTaken
	}

	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	username, err := s.generateUniqueUsername(ctx, name)
	if err != nil {
		return "", err
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
		return "", err
	}
	return s.startRegistrationOTPChallenge(ctx, user)
}

func (s *Service) Login(ctx context.Context, identifier, password string) (accessToken, refreshToken, pendingToken string, err error) {
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
		pendingToken, err = s.startOTPChallenge(ctx, user)
		if err != nil {
			return "", "", "", err
		}
		return "", "", pendingToken, ErrVerificationPending
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

func (s *Service) startOTPChallenge(ctx context.Context, user *model.User) (pendingToken string, err error) {
	if err := s.dispatchOTP(ctx, user); err != nil {
		return "", err
	}
	pendingToken = newToken(user.ID)
	if err := s.rdb.Set(ctx, "pending:"+pendingToken, user.ID, s.cfg.OTPTTL).Err(); err != nil {
		return "", err
	}
	return pendingToken, nil
}

func (s *Service) startRegistrationOTPChallenge(ctx context.Context, user *model.User) (string, error) {
	if err := s.limitOTPSend(ctx, user.ID); err != nil {
		return "", err
	}
	return s.startOTPChallenge(ctx, user)
}

func (s *Service) dispatchOTP(ctx context.Context, user *model.User) error {
	code, err := genOTP()
	if err != nil {
		return err
	}
	if err := s.rdb.Set(ctx, "otp:"+user.ID, code, s.cfg.OTPTTL).Err(); err != nil {
		return err
	}
	channel, destination := PreferredOTPChannel(deref(user.Phone), deref(user.Email))
	return s.otpProvider.SendOTP(ctx, channel, destination, code)
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

func (s *Service) SendOTP(ctx context.Context, userID string) error {
	if err := s.limitOTPSend(ctx, userID); err != nil {
		return err
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	return s.dispatchOTP(ctx, user)
}

func (s *Service) limitOTPSend(ctx context.Context, userID string) error {
	rl := s.rdb.SetNX(ctx, "otpsend:"+userID, "1", 60*time.Second)
	if err := rl.Err(); err != nil {
		return err
	}
	if !rl.Val() {
		return ErrOTPRateLimit
	}
	return nil
}

func (s *Service) VerifyOTP(ctx context.Context, pendingToken, code string) (accessToken, refreshToken string, err error) {
	userID, err := s.rdb.Get(ctx, "pending:"+pendingToken).Result()
	if errors.Is(err, redis.Nil) {
		return "", "", ErrInvalidPendingToken
	}
	if err != nil {
		return "", "", err
	}

	stored, err := s.rdb.Get(ctx, "otp:"+userID).Result()
	if errors.Is(err, redis.Nil) {
		return "", "", ErrOTPExpired
	}
	if err != nil {
		return "", "", err
	}
	if stored != code {
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
	s.rdb.Del(ctx, "otp:"+userID, "pending:"+pendingToken)
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

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		// No enumeration: pretend success when the email is unknown.
		return nil
	}

	code, err := genOTP()
	if err != nil {
		return err
	}
	token := newToken(user.ID)
	if err := s.rdb.Set(ctx, "reset:"+token, user.ID+":"+code, s.cfg.OTPTTL).Err(); err != nil {
		return err
	}
	body := fmt.Sprintf("Your password reset code is %s. Token: %s", code, token)
	return s.emailProvider.SendEmail(ctx, deref(user.Email), "Password reset", body)
}

func (s *Service) ResetPassword(ctx context.Context, token, otp, newPassword string) error {
	stored, err := s.rdb.Get(ctx, "reset:"+token).Result()
	if errors.Is(err, redis.Nil) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	userID, code, ok := strings.Cut(stored, ":")
	if !ok || code != otp {
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
	s.rdb.Del(ctx, "reset:"+token)
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

// PasswordChangeSession is the fresh token pair ChangePassword returns so the
// client can swap out the must-change-flagged token in one round trip.
type PasswordChangeSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ChangePassword rotates the caller's password and ends the forced-change
// state. It returns a freshly minted session so the client can swap tokens in
// the same round trip: the old access token still carries the
// must_change_password claim, and the middleware fails closed on it.
func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword, callerJTI string) (*PasswordChangeSession, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if !comparePassword(user.PasswordHash, currentPassword, metrics.OpChangePassword) {
		return nil, ErrInvalidCredentials
	}
	if len(newPassword) < minPasswordLen {
		return nil, ErrWeakPassword
	}
	if newPassword == currentPassword {
		return nil, ErrWeakPassword
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePasswordHash(ctx, userID, string(hash)); err != nil {
		return nil, err
	}
	// UpdatePasswordHash cleared the flag in the DB; mirror it here so the
	// re-minted token below drops the must_change_password claim.
	user.MustChangePassword = false
	s.revokeAllSessions(ctx, userID, callerJTI)
	access, refresh, err := s.mintSession(ctx, user)
	if err != nil {
		return nil, err
	}
	return &PasswordChangeSession{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) SessionActive(ctx context.Context, jti string) bool {
	exists, _ := s.rdb.Exists(ctx, "session:access:"+jti).Result()
	return exists > 0
}

func (s *Service) mintSession(ctx context.Context, user *model.User) (accessToken, refreshToken string, err error) {
	caps := Capabilities(user.Role)
	tokenString, jti, err := s.jwtSigner.SignAccess(user.ID, user.Role, user.SchoolID, caps, user.MustChangePassword)
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
