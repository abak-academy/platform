-- Biteship rejects a reused reference_id (error 40002060) and echoes the
-- conflicting booking back, so re-booking a cancelled shipment under the bare
-- order uuid returns the DEAD booking rather than creating a new one. Each
-- attempt needs its own reference: attempt 1 keeps the bare uuid so every
-- booking already live at Biteship still resolves, attempt N>1 uses "<uuid>-N".
--
-- A persisted counter rather than a random suffix: the reference has to stay
-- stable across a retry of the same attempt, or a network timeout on a booking
-- that actually succeeded would dispatch a second courier.
ALTER TABLE orders
  ADD COLUMN shipment_attempt INT NOT NULL DEFAULT 1;
