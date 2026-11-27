ALTER TABLE users
    ADD COLUMN IF NOT EXISTS avatar_index smallint CHECK (avatar_index BETWEEN 0 AND 34);
