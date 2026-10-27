ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin', 'affiliator'));

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS school_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS registered_by UUID REFERENCES users (id);

ALTER TABLE promo_code
    ADD COLUMN IF NOT EXISTS starts_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS min_qty INT,
    ADD COLUMN IF NOT EXISTS affiliator_only BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS product_ids UUID[] NOT NULL DEFAULT '{}';
