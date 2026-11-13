-- order_participant becomes item-scoped so a multi-product Mitra order can
-- exclude a student from one product's line while still serving them another.
-- Backfill is unambiguous: this table is only written by partner orders, and
-- every existing partner order has exactly one order_item.
ALTER TABLE order_participant ADD COLUMN order_item_id UUID REFERENCES order_item (id);

UPDATE order_participant op
SET order_item_id = oi.id
FROM order_item oi
WHERE oi.order_id = op.order_id AND op.order_item_id IS NULL;

ALTER TABLE order_participant ALTER COLUMN order_item_id SET NOT NULL;

ALTER TABLE order_participant DROP CONSTRAINT order_participant_pkey;
ALTER TABLE order_participant ADD PRIMARY KEY (order_item_id, student_id);

CREATE INDEX IF NOT EXISTS idx_order_participant_order_id
    ON order_participant (order_id);
