package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// Feature-local sentinels (see affiliate_admin.go for the precedent); the
// parent owns their mapServiceError cases. ErrAffiliateNotFound lives in
// affiliate_admin_money.go and reports unknown flag/appeal/override ids here.
var (
	// ErrAffiliateFlagNotOpen reports reviewing a flag that is no longer open.
	ErrAffiliateFlagNotOpen = errors.New("affiliate flag is not open for review")
	// ErrAffiliateAppealNotPending reports deciding an appeal that is not pending.
	ErrAffiliateAppealNotPending = errors.New("affiliate appeal is not pending")
)

const (
	affiliateAuditSettings = "affiliate.settings_update"
	affiliateAuditOverride = "affiliate.product_override"
	affiliateAuditFlag     = "affiliate.flag_review"
	affiliateAuditAppeal   = "affiliate.appeal_decide"
)

// AffiliateAdminLevelInput is one level row in the PUT settings body; nil
// optional fields clear to NULL because the PUT is a full replace.
type AffiliateAdminLevelInput struct {
	Level                string             `json:"level"`
	CommissionPct        map[string]float64 `json:"commission_pct"`
	DigitalWaitDays      *int               `json:"digital_wait_days"`
	PhysicalReceivedDays *int               `json:"physical_received_days"`
	PhysicalWaitDays     *int               `json:"physical_wait_days"`
	MinPayout            *float64           `json:"min_payout"`
	PayoutDayOfMonth     *int               `json:"payout_day_of_month"`
	RiseThreshold        *float64           `json:"rise_threshold"`
	HoldThreshold        *float64           `json:"hold_threshold"`
}

// AffiliateAdminSettingsInput is the PUT /admin/affiliate-settings body: the
// global singleton plus optional per-level rows.
type AffiliateAdminSettingsInput struct {
	CookieDays                 int                        `json:"cookie_days"`
	BindingMonths              int                        `json:"binding_months"`
	MinExtendAmount            float64                    `json:"min_extend_amount"`
	DigitalWaitDays            int                        `json:"digital_wait_days"`
	PhysicalReceivedDays       int                        `json:"physical_received_days"`
	PhysicalWaitDays           int                        `json:"physical_wait_days"`
	MinPayout                  float64                    `json:"min_payout"`
	PayoutDayOfMonth           *int                       `json:"payout_day_of_month"`
	AdjustmentThreshold        float64                    `json:"adjustment_threshold"`
	AdjustmentThresholdEnabled bool                       `json:"adjustment_threshold_enabled"`
	FreezeMaxDays              int                        `json:"freeze_max_days"`
	SoonEndDays                int                        `json:"soon_end_days"`
	LaunchDate                 *string                    `json:"launch_date"`
	EvalDay                    int                        `json:"eval_day"`
	SilverThreshold            float64                    `json:"silver_threshold"`
	GoldThreshold              float64                    `json:"gold_threshold"`
	HoldThresholds             json.RawMessage            `json:"hold_thresholds"`
	DailyRegistrationCap       *int                       `json:"daily_registration_cap"`
	Levels                     []AffiliateAdminLevelInput `json:"levels"`
}

// AffiliateAdminSettingsView is the GET/PUT affiliate-settings response: the
// global singleton plus all three level rows (missing rows synthesized empty).
type AffiliateAdminSettingsView struct {
	Settings *model.AffiliateSettings       `json:"settings"`
	Levels   []model.AffiliateLevelSettings `json:"levels"`
}

// AffiliateFlagReviewInput is the POST /admin/affiliate-flags/:id/review body.
type AffiliateAdminFlagReviewInput struct {
	Status string  `json:"status"`
	Note   *string `json:"note"`
}

// AffiliateAdminAppealDecisionInput is the POST /admin/affiliate-appeals/:id/decide body.
type AffiliateAdminAppealDecisionInput struct {
	Outcome string  `json:"outcome"`
	Note    *string `json:"note"`
}

