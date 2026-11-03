ALTER TABLE exam
    ADD COLUMN IF NOT EXISTS whatsapp_group_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS additional_rules_note TEXT NULL,
    ADD COLUMN IF NOT EXISTS additional_rules_url TEXT NULL;
