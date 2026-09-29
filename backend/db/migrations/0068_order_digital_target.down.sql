DROP TABLE IF EXISTS order_digital_target_backfill_conflict;
DROP TABLE IF EXISTS order_digital_target;
ALTER TABLE orders DROP COLUMN IF EXISTS digital_targets_snapshot_at;