// AffiliateReportSummary is the GET /admin/affiliate-reports/summary response.
type AffiliateReportSummary struct {
	Data   []repository.AffiliateReportRow `json:"data"`
	Totals repository.AffiliateReportRow   `json:"totals"`
}

var affiliateFlagStatuses = []string{"open", "reviewed", "valid", "ignored"}

var affiliateAppealStatuses = []string{"pending", "upheld", "rejected"}

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
	if in.PhysicalReceivedDays < 0 || in.PhysicalReceivedDays > 30 {
		return fmt.Errorf("%w: physical_received_days must be between 0 and 30", ErrValidation)
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
	if in.AdjustmentThreshold <= 0 {
		return fmt.Errorf("%w: adjustment_threshold must be greater than 0", ErrValidation)
	}
	if in.FreezeMaxDays < 1 || in.FreezeMaxDays > 90 {
		return fmt.Errorf("%w: freeze_max_days must be between 1 and 90", ErrValidation)
	}
	if in.SoonEndDays < 1 || in.SoonEndDays > 180 {
		return fmt.Errorf("%w: soon_end_days must be between 1 and 180", ErrValidation)
	}
	if in.EvalDay < 1 || in.EvalDay > 28 {
		return fmt.Errorf("%w: eval_day must be between 1 and 28", ErrValidation)
	}
	if in.SilverThreshold <= 0 {
		return fmt.Errorf("%w: silver_threshold must be greater than 0", ErrValidation)
	}
	if in.GoldThreshold <= 0 {
		return fmt.Errorf("%w: gold_threshold must be greater than 0", ErrValidation)
	}
	if in.LaunchDate != nil {
		if _, err := time.Parse("2006-01-02", *in.LaunchDate); err != nil {
			return fmt.Errorf("%w: launch_date must be YYYY-MM-DD", ErrValidation)
		}
	}
	if _, err := affiliateNormalizeHoldThresholds(in.HoldThresholds); err != nil {
		return err
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
	for category, pct := range level.CommissionPct {
		if !slices.Contains(affiliateCommissionCategories(), category) {
			return fmt.Errorf("%w: unknown commission category %q", ErrValidation, category)
		}
		if pct < 0 || pct > 100 {
			return fmt.Errorf("%w: commission_pct %q must be between 0 and 100", ErrValidation, category)
		}
	}
	if level.DigitalWaitDays != nil && (*level.DigitalWaitDays < 0 || *level.DigitalWaitDays > 90) {
		return fmt.Errorf("%w: level digital_wait_days must be between 0 and 90", ErrValidation)
	}
	if level.PhysicalReceivedDays != nil && (*level.PhysicalReceivedDays < 0 || *level.PhysicalReceivedDays > 30) {
		return fmt.Errorf("%w: level physical_received_days must be between 0 and 30", ErrValidation)
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

func affiliateCommissionCategories() []string {
	return []string{"exam", "course", "book", "merchandise", "medal"}
}

// affiliateNormalizeHoldThresholds validates the hold_thresholds jsonb (an
// object whose keys are affiliate levels and whose values are null or a
// number) and returns the canonical bytes to store; nil becomes {}.
func affiliateNormalizeHoldThresholds(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("{}"), nil
	}
	var holds map[string]*float64
	if err := json.Unmarshal(raw, &holds); err != nil {
		return nil, fmt.Errorf("%w: hold_thresholds must be an object of levels to null or a number", ErrValidation)
	}
	for key := range holds {
		if !affiliateValidLevel(key) {
			return nil, fmt.Errorf("%w: unknown hold_thresholds key %q", ErrValidation, key)
		}
	}
	out, err := json.Marshal(holds)
	if err != nil {
		return nil, fmt.Errorf("marshal hold_thresholds: %w", err)
	}
	return out, nil
}

// affiliateHoldThresholdsEqual compares two hold_thresholds jsonb blobs by
// their parsed content, not their bytes.
func affiliateHoldThresholdsEqual(a, b json.RawMessage) bool {
	var left, right map[string]*float64
	_ = json.Unmarshal(a, &left)
	_ = json.Unmarshal(b, &right)
	return reflect.DeepEqual(left, right)
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
	holds, err := affiliateNormalizeHoldThresholds(in.HoldThresholds)
	if err != nil {
		return nil, nil, err
	}
	var launch *time.Time
	if in.LaunchDate != nil {
		parsed, _ := time.Parse("2006-01-02", *in.LaunchDate)
		launch = &parsed
	}
	updated := &model.AffiliateSettings{
		ID:                         1,
		CookieDays:                 in.CookieDays,
		BindingMonths:              in.BindingMonths,
		MinExtendAmount:            in.MinExtendAmount,
		DigitalWaitDays:            in.DigitalWaitDays,
		PhysicalReceivedDays:       in.PhysicalReceivedDays,
		PhysicalWaitDays:           in.PhysicalWaitDays,
		MinPayout:                  in.MinPayout,
		PayoutDayOfMonth:           in.PayoutDayOfMonth,
		AdjustmentThreshold:        in.AdjustmentThreshold,
		AdjustmentThresholdEnabled: in.AdjustmentThresholdEnabled,
		FreezeMaxDays:              in.FreezeMaxDays,
		SoonEndDays:                in.SoonEndDays,
		LaunchDate:                 launch,
		EvalDay:                    in.EvalDay,
		SilverThreshold:            in.SilverThreshold,
		GoldThreshold:              in.GoldThreshold,
		HoldThresholds:             holds,
		DailyRegistrationCap:       in.DailyRegistrationCap,
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
	add("physical_received_days", old.PhysicalReceivedDays, updated.PhysicalReceivedDays)
	add("physical_wait_days", old.PhysicalWaitDays, updated.PhysicalWaitDays)
	add("min_payout", old.MinPayout, updated.MinPayout)
	add("payout_day_of_month", old.PayoutDayOfMonth, updated.PayoutDayOfMonth)
	add("adjustment_threshold", old.AdjustmentThreshold, updated.AdjustmentThreshold)
	add("adjustment_threshold_enabled", old.AdjustmentThresholdEnabled, updated.AdjustmentThresholdEnabled)
	add("freeze_max_days", old.FreezeMaxDays, updated.FreezeMaxDays)
	add("soon_end_days", old.SoonEndDays, updated.SoonEndDays)
	add("launch_date", affiliateDateKey(old.LaunchDate), affiliateDateKey(updated.LaunchDate))
	add("eval_day", old.EvalDay, updated.EvalDay)
	add("silver_threshold", old.SilverThreshold, updated.SilverThreshold)
	add("gold_threshold", old.GoldThreshold, updated.GoldThreshold)
	if !affiliateHoldThresholdsEqual(old.HoldThresholds, updated.HoldThresholds) {
		var oldValue, newValue map[string]*float64
		_ = json.Unmarshal(old.HoldThresholds, &oldValue)
		_ = json.Unmarshal(updated.HoldThresholds, &newValue)
		meta["hold_thresholds"] = map[string]any{"old": oldValue, "new": newValue}
	}
	add("daily_registration_cap", old.DailyRegistrationCap, updated.DailyRegistrationCap)

	existing := make(map[string]model.AffiliateLevelSettings, len(oldLevels))
	for _, row := range oldLevels {
		existing[row.Level] = row
	}
	for _, level := range in.Levels {
		newRow := model.AffiliateLevelSettings{
			Level:                level.Level,
			DigitalWaitDays:      level.DigitalWaitDays,
			PhysicalReceivedDays: level.PhysicalReceivedDays,
			PhysicalWaitDays:     level.PhysicalWaitDays,
			MinPayout:            level.MinPayout,
			PayoutDayOfMonth:     level.PayoutDayOfMonth,
			RiseThreshold:        level.RiseThreshold,
			HoldThreshold:        level.HoldThreshold,
		}
		if len(level.CommissionPct) > 0 {
			if raw, err := json.Marshal(level.CommissionPct); err == nil {
				newRow.CommissionPct = raw
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
	var oldPct, newPct map[string]float64
	_ = json.Unmarshal(old.CommissionPct, &oldPct)
	_ = json.Unmarshal(new.CommissionPct, &newPct)
	if len(oldPct) == 0 && len(newPct) == 0 {
		oldPct, newPct = nil, nil
	}
	return reflect.DeepEqual(oldPct, newPct) &&
		reflect.DeepEqual(old.DigitalWaitDays, new.DigitalWaitDays) &&
		reflect.DeepEqual(old.PhysicalReceivedDays, new.PhysicalReceivedDays) &&
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
		Settings: settings,
		Levels:   affiliateSettingsLevelsView(levels),
	}, nil
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
	actor, err := parseUUID(actorID)
	if err != nil {
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
	updated.UpdatedBy = &actor

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
			Level:                level.Level,
			DigitalWaitDays:      level.DigitalWaitDays,
			PhysicalReceivedDays: level.PhysicalReceivedDays,
			PhysicalWaitDays:     level.PhysicalWaitDays,
			MinPayout:            level.MinPayout,
			PayoutDayOfMonth:     level.PayoutDayOfMonth,
			RiseThreshold:        level.RiseThreshold,
			HoldThreshold:        level.HoldThreshold,
		}
		if len(level.CommissionPct) > 0 {
			if raw, err := json.Marshal(level.CommissionPct); err == nil {
				row.CommissionPct = raw
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
func (s *Service) AdminListAffiliateOverrides(ctx context.Context) ([]repository.AffiliateOverrideRow, error) {
	return s.storeRepo.ListAffiliateOverrideRows(ctx)
}

// AdminUpsertAffiliateOverride backs PUT /admin/affiliate-product-overrides/:product_id.
func (s *Service) AdminUpsertAffiliateOverride(ctx context.Context, actorID, productID string, percentage float64) (*model.AffiliateProductOverride, error) {
	if percentage < 0 || percentage > 100 {
		return nil, fmt.Errorf("%w: percentage must be between 0 and 100", ErrValidation)
	}
	id, err := parseUUID(productID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
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
	override := &model.AffiliateProductOverride{ProductID: id, Percentage: percentage, UpdatedBy: &actor}
	if err := s.storeRepo.UpsertAffiliateOverrideTx(ctx, tx, override); err != nil {
		return nil, fmt.Errorf("upsert affiliate override: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_product_override", id.String(), affiliateAuditOverride, map[string]any{
		"percentage": percentage,
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

// AdminListAffiliateFlags backs GET /admin/affiliate-flags?status= — default
// open; "all" disables the filter.
func (s *Service) AdminListAffiliateFlags(ctx context.Context, status string) ([]repository.AffiliateFlagRow, error) {
	if status == "" {
		status = "open"
	}
	if status != "all" && !slices.Contains(affiliateFlagStatuses, status) {
		return nil, fmt.Errorf("%w: %s", ErrValidation, status)
	}
	return s.storeRepo.ListAffiliateFlagRows(ctx, status)
}

// AdminReviewAffiliateFlag backs POST /admin/affiliate-flags/:id/review. Only
// open flags are reviewable — a second review needs a new detection row.
func (s *Service) AdminReviewAffiliateFlag(ctx context.Context, actorID, flagID string, in AffiliateAdminFlagReviewInput) (*model.AffiliateFlag, error) {
	if !slices.Contains(affiliateFlagStatuses[1:], in.Status) {
		return nil, fmt.Errorf("%w: flag review status must be reviewed, valid or ignored", ErrValidation)
	}
	id, err := parseUUID(flagID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	flag, err := s.storeRepo.FindAffiliateFlagByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find affiliate flag: %w", err)
	}
	if flag == nil {
		return nil, ErrAffiliateNotFound
	}
	if flag.Status != "open" {
		return nil, ErrAffiliateFlagNotOpen
	}

	meta := map[string]any{"status": in.Status}
	if in.Note != nil {
		meta["note"] = *in.Note
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.UpdateAffiliateFlagReviewTx(ctx, tx, id, in.Status, in.Note, actor); err != nil {
		return nil, fmt.Errorf("review affiliate flag: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_flag", id.String(), affiliateAuditFlag, meta); err != nil {
		return nil, fmt.Errorf("write flag review audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit flag review: %w", err)
	}
	return s.storeRepo.FindAffiliateFlagByID(ctx, id)
}

// AdminListAffiliateAppeals backs GET /admin/affiliate-appeals?status= — the
// violations awaiting an appeal decision (default pending).
func (s *Service) AdminListAffiliateAppeals(ctx context.Context, status string) ([]repository.AffiliateAppealRow, error) {
	if status == "" {
		status = "pending"
	}
	if status != "all" && !slices.Contains(affiliateAppealStatuses, status) {
		return nil, fmt.Errorf("%w: %s", ErrValidation, status)
	}
	return s.storeRepo.ListAffiliateAppealRows(ctx, status)
}

// AdminDecideAffiliateAppeal backs POST /admin/affiliate-appeals/:id/decide.
// The decision resolves the violation row; applying an upheld sanction stays a
// deliberate manual step through the affiliate status/level endpoints.
func (s *Service) AdminDecideAffiliateAppeal(ctx context.Context, actorID, violationID string, in AffiliateAdminAppealDecisionInput) (*model.AffiliateViolation, error) {
	if in.Outcome != "upheld" && in.Outcome != "rejected" {
		return nil, fmt.Errorf("%w: appeal outcome must be upheld or rejected", ErrValidation)
	}
	id, err := parseUUID(violationID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	violation, err := s.storeRepo.FindAffiliateViolationByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find affiliate violation: %w", err)
	}
	if violation == nil {
		return nil, ErrAffiliateNotFound
	}
	if violation.AppealStatus != "pending" {
		return nil, ErrAffiliateAppealNotPending
	}

	meta := map[string]any{"outcome": in.Outcome}
	if in.Note != nil {
		meta["note"] = *in.Note
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.DecideAffiliateAppealTx(ctx, tx, id, in.Outcome, actor); err != nil {
		return nil, fmt.Errorf("decide affiliate appeal: %w", err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "affiliate_violation", id.String(), affiliateAuditAppeal, meta); err != nil {
		return nil, fmt.Errorf("write appeal audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit appeal decision: %w", err)
	}
	return s.storeRepo.FindAffiliateViolationByID(ctx, id)
}

// AdminAffiliateReportSummary backs GET /admin/affiliate-reports/summary?from&to:
// per-affiliator performance rows over the window plus a totals row.
func (s *Service) AdminAffiliateReportSummary(ctx context.Context, from, to string) (*AffiliateReportSummary, error) {
	if from == "" || to == "" {
		return nil, fmt.Errorf("%w: from and to are required", ErrMissingField)
	}
	start, end, err := parseAffiliateReportWindow(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.storeRepo.ListAffiliateReportRows(ctx, start, end)
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
		if row.Clicks > 0 {
			row.Conversion = float64(row.StudentsNew) / float64(row.Clicks)
		}
	}
	totals := repository.AffiliateReportRow{Commissions: map[string]float64{}}
	for _, row := range rows {
		totals.StudentsNew += row.StudentsNew
		totals.NetRevenue += row.NetRevenue
		totals.Clicks += row.Clicks
		for status, amount := range row.Commissions {
			totals.Commissions[status] += amount
		}
	}
	return &AffiliateReportSummary{Data: rows, Totals: totals}, nil
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
