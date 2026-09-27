---
title: Orders & Payments
updated: 2026-09-25
verified_against: e6cf227c
---

See also: [../architecture/system-overview.md](../architecture/system-overview.md), [../architecture/shipping-biteship.md](../architecture/shipping-biteship.md), [./exam-lifecycle-and-scoring.md](./exam-lifecycle-and-scoring.md).

## Catalog & product

- `model.Product` (`backend/internal/model/product.go`): `type` ∈ `book|merchandise|medal|course|exam`, `status` ∈ `draft|published|hidden|archived` (constraint `product_status_check`, `backend/db/migrations/0009_schema_fixes.up.sql:6-8`).
- `isPhysicalType(t string) bool` (`backend/internal/service/store.go:1953`) — only `book`, `merchandise`, `medal` count as physical (shipped, stock-tracked). `course`/`exam` are digital, unlimited, no stock.
- `AvailableFrom`/`AvailableUntil` bound marketplace visibility (nullable = unbounded side); `*Set` flags on the struct distinguish "field omitted" from "field cleared" on partial updates.
- RBAC on product mutation is per-type: `checkTypeRBAC` (`store.go:1955`) — `admin_store` may touch price/visibility on any type including `exam`, but never exam content (owned by `admin_exam`).

## Cart

- `MintCart` (`store.go:532`) gets-or-creates the student's single `cart`-status order.
- `AddItem` / `RemoveItem` / `UpdateItemQty` (`store.go:540,610,652`) each re-check `order.Status == "cart"` at the service layer, then the repository (`AddItem`/`RemoveItem`/`UpdateItemQty` in `backend/internal/repository/order.go:598-720`) takes `GetOrderByIDForUpdateTx` (a row lock) and re-checks `status = 'cart'` again before writing — see comment at `order.go:592-597`. This closes the TOCTOU window an unlocked service-layer check alone would leave open.
- Digital items (`course`/`exam`) are capped at qty 1 and unique per cart — enforced twice: `ValidateItemQty` (`backend/internal/service/item_qty.go:16`) at the service layer, and a locked re-check inside `AddItem`'s transaction (`order.go:614-623`, `ErrDuplicateDigitalItem`) that catches two concurrent `AddItem` calls racing past the first check.
- Adding/removing a physical item clears `shipping_cost`/`selected_courier`/`selected_service` (`clearShipping`) — the quote goes stale the moment the parcel's weight/contents change.
- `PatchCart` (`store.go:750`) writes the shipping address/courier/promo; `validateAddressHierarchy` (`store.go:723`) enforces province→city→district FK consistency before it's persisted or priced.

## Order status machine

DB constraint (`backend/db/migrations/0013_shipped_status.up.sql:2-4`, current):

```
status IN ('cart', 'payment_pending', 'paid', 'processing', 'shipped', 'completed', 'payment_expired', 'cancelled')
```

There is **no `refunded` status** — a refund is recorded as `status = 'cancelled'` with `cancellation_reason = 'refunded'` (see Refunds below). There is also no separate `checkout` state — checkout is a same-transaction `cart → payment_pending` (or straight to `paid` for free orders) flip, not a persisted intermediate status.

Observed transitions and where each is written:

| From → To | Trigger | Guard on the write |
|---|---|---|
| `cart → payment_pending` | `Checkout` (`store.go:1072`), gateway path | **Guarded**: `GetOrderByIDForUpdateTx` locks the row, re-checks `locked.Status != "cart"` (`store.go:1114-1126`) before `CheckoutOrder` + `SetOrderStatus` in the same tx. |
| `cart → paid` | `Checkout`, zero-total/free path (`store.go:1181-1195`) | Same lock as above; free path skips the gateway and emits `OrderPaid` in the same tx. |
| `payment_pending → paid` | `HandlePaymentWebhook` (`store.go:1915`, Midtrans settlement/capture) | **Unguarded**: reads `order` once, then calls `SetOrderStatus(..., "paid", "")` with no re-check of current status and no row lock. |
| `(any) → paid` | `AdminConfirmOrder` (`store.go:1532`, manual proof) / `AdminReconcileOrder` (`store.go:1748`, gateway query) via shared `markOrderPaidTx` (`store.go:1510`) | **Unguarded**: neither caller checks `order.Status` before flipping to `paid`; idempotency keys dedupe *repeat calls with the same key*, not concurrent/late calls with a fresh key against an order already `completed` or `cancelled`. |
| `paid → processing` or `paid → completed` | `Worker.handleOrderPaid` (`backend/internal/worker/outbox.go:163-267`), consuming the `OrderPaid` outbox event | **Unguarded**: grants course/exam access and writes `nextStatus` (`completed` if all-digital, else `processing`) with a plain `SetOrderStatus`, no check that the order is still `paid` at consume time. Outbox delivery is async and can run after a refund has already moved the order to `cancelled`. |
| `payment_pending → payment_expired` | `Worker.sweepStalePayments` (`outbox.go:283-315`) | **Guarded**: re-fetches the order inside its own tx and skips unless `currentOrder.Status == "payment_pending"` (`outbox.go:305-308`) before writing and restoring stock. |
| `processing`/`shipped` → `completed` | `AdminCompleteOrder` (`store.go:1623`) | **Partially guarded**: a Go-level `switch order.Status` (`shipped` always ok; `processing` only if no physical items) gates the call, but the write itself is a plain `SetOrderStatus` with no row lock or SQL-level status predicate — a concurrent status change between the read and the write is not closed the way `Checkout` closes it. |
| `paid/processing/shipped/completed → cancelled` (refund) | `AdminRefundOrder` (`store.go:1684`) | **Guarded** by `refundableStatuses` map check (`store.go:1671-1676`) before the write, but — like `AdminCompleteOrder` — no row lock, so a same-moment webhook or admin-confirm could theoretically land `paid` after the refund's read and before its write. |
| `payment_pending → payment_pending` (retry) | `RetryPayment` (`store.go:1261`) | Go-level check `order.Status != "payment_pending" && != "payment_expired"` (`store.go:1284`), no row lock. |

