ALTER TABLE product_grouping
  ADD COLUMN slug               TEXT NULL,
  ADD COLUMN short_name         TEXT NULL,
  ADD COLUMN scope              TEXT NULL CHECK (scope IN ('national','international')),
  ADD COLUMN subjects           TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN tone               TEXT NOT NULL DEFAULT 'brand' CHECK (tone IN ('brand','ember','teal','gold','sky','berry','navy')),
  ADD COLUMN illustration       TEXT NOT NULL DEFAULT 'trophy' CHECK (illustration IN ('math','science','language','coding','trophy')),
  ADD COLUMN logo_key           TEXT NULL,
  ADD COLUMN official_url       TEXT NULL,
  ADD COLUMN content            JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN is_visible         BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN participant_prefix TEXT NULL CONSTRAINT product_grouping_participant_prefix_format CHECK (participant_prefix ~ '^[A-Z]{2,6}$'),
  ADD CONSTRAINT product_grouping_visible_published CHECK (NOT is_visible OR status = 'published'),
  ADD CONSTRAINT product_grouping_visible_prefix CHECK (NOT is_visible OR participant_prefix IS NOT NULL),
  ADD CONSTRAINT product_grouping_participant_prefix_key UNIQUE (participant_prefix);

-- A name with no letters or digits falls back to 'event'; same-base rows get -2, -3 by age.
UPDATE product_grouping g SET slug = s.slug FROM (
  SELECT id, base || CASE WHEN rn > 1 THEN '-' || rn ELSE '' END AS slug FROM (
    SELECT id, base, row_number() OVER (PARTITION BY base ORDER BY created_at, id) AS rn FROM (
      SELECT id, created_at,
             COALESCE(NULLIF(trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')), ''), 'event') AS base
      FROM product_grouping) b) x) s
WHERE g.id = s.id;

-- A generated suffix can still hit another grouping's literal slug (names "Event", "Event", "Event 2").
UPDATE product_grouping SET slug = slug || '-' || left(replace(id::text, '-', ''), 8)
WHERE slug IN (SELECT slug FROM product_grouping GROUP BY slug HAVING count(*) > 1);

ALTER TABLE product_grouping ALTER COLUMN slug SET NOT NULL, ADD CONSTRAINT product_grouping_slug_key UNIQUE (slug);

UPDATE product_grouping g SET participant_prefix = d.p FROM (
  SELECT id, p, count(*) OVER (PARTITION BY p) AS n FROM (
    SELECT id, left(upper(regexp_replace(name, '[^A-Za-z]', '', 'g')), 4) AS p FROM product_grouping) x) d
WHERE g.id = d.id AND length(d.p) >= 2 AND d.n = 1;

UPDATE exam_registration SET card_key = NULL
WHERE card_key IS NOT NULL AND exam_id IN (
  SELECT pe.exam_id FROM product_exam pe JOIN product p ON p.id = pe.product_id WHERE p.grouping_id IS NOT NULL);

CREATE TABLE product_grouping_schedule (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  grouping_id UUID NOT NULL REFERENCES product_grouping(id) ON DELETE CASCADE,
  label       TEXT NOT NULL,
  label_en    TEXT NULL,
  starts_on   DATE NOT NULL,
  ends_on     DATE NULL CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX product_grouping_schedule_grouping_idx ON product_grouping_schedule (grouping_id, starts_on);

ALTER TABLE product ADD COLUMN IF NOT EXISTS stages    TEXT[]   NOT NULL DEFAULT '{}' CHECK (stages <@ ARRAY['SD','SMP','SMA']::TEXT[]);
ALTER TABLE product ADD COLUMN IF NOT EXISTS grade_min SMALLINT NULL CHECK (grade_min BETWEEN 1 AND 12);
ALTER TABLE product ADD COLUMN IF NOT EXISTS grade_max SMALLINT NULL CHECK (grade_max BETWEEN 1 AND 12 AND (grade_min IS NULL OR grade_max >= grade_min));

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
