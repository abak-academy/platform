package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

const (
	affiliateAuditSettings = "affiliate.settings_update"
	affiliateAuditOverride = "affiliate.product_override"
)

// AffiliateAdminLevelInput is one level row in the PUT settings body; nil
// optional fields clear to NULL because the PUT is a full replace.
type AffiliateAdminLevelInput struct {
	Level            string                                   `json:"level"`
	CommissionRule   map[string]model.AffiliateCommissionRule `json:"commission_rule"`
	BindingMonths    *int                                     `json:"binding_months"`
	DigitalWaitDays  *int                                     `json:"digital_wait_days"`
	PhysicalWaitDays *int                                     `json:"physical_wait_days"`
	MinPayout        *float64                                 `json:"min_payout"`
	PayoutDayOfMonth *int                                     `json:"payout_day_of_month"`
	RiseThreshold    *float64                                 `json:"rise_threshold"`
	HoldThreshold    *float64                                 `json:"hold_threshold"`
}

// AffiliateAdminSettingsInput is the PUT /admin/affiliate-settings body: the
// global singleton plus optional per-level rows.
type AffiliateAdminSettingsInput struct {
	CookieDays           int                        `json:"cookie_days"`
	BindingMonths        int                        `json:"binding_months"`
	MinExtendAmount      float64                    `json:"min_extend_amount"`
	DigitalWaitDays      int                        `json:"digital_wait_days"`
	PhysicalWaitDays     int                        `json:"physical_wait_days"`
	MinPayout            float64                    `json:"min_payout"`
	PayoutDayOfMonth     *int                       `json:"payout_day_of_month"`
	LaunchDate           *string                    `json:"launch_date"`
	EvalDay              int                        `json:"eval_day"`
	DailyRegistrationCap *int                       `json:"daily_registration_cap"`
	Levels               []AffiliateAdminLevelInput `json:"levels"`
}

// AffiliateAdminSettingsView is the GET/PUT affiliate-settings response: the
// global singleton plus all three level rows (missing rows synthesized empty).
type AffiliateAdminSettingsView struct {
	Settings       *model.AffiliateSettings       `json:"settings"`
	Levels         []model.AffiliateLevelSettings `json:"levels"`
	ProgramEnabled bool                           `json:"program_enabled"`
}

// AffiliateReportSummary is the GET /admin/affiliate-reports/summary response.
type AffiliateReportSummary struct {
	Data    []repository.AffiliateReportRow `json:"data"`
	Totals  repository.AffiliateReportRow   `json:"totals"`
	Total   int                             `json:"total"`
	Page    int                             `json:"page,omitempty"`
	PerPage int                             `json:"per_page,omitempty"`
}

// AffiliateReportOptions narrows and pages the report. A zero Limit returns
// every matching row; the totals always cover every matching row, not just the page.
type AffiliateReportOptions struct {
	Q     string
	Level string
	Page  int
	Limit int
}

// validateAffiliateSettingsInput enforces every BRD range on the PUT body.
func validateAffiliateSettingsInput(in AffiliateAdminSettingsInput) error {
	if in.CookieDays < 1 || in.CookieDays > 180 {
		return fmt.Errorf("%w: cookie_days must be between 1 and 180", ErrValidation)
	}
	if in.BindingMonths < 1 || in.BindingMonths > 12 {
		return fmt.Errorf("%w: binding_months must be between 1 and 12", ErrValidation)
	}
	if in.DigitalWaitDays < 0 || in.DigitalWaitDays > 90 {
		return fmt.Errorf("%w: digital_wait_days must be between 0 and 90", ErrValidation)
	}
	if in.PhysicalWaitDays < 0 || in.PhysicalWaitDays > 90 {
		return fmt.Errorf("%w: physical_wait_days must be between 0 and 90", ErrValidation)
	}
	if in.MinPayout <= 0 {
		return fmt.Errorf("%w: min_payout must be greater than 0", ErrValidation)
	}
	if err := affiliateValidatePayoutDay(in.PayoutDayOfMonth, "payout_day_of_month"); err != nil {
		return err
	}
	if in.EvalDay < 1 || in.EvalDay > 28 {
		return fmt.Errorf("%w: eval_day must be between 1 and 28", ErrValidation)
	}
	if in.LaunchDate != nil {
		if _, err := time.Parse("2006-01-02", *in.LaunchDate); err != nil {
			return fmt.Errorf("%w: launch_date must be YYYY-MM-DD", ErrValidation)
		}
	}
	if in.DailyRegistrationCap != nil && *in.DailyRegistrationCap < 1 {
		return fmt.Errorf("%w: daily_registration_cap must be at least 1", ErrValidation)
	}
	for _, level := range in.Levels {
		if err := affiliateValidateLevelInput(level); err != nil {
			return err
		}
	}
	return nil
}

