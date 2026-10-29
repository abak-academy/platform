package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func intPtr(i int) *int         { return &i }
func f64Ptr(f float64) *float64 { return &f }

func TestAffiliateSettingsDefaults_matchMigrationDefaults(t *testing.T) {
	s := defaultAffiliateSettings()
	if s == nil {
		t.Fatal("defaultAffiliateSettings() = nil, want settings")
	}
	if s.ID != 1 {
		t.Errorf("ID = %d, want 1", s.ID)
	}
	if s.CookieDays != 30 {
		t.Errorf("CookieDays = %d, want 30", s.CookieDays)
	}
	if s.BindingMonths != 3 {
		t.Errorf("BindingMonths = %d, want 3", s.BindingMonths)
	}
	if s.MinExtendAmount != 0 {
		t.Errorf("MinExtendAmount = %v, want 0", s.MinExtendAmount)
	}
	if s.DigitalWaitDays != 7 {
		t.Errorf("DigitalWaitDays = %d, want 7", s.DigitalWaitDays)
	}
	if s.PhysicalWaitDays != 14 {
		t.Errorf("PhysicalWaitDays = %d, want 14", s.PhysicalWaitDays)
	}
	if s.MinPayout != 100000 {
		t.Errorf("MinPayout = %v, want 100000", s.MinPayout)
	}
	if s.PayoutDayOfMonth != nil {
		t.Errorf("PayoutDayOfMonth = %v, want nil", *s.PayoutDayOfMonth)
	}
	if s.LaunchDate != nil {
		t.Errorf("LaunchDate = %v, want nil", s.LaunchDate)
	}
	if s.EvalDay != 8 {
		t.Errorf("EvalDay = %d, want 8", s.EvalDay)
	}
	if s.DailyRegistrationCap != nil {
		t.Errorf("DailyRegistrationCap = %v, want nil", *s.DailyRegistrationCap)
	}
}

// TestAffiliateEffectiveRules_levelOverridesGlobal pins the resolver
// precedence: a per-level row overrides the global setting only for the
// fields it sets; absent levels and absent fields fall back to global
// (defaults when the singleton row is missing).
func TestAffiliateEffectiveRules_levelOverridesGlobal(t *testing.T) {
	settings := defaultAffiliateSettings()
	levels := []model.AffiliateLevelSettings{
		{
			Level:            "silver",
			CommissionRule:   json.RawMessage(`{"exam":{"type":"percent","value":12},"book":{"type":"fixed","value":5000}}`),
			BindingMonths:    intPtr(6),
			DigitalWaitDays:  intPtr(3),
			MinPayout:        f64Ptr(50000),
			PayoutDayOfMonth: intPtr(25),
		},
		{
			Level:            "gold",
			PhysicalWaitDays: intPtr(5),
			RiseThreshold:    f64Ptr(20000000),
			HoldThreshold:    f64Ptr(9000000),
		},
	}

	rules := buildAffiliateEffectiveRules(settings, levels)

	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3 (bronze synthesized even without a row)", len(rules))
	}

	silver := rules["silver"]
	if silver.Level != "silver" {
		t.Errorf("silver.Level = %q, want silver", silver.Level)
	}
	if silver.DigitalWaitDays != 3 {
		t.Errorf("silver.DigitalWaitDays = %d, want 3 (level override)", silver.DigitalWaitDays)
	}
	if silver.MinPayout != 50000 {
		t.Errorf("silver.MinPayout = %v, want 50000 (level override)", silver.MinPayout)
	}
	if silver.PayoutDayOfMonth == nil || *silver.PayoutDayOfMonth != 25 {
		t.Errorf("silver.PayoutDayOfMonth = %v, want 25 (level override)", silver.PayoutDayOfMonth)
	}
	if silver.PhysicalWaitDays != 14 {
		t.Errorf("silver.PhysicalWaitDays = %d, want 14 (global fallback)", silver.PhysicalWaitDays)
	}
	if silver.BindingMonths != 6 {
		t.Errorf("silver.BindingMonths = %d, want 6 (level override)", silver.BindingMonths)
	}
	if got := silver.CommissionRule["exam"]; got != (model.AffiliateCommissionRule{Type: "percent", Value: 12}) {
		t.Errorf("silver exam rule = %+v, want percent 12 (level override)", got)
	}
	if got := silver.CommissionRule["book"]; got != (model.AffiliateCommissionRule{Type: "fixed", Value: 5000}) {
		t.Errorf("silver book rule = %+v, want fixed 5000 (level override)", got)
	}
	if got := silver.CommissionRule["course"]; got != (model.AffiliateCommissionRule{Type: "percent", Value: 10}) {
		t.Errorf("silver course rule = %+v, want default percent 10", got)
	}
	if got := silver.CommissionRule["medal"]; got.Value != 0 {
		t.Errorf("silver medal rule = %+v, want default 0", got)
	}

	gold := rules["gold"]
	if gold.PhysicalWaitDays != 5 {
		t.Errorf("gold.PhysicalWaitDays = %d, want 5 (level override)", gold.PhysicalWaitDays)
	}
	if gold.BindingMonths != 3 {
		t.Errorf("gold.BindingMonths = %d, want 3 (global fallback)", gold.BindingMonths)
	}
	if gold.MinPayout != 100000 {
		t.Errorf("gold.MinPayout = %v, want 100000 (global fallback)", gold.MinPayout)
	}
	if gold.DigitalWaitDays != 7 {
		t.Errorf("gold.DigitalWaitDays = %d, want 7 (global fallback)", gold.DigitalWaitDays)
	}
	if gold.RiseThreshold == nil || *gold.RiseThreshold != 20000000 {
		t.Errorf("gold.RiseThreshold = %v, want 20000000 (from level row)", gold.RiseThreshold)
	}
	if gold.HoldThreshold == nil || *gold.HoldThreshold != 9000000 {
		t.Errorf("gold.HoldThreshold = %v, want 9000000 (from level row)", gold.HoldThreshold)
	}

	bronze := rules["bronze"]
	if bronze.MinPayout != 100000 || bronze.PayoutDayOfMonth != nil {
		t.Errorf("bronze payout fields = %v %v, want global defaults 100000 nil", bronze.MinPayout, bronze.PayoutDayOfMonth)
	}
	if bronze.RiseThreshold != nil || bronze.HoldThreshold != nil {
		t.Errorf("bronze thresholds = %v %v, want nil nil (bronze has no level row)", bronze.RiseThreshold, bronze.HoldThreshold)
	}
	if bronze.CommissionRule["exam"].Value != 10 || bronze.CommissionRule["book"].Value != 3 {
		t.Errorf("bronze.CommissionRule = %v, want defaults exam 10 book 3", bronze.CommissionRule)
	}
}

