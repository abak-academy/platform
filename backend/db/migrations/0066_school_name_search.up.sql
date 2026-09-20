CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_school_active_name_trgm
    ON school USING gin (name gin_trgm_ops)
    WHERE status = 'active';
