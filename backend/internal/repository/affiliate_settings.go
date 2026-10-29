package repository

import (
	"context"
	"errors"
	"time"

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
		        physical_wait_days, min_payout, payout_day_of_month,
		        launch_date, eval_day, silver_threshold, gold_threshold,
		        daily_registration_cap, updated_by, updated_at
		 FROM affiliate_settings
		 WHERE id = 1`,
	).Scan(&s.ID, &s.CookieDays, &s.BindingMonths, &s.MinExtendAmount, &s.DigitalWaitDays,
		&s.PhysicalWaitDays, &s.MinPayout, &s.PayoutDayOfMonth,
		&s.LaunchDate, &s.EvalDay, &s.SilverThreshold, &s.GoldThreshold,
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
		`SELECT level, commission_rule, binding_months, digital_wait_days,
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
		if err := rows.Scan(&l.Level, &l.CommissionRule, &l.BindingMonths, &l.DigitalWaitDays,
			&l.PhysicalWaitDays, &l.MinPayout, &l.PayoutDayOfMonth, &l.RiseThreshold, &l.HoldThreshold); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SetAffiliateLaunchDateIfUnset stores a launch date only while none is set,
// creating the singleton row when it does not exist yet.
func (r *Repository) SetAffiliateLaunchDateIfUnset(ctx context.Context, date time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO affiliate_settings (id, launch_date) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET launch_date = EXCLUDED.launch_date, updated_at = now()
		 WHERE affiliate_settings.launch_date IS NULL`,
		date,
	)
	return err
}

// CountAffiliatorsCreatedSince counts affiliator profiles created at or after since.
func (r *Repository) CountAffiliatorsCreatedSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM affiliators WHERE created_at >= $1`, since).Scan(&n)
	return n, err
}

// LoadProductOverride returns the commission override for one product.
// A missing row returns (nil, nil).
func (r *Repository) LoadProductOverride(ctx context.Context, productID uuid.UUID) (*model.AffiliateProductOverride, error) {
	var o model.AffiliateProductOverride
	err := r.pool.QueryRow(ctx,
		`SELECT product_id, type, value, updated_by, updated_at
		 FROM affiliate_product_overrides
		 WHERE product_id = $1`,
		productID,
	).Scan(&o.ProductID, &o.Type, &o.Value, &o.UpdatedBy, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// FindAffiliatorByCode returns the affiliator owning a referral code.
// A missing code returns (nil, nil).
func (r *Repository) FindAffiliatorByCode(ctx context.Context, code string) (*model.Affiliator, error) {
	var a model.Affiliator
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, COALESCE(code, ''), status, rejection_reason, level,
		        warn_quarters, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE code = $1`,
		code,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level,
		&a.WarnQuarters, &a.TermsVersion, &a.TermsAcceptedAt,
		&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
