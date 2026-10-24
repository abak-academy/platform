package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
)

// affiliateDefaultCommissionPct holds the TRD example's per-category commission percentages; affiliate_settings has no per-category columns, so these live only here.
var affiliateDefaultCommissionPct = map[string]float64{
	"exam":        10,
	"course":      10,
	"book":        3,
	"merchandise": 3,
	"medal":       0,
}

var affiliateLevels = []string{"bronze", "silver", "gold"}

// AffiliateEffectiveRule is one level's resolved rule set: global settings as
// the base, affiliate_level_settings overriding where the level row is set.
type AffiliateEffectiveRule struct {
	Level                string             `json:"level"`
	CommissionPct        map[string]float64 `json:"commission_pct"`
	DigitalWaitDays      int                `json:"digital_wait_days"`
	PhysicalReceivedDays int                `json:"physical_received_days"`
	PhysicalWaitDays     int                `json:"physical_wait_days"`
	MinPayout            float64            `json:"min_payout"`
	PayoutDayOfMonth     *int               `json:"payout_day_of_month"`
	RiseThreshold        *float64           `json:"rise_threshold"`
	HoldThreshold        *float64           `json:"hold_threshold"`
}

// AffiliateCodeValidation is the public validate endpoint response body.
type AffiliateCodeValidation struct {
	Valid          bool   `json:"valid"`
	AffiliatorName string `json:"affiliator_name"`
}

// defaultAffiliateSettings mirrors the 0070 column defaults so a missing
// singleton row behaves as an unconfigured-but-valid install.
func defaultAffiliateSettings() *model.AffiliateSettings {
	return &model.AffiliateSettings{
		ID:                         1,
		CookieDays:                 30,
		BindingMonths:              3,
		MinExtendAmount:            0,
		DigitalWaitDays:            7,
		PhysicalReceivedDays:       7,
		PhysicalWaitDays:           7,
		MinPayout:                  100000,
		PayoutDayOfMonth:           nil,
		AdjustmentThreshold:        1000000,
		AdjustmentThresholdEnabled: false,
		FreezeMaxDays:              14,
		SoonEndDays:                30,
		LaunchDate:                 nil,
		EvalDay:                    8,
		SilverThreshold:            5000000,
		GoldThreshold:              15000000,
		HoldThresholds:             json.RawMessage(`{}`),
		DailyRegistrationCap:       nil,
		UpdatedBy:                  nil,
	}
}

// affiliateLevelCommissionPct overlays a level row's commission_pct jsonb onto
// the package defaults; an unset or unreadable jsonb keeps the defaults.
func affiliateLevelCommissionPct(raw json.RawMessage) map[string]float64 {
	pct := make(map[string]float64, len(affiliateDefaultCommissionPct))
	for category, value := range affiliateDefaultCommissionPct {
		pct[category] = value
	}
	if len(raw) == 0 {
		return pct
	}
	var levelPct map[string]float64
	if err := json.Unmarshal(raw, &levelPct); err != nil {
		return pct
	}
	for category, value := range levelPct {
		if _, ok := affiliateDefaultCommissionPct[category]; ok {
			pct[category] = value
		}
	}
	return pct
}

func buildAffiliateEffectiveRules(settings *model.AffiliateSettings, levels []model.AffiliateLevelSettings) map[string]AffiliateEffectiveRule {
	if settings == nil {
		settings = defaultAffiliateSettings()
	}
	rows := make(map[string]model.AffiliateLevelSettings, len(levels))
	for _, l := range levels {
		rows[l.Level] = l
	}

	rules := make(map[string]AffiliateEffectiveRule, len(affiliateLevels))
	for _, level := range affiliateLevels {
		rule := AffiliateEffectiveRule{
			Level:                level,
			CommissionPct:        affiliateLevelCommissionPct(rows[level].CommissionPct),
			DigitalWaitDays:      settings.DigitalWaitDays,
			PhysicalReceivedDays: settings.PhysicalReceivedDays,
			PhysicalWaitDays:     settings.PhysicalWaitDays,
			MinPayout:            settings.MinPayout,
			PayoutDayOfMonth:     settings.PayoutDayOfMonth,
		}
		if row, ok := rows[level]; ok {
			if row.DigitalWaitDays != nil {
				rule.DigitalWaitDays = *row.DigitalWaitDays
			}
			if row.PhysicalReceivedDays != nil {
				rule.PhysicalReceivedDays = *row.PhysicalReceivedDays
			}
			if row.PhysicalWaitDays != nil {
				rule.PhysicalWaitDays = *row.PhysicalWaitDays
			}
			if row.MinPayout != nil {
				rule.MinPayout = *row.MinPayout
			}
			if row.PayoutDayOfMonth != nil {
				rule.PayoutDayOfMonth = row.PayoutDayOfMonth
			}
			rule.RiseThreshold = row.RiseThreshold
			rule.HoldThreshold = row.HoldThreshold
		}
		rules[level] = rule
	}
	return rules
}

