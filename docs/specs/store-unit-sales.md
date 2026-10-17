---
title: Unit terjual per tipe produk di /admin/store
issue: none (asked in Discord, relayed by Han; owner request by Panca)
status: accepted
created: 2026-10-17
sources: ["backend/internal/repository/dashboard_series.go", "backend/internal/repository/order_reporting.go", "backend/internal/handler/admin_order.go", "backend/internal/server/routes.go", "backend/internal/service/rbac.go", "web/app/(admin)/admin/store/page.tsx", "web/components/admin/PeriodBar.tsx"]
---

# Unit terjual per tipe produk di /admin/store

## Problem and outcome

`admin_store` has no revenue capability by design, but still needs to know how many units each product type sold. Outcome: `/admin/store` keeps its queue and month summary and gains a block with one row per product type (`book`, `merchandise`, `medal`, `course`, `exam`) showing units, never rupiah. Clicking a row opens a modal with every product of that type; the modal downloads a CSV of that type only. No global export.

## Verified facts (read from `origin/main` 6b43b92f, not run)

- `revenueEventCTE` (`repository/dashboard_series.go`) is already the paid/refund ledger: +1 at `orders.paid_at`, -1 at `orders.cancelled_at` where `cancellation_reason = 'refunded'`. `TopProducts` reads it joined to `order_item`. The new query reuses this definition, so units and revenue pages cannot disagree on which orders count.
- `order_item.product_type` and `qty` are per line, so a mixed order is attributable to each type by its lines.
- `parseDayRange` (handler) reads `YYYY-MM-DD` in Asia/Jakarta and returns a half-open `[from, to+1d)`.
- `admin_store` holds `orders:*`; `super_admin` holds `*`; `admin_exam`, `admin_school`, `student` hold no `orders:*`. `/admin/revenue` and `/admin/dashboard` sit behind `revenue:read`, which `admin_store` lacks.
- No spreadsheet library is in `backend/go.mod` or `web/package.json`; the export is CSV, which `encoding/csv` in the standard library covers.
- `PeriodBar` has presets `7d|30d|90d|this_month|custom` and no all-time option; `30d` sends no dates because the revenue endpoints default to 30 days.

## Agreed behavior

- **Unit of count.** Units are `SUM(order_item.qty)`.
  - Paid units: items of orders whose `paid_at` is in the period.
  - Refund units: items of orders with `cancellation_reason = 'refunded'` whose `cancelled_at` is in the period, **including orders paid in an earlier period**.
  - Net units = paid - refund; it may be negative in a period that only gave units back.
