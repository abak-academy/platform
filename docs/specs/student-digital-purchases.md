---
title: Student digital purchase protection
issue: https://github.com/abak-academy/platform/issues/167
status: accepted
created: 2026-09-29
sources: ["backend/internal/service/store.go", "backend/internal/repository/order.go", "backend/internal/worker/outbox.go", "docs/wiki/domain/orders-and-payments.md"]
---

# Student digital purchase protection

## Problem and outcome

A student can be charged for a course/exam entitlement they already own or are buying through another checkout. Different SKUs can point to the same target; fulfillment silently skips an existing enrollment/registration. Student self-purchase must reject conflicts before creating a payment, including concurrent checkouts, while preserving physical purchase behavior.

## Agreed behavior

- Digital product types are `course` and `exam`, each quantity one; physical types `book`, `merchandise`, `medal` keep their existing stock, quantity, shipping, and fulfillment behavior. Mixed carts are allowed.
- A student's live course enrollment or non-revoked exam registration, or a live digital purchase for any linked target, makes an entire digital product ineligible. Compare target IDs, not SKU IDs. No partial bundle price or grant.
- Cart-add and any item replacement/update that could introduce a digital target provide early validation, but checkout is authoritative. A cart is not a reservation.
- If any target is ineligible or repeated across cart lines, checkout rejects the entire order before gateway interaction or durable stock decrement. Return a stable, actionable conflict to the client.
- Checkout atomically reserves each `(student, target kind, target ID)` for digital lines; a paid/fulfilled purchase keeps its claim while access remains live. Cancelled, expired, or refunded orders release reservations, with refund revocation atomic with release. Revoked/refunded course access can be bought again.
- `payment_expired` is final: the worker sets this order status on its next five-minute sweep after `payment_expires_at`; there is no additional grace window. Retry is allowed only for a still-valid `payment_pending` order and must not reopen an expired order. A new attempt starts from a fresh cart and checkout. Late success for an expired/cancelled/refunded order must not resurrect it or fulfill entitlements; record/flag it for reconciliation or manual refund. Free checkouts follow the same uniqueness rules.
- If the student already has access from any source when they reach checkout, reject their entire mixed order before requesting payment and show an actionable conflict. Coordinating a *later* manual exam grant or bulk exam order with an already-pending student purchase is deferred to the cross-flow follow-up; #167 must not claim protection against those writers.
- The targets that checkout reserves are the targets fulfillment grants even if an administrator later edits product links. Previously placed orders without a snapshot must remain fulfillable.

## Technical contract

- Use a durable Postgres claim per student/entitlement target, with transactionally enforced uniqueness for live claims and an order/item reference. Do not rely on unlocked eligibility reads, Redis locks, or UI checks for correctness. Validate existing access as well as claims.
- Resolve and snapshot digital target links while creating claims in the same checkout transaction as `cart → payment_pending` and physical stock changes. Roll back the full transaction on conflict. Paid orders retain their live claim until access is revoked/refunded; claims are not just short-lived payment locks.
- The gateway payment session is pinned to the order's own deadline: checkout reads the `payment_expires_at` the database set and retry reuses the original, and the Midtrans adapter sends an explicit `expiry.start_time` (session creation time) with whole minutes ending no later than that deadline. Snap otherwise starts the clock when the customer charges, so a link opened late would produce a virtual account outliving the order. With under a minute left, checkout and retry refuse (`payment_expired`) and make no gateway call.
- Synchronize claim release with guarded order lifecycle transitions (expiry, cancellation, refund), and guard payment/webhook and async fulfillment against late transitions. Do not broaden physical inventory/shipping rules beyond necessary atomicity.
- Existing active/pending orders need a safe migration/backfill strategy before the new uniqueness guarantee can be asserted for historical transactions; detect conflicts instead of silently discarding them. Only add migration files and local test fixtures here; no shared/live data migration or deployment without explicit authorization.
- Business decisions live in service, persistence/constraints in repository/DB, HTTP conflict mapping in handler. Preserve authz by deriving the recipient from the authenticated student/order, never accepting a recipient from the request.

## Acceptance and proof

