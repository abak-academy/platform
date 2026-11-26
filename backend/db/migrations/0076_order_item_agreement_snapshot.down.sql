-- Additive change; dropping loses every agreement snapshot captured going forward.
ALTER TABLE order_item DROP COLUMN IF EXISTS agreement_snapshot;
