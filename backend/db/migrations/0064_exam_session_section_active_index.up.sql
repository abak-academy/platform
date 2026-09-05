CREATE INDEX IF NOT EXISTS idx_examsessionsection_active
    ON exam_session_section (session_id)
    WHERE status = 'active';
