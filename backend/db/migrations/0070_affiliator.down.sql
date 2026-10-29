ALTER TABLE promo_code
    DROP COLUMN IF EXISTS product_ids,
    DROP COLUMN IF EXISTS affiliator_only,
    DROP COLUMN IF EXISTS min_qty,
    DROP COLUMN IF EXISTS starts_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS registered_by,
    DROP COLUMN IF EXISTS school_ids;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin'));
