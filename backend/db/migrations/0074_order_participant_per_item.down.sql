DROP INDEX IF EXISTS idx_order_participant_order_id;

ALTER TABLE order_participant DROP CONSTRAINT order_participant_pkey;
ALTER TABLE order_participant ADD PRIMARY KEY (order_id, student_id);

ALTER TABLE order_participant DROP COLUMN order_item_id;
