-- Snapshot of the agreement checkboxes agreed to at checkout time, so a later
-- edit to product.agreement_checkboxes never rewrites a past order's record.
ALTER TABLE order_item ADD COLUMN agreement_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb;
