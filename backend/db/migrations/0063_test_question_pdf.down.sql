ALTER TABLE test
  DROP COLUMN IF EXISTS question_bundle_revision,
  DROP COLUMN IF EXISTS question_naskah_key,
  DROP COLUMN IF EXISTS question_naskah_generated_at,
  DROP COLUMN IF EXISTS question_kunci_key,
  DROP COLUMN IF EXISTS question_kunci_generated_at;
