---
title: Orders & Payments
updated: 2026-09-30
verified_against: bf23fb47
---

See also: [../architecture/system-overview.md](../architecture/system-overview.md), [../architecture/shipping-biteship.md](../architecture/shipping-biteship.md), [./exam-lifecycle-and-scoring.md](./exam-lifecycle-and-scoring.md).

## Catalog & product

- `model.Product` (`backend/internal/model/product.go`): `type` ∈ `book|merchandise|medal|course|exam`, `status` ∈ `draft|published|hidden|archived` (constraint `product_status_check`, `backend/db/migrations/0009_schema_fixes.up.sql:6-8`).
- `isPhysicalType(t string) bool` (`backend/internal/service/store.go:2040`) — only `book`, `merchandise`, `medal` count as physical (shipped, stock-tracked). `course`/`exam` are digital, unlimited, no stock.
- `AvailableFrom`/`AvailableUntil` bound marketplace visibility (nullable = unbounded side); `*Set` flags on the struct distinguish "field omitted" from "field cleared" on partial updates.
- RBAC on product mutation is per-type: `checkTypeRBAC` (`store.go:2043`) — `admin_store` may touch price/visibility on any type including `exam`, but never exam content (owned by `admin_exam`).

## Cart

- `MintCart` (`store.go:532`) gets-or-creates the student's single `cart`-status order.
- `AddItem` / `RemoveItem` / `UpdateItemQty` (`store.go:540,615,657`) each re-check `order.Status == "cart"` at the service layer, then the repository (`AddItem`/`RemoveItem`/`UpdateItemQty` in `backend/internal/repository/order.go:610,669,710`) takes `GetOrderByIDForUpdateTx` (a row lock) and re-checks `status = 'cart'` again before writing — see comment at `order.go:592-597`. This closes the TOCTOU window an unlocked service-layer check alone would leave open.
- Digital items (`course`/`exam`) are capped at qty 1 and unique per cart — enforced twice: `ValidateItemQty` (`backend/internal/service/item_qty.go:16`) at the service layer, and a locked re-check inside `AddItem`'s transaction (`ErrDuplicateDigitalItem`) that catches two concurrent `AddItem` calls racing past the first check. Checkout re-validates qty under its own lock (`store.go:1144`), because a row written before those guards existed can still carry qty > 1 on a digital line.
- Adding a digital line for a course/exam the student already owns, or is already buying in another live order, is refused at add time with a `DigitalTargetConflictError` (`checkDigitalCandidate`, `backend/internal/service/store_digital.go:82`). That is an early warning only — a cart is not a reservation — and checkout decides again inside its own transaction. `AddItem` is the only cart mutation that can introduce a digital target: `UpdateItemQty` changes the quantity of an existing line and digital lines are capped at 1.
- Adding/removing a physical item clears `shipping_cost`/`selected_courier`/`selected_service` (`clearShipping`) — the quote goes stale the moment the parcel's weight/contents change.
- `PatchCart` (`store.go:755`) writes the shipping address/courier/promo; `validateAddressHierarchy` (`store.go:728`) enforces province→city→district FK consistency before it's persisted or priced.

## Digital purchases — derived guard

A student cannot buy a course/exam they already own or are already buying, whichever SKU or bundle reaches it. Nothing is stored for this: the guard is a read of rows the platform already has, so there is no reservation table, no snapshot of resolved targets, no backfill and nothing to release. The contract is `docs/specs/student-digital-purchases.md`.

