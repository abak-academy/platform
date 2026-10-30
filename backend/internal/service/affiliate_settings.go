package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
)

const (
	affiliateCommissionPercent = "percent"
	affiliateCommissionFixed   = "fixed"
)

// affiliateDefaultCommissionRule holds the TRD example's per-category commission rules; affiliate_settings has no per-category columns, so these live only here.
var affiliateDefaultCommissionRule = map[string]model.AffiliateCommissionRule{
	"exam":        {Type: affiliateCommissionPercent, Value: 10},
	"course":      {Type: affiliateCommissionPercent, Value: 10},
	"book":        {Type: affiliateCommissionPercent, Value: 3},
	"merchandise": {Type: affiliateCommissionPercent, Value: 3},
	"medal":       {Type: affiliateCommissionPercent, Value: 0},
}

// affiliateDefaultRiseThreshold is the quarterly revenue that rises into a
// level while the admin leaves that level's rise_threshold empty.
var affiliateDefaultRiseThreshold = map[string]float64{
	"silver": 5000000,
	"gold":   15000000,
}

var affiliateLevels = []string{"bronze", "silver", "gold"}

// AffiliateEffectiveRule is one level's resolved rule set: global settings as
// the base, affiliate_level_settings overriding where the level row is set.
type AffiliateEffectiveRule struct {
	Level            string                                   `json:"level"`
	CommissionRule   map[string]model.AffiliateCommissionRule `json:"commission_rule"`
	BindingMonths    int                                      `json:"binding_months"`
	DigitalWaitDays  int                                      `json:"digital_wait_days"`
	PhysicalWaitDays int                                      `json:"physical_wait_days"`
	MinPayout        float64                                  `json:"min_payout"`
	PayoutDayOfMonth *int                                     `json:"payout_day_of_month"`
	RiseThreshold    *float64                                 `json:"rise_threshold"`
	HoldThreshold    *float64                                 `json:"hold_threshold"`
}

// AffiliateCodeValidation is the public validate endpoint response body.
type AffiliateCodeValidation struct {
	Valid          bool   `json:"valid"`
	AffiliatorName string `json:"affiliator_name"`
	// CookieDays is the admin's referral-cookie lifetime, sent whenever the
	// feature is on so /r/{code} can size the cookie even for an unknown code.
	CookieDays int `json:"cookie_days,omitempty"`
}

// defaultAffiliateSettings mirrors the 0070 column defaults so a missing
// singleton row behaves as an unconfigured-but-valid install.
func defaultAffiliateSettings() *model.AffiliateSettings {
	return &model.AffiliateSettings{
		ID:                   1,
		CookieDays:           30,
		BindingMonths:        3,
		MinExtendAmount:      0,
		DigitalWaitDays:      7,
		PhysicalWaitDays:     14,
		MinPayout:            100000,
		PayoutDayOfMonth:     nil,
		LaunchDate:           nil,
		EvalDay:              8,
		DailyRegistrationCap: nil,
	}
}

// affiliateLevelCommissionRule overlays a level row's commission_rule jsonb onto
// the package defaults; an unset or unreadable jsonb keeps the defaults.
func affiliateLevelCommissionRule(raw json.RawMessage) map[string]model.AffiliateCommissionRule {
	rules := make(map[string]model.AffiliateCommissionRule, len(affiliateDefaultCommissionRule))
	for category, rule := range affiliateDefaultCommissionRule {
		rules[category] = rule
	}
	if len(raw) == 0 {
		return rules
	}
	var levelRules map[string]model.AffiliateCommissionRule
	if err := json.Unmarshal(raw, &levelRules); err != nil {
		return rules
	}
	for category, rule := range levelRules {
		if _, ok := affiliateDefaultCommissionRule[category]; ok {
			rules[category] = rule
		}
	}
	return rules
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
			Level:            level,
			CommissionRule:   affiliateLevelCommissionRule(rows[level].CommissionRule),
			BindingMonths:    settings.BindingMonths,
			DigitalWaitDays:  settings.DigitalWaitDays,
			PhysicalWaitDays: settings.PhysicalWaitDays,
			MinPayout:        settings.MinPayout,
			PayoutDayOfMonth: settings.PayoutDayOfMonth,
		}
		if row, ok := rows[level]; ok {
			if row.DigitalWaitDays != nil {
				rule.DigitalWaitDays = *row.DigitalWaitDays
			}
			if row.BindingMonths != nil {
				rule.BindingMonths = *row.BindingMonths
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
		if rule.RiseThreshold == nil {
			if def, ok := affiliateDefaultRiseThreshold[level]; ok {
				rule.RiseThreshold = &def
			}
		}
		rules[level] = rule
	}
	return rules
}

// resolveAffiliateCommissionRule picks the effective commission rule for one
// product type at one level: product override > level value > package default.
// The winning rule is used whole, so an override's type replaces the level's.
func resolveAffiliateCommissionRule(override *model.AffiliateProductOverride, rule AffiliateEffectiveRule, productType string) model.AffiliateCommissionRule {
	if override != nil {
		return model.AffiliateCommissionRule{Type: override.Type, Value: override.Value}
	}
	if r, ok := rule.CommissionRule[productType]; ok {
		return r
	}
	return model.AffiliateCommissionRule{Type: affiliateCommissionPercent}
}

// affiliateEnabled gates the whole affiliate feature; the flag is default-off:
// only the literal "true" in system_config enables it.
func affiliateEnabled(cfg map[string]string) bool {
	return cfg["affiliate_enabled"] == "true"
}

// affiliateEnabledErr reads the flag and surfaces a read failure. Money paths
// (settlement binding, commission creation, refund settlement) must use it: a
// transient database error read as "off" would silently skip a binding or lose
// a commission instead of failing and being retried.
func (s *Service) affiliateEnabledErr(ctx context.Context) (bool, error) {
	value, found, err := s.storeRepo.GetSystemConfigValue(ctx, "affiliate_enabled")
	if err != nil {
		return false, fmt.Errorf("read affiliate_enabled: %w", err)
	}
	return found && value == "true", nil
}

// affiliateEnabled is the read-path form: a failed read counts as off, which
// only hides a page or ignores a referral hint.
func (s *Service) affiliateEnabled(ctx context.Context) bool {
	on, err := s.affiliateEnabledErr(ctx)
	return err == nil && on
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
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || !s.affiliateEnabled(ctx) {
		return res, nil
	}
	settings, err := s.LoadAffiliateSettings(ctx)
	if err != nil {
		return res, fmt.Errorf("load affiliate settings: %w", err)
	}
	res.CookieDays = settings.CookieDays
	aff, err := s.storeRepo.FindAffiliatorByCode(ctx, code)
	if err != nil {
		return res, fmt.Errorf("find affiliator: %w", err)
	}
	if aff == nil || aff.Status != "active" {
		return res, nil
	}
	user, err := s.repo.GetUserByID(ctx, aff.ID.String())
	if err != nil {
		return res, fmt.Errorf("get affiliator user: %w", err)
	}
	if user != nil {
		res.AffiliatorName = user.Name
	}
	res.Valid = true
	return res, nil
}

func affiliateWIBLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}
