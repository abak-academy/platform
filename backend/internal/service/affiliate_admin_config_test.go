package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validAffiliateConfigSettingsInput() AffiliateAdminSettingsInput {
	return AffiliateAdminSettingsInput{
		CookieDays:           30,
		BindingMonths:        3,
		MinExtendAmount:      0,
		DigitalWaitDays:      7,
		PhysicalReceivedDays: 7,
		PhysicalWaitDays:     7,
		MinPayout:            100000,
		AdjustmentThreshold:  1000000,
		FreezeMaxDays:        14,
		SoonEndDays:          30,
		EvalDay:              8,
		SilverThreshold:      5000000,
		GoldThreshold:        15000000,
		HoldThresholds:       json.RawMessage(`{"bronze":1000000,"silver":null}`),
	}
}

func TestAffiliateConfigSettingsPUTValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(in *AffiliateAdminSettingsInput)
	}{
		{"cookie_days below 1", func(in *AffiliateAdminSettingsInput) { in.CookieDays = 0 }},
		{"cookie_days above 180", func(in *AffiliateAdminSettingsInput) { in.CookieDays = 181 }},
		{"binding_months below 1", func(in *AffiliateAdminSettingsInput) { in.BindingMonths = 0 }},
		{"binding_months above 12", func(in *AffiliateAdminSettingsInput) { in.BindingMonths = 13 }},
		{"digital_wait_days below 0", func(in *AffiliateAdminSettingsInput) { in.DigitalWaitDays = -1 }},
		{"digital_wait_days above 90", func(in *AffiliateAdminSettingsInput) { in.DigitalWaitDays = 91 }},
		{"physical_received_days below 0", func(in *AffiliateAdminSettingsInput) { in.PhysicalReceivedDays = -1 }},
		{"physical_received_days above 30", func(in *AffiliateAdminSettingsInput) { in.PhysicalReceivedDays = 31 }},
		{"physical_wait_days below 0", func(in *AffiliateAdminSettingsInput) { in.PhysicalWaitDays = -1 }},
		{"physical_wait_days above 90", func(in *AffiliateAdminSettingsInput) { in.PhysicalWaitDays = 91 }},
		{"min_payout zero", func(in *AffiliateAdminSettingsInput) { in.MinPayout = 0 }},
		{"min_payout negative", func(in *AffiliateAdminSettingsInput) { in.MinPayout = -1 }},
		{"payout_day_of_month below 1", func(in *AffiliateAdminSettingsInput) { in.PayoutDayOfMonth = intPtr(0) }},
		{"payout_day_of_month above 28", func(in *AffiliateAdminSettingsInput) { in.PayoutDayOfMonth = intPtr(29) }},
		{"adjustment_threshold zero", func(in *AffiliateAdminSettingsInput) { in.AdjustmentThreshold = 0 }},
		{"freeze_max_days below 1", func(in *AffiliateAdminSettingsInput) { in.FreezeMaxDays = 0 }},
		{"freeze_max_days above 90", func(in *AffiliateAdminSettingsInput) { in.FreezeMaxDays = 91 }},
		{"soon_end_days below 1", func(in *AffiliateAdminSettingsInput) { in.SoonEndDays = 0 }},
		{"soon_end_days above 180", func(in *AffiliateAdminSettingsInput) { in.SoonEndDays = 181 }},
		{"eval_day below 1", func(in *AffiliateAdminSettingsInput) { in.EvalDay = 0 }},
		{"eval_day above 28", func(in *AffiliateAdminSettingsInput) { in.EvalDay = 29 }},
		{"silver_threshold zero", func(in *AffiliateAdminSettingsInput) { in.SilverThreshold = 0 }},
		{"gold_threshold zero", func(in *AffiliateAdminSettingsInput) { in.GoldThreshold = 0 }},
		{"hold_thresholds unknown key", func(in *AffiliateAdminSettingsInput) { in.HoldThresholds = json.RawMessage(`{"platinum":1}`) }},
		{"hold_thresholds non-number", func(in *AffiliateAdminSettingsInput) { in.HoldThresholds = json.RawMessage(`{"bronze":"x"}`) }},
		{"hold_thresholds non-object", func(in *AffiliateAdminSettingsInput) { in.HoldThresholds = json.RawMessage(`[1]`) }},
		{"daily_registration_cap zero", func(in *AffiliateAdminSettingsInput) { in.DailyRegistrationCap = intPtr(0) }},
		{"launch_date bad format", func(in *AffiliateAdminSettingsInput) { in.LaunchDate = strPtr("tomorrow") }},
		{"level unknown", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "platinum"})
		}},
		{"level pct unknown category", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", CommissionPct: map[string]float64{"webinar": 5}})
		}},
		{"level pct above 100", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", CommissionPct: map[string]float64{"exam": 101}})
		}},
		{"level pct below 0", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "gold", CommissionPct: map[string]float64{"exam": -1}})
		}},
		{"level wait above 90", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", DigitalWaitDays: intPtr(91)})
		}},
		{"level received days above 30", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", PhysicalReceivedDays: intPtr(31)})
		}},
		{"level min_payout zero", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", MinPayout: f64Ptr(0)})
		}},
		{"level payout day 29", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "silver", PayoutDayOfMonth: intPtr(29)})
		}},
		{"level rise threshold negative", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "gold", RiseThreshold: f64Ptr(-1)})
		}},
		{"level hold threshold negative", func(in *AffiliateAdminSettingsInput) {
			in.Levels = append(in.Levels, AffiliateAdminLevelInput{Level: "gold", HoldThreshold: f64Ptr(-1)})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validAffiliateConfigSettingsInput()
			tt.mutate(&in)
			err := validateAffiliateSettingsInput(in)
			if !errors.Is(err, ErrValidation) {
				t.Errorf("validateAffiliateSettingsInput() err = %v, want ErrValidation", err)
			}
		})
	}

	t.Run("boundaries and nullables are accepted", func(t *testing.T) {
		in := validAffiliateConfigSettingsInput()
		in.CookieDays = 180
		in.BindingMonths = 12
		in.DigitalWaitDays = 90
		in.PhysicalReceivedDays = 30
		in.PhysicalWaitDays = 90
		in.PayoutDayOfMonth = intPtr(28)
		in.FreezeMaxDays = 90
		in.SoonEndDays = 180
		in.EvalDay = 28
		in.LaunchDate = strPtr("2026-11-01")
		in.DailyRegistrationCap = intPtr(1)
		in.HoldThresholds = nil
		in.Levels = []AffiliateAdminLevelInput{
			{
				Level:                "bronze",
				CommissionPct:        map[string]float64{"exam": 0, "course": 100},
				DigitalWaitDays:      intPtr(0),
				PhysicalReceivedDays: intPtr(0),
				PhysicalWaitDays:     intPtr(90),
				MinPayout:            f64Ptr(1),
				PayoutDayOfMonth:     nil,
				RiseThreshold:        f64Ptr(0),
				HoldThreshold:        nil,
			},
			{Level: "silver", CommissionPct: nil, MinPayout: f64Ptr(50000), PayoutDayOfMonth: intPtr(1)},
			{Level: "gold"},
		}
		if err := validateAffiliateSettingsInput(in); err != nil {
			t.Errorf("validateAffiliateSettingsInput(valid) err = %v, want nil", err)
		}
	})
}

