UPDATE affiliate_settings SET
    cookie_days = COALESCE(cookie_days, 30),
    binding_months = COALESCE(binding_months, 3),
    digital_wait_days = COALESCE(digital_wait_days, 7),
    physical_wait_days = COALESCE(physical_wait_days, 14),
    min_payout = COALESCE(min_payout, 100000),
    eval_day = COALESCE(eval_day, 8);

ALTER TABLE affiliate_settings
    ALTER COLUMN cookie_days SET DEFAULT 30,
    ALTER COLUMN cookie_days SET NOT NULL,
    ALTER COLUMN binding_months SET DEFAULT 3,
    ALTER COLUMN binding_months SET NOT NULL,
    ALTER COLUMN digital_wait_days SET DEFAULT 7,
    ALTER COLUMN digital_wait_days SET NOT NULL,
    ALTER COLUMN physical_wait_days SET DEFAULT 14,
    ALTER COLUMN physical_wait_days SET NOT NULL,
    ALTER COLUMN min_payout SET DEFAULT 100000,
    ALTER COLUMN min_payout SET NOT NULL,
    ALTER COLUMN eval_day SET DEFAULT 8,
    ALTER COLUMN eval_day SET NOT NULL;

ALTER TABLE promo_code DROP COLUMN IF EXISTS partner;
ALTER TABLE product DROP COLUMN IF EXISTS partner_access;
ALTER TABLE affiliators DROP COLUMN IF EXISTS programs;
DROP INDEX IF EXISTS idx_users_registered_by;
ALTER TABLE users DROP COLUMN IF EXISTS registered_by;
