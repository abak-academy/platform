package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"akademi-bimbel/internal/model"
)

const (
	programAffiliate = "affiliate"
	programPartner   = "partner"

	partnerEnabledKey = "partner_enabled"

	partnerAuditSchoolsSet    = "partner.schools_set"
	affiliateAuditProgramsAdd = "affiliate.programs_change"
	partnerMaxAssignedSchools = 500
)

var (
	ErrPartnerRegistrationClosed = errors.New("partner program is not open for registration")
	ErrAffiliateInvalidProgram   = errors.New("program must be affiliate or partner")
	ErrAffiliatorNotApproved     = errors.New("affiliator is not approved yet")
	ErrAffiliatorNotPartner      = errors.New("affiliator does not take part in the partner program")
)

// ProgramState is how one program sees an account, for the portal menu and the
// access gates.
type ProgramState string

const (
	ProgramNotEnrolled ProgramState = "not_enrolled"
	ProgramPending     ProgramState = "pending"
	ProgramRejected    ProgramState = "rejected"
	ProgramActive      ProgramState = "active"
)

// AffiliatorProgramStates is one account's standing in both programs.
// AccountActive is false for a suspended or deactivated account, which stays
// readable in every program it belongs to.
type AffiliatorProgramStates struct {
	Affiliate       ProgramState `json:"affiliate"`
	Partner         ProgramState `json:"partner"`
	AccountActive   bool         `json:"account_active"`
	RejectionReason *string      `json:"rejection_reason,omitempty"`
	PartnerSchools  []string     `json:"partner_school_ids,omitempty"`
}

// AffiliatorProgramStates resolves the states from one query. A user without an
// affiliator profile belongs to neither program.
func (s *Service) AffiliatorProgramStates(ctx context.Context, userID string) (*AffiliatorProgramStates, error) {
	access, err := s.storeRepo.GetAffiliatorAccess(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator access: %w", err)
	}
	states := &AffiliatorProgramStates{Affiliate: ProgramNotEnrolled, Partner: ProgramNotEnrolled}
	if access == nil {
		return states, nil
	}
	states.AccountActive = access.UserStatus == "active"
	states.RejectionReason = access.RejectionReason

	state := programStateOf(access.ProgramStatus)
	if access.Programs.HasAffiliate() {
		states.Affiliate = state
	}
	if access.Programs.HasPartner() {
		states.Partner = state
		states.PartnerSchools = slices.Clone(access.Programs.Partner.SchoolIDs)
	}
	return states, nil
}

func programStateOf(programStatus string) ProgramState {
	switch programStatus {
	case "pending_approval":
		return ProgramPending
	case "rejected":
		return ProgramRejected
	default:
		return ProgramActive
	}
}

// partnerEnabledErr reads the Mitra program switch; a failed read is an error,
// never "off", because the money and access paths must not guess.
func (s *Service) partnerEnabledErr(ctx context.Context) (bool, error) {
	value, found, err := s.storeRepo.GetSystemConfigValue(ctx, partnerEnabledKey)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", partnerEnabledKey, err)
	}
	return found && value == "true", nil
}

// PartnerProgramEnabled is the read-path form: a failed read counts as off.
func (s *Service) PartnerProgramEnabled(ctx context.Context) bool {
	on, err := s.partnerEnabledErr(ctx)
	return err == nil && on
}

// SetPartnerProgramEnabled switches the Mitra program on or off through
// UpdateSystemConfig so the audit row is written like any config change.
func (s *Service) SetPartnerProgramEnabled(ctx context.Context, actorID string, enabled bool) error {
	_, err := s.UpdateSystemConfig(ctx, actorID, map[string]string{partnerEnabledKey: strconv.FormatBool(enabled)})
	return err
}

// canHoldSession reports whether a user may be given a session. A deactivated
// affiliator can still sign in: the account is read-only, not locked out.
func canHoldSession(user *model.User) bool {
	if user.Status == "active" {
		return true
	}
	return user.Status == "deactivated" && user.Role == RoleAffiliator
}

