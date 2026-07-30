ALTER TABLE orders ADD COLUMN is_estimate BOOLEAN NOT NULL DEFAULT false;

UPDATE orders SET is_estimate = true WHERE selected_courier IN ('Ongkir Flat', 'Flat');
