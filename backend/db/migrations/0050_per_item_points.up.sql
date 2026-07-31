-- Per-item point values (user decision 2026-07-31): a blank, statement or
-- correct option may carry its own worth. NULL means "use the question's
-- point_correct", so every existing row keeps today's behaviour with no
-- backfill. The penalty stays question-level (point_wrong) by design.
ALTER TABLE question_blank
    ADD COLUMN IF NOT EXISTS points NUMERIC CHECK (points > 0);
ALTER TABLE question_statement
    ADD COLUMN IF NOT EXISTS points NUMERIC CHECK (points > 0);
ALTER TABLE question_option
    ADD COLUMN IF NOT EXISTS points NUMERIC CHECK (points > 0);
