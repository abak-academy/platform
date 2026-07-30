DROP TABLE order_shipment_events;

ALTER TABLE orders
  DROP COLUMN biteship_order_id,
  DROP COLUMN shipment_status,
  DROP COLUMN waybill_source,
  DROP COLUMN courier_code,
  DROP COLUMN courier_service_code;
