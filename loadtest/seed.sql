\set ON_ERROR_STOP on
\getenv loadtest_password LOADTEST_PASSWORD

SELECT current_database() = :'confirm_db' AS database_confirmed \gset
\if :database_confirmed
\else
  \echo 'refusing seed: confirm_db does not match current_database()'
  \quit 3
\endif

SELECT :'run_id' ~ '^[a-z0-9][a-z0-9_-]{2,30}$' AS run_id_valid \gset
\if :run_id_valid
\else
  \echo 'refusing seed: run_id must match ^[a-z0-9][a-z0-9_-]{2,30}$'
  \quit 3
\endif

SELECT :'user_count'::int BETWEEN 1 AND 10000 AS user_count_valid \gset
\if :user_count_valid
\else
  \echo 'refusing seed: user_count must be between 1 and 10000'
  \quit 3
\endif

SELECT length(:'loadtest_password') >= 8 AS password_valid \gset
\if :password_valid
\else
  \echo 'refusing seed: LOADTEST_PASSWORD must contain at least 8 characters'
  \quit 3
\endif

SELECT EXISTS (
  SELECT 1
  FROM exam
  WHERE id = :'exam_id'::uuid
) AS exam_exists \gset
\if :exam_exists
\else
  \echo 'refusing seed: exam_id does not exist'
  \quit 3
\endif

BEGIN;

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

CREATE TEMP TABLE loadtest_sessions ON COMMIT DROP AS
SELECT s.id
FROM exam_session s
JOIN exam_registration r ON r.id = s.registration_id
JOIN loadtest_students u ON u.id = r.student_id
WHERE r.exam_id = :'exam_id'::uuid;

DELETE FROM session_violation_log
WHERE session_id IN (SELECT id FROM loadtest_sessions);

DELETE FROM outbox
WHERE aggregate_id IN (SELECT id FROM loadtest_sessions);

DELETE FROM exam_session
WHERE id IN (SELECT id FROM loadtest_sessions);

DELETE FROM exam_registration r
USING loadtest_students u
WHERE r.student_id = u.id
  AND r.exam_id = :'exam_id'::uuid;

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
  format('lt-%s-%s', :'run_id', lpad(u.n::text, 5, '0')),
  'registered',
  participant_base.value + u.n
FROM loadtest_students u
CROSS JOIN participant_base;

COMMIT;

SELECT
  :'run_id' AS run_id,
  :'exam_id' AS exam_id,
  :'user_count'::int AS users_seeded;
