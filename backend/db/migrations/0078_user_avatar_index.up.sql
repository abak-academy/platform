ALTER TABLE users
    ADD COLUMN avatar_index smallint CHECK (avatar_index BETWEEN 0 AND 34);