func affiliateValidatePayoutDay(day *int, field string) error {
	if day != nil && (*day < 1 || *day > 28) {
		return fmt.Errorf("%w: %s must be between 1 and 28", ErrValidation, field)
	}
	return nil
}

func affiliateValidateLevelInput(level AffiliateAdminLevelInput) error {
	if !affiliateValidLevel(level.Level) {
		return fmt.Errorf("%w: unknown level %q", ErrValidation, level.Level)
	}
	for category, rule := range level.CommissionRule {
		if !slices.Contains(affiliateCommissionCategories(), category) {
			return fmt.Errorf("%w: unknown commission category %q", ErrValidation, category)
		}
		if err := validateAffiliateCommissionRule(rule); err != nil {
			return fmt.Errorf("commission_rule %q: %w", category, err)
		}
	}
	if level.BindingMonths != nil && (*level.BindingMonths < 1 || *level.BindingMonths > 12) {
		return fmt.Errorf("%w: level binding_months must be between 1 and 12", ErrValidation)
	}
	if level.DigitalWaitDays != nil && (*level.DigitalWaitDays < 0 || *level.DigitalWaitDays > 90) {
		return fmt.Errorf("%w: level digital_wait_days must be between 0 and 90", ErrValidation)
	}
	if level.PhysicalWaitDays != nil && (*level.PhysicalWaitDays < 0 || *level.PhysicalWaitDays > 90) {
		return fmt.Errorf("%w: level physical_wait_days must be between 0 and 90", ErrValidation)
	}
	if level.MinPayout != nil && *level.MinPayout <= 0 {
		return fmt.Errorf("%w: level min_payout must be greater than 0", ErrValidation)
	}
	if err := affiliateValidatePayoutDay(level.PayoutDayOfMonth, "level payout_day_of_month"); err != nil {
		return err
	}
	if level.RiseThreshold != nil && *level.RiseThreshold < 0 {
		return fmt.Errorf("%w: level rise_threshold must be at least 0", ErrValidation)
	}
	if level.HoldThreshold != nil && *level.HoldThreshold < 0 {
		return fmt.Errorf("%w: level hold_threshold must be at least 0", ErrValidation)
	}
	return nil
}

