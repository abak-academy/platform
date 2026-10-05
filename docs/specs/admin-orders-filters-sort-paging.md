---
title: Admin orders filters, sort and paging
issue: none yet
status: proposed
created: 2026-10-05
sources: ["backend/internal/handler/admin_order.go", "backend/internal/repository/order.go", "backend/internal/repository/order_reporting.go", "backend/internal/repository/cursor.go", "web/components/admin/OrdersToolbar.tsx", "web/app/(admin)/admin/orders/page.tsx", "web/lib/hooks/admin-orders.ts", "docs/specs/admin-orders-status-filter.md"]
---

# Admin orders filters, sort and paging

Second step after `admin-orders-status-filter.md` (status dropdown). Client request for Abak Hub: the admin orders page has too few filters and no sort.

## Agreed behavior

Decisions (Panca, 2026-10-05):

- **Filters added:** product type and courier. Class/grade is not a filter; school and buyer name stay reachable through the existing search.
- **Sort:** order date, paid date, and item count (not total amount), each asc or desc.
- **Page size:** the admin picks 20, 50 or 100 per page (default 20).
- **Paging:** Prev / Next only, no page numbers and no jump to a page.

Details decided while writing this spec (open to correction):

- **Item count** is `SUM(order_item.qty)` per order; an order with no items counts 0.
- **Courier** filters on `orders.courier_code` (set at checkout from the chosen Biteship rate, so digital orders have none). The dropdown lists the distinct non-empty `courier_code` values of non-cart orders, from the summary response, not a static list.
- **Product type** matches orders that have at least one item of that type (`book`, `merchandise`, `medal`, `course`, `exam`), the semantics `OrderFilter.ProductType` already has.
- **Paid date** sort puts unpaid orders (`paid_at IS NULL`) last for both directions, so sorting by paid date never opens with a screen of pending orders.
- Default order is unchanged: order date (`COALESCE(checked_out_at, created_at)`), newest first.
- Changing any filter, sort or the page size returns to page 1.
- Filters and sort combine with the status dropdown, search and dates. The status dropdown counts follow the type and courier filters (same predicate builder), so a count equals the rows it lists.

## Technical contract

Backend, `GET /admin/orders` and `GET /admin/orders/summary` (the store dashboard calls summary without these params and is unaffected):

- `?type=` accepts the five product types; anything else is 400 (today the value is passed through unchecked).
- `?courier=` is matched exactly against `courier_code`. Not validated against a list (a courier that was later disabled still has orders).
- `/admin/orders` only: `?sort=order_at|paid_at|item_qty` and `?dir=asc|desc`. Unknown values are 400; absent means `order_at` desc. `limit` already exists (default 20, max 100) and is unchanged.
- The whitelist and its validation live in the handler/service; the repository receives a typed sort and maps it to a fixed SQL expression. No user text reaches SQL.
- `OrderFilter` gains `CourierCode`; `orderFilterPredicateFrom` adds the predicate, so list and counts share it. The summary handler also reads `type` and `courier`.
- The summary response gains `couriers: string[]` (distinct, sorted, unfiltered by the other params).
- **Keyset cursor per sort.** Order by `<sort key> <dir> NULLS LAST, id <dir>`; the cursor carries the last row's key and id. `order_at` keeps today's `<rfc3339nano>,<uuid>`; `paid_at` carries the key or a null marker; `item_qty` carries the integer. Resume predicate for `paid_at` with NULLS LAST: after a non-null key, take rows past the key (strictly, with the id tiebreak) **plus every null-key row**; after a null key, take only null-key rows past the id. A cursor is valid only with the sort that produced it; one that does not decode for the requested sort is `ErrInvalidCursor` (400 as today). The web resets its cursors on any sort change.
- No migration and no index: ~700 orders today; sort on expressions is no worse than the existing `placedAtExpr` sort.

Web:

- `AdminOrderQuery` gains `type`, `courier`, `sort`, `dir`, `limit`; the list query key and the summary scope include what applies.
- Toolbar: product type select, courier select (options from `summary.couriers`), sort select with an asc/desc toggle, page-size select (20/50/100). Labels in id and en; reuse existing `product_type_*` keys.
- Prev/Next replaces the infinite list: one request per page, keyed by (query, cursor); the page holds a stack of cursors (index 0 is the first page). Next pushes the response's `next_cursor`, Prev pops. Prev is disabled on page 1, Next when there is no `next_cursor`. The stack resets whenever filter, sort or page size changes. The previous page stays visible while the next loads.
- The "Showing a-b of N" line uses the stack depth and page size for a-b, and N is the count of the selected status option.

## Acceptance

1. `type=medal` returns only orders with a medal item; `courier=jne` only `courier_code='jne'`; both combine with status, queue, search and dates; an unknown `type` is 400.
2. For every sort x dir (6 combinations), paging a seeded set (including equal keys, equal paid dates, null `paid_at`, orders with 0 and many items) with a small limit returns exactly the same ids as the unpaged query in the same order: no duplicates, no gaps.
3. With `sort=paid_at`, null `paid_at` rows are last for asc and desc, on every page boundary, including a boundary that falls on the last non-null row.
4. Invalid `sort` or `dir` is 400; with neither, the order is identical to today.
5. Status dropdown counts equal the rows for the selected type/courier combination (extends the existing count-equals-rows test).
6. `summary.couriers` lists each distinct courier once, sorted, excluding null/empty.
7. Web: each control sends the right param; changing any control or page size returns to page 1 with Prev disabled; Next -> Prev shows page 1's rows again; Next is disabled on the last page; page sizes 20, 50, 100 send `limit`; labels exist in id and en.
8. Backend and frontend gates pass, run sequentially.

## Out of scope

Class/grade filter; sorting by total amount or buyer name; page numbers or jump-to-page; showing paid date or item count as new columns; an index; the cancel/refund flow.
