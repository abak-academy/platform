CREATE TABLE testimonial (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           TEXT NOT NULL,
  role_label     TEXT NOT NULL,
  photo_key      TEXT NULL,
  content        JSONB NOT NULL DEFAULT '{}',
  has_consent    BOOLEAN NOT NULL DEFAULT false,
  consent_note   TEXT NULL,
  consent_by     UUID NULL REFERENCES users(id),
  consent_at     TIMESTAMPTZ NULL,
  is_visible     BOOLEAN NOT NULL DEFAULT false,
  sort_order     INT NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (NOT is_visible OR (has_consent AND consent_note IS NOT NULL)),
  CHECK (NOT is_visible OR photo_key IS NOT NULL)
);
CREATE INDEX testimonial_visible_idx ON testimonial (sort_order) WHERE is_visible;