// validateAffiliateCommissionRule accepts a percent rule of 0-100 with at most
// two decimals, or a fixed rule of whole rupiah.
func validateAffiliateCommissionRule(rule model.AffiliateCommissionRule) error {
	switch rule.Type {
	case affiliateCommissionPercent:
		if rule.Value < 0 || rule.Value > 100 {
			return fmt.Errorf("%w: percent must be between 0 and 100", ErrValidation)
		}
		if math.Abs(rule.Value*100-math.Round(rule.Value*100)) > 1e-6 {
			return fmt.Errorf("%w: percent allows at most 2 decimals", ErrValidation)
		}
	case affiliateCommissionFixed:
		if rule.Value < 0 || rule.Value != math.Floor(rule.Value) {
			return fmt.Errorf("%w: fixed amount must be a whole rupiah of at least 0", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: commission type must be percent or fixed", ErrValidation)
	}
	return nil
}

func affiliateCommissionCategories() []string {
	return []string{"exam", "course", "book", "merchandise", "medal"}
}

// affiliateDateKey normalizes a launch_date value to YYYY-MM-DD so a re-sent
// date compares equal to the DATE column readback.
func affiliateDateKey(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// affiliateSettingsAuditMeta maps each changed field to {old, new}; level rows
// are keyed "level.<name>" with the row itself as the new value. The settings
// history FR-SET-04 asks for is served by this audit_log row — one per PUT,
// carrying every changed field's old and new values.
func affiliateSettingsAuditMeta(old *model.AffiliateSettings, oldLevels []model.AffiliateLevelSettings, in AffiliateAdminSettingsInput) (map[string]any, *model.AffiliateSettings, error) {
	if old == nil {
		old = defaultAffiliateSettings()
	}
	var launch *time.Time
	if in.LaunchDate != nil {
		parsed, _ := time.Parse("2006-01-02", *in.LaunchDate)
		launch = &parsed
	}
	updated := &model.AffiliateSettings{
		ID:                   1,
		CookieDays:           in.CookieDays,
		BindingMonths:        in.BindingMonths,
		MinExtendAmount:      in.MinExtendAmount,
		DigitalWaitDays:      in.DigitalWaitDays,
		PhysicalWaitDays:     in.PhysicalWaitDays,
		MinPayout:            in.MinPayout,
		PayoutDayOfMonth:     in.PayoutDayOfMonth,
		LaunchDate:           launch,
		EvalDay:              in.EvalDay,
		DailyRegistrationCap: in.DailyRegistrationCap,
	}

	meta := map[string]any{}
	add := func(field string, oldValue, newValue any) {
		if !reflect.DeepEqual(oldValue, newValue) {
			meta[field] = map[string]any{"old": oldValue, "new": newValue}
		}
	}
	add("cookie_days", old.CookieDays, updated.CookieDays)
	add("binding_months", old.BindingMonths, updated.BindingMonths)
	add("min_extend_amount", old.MinExtendAmount, updated.MinExtendAmount)
	add("digital_wait_days", old.DigitalWaitDays, updated.DigitalWaitDays)
	add("physical_wait_days", old.PhysicalWaitDays, updated.PhysicalWaitDays)
	add("min_payout", old.MinPayout, updated.MinPayout)
	add("payout_day_of_month", old.PayoutDayOfMonth, updated.PayoutDayOfMonth)
	add("launch_date", affiliateDateKey(old.LaunchDate), affiliateDateKey(updated.LaunchDate))
	add("eval_day", old.EvalDay, updated.EvalDay)
	add("daily_registration_cap", old.DailyRegistrationCap, updated.DailyRegistrationCap)

	existing := make(map[string]model.AffiliateLevelSettings, len(oldLevels))
	for _, row := range oldLevels {
		existing[row.Level] = row
	}
	for _, level := range in.Levels {
		newRow := model.AffiliateLevelSettings{
			Level:            level.Level,
			BindingMonths:    level.BindingMonths,
			DigitalWaitDays:  level.DigitalWaitDays,
			PhysicalWaitDays: level.PhysicalWaitDays,
			MinPayout:        level.MinPayout,
			PayoutDayOfMonth: level.PayoutDayOfMonth,
			RiseThreshold:    level.RiseThreshold,
			HoldThreshold:    level.HoldThreshold,
		}
		if len(level.CommissionRule) > 0 {
			if raw, err := json.Marshal(level.CommissionRule); err == nil {
				newRow.CommissionRule = raw
			}
		}
		if !affiliateLevelRowsEqual(existing[level.Level], newRow) {
			var oldValue any
			if row, ok := existing[level.Level]; ok {
				oldValue = row
			}
			meta["level."+level.Level] = map[string]any{"old": oldValue, "new": newRow}
		}
	}
	return meta, updated, nil
}

// affiliateLevelRowsEqual compares a stored row with the row about to be
// stored, jsonb percentage maps compared by content because Postgres rewrites
// jsonb text.
func affiliateLevelRowsEqual(old, new model.AffiliateLevelSettings) bool {
	if old.Level != new.Level {
		return false
	}
	var oldRule, newRule map[string]model.AffiliateCommissionRule
	_ = json.Unmarshal(old.CommissionRule, &oldRule)
	_ = json.Unmarshal(new.CommissionRule, &newRule)
	if len(oldRule) == 0 && len(newRule) == 0 {
		oldRule, newRule = nil, nil
	}
	return reflect.DeepEqual(oldRule, newRule) &&
		reflect.DeepEqual(old.BindingMonths, new.BindingMonths) &&
		reflect.DeepEqual(old.DigitalWaitDays, new.DigitalWaitDays) &&
		reflect.DeepEqual(old.PhysicalWaitDays, new.PhysicalWaitDays) &&
		reflect.DeepEqual(old.MinPayout, new.MinPayout) &&
		reflect.DeepEqual(old.PayoutDayOfMonth, new.PayoutDayOfMonth) &&
		reflect.DeepEqual(old.RiseThreshold, new.RiseThreshold) &&
		reflect.DeepEqual(old.HoldThreshold, new.HoldThreshold)
}

// GetAffiliateAdminSettings backs GET /admin/affiliate-settings: the global
// singleton (column defaults when never saved) plus the three level rows.
func (s *Service) GetAffiliateAdminSettings(ctx context.Context) (*AffiliateAdminSettingsView, error) {
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := s.storeRepo.LoadAffiliateLevelSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate level settings: %w", err)
	}
	return &AffiliateAdminSettingsView{
		Settings:       settings,
		Levels:         affiliateSettingsLevelsView(levels),
		ProgramEnabled: s.affiliateEnabled(ctx),
	}, nil
}

// SetAffiliateProgramEnabled switches the whole program on or off. It goes
// through UpdateSystemConfig so the audit row and the "switched on at" marker
// the reconcile job relies on are written the same way as from the config API.
func (s *Service) SetAffiliateProgramEnabled(ctx context.Context, actorID string, enabled bool) error {
	_, err := s.UpdateSystemConfig(ctx, actorID, map[string]string{"affiliate_enabled": strconv.FormatBool(enabled)})
	return err
}

func affiliateSettingsLevelsView(rows []model.AffiliateLevelSettings) []model.AffiliateLevelSettings {
	existing := make(map[string]model.AffiliateLevelSettings, len(rows))
	for _, row := range rows {
		existing[row.Level] = row
	}
	out := make([]model.AffiliateLevelSettings, 0, len(affiliateLevels))
	for _, level := range affiliateLevels {
		if row, ok := existing[level]; ok {
			out = append(out, row)
		} else {
			out = append(out, model.AffiliateLevelSettings{Level: level})
		}
	}
	return out
}

// UpdateAffiliateAdminSettings backs PUT /admin/affiliate-settings: one
// transaction upserts the singleton, the submitted level rows and a single
// audit row carrying each changed field's old and new values. The
// affiliate_enabled flag lives in system_config and is deliberately untouched.
func (s *Service) UpdateAffiliateAdminSettings(ctx context.Context, actorID string, in AffiliateAdminSettingsInput) (*AffiliateAdminSettingsView, error) {
	if err := validateAffiliateSettingsInput(in); err != nil {
		return nil, err
	}
	if _, err := parseUUID(actorID); err != nil {
		return nil, ErrInvalidUUID
	}
	current, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, err
	}
	currentLevels, err := s.storeRepo.LoadAffiliateLevelSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate level settings: %w", err)
	}
	meta, updated, err := affiliateSettingsAuditMeta(current, currentLevels, in)
	if err != nil {
		return nil, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.UpsertAffiliateSettingsTx(ctx, tx, updated); err != nil {
		return nil, fmt.Errorf("upsert affiliate settings: %w", err)
	}
	for _, level := range in.Levels {
		row := model.AffiliateLevelSettings{
			Level:            level.Level,
			BindingMonths:    level.BindingMonths,
			DigitalWaitDays:  level.DigitalWaitDays,
			PhysicalWaitDays: level.PhysicalWaitDays,
			MinPayout:        level.MinPayout,
			PayoutDayOfMonth: level.PayoutDayOfMonth,
			RiseThreshold:    level.RiseThreshold,
			HoldThreshold:    level.HoldThreshold,
		}
		if len(level.CommissionRule) > 0 {
			if raw, err := json.Marshal(level.CommissionRule); err == nil {
				row.CommissionRule = raw
			}
		}
		if err := s.storeRepo.UpsertAffiliateLevelSettingsTx(ctx, tx, &row); err != nil {
			return nil, fmt.Errorf("upsert affiliate level settings: %w", err)
		}
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_settings", "1", affiliateAuditSettings, meta); err != nil {
		return nil, fmt.Errorf("write settings audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit affiliate settings: %w", err)
	}

	settings, err := s.storeRepo.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate settings: %w", err)
	}
	if settings == nil {
		settings = defaultAffiliateSettings()
	}
	levels, err := s.storeRepo.LoadAffiliateLevelSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate level settings: %w", err)
	}
	return &AffiliateAdminSettingsView{
		Settings: settings,
		Levels:   affiliateSettingsLevelsView(levels),
	}, nil
}

