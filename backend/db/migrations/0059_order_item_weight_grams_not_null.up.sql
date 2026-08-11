-- 0009_schema_fixes backfilled the sibling `jumlah` column but missed this
-- one, so weight_grams has been nullable-with-no-default since. Every insert
-- path already writes an explicit int (model.OrderItem.WeightGrams has no nil
-- state), so backfilling existing NULLs to 0 and locking the column down
-- cannot break a future write — it only removes the class of row a NULL-typed
-- *int scan cannot handle.
UPDATE order_item SET weight_grams = 0 WHERE weight_grams IS NULL;
ALTER TABLE order_item ALTER COLUMN weight_grams SET DEFAULT 0;
ALTER TABLE order_item ALTER COLUMN weight_grams SET NOT NULL;
