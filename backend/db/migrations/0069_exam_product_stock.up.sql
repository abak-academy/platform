ALTER TABLE product ALTER COLUMN stock DROP NOT NULL;

-- Existing exam products must stay unlimited; only exams may hold NULL.
UPDATE product SET stock = NULL WHERE type = 'exam';