- **Orders.** Paid orders and refund orders are `COUNT(DISTINCT order)` that have at least one item of the type (per type) or of the product (per product). A mixed order counts once in each type it contains. Order counts are never summed across types and no all-types total row exists. Product-level order counts may add up to more than the type's count when one order holds several products of that type.
- **Period.** Dates are WIB, inclusive on both ends (`parseDayRange`). Neither `from` nor `to` = all-time. Only `from` or only `to` = open-ended range. `from` after `to` or a malformed date = 400. The UI always sends explicit dates for presets and sends nothing for "Semua waktu".
- **Rows.** All five types are always returned, zero-filled, in the fixed order above. Within a type, only products with paid or refund activity in the period appear, sorted by net units desc, then name asc, then product id. Product name is the order-line snapshot, as in `TopProducts`.
- **UI.** A titled section on `/admin/store` with its own period control (presets, custom range, and all-time). The whole type row is one native `<button>`; Enter and Space open the modal. The modal (Radix dialog) lists the type's products with the six figures; Escape or close returns focus to the row that opened it. "Unduh CSV" downloads only the open type for the same period. A type with no products shows an empty state and a disabled download.
- **Export.** CSV (UTF-8 with a BOM so Excel reads Indonesian names correctly, comma-separated, RFC 4180 quoting via `encoding/csv`). One header row (Produk, Order dibayar, Order refund, Unit dibayar, Unit refund, Unit neto) and one row per product of the opened type; no total row, because the product order counts do not sum to the type's order count. A product name starting with `=`, `+`, `-` or `@` is prefixed with `'` so a spreadsheet does not run it as a formula. File name `unit-terjual-<type>-<from>_<to>.csv` (`semua-waktu` for open ends); the type and period live in the name only. Export never contains rupiah, price, buyer, order id or another type.
- **Access.** New routes under the existing `adminOrders` group (`orders:write`): only `admin_store` and `super_admin` reach them; the server returns 403 for `admin_exam`, `admin_school` and `student`. No `revenue:read` route is used or widened.

## Technical contract

- `GET /api/admin/orders/unit-sales?from&to` → `{ from, to, types: [{ type, paid_orders, refund_orders, paid_units, refund_units, net_units, products: [{ product_id, name, paid_orders, refund_orders, paid_units, refund_units, net_units }] }] }`. `from`/`to` echo the inclusive dates or null. No money field exists in the response structs (same stance as `OrderSummaryProduct`).
- `GET /api/admin/orders/unit-sales/export?type&from&to` → CSV bytes (`text/csv; charset=utf-8`) with `Content-Disposition: attachment`; `type` outside the five values → 400. Both routes are registered before `/:id`.
- Repository: one ledger-based aggregation returning rows per type and per product, with type-level order counts computed `DISTINCT` (never by summing product rows). Service: zero-fill and ordering of the five types, net units, export rows. Handler: parameter parsing and range validation (`parseDayRange` plus a reversed-range check, as the other admin order endpoints do), and response.
- CSV writer: `encoding/csv` in the service layer (rows + formula-guard helper), no new dependency.
- Frontend: a hook `useAdminUnitSales(range)` and `downloadUnitSalesCsv(type, range)` in `web/lib/hooks`, a `UnitSalesBlock` component (block + modal in one file), a wiring change in the store page, i18n keys in `id` and `en` written separately, and an optional all-time preset on `PeriodBar` (default behavior of the existing dashboard unchanged).
- Wiki: update `docs/wiki/domain/orders-and-payments.md` with the unit-sales definition and the access rule; re-point its `verified_against`.

## Acceptance and proof

DB-backed (real Postgres):

1. Mixed order (book + course): counts once in each type, its units land in each type; the two type order counts are not added anywhere.
2. Several products in one type: product units sum to the type's units; the type's order count is below the product order counts' sum for an order holding two of its products.
3. Refund across periods: paid in period A, refunded in period B: A shows paid units, B shows refund units and a negative net; all-time nets to zero.
4. Refund only counts when `cancellation_reason = 'refunded'`; a cancelled order with another reason and an unpaid order contribute nothing.
5. Period edges in WIB: an order at 23:59 WIB on `to` is included, 00:00 WIB the next day is not.
6. Empty period: five zero rows, no products. All-time (no params) includes everything. Open-ended and invalid ranges (`from` > `to`, malformed) behave as specified, the latter with 400.
7. Sum identity: for every type, the type row = the sum of its products' units = the sum of the units columns read back from the produced CSV.
8. Role matrix on the real route: `admin_store` and `super_admin` 200; `admin_exam`, `admin_school`, `student`, no token = 403/401. Non-leakage: the JSON key set equals the whitelist (no price, total, revenue, buyer, order id fields); the CSV contains no such columns or strings and no other type's products. Unknown `type` on export = 400.

Frontend (vitest, mocked hooks): five rows rendered with figures; click on the row (not just a cell) opens the modal; Enter and Space open it; Escape closes it and focus returns to the opened row; the modal lists the type's products and its total equals the row; the download is called with the open type and the current period; changing the period refetches; empty type shows empty state and disabled download; all-time sends no dates; no rupiah or currency string renders anywhere in the block or modal.

Gate: `deploy/pipeline/backend.sh` then `frontend.sh`, run sequentially. Not provable here: the layout in a real browser, and the CSV opening in Excel/LibreOffice (checked by re-parsing with `encoding/csv`; an Indonesian-locale Excel that expects `;` may show one column, which is a known trade-off of comma CSV).

## Out of scope

Revenue or price of any kind, a global export, per-buyer or per-order drilldown, a total row across types, charts, changes to the existing queue/summary endpoints or `/admin/revenue`, a new capability name, caching, and an issue-map entry (no issue).

## Decided by Panca

- Metric is units sold, not revenue; one block per type; whole-row click opens a modal; export only for the opened type; no global export.
- Download format is CSV, not XLSX: no dependency and no zip writer; the cost is no formatting and the comma/locale caveat in the proof section.
- Period semantics, refund-on-`cancelled_at` rule, mixed-order counting and the access rule as in "Agreed behavior".

## Judgement calls (cheap to overrule)

- Gate is `orders:write` via the existing group, not a new `unit-sales:read` capability: no change to `rbac.go` or the nav config, at the cost of a read endpoint behind a write capability (same as `/orders/summary`).
- No grand-total row and no all-types order count, as requested for orders; units are not totalled either because they would mix books with exam seats.
