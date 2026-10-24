DROP TRIGGER IF EXISTS trg_affiliate_ledger_append_only ON affiliate_ledger_entries;
DROP FUNCTION IF EXISTS affiliate_ledger_append_only();

DROP TABLE IF EXISTS affiliate_flags;
DROP TABLE IF EXISTS affiliate_clicks;
DROP TABLE IF EXISTS affiliate_product_overrides;
DROP TABLE IF EXISTS affiliate_level_settings;
DROP TABLE IF EXISTS affiliate_settings;
DROP TABLE IF EXISTS affiliate_level_history;
DROP TABLE IF EXISTS affiliate_violations;
DROP TABLE IF EXISTS affiliate_ledger_entries;
DROP TABLE IF EXISTS affiliate_adjustments;
DROP TABLE IF EXISTS affiliate_payouts;
DROP TABLE IF EXISTS affiliate_commissions;
DROP TABLE IF EXISTS affiliate_bindings;
DROP TABLE IF EXISTS affiliators;

DELETE FROM users WHERE role = 'affiliator';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin'));
