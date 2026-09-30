-- 0068_order_digital_target.up.sql
-- Durable per-student claim on each course/exam a digital order line grants.
-- A row is both the snapshot fulfilment reads (targets resolved at checkout)
-- and the reservation: the partial unique index lets one live claim exist per
-- (student, target), across SKUs and bundles. released_at frees the target.
-- target_id is polymorphic (course.id or exam.id by target_kind), so no FK.
-- orders.digital_targets_snapshot_at is NULL for orders that predate this
-- migration; fulfilment falls back to the product's live links for them.

ALTER TABLE orders ADD COLUMN IF NOT EXISTS digital_targets_snapshot_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS order_digital_target (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID        NOT NULL REFERENCES orders (id),
    order_item_id  UUID        NOT NULL REFERENCES order_item (id),
    student_id     UUID        NOT NULL REFERENCES users (id),
    target_kind    TEXT        NOT NULL CHECK (target_kind IN ('course', 'exam')),
    target_id      UUID        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at    TIMESTAMPTZ,
    release_reason TEXT,
    CONSTRAINT uq_order_digital_target_item UNIQUE (order_item_id, target_kind, target_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_order_digital_target_live
    ON order_digital_target (student_id, target_kind, target_id)
    WHERE released_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_order_digital_target_order
    ON order_digital_target (order_id);

-- Historical duplicates are recorded here, never dropped: an operator must
-- resolve each row (usually by refunding or cancelling the losing order)
-- before the losing order's targets can be treated as reserved.
CREATE TABLE IF NOT EXISTS order_digital_target_backfill_conflict (
    order_id         UUID        NOT NULL REFERENCES orders (id),
    order_item_id    UUID        NOT NULL REFERENCES order_item (id),
    student_id       UUID        NOT NULL REFERENCES users (id),
    target_kind      TEXT        NOT NULL,
    target_id        UUID        NOT NULL,
    winning_order_id UUID        NOT NULL REFERENCES orders (id),
    detected_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (order_item_id, target_kind, target_id)
);

-- Backfill live claims for in-flight student self-purchases only: unpaid or
-- paid-but-unfulfilled orders. Fulfilled orders are already covered by the
-- live course_session / exam_registration they created, and cancelled or
-- expired orders hold nothing. Bulk orders (order_participant rows) and
-- non-student buyers are left to their own flows. A paid order wins a
-- contested target over an unpaid one, then the earliest placed order (order
-- and item id break ties); the rest go to the conflict table above.
WITH candidate AS (
    SELECT o.id AS order_id, oi.id AS order_item_id, o.student_id,
           'course'::text AS target_kind, pc.course_id AS target_id,
           COALESCE(o.checked_out_at, o.created_at) AS placed_at,
           (o.status = 'paid') AS is_paid
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
    UNION ALL
    SELECT o.id, oi.id, o.student_id,
           'exam'::text, pe.exam_id,
           COALESCE(o.checked_out_at, o.created_at),
           (o.status = 'paid')
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
), ranked AS (
    SELECT candidate.*,
           ROW_NUMBER() OVER w AS rn,
           FIRST_VALUE(order_id) OVER w AS winning_order_id
      FROM candidate
    WINDOW w AS (PARTITION BY student_id, target_kind, target_id
                 ORDER BY is_paid DESC, placed_at, order_id, order_item_id)
), claimed AS (
    INSERT INTO order_digital_target (order_id, order_item_id, student_id, target_kind, target_id)
    SELECT order_id, order_item_id, student_id, target_kind, target_id
      FROM ranked
     WHERE rn = 1
    RETURNING 1
)
INSERT INTO order_digital_target_backfill_conflict
    (order_id, order_item_id, student_id, target_kind, target_id, winning_order_id)
SELECT order_id, order_item_id, student_id, target_kind, target_id, winning_order_id
  FROM ranked
 WHERE rn > 1 AND winning_order_id <> order_id;

-- The sweeper expires only orders that have a deadline, and checkout now always
-- sets one. A student digital order left payment_pending with none predates
-- that (gateway creation never finished), holds the claim backfilled above and
-- would keep it forever. Give exactly those orders one standard 24h window from
-- migration time so they expire and release like any other. Physical-only, bulk
-- (order_participant) and non-student orders are outside this feature.
UPDATE orders o
   SET payment_expires_at = now() + interval '24 hours', updated_at = now()
 WHERE o.status = 'payment_pending'
   AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM users u WHERE u.id = o.student_id AND u.role = 'student')
   AND EXISTS (SELECT 1 FROM order_item oi WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id);