// AdminListAffiliateOverrides backs GET /admin/affiliate-product-overrides.
func (s *Service) AdminListAffiliateOverrides(ctx context.Context, q string, limit, offset int) ([]repository.AffiliateOverrideRow, int, error) {
	return s.storeRepo.ListAffiliateOverrideRows(ctx, q, limit, offset)
}

// AdminSearchAffiliateOverrideProducts backs GET /admin/affiliate-product-overrides/products:
// the picker that replaces typing a product id.
func (s *Service) AdminSearchAffiliateOverrideProducts(ctx context.Context, q string, limit int) ([]repository.AffiliateOverrideProduct, error) {
	return s.storeRepo.SearchAffiliateOverrideProducts(ctx, q, limit)
}

// AdminUpsertAffiliateOverride backs PUT /admin/affiliate-product-overrides/:product_id.
func (s *Service) AdminUpsertAffiliateOverride(ctx context.Context, actorID, productID string, rule model.AffiliateCommissionRule) (*model.AffiliateProductOverride, error) {
	if err := validateAffiliateCommissionRule(rule); err != nil {
		return nil, err
	}
	id, err := parseUUID(productID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	if _, err := parseUUID(actorID); err != nil {
		return nil, ErrInvalidUUID
	}
	exists, err := s.storeRepo.AffiliateProductExists(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check product: %w", err)
	}
	if !exists {
		return nil, ErrProductNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	override := &model.AffiliateProductOverride{ProductID: id, Type: rule.Type, Value: rule.Value}
	if err := s.storeRepo.UpsertAffiliateOverrideTx(ctx, tx, override); err != nil {
		return nil, fmt.Errorf("upsert affiliate override: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_product_override", id.String(), affiliateAuditOverride, map[string]any{
		"type":  rule.Type,
		"value": rule.Value,
	}); err != nil {
		return nil, fmt.Errorf("write override audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit affiliate override: %w", err)
	}
	return s.storeRepo.LoadProductOverride(ctx, id)
}

// AdminDeleteAffiliateOverride backs DELETE /admin/affiliate-product-overrides/:product_id;
// without a row the product simply falls back to its level/global percentage.
func (s *Service) AdminDeleteAffiliateOverride(ctx context.Context, actorID, productID string) error {
	id, err := parseUUID(productID)
	if err != nil {
		return ErrInvalidUUID
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	deleted, err := s.storeRepo.DeleteAffiliateOverrideTx(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("delete affiliate override: %w", err)
	}
	if !deleted {
		return ErrAffiliateNotFound
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_product_override", id.String(), affiliateAuditOverride, map[string]any{
		"deleted": true,
	}); err != nil {
		return fmt.Errorf("write override audit: %w", err)
	}
	return tx.Commit(ctx)
}

// AdminAffiliateReportSummary backs GET /admin/affiliate-reports/summary?from&to:
// per-affiliator performance rows over the window plus a totals row.
func (s *Service) AdminAffiliateReportSummary(ctx context.Context, from, to string, opts AffiliateReportOptions) (*AffiliateReportSummary, error) {
	if from == "" || to == "" {
		return nil, fmt.Errorf("%w: from and to are required", ErrMissingField)
	}
	if opts.Level != "" && !slices.Contains(affiliateLevels, opts.Level) {
		return nil, fmt.Errorf("%w: %s", ErrValidation, opts.Level)
	}
	start, end, err := parseAffiliateReportWindow(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.storeRepo.ListAffiliateReportRows(ctx, start, end, opts.Q, opts.Level)
	if err != nil {
		return nil, fmt.Errorf("affiliate report rows: %w", err)
	}
	buckets, err := s.storeRepo.ListAffiliateCommissionStatusTotals(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("affiliate report commissions: %w", err)
	}
	for i := range rows {
		row := &rows[i]
		if bucket := buckets[row.AffiliatorID]; bucket != nil {
			row.Commissions = bucket
		}
	}
	totals := repository.AffiliateReportRow{Commissions: map[string]float64{}}
	for _, row := range rows {
		totals.StudentsNew += row.StudentsNew
		totals.NetRevenue += row.NetRevenue
		for status, amount := range row.Commissions {
			totals.Commissions[status] += amount
		}
	}
	summary := &AffiliateReportSummary{Data: rows, Totals: totals, Total: len(rows)}
	if opts.Limit > 0 {
		page := max(opts.Page, 1)
		lo := min((page-1)*opts.Limit, len(rows))
		hi := min(lo+opts.Limit, len(rows))
		summary.Data, summary.Page, summary.PerPage = rows[lo:hi], page, opts.Limit
	}
	return summary, nil
}

// parseAffiliateReportWindow accepts RFC3339 instants or WIB calendar dates;
// a date-only range covers that whole WIB day (end exclusive).
func parseAffiliateReportWindow(from, to string) (time.Time, time.Time, error) {
	start, err := time.Parse(time.RFC3339, from)
	if err == nil {
		end, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: %s", ErrInvalidDateFormat, to)
		}
		return start, end, nil
	}
	startDay, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: %s", ErrInvalidDateFormat, from)
	}
	endDay, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: %s", ErrInvalidDateFormat, to)
	}
	wib := affiliateWIBLocation()
	start = time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, wib)
	end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, wib).AddDate(0, 0, 1)
	return start, end, nil
}

// AdminAffiliateReportLiability backs GET /admin/affiliate-reports/liability:
// the append-only ledger net per commission status plus payouts still owed.
func (s *Service) AdminAffiliateReportLiability(ctx context.Context) (*repository.AffiliateLiabilityReport, error) {
	byStatus, err := s.storeRepo.SumAffiliateCommissionLedgerByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("affiliate commission liability: %w", err)
	}
	payout, err := s.storeRepo.SumAffiliatePayoutLiability(ctx)
	if err != nil {
		return nil, fmt.Errorf("affiliate payout liability: %w", err)
	}
	return &repository.AffiliateLiabilityReport{ByStatus: byStatus, PayoutLiability: payout}, nil
}
