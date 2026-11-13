DROP INDEX IF EXISTS idx_product_grouping_id;
ALTER TABLE product DROP COLUMN IF EXISTS grouping_order;
ALTER TABLE product DROP COLUMN IF EXISTS grouping_id;
DROP TABLE IF EXISTS product_grouping;
