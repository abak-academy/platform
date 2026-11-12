package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
)

// devAffiliateLaunchDate is far enough back that every local account counts as
// registered after launch.
var devAffiliateLaunchDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// SeedDevAffiliateDefaults turns the affiliate program on for local development
// (APP_ENV=dev) and nowhere else: staging and production keep the flag off until
// an admin enables it. Every write is insert-if-absent, so a value an admin set
// by hand (including turning the flag off) is never overwritten. The required
// settings, level commission rules, and launch date are seeded too, because the
// program has no built-in defaults and without them no student is ever bound.
func (s *Service) SeedDevAffiliateDefaults(ctx context.Context, env string) error {
	if env != "dev" {
		return nil
	}
	if err := s.storeRepo.InsertSystemConfigIfAbsent(ctx, "affiliate_enabled", "true"); err != nil {
		return fmt.Errorf("seed affiliate_enabled: %w", err)
	}
	levels, err := json.Marshal(devAffiliateLevels())
	if err != nil {
		return fmt.Errorf("marshal affiliate dev levels: %w", err)
	}
	if err := s.storeRepo.SeedAffiliateDevSettingsIfUnset(ctx, levels); err != nil {
		return fmt.Errorf("seed affiliate dev settings: %w", err)
	}
	if err := s.storeRepo.SetAffiliateLaunchDateIfUnset(ctx, devAffiliateLaunchDate); err != nil {
		return fmt.Errorf("seed affiliate launch date: %w", err)
	}
	return nil
}

// devAffiliateLevels gives every level the commission rules local development
// used to get from the built-in defaults.
func devAffiliateLevels() map[string]map[string]map[string]model.AffiliateCommissionRule {
	rules := map[string]model.AffiliateCommissionRule{
		"exam":        {Type: affiliateCommissionPercent, Value: 10},
		"course":      {Type: affiliateCommissionPercent, Value: 10},
		"book":        {Type: affiliateCommissionPercent, Value: 3},
		"merchandise": {Type: affiliateCommissionPercent, Value: 3},
	}
	levels := make(map[string]map[string]map[string]model.AffiliateCommissionRule, len(affiliateLevels))
	for _, level := range affiliateLevels {
		levels[level] = map[string]map[string]model.AffiliateCommissionRule{"commission_rule": rules}
	}
	return levels
}
