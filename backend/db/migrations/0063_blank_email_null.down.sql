DROP INDEX IF EXISTS idx_users_email_active;

CREATE UNIQUE INDEX idx_users_email_active
    ON users (email)
    WHERE email IS NOT NULL AND status != 'deleted';
