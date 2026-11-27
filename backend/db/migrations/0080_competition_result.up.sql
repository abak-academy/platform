DROP TABLE IF EXISTS exam_external_result;
DROP TABLE IF EXISTS exam_external_round;

CREATE TABLE competition_result (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id    UUID NOT NULL UNIQUE REFERENCES product(id) ON DELETE CASCADE,
  source_key    TEXT NULL,
  published_at  TIMESTAMPTZ NULL,
  updated_by    UUID NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE competition_result_row (
  result_id        UUID NOT NULL REFERENCES competition_result(id) ON DELETE CASCADE,
  registration_id  UUID NOT NULL REFERENCES exam_registration(id) ON DELETE CASCADE,
  score            NUMERIC(8,2) NOT NULL,
  rank             INT NULL CHECK (rank >= 1),
  award            TEXT NULL CHECK (char_length(award) <= 60),
  certificate_url  TEXT NULL CHECK (certificate_url IS NULL OR certificate_url LIKE 'https://%'),
  PRIMARY KEY (result_id, registration_id)
);
CREATE INDEX competition_result_row_registration_idx ON competition_result_row (registration_id);
