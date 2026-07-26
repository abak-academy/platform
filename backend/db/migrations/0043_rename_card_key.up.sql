-- exam_registration.card_pdf_url stores a private-bucket object KEY, not a URL (FR-29).
ALTER TABLE exam_registration RENAME COLUMN card_pdf_url TO card_key;
