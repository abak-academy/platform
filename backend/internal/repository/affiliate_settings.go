package repository

import (
	"context"
	"errors"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LoadAffiliateSettings returns the affiliate_settings singleton row (id=1).
// A missing row returns (nil, nil) — the service owns the defaults policy.
func (r *Repository) LoadAffiliateSettings(ctx context.Context) (*model.AffiliateSettings, error) {
	var s model.AffiliateSettings
	err := r.pool.QueryRow(ctx,
		`SELECT id, cookie_days, binding_months, min_extend_amount, digital_wait_days,
		        physical_received_days, physical_wait_days, min_payout, payout_day_of_month,
		        adjustment_threshold, adjustment_threshold_enabled, freeze_max_days, soon_end_days,
		        launch_date, eval_day, silver_threshold, gold_threshold, hold_thresholds,
		        daily_registration_cap, updated_by, updated_at
		 FROM affiliate_settings
		 WHERE id = 1`,
	).Scan(&s.ID, &s.CookieDays, &s.BindingMonths, &s.MinExtendAmount, &s.DigitalWaitDays,
		&s.PhysicalReceivedDays, &s.PhysicalWaitDays, &s.MinPayout, &s.PayoutDayOfMonth,
		&s.AdjustmentThreshold, &s.AdjustmentThresholdEnabled, &s.FreezeMaxDays, &s.SoonEndDays,
		&s.LaunchDate, &s.EvalDay, &s.SilverThreshold, &s.GoldThreshold, &s.HoldThresholds,
		&s.DailyRegistrationCap, &s.UpdatedBy, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadAffiliateLevelSettings returns every affiliate_level_settings row.
func (r *Repository) LoadAffiliateLevelSettings(ctx context.Context) ([]model.AffiliateLevelSettings, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT level, commission_pct, digital_wait_days, physical_received_days,
		        physical_wait_days, min_payout, payout_day_of_month, rise_threshold, hold_threshold
		 FROM affiliate_level_settings
		 ORDER BY level`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AffiliateLevelSettings
	for rows.Next() {
		var l model.AffiliateLevelSettings
		if err := rows.Scan(&l.Level, &l.CommissionPct, &l.DigitalWaitDays, &l.PhysicalReceivedDays,
			&l.PhysicalWaitDays, &l.MinPayout, &l.PayoutDayOfMonth, &l.RiseThreshold, &l.HoldThreshold); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// LoadProductOverride returns the commission override for one product.
// A missing row returns (nil, nil).
func (r *Repository) LoadProductOverride(ctx context.Context, productID uuid.UUID) (*model.AffiliateProductOverride, error) {
	var o model.AffiliateProductOverride
	err := r.pool.QueryRow(ctx,
		`SELECT product_id, percentage, updated_by, updated_at
		 FROM affiliate_product_overrides
		 WHERE product_id = $1`,
		productID,
	).Scan(&o.ProductID, &o.Percentage, &o.UpdatedBy, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// InsertAffiliateClick persists one click-statistics row and fills ID/CreatedAt.
// The uq_affiliate_clicks_daily index dedupes same-day repeat visitors; the
// returned bool reports whether a new row was stored.
func (r *Repository) InsertAffiliateClick(ctx context.Context, click *model.AffiliateClick) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO affiliate_clicks (affiliator_id, code, ip_hash, user_agent, click_day)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (code, ip_hash, click_day) WHERE ip_hash IS NOT NULL
		 DO NOTHING`,
		click.AffiliatorID, click.Code, click.IPHash, click.UserAgent, click.ClickDay,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// FindAffiliatorByCode returns the affiliator owning a referral code.
// A missing code returns (nil, nil).
func (r *Repository) FindAffiliatorByCode(ctx context.Context, code string) (*model.Affiliator, error) {
	var a model.Affiliator
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, code, status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE code = $1`,
		code,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level, &a.LevelLockedUntil,
		&a.WarnQuarters, &a.SuspendedUntil, &a.FrozenUntil, &a.TermsVersion, &a.TermsAcceptedAt,
		&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