// affiliatorProgramsFrom turns a list of program names into a membership. An
// empty list and "affiliate" alone both give the plain Affiliator account.
func affiliatorProgramsFrom(names []string) (model.AffiliatorPrograms, error) {
	if len(names) == 0 {
		return model.AffiliatorPrograms{}, ErrAffiliateInvalidProgram
	}
	var programs model.AffiliatorPrograms
	affiliate := false
	for _, name := range names {
		switch name {
		case programAffiliate:
			affiliate = true
		case programPartner:
			programs.Partner = &model.PartnerProfile{SchoolIDs: []string{}}
		default:
			return model.AffiliatorPrograms{}, fmt.Errorf("%w: %q", ErrAffiliateInvalidProgram, name)
		}
	}
	if programs.Partner != nil {
		programs.Affiliate = &affiliate
	}
	return programs, nil
}

// AdminAddAffiliatorProgram adds a program to an approved account. Adding a
// program the account already has changes nothing. Programs are never removed:
// suspending the account is how access stops.
func (s *Service) AdminAddAffiliatorProgram(ctx context.Context, actorID, id, program string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if program != programAffiliate && program != programPartner {
		return nil, ErrAffiliateInvalidProgram
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if aff.Status == "pending_approval" || aff.Status == "rejected" {
		return nil, ErrAffiliatorNotApproved
	}

	programs := aff.Programs
	switch program {
	case programAffiliate:
		if programs.HasAffiliate() {
			return aff, nil
		}
		yes := true
		programs.Affiliate = &yes
	case programPartner:
		if programs.HasPartner() {
			return aff, nil
		}
		programs.Partner = &model.PartnerProfile{SchoolIDs: []string{}}
		if programs.Affiliate == nil {
			yes := true
			programs.Affiliate = &yes
		}
	}

	code := ""
	if program == programAffiliate && aff.Code == "" {
		if code, err = s.generateUniqueAffiliateCode(ctx); err != nil {
			return nil, err
		}
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SetAffiliatorProgramsTx(ctx, tx, affID, programs); err != nil {
		return nil, fmt.Errorf("set affiliator programs: %w", err)
	}
	meta := map[string]any{"added": program}
	if code != "" {
		if _, err := s.storeRepo.SetAffiliatorReferralCodeIfNoneTx(ctx, tx, affID, code); err != nil {
			return nil, fmt.Errorf("set referral code: %w", err)
		}
		meta["code"] = code
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditProgramsAdd, meta); err != nil {
		return nil, fmt.Errorf("write programs audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit programs change: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

// AdminSetPartnerSchools replaces the schools a Mitra may order for.
func (s *Service) AdminSetPartnerSchools(ctx context.Context, actorID, id string, schoolIDs []string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if len(schoolIDs) > partnerMaxAssignedSchools {
		return nil, fmt.Errorf("%w: at most %d schools", ErrValidation, partnerMaxAssignedSchools)
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if !aff.Programs.HasPartner() {
		return nil, ErrAffiliatorNotPartner
	}

	ids := make([]string, 0, len(schoolIDs))
	for _, raw := range schoolIDs {
		schoolID, err := parseUUID(raw)
		if err != nil {
			return nil, ErrInvalidUUID
		}
		key := schoolID.String()
		if slices.Contains(ids, key) {
			continue
		}
		school, err := s.storeRepo.GetSchoolByID(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get school: %w", err)
		}
		if school == nil {
			return nil, ErrSchoolNotFound
		}
		if school.Status != "active" {
			return nil, ErrSchoolDeactivated
		}
		ids = append(ids, key)
	}

	programs := aff.Programs
	programs.Partner = &model.PartnerProfile{SchoolIDs: ids}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SetAffiliatorProgramsTx(ctx, tx, affID, programs); err != nil {
		return nil, fmt.Errorf("set partner schools: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), partnerAuditSchoolsSet, map[string]any{
		"from": aff.Programs.Partner.SchoolIDs,
		"to":   ids,
	}); err != nil {
		return nil, fmt.Errorf("write schools audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit partner schools: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}
