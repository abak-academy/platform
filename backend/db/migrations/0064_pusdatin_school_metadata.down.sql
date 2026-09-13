ALTER TABLE school
    DROP COLUMN IF EXISTS kota_id,
    DROP COLUMN IF EXISTS provinsi_id,
    DROP COLUMN IF EXISTS category;