1. Existing ownership or an earlier pending/paid purchase rejects same-SKU, different-SKU, and any-target-of-bundle purchases; focused DB-backed service/handler tests must fail without this guard.
2. Concurrent separate student carts/checkouts for the same target cannot both reserve/pay; assert one committed claim and no gateway request for the loser.
3. Duplicate targets among cart lines reject at mutation or checkout before payment, including mixed physical+digital rollback; existing physical-only tests stay green.
4. Expiry/refund/revocation allow legitimate repurchase; expired-order retry is rejected and delayed webhook/outbox events cannot bypass the invariant. Exercise free orders and product-link edits between checkout and fulfillment.
5. Targeted Go tests and repository backend pipeline pass, or blockers are reported accurately. Frontend typecheck, tests, build, and relevant browser behavior are verified if the UI changes.

The original issue included a school marketplace and exam bulk ordering. Those acceptance criteria moved to #192; #167 alone does not prove cross-flow protection against school/bulk checkouts.

## Scope

- In scope: student cart/checkout, reservation persistence and lifecycle, payment retry/webhook safety, fulfillment snapshot, focused client conflict handling, migration files and tests.
- Out of scope: manual exam grant versus an in-flight purchase, admin-school marketplace, participant pricing/fulfillment, changing existing exam bulk behavior, shipping redesign, partial bundle repricing, and running a live migration/deployment.
- Product decision (Panca): late settlements get no grace. A settlement that arrives after `payment_expires_at` never fulfils; it is flagged for a manual refund. The trade-off is that a student whose payment notification is delayed past the deadline is refunded rather than served. The gateway session is asked to end no later than the deadline (see the technical contract), so late settlements should come only from notification delay or clock skew; this repository cannot prove the gateway enforces the session end.
- Residual risks: existing orders may contain duplicate active targets and require an audited rollout; gateway settlement arriving after release may require a manual refund because `PaymentClient` has no refund operation. A manual grant after checkout but before payment can still make the paid exam fulfillment a no-op, and school/bulk flows can race with student purchases. Both need coordination in the cross-flow follow-up before cross-flow safety can be claimed.

## Rollout and operator audit (migration 0068)

Facts the rollout plan rests on: `RunMigrations` runs at API startup (`backend/cmd/api/main.go`), takes no advisory lock, and applies each migration file in one transaction. `deploy/compose/production.yml` runs 2 API replicas plus a worker, and a plain `up -d` recreates containers without a documented guarantee that every old replica stops first, so old and new API versions may serve traffic at the same time. The migration is additive: the previous binary keeps running against it, but it neither creates claims nor releases them.

### What the migration does to existing data

- Creates `order_digital_target`, `order_digital_target_backfill_conflict` and `orders.digital_targets_snapshot_at`; no existing row is deleted, and the only existing rows updated are the deadline-less orders described in the last bullet.
- Backfills live claims only for student-buyer orders in `payment_pending` or `paid` that have digital lines and no `order_participant` rows. A `paid` order wins a contested target over an unpaid one; ties go to the earliest placed order, then order id, then item id. Every loser is written to `order_digital_target_backfill_conflict` with the winning order id. Two lines of one order reaching the same target are not a conflict — that order's first claim stands and no row is written for the second line. Nothing is refunded, cancelled or auto-resolved.
- Backfilled orders get no snapshot marker, so the worker keeps fulfilling them from the product's live links. Because of that, a retry for such an order checks every live-linked target (excluding only the order's own claims), not just the ones the backfill claimed: a bundle that lost one target to another order's claim is refused at retry instead of being charged.
- Fulfilled orders (`processing`, `shipped`, `completed`) are not claimed; their live `course_session` / `exam_registration` already blocks a repurchase.
- Bounds student self-purchase orders that are `payment_pending` with `payment_expires_at IS NULL`, have at least one `course` or `exam` line and no `order_participant` rows: `payment_expires_at` becomes migration time plus 24 hours (the checkout window), so the worker expires them and releases their claims and stock like any other order. Such an order predates deadline-at-checkout (gateway creation never finished) and has no payment link, so nothing a student holds is shortened. The window starts at migration time rather than at placement so a student is not expired before they can retry; the order can still be retried until then, and a retry never moves the deadline. Mixed physical+digital student orders are included because they hold claims (their stock returns on expiry); the backfill's losing orders are included too, since they are unpaid and hold nothing. Physical-only, bulk/school (`order_participant`) and non-student orders, and orders in any other status, are not touched. Before this change a deadline-less order was never expired by the sweeper, which requires a deadline; that was a defect, not a documented behaviour.

