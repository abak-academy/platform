-- 0045_product_specs.down.sql
-- Additive change; dropping loses every specification captured going forward.
ALTER TABLE product DROP COLUMN IF EXISTS specs;
