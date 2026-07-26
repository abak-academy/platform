-- 0042_certificate_design.up.sql
-- Consolidates certificate_template + certificate_background_key + certificate_layout
-- into a single certificate_design JSONB column (Task 8, FR-26/FR-27).

ALTER TABLE exam RENAME COLUMN certificate_layout TO certificate_design;

-- Seed an object for rows where certificate_design is NULL so jsonb_set below has
-- something to set keys on (jsonb_set on a NULL target returns NULL, not an object).
UPDATE exam SET certificate_design = '{}'::jsonb WHERE certificate_design IS NULL;

-- Fold certificate_template into the blob's "template" key.
UPDATE exam
SET certificate_design = jsonb_set(certificate_design, '{template}', to_jsonb(certificate_template));

-- Fold certificate_background_key into the blob's "background_key" key, only when set
-- (leaves the key absent, not null, for exams without a custom background).
UPDATE exam
SET certificate_design = jsonb_set(certificate_design, '{background_key}', to_jsonb(certificate_background_key))
WHERE certificate_background_key IS NOT NULL;

ALTER TABLE exam DROP CONSTRAINT IF EXISTS chk_certificate_template;
ALTER TABLE exam DROP COLUMN certificate_template;
ALTER TABLE exam DROP COLUMN certificate_background_key;
