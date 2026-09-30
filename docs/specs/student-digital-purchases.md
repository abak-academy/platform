---
title: Student digital purchase protection
issue: https://github.com/abak-academy/platform/issues/167
status: accepted
created: 2026-09-29
updated: 2026-09-30
sources: ["backend/internal/service/store.go", "backend/internal/repository/order.go", "backend/internal/repository/order_digital.go", "backend/internal/repository/course.go", "backend/internal/repository/exam.go", "backend/internal/worker/outbox.go", "docs/wiki/domain/orders-and-payments.md"]
---

# Student digital purchase protection

## Problem and outcome

A student can be charged for a course/exam entitlement they already own — or are already buying in another order. Two shapes:

- different SKUs, or a bundle, resolve to the same target, and the earlier purchase is still waiting for payment, so it holds nothing anywhere and every eligibility read misses it;
- two checkouts overlap in time.

Fulfillment then silently skips the existing enrollment/registration: money taken, nothing extra delivered. Student self-purchase must reject the conflict before a payment exists, and keep doing so while the earlier order is still alive, while preserving physical purchase behaviour.

## Decisions (Panca, 2026-09-30)

- **No per-order reservation table and no snapshot of resolved targets.** The guard is derived from rows the platform already has: live access, plus live orders joined to the product's links. Nothing new is stored, so there is nothing to backfill and nothing to release.
- **Live links.** A paid order grants what the product links at fulfilment time. Keeping those links stable is the **catalog's** responsibility, tracked in #194 (relations become immutable after create; today an edit can even wipe them).
- The earlier claim-table design is withdrawn: it duplicated a fact the order tables already express, and it dragged in a backfill, a conflict table and release bookkeeping.

## Agreed behaviour

- Digital product types are `course` and `exam`, one per line; physical types `book`, `merchandise`, `medal` keep their stock, quantity, shipping and fulfillment behaviour. Mixed carts are allowed.
- A student's live course enrollment or non-revoked exam registration, **or a live order for any linked target**, makes an entire digital product ineligible. Compare target IDs, not SKU IDs. No partial bundle price or grant.
- A **live order** of the student is one that still owes or holds its targets: every status except `cart`, `cancelled` and `payment_expired` (`payment_pending`, `paid`, `processing`, `shipped`, `completed`). The set is excluded rather than listed so a status added later blocks instead of silently reopening a double purchase. `shipped` and `completed` stay in because a mixed order can be shipped before its `OrderPaid` event grants access, and a completed order's lines may never have been granted. `processing` does not mean fulfilment is pending: `handleOrderPaid` sets it after granting access, for orders that also have physical lines, and `completed` is its digital-only outcome. A refund is `cancelled` with `cancellation_reason = 'refunded'`, so it drops out.
- Cart-add and any item replacement/update that could introduce a digital target give early validation, but checkout is authoritative. **A cart is not a reservation.**
- If any target is ineligible or repeated across cart lines, checkout rejects the whole order before gateway interaction or durable stock decrement. Return a stable, actionable conflict to the client.
- Retry re-runs the guard under the order's row lock before any gateway call, with the order's own lines excluded, so ownership granted since checkout or another live order's hold on the same target refuses it.
- A digital line whose product links no course/exam is refused at checkout and retry rather than charged for nothing.
- Coordinating a later manual exam grant or school/bulk order with an in-flight student purchase is deferred to #192; #167 does not claim protection against those writers. Catalog link rules are #194; the payment window and settlement after the deadline are #196.

## Technical contract

**The guard.** One read inside the checkout transaction, before any gateway call:

```sql
WITH in_flight AS (
    SELECT 'course'::text AS kind, pc.course_id AS target_id
      FROM orders o
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.student_id = $1 AND o.id <> $2
       AND o.status NOT IN ('cart', 'cancelled', 'payment_expired')
    UNION ALL
    SELECT 'exam'::text, pe.exam_id
      FROM orders o
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.student_id = $1 AND o.id <> $2
       AND o.status NOT IN ('cart', 'cancelled', 'payment_expired')
)
SELECT f.kind, f.target_id, 'in_flight' AS reason FROM in_flight f
UNION
SELECT 'course', cs.course_id, 'owned' FROM course_session cs
 WHERE cs.student_id = $1 AND cs.status = 'active'
UNION
SELECT 'exam', er.exam_id, 'owned' FROM exam_registration er
 WHERE er.student_id = $1 AND er.status <> 'revoked';
```

`$1` is the authenticated student (never a request field) and `$2` the order being checked out, so an order's own lines never block themselves — including at retry. The same read without `$2` backs the cart-add and item-update validations. Each result row carries the reason the handler maps to a client conflict.

