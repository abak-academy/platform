DROP TABLE IF EXISTS competition_result_row;
DROP TABLE IF EXISTS competition_result;

CREATE TABLE exam_external_round (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  exam_id       UUID NOT NULL REFERENCES exam(id) ON DELETE CASCADE,
  label         TEXT NOT NULL,
  label_en      TEXT NULL,
  source_key    TEXT NULL,
  published_at  TIMESTAMPTZ NULL,
  updated_by    UUID NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (exam_id, label)
);
CREATE TABLE exam_external_result (
  round_id         UUID NOT NULL REFERENCES exam_external_round(id) ON DELETE CASCADE,
  registration_id  UUID NOT NULL REFERENCES exam_registration(id) ON DELETE CASCADE,
  score            NUMERIC(8,2) NOT NULL,
  rank             INT NULL CHECK (rank >= 1),
  award            TEXT NULL CHECK (char_length(award) <= 60),
  certificate_url  TEXT NULL CHECK (certificate_url IS NULL OR certificate_url LIKE 'https://%'),
  PRIMARY KEY (round_id, registration_id)
);
CREATE INDEX exam_external_result_registration_idx ON exam_external_result (registration_id);