func TestAffiliateConfigSettingsUpdateAuditAndReadback(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	actor := seedAffiliateAdminActor(t, repo, "affcfg-actor-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliate_settings WHERE id = 1`) })
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliate_level_settings`) })

	_, err := pool.Exec(ctx, `DELETE FROM affiliate_level_settings`)
	require.NoError(t, err)

	input := validAffiliateConfigSettingsInput()
	input.CookieDays = 45
	input.MinPayout = 250000
	input.LaunchDate = strPtr("2026-11-01")
	input.HoldThresholds = json.RawMessage(`{"bronze":1000000,"silver":null}`)
	input.Levels = []AffiliateAdminLevelInput{
		{Level: "silver", CommissionPct: map[string]float64{"exam": 12}, DigitalWaitDays: intPtr(3), MinPayout: f64Ptr(50000)},
	}

	got, err := svc.UpdateAffiliateAdminSettings(ctx, actor, input)
	require.NoError(t, err)

	if got.Settings.CookieDays != 45 || got.Settings.MinPayout != 250000 {
		t.Errorf("readback settings = %d %v, want 45 250000", got.Settings.CookieDays, got.Settings.MinPayout)
	}
	if got.Settings.LaunchDate == nil || !got.Settings.LaunchDate.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("readback LaunchDate = %v, want 2026-11-01", got.Settings.LaunchDate)
	}
	var holds map[string]*float64
	require.NoError(t, json.Unmarshal(got.Settings.HoldThresholds, &holds))
	if holds["bronze"] == nil || *holds["bronze"] != 1000000 || holds["silver"] != nil {
		t.Errorf("readback hold_thresholds = %v, want bronze 1000000 silver null", holds)
	}
	if got.Settings.UpdatedBy == nil || got.Settings.UpdatedBy.String() != actor {
		t.Errorf("readback UpdatedBy = %v, want actor %s", got.Settings.UpdatedBy, actor)
	}

	if len(got.Levels) != 3 {
		t.Fatalf("readback levels = %d rows, want 3 (bronze/gold synthesized)", len(got.Levels))
	}
	byLevel := make(map[string]model.AffiliateLevelSettings, len(got.Levels))
	for _, l := range got.Levels {
		byLevel[l.Level] = l
	}
	silver := byLevel["silver"]
	var silverPct map[string]float64
	require.NoError(t, json.Unmarshal(silver.CommissionPct, &silverPct))
	if silverPct["exam"] != 12 {
		t.Errorf("silver commission_pct = %v, want exam 12", silverPct)
	}
	if silver.DigitalWaitDays == nil || *silver.DigitalWaitDays != 3 || silver.MinPayout == nil || *silver.MinPayout != 50000 {
		t.Errorf("silver waits/min_payout = %v %v, want 3 50000", silver.DigitalWaitDays, silver.MinPayout)
	}
	if byLevel["bronze"].DigitalWaitDays != nil || byLevel["bronze"].MinPayout != nil {
		t.Errorf("bronze row = %+v, want synthesized nil fields", byLevel["bronze"])
	}

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliate_settings' AND target_id = '1' AND action = 'affiliate.settings_update'`,
	).Scan(&auditCount))
	if auditCount != 1 {
		t.Fatalf("affiliate.settings_update audit rows = %d, want 1", auditCount)
	}
	var actorID string
	var rawMeta []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT actor_id::text, metadata FROM audit_log WHERE target_type = 'affiliate_settings' AND action = 'affiliate.settings_update'`,
	).Scan(&actorID, &rawMeta))
	if actorID != actor {
		t.Errorf("audit actor_id = %s, want %s", actorID, actor)
	}
	var meta map[string]map[string]any
	require.NoError(t, json.Unmarshal(rawMeta, &meta))
	cookieChange, ok := meta["cookie_days"]
	if !ok {
		t.Fatalf("audit meta missing cookie_days change: %s", rawMeta)
	}
	if cookieChange["old"].(float64) != 30 || cookieChange["new"].(float64) != 45 {
		t.Errorf("audit meta cookie_days = %v, want old 30 new 45", cookieChange)
	}
	minPayoutChange, ok := meta["min_payout"]
	if !ok || minPayoutChange["old"].(float64) != 100000 || minPayoutChange["new"].(float64) != 250000 {
		t.Errorf("audit meta min_payout = %v, want old 100000 new 250000", minPayoutChange)
	}
	if _, ok := meta["binding_months"]; ok {
		t.Errorf("audit meta contains unchanged binding_months: %s", rawMeta)
	}
	levelChange, ok := meta["level.silver"]
	if !ok {
		t.Fatalf("audit meta missing level.silver change: %s", rawMeta)
	}
	newRow, ok := levelChange["new"].(map[string]any)
	if !ok || newRow["level"] != "silver" {
		t.Errorf("audit meta level.silver = %v, want row with level silver", levelChange)
	}

	// A second PUT that changes only one field records only that field.
	input.EvalDay = 12
	_, err = svc.UpdateAffiliateAdminSettings(ctx, actor, input)
	require.NoError(t, err)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliate_settings' AND target_id = '1' AND action = 'affiliate.settings_update'`,
	).Scan(&auditCount))
	if auditCount != 2 {
		t.Fatalf("affiliate.settings_update audit rows after second PUT = %d, want 2", auditCount)
	}
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT metadata FROM audit_log WHERE target_type = 'affiliate_settings' AND target_id = '1' AND action = 'affiliate.settings_update'
		 ORDER BY id DESC LIMIT 1`,
	).Scan(&rawMeta))
	var secondMeta map[string]map[string]any
	require.NoError(t, json.Unmarshal(rawMeta, &secondMeta))
	if _, ok := secondMeta["eval_day"]; !ok {
		t.Fatalf("second audit meta missing eval_day change: %s", rawMeta)
	}
	if len(secondMeta) != 1 {
		t.Errorf("second audit meta records %d changes, want only eval_day: %s", len(secondMeta), rawMeta)
	}
}

func TestAffiliateConfigOverridesUpsertDeleteFallback(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	actor := seedAffiliateAdminActor(t, repo, "affcfg-ovr-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })

	var productID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO product (name, type, price, status) VALUES ($1, 'book', 60000, 'published') RETURNING id`,
		"Config Override Product "+uniqueSuffix(),
	).Scan(&productID))
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, productID) })

	override, err := svc.AdminUpsertAffiliateOverride(ctx, actor, productID.String(), 25)
	require.NoError(t, err)
	if override.Percentage != 25 || override.ProductID != productID {
		t.Errorf("AdminUpsertAffiliateOverride() = %+v, want percentage 25 product %v", override, productID)
	}

	rows, err := svc.AdminListAffiliateOverrides(ctx)
	require.NoError(t, err)
	var listed *repository.AffiliateOverrideRow
	for i := range rows {
		if rows[i].ProductID == productID {
			listed = &rows[i]
		}
	}
	if listed == nil {
		t.Fatalf("override list misses product %v: %+v", productID, rows)
	}
	if listed.ProductName == "" {
		t.Errorf("override list row carries no product name: %+v", listed)
	}

	// Re-upsert the same product: one row, new percentage.
	override, err = svc.AdminUpsertAffiliateOverride(ctx, actor, productID.String(), 50)
	require.NoError(t, err)
	if override.Percentage != 50 {
		t.Errorf("re-upsert percentage = %v, want 50", override.Percentage)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_product_overrides WHERE product_id = $1`, productID).Scan(&count))
	if count != 1 {
		t.Fatalf("affiliate_product_overrides rows = %d, want 1 after upsert", count)
	}

	require.NoError(t, svc.AdminDeleteAffiliateOverride(ctx, actor, productID.String()))
	stored, err := repo.LoadProductOverride(ctx, productID)
	require.NoError(t, err)
	if stored != nil {
		t.Errorf("LoadProductOverride after delete = %+v, want nil (falls back to level/global)", stored)
	}

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliate_product_override' AND target_id = $1 AND action = 'affiliate.product_override'`,
		productID).Scan(&auditCount))
	if auditCount != 3 {
		t.Errorf("affiliate.product_override audit rows = %d, want 3 (two upserts + delete)", auditCount)
	}

	err = svc.AdminDeleteAffiliateOverride(ctx, actor, productID.String())
	if !errors.Is(err, ErrAffiliateNotFound) {
		t.Errorf("delete missing override err = %v, want ErrAffiliateNotFound", err)
	}
	unknown := uuid.New()
	_, err = svc.AdminUpsertAffiliateOverride(ctx, actor, unknown.String(), 10)
	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("upsert override for unknown product err = %v, want ErrProductNotFound", err)
	}
	_, err = svc.AdminUpsertAffiliateOverride(ctx, actor, productID.String(), 101)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("upsert override percentage 101 err = %v, want ErrValidation", err)
	}
}