- **Race safety rests on two invariants; only (a) holds today, and that dependency is load-bearing.** (a) At most one open cart exists per student (`idx_orders_student_cart`, partial unique on `status = 'cart'`), and checkout locks the cart row before validating (`GetOrderByIDForUpdateTx`), so two checkouts of one cart serialize and the loser fails its own status guard; a concurrent `MintCart` waits on the uncommitted cart update (its insert conflicts with the row leaving `cart`) and then receives a new cart, whose add and checkout read the first order's committed state. (b) Cart checkout is the only writer into the live set. **(b) does not hold while `RetryPayment` can reopen a `payment_expired` order.** The interleaving: the student has expired order A for target X; a retry of A passes validation (A's own lines are excluded and an expired A is not live); during the gateway call a checkout of cart B for X also passes, because A is not live yet; the retry then writes `payment_pending` for A, leaving two live orders for X that can both be paid. Retry re-checks under A's row lock, which checkout of B never takes, so nothing serializes them. #197 closes this by making expiry final (a `payment_expired` order is never reopened), after which (b) holds. Until then the read-based guard is race-safe for cart checkouts only; any other path that puts an order into the live set makes it racy and would need a database constraint instead of a read. Only (a) is stated at the validation site in code.
- **Nothing is released, because nothing is held.** The sweeper's `payment_pending → payment_expired` transition drops the order out of the live set, and a refund (`cancelled` with `cancellation_reason = 'refunded'`, there is no `refunded` status) plus the revocation that accompanies it does the same, so the target becomes purchasable again by construction. There is no release path to keep in sync and no orphan state to audit.
- Fulfillment keeps resolving `product_course` / `product_exam` at fulfilment time — deliberate, per the live-links decision. #194 is what makes those links stable; until it lands, an edit can change what a paid order grants.
- Business rules live in service, the query and its locking in repository, HTTP conflict mapping in handler. Derive the recipient from the authenticated student/order, never from the request.
- **No schema change: no new table, no snapshot column, no migration.** Nothing touches historical rows.

## Acceptance and proof

1. Ownership, or a live earlier order, rejects same-SKU, different-SKU and any-target-of-bundle purchases; focused DB-backed tests must fail without the guard.
2. Concurrent checkouts for one student leave at most one live order per target: assert one `payment_pending` order, one gateway call, and a loser rejected without any gateway request.
3. Duplicate targets across cart lines reject at mutation or checkout before payment, including the mixed physical+digital rollback case; existing physical-only tests stay green.
4. An expired, cancelled or refunded order stops blocking its target; revoked course access can be purchased again; a paid, processing, shipped or completed order still blocks.
5. Targeted Go tests and the backend pipeline pass, or blockers are reported accurately. Frontend typecheck, tests, build and the relevant browser behaviour are verified when the UI changes.

## Scope

- In scope: student cart/checkout validation, its interaction with the order lifecycle (retry re-check, settle, confirm and refund under the order row lock), focused client conflict handling, tests, and the wiki.
- Out of scope: catalog relation rules (#194), the payment window and post-deadline settlement handling (#196), manual exam grant versus an in-flight purchase, admin-school marketplace, participant pricing and fulfilment, changing existing bulk behaviour, shipping redesign, partial bundle repricing, and running any migration or deployment.
- Residual risk: a manual grant or school/bulk order created after a student's checkout can still make a paid exam fulfilment a no-op (#192). Until #194 lands, an admin link edit can change what a paid order grants. Until #196 lands, an order whose gateway creation failed has no deadline, is never expired by the sweeper and keeps holding its target until a retry succeeds, and that hold can be permanent: no path cancels a `payment_pending` order, and two twin orders for one target make the digital guard refuse each other's retry. Until #197 lands, the retry race above (invariant (b)) can leave two payable live orders for one target. A settlement that arrives after its order expired or was refunded is logged in `webhook_log` (`payment_success`) and acknowledged, but not flagged as late; that flag is #197.

## Rollout and operator audit (no schema change)

- **No schema change.** No table or column is added, nothing is backfilled and no migration ships.
- **Rolling deploy.** An old replica's checkout still produces a `payment_pending` order, which the new guard reads. But an old replica still runs an unguarded checkout, so while one is serving it can still create a duplicate; the new guard sees the rows those replicas create but cannot prevent the duplicate.

### Operator audit (read-only queries)

Run against the shared database only with explicit authorization; the queries change nothing.

**1. Contested in-flight targets.** Each row is a student with two or more live lines for one course or exam — the damage this issue prevents. Legacy duplicate live orders can still both be paid: the settlement path does not run the guard, so a second `payment_pending` order that predates this change can settle after the first. Those rows need a refund decision, not a code change.

```sql
WITH in_flight AS (
    SELECT o.id AS order_id, o.student_id, 'course'::text AS kind, pc.course_id AS target_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.status NOT IN ('cart', 'cancelled', 'payment_expired')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
    UNION ALL
    SELECT o.id, o.student_id, 'exam'::text, pe.exam_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.status NOT IN ('cart', 'cancelled', 'payment_expired')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
)
SELECT student_id, kind, target_id, count(*) AS live_lines, array_agg(order_id ORDER BY order_id) AS orders
  FROM in_flight
 GROUP BY student_id, kind, target_id
HAVING count(*) > 1
 ORDER BY student_id, kind, target_id;
```

**2. Live orders for a target the student already owns.** A `payment_pending` row will be refused at retry; a `paid` row is a double charge whose fulfilment is a no-op and needs a manual refund decision.

```sql
WITH in_flight AS (
    SELECT o.id AS order_id, o.status, o.student_id, 'course'::text AS kind, pc.course_id AS target_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'course'
      JOIN product_course pc ON pc.product_id = oi.product_id
     WHERE o.status NOT IN ('cart', 'cancelled', 'payment_expired')
       AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
    UNION ALL
    SELECT o.id, o.status, o.student_id, 'exam'::text, pe.exam_id
      FROM orders o
      JOIN users u ON u.id = o.student_id AND u.role = 'student'
      JOIN order_item oi ON oi.order_id = o.id AND oi.product_type = 'exam'
      JOIN product_exam pe ON pe.product_id = oi.product_id
     WHERE o.status NOT IN ('cart', 'cancelled', 'payment_expired')
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