- `ListLiveDigitalTargets` (`backend/internal/repository/order_digital.go:64`) returns the student's live targets with a reason. `in_flight`: a live order of this student (other than `excludeOrderID`) that still owes or holds the target — every status except `cart`, `cancelled` and `payment_expired` — whose `order_item` lines reach it through `product_course`/`product_exam`. `owned`: an `active` `course_session` or a non-`revoked` `exam_registration`. Targets are compared by course/exam id, never SKU id. `excludeOrderID` is how an order never blocks itself — required at retry, where the order is itself `payment_pending`; cart-add passes `uuid.Nil`.
- A live order stops counting by status alone: `payment_expired` and `cancelled` (which is how a refund is stored) drop out. `shipped` and `completed` keep blocking, because a mixed order shipped before fulfilment, or a completed order whose lines were never granted, can still owe access; the set is excluded rather than listed so a status added later blocks instead of reopening a double purchase. Expiry, refund and access revocation therefore make the target purchasable again with no release step to keep in sync.
- The service turns that list into conflicts (`unavailableDigitalTargets`, `store_digital.go:55`): `owned` wins over `in_flight`, and `in_flight` is sent to clients under the wire reason `claimed` so the client mapping is unchanged. Two lines of one order reaching the same target are `duplicate_in_cart` (`repeatedDigitalTargets`, `store_digital.go:40`). A digital line whose product links no course/exam is refused rather than charged for nothing (`ErrDigitalProductNoTargets`, `requireTargetsForDigitalLines`, `store_digital.go:111`).
- Call sites, all on the caller's transaction where there is one (`digitalQueryer`, `order_digital.go:19`; a nil transaction reads on the pool): cart add (`checkDigitalCandidate`, `store.go:598`, no transaction); **checkout** (`checkDigitalTargets`, `store.go:1183`, after the cart row lock, skipped for bulk orders that carry `order_participant` rows) — authoritative; **retry** (`recheckDigitalTargets` via `validateRetryableOrder`, `store.go:1338`). Any conflict rejects the **whole** order before a gateway call or durable stock decrement, so a mixed digital+physical cart keeps its physical stock.
- **Why a read is enough — and why that is load-bearing.** `idx_orders_student_cart` (`backend/db/migrations/0004_commerce.up.sql:58`) allows one open cart per student, and checkout locks that cart row (`GetOrderByIDForUpdateTx`) before validating. Two checkouts for one student therefore cannot run on two different carts: the second waits, then sees the first order committed as `payment_pending` or fails its own `status = 'cart'` guard. If a second concurrent cart ever became possible the read would be racy and a database constraint would be needed instead. The site carries a comment (`store.go:1182`) and `TestOpenCart_AStudentCanHoldOnlyOne` pins the invariant.
- Fulfilment and exam refund resolve `product_course`/`product_exam` at the time they run (`Worker.handleOrderPaid`, `revokeOrderExamRegistrationsTx`): **live links**, deliberately. Keeping those links stable after checkout is the catalog's job (#194); until then an admin link edit can change what a paid order grants.
- API surface: a conflict is HTTP 409 `digital_target_conflict` with a `conflicts` detail list; a digital product with no target is 409 `digital_product_unavailable` (`backend/internal/handler/errors.go:244-251`). The student cart and checkout show the reason instead of opening a payment page.
- Not covered here: a manual exam grant or school/bulk order created after a student's checkout can still make a paid exam fulfilment a no-op (#192). Audit queries for historic damage live in the spec.

## Order status machine

DB constraint (`backend/db/migrations/0013_shipped_status.up.sql:2-4`, current):

```
status IN ('cart', 'payment_pending', 'paid', 'processing', 'shipped', 'completed', 'payment_expired', 'cancelled')
```

There is **no `refunded` status** — a refund is recorded as `status = 'cancelled'` with `cancellation_reason = 'refunded'` (see Refunds below). There is also no separate `checkout` state — checkout is a same-transaction `cart → payment_pending` (or straight to `paid` for free orders) flip, not a persisted intermediate status.

Observed transitions and where each is written:

| From → To | Trigger | Guard on the write |
|---|---|---|
| `cart → payment_pending` | `Checkout` (`store.go:1077`), gateway path | **Guarded**: `GetOrderByIDForUpdateTx` locks the row and re-checks `locked.Status != "cart"` (`store.go:1129`) before `CheckoutOrder` writes the status in the same tx. |
| `cart → paid` | `Checkout`, zero-total/free path (`store.go:1199`) | Same lock as above; the free path skips the gateway and emits `OrderPaid` in the same tx. |
| `payment_pending → paid` | `HandlePaymentWebhook` (`store.go:1922`, Midtrans settlement/capture) | **Guarded**: the order is read under its row lock (`store.go:1967`). A settlement for an order that is then not `payment_pending` is logged and acknowledged, not fulfilled; recording it is #196. |
| `payment_pending → paid` | `AdminConfirmOrder` (`store.go:1566`, manual proof) / `AdminReconcileOrder` (`store.go:1787`, gateway query), both via shared `markOrderPaidTx` (`store.go:1544`) | **Guarded at the callers, and differently**: confirm locks the order and, when it is not `payment_pending`, returns `order_not_payable` (`store.go:1592-1602`); reconcile locks the order and answers 409 `order_not_payable`, without marking it paid, for one that is neither `payment_pending` nor already settled (`store.go:1818-1856`). `markOrderPaidTx` itself is still a bare write, so any future caller must bring its own guard. |
| `paid → processing` or `paid → completed` | `Worker.handleOrderPaid` (`backend/internal/worker/outbox.go:164`) consuming the `OrderPaid` outbox event | **Guarded**: the order is read with `GetOrderByIDForUpdateTx` inside the fulfilment tx, and a `cancelled` or `payment_expired` order is marked processed and skipped (`outbox.go:181-203`). `paid`, `processing`, `shipped` and `completed` orders are granted (idempotently) because an admin can ship a mixed order before the event is consumed, but the status is only set when it is still `paid`, so a shipped order is never walked back (`outbox.go:281`). A refund or expiry that committed first is seen here; one that commits later waits for this commit and then revokes what was granted. |
| `payment_pending → payment_expired` | `Worker.sweepStalePayments` (`backend/internal/worker/outbox.go:307`) | **Partially guarded**: the sweeper selects orders past `payment_expires_at`, re-reads the status without a lock and writes a plain `SetOrderStatus`, restoring stock per line; a settlement that commits between its read and its write is not closed. |
| `processing`/`shipped` → `completed` | `AdminCompleteOrder` (`store.go:1660`) | **Partially guarded**: a Go-level `switch order.Status` (`shipped` always ok; `processing` only if no physical items) gates the call, but the order is read *without* a lock and the write is a plain `SetOrderStatus` (`store.go:1693`) with no SQL-level status predicate — a concurrent status change between the read and the write is not closed the way `Checkout` closes it. |
| `paid/processing/shipped/completed → cancelled` (refund) | `AdminRefundOrder` (`store.go:1721`) | **Guarded**: refundability is judged under the order row lock (`store.go:1741-1749`) against `refundableStatuses` (`store.go:1708`), so a concurrent refund or fulfilment cannot slip between the check and the status change. |
| `payment_pending → payment_pending` (re-issue a payment link) | `RetryPayment` (`store.go:1278`) | **Guarded before the gateway call**: the order is locked, must belong to the student and be `payment_pending` or `payment_expired`, and the digital guard is re-run with the order excluded. The reference write and status flip after the gateway call are not re-validated, and retry reopens an expired order (it moves back to `payment_pending`) as before. |