func TestAffiliateConfigFlagReview(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	owner := seedAffiliatorOwner(t, repo, "affcfg-flag-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "affcfg-flagadm-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_flags WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, owner.userID, actor) })

	var flagID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_flags (affiliator_id, rule, payload, status)
		 VALUES ($1, 'self_purchase', '{}', 'open') RETURNING id`,
		owner.affiliatorID).Scan(&flagID))

	rows, err := svc.AdminListAffiliateFlags(ctx, "")
	require.NoError(t, err)
	var listed bool
	for _, row := range rows {
		if row.ID == flagID {
			listed = row.AffiliatorCode == owner.code && row.AffiliatorName != ""
		}
	}
	if !listed {
		t.Errorf("open flag %v not listed with affiliator code/name: %+v", flagID, rows)
	}

	if _, err := svc.AdminListAffiliateFlags(ctx, "bogus"); !errors.Is(err, ErrValidation) {
		t.Errorf("AdminListAffiliateFlags(bogus) err = %v, want ErrValidation", err)
	}

	_, err = svc.AdminReviewAffiliateFlag(ctx, actor, flagID.String(), AffiliateAdminFlagReviewInput{Status: "open"})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("review with status open err = %v, want ErrValidation", err)
	}

	reviewed, err := svc.AdminReviewAffiliateFlag(ctx, actor, flagID.String(), AffiliateAdminFlagReviewInput{Status: "reviewed", Note: strPtr("cek manual")})
	require.NoError(t, err)
	if reviewed.Status != "reviewed" || reviewed.Note == nil || *reviewed.Note != "cek manual" {
		t.Errorf("reviewed flag = %+v, want status reviewed note 'cek manual'", reviewed)
	}
	if reviewed.ReviewedBy == nil || reviewed.ReviewedBy.String() != actor || reviewed.ReviewedAt == nil {
		t.Errorf("reviewed flag reviewed_by/at = %v %v, want actor %s and timestamp", reviewed.ReviewedBy, reviewed.ReviewedAt, actor)
	}

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliate_flag' AND target_id = $1 AND action = 'affiliate.flag_review'`,
		flagID).Scan(&auditCount))
	if auditCount != 1 {
		t.Errorf("affiliate.flag_review audit rows = %d, want 1", auditCount)
	}

	_, err = svc.AdminReviewAffiliateFlag(ctx, actor, flagID.String(), AffiliateAdminFlagReviewInput{Status: "valid"})
	if !errors.Is(err, ErrAffiliateFlagNotOpen) {
		t.Errorf("second review err = %v, want ErrAffiliateFlagNotOpen", err)
	}
	_, err = svc.AdminReviewAffiliateFlag(ctx, actor, uuid.NewString(), AffiliateAdminFlagReviewInput{Status: "valid"})
	if !errors.Is(err, ErrAffiliateNotFound) {
		t.Errorf("review unknown flag err = %v, want ErrAffiliateNotFound", err)
	}
}

