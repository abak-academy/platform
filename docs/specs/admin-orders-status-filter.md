---
title: Admin orders status filter
issue: none yet
status: proposed
created: 2026-10-05
sources: ["backend/internal/handler/admin_order.go", "backend/internal/repository/order.go", "backend/internal/repository/order_reporting.go", "backend/internal/worker/outbox.go", "backend/internal/service/store.go", "web/components/admin/OrdersToolbar.tsx", "web/components/admin/OrderRow.tsx", "web/components/orders/OrderStatusBadge.tsx", "docs/wiki/domain/orders-and-payments.md"]
---

# Admin orders status filter

## Problem and outcome

The status dropdown on the admin orders page no longer matches how orders move, and it uses developer words. Audit against the live state machine (`docs/wiki/domain/orders-and-payments.md`):

- **Paid** is transient. `Worker.handleOrderPaid` moves a paid order to `processing` (physical line) or `completed` (digital only) as soon as it consumes the `OrderPaid` event (`outbox.go:281-286`), so the option is almost always empty.
- **Processing** has the same predicate as the `ready_to_ship` queue in practice (paid/processing with a physical line); the rest is legacy digital-only orders.
- **Failed** filters `payment_expired`: the buyer did not pay inside the window, no payment failed. `payment_failed` was folded into `cancelled` by migration 0009.
- **Completed** has no option, although every digital order ends there.
- **Shipped** shows the `in_transit` bucket count (`status='shipped'` and not a failed shipment) but the option lists every `status='shipped'` row, so the number can differ from the rows.
- Only 5 of 9 options show a count.

Outcome: every option is something the operator acts on, is named in the operator's vocabulary, and shows a count equal to the rows it lists.

## Agreed behavior

Decisions (Panca, 2026-10-05): the 8 options below; Paid and Processing are dropped from the dropdown; cancelled and refunded stay one option; row badges use the same words.

Dropdown, in this order (id / en):

1. Semua / All: no filter
2. Menunggu pembayaran / Awaiting payment: `status = payment_pending`
3. Siap kirim / Ready to ship: queue `ready_to_ship` (unchanged predicate)
4. Pengiriman bermasalah / Delivery problem: queue `shipment_failed` (unchanged predicate)
5. Dalam pengiriman / In transit: new queue `in_transit`: `status = 'shipped'` and not a failed shipment
6. Selesai / Completed: `status = completed`
7. Kedaluwarsa / Payment expired: `status = payment_expired`
8. Dibatalkan / Cancelled: `status = cancelled` (includes refunds, see below)

- Every option shows a count that comes from the same predicate builder as the rows (`orderFilterPredicateFrom`), so count and rows cannot drift.
- The API keeps accepting `?status=paid` and `?status=processing`, so bookmarks and the store dashboard deep links survive; the web just no longer offers them.
- Row and detail badges in the admin use operator words derived from status, items and shipment status, matching the dropdown. The student-facing `OrderStatusBadge` default labels are unchanged.

| Order | Admin badge |
|---|---|
| `payment_pending` | Menunggu pembayaran |
| `paid`/`processing` with a physical line | Siap kirim |
| `paid` / `processing`, digital only | Dibayar / Diproses (as today) |
| `shipped`, failed shipment status | Pengiriman bermasalah |
| `shipped`, otherwise | Dalam pengiriman |
| `completed` | Selesai |
| `payment_expired` | Kedaluwarsa |
| `cancelled` | Dibatalkan |

Known limitation, not fixed here: refund is stored as `cancelled` with `cancellation_reason = 'refunded'` and the cancel/refund flow is not settled yet (cancel should mean refund; refund becomes cancel once the refund is done). The two stay one filter option until that flow is defined.

Legacy digital-only orders still in `processing` have no option of their own; they appear under All and their badge.

## Technical contract

- `GET /admin/orders` and `GET /admin/orders/summary`: `?queue=` accepts `in_transit` in addition to `ready_to_ship` and `shipment_failed`; any other value is still `400`. `?status=` is unchanged.
- New `OrderFilter` field for the `in_transit` predicate, built in `orderFilterPredicateFrom` with the failure-status list the service already owns (`service.ShipmentFailureStatusValues()`); the handler sets it in `applyOrderQueue` like the other queues. Spelling decisions stay in the service.
- `OrderBucketCounts` gains `completed`, `payment_expired`, `cancelled` (json keys the same), each a `COUNT(*) FILTER` in the existing `CountOrdersByBucket` query. Existing fields and the dashboard consumers are unchanged.
- Web: `AdminOrderFilterStatus` gains `completed`, drops `paid` and `processing` from the offered set; `AdminOrderQueue` gains `in_transit`; `FILTER_OPTIONS`, `chipCount` and i18n (id and en) follow the table above. The page's mount-time `?queue=` / legacy `?status=<queue>` deep-link handling gains `in_transit` alongside the two existing queues.
- No migration, no new index.

## Acceptance

1. For each of the 8 options, over a seeded set covering every status and a shipped order with a failed shipment status, the summary count equals the number of rows returned by paging the list to the end (DB-backed test).
2. `queue=in_transit` returns shipped orders without a failed shipment status only; an unknown queue is `400`.
3. `?status=paid` and `?status=processing` still filter as before.
4. The dropdown offers exactly the 8 options in order, each with a count, in both locales; selecting one sends the matching `status` or `queue` param and clears the other.
5. Admin row and detail badges follow the table; the student order pages render the same labels as before.
6. Backend and frontend gates pass, run sequentially.

## Out of scope

Product type and courier filters, sorting, page size and Prev/Next (second PR); the cancel/refund flow; moving legacy `processing` digital orders to `completed`; the `ready_to_ship` backlog volume.
