-- exam_session.certificate_url stores a private-bucket object KEY, not a URL (FR-28).
ALTER TABLE exam_session RENAME COLUMN certificate_url TO certificate_key;
