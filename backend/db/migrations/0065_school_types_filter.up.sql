DROP INDEX IF EXISTS idx_school_provinsi_kota_category_name_id;
ALTER TABLE school DROP COLUMN IF EXISTS category;

CREATE INDEX IF NOT EXISTS idx_school_provinsi_kota_name_id
    ON school (provinsi_id, kota_id, name, id)
    WHERE status = 'active';
