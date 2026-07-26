-- 0035_certificate_design.up.sql
-- Certificate design fields (background image, layout, staleness timestamp) on exam,
-- plus certificate number allocation on exam_session. See spec.md OQ2/C3/FR-9..FR-14.

ALTER TABLE exam ADD COLUMN certificate_background_key TEXT;
ALTER TABLE exam ADD COLUMN certificate_layout JSONB;
ALTER TABLE exam ADD COLUMN certificate_design_updated_at TIMESTAMPTZ;

-- NFR-4: certificate_template widens to admit 'custom' alongside the built-ins;
-- existing classic|modern|elegant values keep their meaning.
ALTER TABLE exam DROP CONSTRAINT IF EXISTS chk_certificate_template;
ALTER TABLE exam ADD CONSTRAINT chk_certificate_template
    CHECK (certificate_template IN ('classic', 'modern', 'elegant', 'custom'));

ALTER TABLE exam_session ADD COLUMN certificate_number TEXT;

CREATE UNIQUE INDEX idx_exam_session_certificate_number
    ON exam_session(certificate_number) WHERE certificate_number IS NOT NULL;

CREATE SEQUENCE IF NOT EXISTS certificate_number_seq;