func TestAffiliateConfigAppealDecide(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	owner := seedAffiliatorOwner(t, repo, "affcfg-apl-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "affcfg-apladm-"+uniqueSuffix()+"@test.local")
	actorUUID := uuid.MustParse(actor)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_violations WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, owner.userID, actor) })

	violationID := seedAffiliateMeViolation(t, repo, owner.affiliatorID, actorUUID, "resolved", "pending", time.Now())

	rows, err := svc.AdminListAffiliateAppeals(ctx, "")
	require.NoError(t, err)
	var listed bool
	for _, row := range rows {
		if row.ID == violationID {
			listed = row.AppealStatus == "pending" && row.AffiliatorCode == owner.code
		}
	}
	if !listed {
		t.Errorf("pending appeal %v not listed with affiliator code: %+v", violationID, rows)
	}

	_, err = svc.AdminDecideAffiliateAppeal(ctx, actor, violationID.String(), AffiliateAdminAppealDecisionInput{Outcome: "maybe"})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("decide with bad outcome err = %v, want ErrValidation", err)
	}
	_, err = svc.AdminDecideAffiliateAppeal(ctx, actor, uuid.NewString(), AffiliateAdminAppealDecisionInput{Outcome: "upheld"})
	if !errors.Is(err, ErrAffiliateNotFound) {
		t.Errorf("decide unknown violation err = %v, want ErrAffiliateNotFound", err)
	}

	decided, err := svc.AdminDecideAffiliateAppeal(ctx, actor, violationID.String(), AffiliateAdminAppealDecisionInput{Outcome: "rejected", Note: strPtr("bukti kurang")})
	require.NoError(t, err)
	if decided.AppealStatus != "rejected" || decided.Status != "resolved" {
		t.Errorf("decided violation = %+v, want appeal rejected and violation resolved", decided)
	}
	if decided.AppealDecidedBy == nil || *decided.AppealDecidedBy != actorUUID || decided.AppealDecidedAt == nil {
		t.Errorf("appeal_decided_by/at = %v %v, want actor %s and timestamp", decided.AppealDecidedBy, decided.AppealDecidedAt, actor)
	}

	var auditCount int
	var rawMeta []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*), max(metadata::text) FROM audit_log WHERE target_type = 'affiliate_violation' AND target_id = $1 AND action = 'affiliate.appeal_decide'`,
		violationID).Scan(&auditCount, &rawMeta))
	if auditCount != 1 {
		t.Fatalf("affiliate.appeal_decide audit rows = %d, want 1", auditCount)
	}
	if rawMeta == nil || !bytes.Contains(rawMeta, []byte("bukti kurang")) {
		t.Errorf("appeal audit metadata = %s, want the decision note", rawMeta)
	}

	_, err = svc.AdminDecideAffiliateAppeal(ctx, actor, violationID.String(), AffiliateAdminAppealDecisionInput{Outcome: "upheld"})
	if !errors.Is(err, ErrAffiliateAppealNotPending) {
		t.Errorf("second decide err = %v, want ErrAffiliateAppealNotPending", err)
	}
}

func TestAffiliateConfigReportSummary(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	owner := seedAffiliatorOwner(t, repo, "affcfg-rep-"+uniqueSuffix()+"@test.local", "", "active")
	other := seedAffiliatorOwner(t, repo, "affcfg-rep2-"+uniqueSuffix()+"@test.local", "", "active")
	studentA := seedAffiliateTestStudent(t, repo, "affcfg-rep-sa-"+uniqueSuffix()+"@test.local", "")
	studentB := seedAffiliateTestStudent(t, repo, "affcfg-rep-sb-"+uniqueSuffix()+"@test.local", "")
	studentC := seedAffiliateTestStudent(t, repo, "affcfg-rep-sc-"+uniqueSuffix()+"@test.local", "")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_bindings WHERE affiliator_id IN ($1, $2)`, owner.affiliatorID, other.affiliatorID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_commissions WHERE affiliator_id IN ($1, $2)`, owner.affiliatorID, other.affiliatorID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_clicks WHERE affiliator_id IN ($1, $2)`, owner.affiliatorID, other.affiliatorID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3, $4, $5)`, owner.userID, other.userID, studentA.ID, studentB.ID, studentC.ID)
	})

	from := time.Now().Add(-time.Hour).UTC()
	to := time.Now().Add(time.Hour).UTC()

	// Delta baseline: the shared fixture carries other tests' rows.
	baseline, err := svc.AdminAffiliateReportSummary(ctx, from.Format(time.RFC3339), to.Format(time.RFC3339))
	require.NoError(t, err)

	createAffiliateBindingCommitted(t, svc, repo, studentA.ID, owner.affiliatorID, "manual_code")
	createAffiliateBindingCommitted(t, svc, repo, studentB.ID, owner.affiliatorID, "link")
	createAffiliateBindingCommitted(t, svc, repo, studentC.ID, owner.affiliatorID, "manual_code")
	// A binding started before the window is not a students_new.
	_, err = pool.Exec(ctx,
		`UPDATE affiliate_bindings SET started_at = $2 WHERE affiliator_id = $1 AND student_id = $3`,
		owner.affiliatorID, from.Add(-48*time.Hour), studentC.ID)
	require.NoError(t, err)

	cAvailable := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(studentA.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: "available", level: "bronze",
		createdAt: from, paidAt: from.Add(time.Minute),
	})
	cPaid := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(studentB.ID), affiliateJobCommissionSeed{
		productType: "book", base: 200000, pct: 3, amount: 6000, status: "paid", level: "bronze",
		createdAt: from, paidAt: from.Add(2 * time.Minute),
	})
	// Outside-window available commission does not count into net_revenue.
	seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(studentA.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 999999, pct: 10, amount: 99999, status: "available", level: "bronze",
		createdAt: from.Add(-72 * time.Hour), paidAt: from.Add(-72 * time.Hour),
	})
	// Pending/cancelled commissions appear in the status buckets only.
	seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(studentB.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 50000, pct: 10, amount: 5000, status: "pending", level: "bronze",
		createdAt: from, paidAt: from.Add(time.Minute),
	})
	seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(studentB.ID), affiliateJobCommissionSeed{
		productType: "book", base: 30000, pct: 3, amount: 900, status: "cancelled", level: "bronze",
		createdAt: from, paidAt: from.Add(time.Minute),
	})

	for i := 0; i < 2; i++ {
		_, err := pool.Exec(ctx,
			`INSERT INTO affiliate_clicks (affiliator_id, code, ip_hash, click_day, created_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			owner.affiliatorID, owner.code, uuid.NewString()[:16], from.Add(time.Duration(i)*time.Minute).In(affiliateWIBLocation()), from.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO affiliate_clicks (affiliator_id, code, ip_hash, click_day, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		owner.affiliatorID, owner.code, uuid.NewString()[:16], from.Add(-72*time.Hour).In(affiliateWIBLocation()), from.Add(-72*time.Hour))
	require.NoError(t, err)

	report, err := svc.AdminAffiliateReportSummary(ctx, from.Format(time.RFC3339), to.Format(time.RFC3339))
	require.NoError(t, err)

	cOwner := findAffiliateReportRow(report, owner.affiliatorID)
	if cOwner == nil {
		t.Fatalf("summary misses owner row: %+v", report.Data)
	}
	_ = cAvailable
	_ = cPaid
	if cOwner.StudentsNew != 2 {
		t.Errorf("students_new = %d, want 2", cOwner.StudentsNew)
	}
	if cOwner.NetRevenue != 300000 {
		t.Errorf("net_revenue = %v, want 300000 (available 100000 + paid 200000 base)", cOwner.NetRevenue)
	}
	if cOwner.Commissions["available"] != 10000 || cOwner.Commissions["paid"] != 6000 ||
		cOwner.Commissions["pending"] != 5000 || cOwner.Commissions["cancelled"] != 900 {
		t.Errorf("commissions by status = %v, want available 10000 paid 6000 pending 5000 cancelled 900", cOwner.Commissions)
	}
	if cOwner.Clicks != 2 {
		t.Errorf("clicks = %d, want 2", cOwner.Clicks)
	}
	if cOwner.Conversion != 1 {
		t.Errorf("conversion = %v, want 1 (2 registrations / 2 clicks)", cOwner.Conversion)
	}
	cOther := findAffiliateReportRow(report, other.affiliatorID)
	if cOther == nil || cOther.NetRevenue != 0 {
		t.Errorf("other affiliator row = %+v, want zero-data row", cOther)
	}
	totalsBefore, totalsAfter := baseline.Totals, report.Totals
	if totalsAfter.StudentsNew-totalsBefore.StudentsNew != 2 {
		t.Errorf("totals students_new delta = %d, want 2", totalsAfter.StudentsNew-totalsBefore.StudentsNew)
	}
	if totalsAfter.NetRevenue-totalsBefore.NetRevenue != 300000 {
		t.Errorf("totals net_revenue delta = %v, want 300000", totalsAfter.NetRevenue-totalsBefore.NetRevenue)
	}
	if totalsAfter.Clicks-totalsBefore.Clicks != 2 {
		t.Errorf("totals clicks delta = %d, want 2", totalsAfter.Clicks-totalsBefore.Clicks)
	}
	if totalsAfter.Commissions["pending"]-totalsBefore.Commissions["pending"] != 5000 {
		t.Errorf("totals pending delta = %v, want 5000", totalsAfter.Commissions["pending"]-totalsBefore.Commissions["pending"])
	}

	_, err = svc.AdminAffiliateReportSummary(ctx, "", to.Format(time.RFC3339))
	if !errors.Is(err, ErrMissingField) {
		t.Errorf("summary without from err = %v, want ErrMissingField", err)
	}
	_, err = svc.AdminAffiliateReportSummary(ctx, "not-a-date", to.Format(time.RFC3339))
	if !errors.Is(err, ErrInvalidDateFormat) {
		t.Errorf("summary with bad from err = %v, want ErrInvalidDateFormat", err)
	}
}

