package service

import (
	"context"
	"errors"
	"strings"

	"akademi-bimbel/internal/model"
)

var (
	ErrAffiliateRegistrationClosed = errors.New("affiliate program is not open for registration")
	ErrAffiliateTermsRequired      = errors.New("program terms must be accepted")
	ErrAffiliatePhoneRequired      = errors.New("phone number is required")
)

// RegisterAffiliator backs POST /auth/affiliators/register (BR-REG-01/03/04).
// It mirrors Register: the account starts pending_verification behind the same
// OTP, with a pending_approval profile that has no referral code yet.
func (s *Service) RegisterAffiliator(ctx context.Context, email, password, name, phone, termsVersion, ip string) (OTPChallenge, error) {
	if s.storeRepo == nil || !s.affiliateEnabled(ctx) {
		return OTPChallenge{}, ErrAffiliateRegistrationClosed
	}
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
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return OTPChallenge{}, ErrAffiliatePhoneRequired
	}
	termsVersion = strings.TrimSpace(termsVersion)
	if termsVersion == "" {
		return OTPChallenge{}, ErrAffiliateTermsRequired
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return OTPChallenge{}, err
	}
	if existing != nil {
		if existing.Status == "pending_verification" && existing.Role == RoleAffiliator {
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
		Phone:        &phone,
		PasswordHash: string(hash),
		Role:         RoleAffiliator,
		Name:         name,
		Status:       "pending_verification",
		OTPEnabled:   true,
	}
	if err := s.storeRepo.CreateAffiliatorAccount(ctx, user, termsVersion); err != nil {
		return OTPChallenge{}, err
	}
	return s.startOTPChallenge(ctx, user, ip)
}
