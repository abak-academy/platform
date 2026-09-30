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
- A manual exam grant that conflicts with a student's in-flight purchase must cancel the *entire* unpaid order, including physical items, and release all reservations; never silently grant the exam and fulfill only the physical portion. Reject checkout/retry of an already-owned exam before a new payment. Show the failed/cancelled order and an actionable reason to the student when they try to pay or view the order; do not add email or an in-app notification feed. Do not cancel the Midtrans Snap session/transaction: an existing link can still take payment outside ABAK. A late success must not reactivate or fulfill any part of the cancelled order and must be durably flagged for manual refund and operator reconciliation.
- The targets that checkout reserves are the targets fulfillment grants even if an administrator later edits product links. Previously placed orders without a snapshot must remain fulfillable.

## Technical contract

- Use a durable Postgres claim per student/entitlement target, with transactionally enforced uniqueness for live claims and an order/item reference. Do not rely on unlocked eligibility reads, Redis locks, or UI checks for correctness. Validate existing access as well as claims.
- Resolve and snapshot digital target links while creating claims in the same checkout transaction as `cart → payment_pending` and physical stock changes. Roll back the full transaction on conflict. Paid orders retain their live claim until access is revoked/refunded; claims are not just short-lived payment locks.
- Synchronize claim release with guarded order lifecycle transitions (expiry, cancellation, refund), and guard payment/webhook and async fulfillment against late transitions. Do not broaden physical inventory/shipping rules beyond necessary atomicity.
- Existing active/pending orders need a safe migration/backfill strategy before the new uniqueness guarantee can be asserted for historical transactions; detect conflicts instead of silently discarding them. Only add migration files and local test fixtures here; no shared/live data migration or deployment without explicit authorization.
- Business decisions live in service, persistence/constraints in repository/DB, HTTP conflict mapping in handler. Preserve authz by deriving the recipient from the authenticated student/order, never accepting a recipient from the request.

## Acceptance and proof

1. Existing ownership or an earlier pending/paid purchase rejects same-SKU, different-SKU, and any-target-of-bundle purchases; focused DB-backed service/handler tests must fail without this guard.
2. Concurrent separate student carts/checkouts for the same target cannot both reserve/pay; assert one committed claim and no gateway request for the loser.
3. Duplicate targets among cart lines reject at mutation or checkout before payment, including mixed physical+digital rollback; existing physical-only tests stay green.
4. Expiry/cancellation/refund/revocation allow legitimate repurchase; expired-order retry is rejected and delayed webhook/outbox events cannot bypass the invariant. Exercise free orders and product-link edits between checkout and fulfillment.
5. A manual exam grant conflicting with a pending mixed order cancels the whole order in ABAK, restores physical stock, releases all claims, and shows the failure reason in the student's order/payment flow. Existing Midtrans links are not revoked; a late settlement never fulfills the cancelled order and is recorded for manual refund. Exercise grant-versus-checkout and grant-versus-settlement races with DB-backed tests.
6. Targeted Go tests and repository backend pipeline pass, or blockers are reported accurately. Frontend typecheck, tests, build, and relevant browser behavior are verified if the UI changes.

The original issue included a school marketplace and exam bulk ordering. Those acceptance criteria moved to #192; #167 alone does not prove cross-flow protection against school/bulk checkouts.

## Scope

- In scope: student cart/checkout, reservation persistence and lifecycle, payment retry/webhook safety, fulfillment snapshot, focused client conflict handling, migration files and tests.
- Out of scope: admin-school marketplace, participant pricing/fulfillment, changing existing exam bulk behavior, shipping redesign, partial bundle repricing, and running a live migration/deployment.
- Residual risks: existing orders may contain duplicate active targets and require an audited rollout; gateway settlement arriving after release may require a manual refund because `PaymentClient` has no refund operation. #192 must adapt school/bulk paths to the same claim invariant before cross-flow safety can be claimed.

## Rollout and operator audit (migration 0068)

Facts the rollout plan rests on: `RunMigrations` runs at API startup (`backend/cmd/api/main.go`), takes no advisory lock, and applies each migration file in one transaction. `deploy/compose/production.yml` runs 2 API replicas plus a worker, and a plain `up -d` recreates containers without a documented guarantee that every old replica stops first, so old and new API versions may serve traffic at the same time. The migration is additive: the previous binary keeps running against it, but it neither creates claims nor releases them.

### What the migration does to existing data

- Creates `order_digital_target`, `order_digital_target_backfill_conflict` and `orders.digital_targets_snapshot_at`; no existing row is updated or deleted.
- Backfills live claims only for student-buyer orders in `payment_pending` or `paid` that have digital lines and no `order_participant` rows. A `paid` order wins a contested target over an unpaid one; ties go to the earliest placed order, then order id, then item id. Every loser is written to `order_digital_target_backfill_conflict` with the winning order id. Nothing is refunded, cancelled or auto-resolved.
- Backfilled orders get no snapshot marker, so the worker keeps fulfilling them from the product's live links.
- Fulfilled orders (`processing`, `shipped`, `completed`) are not claimed; their live `course_session` / `exam_registration` already blocks a repurchase.

### Preconditions

1. **Drain old API replicas before 0068 can serve new traffic.** After 0068 commits, no old-version API replica may serve `POST /orders/:id/checkout`. An old replica's checkout creates a `payment_pending` order with no claim, which the uniqueness guarantee cannot see. The backfill does not run again. Stop the old `api` containers (or take checkout offline at nginx) before starting the first new API replica, and keep them stopped. `up -d` alone does not guarantee this.
2. **Upgrade the worker no later than the API.** An old worker ignores the snapshot and grants from live links, which loses the link-edit protection for orders placed by new replicas. It does not break fulfilment. A new worker handles orders without a snapshot by the fallback, so upgrading it first is safe.
3. **Expect one transient startup failure if two new API replicas start together.** The second replica's migration blocks on the first, then fails (on the claim index if the backfill had rows, otherwise on the `schema_migrations` key) and it exits; after a restart it sees the migration recorded and continues. The backfill insert deliberately has no `ON CONFLICT DO NOTHING`, so a claim created by a new-version checkout in between is never skipped silently.
4. **Run the pre-deploy audit below and review contested targets before the deploy window.** No shared-database migration or data operation is performed by this repository's tests or by these documents.
5. **Rollback** to the previous image is schema-safe (0068 is additive), but checkouts served by the old binary create no claims and refunds by the old binary release none. Repeat audit query 3 after any rollback and after rolling forward again, because the backfill does not re-run. The `.down.sql` drops the audit table and claims; do not run it once orders depend on them.

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

### Residual risk

- Until the old replicas are drained, duplicate purchases from an old replica are possible; the audit detects them but nothing repairs them automatically.
- The order of a rolling replace across replicas is decided by the operator, not by the migration. If the drain in precondition 1 cannot be guaranteed, a maintenance window for checkout is the only safe choice, and that is a deploy decision for the owners rather than a code change.
