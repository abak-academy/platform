-- Global human-friendly exam serial (FR-23), distinct from the exam UUID.
CREATE SEQUENCE IF NOT EXISTS exam_number_seq;

ALTER TABLE exam ADD COLUMN IF NOT EXISTS exam_number INT;

-- Backfill existing rows ordered by created_at, id (stable, deterministic); the
-- ORDER BY on the driving subquery forces nextval() to be consumed in that order.
UPDATE exam e
SET exam_number = t.exam_number
FROM (
    SELECT id, nextval('exam_number_seq') AS exam_number
    FROM exam
    WHERE exam_number IS NULL
    ORDER BY created_at, id
) t
WHERE e.id = t.id;

ALTER TABLE exam ALTER COLUMN exam_number SET DEFAULT nextval('exam_number_seq');
ALTER TABLE exam ALTER COLUMN exam_number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_exam_exam_number
    ON exam (exam_number);
