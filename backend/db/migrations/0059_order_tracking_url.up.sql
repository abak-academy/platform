-- Biteship's GET /v1/orders/:id returns courier.link, a public tracking page
-- for the shipment. We fetched it on every webhook and threw it away, so the
-- only way to give a student a tracking link was to hand them the raw resi
-- and let them find the courier's own site.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tracking_url TEXT;
