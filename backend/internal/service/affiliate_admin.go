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
		if err := s.storeRepo.ApproveAffiliatorTx(ctx, tx, affID, code); err != nil {
			return nil, fmt.Errorf("approve affiliator: %w", err)
		}
		meta["code"] = code
	case "reject":
		if strings.TrimSpace(reason) == "" {
			return nil, ErrAffiliateRejectReasonRequired
		}
		if err := s.storeRepo.RejectAffiliatorTx(ctx, tx, affID, reason); err != nil {
			return nil, fmt.Errorf("reject affiliator: %w", err)
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
// suspension lasts the given days (clamped to 1..365) counted from WIB now;
// deactivation is a plain status write — pending-commission handling is
// deferred as a policy question (BR-PLG-07) and C1 already stops new
// commissions for non-active accounts.
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

	var suspendedUntil *time.Time
	var clamped int
	if newStatus == "suspended" {
		if days == nil {
			return nil, ErrAffiliateInvalidDays
		}
		clamped = *days
		if clamped < 1 {
			clamped = 1
		}
		if clamped > affiliateSuspendMaxDays {
			clamped = affiliateSuspendMaxDays
		}
		until := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, clamped)
		suspendedUntil = &until
	}

	meta := map[string]any{"from": aff.Status, "to": newStatus, "reason": reason}
	if suspendedUntil != nil {
		meta["days"] = clamped
		meta["suspended_until"] = *suspendedUntil
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SetAffiliateStatusTx(ctx, tx, affID, newStatus, suspendedUntil); err != nil {
		return nil, fmt.Errorf("set affiliator status: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditStatusChange, meta); err != nil {
		return nil, fmt.Errorf("write status change audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit status change: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
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
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate settings: %w", err)
	}
	max := settings.FreezeMaxDays
	if max < 1 {
		max = 1
	}
	if days > max {
		days = max
	}
	until := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, days)

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SetAffiliateFrozenUntilTx(ctx, tx, affID, &until); err != nil {
		return nil, fmt.Errorf("freeze affiliator balance: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditFreeze, map[string]any{
		"days":         days,
		"frozen_until": until,
		"reason":       reason,
	}); err != nil {
		return nil, fmt.Errorf("write freeze audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit freeze: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
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
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateNotFound
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, fmt.Errorf("parse actor: %w", err)
	}

	meta := map[string]any{"reason": reason}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if level != "" {
		if err := s.storeRepo.UpdateAffiliateLevelTx(ctx, tx, affID, level, 0); err != nil {
			return nil, fmt.Errorf("update affiliate level: %w", err)
		}
		from := aff.Level
		if err := s.storeRepo.InsertAffiliateLevelHistoryManualTx(ctx, tx, &model.AffiliateLevelHistory{
			AffiliatorID: affID,
			Quarter:      affiliateQuarterKey(time.Now().In(affiliateWIBLocation())),
			FromLevel:    &from,
			ToLevel:      level,
			Source:       affiliateLevelSourceManual,
			Reason:       &reason,
			CreatedBy:    &actor,
		}); err != nil {
			return nil, fmt.Errorf("insert level history: %w", err)
		}
		meta["from"] = from
		meta["to"] = level
	}
	if lock != nil {
		if err := s.storeRepo.SetAffiliateLevelLockedUntilTx(ctx, tx, affID, lock); err != nil {
			return nil, fmt.Errorf("lock affiliate level: %w", err)
		}
		meta["lock_until"] = *lock
	}

	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, affiliateAuditTargetType, affID.String(), affiliateAuditLevelChange, meta); err != nil {
		return nil, fmt.Errorf("write level change audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit level change: %w", err)
	}
	return s.storeRepo.GetAffiliatorByID(ctx, nil, affID)
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
