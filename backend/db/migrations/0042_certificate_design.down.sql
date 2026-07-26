-- 0042_certificate_design.down.sql
-- Reverses the certificate_design consolidation: extracts template/background_key
-- back into their own columns, restores the CHECK constraint, and strips the two
-- folded keys back out of the JSON blob before renaming it back to certificate_layout.

ALTER TABLE exam ADD COLUMN certificate_background_key TEXT;
UPDATE exam SET certificate_background_key = certificate_design->>'background_key'
WHERE certificate_design ? 'background_key';

ALTER TABLE exam ADD COLUMN certificate_template TEXT NOT NULL DEFAULT 'classic';
UPDATE exam SET certificate_template = COALESCE(certificate_design->>'template', 'classic');

ALTER TABLE exam DROP CONSTRAINT IF EXISTS chk_certificate_template;
ALTER TABLE exam ADD CONSTRAINT chk_certificate_template
    CHECK (certificate_template IN ('classic', 'modern', 'elegant', 'custom'));

UPDATE exam
SET certificate_design = (certificate_design - 'template' - 'background_key');

ALTER TABLE exam RENAME COLUMN certificate_design TO certificate_layout;
