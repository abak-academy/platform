-- FR-18: intentionally a no-op. This migration only corrects data, and the
-- correction is not reversible — the original qty cannot be recovered from a
-- qty = 1 row. There is no schema change to undo.
SELECT 1;
