package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Feature-local sentinels (see affiliate_jobs.go for the precedent); the
// parent owns their mapServiceError cases. ErrAffiliateNotFound lives in
// affiliate_admin_money.go and reports an unknown affiliator here too.
var (
	ErrAffiliateNotPending              = errors.New("affiliator is not awaiting approval")
	ErrAffiliateInvalidDecision         = errors.New("decision must be approve or reject")
	ErrAffiliateRejectReasonRequired    = errors.New("reason is required to reject an affiliator")
	ErrAffiliateInvalidStatus           = errors.New("invalid affiliator status")
	ErrAffiliateReasonRequired          = errors.New("reason is required")
	ErrAffiliateInvalidDays             = errors.New("days must be at least 1")
	ErrAffiliateInvalidLevel            = errors.New("invalid affiliate level")
	ErrAffiliateCodeGenerationExhausted = errors.New("referral code generation exhausted after 10 attempts")
	ErrAffiliateInvalidTransition       = errors.New("affiliator status cannot be changed from its current state")
)

// affiliateCodeChars excludes O/0/I/1 — glyphs that read as each other once a
// code is printed or copied by hand.
const (
	affiliateCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	affiliateCodeLen   = 8
	affiliateCodeTries = 10
)

var affiliateAdminStatuses = []string{"pending_approval", "active", "suspended", "deactivated", "rejected"}

var affiliateChangeStatuses = []string{"active", "suspended", "deactivated"}

const (
	affiliateAuditTargetType   = "affiliator"
	affiliateAuditApproval     = "affiliate.approval"
	affiliateAuditStatusChange = "affiliate.status_change"
	affiliateAuditFreeze       = "affiliate.freeze"
	affiliateAuditUnfreeze     = "affiliate.unfreeze"
	affiliateAuditLevelChange  = "affiliate.level_change"
	affiliateLevelSourceManual = "manual"
	affiliateSuspendMaxDays    = 365
)

// AffiliateAdminListFilter carries the admin list's filters and pagination.
type AffiliateAdminListFilter struct {
	Status  string
	Level   string
	Query   string
	Page    int
	PerPage int
}

// AffiliateAdminListPage is the admin list response body.
type AffiliateAdminListPage struct {
	Data    []repository.AffiliateAdminListRow `json:"data"`
	Page    int                                `json:"page"`
	PerPage int                                `json:"per_page"`
	Total   int                                `json:"total"`
}

const (
	affiliateDetailBindingsLimit = 10
	affiliateDetailHistoryCap    = 50
	affiliateDetailViolationsCap = 50
)

// AffiliateAdminBindingsSummary counts the bindings by status and carries the
// newest affiliateDetailBindingsLimit rows, privacy-formatted.
type AffiliateAdminBindingsSummary struct {
	Active int                  `json:"active"`
	Ended  int                  `json:"ended"`
	Data   []AffiliateMeBinding `json:"data"`
}

// AffiliateAdminCommissionSummary counts commissions by status and totals
// their amounts.
type AffiliateAdminCommissionSummary struct {
	Counts      map[string]int `json:"counts"`
	TotalAmount float64        `json:"total_amount"`
}

// AffiliateAdminDetail is the GET /admin/affiliates/:id body. The bindings
// rows reuse the affiliator-facing privacy shape (BR-PRV-02).
type AffiliateAdminDetail struct {
	Affiliate    *model.Affiliator               `json:"affiliate"`
	Name         string                          `json:"name"`
	Email        *string                         `json:"email"`
	Bindings     AffiliateAdminBindingsSummary   `json:"bindings"`
	Commissions  AffiliateAdminCommissionSummary `json:"commissions"`
	Saldo        float64                         `json:"saldo"`
	LevelHistory []model.AffiliateLevelHistory   `json:"level_history"`
	Violations   []model.AffiliateViolation      `json:"violations"`
}

// ListAffiliateAdmin backs GET /admin/affiliates.
func (s *Service) ListAffiliateAdmin(ctx context.Context, filter AffiliateAdminListFilter) (*AffiliateAdminListPage, error) {
	if filter.Status != "" && !slices.Contains(affiliateAdminStatuses, filter.Status) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, filter.Status)
	}
	if filter.Level != "" && !affiliateValidLevel(filter.Level) {
		return nil, fmt.Errorf("%w: %s", ErrAffiliateInvalidLevel, filter.Level)
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	if filter.PerPage > 50 {
		filter.PerPage = 50
	}

	rows, total, err := s.storeRepo.ListAffiliateAdmins(ctx, repository.AffiliateAdminFilter{
		Status: filter.Status,
		Level:  filter.Level,
		Query:  filter.Query,
		Offset: (filter.Page - 1) * filter.PerPage,
		Limit:  filter.PerPage,
	})
	if err != nil {
		return nil, fmt.Errorf("list affiliators: %w", err)
	}
	return &AffiliateAdminListPage{
		Data:    rows,
		Page:    filter.Page,
		PerPage: filter.PerPage,
		Total:   total,
	}, nil
}

