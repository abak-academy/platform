\set ON_ERROR_STOP on
\getenv loadtest_password LOADTEST_PASSWORD

BEGIN;

SELECT
  set_config('loadtest.confirm_db', :'confirm_db', true) AS confirm_db_setting,
  set_config('loadtest.run_id', :'run_id', true) AS run_id_setting,
  set_config('loadtest.user_count', :'user_count', true) AS user_count_setting,
  set_config('loadtest.password', :'loadtest_password', true) AS password_setting,
  set_config('loadtest.exam_id', :'exam_id', true) AS exam_id_setting
\gset

DO $loadtest_guard$
DECLARE
  requested_run_id TEXT := current_setting('loadtest.run_id');
  requested_user_count TEXT := current_setting('loadtest.user_count');
  requested_exam_id TEXT := current_setting('loadtest.exam_id');
  requested_exam_uuid UUID;
BEGIN
  IF current_database() <> current_setting('loadtest.confirm_db') THEN
    RAISE EXCEPTION 'refusing seed: confirm_db does not match current_database()';
  END IF;
  IF requested_run_id !~ '^[a-z0-9][a-z0-9_-]{2,30}$' THEN
    RAISE EXCEPTION 'refusing seed: run_id must match ^[a-z0-9][a-z0-9_-]{2,30}$';
  END IF;
  IF requested_user_count !~ '^[0-9]+$'
     OR requested_user_count::numeric NOT BETWEEN 1 AND 10000 THEN
    RAISE EXCEPTION 'refusing seed: user_count must be an integer between 1 and 10000';
  END IF;
  IF length(current_setting('loadtest.password')) < 8 THEN
    RAISE EXCEPTION 'refusing seed: LOADTEST_PASSWORD must contain at least 8 characters';
  END IF;
  BEGIN
    requested_exam_uuid := requested_exam_id::uuid;
  EXCEPTION WHEN invalid_text_representation THEN
    RAISE EXCEPTION 'refusing seed: exam_id must be a valid UUID';
  END;
  IF NOT EXISTS (SELECT 1 FROM exam WHERE id = requested_exam_uuid) THEN
    RAISE EXCEPTION 'refusing seed: exam_id does not exist';
  END IF;
END
$loadtest_guard$;

CREATE TEMP TABLE loadtest_run_students ON COMMIT DROP AS
SELECT u.id, u.username
FROM users u
WHERE u.username LIKE
  ('lt\_' || replace(:'run_id', '_', '\_') || '\_%') ESCAPE '\';

CREATE TEMP TABLE loadtest_sessions ON COMMIT DROP AS
SELECT s.id
FROM exam_session s
JOIN exam_registration r ON r.id = s.registration_id
JOIN loadtest_run_students u ON u.id = r.student_id
WHERE r.exam_id = :'exam_id'::uuid;

DELETE FROM session_violation_log
WHERE session_id IN (SELECT id FROM loadtest_sessions);

DELETE FROM outbox
WHERE aggregate_id IN (SELECT id FROM loadtest_sessions);

DELETE FROM exam_session
WHERE id IN (SELECT id FROM loadtest_sessions);

DELETE FROM exam_registration r
USING loadtest_run_students u
WHERE r.student_id = u.id
  AND r.exam_id = :'exam_id'::uuid;

DELETE FROM users u
USING loadtest_run_students previous
WHERE u.id = previous.id
  AND NOT EXISTS (
    SELECT 1
    FROM generate_series(1, :'user_count'::int) AS desired(n)
    WHERE u.username = format('lt_%s_%s', :'run_id', lpad(desired.n::text, 5, '0'))
  )
  AND NOT EXISTS (
    SELECT 1
    FROM exam_registration r
    WHERE r.student_id = u.id
  );

WITH password AS MATERIALIZED (
  SELECT crypt(:'loadtest_password', gen_salt('bf', 10)) AS hash
)
INSERT INTO users (username, password_hash, role, name, status, otp_enabled, jenjang)
SELECT
  format('lt_%s_%s', :'run_id', lpad(n::text, 5, '0')),
  password.hash,
  'student',
  format('Load Test Student %s', lpad(n::text, 5, '0')),
  'active',
  false,
  'sma'
FROM generate_series(1, :'user_count'::int) AS n
CROSS JOIN password
ON CONFLICT (username) WHERE username IS NOT NULL DO UPDATE
SET password_hash = EXCLUDED.password_hash,
    status = 'active',
    updated_at = now();

CREATE TEMP TABLE loadtest_students ON COMMIT DROP AS
SELECT u.id, generated.n
FROM generate_series(1, :'user_count'::int) AS generated(n)
JOIN users u
  ON u.username = format('lt_%s_%s', :'run_id', lpad(generated.n::text, 5, '0'));

LOCK TABLE exam_registration IN SHARE ROW EXCLUSIVE MODE;

WITH participant_base AS (
  SELECT COALESCE(MAX(participant_number), 0) AS value
  FROM exam_registration
  WHERE exam_id = :'exam_id'::uuid
)
INSERT INTO exam_registration (
  student_id,
  exam_id,
  token,
  status,
  participant_number
)
SELECT
  u.id,
  :'exam_id'::uuid,
  format('lt-%s-%s-%s', :'run_id', :'exam_id', lpad(u.n::text, 5, '0')),
  'registered',
  participant_base.value + u.n
FROM loadtest_students u
CROSS JOIN participant_base;

COMMIT;

SELECT
  :'run_id' AS run_id,
  :'exam_id' AS exam_id,
  :'user_count'::int AS users_seeded;
