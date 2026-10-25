package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// Feature-local sentinels for the affiliator money/violation surfaces
// (BR-PCR-01..05, BR-PLG-06); the parent owns their mapServiceError cases.
var (
	ErrAffiliateFrozen              = errors.New("affiliator balance is frozen")
	ErrAffiliateBankMissing         = errors.New("bank account is not set")
	ErrAffiliatePayoutBelowMin      = errors.New("saldo is below the minimum payout")
	ErrAffiliatePayoutInFlight      = errors.New("a payout is already requested or processing")
	ErrAffiliatePayoutAmountInvalid = errors.New("payout amount must be at least the minimum payout and at most the saldo")
	ErrAffiliateNotAppealable       = errors.New("violation is not appealable")
)

type AffiliateMePayoutPage struct {
	Data             []model.AffiliatePayout `json:"data"`
	Saldo            float64                 `json:"saldo"`
	MinPayout        float64                 `json:"min_payout"`
	PayoutDayOfMonth *int                    `json:"payout_day_of_month,omitempty"`
	Page             int                     `json:"page"`
	PerPage          int                     `json:"per_page"`
	Total            int                     `json:"total"`
}

type AffiliateMeAdjustmentsPage struct {
	Data    []repository.AffiliateAdjustmentWithApplied `json:"data"`
	Page    int                                         `json:"page"`
	PerPage int                                         `json:"per_page"`
	Total   int                                         `json:"total"`
}

type AffiliateMeViolationsPage struct {
	Data    []model.AffiliateViolation `json:"data"`
	Page    int                        `json:"page"`
	PerPage int                        `json:"per_page"`
	Total   int                        `json:"total"`
}

// affiliateBankIncomplete is BR-PCR-02: all three bank fields must be filled
// (PATCH /affiliates/me/bank) before a payout can be requested.
func affiliateBankIncomplete(aff *model.Affiliator) bool {
	return aff.BankName == nil || strings.TrimSpace(*aff.BankName) == "" ||
		aff.BankAccountNumber == nil || strings.TrimSpace(*aff.BankAccountNumber) == "" ||
		aff.BankAccountHolder == nil || strings.TrimSpace(*aff.BankAccountHolder) == ""
}

// RequestMyAffiliatePayout files one payout request (BR-PCR-01..05): active
// status, no freeze, no requested/processing payout in flight, saldo at or
// above the level's min_payout, bank fields complete. A nil amount defaults
// to the full saldo. One transaction writes the payout row (bank snapshot),
// the negative ledger deduction the admin decide path reverses on rejection,
// and the audit row; the affiliator row is locked FOR UPDATE so two
// concurrent requests cannot both pass the in-flight check.
func (s *Service) RequestMyAffiliatePayout(ctx context.Context, userID string, amount *float64) (*model.AffiliatePayout, error) {
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return nil, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin payout request tx: %w", err)
	}
	defer tx.Rollback(ctx)

	fresh, err := s.storeRepo.GetAffiliatorForUpdateTx(ctx, tx, aff.ID)
	if err != nil {
		return nil, fmt.Errorf("lock affiliator: %w", err)
	}
	if fresh == nil || fresh.Status != affiliateAffiliatorStatusActive {
		return nil, ErrAffiliateNotActive
	}
	if fresh.FrozenUntil != nil && fresh.FrozenUntil.After(time.Now()) {
		return nil, ErrAffiliateFrozen
	}
	inFlight, err := s.storeRepo.HasAffiliatePayoutInFlightTx(ctx, tx, fresh.ID)
	if err != nil {
		return nil, fmt.Errorf("check payout in flight: %w", err)
	}
	if inFlight {
		return nil, ErrAffiliatePayoutInFlight
	}

	_, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate effective rules: %w", err)
	}
	rule := rules[fresh.Level]

	saldo, err := s.storeRepo.SumAffiliateLedgerTx(ctx, tx, fresh.ID)
	if err != nil {
		return nil, fmt.Errorf("sum affiliate ledger: %w", err)
	}
	if saldo < rule.MinPayout {
		return nil, ErrAffiliatePayoutBelowMin
	}
	if affiliateBankIncomplete(fresh) {
		return nil, ErrAffiliateBankMissing
	}

	payoutAmount := saldo
	if amount != nil {
		if *amount <= 0 || *amount > saldo || *amount < rule.MinPayout {
			return nil, ErrAffiliatePayoutAmountInvalid
		}
		payoutAmount = *amount
	}

	payout := &model.AffiliatePayout{
		AffiliatorID:      fresh.ID,
		Amount:            payoutAmount,
		Status:            "requested",
		BankName:          *fresh.BankName,
		BankAccountNumber: *fresh.BankAccountNumber,
		BankAccountHolder: *fresh.BankAccountHolder,
	}
	if err := s.storeRepo.InsertAffiliatePayoutTx(ctx, tx, payout); err != nil {
		return nil, fmt.Errorf("insert affiliate payout: %w", err)
	}
	note := "payout requested"
	if err := s.storeRepo.InsertAffiliateLedgerEntryTx(ctx, tx, &model.AffiliateLedgerEntry{
		AffiliatorID: fresh.ID,
		EntryType:    "payout",
		Amount:       -payoutAmount,
		PayoutID:     &payout.ID,
		Note:         &note,
		ActorID:      &fresh.UserID,
	}); err != nil {
		return nil, fmt.Errorf("insert payout ledger entry: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &userID, "affiliate_payout", payout.ID.String(), "affiliate.payout_requested", map[string]any{
		"affiliator_id": fresh.ID,
		"amount":        payoutAmount,
	}); err != nil {
		return nil, fmt.Errorf("insert payout request audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit payout request: %w", err)
	}
	return payout, nil
}

