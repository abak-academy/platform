-- B-7: exam/course are one-per-account digital products, so any historical
-- qty > 1 row was an overcharge. The rule itself is enforced in code —
-- ValidateItemQty runs in AddToCart, UpdateItemQty and (as of this branch)
-- Checkout — so this migration only cleans up rows that predate the guard.
--
-- Only carts are corrected. An order past cart is the record of money actually
-- taken; rewriting qty there would falsify what was charged and break
-- qty * unit_price = jumlah. Those rows are left exactly as they are, which
-- also means they stay queryable afterwards — the overcharged buyers can be
-- listed at any time with:
--
--   SELECT o.id, o.student_id, oi.name, oi.qty, oi.unit_price,
--          oi.unit_price * (oi.qty - 1) AS overcharge, o.status, o.paid_at
--   FROM order_item oi
--   JOIN orders o ON o.id = oi.order_id
--   WHERE oi.product_type IN ('exam', 'course')
--     AND oi.qty > 1
--     AND o.status <> 'cart'
--   ORDER BY overcharge DESC;
--
-- No report table: the only rows this migration destroys are carts, which were
-- never charged, so freezing them would preserve the evidence nobody needs.
-- Whether the buyers above get money back is an open decision — see issue #72.
--
-- The promo on a corrected cart was validated against the inflated subtotal, so
-- its discount cannot survive: a percentage discount would stay sized for a
-- subtotal that no longer exists, and a fixed discount larger than the corrected
-- subtotal drives total negative — a figure checkout would hand straight to the
-- payment gateway. Detaching beats re-deriving it here, because promo validity
-- (window, max_uses, min_order_amount, max_discount_amount) lives in
-- ValidatePromo and a second implementation would make money authoritative in
-- two places. These are carts, so the buyer simply re-applies the code.
-- used_count is untouched: it is only incremented at checkout.
-- One statement, because every part of it sees the same snapshot: a
-- data-modifying CTE's effects are NOT visible to the rest of the statement,
-- so the new subtotal has to be derived arithmetically (the CASE below) rather
-- than re-summed after the fact. Re-summing here would read the pre-correction
-- rows and silently leave the inflated subtotal in place.
WITH victims AS (
    SELECT oi.id AS item_id, oi.order_id
    FROM order_item oi
    JOIN orders o ON o.id = oi.order_id
    WHERE oi.product_type IN ('exam', 'course')
      AND oi.qty > 1
      AND o.status = 'cart'
),
recomputed AS (
    SELECT oi.order_id,
           SUM(CASE WHEN v.item_id IS NULL THEN oi.jumlah ELSE oi.unit_price END) AS new_subtotal
    FROM order_item oi
    LEFT JOIN victims v ON v.item_id = oi.id
    WHERE oi.order_id IN (SELECT order_id FROM victims)
    GROUP BY oi.order_id
),
fixed AS (
    UPDATE order_item oi
    SET qty    = 1,
        jumlah = oi.unit_price
    FROM victims v
    WHERE oi.id = v.item_id
    RETURNING 1
)
UPDATE orders o
SET subtotal      = r.new_subtotal,
    discount      = 0,
    promo_code_id = NULL,
    total         = r.new_subtotal + o.shipping_cost,
    updated_at    = now()
FROM recomputed r
WHERE o.id = r.order_id;
