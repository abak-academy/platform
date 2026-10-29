package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// AffiliatorResponse is the affiliator shape returned by the admin and self endpoints.
type AffiliatorResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Email     *string        `json:"email"`
	Status    string         `json:"status"`
	SchoolIDs []string       `json:"school_ids"`
	Schools   []model.School `json:"schools,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	// TempPassword is set only on create, when the password was generated.
	TempPassword string `json:"temp_password,omitempty"`
	// Monitoring figures, filled by the list and detail reads.
	StudentCount int     `json:"student_count"`
	OrderCount   int     `json:"order_count"`
	PaidAmount   float64 `json:"paid_amount"`
}

// AffiliatorInput is one affiliator account to create.
type AffiliatorInput struct {
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Password  string   `json:"password"`
	SchoolIDs []string `json:"school_ids"`
}

func toAffiliatorResponse(row repository.AffiliatorRow) AffiliatorResponse {
	return AffiliatorResponse{
		ID:        row.ID,
		Name:      row.Name,
		Email:     row.Email,
		Status:    row.Status,
		SchoolIDs: row.SchoolIDs,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),

		StudentCount: row.StudentCount,
		OrderCount:   row.OrderCount,
		PaidAmount:   row.PaidAmount,
	}
}

// normalizeSchoolIDs drops duplicates, keeps order, and requires at least one valid UUID.
func normalizeSchoolIDs(ids []string) ([]string, error) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, err := parseUUID(id); err != nil {
			return nil, ErrInvalidUUID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrAffiliatorSchoolsRequired
	}
	return out, nil
}

func (s *Service) validateAffiliatorSchools(ctx context.Context, ids []string) ([]string, error) {
	ids, err := normalizeSchoolIDs(ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		sch, err := s.storeRepo.GetSchoolByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if sch == nil {
			return nil, ErrSchoolNotFound
		}
		if sch.Status != "active" {
			return nil, ErrSchoolDeactivated
		}
	}
	return ids, nil
}

// CreateAffiliator creates an active affiliator account bound to the given
// schools. An empty password is replaced by a generated one that is returned
// once in TempPassword, like student registration.
func (s *Service) CreateAffiliator(ctx context.Context, actorID string, in AffiliatorInput) (*AffiliatorResponse, error) {
	schoolIDs, err := s.validateAffiliatorSchools(ctx, in.SchoolIDs)
	if err != nil {
		return nil, err
	}
	return s.createAffiliator(ctx, actorID, in.Email, in.Name, in.Password, schoolIDs)
}

// createAffiliator is the shared create path; schoolIDs are already validated.
func (s *Service) createAffiliator(ctx context.Context, actorID, rawEmail, rawName, password string, schoolIDs []string) (*AffiliatorResponse, error) {
	email := normalizeEmail(rawEmail)
	name := strings.TrimSpace(rawName)
	if email == "" || name == "" {
		return nil, ErrMissingField
	}
	generated := ""
	if password == "" {
		var err error
		if generated, err = genTempPassword(); err != nil {
			return nil, fmt.Errorf("generate temp password: %w", err)
		}
		password = generated
	} else if len(password) < minPasswordLen {
		return nil, ErrWeakPassword
	}
	if err := checkEmailUniqueness(ctx, s.repo, email); err != nil {
		return nil, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{Email: &email, Name: name, PasswordHash: string(hash)}
	if err := s.storeRepo.CreateAffiliatorUser(ctx, user, schoolIDs); err != nil {
		return nil, fmt.Errorf("create affiliator: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &actorID, "user", user.ID, "affiliator.create", map[string]any{
		"email": email, "school_ids": schoolIDs,
	}); err != nil {
		return nil, err
	}
	return &AffiliatorResponse{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Status:       "active",
		SchoolIDs:    schoolIDs,
		CreatedAt:    user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    user.UpdatedAt.Format(time.RFC3339),
		TempPassword: generated,
	}, nil
}

// PromoteToAffiliator turns an admin_school account into an affiliator with the given schools.
// Its sessions are revoked so the next login carries the new role.
func (s *Service) PromoteToAffiliator(ctx context.Context, actorID, targetID string, schoolIDs []string) error {
	if _, err := parseUUID(targetID); err != nil {
		return ErrInvalidUUID
	}
	schoolIDs, err := s.validateAffiliatorSchools(ctx, schoolIDs)
	if err != nil {
		return err
	}
	user, err := s.storeRepo.GetAdminUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if user.Role != RoleAdminSchool {
		return ErrNotAdminSchool
	}
	ok, err := s.storeRepo.PromoteSchoolAdminToAffiliator(ctx, targetID, schoolIDs)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAdminSchool
	}
	s.revokeAllSessions(ctx, targetID)
	return s.storeRepo.InsertAuditLogMeta(ctx, nil, &actorID, "user", targetID, "affiliator.promote", map[string]any{
		"from": user.Role, "school_ids": schoolIDs,
	})
}

// SetAffiliatorSchools replaces the schools an affiliator may access.
func (s *Service) SetAffiliatorSchools(ctx context.Context, actorID, targetID string, schoolIDs []string) error {
	if _, err := parseUUID(targetID); err != nil {
		return ErrInvalidUUID
	}
	schoolIDs, err := s.validateAffiliatorSchools(ctx, schoolIDs)
	if err != nil {
		return err
	}
	ok, err := s.storeRepo.SetAffiliatorSchoolIDs(ctx, targetID, schoolIDs)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAffiliatorNotFound
	}
	return s.storeRepo.InsertAuditLogMeta(ctx, nil, &actorID, "user", targetID, "affiliator.set_schools", map[string]any{
		"school_ids": schoolIDs,
	})
}

// ListAffiliators returns affiliators filtered by optional status and name/email
// search, cursor-paginated, with the total matching the filters.
func (s *Service) ListAffiliators(ctx context.Context, statusFilter, q string, limit int, cursor string) ([]AffiliatorResponse, string, int, error) {
	if statusFilter != "" && !isValidStatusFilter(statusFilter) {
		return nil, "", 0, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, statusFilter)
	}
	rows, next, total, err := s.storeRepo.ListAffiliators(ctx, repository.AffiliatorFilter{Status: statusFilter, Q: strings.TrimSpace(q), Cursor: cursor, Limit: limit})
	if err != nil {
		return nil, "", 0, err
	}
	out := make([]AffiliatorResponse, len(rows))
	for i, r := range rows {
		out[i] = toAffiliatorResponse(r)
	}
	return out, next, total, nil
}

// ListPromotableAdmins returns the admin_school accounts that can be promoted.
func (s *Service) ListPromotableAdmins(ctx context.Context, q string, limit int, cursor string) ([]repository.PromotableAdmin, string, int, error) {
	return s.storeRepo.ListPromotableAdmins(ctx, strings.TrimSpace(q), cursor, limit)
}

// GetAffiliatorProfile returns an affiliator's account with the details of its schools.
func (s *Service) GetAffiliatorProfile(ctx context.Context, userID string) (*AffiliatorResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Role != RoleAffiliator {
		return nil, ErrAffiliatorNotFound
	}
	ids, err := s.storeRepo.GetAffiliatorSchoolIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := &AffiliatorResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Status:    user.Status,
		SchoolIDs: ids,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}
	for _, id := range ids {
		sch, err := s.storeRepo.GetSchoolByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if sch != nil {
			resp.Schools = append(resp.Schools, *sch)
		}
	}
	return resp, nil
}

// AffiliatorActive reports whether the affiliator account is active; a
// deactivated one keeps a read-only session.
func (s *Service) AffiliatorActive(ctx context.Context, userID string) (bool, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil && user.Status == "active", nil
}

// canHoldSession reports whether a user in this state may be issued a session.
func canHoldSession(user *model.User) bool {
	return user.Status == "active" || (user.Role == RoleAffiliator && user.Status == "deactivated")
}

// AffiliatorSchoolIDs returns the schools an affiliator may act on.
func (s *Service) AffiliatorSchoolIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.storeRepo.GetAffiliatorSchoolIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// affiliatorRegistration returns the id to record as registered_by when the
// actor is an affiliator, after checking the school is one of theirs. Other
// actors return nil and are not restricted here.
func (s *Service) affiliatorRegistration(ctx context.Context, actorID, schoolID string) (*string, error) {
	if _, err := parseUUID(actorID); err != nil {
		return nil, nil
	}
	actor, err := s.repo.GetUserByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if actor == nil || actor.Role != RoleAffiliator {
		return nil, nil
	}
	ids, err := s.storeRepo.GetAffiliatorSchoolIDs(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if schoolID == "" || !slices.Contains(ids, strings.ToLower(schoolID)) {
		return nil, ErrAffiliatorSchoolNotAllowed
	}
	return &actorID, nil
}

// ListAffiliatorStudents lists students of the affiliator's allowed schools,
// optionally narrowed to one school and to the ones it registered itself.
func (s *Service) ListAffiliatorStudents(ctx context.Context, affiliatorID, schoolID string, registeredByMe, notPurchased bool, statusFilter, q string, limit int, cursor string, grade *int, jenjang string) ([]StudentResponse, string, repository.StudentAdminCounts, error) {
	ids, err := s.AffiliatorSchoolIDs(ctx, affiliatorID)
	if err != nil {
		return nil, "", repository.StudentAdminCounts{}, err
	}
	filter := repository.StudentFilter{
		Status: statusFilter, Cursor: cursor, Limit: limit, Q: q, Grade: grade, Jenjang: jenjang,
		SchoolIDs: ids,
	}
	if schoolID != "" {
		if !slices.Contains(ids, strings.ToLower(schoolID)) {
			return nil, "", repository.StudentAdminCounts{}, ErrAffiliatorSchoolNotAllowed
		}
		filter.SchoolIDs = []string{strings.ToLower(schoolID)}
	} else {
		filter.IncludeNoSchoolOf = affiliatorID
	}
	if registeredByMe {
		filter.RegisteredBy = affiliatorID
	}
	if notPurchased {
		filter.NotPurchasedBy = affiliatorID
	}
	rows, next, err := s.storeRepo.ListStudentsBySchool(ctx, "", filter)
	if err != nil {
		return nil, "", repository.StudentAdminCounts{}, err
	}
	counts, err := s.storeRepo.CountStudentsAdmin(ctx, "", filter)
	if err != nil {
		return nil, "", repository.StudentAdminCounts{}, err
	}
	students := make([]StudentResponse, len(rows))
	for i, r := range rows {
		students[i] = toStudentResponse(r)
	}
	return students, next, counts, nil
}
