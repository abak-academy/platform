DROP INDEX IF EXISTS idx_school_city_category_name_id;
DROP INDEX IF EXISTS idx_school_active_name_trgm;

ALTER TABLE school
    DROP COLUMN IF EXISTS city_id,
    DROP COLUMN IF EXISTS category;
