CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE school
    ADD COLUMN IF NOT EXISTS category TEXT,
    ADD COLUMN IF NOT EXISTS provinsi_id TEXT REFERENCES province(id),
    ADD COLUMN IF NOT EXISTS kota_id TEXT REFERENCES city(id);

CREATE INDEX IF NOT EXISTS idx_school_active_name_trgm
    ON school USING GIN (LOWER(name) gin_trgm_ops)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_school_provinsi_category_name_id
    ON school (provinsi_id, category, name, id)
    WHERE status = 'active';