### Preconditions

1. **Drain old API replicas before 0068 can serve new traffic.** After 0068 commits, no old-version API replica may serve `POST /orders/:id/checkout`. An old replica's checkout creates a `payment_pending` order with no claim, which the uniqueness guarantee cannot see. The backfill does not run again. Stop the old `api` containers (or take checkout offline at nginx) before starting the first new API replica, and keep them stopped. `up -d` alone does not guarantee this.
2. **Upgrade the worker no later than the API.** An old worker ignores the snapshot and grants from live links, which loses the link-edit protection for orders placed by new replicas. It does not break fulfilment. A new worker handles orders without a snapshot by the fallback, so upgrading it first is safe.
3. **Expect one transient startup failure if two new API replicas start together.** The second replica's migration blocks on the first, then fails (on the claim index if the backfill had rows, otherwise on the `schema_migrations` key) and it exits; after a restart it sees the migration recorded and continues. The backfill insert deliberately has no `ON CONFLICT DO NOTHING`, so a claim created by a new-version checkout in between is never skipped silently.
4. **Run the pre-deploy audit below (queries 1, 4 and 5) and review contested targets, deadline-less orders and already-owned targets before the deploy window.** No shared-database migration or data operation is performed by this repository's tests or by these documents.
5. **Rollback** to the previous image is schema-safe (0068 is additive), but checkouts served by the old binary create no claims and refunds by the old binary release none. Repeat audit queries 3 and 4 after any rollback and after rolling forward again, because neither the backfill nor the deadline bound re-runs; an old replica's checkout whose gateway call fails leaves a deadline-less order again. The `.down.sql` drops the audit table and claims; do not run it once orders depend on them.

### Operator audit (read-only queries)

Run against the shared database only with explicit authorization; the queries change nothing.

**1. Before deploy: contested in-flight targets.** Each row is a student with two or more in-flight lines for one course or exam. Every row will produce at least one conflict-table row.

```sql
WITH in_flight AS (
    SELECT o.id AS order_id, o.student_id, 'course'::text AS kind, pc.course_id AS target_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
    UNION ALL
    SELECT o.id, o.student_id, 'exam'::text, pe.exam_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
)
SELECT student_id, kind, target_id, count(*) AS in_flight_lines, array_agg(order_id ORDER BY order_id) AS orders
  FROM in_flight
 GROUP BY student_id, kind, target_id
HAVING count(*) > 1
 ORDER BY student_id, kind, target_id;
```

**2. After deploy: backfill conflicts to resolve.** Each row is an in-flight order that lost a contested target. Decide per row (for example refund or cancel the loser through the existing admin flow); the losing order holds no claim, so its refund releases nothing.

```sql
SELECT * FROM order_digital_target_backfill_conflict ORDER BY detected_at, order_id;
```

**3. After deploy, after old replicas are gone, and after any rollback: in-flight digital orders with no live claim.** Expected rows are the backfill losers from query 2. Any other row was checked out by an old-version replica (or by a product with no linked targets) and needs the same manual review.

```sql
SELECT o.id AS order_id, o.status, o.student_id, oi.id AS order_item_id, oi.product_id, oi.product_type
  FROM orders o
  JOIN users u ON u.id = o.student_id AND u.role = 'student'
  JOIN order_item oi ON oi.order_id = o.id AND oi.product_type IN ('course', 'exam')
 WHERE o.status IN ('payment_pending', 'paid')
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
   AND NOT EXISTS (SELECT 1 FROM order_digital_target t
                    WHERE t.order_item_id = oi.id AND t.released_at IS NULL)
 ORDER BY o.id, oi.id;
```

**4. Before and after deploy: student digital orders with no payment deadline.** Before deploy, each row is an order 0068 will bound to 24 hours from migration time. After deploy, any row was created by an old-version replica (or a failed old checkout) and never expires until it is retried or bounded by hand.

