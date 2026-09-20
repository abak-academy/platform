DROP INDEX IF EXISTS idx_school_provinsi_kota_name_id;
ALTER TABLE school ADD COLUMN IF NOT EXISTS category TEXT;

CREATE INDEX IF NOT EXISTS idx_school_provinsi_kota_category_name_id
    ON school (provinsi_id, kota_id, category, name, id)
    WHERE status = 'active';