func TestAffiliateEffectiveRules_nilSettingsFallsBackToDefaults(t *testing.T) {
	rules := buildAffiliateEffectiveRules(nil, nil)
	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(rules))
	}
	silver := rules["silver"]
	if silver.DigitalWaitDays != 7 || silver.MinPayout != 100000 {
		t.Errorf("silver wait/payout = %d %v, want defaults 7 100000", silver.DigitalWaitDays, silver.MinPayout)
	}
	if silver.CommissionRule["exam"].Value != 10 {
		t.Errorf("silver.CommissionRule[exam] = %v, want default 10", silver.CommissionRule["exam"])
	}
}

func TestResolveAffiliateCommissionRule_precedence(t *testing.T) {
	defaultRules := buildAffiliateEffectiveRules(defaultAffiliateSettings(), nil)
	levelRules := buildAffiliateEffectiveRules(defaultAffiliateSettings(), []model.AffiliateLevelSettings{
		{Level: "silver", CommissionRule: json.RawMessage(`{"exam":{"type":"percent","value":12},"book":{"type":"fixed","value":5000}}`)},
	})

	tests := []struct {
		name        string
		override    *model.AffiliateProductOverride
		rule        AffiliateEffectiveRule
		productType string
		want        model.AffiliateCommissionRule
	}{
		{"override beats level and default", &model.AffiliateProductOverride{Type: "percent", Value: 25}, levelRules["silver"], "book", model.AffiliateCommissionRule{Type: "percent", Value: 25}},
		{"override type replaces the level type", &model.AffiliateProductOverride{Type: "fixed", Value: 7500}, levelRules["silver"], "exam", model.AffiliateCommissionRule{Type: "fixed", Value: 7500}},
		{"override wins even for defaulted category", &model.AffiliateProductOverride{Type: "percent", Value: 1}, levelRules["silver"], "course", model.AffiliateCommissionRule{Type: "percent", Value: 1}},
		{"level value beats default", nil, levelRules["silver"], "exam", model.AffiliateCommissionRule{Type: "percent", Value: 12}},
		{"level fixed value set", nil, levelRules["silver"], "book", model.AffiliateCommissionRule{Type: "fixed", Value: 5000}},
		{"default when level omits category", nil, levelRules["silver"], "merchandise", model.AffiliateCommissionRule{Type: "percent", Value: 3}},
		{"default when level has no rule", nil, defaultRules["bronze"], "exam", model.AffiliateCommissionRule{Type: "percent", Value: 10}},
		{"medal defaults to zero", nil, defaultRules["bronze"], "medal", model.AffiliateCommissionRule{Type: "percent", Value: 0}},
		{"unknown product type has no commission", nil, defaultRules["bronze"], "webinar", model.AffiliateCommissionRule{Type: "percent", Value: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAffiliateCommissionRule(tt.override, tt.rule, tt.productType)
			if got != tt.want {
				t.Errorf("resolveAffiliateCommissionRule(%v, %v, %q) = %v, want %v", tt.override, tt.rule.CommissionRule, tt.productType, got, tt.want)
			}
		})
	}
}

