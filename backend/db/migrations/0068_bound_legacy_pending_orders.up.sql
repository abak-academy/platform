-- 0068_bound_legacy_pending_orders.up.sql
-- Checkout now sets payment_expires_at, and the sweeper expires only orders that
-- have one. A student digital order left payment_pending with none predates that
-- (its gateway creation never finished), never expires, and would hold its
-- course/exam against a new purchase forever. Give exactly those orders one
-- standard 24h window from migration time so they expire like any other.
-- Physical-only, bulk (order_participant) and non-student orders are untouched.

UPDATE orders o SET payment_expires_at = now() + interval '24 hours'
 WHERE o.status = 'payment_pending' AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM order_item oi
                WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND EXISTS (SELECT 1 FROM users u WHERE u.id = o.student_id AND u.role = 'student')
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id);