// ListMyAffiliatePayouts returns the affiliator's own payout history newest
// first, with the saldo and the level's payout schedule (min_payout,
// payout_day_of_month) so the UI can explain when transfers happen.
func (s *Service) ListMyAffiliatePayouts(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMePayoutPage, error) {
	page := AffiliateMePayoutPage{Data: []model.AffiliatePayout{}, Page: filter.Page, PerPage: filter.PerPage}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}

	_, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return page, fmt.Errorf("load affiliate effective rules: %w", err)
	}
	rule := rules[aff.Level]
	page.MinPayout = rule.MinPayout
	page.PayoutDayOfMonth = rule.PayoutDayOfMonth

	saldo, err := s.storeRepo.SumAffiliateLedgerForAffiliator(ctx, aff.ID)
	if err != nil {
		return page, fmt.Errorf("sum affiliate ledger: %w", err)
	}
	page.Saldo = saldo

	rows, total, err := s.storeRepo.ListAffiliatePayoutsForAffiliator(ctx, aff.ID, filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return page, fmt.Errorf("list affiliate payouts: %w", err)
	}
	page.Total = total
	page.Data = rows
	return page, nil
}

// ListMyAffiliateAdjustments returns the affiliator's own adjustments newest
// first; Applied marks the rows whose ledger entry exists (Fase 1 applies
// immediately, a threshold-pending row has none).
func (s *Service) ListMyAffiliateAdjustments(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMeAdjustmentsPage, error) {
	page := AffiliateMeAdjustmentsPage{Data: []repository.AffiliateAdjustmentWithApplied{}, Page: filter.Page, PerPage: filter.PerPage}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}
	rows, total, err := s.storeRepo.ListAffiliateAdjustmentsWithApplied(ctx, aff.ID, filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return page, fmt.Errorf("list affiliate adjustments: %w", err)
	}
	page.Total = total
	page.Data = rows
	return page, nil
}

// ListMyAffiliateViolations returns the affiliator's own violations newest
// first, with the sanction and appeal fields.
func (s *Service) ListMyAffiliateViolations(ctx context.Context, userID string, filter AffiliateMeListFilter) (AffiliateMeViolationsPage, error) {
	page := AffiliateMeViolationsPage{Data: []model.AffiliateViolation{}, Page: filter.Page, PerPage: filter.PerPage}
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return page, err
	}
	rows, total, err := s.storeRepo.ListAffiliateViolationsForAffiliator(ctx, aff.ID, filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return page, fmt.Errorf("list affiliate violations: %w", err)
	}
	page.Total = total
	page.Data = rows
	return page, nil
}

// AppealMyAffiliateViolation records an appeal on one violation (BR-PLG-06):
// only a resolved violation with no appeal yet accepts one, and it moves only
// to 'pending' — deciding the appeal stays an admin action.
func (s *Service) AppealMyAffiliateViolation(ctx context.Context, userID, violationID, text string) (*model.AffiliateViolation, error) {
	aff, err := s.affiliateMeAffiliator(ctx, userID)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(violationID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: text", ErrMissingField)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin appeal tx: %w", err)
	}
	defer tx.Rollback(ctx)

	violation, err := s.storeRepo.GetAffiliateViolationForUpdateTx(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("lock affiliate violation: %w", err)
	}
	if violation == nil || violation.AffiliatorID != aff.ID {
		return nil, ErrAffiliateNotFound
	}
	if violation.Status != "resolved" || violation.AppealStatus != "none" {
		return nil, ErrAffiliateNotAppealable
	}

	updated, err := s.storeRepo.UpdateAffiliateViolationAppealTx(ctx, tx, id, text)
	if err != nil {
		return nil, fmt.Errorf("record violation appeal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit appeal: %w", err)
	}
	return updated, nil
}
