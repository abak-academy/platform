-- 0063_exam_session_active_index.up.sql
-- Serves the worker's exam_sessions_active gauge (internal/worker/
-- exam_sessions.go): no existing exam_session index serves submitted_at IS
-- NULL, so the 30s poll was a full seq scan over every session ever taken.
-- The partial index narrows the count to in-progress, unsubmitted sessions —
-- the live-window predicate in the worker query filters the few remaining
-- rows on top of this scan.

CREATE INDEX IF NOT EXISTS idx_examsession_active
    ON exam_session (started_at)
    WHERE status = 'in_progress' AND submitted_at IS NULL;
