-- FR-18: the corrective UPDATE is not reversible (original qty is not
-- recoverable from a qty=1 row), so down only drops the report table.
DROP TABLE IF EXISTS digital_qty_overcharge_report;
