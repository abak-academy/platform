-- B-7: exam/course items are one-per-account digital products, so any
-- historical qty > 1 row was an overcharge. Report first, correct second —
-- reversing the order would destroy the evidence of what was actually charged.
CREATE TABLE digital_qty_overcharge_report (
    order_id      UUID NOT NULL,
    student_id    UUID NOT NULL,
    order_item_id UUID NOT NULL,
    product_id    UUID NOT NULL,
    product_name  TEXT NOT NULL,
    qty           INT NOT NULL,
    unit_price    NUMERIC(12, 2) NOT NULL,
    overcharge    NUMERIC(12, 2) NOT NULL,
    order_status  TEXT NOT NULL,
    paid_at       TIMESTAMPTZ,
    reported_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO digital_qty_overcharge_report
    (order_id, student_id, order_item_id, product_id, product_name, qty, unit_price, overcharge, order_status, paid_at)
SELECT o.id, o.student_id, oi.id, oi.product_id, oi.name, oi.qty, oi.unit_price,
       oi.unit_price * (oi.qty - 1), o.status, o.paid_at
FROM order_item oi
JOIN orders o ON o.id = oi.order_id
WHERE oi.product_type IN ('exam', 'course') AND oi.qty > 1;

-- FR-17 restricts the corrective UPDATE to status = 'cart': an order past
-- cart is the historical record of money actually taken, and rewriting qty
-- there would falsify it and break qty * unit_price = jumlah. Paid/other
-- rows are captured above and left alone; refunds are a separate decision.
-- To widen back to the brief's unrestricted UPDATE, drop the
-- "AND o.status = 'cart'" clause below (and in the orders recompute).
UPDATE order_item oi
SET qty    = 1,
    jumlah = oi.unit_price
FROM orders o
WHERE oi.order_id = o.id
  AND oi.product_type IN ('exam', 'course')
  AND oi.qty > 1
  AND o.status = 'cart';

-- The promo on these carts was validated against the inflated subtotal, so its
-- discount cannot be carried across the correction: a percentage discount would
-- stay sized for a subtotal that no longer exists, and a fixed discount larger
-- than the corrected subtotal drives total negative — a figure checkout would
-- hand straight to the payment gateway.
--
-- Detach it instead of trying to re-derive it in SQL: promo validity (window,
-- max_uses, min_order_amount, max_discount_amount) lives in ValidatePromo, and
-- a second implementation here would be the authority on money in two places.
-- These are carts, not paid orders, so the buyer simply re-applies the code
-- against the real subtotal. used_count is untouched because it is only
-- incremented at checkout, which these carts have not reached.
UPDATE orders o
SET subtotal      = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = o.id), 0),
    discount      = 0,
    promo_code_id = NULL,
    total         = COALESCE((SELECT SUM(jumlah) FROM order_item WHERE order_id = o.id), 0) + o.shipping_cost,
    updated_at    = now()
WHERE o.id IN (
    SELECT order_id FROM digital_qty_overcharge_report WHERE order_status = 'cart'
);
