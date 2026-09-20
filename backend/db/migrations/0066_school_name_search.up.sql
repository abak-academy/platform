CREATE EXTENSION IF NOT EXISTS pg_trgm;

DROP INDEX IF EXISTS idx_school_active_name_trgm;

CREATE INDEX idx_school_active_name_trgm
    ON school USING gin (name gin_trgm_ops)
    WHERE status = 'active';
