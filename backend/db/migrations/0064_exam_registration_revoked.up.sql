-- 0064_exam_registration_revoked.up.sql
-- Soft revoke for exam registrations (mirror of course_session's
-- status='revoked' pattern). Rows are never deleted: exam_session rows
-- reference exam_registration.id and participant_number/certificate history
-- must survive a revoke.

ALTER TABLE exam_registration ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
ALTER TABLE exam_registration ADD COLUMN IF NOT EXISTS revoked_by UUID REFERENCES users (id);
