-- Question authoring schema (E2 Parts A & B, NFR-1: everything in one migration).

-- (a) question_number: global human-friendly serial (FR-1/FR-2), modelled on
-- 0039_exam_number.up.sql. question has no created_at column, so the backfill
-- orders by id — deterministic, just not chronological.
CREATE SEQUENCE IF NOT EXISTS question_number_seq;

ALTER TABLE question ADD COLUMN IF NOT EXISTS question_number INT;

UPDATE question q
SET question_number = t.question_number
FROM (
    SELECT id, nextval('question_number_seq') AS question_number
    FROM question
    WHERE question_number IS NULL
    ORDER BY id
) t
WHERE q.id = t.id;

ALTER TABLE question ALTER COLUMN question_number SET DEFAULT nextval('question_number_seq');
ALTER TABLE question ALTER COLUMN question_number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_question_question_number
    ON question (question_number);

-- (b) point_correct / point_wrong become fractional; the floor for point_correct
-- moves from >= 1 to > 0 (zero-point questions are still disallowed).
ALTER TABLE question DROP CONSTRAINT IF EXISTS question_point_correct_check;
ALTER TABLE question ALTER COLUMN point_correct TYPE NUMERIC;
ALTER TABLE question ADD CONSTRAINT question_point_correct_check CHECK (point_correct > 0);

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_point_wrong_check;
ALTER TABLE question ALTER COLUMN point_wrong TYPE NUMERIC;
ALTER TABLE question ADD CONSTRAINT question_point_wrong_check CHECK (point_wrong >= 0);

-- (c) widen question.format CHECK to admit 'true_false' as a seventh format.
ALTER TABLE question DROP CONSTRAINT IF EXISTS question_format_check;
ALTER TABLE question ADD CONSTRAINT question_format_check
    CHECK (format IN ('mcq', 'multi_answer', 'short', 'fill_blank', 'essay', 'multi_blank', 'true_false'));

-- (d) question_accepted_answer: question-level (blank_index = 0) and per-blank
-- accepted answers, backfilled from the columns they replace.
CREATE TABLE IF NOT EXISTS question_accepted_answer (
    question_id  UUID NOT NULL REFERENCES question (id) ON DELETE CASCADE,
    blank_index  INT  NOT NULL,
    answer_index INT  NOT NULL,
    answer       TEXT NOT NULL,
    PRIMARY KEY (question_id, blank_index, answer_index)
);

INSERT INTO question_accepted_answer (question_id, blank_index, answer_index, answer)
SELECT id, 0, 1, correct_answer
FROM question
WHERE correct_answer IS NOT NULL;

INSERT INTO question_accepted_answer (question_id, blank_index, answer_index, answer)
SELECT question_id, blank_index, 1, correct_answer
FROM question_blank;

-- (e) question_statement: ordered true/false statements for the new format.
CREATE TABLE IF NOT EXISTS question_statement (
    question_id     UUID    NOT NULL REFERENCES question (id) ON DELETE CASCADE,
    statement_index INT     NOT NULL,
    body            TEXT    NOT NULL,
    is_true         BOOLEAN NOT NULL,
    PRIMARY KEY (question_id, statement_index)
);
