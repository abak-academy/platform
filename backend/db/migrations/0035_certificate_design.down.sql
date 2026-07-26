-- 0035_certificate_design.down.sql

DROP SEQUENCE IF EXISTS certificate_number_seq;

DROP INDEX IF EXISTS idx_exam_session_certificate_number;

ALTER TABLE exam_session DROP COLUMN IF EXISTS certificate_number;

-- Any exam left on 'custom' can't satisfy the narrower constraint below; fall back
-- to 'classic' (same default the column already uses for new rows).
UPDATE exam SET certificate_template = 'classic' WHERE certificate_template = 'custom';

ALTER TABLE exam DROP CONSTRAINT IF EXISTS chk_certificate_template;
ALTER TABLE exam ADD CONSTRAINT chk_certificate_template
    CHECK (certificate_template IN ('classic', 'modern', 'elegant'));

ALTER TABLE exam DROP COLUMN IF EXISTS certificate_design_updated_at;
ALTER TABLE exam DROP COLUMN IF EXISTS certificate_layout;
ALTER TABLE exam DROP COLUMN IF EXISTS certificate_background_key;