```sql
SELECT o.id AS order_id, o.status, o.student_id, o.created_at, o.checked_out_at, o.gateway_ref
  FROM orders o
  JOIN users u ON u.id = o.student_id AND u.role = 'student'
 WHERE o.status = 'payment_pending'
   AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM order_item oi WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
 ORDER BY o.created_at, o.id;
```

**5. Before deploy: in-flight orders for a target the student already owns.** A `payment_pending` row will be refused at retry; a `paid` row is a double charge whose fulfilment is a no-op and needs a manual refund decision.

```sql
WITH in_flight AS (
    SELECT o.id AS order_id, o.status, o.student_id, 'course'::text AS kind, pc.course_id AS target_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
    UNION ALL
    SELECT o.id, o.status, o.student_id, 'exam'::text, pe.exam_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.status IN ('payment_pending', 'paid')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
)
SELECT f.order_id, f.status, f.student_id, f.kind, f.target_id
  FROM in_flight f
 WHERE (f.kind = 'course' AND EXISTS (SELECT 1 FROM course_session cs
            WHERE cs.student_id = f.student_id AND cs.course_id = f.target_id AND cs.status = 'active'))
    OR (f.kind = 'exam' AND EXISTS (SELECT 1 FROM exam_registration er
            WHERE er.student_id = f.student_id AND er.exam_id = f.target_id AND er.status <> 'revoked'))
 ORDER BY f.status, f.order_id, f.kind, f.target_id;
```

### Late-payment reconciliation (read-only query)

A settlement for an order that is expired, cancelled or refunded is flagged, never fulfilled, as a `webhook_log` row with `event_type = 'payment_late'` and the order id in `gateway_ref`. `PaymentClient` has no refund operation, so an operator refunds by hand in the Midtrans dashboard using the payment reference. One flag exists per order and payment: a webhook flags under the gateway's transaction id; an admin reconcile cannot learn that id and flags under the order's gateway reference, is skipped when the order already has a flag, and is attributed to the transaction id in place when a webhook later names it. Distinct transaction ids stay separate rows.

```sql
SELECT o.id AS order_id, o.status AS order_status, o.student_id, o.total,
       o.gateway_ref AS order_gateway_ref,
       w.payload->>'reference' AS payment_reference,
       w.payload->>'source' AS flagged_by,
       w.payload->'notification'->>'payment_type' AS payment_type,
       w.payload->'notification'->>'gross_amount' AS gross_amount,
       w.created_at AS flagged_at,
       EXISTS (SELECT 1 FROM webhook_log s
                WHERE s.event_type = 'payment_success' AND s.gateway_ref = w.gateway_ref
                  AND s.payload->>'transaction_id' = w.payload->>'reference') AS fulfilled_before
  FROM webhook_log w
  JOIN orders o ON o.id::text = w.gateway_ref
 WHERE w.event_type = 'payment_late'
 ORDER BY w.created_at, o.id;
```

Read a row as follows. `fulfilled_before = true` on a `cancelled` order is a redelivery of a payment that was already fulfilled and then refunded through the admin flow: nothing to do. Otherwise the customer paid for an order that will not be fulfilled: refund `payment_reference` in the gateway. A `payment_reference` equal to `order_gateway_ref` came from a reconcile and has no transaction id; find the transaction in the gateway by that order id. The table keeps no "resolved" state, so track handled rows outside the database.

### Residual risk

- Until the old replicas are drained, duplicate purchases from an old replica are possible; the audit detects them but nothing repairs them automatically.
- A late-payment flag from an admin reconcile carries no transaction id. If a gateway order could ever settle twice under different transaction ids, only the webhook-named ones would be separate rows; the reconcile-only flag would be absorbed into the first. Midtrans order ids are meant to be unique per transaction, so this is believed not to occur, but the repository cannot prove it.
- The order of a rolling replace across replicas is decided by the operator, not by the migration. If the drain in precondition 1 cannot be guaranteed, a maintenance window for checkout is the only safe choice, and that is a deploy decision for the owners rather than a code change.
