-- Additive change; dropping loses every agreement checkbox captured going forward.
ALTER TABLE product DROP COLUMN IF EXISTS agreement_checkboxes;
