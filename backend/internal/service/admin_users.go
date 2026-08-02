package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// adminRoles is the set of valid admin roles assignable via system endpoints.
var adminRoles = map[string]bool{
	RoleSuperAdmin:  true,
	RoleAdminStore:  true,
	RoleAdminExam:   true,
	RoleAdminSchool: true,
}

func isValidAdminRole(role string) bool {
	return adminRoles[role]
}

func isValidStatusFilter(status string) bool {
	return status == "active" || status == "deactivated"
}

func checkSelfDeactivation(targetID, actorID, newStatus string) error {
	if targetID == actorID && newStatus == "deactivated" {
		return ErrCannotDeactivateSelf
	}
	return nil
}

func checkEmailUniqueness(ctx context.Context, repo UserRepository, email string) error {
	existing, err := repo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrEmailTaken
	}
	return nil
}

// AdminAccountResponse is the trimmed account shape returned in admin responses.
type AdminAccountResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     *string `json:"email"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	SchoolID  *string `json:"school_id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func toAdminAccountResponse(row repository.AdminUserRow) AdminAccountResponse {
	return AdminAccountResponse{
		ID:        row.ID,
		Name:      row.Name,
		Email:     row.Email,
		Role:      row.Role,
		Status:    row.Status,
		SchoolID:  row.SchoolID,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
}

// ListAdminAccounts returns admin accounts filtered by optional role/status, cursor-paginated.
func (s *Service) ListAdminAccounts(ctx context.Context, roleFilter, statusFilter string, limit int, cursor string) ([]AdminAccountResponse, string, error) {
	if roleFilter != "" && !isValidAdminRole(roleFilter) {
		return nil, "", fmt.Errorf("%w: %s", ErrInvalidRoleFilter, roleFilter)
	}
	if statusFilter != "" && !isValidStatusFilter(statusFilter) {
		return nil, "", fmt.Errorf("%w: %s", ErrInvalidStatusFilter, statusFilter)
	}

	rows, nextCursor, err := s.storeRepo.ListAdminUsers(ctx, repository.AdminUserFilter{
		Role:   roleFilter,
		Status: statusFilter,
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		return nil, "", err
	}

	accounts := make([]AdminAccountResponse, len(rows))
	for i, r := range rows {
		accounts[i] = toAdminAccountResponse(r)
	}
	return accounts, nextCursor, nil
}

// CreateAdminAccount creates a new admin account with the given role.
func (s *Service) CreateAdminAccount(ctx context.Context, actorID, email, name, role, password string, schoolID *string) (*AdminAccountResponse, error) {
	email = normalizeEmail(email)
	if email == "" || strings.TrimSpace(name) == "" || role == "" || password == "" {
		return nil, ErrMissingField
	}
	if !isValidAdminRole(role) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAdminRole, role)
	}
	if len(password) < minPasswordLen {
		return nil, ErrWeakPassword
	}
	if err := checkEmailUniqueness(ctx, s.repo, email); err != nil {
		return nil, err
	}

	if role == RoleAdminSchool {
		if schoolID == nil || *schoolID == "" {
			return nil, ErrSchoolRequired
		}
		sch, err := s.storeRepo.GetSchoolByID(ctx, *schoolID)
		if err != nil {
			return nil, err
		}
		if sch == nil {
			return nil, ErrSchoolNotFound
		}
	} else if schoolID != nil {
		return nil, ErrSchoolNotAllowed
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:        &email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         role,
		SchoolID:     schoolID,
		Status:       "active",
		OTPEnabled:   false,
	}
	if err := s.storeRepo.CreateAdminUser(ctx, user); err != nil {
		return nil, err
	}

	actor := &actorID
	auditMeta := map[string]any{
		"role":  role,
		"email": email,
	}
	if schoolID != nil {
		auditMeta["school_id"] = *schoolID
	}
	if auditErr := s.storeRepo.InsertAuditLogMeta(ctx, nil, actor, "user", user.ID, "account.create", auditMeta); auditErr != nil {
		return nil, auditErr
	}

	return &AdminAccountResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Status,
		SchoolID:  user.SchoolID,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// ChangeAccountRole changes an account's role.
func (s *Service) ChangeAccountRole(ctx context.Context, actorID, targetID, newRole string, schoolID *string) error {
	if _, err := parseUUID(targetID); err != nil {
		return ErrInvalidUUID
	}
	if !isValidAdminRole(newRole) {
		return fmt.Errorf("%w: %s", ErrInvalidAdminRole, newRole)
	}

	user, err := s.storeRepo.GetAdminUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	if newRole == RoleAdminSchool {
		if schoolID == nil || *schoolID == "" {
			return ErrSchoolRequired
		}
		sch, err := s.storeRepo.GetSchoolByID(ctx, *schoolID)
		if err != nil {
			return err
		}
		if sch == nil {
			return ErrSchoolNotFound
		}
	} else {
		schoolID = nil
	}

	oldRole := user.Role
	if err := s.storeRepo.UpdateAdminUserRole(ctx, targetID, newRole); err != nil {
		return err
	}
	if err := s.storeRepo.SetUserSchoolID(ctx, targetID, schoolID); err != nil {
		return err
	}

	actor := &actorID
	auditMeta := map[string]any{
		"from": oldRole,
		"to":   newRole,
	}
	if schoolID != nil {
		auditMeta["school_id"] = *schoolID
	}
	if auditErr := s.storeRepo.InsertAuditLogMeta(ctx, nil, actor, "user", targetID, "account.role_change", auditMeta); auditErr != nil {
		return auditErr
	}

	return nil
}

// ChangeAccountStatus changes an account's status. Deactivation revokes all sessions.
func (s *Service) ChangeAccountStatus(ctx context.Context, actorID, targetID, newStatus string) error {
	if _, err := parseUUID(targetID); err != nil {
		return ErrInvalidUUID
	}
	if newStatus != "active" && newStatus != "deactivated" {
		return fmt.Errorf("%w: %s", ErrInvalidStatusFilter, newStatus)
	}
	if err := checkSelfDeactivation(targetID, actorID, newStatus); err != nil {
		return err
	}

	user, err := s.storeRepo.GetAdminUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	oldStatus := user.Status
	if err := s.storeRepo.UpdateAdminUserStatus(ctx, targetID, newStatus); err != nil {
		return err
	}

	if newStatus == "deactivated" {
		s.revokeAllSessions(ctx, targetID)
	}

	actor := &actorID
	if auditErr := s.storeRepo.InsertAuditLogMeta(ctx, nil, actor, "user", targetID, "account.status_change", map[string]any{
		"from": oldStatus,
		"to":   newStatus,
	}); auditErr != nil {
		return auditErr
	}

	return nil
}

// TriggerAccountPasswordReset sends a password reset email for an admin account.
func (s *Service) TriggerAccountPasswordReset(ctx context.Context, actorID, targetID string) error {
	if _, err := parseUUID(targetID); err != nil {
		return ErrInvalidUUID
	}

	user, err := s.storeRepo.GetAdminUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if user.Email == nil || *user.Email == "" {
		return ErrAccountNoEmail
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
	if err := s.emailProvider.SendEmail(ctx, *user.Email, "Password reset", body); err != nil {
		return err
	}

	actor := &actorID
	if auditErr := s.storeRepo.InsertAuditLogMeta(ctx, nil, actor, "user", targetID, "account.reset_password", map[string]any{}); auditErr != nil {
		return auditErr
	}

	return nil
}