`SetOrderStatus` itself (`backend/internal/repository/order.go:796-812`) is a bare `UPDATE ... WHERE id = $3` — it carries **no status-transition guard of its own**; every caller is individually responsible for checking the prior state, and `Checkout`/`sweepStalePayments` are the only two that actually close the race with a row lock or a tx-scoped re-read immediately before the write. The rest rely on a single unlocked read, which is a real (if narrow) gap given `AdminRefundOrder`, `AdminConfirmOrder`/`AdminReconcileOrder`, and the async `handleOrderPaid` consumer can all touch the same order from different processes.

`paid_at` / `completed_at` / `cancelled_at` are stamped inside `SetOrderStatus` itself via `CASE WHEN $1 = '...'` (`order.go:800-802`) — they are side effects of the status value passed, not separately settable, and only ever move forward (nothing clears them on a later transition).

## Payment gateway (Midtrans)

- `PaymentClient` port: `CreatePayment`, `QueryStatus`, `VerifySignature` (`backend/internal/service/ports_payment.go:44-48`); `MidtransClient` is the real implementation (`backend/internal/adapter/midtrans.go`), `NoopPaymentClient` the fallback when unconfigured.
- Webhook signature: `VerifySignature` (`midtrans.go:111-124`) recomputes `sha512(order_id + status_code + gross_amount + server_key)` and compares with `subtle.ConstantTimeCompare`. Verified **before** the idempotency check and before any DB read in `HandlePaymentWebhook` (`store.go:1861-1864`).
- Only `transaction_status ∈ {settlement, capture}` is treated as paid; everything else (`pending`, `deny`, `expire`, `cancel`, Midtrans's own test-button notifications) is logged and dropped (`store.go:1877-1883`).
- `AdminReconcileOrder` refuses to call the gateway at all when `order.GatewayRef == ""` (`store.go:1770-1772`) — an order that never reached the gateway has nothing to reconcile against.
- Free/zero-total orders (`order.Total == 0`) skip the gateway entirely inside `Checkout` and self-mark `paid` in the same tx as the cart→order flip (`store.go:1181`).

## Refunds — no money moves

- `AdminRefundOrder` (`store.go:1684-1746`) is a manual bank-transfer record, **not** a real refund: `PaymentClient` has no refund method, so nothing reaches Midtrans, stock is not restored, and no `OrderRefunded` event is emitted (comment cites issue #72 as the real flow, `store.go:1678-1682`).
- What it does do, atomically in one tx: flip `status → cancelled` with `cancellation_reason = 'refunded'`, revoke course enrollments (`RevokeEnrollmentsByOrder`) and exam registrations (`revokeOrderExamRegistrationsTx`), clear shipment tracking (`ClearOrderTracking`), store the transfer receipt (`refund_proof_url`, added in `backend/db/migrations/0058_order_refund_proof.up.sql`), and write an audit row.
- Only orders in `refundableStatuses = {paid, processing, shipped, completed}` are eligible (`store.go:1671-1676`) — refunding a `cart` or an already-`cancelled` order is rejected with `ErrOrderNotRefundable`.
- `RefundProofURL`/`PaymentProofURL` are read back only through presigned URLs (`PresignRefundProofURL`/`PresignPaymentProofURL`, `store.go:1472-1503`) scoped to the fixed field on the order row — never an arbitrary storage key from the caller.

## Revenue — signed ledger on `paid_at`

- `revenueEventCTE` (`backend/internal/repository/dashboard_series.go:53-65`) is the single source every revenue query builds on:
  - `+1` row per order where `paid_at IS NOT NULL`, timestamped at `paid_at`.
  - `-1` row per order where `cancellation_reason = 'refunded' AND cancelled_at IS NOT NULL`, timestamped at `cancelled_at`.
- `GetRevenue` (`backend/internal/repository/order.go:1065-1126`) sums `sign * total/shipping_cost/discount` over that union — a paid-then-refunded order nets to zero in the window that contains both events, and a refund landing in a *later* window shows as negative revenue there while the original window keeps its positive figure (bucket boundaries are not retroactively adjusted).
- Order counts are **not** signed: `COUNT(*) FILTER (WHERE sign = 1)` (`order.go:1075`) and `COUNT(DISTINCT e.id) FILTER (WHERE e.sign = 1)` (`order.go:1087`) — a refund reduces revenue but does not decrement the count of orders that were paid.
- `DashboardSeries` buckets in `Asia/Jakarta`, anchored to the query's `from` (not calendar week/month boundaries) — see comment block at `dashboard_series.go:78-89`.
- The digital/physical split sums `order_item.jumlah` per line, deliberately separate from the per-order `total` sum, to avoid multiplying a multi-item order's revenue by its item count (comment references the PR #80 fan-out fix, `dashboard_series.go:90-93`).

## Order → access / shipping fan-out

`Worker.handleOrderPaid` (`backend/internal/worker/outbox.go:163-267`), consuming the `OrderPaid` outbox event emitted by every paid path above, in one tx:

- `course` line items → `CreateCourseSession` per linked course, `status: "active"`, `source: "order"`.
- `exam` line items → `CreateExamRegistration` per linked exam. If `order_participant` rows exist (bulk purchase, `backend/db/migrations/0031_order_participant.up.sql`), each participant is registered instead of the buyer — fan-out semantics live entirely in presence/absence of those rows.
- `book`/`merchandise`/`medal` line items set `hasPhysicalItem = true`, which decides the resulting status: `completed` if none, else `processing`.
- Course/exam items are stamped `fulfilled_at` via `StampOrderItemFulfilledAt` as they're granted (`outbox.go:246`).

Physical fulfillment then runs entirely under admin action, not automatically: `AdminShipOrder`/`AdminShipOrderManual` (`backend/internal/service/shipping_order.go:113,212`) gate on `shippable(order)` (`shipping_order.go:47-52`) — true for `paid`/`processing`, or for `shipped` if the last known `shipment_status` is a failure state (`isShipmentFailureStatus`, `backend/internal/service/shipment_status.go:61`), which is the only escape hatch for a dead booking since `orders.status` is never walked back from `shipped` (FR-C-15). `AdminCompleteOrder` is the only path from `shipped`/digital-only-`processing` to `completed` — see the status table above for its guard gap.

Biteship's shipping webhook (`HandleShippingWebhook`, `backend/internal/service/shipping_webhook.go:52-87`) verifies `X-Biteship-Signature` against a configured shared secret with `subtle.ConstantTimeCompare`, refusing outright when the secret is unset (an empty-vs-empty compare would otherwise pass — the exact fail-open bug commit `841cd84` shipped once, per the comment at `shipping_webhook.go:44-47`). On a verified ping it only re-fetches authoritative state from Biteship (`GetOrder`) and writes `shipment_status`/tracking/`order_shipment_events` — **`orders.status` itself is never advanced by this webhook** (FR-C-12/FR-C-15).

## Dropped from source material

- The general [[order-state-machine]] learning's canonical FSM (`cart → checkout → payment_pending → paid → processing → completed`, with a distinct `checkout` state and a `refunded`-flavored terminal) does not match this codebase: there is no `checkout` status row, `shipped` exists as a real state between `processing` and `completed`, and "refunded" is a `cancellation_reason` value on `cancelled`, not a status. Used only as a lens per the task brief, not restated as fact here.
- Older personal-wiki notes describing refunds as moving money, or listing a `refunded` order status, are superseded by the `AdminRefundOrder` comment block and the current `orders_status_check` constraint above and were not carried into this page.

## Unverified

- Whether a same-millisecond race between `AdminRefundOrder` and the async `handleOrderPaid` consumer (or a late webhook retry) has ever actually re-granted access after a refund in production — the code path exists as described above, but no incident evidence was reviewed here.
- `AdminCompleteOrder` and `AdminRefundOrder`'s lack of a row lock is a real TOCTOU gap by inspection; whether it has manifested is unverified.
