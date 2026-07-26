DROP INDEX IF EXISTS uq_examregistration_participant;
ALTER TABLE exam_registration DROP COLUMN IF EXISTS participant_number;
