DROP INDEX IF EXISTS uq_affiliate_level_history_auto_quarter;

ALTER TABLE affiliate_level_settings ADD COLUMN commission_pct JSONB;

UPDATE affiliate_level_settings
SET commission_pct = (
    SELECT jsonb_object_agg(e.key, e.value -> 'value')
    FROM jsonb_each(commission_rule) AS e
    WHERE e.value ->> 'type' = 'percent'
)
WHERE commission_rule IS NOT NULL;

ALTER TABLE affiliate_level_settings DROP COLUMN binding_months;
ALTER TABLE affiliate_level_settings DROP COLUMN commission_rule;

DELETE FROM affiliate_product_overrides WHERE type = 'fixed';
ALTER TABLE affiliate_product_overrides DROP CONSTRAINT IF EXISTS affiliate_product_overrides_value_check;
ALTER TABLE affiliate_product_overrides RENAME COLUMN value TO percentage;
ALTER TABLE affiliate_product_overrides ALTER COLUMN percentage TYPE NUMERIC(5, 2);
ALTER TABLE affiliate_product_overrides
    ADD CONSTRAINT affiliate_product_overrides_percentage_check CHECK (percentage >= 0 AND percentage <= 100);
ALTER TABLE affiliate_product_overrides DROP COLUMN type;

ALTER TABLE affiliate_commissions ADD COLUMN percentage NUMERIC(5, 2);
UPDATE affiliate_commissions
SET percentage = CASE
    WHEN commission_type = 'percent' THEN commission_value
    WHEN base_amount > 0 THEN LEAST(round(amount / base_amount * 100, 2), 999.99)
    ELSE 0
END;
ALTER TABLE affiliate_commissions ALTER COLUMN percentage SET NOT NULL;
ALTER TABLE affiliate_commissions
    DROP COLUMN commission_value,
    DROP COLUMN commission_type,
    DROP COLUMN quantity;

ALTER TABLE affiliate_settings ALTER COLUMN physical_wait_days SET DEFAULT 7;
ALTER TABLE affiliate_settings ADD COLUMN physical_received_days INT NOT NULL DEFAULT 7;
ALTER TABLE affiliate_level_settings ADD COLUMN physical_received_days INT;