func TestAffiliateFlag_gate(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]string
		want bool
	}{
		{"unset disables (default off)", map[string]string{}, false},
		{"empty disables", map[string]string{"affiliate_enabled": ""}, false},
		{"false disables", map[string]string{"affiliate_enabled": "false"}, false},
		{"true enables", map[string]string{"affiliate_enabled": "true"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := affiliateEnabled(tt.cfg); got != tt.want {
				t.Errorf("affiliateEnabled(%v) = %v, want %v", tt.cfg, got, tt.want)
			}
		})
	}
}

func TestAffiliateSettingsLoadIntegration(t *testing.T) {
	svc, repo := newRealDBService(t)
	pool := repo.Pool()
	ctx := context.Background()

	var userID string
	var productID uuid.UUID
	var affID uuid.UUID

	st, err := svc.LoadAffiliateSettings(ctx)
	if err != nil {
		t.Fatalf("LoadAffiliateSettings (no row): %v", err)
	}
	if st == nil {
		t.Fatal("LoadAffiliateSettings() = nil with no row, want defaults")
	}
	if st.CookieDays != 30 {
		t.Errorf("no-row CookieDays = %d, want default 30", st.CookieDays)
	}

	// Cleanup is registered in FK-dependency order: t.Cleanup runs LIFO, so
	// children (clicks, overrides) delete before the parents they reference.
	// The shared real-DB fixture never resets rows between tests.
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliate_settings WHERE id = 1`) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliators WHERE user_id = $1`, userID) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, productID) })

	// Seed singleton row with non-defaults plus one level row and one override.
	// UPSERT because the shared real-DB fixture may already carry the row from
	// a sibling test.
	if _, err := pool.Exec(ctx,
		`INSERT INTO affiliate_settings (id, cookie_days, digital_wait_days, min_payout)
		 VALUES (1, 45, 3, 250000)
		 ON CONFLICT (id) DO UPDATE SET cookie_days = 45, digital_wait_days = 3, min_payout = 250000, updated_at = now()`); err != nil {
		t.Fatalf("seed affiliate_settings: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE affiliate_settings SET levels = '{"silver": {"commission_rule": {"exam":{"type":"percent","value":12},"book":{"type":"fixed","value":5000}}, "digital_wait_days": 3, "min_payout": 50000}}' WHERE id = 1`); err != nil {
		t.Fatalf("seed affiliate_settings levels: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO product (name, type, price, status) VALUES ($1, 'book', 60000, 'published') RETURNING id`,
		"Affiliate Override Product "+uniqueSuffix()).Scan(&productID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE product SET affiliate_commission = '{"type":"percent","value":25,"updated_at":"2026-10-01T00:00:00Z"}' WHERE id = $1`, productID); err != nil {
		t.Fatalf("seed product override: %v", err)
	}

	override, err := repo.LoadProductOverride(ctx, productID)
	if err != nil {
		t.Fatalf("LoadProductOverride: %v", err)
	}
	if override == nil || override.Type != "percent" || override.Value != 25 || override.ProductID != productID {
		t.Errorf("LoadProductOverride(%v) = %+v, want percentage 25", productID, override)
	}
	other, err := repo.LoadProductOverride(ctx, uuid.New())
	if err != nil || other != nil {
		t.Errorf("LoadProductOverride(unknown) = %v, %v; want nil, nil", other, err)
	}

	settings, rules, err := svc.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		t.Fatalf("LoadAffiliateEffectiveRules: %v", err)
	}
	if settings == nil || settings.CookieDays != 45 || settings.DigitalWaitDays != 3 || settings.MinPayout != 250000 {
		t.Errorf("LoadAffiliateEffectiveRules settings = %+v, want row values 45/3/250000", settings)
	}
	silver := rules["silver"]
	if silver.DigitalWaitDays != 3 {
		t.Errorf("silver.DigitalWaitDays = %d, want 3 (level row overrides row setting 3)", silver.DigitalWaitDays)
	}
	if silver.MinPayout != 50000 {
		t.Errorf("silver.MinPayout = %v, want 50000 (level override)", silver.MinPayout)
	}
	if silver.CommissionRule["exam"].Value != 12 {
		t.Errorf("silver.CommissionRule[exam] = %v, want 12 (level override)", silver.CommissionRule["exam"])
	}

	// FindAffiliatorByCode round trip on the seeded affiliator.
	code := strings.ToUpper("B1" + uniqueSuffix()[:6])
	name := "Integration Affiliator " + uniqueSuffix()
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affload_"+uniqueSuffix()).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version)
		 VALUES ($1, $2, 'active', 'v1') RETURNING user_id`,
		userID, code).Scan(&affID); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}

	aff, err := repo.FindAffiliatorByCode(ctx, code)
	if err != nil {
		t.Fatalf("FindAffiliatorByCode: %v", err)
	}
	if aff == nil || aff.Status != "active" || aff.ID != affID {
		t.Errorf("FindAffiliatorByCode(%q) = %+v, want active affiliator %v", code, aff, affID)
	}
	missing, err := repo.FindAffiliatorByCode(ctx, "NOPE"+uniqueSuffix())
	if err != nil || missing != nil {
		t.Errorf("FindAffiliatorByCode(unknown) = %v, %v; want nil, nil", missing, err)
	}

}

