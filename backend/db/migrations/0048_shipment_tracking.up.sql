ALTER TABLE orders
  ADD COLUMN biteship_order_id TEXT,
  ADD COLUMN shipment_status TEXT,
  ADD COLUMN waybill_source TEXT,
  ADD COLUMN courier_code TEXT,
  ADD COLUMN courier_service_code TEXT;

CREATE TABLE order_shipment_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status              TEXT NOT NULL,
    courier_waybill_id  TEXT,
    courier_driver_name TEXT,
    occurred_at         TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, status, occurred_at)
);
