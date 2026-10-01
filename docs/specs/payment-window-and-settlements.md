---
title: Payment window and settlement after the deadline
issue: https://github.com/abak-academy/platform/issues/196
status: proposed
created: 2026-09-30
updated: 2026-09-30
sources: ["backend/internal/service/store.go", "backend/internal/repository/order.go", "backend/internal/repository/order_lifecycle.go", "backend/internal/adapter/midtrans.go", "backend/internal/service/ports_payment.go", "backend/internal/worker/outbox.go", "backend/db/migrations/0068_bound_legacy_pending_orders.up.sql", "docs/wiki/domain/orders-and-payments.md"]
---

# Payment window and settlement after the deadline

This spec covers what happens to an order's payment from checkout to and past its deadline. The duplicate-purchase guard that depends on it is `docs/specs/student-digital-purchases.md` (#167).

## Problem and outcome

A `payment_pending` order that has no payment deadline is never expired: the sweeper only selects orders that have one. A checkout whose gateway call failed after the order left `cart` therefore holds its stock, and for a student digital order its course/exam, forever. A retry could also hand out a new gateway link that outlives the order, and a settlement could fulfil an order the sweeper had not yet got to.

Outcome: every pending order has one bounded deadline set at checkout, the gateway session cannot outlive it, nothing settles or attaches a payment after it, and money that still arrives after it is recorded and handed to a human as a refund task instead of disappearing from view.

## Decisions

- **One deadline, set by ABAK (Panca, 2026-09-30).** `orders.payment_expires_at` is written at checkout, in the transaction that moves the cart to `payment_pending`, before any gateway call. A gateway response or a retry never moves it.
- **No grace.** A settlement after the deadline never fulfils the order and never moves it back out of `payment_expired`. `payment_expired` is final; a new attempt starts from a fresh cart.
- **Flag, do not grant.** The `payment_late` flag is an operator signal. It gives no access. Whether the flag is used at all is still open, which is why this spec is `proposed`; this change implements it as a database row with a read-only operator query, and it grants nothing either way.
- Refund stays manual: `PaymentClient` has no refund operation, so an operator refunds in the gateway dashboard using the reference the flag carries.

## Required behaviour

**Window**

- Checkout sets the deadline to 24 hours from checkout on the database clock. The deadline is judged on that same clock everywhere, so a retry, a webhook and the sweeper cannot disagree about whether an order is still payable.
- The worker sets `payment_expired` on its next five-minute sweep after the deadline. Until it does, no path treats the order as payable: every path that could settle or attach a payment re-checks the deadline under the order's row lock.
- Retry is allowed only for a `payment_pending` order inside its deadline. It never reopens an expired order and never moves the deadline; an expired or elapsed order answers `payment_expired` (409) without reaching the gateway. A retry that finds the order changed after the gateway call attaches nothing and offers no link.
- Checkout attaches the gateway reference, and hands out the link, only while the order is still an unexpired `payment_pending` one; otherwise it answers `payment_expired` for an order that expired or whose deadline elapsed during the gateway call and `order_not_editable` for one that was paid or cancelled meanwhile. A checkout replay validates ownership and state before returning a cached reference, so it never hands out a payment reference for another student's order or for one that has since expired or been cancelled. A free-checkout replay is unchanged.
- The gateway payment session is pinned to the order's own deadline: `expiry.start_time` is session creation and the duration is whole minutes to the deadline rounded down, so a link opened late cannot outlive the order. The expiry advertised to the client is the earlier of the session end and the order deadline. With under a minute left, checkout and retry refuse with `payment_deadline_too_close` (409) — the order is still payable, so this is not `payment_expired`. This repository cannot prove the gateway enforces the session end. The under-a-minute check compares the app clock against the database deadline: a host behind the database can pass it and is then refused by the database-clock guard when the reference is attached, and a host ahead refuses up to that skew early. Neither case hands out a link for an order that has expired.
- An order that predates deadline-at-checkout and has none gets one once, at its first payment attachment.

**Settlement at or after the deadline**

