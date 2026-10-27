-- Physical wait becomes one parameter: received + wait is folded into wait.
-- Level rows fall back to the global value for whichever half they left NULL.
UPDATE affiliate_level_settings
SET physical_wait_days =
        COALESCE(physical_received_days, (SELECT physical_received_days FROM affiliate_settings WHERE id = 1), 7)
      + COALESCE(physical_wait_days, (SELECT physical_wait_days FROM affiliate_settings WHERE id = 1), 7)
WHERE physical_received_days IS NOT NULL OR physical_wait_days IS NOT NULL;

UPDATE affiliate_settings SET physical_wait_days = physical_received_days + physical_wait_days;

ALTER TABLE affiliate_level_settings DROP COLUMN physical_received_days;
ALTER TABLE affiliate_settings DROP COLUMN physical_received_days;
ALTER TABLE affiliate_settings ALTER COLUMN physical_wait_days SET DEFAULT 14;

-- Commission rules are percent or fixed (rupiah per unit); the commission row
-- snapshots the rule that produced it.
ALTER TABLE affiliate_commissions
    ADD COLUMN quantity         INT NOT NULL DEFAULT 1,
    ADD COLUMN commission_type  TEXT NOT NULL DEFAULT 'percent' CHECK (commission_type IN ('percent', 'fixed')),
    ADD COLUMN commission_value NUMERIC(12, 2);

UPDATE affiliate_commissions SET commission_value = percentage;
UPDATE affiliate_commissions c SET quantity = oi.qty FROM order_item oi WHERE oi.id = c.order_item_id;

ALTER TABLE affiliate_commissions ALTER COLUMN commission_value SET NOT NULL;
ALTER TABLE affiliate_commissions DROP COLUMN percentage;

ALTER TABLE affiliate_product_overrides
    ADD COLUMN type TEXT NOT NULL DEFAULT 'percent' CHECK (type IN ('percent', 'fixed'));
ALTER TABLE affiliate_product_overrides RENAME COLUMN percentage TO value;
ALTER TABLE affiliate_product_overrides DROP CONSTRAINT IF EXISTS affiliate_product_overrides_percentage_check;
ALTER TABLE affiliate_product_overrides ALTER COLUMN value TYPE NUMERIC(12, 2);
ALTER TABLE affiliate_product_overrides
    ADD CONSTRAINT affiliate_product_overrides_value_check
    CHECK (value >= 0 AND (type <> 'percent' OR value <= 100));

ALTER TABLE affiliate_level_settings
    ADD COLUMN commission_rule JSONB,
    ADD COLUMN binding_months  INT;

UPDATE affiliate_level_settings
SET commission_rule = (
    SELECT jsonb_object_agg(e.key, jsonb_build_object('type', 'percent', 'value', e.value))
    FROM jsonb_each(commission_pct) AS e
)
WHERE commission_pct IS NOT NULL AND jsonb_typeof(commission_pct) = 'object';

ALTER TABLE affiliate_level_settings DROP COLUMN commission_pct;

-- Backstop for the per-quarter rerun guard in the level evaluator.
CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_level_history_auto_quarter
    ON affiliate_level_history (affiliator_id, quarter)
    WHERE source = 'auto';