func TestAffiliateConfigReportSummary_dateOnlyWindowIncludesPaidDay(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	owner := seedAffiliatorOwner(t, repo, "affcfg-day-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "affcfg-day-s-"+uniqueSuffix()+"@test.local", "")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_commissions WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, owner.userID, student.ID) })

	paidAt := time.Now().In(affiliateWIBLocation())
	seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 120000, pct: 10, amount: 12000, status: "available", level: "bronze",
		createdAt: paidAt, paidAt: paidAt,
	})

	day := paidAt.Format("2006-01-02")
	report, err := svc.AdminAffiliateReportSummary(ctx, day, day)
	require.NoError(t, err)
	row := findAffiliateReportRow(report, owner.affiliatorID)
	if row == nil || row.NetRevenue != 120000 {
		t.Errorf("date-only window net_revenue = %+v, want 120000 (paid same WIB day)", row)
	}
}

func TestAffiliateConfigReportLiability(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	owner := seedAffiliatorOwner(t, repo, "affcfg-liab-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "affcfg-liab-s-"+uniqueSuffix()+"@test.local", "")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_ledger_entries WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_payouts WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_commissions WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, owner.userID, student.ID) })

	// Delta baseline: the shared fixture carries other tests' ledger/payout rows.
	before, err := svc.AdminAffiliateReportLiability(ctx)
	require.NoError(t, err)

	cAvailable := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: "available", level: "bronze", createdAt: time.Now(),
	})
	cPaid := seedAffiliateJobCommission(t, repo, owner.affiliatorID, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
		productType: "book", base: 200000, pct: 3, amount: 6000, status: "paid", level: "bronze", createdAt: time.Now(),
	})
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id) VALUES ($1, 'commission', 10000, $2) RETURNING id`,
		owner.affiliatorID, cAvailable.ID).Scan(new(uuid.UUID)))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id) VALUES ($1, 'commission', 6000, $2) RETURNING id`,
		owner.affiliatorID, cPaid.ID).Scan(new(uuid.UUID)))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, commission_id) VALUES ($1, 'refund_deduction', -4000, $2) RETURNING id`,
		owner.affiliatorID, cAvailable.ID).Scan(new(uuid.UUID)))

	payoutRequested := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "requested", 30000, "Liability Seed")
	payoutProcessing := seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "processing", 10000, "Liability Seed")
	seedAffiliateMoneyPayout(t, repo, owner.affiliatorID, "paid", 50000, "Liability Seed")
	_ = payoutRequested
	_ = payoutProcessing

	after, err := svc.AdminAffiliateReportLiability(ctx)
	require.NoError(t, err)
	// Delta assertions: the shared fixture carries other tests' ledger rows.
	if after.ByStatus["available"]-before.ByStatus["available"] != 6000 {
		t.Errorf("liability available delta = %v, want 6000 (10000 commission - 4000 refund deduction)", after.ByStatus["available"]-before.ByStatus["available"])
	}
	if after.ByStatus["paid"]-before.ByStatus["paid"] != 6000 {
		t.Errorf("liability paid delta = %v, want 6000", after.ByStatus["paid"]-before.ByStatus["paid"])
	}
	if after.PayoutLiability-before.PayoutLiability != 40000 {
		t.Errorf("payout liability delta = %v, want 40000 (requested + processing)", after.PayoutLiability-before.PayoutLiability)
	}
}

func findAffiliateReportRow(report *AffiliateReportSummary, id uuid.UUID) *repository.AffiliateReportRow {
	for i := range report.Data {
		if report.Data[i].AffiliatorID == id {
			return &report.Data[i]
		}
	}
	return nil
}
