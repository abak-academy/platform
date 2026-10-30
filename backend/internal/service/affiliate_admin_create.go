package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"akademi-bimbel/internal/model"
)

// affiliateAdminTermsVersion marks profiles an admin created: the affiliator
// never accepted a terms version at sign-up.
const affiliateAdminTermsVersion = "admin_created"

const affiliateBulkCreateMax = 200

// ErrAffiliateBulkTooLarge reports a bulk create over affiliateBulkCreateMax rows.
var ErrAffiliateBulkTooLarge = errors.New("too many rows in one bulk create")

// AdminCreateAffiliatorInput is one affiliator an admin adds; the phone is
// mandatory for every affiliator.
type AdminCreateAffiliatorInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// AdminCreatedAffiliator carries the new account and its temporary password,
// which is shown once.
type AdminCreatedAffiliator struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Code         string `json:"code"`
	TempPassword string `json:"temp_password"`
}

// AdminAffiliatorBulkResult is one row's outcome: Created on success, Error
// (the API error code) otherwise. Row is the 1-based position in the request.
type AdminAffiliatorBulkResult struct {
	Row     int                     `json:"row"`
	Created *AdminCreatedAffiliator `json:"created,omitempty"`
	Error   string                  `json:"error,omitempty"`
	Email   string                  `json:"email"`
}

// AdminCreateAffiliator creates an active affiliator with a referral code and a
// temporary password the affiliator must change at first login.
func (s *Service) AdminCreateAffiliator(ctx context.Context, actorID string, in AdminCreateAffiliatorInput) (*AdminCreatedAffiliator, error) {
	if _, err := parseUUID(actorID); err != nil {
		return nil, ErrInvalidUUID
	}
	email := normalizeEmail(in.Email)
	if email == "" {
		return nil, ErrInvalidCredentials
	}
	name := strings.TrimSpace(stripFormatRunes(in.Name))
	if name == "" {
		return nil, ErrInvalidCredentials
	}
	phone := strings.TrimSpace(in.Phone)
	if phone == "" {
		return nil, ErrAffiliatePhoneRequired
	}
	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}
	temp, err := genTempPassword()
	if err != nil {
		return nil, err
	}
	hash, err := hashPassword(temp)
	if err != nil {
		return nil, err
	}
	username, err := s.generateUniqueUsername(ctx, name)
	if err != nil {
		return nil, err
	}
	code, err := s.generateUniqueAffiliateCode(ctx)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Email: &email, Username: &username, Phone: &phone,
		PasswordHash: string(hash), Role: RoleAffiliator, Name: name,
	}
	if err := s.storeRepo.CreateActiveAffiliatorAccount(ctx, user, affiliateAdminTermsVersion, code); err != nil {
		return nil, fmt.Errorf("create affiliator account: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &actorID, affiliateAuditTargetType, user.ID, "affiliate.created", map[string]any{"code": code}); err != nil {
		return nil, fmt.Errorf("write create audit: %w", err)
	}
	return &AdminCreatedAffiliator{ID: user.ID, Name: name, Email: email, Phone: phone, Code: code, TempPassword: temp}, nil
}

// AdminBulkCreateAffiliators creates each row on its own, so one bad row does
// not stop the rest; the response lists every row's outcome.
func (s *Service) AdminBulkCreateAffiliators(ctx context.Context, actorID string, rows []AdminCreateAffiliatorInput) ([]AdminAffiliatorBulkResult, error) {
	if len(rows) > affiliateBulkCreateMax {
		return nil, ErrAffiliateBulkTooLarge
	}
	out := make([]AdminAffiliatorBulkResult, 0, len(rows))
	for i, row := range rows {
		res := AdminAffiliatorBulkResult{Row: i + 1, Email: row.Email}
		created, err := s.AdminCreateAffiliator(ctx, actorID, row)
		switch {
		case err == nil:
			res.Created = created
		case errors.Is(err, ErrEmailTaken):
			res.Error = "email_taken"
		case errors.Is(err, ErrAffiliatePhoneRequired):
			res.Error = "phone_required"
		case errors.Is(err, ErrInvalidCredentials):
			res.Error = "invalid_row"
		default:
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}
