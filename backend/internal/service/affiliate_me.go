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
)

// Feature-local sentinels (see affiliate_jobs.go for the precedent); the
// parent owns their mapServiceError cases.
var (
	ErrAffiliateProfileNotFound = errors.New("affiliator profile not found")
	ErrAffiliateNotActive       = errors.New("affiliator account is not active")
	ErrAffiliateProgramOff      = errors.New("affiliate program is switched off")
)

const affiliateReferralLinkPrefix = "/r/"

// AffiliateProgramEnabled reports the program switch for the public status endpoint.
func (s *Service) AffiliateProgramEnabled(ctx context.Context) bool {
	return s.affiliateEnabled(ctx)
}

// requireAffiliateProgramOn refuses affiliator-initiated changes while the
// program is switched off.
func (s *Service) requireAffiliateProgramOn(ctx context.Context) error {
	if !s.affiliateEnabled(ctx) {
		return ErrAffiliateProgramOff
	}
	return nil
}

type AffiliateMeLevelProgress struct {
	Quarter       string   `json:"quarter"`
	NetRevenue    float64  `json:"net_revenue"`
	NextLevel     string   `json:"next_level,omitempty"`
	RiseThreshold *float64 `json:"rise_threshold,omitempty"`
	HoldThreshold *float64 `json:"hold_threshold,omitempty"`
}

type AffiliateMeProfile struct {
	Code              string                    `json:"code"`
	ReferralLink      string                    `json:"referral_link"`
	Level             string                    `json:"level"`
	Status            string                    `json:"status"`
	ProgramEnabled    bool                      `json:"program_enabled"`
	RejectionReason   *string                   `json:"rejection_reason,omitempty"`
	WarnQuarters      int                       `json:"warn_quarters"`
	BankName          *string                   `json:"bank_name"`
	BankAccountNumber *string                   `json:"bank_account_number"`
	BankAccountHolder *string                   `json:"bank_account_holder"`
	TermsVersion      string                    `json:"terms_version"`
	TermsAcceptedAt   time.Time                 `json:"terms_accepted_at"`
	LevelProgress     *AffiliateMeLevelProgress `json:"level_progress"`
}

type AffiliateMeListFilter struct {
	Page        int
	PerPage     int
	Query       string
	Status      string
	Type        string
	ProductType string
	Order       string
	From        *time.Time
	To          *time.Time
}

func (f AffiliateMeListFilter) repoQuery() repository.AffiliateListQuery {
	return repository.AffiliateListQuery{Q: f.Query, Type: f.Type, Status: f.Status, From: f.From, To: f.To}
}

// checkAffiliateFilter rejects a status or type outside the list's known values
// (an empty value means "no filter").
func checkAffiliateFilter(allowed map[string]bool, value string) error {
	if value != "" && !allowed[value] {
		return fmt.Errorf("%w: %s", ErrInvalidStatusFilter, value)
	}
	return nil
}

var (
	affiliateMeBindingSources = map[string]bool{"link": true, "manual_code": true, "checkout_code": true}
	affiliateMeLedgerTypes    = map[string]bool{"commission": true, "refund_deduction": true, "adjustment": true, "payout": true, "payout_reversal": true}
)

