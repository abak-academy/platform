ALTER TABLE exam_registration
    ADD COLUMN IF NOT EXISTS device_fingerprint TEXT;
