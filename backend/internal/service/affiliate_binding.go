package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	affiliateSourceLink         = "link"
	affiliateSourceManualCode   = "manual_code"
	affiliateSourceCheckoutCode = "checkout_code"

	affiliateBindingStatusActive    = "active"
	affiliateAffiliatorStatusActive = "active"
)

// AffiliateBindingDecision is the attribution outcome for one student; a nil
// decision from ResolveBinding means no binding should be created.
type AffiliateBindingDecision struct {
	AffiliatorID uuid.UUID
	Source       string
}

// ResolveBinding decides whether a student gets bound to an affiliator from a
// manual referral code (register/OTP) or a link cookie. Precedence is manual >
// cookie; unknown/inactive codes, self-purchase, pre-launch accounts and
// already-bound students all yield a nil decision with no error, so callers
// simply skip binding creation.
func (s *Service) ResolveBinding(ctx context.Context, student *model.User, manualCode, cookieCode *string) (*AffiliateBindingDecision, error) {
	code, source := affiliateAttributionCode(manualCode, cookieCode)
	if code == "" {
		return nil, nil
	}

	owner, err := s.storeRepo.GetAffiliatorByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get affiliator by code: %w", err)
	}
	if owner == nil || owner.Status != affiliateAffiliatorStatusActive {
		return nil, nil
	}

	if affiliateSelfMatch(owner.Email, owner.Phone, student.Email, student.Phone) {
		return nil, nil
	}

	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate settings: %w", err)
	}
	if settings.LaunchDate == nil || student.CreatedAt.Before(*settings.LaunchDate) {
		return nil, nil
	}

	existing, err := s.storeRepo.GetActiveBindingByStudent(ctx, nil, student.ID)
	if err != nil {
		return nil, fmt.Errorf("get active binding: %w", err)
	}
	if existing != nil {
		return nil, nil
	}

	return &AffiliateBindingDecision{AffiliatorID: owner.ID, Source: source}, nil
}

func affiliateAttributionCode(manual, cookie *string) (code, source string) {
	if c := affiliateCodeString(manual); c != "" {
		return c, affiliateSourceManualCode
	}
	if c := affiliateCodeString(cookie); c != "" {
		return c, affiliateSourceLink
	}
	return "", ""
}

func affiliateCodeString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*p))
}

func affiliateSelfMatch(affEmail, affPhone, studentEmail, studentPhone *string) bool {
	if affEmail != nil && studentEmail != nil &&
		strings.EqualFold(strings.TrimSpace(*affEmail), strings.TrimSpace(*studentEmail)) {
		return true
	}
	return affPhone != nil && studentPhone != nil &&
		strings.TrimSpace(*affPhone) == strings.TrimSpace(*studentPhone)
}

// affiliateBindingMonths is the binding length for an affiliator: its current
// level's binding_months, or the global value when the level leaves it unset.
func (s *Service) affiliateBindingMonths(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (int, error) {
	settings, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return 0, fmt.Errorf("load affiliate effective rules: %w", err)
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, tx, affiliatorID)
	if err != nil {
		return 0, fmt.Errorf("get affiliator: %w", err)
	}
	if aff != nil {
		if rule, ok := rules[aff.Level]; ok {
			return rule.BindingMonths, nil
		}
	}
	return settings.BindingMonths, nil
}

// CreateBinding inserts one active binding inside the caller's transaction,
// lasting the affiliator's level binding months from now.
func (s *Service) CreateBinding(ctx context.Context, tx pgx.Tx, studentID string, affiliatorID uuid.UUID, source string) error {
	months, err := s.affiliateBindingMonths(ctx, tx, affiliatorID)
	if err != nil {
		return err
	}
	sid, err := parseUUID(studentID)
	if err != nil {
		return err
	}
	startedAt := time.Now()
	endedAt := startedAt.AddDate(0, months, 0)
	binding := &model.AffiliateBinding{
		AffiliatorID: affiliatorID,
		StudentID:    sid,
		Status:       affiliateBindingStatusActive,
		Source:       source,
		StartedAt:    startedAt,
		EndedAt:      &endedAt,
	}
	if err := s.storeRepo.InsertBinding(ctx, tx, binding); err != nil {
		// A concurrent binding won the partial unique index; the student keeps it.
		if errors.Is(err, repository.ErrAffiliateBindingExists) {
			return nil
		}
		return fmt.Errorf("insert affiliate binding: %w", err)
	}
	return nil
}

// ExtendBinding extends the student's active binding when the paid order meets
// the extension threshold (0 = every payment extends).
func (s *Service) ExtendBinding(ctx context.Context, tx pgx.Tx, studentID string, paidAt time.Time, baseAmount float64) error {
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return fmt.Errorf("load affiliate settings: %w", err)
	}
	// min_extend_amount defaults to 0 (every purchase extends), but a free order
	// is not a purchase and must not keep a binding alive.
	if baseAmount <= 0 || baseAmount < settings.MinExtendAmount {
		return nil
	}
	binding, err := s.storeRepo.GetActiveBindingByStudent(ctx, tx, studentID)
	if err != nil {
		return fmt.Errorf("get active binding: %w", err)
	}
	if binding == nil {
		return nil
	}
	months, err := s.affiliateBindingMonths(ctx, tx, binding.AffiliatorID)
	if err != nil {
		return err
	}
	// ended_at re-anchors from the payment date instead of stacking onto the remaining window.
	newEndedAt := paidAt.AddDate(0, months, 0)
	if binding.EndedAt != nil && binding.EndedAt.After(newEndedAt) {
		newEndedAt = *binding.EndedAt
	}
	return s.storeRepo.ExtendBindingTx(ctx, tx, binding.ID, newEndedAt, paidAt)
}

// ExpireBindings ends every active binding whose ended_at has passed; repeat
// runs change nothing.
func (s *Service) ExpireBindings(ctx context.Context, now time.Time) (int, error) {
	rows, err := s.storeRepo.ExpireDueBindings(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("expire due bindings: %w", err)
	}
	return int(rows), nil
}

// AffiliateCodeEligible reports whether a checkout referral code could still
// bind this student (flag on, account created after launch, no active binding).
// It deliberately says nothing about who the binding belongs to, and any lookup
// failure reads as false so the cart never fails over a referral hint.
func (s *Service) AffiliateCodeEligible(ctx context.Context, studentID string) bool {
	if s.storeRepo == nil || !s.affiliateEnabled(ctx) {
		return false
	}
	student, err := s.storeRepo.GetUserByID(ctx, studentID)
	if err != nil || student == nil || student.Role != RoleStudent {
		return false
	}
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil || settings.LaunchDate == nil || student.CreatedAt.Before(*settings.LaunchDate) {
		return false
	}
	existing, err := s.storeRepo.GetActiveBindingByStudent(ctx, nil, student.ID)
	return err == nil && existing == nil
}
