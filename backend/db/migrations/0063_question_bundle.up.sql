-- Question bundles for async PDF generation of test/exam question packets
CREATE TABLE question_bundle (
  id UUID PRIMARY KEY,
  exam_id UUID REFERENCES exam(id) ON DELETE CASCADE,
  test_id UUID REFERENCES test(id) ON DELETE CASCADE,
  variant VARCHAR(10) NOT NULL CHECK(variant IN ('naskah', 'kunci')),
  status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK(status IN ('queued', 'processing', 'ready', 'failed')),
  object_key VARCHAR(500),
  error TEXT,
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  generated_at TIMESTAMPTZ,
  CONSTRAINT exactly_one_scope CHECK(
    (exam_id IS NOT NULL AND test_id IS NULL) OR
    (exam_id IS NULL AND test_id IS NOT NULL)
  )
);

CREATE INDEX idx_question_bundle_created_by ON question_bundle(created_by, created_at DESC);
CREATE INDEX idx_question_bundle_exam_id ON question_bundle(exam_id, created_at DESC);
CREATE INDEX idx_question_bundle_test_id ON question_bundle(test_id, created_at DESC);
