ALTER TABLE exam
    DROP COLUMN IF EXISTS whatsapp_group_url,
    DROP COLUMN IF EXISTS additional_rules_note,
    DROP COLUMN IF EXISTS additional_rules_url;
