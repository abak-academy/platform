ALTER TABLE exam
  ADD COLUMN card_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN card_notes JSONB NOT NULL DEFAULT '[]'::jsonb;

-- New exams are opt-in, but existing ones already hand out cards; without this
-- backfill every student on a live exam loses card access on deploy.
UPDATE exam SET card_enabled = true;
