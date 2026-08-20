WITH latest_attempt AS (
    SELECT DISTINCT ON (registration_id)
        registration_id,
        status
    FROM exam_session
    ORDER BY registration_id, attempt_number DESC
)
UPDATE exam_registration AS registration
SET status = latest_attempt.status
FROM latest_attempt
WHERE registration.id = latest_attempt.registration_id
  AND latest_attempt.status IN ('submitted', 'in_progress')
  AND registration.status IS DISTINCT FROM latest_attempt.status;
