package service

import (
	"context"
	"fmt"
	"time"
)

// devAffiliateLaunchDate is far enough back that every local account counts as
// registered after launch.
var devAffiliateLaunchDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// SeedDevAffiliateDefaults turns the affiliate program on for local development
// (APP_ENV=dev) and nowhere else: staging and production keep the flag off until
// an admin enables it. Every write is insert-if-absent, so a value an admin set
// by hand (including turning the flag off) is never overwritten. The launch date
// is seeded too, because without it no student can ever be bound.
func (s *Service) SeedDevAffiliateDefaults(ctx context.Context, env string) error {
	if env != "dev" {
		return nil
	}
	if err := s.storeRepo.InsertSystemConfigIfAbsent(ctx, "affiliate_enabled", "true"); err != nil {
		return fmt.Errorf("seed affiliate_enabled: %w", err)
	}
	if err := s.storeRepo.SetAffiliateLaunchDateIfUnset(ctx, devAffiliateLaunchDate); err != nil {
		return fmt.Errorf("seed affiliate launch date: %w", err)
	}
	return nil
}
