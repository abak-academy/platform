DROP TABLE IF EXISTS exam_external_result;
DROP TABLE IF EXISTS exam_external_round;

ALTER TABLE product DROP COLUMN IF EXISTS grade_max;
ALTER TABLE product DROP COLUMN IF EXISTS grade_min;
ALTER TABLE product DROP COLUMN IF EXISTS stages;

DROP TABLE IF EXISTS product_grouping_schedule;

ALTER TABLE product_grouping
  DROP CONSTRAINT IF EXISTS product_grouping_visible_prefix,
  DROP CONSTRAINT IF EXISTS product_grouping_visible_published,
  DROP CONSTRAINT IF EXISTS product_grouping_participant_prefix_key,
  DROP CONSTRAINT IF EXISTS product_grouping_slug_key,
  DROP COLUMN IF EXISTS participant_prefix,
  DROP COLUMN IF EXISTS is_visible,
  DROP COLUMN IF EXISTS content,
  DROP COLUMN IF EXISTS official_url,
  DROP COLUMN IF EXISTS logo_key,
  DROP COLUMN IF EXISTS illustration,
  DROP COLUMN IF EXISTS tone,
  DROP COLUMN IF EXISTS subjects,
  DROP COLUMN IF EXISTS scope,
  DROP COLUMN IF EXISTS short_name,
  DROP COLUMN IF EXISTS slug;
