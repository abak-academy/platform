-- Cached naskah/kunci PDFs belong to one test, never to an exam aggregate.
ALTER TABLE test
  ADD COLUMN question_naskah_key VARCHAR(500),
  ADD COLUMN question_naskah_generated_at TIMESTAMPTZ,
  ADD COLUMN question_kunci_key VARCHAR(500),
  ADD COLUMN question_kunci_generated_at TIMESTAMPTZ,
  ADD COLUMN question_bundle_revision BIGINT NOT NULL DEFAULT 0;
