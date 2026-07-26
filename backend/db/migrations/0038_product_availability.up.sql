-- Marketplace availability window for products (P-A). NULL = unbounded on that side.
ALTER TABLE product ADD COLUMN IF NOT EXISTS available_from  TIMESTAMPTZ;
ALTER TABLE product ADD COLUMN IF NOT EXISTS available_until TIMESTAMPTZ;