// AffiliateMeBinding is one affiliator-facing student row: name is
// privacy-formatted and carries no contact or exam data (BR-PRV-02).
type AffiliateMeBinding struct {
	BindingID    uuid.UUID  `json:"binding_id"`
	Name         string     `json:"name"`
	RegisteredAt time.Time  `json:"registered_at"`
	Status       string     `json:"status"`
	Source       string     `json:"source"`
	StartedAt    time.Time  `json:"started_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	TotalSpend   float64    `json:"total_spend"`
}

type AffiliateMeBindingsPage struct {
	Data    []AffiliateMeBinding `json:"data"`
	Page    int                  `json:"page"`
	PerPage int                  `json:"per_page"`
	Total   int                  `json:"total"`
}

type AffiliateMeCommissionsPage struct {
	Data    []repository.AffiliateCommissionRow `json:"data"`
	Page    int                                 `json:"page"`
	PerPage int                                 `json:"per_page"`
	Total   int                                 `json:"total"`
}

type AffiliateMeLedgerPage struct {
	Data    []repository.AffiliateLedgerRow `json:"data"`
	Saldo   float64                         `json:"saldo"`
	Page    int                             `json:"page"`
	PerPage int                             `json:"per_page"`
	Total   int                             `json:"total"`
}

type AffiliateMeStats struct {
	Registrations         int            `json:"registrations"`
	RegistrationsBySource map[string]int `json:"registrations_by_source"`
	TotalStudents         int            `json:"total_students"`
	ActiveBindings        int            `json:"active_bindings"`
	NewStudents           int            `json:"new_students"`
	CommissionEarned      float64        `json:"commission_earned"`
}

// affiliateStudentDisplayName renders a student's name as first name plus the
// initials of the remaining words ("Budi Santoso" → "Budi S."), BR-PRV-02.
func affiliateStudentDisplayName(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	initials := make([]string, 0, len(words)-1)
	for _, word := range words[1:] {
		runes := []rune(word)
		initials = append(initials, strings.ToUpper(string(runes[0]))+".")
	}
	if len(initials) == 0 {
		return words[0]
	}
	return words[0] + " " + strings.Join(initials, " ")
}

// affiliateMeAffiliator resolves the affiliators row behind the JWT user_id —
// the privacy boundary for every /affiliates/me read (BR-PRV-01). Reads keep
// working while the program is switched off; the writes call
// requireAffiliateProgramOn.
func (s *Service) affiliateMeAffiliator(ctx context.Context, userID string) (*model.Affiliator, error) {
	aff, err := s.storeRepo.GetAffiliatorByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get affiliator by user: %w", err)
	}
	if aff == nil {
		return nil, ErrAffiliateProfileNotFound
	}
	return aff, nil
}

// ListMyAffiliatePromos lists the still-valid promo codes meant for affiliators
// to share with their students. Only an active affiliator sees them.
func (s *Service) ListMyAffiliatePromos(ctx context.Context, userID string) ([]model.PromoCode, error) {
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return nil, err
	}
	if aff.Status != affiliateAffiliatorStatusActive || !s.affiliateEnabled(ctx) {
		return []model.PromoCode{}, nil
	}
	return s.storeRepo.ListAffiliatorOnlyPromos(ctx)
}

func (s *Service) GetMyAffiliateProfile(ctx context.Context, userID string) (AffiliateMeProfile, error) {
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return AffiliateMeProfile{}, err
	}
	referralLink := ""
	if aff.Code != "" {
		referralLink = affiliateReferralLinkPrefix + aff.Code
	}
	profile := AffiliateMeProfile{
		Code:              aff.Code,
		ReferralLink:      referralLink,
		Level:             aff.Level,
		Status:            aff.Status,
		ProgramEnabled:    s.affiliateEnabled(ctx),
		RejectionReason:   aff.RejectionReason,
		WarnQuarters:      aff.WarnQuarters,
		BankName:          aff.BankName,
		BankAccountNumber: aff.BankAccountNumber,
		BankAccountHolder: aff.BankAccountHolder,
		TermsVersion:      aff.TermsVersion,
		TermsAcceptedAt:   aff.CreatedAt,
	}

	_, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return profile, fmt.Errorf("load affiliate effective rules: %w", err)
	}
	nowWIB := time.Now().In(affiliateWIBLocation())
	quarter := affiliateQuarterKey(nowWIB)
	start, end, err := affiliateQuarterRange(quarter, nowWIB.Location())
	if err != nil {
		return profile, err
	}
	net, err := s.storeRepo.GetAffiliateQuarterNetRevenue(ctx, aff.ID, start, end)
	if err != nil {
		return profile, fmt.Errorf("sum quarter net revenue: %w", err)
	}
	progress := &AffiliateMeLevelProgress{
		Quarter:       quarter,
		NetRevenue:    net,
		HoldThreshold: rules[aff.Level].HoldThreshold,
	}
	if next, ok := affiliateNextLevel(aff.Level); ok {
		rise := affiliateRiseThreshold(next, rules)
		progress.NextLevel = next
		progress.RiseThreshold = &rise
	}
	profile.LevelProgress = progress
	return profile, nil
}

func (s *Service) UpdateMyAffiliateBank(ctx context.Context, userID string, bankName, bankAccountNumber, bankAccountHolder string) (AffiliateMeProfile, error) {
	if err := s.requireAffiliateProgramOn(ctx); err != nil {
		return AffiliateMeProfile{}, err
	}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return AffiliateMeProfile{}, err
	}
	if aff.Status != affiliateAffiliatorStatusActive {
		return AffiliateMeProfile{}, ErrAffiliateNotActive
	}
	bankName = strings.TrimSpace(bankName)
	bankAccountNumber = strings.TrimSpace(bankAccountNumber)
	bankAccountHolder = strings.TrimSpace(bankAccountHolder)
	if bankName == "" || bankAccountNumber == "" || bankAccountHolder == "" {
		return AffiliateMeProfile{}, fmt.Errorf("%w: bank_name, bank_account_number, bank_account_holder", ErrMissingField)
	}
	if err := s.storeRepo.UpdateAffiliatorBank(ctx, aff.ID, bankName, bankAccountNumber, bankAccountHolder); err != nil {
		return AffiliateMeProfile{}, fmt.Errorf("update affiliator bank: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &userID, "affiliator", aff.ID.String(), "affiliate.bank_update", map[string]any{
		"bank_name":           bankName,
		"bank_account_number": bankAccountNumber,
		"bank_account_holder": bankAccountHolder,
	}); err != nil {
		return AffiliateMeProfile{}, fmt.Errorf("audit affiliate bank update: %w", err)
	}
	return s.GetMyAffiliateProfile(ctx, userID)
}

var affiliateMeBindingStatuses = map[string]bool{"active": true, "ended": true}

func (s *Service) ListMyAffiliateBindings(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMeBindingsPage, error) {
	page := AffiliateMeBindingsPage{Data: []AffiliateMeBinding{}, Page: filter.Page, PerPage: filter.PerPage}
	if err := checkAffiliateFilter(affiliateMeBindingStatuses, filter.Status); err != nil {
		return page, err
	}
	if err := checkAffiliateFilter(affiliateMeBindingSources, filter.Type); err != nil {
		return page, err
	}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}
	total, err := s.storeRepo.CountAffiliateBindingsForAffiliator(ctx, aff.ID, filter.repoQuery())
	if err != nil {
		return page, fmt.Errorf("count affiliate bindings: %w", err)
	}
	page.Total = total
	rows, err := s.storeRepo.ListAffiliateBindingsForAffiliator(ctx, aff.ID, filter.repoQuery(), filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return page, fmt.Errorf("list affiliate bindings: %w", err)
	}
	for _, row := range rows {
		page.Data = append(page.Data, AffiliateMeBinding{
			BindingID:    row.BindingID,
			Name:         affiliateStudentDisplayName(row.StudentName),
			RegisteredAt: row.RegisteredAt,
			Status:       row.Status,
			Source:       row.Source,
			StartedAt:    row.StartedAt,
			ExpiresAt:    row.ExpiresAt,
			TotalSpend:   row.TotalSpend,
		})
	}
	return page, nil
}

var affiliateMeCommissionStatuses = map[string]bool{"pending": true, "available": true, "cancelled": true}

func (s *Service) ListMyAffiliateCommissions(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMeCommissionsPage, error) {
	page := AffiliateMeCommissionsPage{Data: []repository.AffiliateCommissionRow{}, Page: filter.Page, PerPage: filter.PerPage}
	if filter.Status != "" && !affiliateMeCommissionStatuses[filter.Status] {
		return page, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, filter.Status)
	}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}
	repoFilter := repository.AffiliateCommissionFilter{
		Q:           filter.Query,
		Status:      filter.Status,
		ProductType: filter.ProductType,
		PaidFrom:    filter.From,
		PaidTo:      filter.To,
		Limit:       filter.PerPage,
		Offset:      (filter.Page - 1) * filter.PerPage,
	}
	total, err := s.storeRepo.CountAffiliateCommissionsForAffiliator(ctx, aff.ID, repoFilter)
	if err != nil {
		return page, fmt.Errorf("count affiliate commissions: %w", err)
	}
	page.Total = total
	data, err := s.storeRepo.ListAffiliateCommissionsForAffiliator(ctx, aff.ID, repoFilter)
	if err != nil {
		return page, fmt.Errorf("list affiliate commissions: %w", err)
	}
	for i := range data {
		data[i].StudentName = affiliateStudentDisplayName(data[i].StudentName)
	}
	page.Data = data
	return page, nil
}

func (s *Service) GetMyAffiliateLedger(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMeLedgerPage, error) {
	page := AffiliateMeLedgerPage{Data: []repository.AffiliateLedgerRow{}, Page: filter.Page, PerPage: filter.PerPage}
	if err := checkAffiliateFilter(affiliateMeLedgerTypes, filter.Type); err != nil {
		return page, err
	}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}
	total, err := s.storeRepo.CountAffiliateLedgerEntriesForAffiliator(ctx, aff.ID, filter.repoQuery())
	if err != nil {
		return page, fmt.Errorf("count affiliate ledger: %w", err)
	}
	page.Total = total
	saldo, err := s.storeRepo.SumAffiliateLedgerForAffiliator(ctx, aff.ID)
	if err != nil {
		return page, fmt.Errorf("sum affiliate ledger: %w", err)
	}
	page.Saldo = saldo
	data, err := s.storeRepo.ListAffiliateLedgerForAffiliator(ctx, aff.ID, filter.Order == "asc", filter.repoQuery(), filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return page, fmt.Errorf("list affiliate ledger: %w", err)
	}
	for i := range data {
		if data[i].StudentName != nil {
			name := affiliateStudentDisplayName(*data[i].StudentName)
			data[i].StudentName = &name
		}
		// The stored note is internal bookkeeping text (it carries order ids);
		// only an adjustment's note is a reason written for the affiliator.
		if data[i].EntryType != "adjustment" {
			data[i].Note = nil
		}
	}
	page.Data = data
	return page, nil
}

// GetMyAffiliateStats counts registrations, new students and earned
// commission inside the optional [from, to) range; without one it is all time.
func (s *Service) GetMyAffiliateStats(ctx context.Context, userID string, from, to *time.Time) (AffiliateMeStats, error) {
	stats := AffiliateMeStats{}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return stats, err
	}
	counts, err := s.storeRepo.GetAffiliateMeCounts(ctx, aff.ID, from, to)
	if err != nil {
		return stats, fmt.Errorf("count affiliate stats: %w", err)
	}
	stats.Registrations = counts.Registrations
	stats.TotalStudents = counts.TotalStudents
	stats.ActiveBindings = counts.ActiveBindings
	stats.NewStudents = counts.NewStudents
	stats.CommissionEarned = counts.CommissionEarned
	stats.RegistrationsBySource, err = s.storeRepo.CountAffiliateBindingsBySource(ctx, aff.ID)
	if err != nil {
		return stats, fmt.Errorf("count affiliate bindings by source: %w", err)
	}
	return stats, nil
}