// resolveAffiliateCommissionPct picks the effective percentage for one product
// type at one level: product override > level value > package default.
func resolveAffiliateCommissionPct(override *model.AffiliateProductOverride, rule AffiliateEffectiveRule, productType string) float64 {
	if override != nil {
		return override.Percentage
	}
	if value, ok := rule.CommissionPct[productType]; ok {
		return value
	}
	return 0
}

// affiliateEnabled gates the whole affiliate feature; the flag is default-off:
// only the literal "true" in system_config enables it.
func affiliateEnabled(cfg map[string]string) bool {
	return cfg["affiliate_enabled"] == "true"
}

func (s *Service) affiliateEnabled(ctx context.Context) bool {
	rows, _ := s.storeRepo.ListSystemConfig(ctx)
	cfg := make(map[string]string, len(rows))
	for _, row := range rows {
		cfg[row.Key] = row.Value
	}
	return affiliateEnabled(cfg)
}

// LoadAffiliateSettings returns the global settings, applying the 0070 column
// defaults when the singleton row has not been created yet.
func (s *Service) LoadAffiliateSettings(ctx context.Context) (*model.AffiliateSettings, error) {
	settings, err := s.storeRepo.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load affiliate settings: %w", err)
	}
	if settings == nil {
		return defaultAffiliateSettings(), nil
	}
	return settings, nil
}

// LoadAffiliateEffectiveRules returns the global settings plus each level's
// resolved rule set.
func (s *Service) LoadAffiliateEffectiveRules(ctx context.Context) (*model.AffiliateSettings, map[string]AffiliateEffectiveRule, error) {
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return nil, nil, err
	}
	levels, err := s.storeRepo.LoadAffiliateLevelSettings(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load affiliate level settings: %w", err)
	}
	return settings, buildAffiliateEffectiveRules(settings, levels), nil
}

// ValidateAffiliateCode backs the public validate endpoint: flag on and an
// active affiliator name the owner; everything else is valid:false with no
// error so the endpoint cannot enumerate accounts.
func (s *Service) ValidateAffiliateCode(ctx context.Context, code string) (AffiliateCodeValidation, error) {
	res := AffiliateCodeValidation{}
	code = strings.TrimSpace(code)
	if code == "" || !s.affiliateEnabled(ctx) {
		return res, nil
	}
	aff, err := s.storeRepo.FindAffiliatorByCode(ctx, code)
	if err != nil {
		return res, fmt.Errorf("find affiliator: %w", err)
	}
	if aff == nil || aff.Status != "active" {
		return res, nil
	}
	user, err := s.repo.GetUserByID(ctx, aff.UserID.String())
	if err != nil {
		return res, fmt.Errorf("get affiliator user: %w", err)
	}
	if user != nil {
		res.AffiliatorName = user.Name
	}
	res.Valid = true
	return res, nil
}

// RecordAffiliateClick stores one click-statistics row, deduped per code +
// visitor IP hash + WIB calendar day; flag off or a non-active code is a
// silent no-op.
func (s *Service) RecordAffiliateClick(ctx context.Context, code, ip, userAgent string) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" || !s.affiliateEnabled(ctx) {
		return false, nil
	}
	aff, err := s.storeRepo.FindAffiliatorByCode(ctx, code)
	if err != nil {
		return false, fmt.Errorf("find affiliator: %w", err)
	}
	if aff == nil || aff.Status != "active" {
		return false, nil
	}
	click := &model.AffiliateClick{AffiliatorID: aff.ID, Code: code}
	if ip != "" {
		sum := sha256.Sum256([]byte(ip))
		hash := hex.EncodeToString(sum[:])[:16]
		click.IPHash = &hash
	}
	if userAgent != "" {
		click.UserAgent = &userAgent
	}
	click.ClickDay = time.Now().In(affiliateWIBLocation())
	return s.storeRepo.InsertAffiliateClick(ctx, click)
}

func affiliateWIBLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}
