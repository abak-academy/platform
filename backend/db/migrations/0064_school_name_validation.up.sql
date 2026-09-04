ALTER TABLE school
    ADD CONSTRAINT school_name_meaningful_check CHECK (name ~ '[[:alnum:]]') NOT VALID;

ALTER TABLE school
    VALIDATE CONSTRAINT school_name_meaningful_check;
