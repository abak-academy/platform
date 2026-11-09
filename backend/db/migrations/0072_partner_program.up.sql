ALTER TABLE users ADD COLUMN IF NOT EXISTS registered_by UUID REFERENCES users (id);

CREATE INDEX IF NOT EXISTS idx_users_registered_by
    ON users (registered_by) WHERE registered_by IS NOT NULL;

ALTER TABLE affiliators ADD COLUMN IF NOT EXISTS programs JSONB;

ALTER TABLE product ADD COLUMN IF NOT EXISTS partner_access JSONB;

ALTER TABLE promo_code ADD COLUMN IF NOT EXISTS partner JSONB;

ALTER TABLE affiliate_settings
    ALTER COLUMN cookie_days DROP NOT NULL,
    ALTER COLUMN cookie_days DROP DEFAULT,
    ALTER COLUMN binding_months DROP NOT NULL,
    ALTER COLUMN binding_months DROP DEFAULT,
    ALTER COLUMN digital_wait_days DROP NOT NULL,
    ALTER COLUMN digital_wait_days DROP DEFAULT,
    ALTER COLUMN physical_wait_days DROP NOT NULL,
    ALTER COLUMN physical_wait_days DROP DEFAULT,
    ALTER COLUMN min_payout DROP NOT NULL,
    ALTER COLUMN min_payout DROP DEFAULT,
    ALTER COLUMN eval_day DROP NOT NULL,
    ALTER COLUMN eval_day DROP DEFAULT;