- A settlement for a `payment_pending` order past its deadline expires the order under the row lock, with the sweeper's own transition (stock released once), and is then treated as a settlement for an expired order.
- A settlement reported for an order that is `payment_expired` or `cancelled` (a refund is `cancelled` with `cancellation_reason = 'refunded'`) writes a flag with the order id in `gateway_ref` and the reported reference in the payload. No entitlement is written and the order status does not change. The webhook answers 2xx once the flag is written, so the gateway stops redelivering; if the flag cannot be written it answers 5xx and the gateway redelivers.
- A settlement for an order still inside its window behaves as before (fulfilment under the order's row lock).
- Repeated notifications for the same settlement do not duplicate flags, records or refunds.
- The reconcile path follows the same rule as the webhook: it expires an overdue order under the lock and, for an order that can no longer be fulfilled, flags the payment and answers 409 `order_not_payable` without marking it paid. Admin confirm of an order that is not `payment_pending`, or whose deadline has elapsed, answers `order_not_payable` and leaves the transition to the sweeper.
- A flagged order's target stays purchasable: the student can complete a fresh checkout for the same target.

## Technical contract

- `CheckoutOrder` writes `payment_expires_at = now() + window` together with the status change. It does so before the zero-total branch, so a free order that goes straight to `paid` carries a deadline too; it is inert (the sweeper selects only `payment_pending`) and the order page shows it as "Berlaku sampai", as it already does for every paid gateway order. `SetPaymentRefTx` attaches the gateway reference in the caller's transaction only while `status = 'payment_pending'` and the deadline has not passed, keeps an existing deadline, and reports not-attached otherwise.
- The deadline check is `PaymentDeadlineElapsedTx`, on the database clock; `ExpirePaymentOrderTx` carries `status = 'payment_pending' AND payment_expires_at <= now()` in one UPDATE and restores physical stock in the same transaction, so only one caller among webhook, reconcile and sweeper ever sees true and stock is restored once. The service's `bindPaymentDeadline` compares the app clock against the database deadline, so host clock skew shifts the session end by that much.
- Checkout and retry idempotency keys are scoped to the student and the order (`idempotency:{checkout,retry}:<student>:<order>:<client key>`). That scoping, not the ownership and state check alone, is what makes a reused or colliding client key unable to return another order's, or another student's, payment reference; the state check then keeps a replay from handing out a reference for an order that has since expired or been cancelled. A request replayed across the deploy that introduced the scoping finds no cached entry and is answered as a fresh request (`order_not_editable` for a checkout whose cart already left `cart`).
- Flags are `webhook_log` rows with `event_type = 'payment_late'`, the order id in `gateway_ref`, and the reported reference in the payload. `audit_log` cannot carry them: it requires an actor and a webhook has none.
- One flag per order and payment. A webhook names the gateway's transaction id; an admin reconcile cannot learn it and flags under the order's own gateway reference. A reference equal to the order's own reference is skipped when the order already has a flag, and is attributed to the transaction id in place when a webhook later names it. Distinct transaction ids stay distinct rows.
- The caller's row lock on the order serializes concurrent writers, so a redelivered webhook and a reconcile cannot both insert. The writer runs inside the caller's transaction; failure to record must not turn a settled payment into a fulfilled order.
- No schema change for the flag: `webhook_log` and its index already exist. The one migration is data-only (see rollout).

## Acceptance and proof

1. Checkout gives every pending order a bounded deadline, including when the gateway call then fails, and the sweeper frees the stock.
2. A retry never extends the deadline, is refused with under a minute left and for an expired or elapsed order, and an order that expires during the gateway call gets no payment link.
3. The gateway request ends at the order's deadline for checkout and for retry, and the advertised expiry never exceeds it.
4. A settlement after the deadline, before the sweeper has run, expires the order once and fulfils nothing, including under concurrent webhook and sweeper.
5. Settlement after the deadline: order stays `payment_expired`, no course/exam access is written, one `payment_late` flag is recorded with the reference. A settlement just before the deadline still fulfils.
6. Duplicate notifications, and a reconcile after a webhook, are idempotent — one flag per order and payment.
7. A flagged `payment_expired` order cannot be refunded through the admin path because the app holds no payment to reverse — the operator refunds the flagged reference in the gateway. A flag on an order that was paid and then refunded (`cancelled`, `cancellation_reason = 'refunded'`, `paid_at` set) may be a redelivery of the refunded payment or a second payment; the operator checks the gateway before refunding anything. No access is revoked that was never granted, and the target stays purchasable after the flag.
8. Targeted Go tests pass, including the race-detector run the backend pipeline uses.

## Scope

- In scope: the deadline at checkout, the retry window rules, gateway session pinning, the too-close refusal, expiry and refusal at the deadline in the webhook, reconcile and confirm paths, the flag writer, migration 0068, their tests and the wiki.
- Out of scope: the duplicate-purchase guard itself (#167); automatic refunds or gateway refund support; any change to what a fulfilled order grants.
- Residual risk: this repository cannot prove the gateway enforces the session end. The flag carries no "resolved" state, so handled rows are tracked outside the database; a reconcile-only flag has no transaction id until a webhook names it.

## Rollout and operator audit

- **One data-only migration, no down file.** `0068_bound_legacy_pending_orders` gives a `payment_pending` student digital order with `payment_expires_at IS NULL` — its gateway creation never finished — one standard 24h window from migration time, so it expires like any other. Physical-only, bulk (`order_participant`) and non-student orders are untouched on purpose: they still hold stock with no deadline and are for a human to resolve; query 1b lists them. The runner only reads `*.up.sql`, and restoring `NULL` would bring the defect back, so no down migration ships. This deliberately departs from the Go standard's "paired up/down migrations plus integration coverage" (`docs/standards/go-style-and-tests.md`): the migration runner globs only `migrations/*.up.sql`, so a `.down.sql` would never run, and the owner decided to ship without one. `TestMigration0068_BoundsLegacyStudentDigitalOrders` runs the statement against fixtures:

```sql
UPDATE orders o SET payment_expires_at = now() + interval '24 hours'
 WHERE o.status = 'payment_pending' AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM order_item oi
                WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND EXISTS (SELECT 1 FROM users u WHERE u.id = o.student_id AND u.role = 'student')
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id);
```

The window is set at migration time rather than at placement so no student is expired before they can retry, and a retry never moves it. Before this change a deadline-less order was never expired at all, which was a defect rather than a documented behaviour.

- **Rolling deploy.** While old replicas still serve traffic, expiry is not final. An old replica's checkout that fails at the gateway still leaves an order with no deadline, so a row can appear after the migration ran. Its retry can reopen a `payment_expired` order, its `SetPaymentRef` write to the pool can move the deadline, and its webhook still fulfils a pending order past its deadline. Its sweeper reads the status without a lock and then writes `payment_expired`, so it can overwrite an order a webhook has just marked `paid`: `paid_at` stays set and the worker still grants access for such an order (`handleOrderPaid` also advances `payment_expired`), so the student keeps access while the order reads as expired. Treat an order as settled-as-expired only once every replica runs this version.

Run the queries below against the shared database only with explicit authorization; they change nothing.

**1. Student digital orders with no payment deadline.** Each row is an order the migration bounds; a row appearing after it ran means an old replica or a failed checkout created one and it will not expire until retried or bounded by hand.

```sql
SELECT o.id AS order_id, o.status, o.student_id, o.created_at, o.checked_out_at, o.gateway_ref
  FROM orders o
  JOIN users u ON u.id = o.student_id AND u.role = 'student'
 WHERE o.status = 'payment_pending'
   AND o.payment_expires_at IS NULL
   AND EXISTS (SELECT 1 FROM order_item oi
                WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
   AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id)
 ORDER BY o.created_at, o.id;
```

**1b. Pending orders with no deadline that 0068 left alone.** Physical-only, bulk and non-student orders that still hold stock, because resolving them (cancel and restore stock, or keep) is a human call the migration does not make.

```sql
SELECT o.id AS order_id, o.student_id, u.role AS buyer_role, o.created_at, o.gateway_ref,
       EXISTS (SELECT 1 FROM order_item oi WHERE oi.order_id = o.id
                AND oi.product_type IN ('book', 'merchandise', 'medal')) AS holds_stock,
       EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id) AS is_bulk
  FROM orders o
  JOIN users u ON u.id = o.student_id
 WHERE o.status = 'payment_pending'
   AND o.payment_expires_at IS NULL
   AND NOT (u.role = 'student'
            AND EXISTS (SELECT 1 FROM order_item oi
                         WHERE oi.order_id = o.id AND oi.product_type IN ('course', 'exam'))
            AND NOT EXISTS (SELECT 1 FROM order_participant op WHERE op.order_id = o.id))
 ORDER BY o.created_at, o.id;
```

**2. Late payments to refund.**

```sql
SELECT o.id AS order_id, o.status AS order_status, o.student_id, o.total,
       o.gateway_ref AS order_gateway_ref,
       w.payload->>'reference' AS payment_reference,
       w.payload->>'source' AS flagged_by,
       w.payload->'notification'->>'payment_type' AS payment_type,
       w.payload->'notification'->>'gross_amount' AS gross_amount,
       w.created_at AS flagged_at,
       (COALESCE(o.cancellation_reason, '') = 'refunded' AND o.paid_at IS NOT NULL) AS refunded_after_paid,
       (o.paid_at IS NOT NULL) AS paid_at_set
  FROM webhook_log w
  JOIN orders o ON o.id::text = w.gateway_ref
 WHERE w.event_type = 'payment_late'
 ORDER BY w.created_at, o.id;
```

Read a row as follows. `paid_at_set = true` on an order that is not `cancelled` means the order was marked paid at some point and then read as `payment_expired` (an old replica's sweeper overwriting a settled payment), so access may already have been granted: check the gateway **and** the student's enrollment or registration before refunding, because refunding then would leave the student with access and their money back. `refunded_after_paid` does not cover this, since it needs `cancellation_reason = 'refunded'`. `refunded_after_paid = true` means the order was paid and then refunded through the admin flow, which records only a transfer receipt and moves no money at the gateway. The flag is then either a redelivery of the payment that was already refunded or a second payment: look the order up in the gateway and refund `payment_reference` only if it is a different transaction from the one already refunded. Do not infer this from `webhook_log`: fulfilment through reconcile or admin confirm writes no `payment_success` row, so its absence proves nothing. Otherwise the customer paid for an order that will not be fulfilled: refund `payment_reference` in the gateway. A `payment_reference` equal to `order_gateway_ref` came from a reconcile and has no transaction id; find the transaction in the gateway by that order id. The table keeps no "resolved" state, so track handled rows outside the database.
