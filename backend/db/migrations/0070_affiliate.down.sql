DROP TRIGGER IF EXISTS trg_affiliate_transactions_guard ON affiliate_transactions;
DROP FUNCTION IF EXISTS affiliate_transactions_guard();

DROP TABLE IF EXISTS affiliate_settings;
DROP TABLE IF EXISTS affiliate_transactions;
DROP TABLE IF EXISTS affiliate_bindings;
DROP TABLE IF EXISTS affiliators;

ALTER TABLE product DROP COLUMN IF EXISTS affiliate_commission;
ALTER TABLE orders DROP COLUMN IF EXISTS affiliate_code;

DELETE FROM users WHERE role = 'affiliator';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin'));
