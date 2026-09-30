-- Checkout sets a payment deadline now, and the sweeper only expires orders
-- that have one: a student digital order left payment_pending without a
-- deadline never expires and would hold its course/exam forever. Bound those.

UPDATE orders o SET payment_expires_at = now() + interval '24 hours'
 WHERE o.status = 'payment_pending' AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM order_item oi
                WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND EXISTS (SELECT 1 FROM users u WHERE u.id = o.student_id AND u.role = 'student')
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id);
