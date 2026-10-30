package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
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
		        launch_date, eval_day, daily_registration_cap, updated_at
		 FROM affiliate_settings
		 WHERE id = 1`,
	).Scan(&s.ID, &s.CookieDays, &s.BindingMonths, &s.MinExtendAmount, &s.DigitalWaitDays,
		&s.PhysicalWaitDays, &s.MinPayout, &s.PayoutDayOfMonth,
		&s.LaunchDate, &s.EvalDay, &s.DailyRegistrationCap, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// affiliateLevelRule is the JSON shape of one level inside
// affiliate_settings.levels; the level name is the object key.
type affiliateLevelRule struct {
	CommissionRule   json.RawMessage `json:"commission_rule,omitempty"`
	BindingMonths    *int            `json:"binding_months,omitempty"`
	DigitalWaitDays  *int            `json:"digital_wait_days,omitempty"`
	PhysicalWaitDays *int            `json:"physical_wait_days,omitempty"`
	MinPayout        *float64        `json:"min_payout,omitempty"`
	PayoutDayOfMonth *int            `json:"payout_day_of_month,omitempty"`
	RiseThreshold    *float64        `json:"rise_threshold,omitempty"`
	HoldThreshold    *float64        `json:"hold_threshold,omitempty"`
}

// LoadAffiliateLevelSettings returns the levels stored in the settings row,
// ordered by level name; a missing row has none.
func (r *Repository) LoadAffiliateLevelSettings(ctx context.Context) ([]model.AffiliateLevelSettings, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT levels FROM affiliate_settings WHERE id = 1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var byLevel map[string]affiliateLevelRule
	if err := json.Unmarshal(raw, &byLevel); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(byLevel))
	for name := range byLevel {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []model.AffiliateLevelSettings
	for _, name := range names {
		rule := byLevel[name]
		out = append(out, model.AffiliateLevelSettings{
			Level: name, CommissionRule: rule.CommissionRule, BindingMonths: rule.BindingMonths,
			DigitalWaitDays: rule.DigitalWaitDays, PhysicalWaitDays: rule.PhysicalWaitDays,
			MinPayout: rule.MinPayout, PayoutDayOfMonth: rule.PayoutDayOfMonth,
			RiseThreshold: rule.RiseThreshold, HoldThreshold: rule.HoldThreshold,
		})
	}
	return out, nil
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

// CountAffiliatorsCreatedSince counts affiliator accounts registered at or after since.
func (r *Repository) CountAffiliatorsCreatedSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'affiliator' AND created_at >= $1`, since).Scan(&n)
	return n, err
}

// affiliateProductOverride is the JSON shape of product.affiliate_commission.
type affiliateProductOverride struct {
	Type      string    `json:"type"`
	Value     float64   `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoadProductOverride returns the commission override stored on one product.
// A product without an override, or an unknown product, returns (nil, nil).
func (r *Repository) LoadProductOverride(ctx context.Context, productID uuid.UUID) (*model.AffiliateProductOverride, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT affiliate_commission FROM product WHERE id = $1`, productID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && len(raw) == 0) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var o affiliateProductOverride
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, err
	}
	return &model.AffiliateProductOverride{ProductID: productID, Type: o.Type, Value: o.Value, UpdatedAt: o.UpdatedAt}, nil
}

// FindAffiliatorByCode returns the affiliator owning a referral code.
// A missing code returns (nil, nil).
func (r *Repository) FindAffiliatorByCode(ctx context.Context, code string) (*model.Affiliator, error) {
	a, err := scanAffiliator(r.pool.QueryRow(ctx,
		`SELECT `+affiliatorColumns+` `+affiliatorFrom+` WHERE a.referral_code = $1`, code))
	if isNotFound(err) {
		return nil, nil
	}
	return a, err
}
