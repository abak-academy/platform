-- Per-product checkout agreement checkboxes (title + body), stored as an
-- ordered JSON array, same shape as product.specs (0045).
ALTER TABLE product ADD COLUMN agreement_checkboxes JSONB NOT NULL DEFAULT '[]'::jsonb;