func TestAffiliateFlagGate_disablesPublicSurfaces(t *testing.T) {
	svc, repo := newRealDBService(t)
	pool := repo.Pool()
	ctx := context.Background()

	code := strings.ToUpper("C1" + uniqueSuffix()[:6])
	name := "Flag Gate Affiliator " + uniqueSuffix()
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, 'affiliator', 'active', false) RETURNING id`,
		name, "affflag_"+uniqueSuffix()).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	var affID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version)
		 VALUES ($1, $2, 'active', 'v1') RETURNING user_id`,
		userID, code).Scan(&affID); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	// Registration order is FK-dependency order: t.Cleanup runs LIFO, so
	// clicks delete before affiliators, affiliators before users.
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM system_config WHERE key = 'affiliate_enabled'`) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliators WHERE user_id = $1`, userID) })

	// Flag unset → default off: validate invalid.
	res, err := svc.ValidateAffiliateCode(ctx, code)
	if err != nil {
		t.Fatalf("ValidateAffiliateCode (flag unset): %v", err)
	}
	if res.Valid {
		t.Errorf("ValidateAffiliateCode (flag unset) = %+v, want valid:false", res)
	}

	// Flag on → validate names the affiliator.
	if _, err := pool.Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', 'true', false)
		 ON CONFLICT (key) DO UPDATE SET value = 'true'`); err != nil {
		t.Fatalf("set flag on: %v", err)
	}
	res, err = svc.ValidateAffiliateCode(ctx, code)
	if err != nil {
		t.Fatalf("ValidateAffiliateCode (flag on): %v", err)
	}
	if !res.Valid || res.AffiliatorName != name {
		t.Errorf("ValidateAffiliateCode (flag on) = %+v, want valid with name %q", res, name)
	}

	// Flag off → validate invalid.
	if _, err := pool.Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', 'false', false)
		 ON CONFLICT (key) DO UPDATE SET value = 'false'`); err != nil {
		t.Fatalf("set flag off: %v", err)
	}
	res, err = svc.ValidateAffiliateCode(ctx, code)
	if err != nil {
		t.Fatalf("ValidateAffiliateCode (flag off): %v", err)
	}
	if res.Valid {
		t.Errorf("ValidateAffiliateCode (flag off) = %+v, want valid:false", res)
	}

	// Inactive affiliator with flag on → invalid.
	if _, err := pool.Exec(ctx,
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', 'true', false)
		 ON CONFLICT (key) DO UPDATE SET value = 'true'`); err != nil {
		t.Fatalf("set flag on: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE affiliators SET program_status = 'pending_approval' WHERE user_id = $1`, affID); err != nil {
		t.Fatalf("deactivate affiliator: %v", err)
	}
	res, err = svc.ValidateAffiliateCode(ctx, code)
	if err != nil {
		t.Fatalf("ValidateAffiliateCode (inactive): %v", err)
	}
	if res.Valid {
		t.Errorf("ValidateAffiliateCode (inactive) = %+v, want valid:false", res)
	}

	// Unknown code → invalid, empty name, no error (no user enumeration).
	res, err = svc.ValidateAffiliateCode(ctx, "UNKNOWN1")
	if err != nil {
		t.Fatalf("ValidateAffiliateCode (unknown): %v", err)
	}
	if res.Valid || res.AffiliatorName != "" {
		t.Errorf("ValidateAffiliateCode (unknown) = %+v, want valid:false empty name", res)
	}
}
