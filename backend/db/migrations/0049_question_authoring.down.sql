DROP TABLE IF EXISTS question_statement;
DROP TABLE IF EXISTS question_accepted_answer;

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_format_check;
ALTER TABLE question ADD CONSTRAINT question_format_check
    CHECK (format IN ('mcq', 'multi_answer', 'short', 'fill_blank', 'essay', 'multi_blank'));

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_point_wrong_check;
ALTER TABLE question ALTER COLUMN point_wrong TYPE INT USING point_wrong::INT;
ALTER TABLE question ADD CONSTRAINT question_point_wrong_check CHECK (point_wrong >= 0);

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_point_correct_check;
ALTER TABLE question ALTER COLUMN point_correct TYPE INT USING point_correct::INT;
ALTER TABLE question ADD CONSTRAINT question_point_correct_check CHECK (point_correct >= 1);

DROP INDEX IF EXISTS uq_question_question_number;
ALTER TABLE question ALTER COLUMN question_number DROP DEFAULT;
ALTER TABLE question ALTER COLUMN question_number DROP NOT NULL;
ALTER TABLE question DROP COLUMN IF EXISTS question_number;
DROP SEQUENCE IF EXISTS question_number_seq;