// DecideAffiliateApproval backs POST /admin/affiliates/:id/approval. Approval
// and rejection both start from pending_approval and run once: an
// already-approved profile is an invalid transition, so the referral code is
// never regenerated.
func (s *Service) DecideAffiliateApproval(ctx context.Context, actorID, id, decision, reason string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if decision != "approve" && decision != "reject" {
		return nil, fmt.Errorf("%w: %s", ErrAffiliateInvalidDecision, decision)
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if aff.Status != "pending_approval" {
		return nil, fmt.Errorf("%w: status %s", ErrAffiliateNotPending, aff.Status)
	}

	meta := map[string]any{"decision": decision}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	switch decision {
	case "approve":
		code, err := s.generateUniqueAffiliateCode(ctx)
		if err != nil {
			return nil, err
		}
		approved, err := s.storeRepo.ApproveAffiliatorTx(ctx, tx, affID, code)
		if err != nil {
			return nil, fmt.Errorf("approve affiliator: %w", err)
		}
		if !approved {
			return nil, ErrAffiliateNotPending
		}
		meta["code"] = code
	case "reject":
		if strings.TrimSpace(reason) == "" {
			return nil, ErrAffiliateRejectReasonRequired
		}
		rejected, err := s.storeRepo.RejectAffiliatorTx(ctx, tx, affID, reason)
		if err != nil {
			return nil, fmt.Errorf("reject affiliator: %w", err)
		}
		if !rejected {
			return nil, ErrAffiliateNotPending
		}
		meta["reason"] = reason
	}

	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditApproval, meta); err != nil {
		return nil, fmt.Errorf("write approval audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit approval: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

// ChangeAffiliateStatus backs PATCH /admin/affiliates/:id/status. A
// suspension lasts the given days (clamped to 1..365) counted from WIB now.
// Only an approved profile (active, suspended or deactivated) can move: a
// pending or rejected one has no referral code yet. Deactivation also cancels
// the commissions still waiting out their hold (BR-PLG-07); saldo already
// available is left alone (policy TBD in the BRD).
func (s *Service) ChangeAffiliateStatus(ctx context.Context, actorID, id, newStatus, reason string, days *int) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrAffiliateReasonRequired
	}
	if !slices.Contains(affiliateChangeStatuses, newStatus) {
		return nil, fmt.Errorf("%w: %s", ErrAffiliateInvalidStatus, newStatus)
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	if err := affiliateCheckStatusChange(aff, newStatus, days); err != nil {
		return nil, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.applyAffiliateStatusTx(ctx, tx, actorID, aff, newStatus, reason, days); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit status change: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

func affiliateCheckStatusChange(aff *model.Affiliator, newStatus string, days *int) error {
	if !slices.Contains(affiliateChangeStatuses, aff.Status) {
		return fmt.Errorf("%w: status %s", ErrAffiliateInvalidTransition, aff.Status)
	}
	if newStatus == "suspended" && days == nil {
		return ErrAffiliateInvalidDays
	}
	return nil
}

// affiliateSuspendUntil clamps the suspension length to 1..365 days and dates
// it from WIB now.
func affiliateSuspendUntil(days int) (time.Time, int) {
	if days < 1 {
		days = 1
	}
	if days > affiliateSuspendMaxDays {
		days = affiliateSuspendMaxDays
	}
	return time.Now().In(affiliateWIBLocation()).AddDate(0, 0, days), days
}

// applyAffiliateStatusTx writes the status change, its pending-commission
// cancellation on deactivation, and the audit row inside the caller's tx.
func (s *Service) applyAffiliateStatusTx(ctx context.Context, tx pgx.Tx, actorID string, aff *model.Affiliator, newStatus, reason string, days *int) error {
	var suspendedUntil *time.Time
	meta := map[string]any{"from": aff.Status, "to": newStatus, "reason": reason}
	if newStatus == "suspended" {
		until, clamped := affiliateSuspendUntil(*days)
		suspendedUntil = &until
		meta["days"] = clamped
		meta["suspended_until"] = until
	}
	if err := s.storeRepo.SetAffiliateStatusTx(ctx, tx, aff.ID, newStatus, suspendedUntil); err != nil {
		return fmt.Errorf("set affiliator status: %w", err)
	}
	if newStatus == "deactivated" {
		cancelled, err := s.storeRepo.CancelPendingCommissionsByAffiliatorTx(ctx, tx, aff.ID)
		if err != nil {
			return fmt.Errorf("cancel pending commissions: %w", err)
		}
		meta["commissions_cancelled"] = cancelled
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, aff.ID.String(), affiliateAuditStatusChange, meta); err != nil {
		return fmt.Errorf("write status change audit: %w", err)
	}
	return nil
}

// FreezeAffiliateBalance backs POST /admin/affiliates/:id/freeze. The days are
// clamped to 1..freeze_max_days; a non-positive request is rejected instead of
// silently freezing for a day the admin did not ask for.
func (s *Service) FreezeAffiliateBalance(ctx context.Context, actorID, id string, days int, reason string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrAffiliateReasonRequired
	}
	if days < 1 {
		return nil, ErrAffiliateInvalidDays
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.applyAffiliateFreezeTx(ctx, tx, actorID, affID, days, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit freeze: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

func (s *Service) applyAffiliateFreezeTx(ctx context.Context, tx pgx.Tx, actorID string, affID uuid.UUID, days int, reason string) error {
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return fmt.Errorf("load affiliate settings: %w", err)
	}
	max := settings.FreezeMaxDays
	if max < 1 {
		max = 1
	}
	if days > max {
		days = max
	}
	until := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, days)
	if err := s.storeRepo.SetAffiliateFrozenUntilTx(ctx, tx, affID, &until); err != nil {
		return fmt.Errorf("freeze affiliator balance: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditFreeze, map[string]any{
		"days":         days,
		"frozen_until": until,
		"reason":       reason,
	}); err != nil {
		return fmt.Errorf("write freeze audit: %w", err)
	}
	return nil
}

// UnfreezeAffiliateBalance backs POST /admin/affiliates/:id/unfreeze.
func (s *Service) UnfreezeAffiliateBalance(ctx context.Context, actorID, id, reason string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrAffiliateReasonRequired
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SetAffiliateFrozenUntilTx(ctx, tx, affID, nil); err != nil {
		return nil, fmt.Errorf("unfreeze affiliator balance: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditUnfreeze, map[string]any{
		"reason": reason,
	}); err != nil {
		return nil, fmt.Errorf("write unfreeze audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit unfreeze: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

// ChangeAffiliateLevel backs POST /admin/affiliates/:id/level. A level change
// writes one manual history row (BR-LVL-09/10) and resets the warn counter; a
// pure lock only sets level_locked_until. The lock date is inclusive: the
// level stays locked through the end of that WIB day.
func (s *Service) ChangeAffiliateLevel(ctx context.Context, actorID, id, level, lockUntil, reason string) (*model.Affiliator, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrAffiliateReasonRequired
	}
	lock, err := affiliateValidateLevelChange(level, lockUntil)
	if err != nil {
		return nil, err
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.applyAffiliateLevelTx(ctx, tx, actorID, aff, level, lock, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit level change: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
}

// affiliateValidateLevelChange checks the requested level and parses the
// inclusive lock date (end of that WIB day).
func affiliateValidateLevelChange(level, lockUntil string) (*time.Time, error) {
	if level == "" && lockUntil == "" {
		return nil, fmt.Errorf("%w: level or lock_until", ErrMissingField)
	}
	var lock *time.Time
	if lockUntil != "" {
		date, err := time.Parse("2006-01-02", lockUntil)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidDateFormat, lockUntil)
		}
		until := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, affiliateWIBLocation()).AddDate(0, 0, 1)
		lock = &until
	}
	if level != "" && !affiliateValidLevel(level) {
		return nil, fmt.Errorf("%w: %s", ErrAffiliateInvalidLevel, level)
	}
	return lock, nil
}

func (s *Service) applyAffiliateLevelTx(ctx context.Context, tx pgx.Tx, actorID string, aff *model.Affiliator, level string, lock *time.Time, reason string) error {
	actor, err := parseUUID(actorID)
	if err != nil {
		return fmt.Errorf("parse actor: %w", err)
	}
	meta := map[string]any{"reason": reason}
	if level != "" {
		if err := s.storeRepo.UpdateAffiliateLevelTx(ctx, tx, aff.ID, level, 0); err != nil {
			return fmt.Errorf("update affiliate level: %w", err)
		}
		from := aff.Level
		if err := s.storeRepo.InsertAffiliateLevelHistoryManualTx(ctx, tx, &model.AffiliateLevelHistory{
			AffiliatorID: aff.ID,
			Quarter:      affiliateQuarterKey(time.Now().In(affiliateWIBLocation())),
			FromLevel:    &from,
			ToLevel:      level,
			Source:       affiliateLevelSourceManual,
			Reason:       &reason,
			CreatedBy:    &actor,
		}); err != nil {
			return fmt.Errorf("insert level history: %w", err)
		}
		meta["from"] = from
		meta["to"] = level
	}
	if lock != nil {
		if err := s.storeRepo.SetAffiliateLevelLockedUntilTx(ctx, tx, aff.ID, lock); err != nil {
			return fmt.Errorf("lock affiliate level: %w", err)
		}
		meta["lock_until"] = *lock
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, aff.ID.String(), affiliateAuditLevelChange, meta); err != nil {
		return fmt.Errorf("write level change audit: %w", err)
	}
	return nil
}

// GetAffiliateAdminDetail backs GET /admin/affiliates/:id: one affiliator's
// profile plus the bindings, commissions, saldo, level history and violations
// summaries the detail tabs render.
func (s *Service) GetAffiliateAdminDetail(ctx context.Context, id string) (*AffiliateAdminDetail, error) {
	affID, err := parseUUID(id)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}

	contact, err := s.storeRepo.GetAffiliateAdminUserContact(ctx, aff.UserID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator user: %w", err)
	}
	detail := &AffiliateAdminDetail{
		Affiliate:   aff,
		Name:        contact.Name,
		Email:       contact.Email,
		Bindings:    AffiliateAdminBindingsSummary{Data: []AffiliateMeBinding{}},
		Commissions: AffiliateAdminCommissionSummary{Counts: map[string]int{}},
	}

	detail.Bindings.Active, err = s.storeRepo.CountAffiliateBindingsForAffiliator(ctx, affID, "active")
	if err != nil {
		return nil, fmt.Errorf("count active affiliate bindings: %w", err)
	}
	detail.Bindings.Ended, err = s.storeRepo.CountAffiliateBindingsForAffiliator(ctx, affID, "ended")
	if err != nil {
		return nil, fmt.Errorf("count ended affiliate bindings: %w", err)
	}
	rows, err := s.storeRepo.ListAffiliateBindingsForAffiliator(ctx, affID, "", affiliateDetailBindingsLimit, 0)
	if err != nil {
		return nil, fmt.Errorf("list affiliate bindings: %w", err)
	}
	for _, row := range rows {
		detail.Bindings.Data = append(detail.Bindings.Data, AffiliateMeBinding{
			BindingID:    row.BindingID,
			Name:         affiliateStudentDisplayName(row.StudentName),
			RegisteredAt: row.RegisteredAt,
			Status:       row.Status,
			Source:       row.Source,
			StartedAt:    row.StartedAt,
			EndedAt:      row.EndedAt,
			TotalSpend:   row.TotalSpend,
		})
	}

	totals, err := s.storeRepo.SumAffiliateCommissionsByStatus(ctx, affID)
	if err != nil {
		return nil, fmt.Errorf("sum affiliate commissions by status: %w", err)
	}
	for _, t := range totals {
		detail.Commissions.Counts[t.Status] = t.Count
		detail.Commissions.TotalAmount += t.Amount
	}

	detail.Saldo, err = s.storeRepo.SumAffiliateLedgerForAffiliator(ctx, affID)
	if err != nil {
		return nil, fmt.Errorf("sum affiliate ledger: %w", err)
	}
	detail.LevelHistory, err = s.storeRepo.ListAffiliateLevelHistory(ctx, affID, affiliateDetailHistoryCap)
	if err != nil {
		return nil, fmt.Errorf("list affiliate level history: %w", err)
	}
	if detail.LevelHistory == nil {
		detail.LevelHistory = []model.AffiliateLevelHistory{}
	}
	detail.Violations, _, err = s.storeRepo.ListAffiliateViolationsForAffiliator(ctx, affID, affiliateDetailViolationsCap, 0)
	if err != nil {
		return nil, fmt.Errorf("list affiliate violations: %w", err)
	}
	if detail.Violations == nil {
		detail.Violations = []model.AffiliateViolation{}
	}
	return detail, nil
}

// generateAffiliateReferralCode returns one random 8-char referral code from
// the unambiguous charset.
func generateAffiliateReferralCode() (string, error) {
	code := make([]byte, affiliateCodeLen)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(affiliateCodeChars))))
		if err != nil {
			return "", err
		}
		code[i] = affiliateCodeChars[n.Int64()]
	}
	return string(code), nil
}

// generateUniqueAffiliateCode retries on collision with the affiliators.code
// unique column, up to affiliateCodeTries attempts.
func (s *Service) generateUniqueAffiliateCode(ctx context.Context) (string, error) {
	for i := 0; i < affiliateCodeTries; i++ {
		code, err := generateAffiliateReferralCode()
		if err != nil {
			return "", err
		}
		taken, err := s.storeRepo.AffiliateCodeExists(ctx, code)
		if err != nil {
			return "", fmt.Errorf("check referral code: %w", err)
		}
		if !taken {
			return code, nil
		}
	}
	return "", ErrAffiliateCodeGenerationExhausted
}

func affiliateValidLevel(level string) bool {
	return slices.Contains(affiliateLevels, level)
}
