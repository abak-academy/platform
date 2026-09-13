DROP INDEX IF EXISTS idx_school_provinsi_name_id;
DROP INDEX IF EXISTS idx_school_active_name_trgm;
DROP INDEX IF EXISTS idx_school_active_npsn;

ALTER TABLE school
    DROP COLUMN IF EXISTS kota_id,
    DROP COLUMN IF EXISTS provinsi_id,
    DROP COLUMN IF EXISTS category;
