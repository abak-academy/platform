-- 0045_product_specs.up.sql
-- Free-form product specification rows (publisher, cover type, material, ...)
-- stored as an ordered JSON array so display order survives round-trips. The
-- canonical field list per product type lives in the frontend, not here; the
-- backend only bounds the shape.
--
-- Numbered 0045 deliberately: base is 0036 and unmerged PR #44 holds 0037-0044,
-- so any lower number collides depending on merge order.

ALTER TABLE product ADD COLUMN specs JSONB NOT NULL DEFAULT '[]'::jsonb;
