-- Per-exam participant number for exam registrations (FUP-1).
ALTER TABLE exam_registration ADD COLUMN IF NOT EXISTS participant_number INT;

-- Backfill existing rows: a stable per-exam sequence ordered by registration time.
WITH numbered AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY exam_id ORDER BY created_at, id) AS seq
    FROM exam_registration
    WHERE participant_number IS NULL
)
UPDATE exam_registration r
SET participant_number = n.seq
FROM numbered n
WHERE r.id = n.id;

-- One participant number per exam. NULLs remain distinct, but every insert
-- assigns a value under a per-exam advisory lock (see CreateExamRegistration).
CREATE UNIQUE INDEX IF NOT EXISTS uq_examregistration_participant
    ON exam_registration (exam_id, participant_number);