`SetOrderStatus` itself (`backend/internal/repository/order.go:808`) is a bare `UPDATE ... WHERE id = $3` — it carries **no status-transition guard of its own**; every caller is individually responsible for checking the prior state. `Checkout`, `handleOrderPaid`, `AdminConfirmOrder`, `AdminReconcileOrder`, `AdminRefundOrder` and `RetryPayment` all take the order row lock before deciding; `AdminCompleteOrder` and the sweeper still read without one.

`paid_at` / `completed_at` / `cancelled_at` are stamped inside `SetOrderStatus` itself via `CASE WHEN $1 = '...'` (`order.go:812-814`) — they are side effects of the status value passed, not separately settable, and only ever move forward (nothing clears them on a later transition).

## Payment gateway (Midtrans)

- `PaymentClient` port: `CreatePayment`, `QueryStatus`, `VerifySignature` (`backend/internal/service/ports_payment.go:52-56`); `MidtransClient` is the real implementation (`backend/internal/adapter/midtrans.go`), `NoopPaymentClient` the fallback when unconfigured.
- Webhook signature: `VerifySignature` (`midtrans.go:124`) recomputes `sha512(order_id + status_code + gross_amount + server_key)` and compares with `subtle.ConstantTimeCompare`. Verified **before** the idempotency check and before any DB read in `HandlePaymentWebhook`.
- Only `transaction_status ∈ {settlement, capture}` is treated as paid; everything else (`pending`, `deny`, `expire`, `cancel`, Midtrans's own test-button notifications) is logged and dropped.
- `AdminReconcileOrder` refuses to call the gateway at all when `order.GatewayRef == ""` — an order that never reached the gateway has nothing to reconcile against.
- Free/zero-total orders (`order.Total == 0`) skip the gateway entirely inside `Checkout` and self-mark `paid` in the same tx as the cart→order flip (`store.go:1199`).

## Refunds — no money moves

- `AdminRefundOrder` (`store.go:1721`) is a manual bank-transfer record, **not** a real refund: `PaymentClient` has no refund method, so nothing reaches Midtrans, stock is not restored, and no `OrderRefunded` event is emitted (comment cites issue #72 as the real flow).
- What it does do, atomically in one tx: flip `status → cancelled` with `cancellation_reason = 'refunded'`, revoke course enrollments (`RevokeEnrollmentsByOrder`, `backend/internal/repository/course.go:339`) and exam registrations (`revokeOrderExamRegistrationsTx`, `exam_revoke.go:287`, from the product's live `product_exam` links), clear shipment tracking (`ClearOrderTracking`, `order.go:1061`), store the transfer receipt (`refund_proof_url`, added in `backend/db/migrations/0058_order_refund_proof.up.sql`), and write an audit row.
- Only orders in `refundableStatuses = {paid, processing, shipped, completed}` are eligible (`store.go:1708`) — refunding a `cart` or an already-`cancelled` order is rejected with `ErrOrderNotRefundable`.
- `RefundProofURL`/`PaymentProofURL` are read back only through presigned URLs (`PresignRefundProofURL`/`PresignPaymentProofURL`, `store.go:1506,1525`) scoped to the fixed field on the order row — never an arbitrary storage key from the caller.

## Revenue — signed ledger on `paid_at`

- `revenueEventCTE` (`backend/internal/repository/dashboard_series.go:53`) is the single source every revenue query builds on:
  - `+1` row per order where `paid_at IS NOT NULL`, timestamped at `paid_at`.
  - `-1` row per order where `cancellation_reason = 'refunded' AND cancelled_at IS NOT NULL`, timestamped at `cancelled_at`.
- `GetRevenue` (`order.go:1077`) sums `sign * total/shipping_cost/discount` over that union — a paid-then-refunded order nets to zero in the window that contains both events, and a refund landing in a *later* window shows as negative revenue there while the original window keeps its positive figure (bucket boundaries are not retroactively adjusted).
- Order counts are **not** signed: a refund reduces revenue but does not decrement the count of orders that were paid.
- `DashboardSeries` buckets in `Asia/Jakarta`, anchored to the query's `from` (not calendar week/month boundaries) — see the comment block in `dashboard_series.go`.
- The digital/physical split sums `order_item.jumlah` per line, deliberately separate from the per-order `total` sum, to avoid multiplying a multi-item order's revenue by its item count (comment references the PR #80 fan-out fix).

## Order → access / shipping fan-out

`Worker.handleOrderPaid` (`backend/internal/worker/outbox.go:164`), consuming the `OrderPaid` outbox event emitted by every paid path above, in one tx that skips an order no longer `paid`:

- `course` line items → `CreateCourseSession` per course the product links **at fulfilment time**, `status: "active"`, `source: "order"`.
- `exam` line items → `CreateExamRegistration` per exam the product links at fulfilment time. If `order_participant` rows exist (bulk purchase, `backend/db/migrations/0031_order_participant.up.sql`), each participant is registered instead of the buyer — fan-out semantics live entirely in presence/absence of those rows.
- `book`/`merchandise`/`medal` line items set `hasPhysicalItem = true`, which decides the resulting status: `completed` if none, else `processing`.
- Course/exam items are stamped `fulfilled_at` via `StampOrderItemFulfilledAt` as they're granted.

Physical fulfillment then runs entirely under admin action, not automatically: `AdminShipOrder`/`AdminShipOrderManual` (`backend/internal/service/shipping_order.go:113,212`) gate on `shippable(order)` (`shipping_order.go:47-52`) — true for `paid`/`processing`, or for `shipped` if the last known `shipment_status` is a failure state (`isShipmentFailureStatus`, `backend/internal/service/shipment_status.go:61`), which is the only escape hatch for a dead booking since `orders.status` is never walked back from `shipped` (FR-C-15). `AdminCompleteOrder` is the only path from `shipped`/digital-only-`processing` to `completed` — see the status table above for its guard gap.

Biteship's shipping webhook (`HandleShippingWebhook`, `backend/internal/service/shipping_webhook.go:52`) verifies `X-Biteship-Signature` against a configured shared secret with `subtle.ConstantTimeCompare`, refusing outright when the secret is unset (an empty-vs-empty compare would otherwise pass — the exact fail-open bug commit `841cd84` shipped once, per the comment in that file). On a verified ping it only re-fetches authoritative state from Biteship (`GetOrder`) and writes `shipment_status`/tracking/`order_shipment_events` — **`orders.status` itself is never advanced by this webhook** (FR-C-12/FR-C-15).

## Dropped from source material

- The general [[order-state-machine]] learning's canonical FSM (`cart → checkout → payment_pending → paid → processing → completed`, with a distinct `checkout` state and a `refunded`-flavored terminal) does not match this codebase: there is no `checkout` status row, `shipped` exists as a real state between `processing` and `completed`, and "refunded" is a `cancellation_reason` value on `cancelled`, not a status. Used only as a lens per the task brief, not restated as fact here.
- Older personal-wiki notes describing refunds as moving money, or listing a `refunded` order status, are superseded by the `AdminRefundOrder` comment block and the current `orders_status_check` constraint above and were not carried into this page.

## Unverified

- Whether a same-millisecond race between `AdminRefundOrder` and the async `handleOrderPaid` consumer (or a late webhook retry) has ever actually re-granted access after a refund in production — both now take the order row lock, but no incident evidence was reviewed here.
- `AdminCompleteOrder`'s read-without-lock is still a narrow TOCTOU gap by inspection; whether it has manifested is unverified.
