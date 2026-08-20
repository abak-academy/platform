-- Blank emails must be NULL so they do not occupy idx_users_email_active.
-- Empty string is NOT NULL, so a second single-register with no email hit 23505.
UPDATE users SET email = NULL
WHERE email IS NOT NULL AND btrim(email) = '';

DROP INDEX IF EXISTS idx_users_email_active;

CREATE UNIQUE INDEX idx_users_email_active
    ON users (email)
    WHERE email IS NOT NULL AND btrim(email) <> '' AND status != 'deleted';
