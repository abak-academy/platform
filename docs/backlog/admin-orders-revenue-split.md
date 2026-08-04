# Admin orders redesign + revenue role split

| | |
|---|---|
| **Raised** | 2026-08-04 |
| **Status** | ▶ design approved, implementation plan next |
| **Objective** | The orders page answers both jobs it is opened for — clear the backlog, find one order — and `admin_store` loses revenue while keeping everything fulfilment needs. |
| **Surfaces** | `/admin/orders`, `/admin/store`, `/admin/revenue`, `/admin` |
| **Defects fixed** | D1 revenue inflated by a join fan-out · D2 order-list cursor keyset does not match its sort · D3 dashboard counters capped at 20 |
| **Depends on** | — |
| **Verified against** | `main` @ `a1e6c3b`, 2026-08-04 |

Design agreed with the client on 2026-08-04. Layout chosen from three options
(consolidated table over triage-strip and row-cards). The role split is folded in
here rather than split out, at the client's request, because the order summary
the store dashboard needs is the same query the orders page needs.

---

## 1. Why

`/admin/orders` is the page store admins work from every day. Two jobs bring them
there: clearing the backlog (confirm payments, ship parcels, chase dead
shipments) and looking up a single order when a parent calls. The page today
serves neither well, and while scoping it three production defects surfaced that
are more serious than the layout.

Separately, the client wants `admin_store` to lose access to revenue while
keeping everything they need to fulfil orders. That capability is granted today.

Both are covered here in one spec, at the client's request, because the order
summary the store dashboard needs is the same query the redesigned orders page
needs.

### Defects found during scoping

**D1 — Revenue is inflated in production.** `Repository.GetRevenue` joins
`orders` to `order_item` and then sums `o.total`, the whole order total, once per
item row. A Rp 500.000 order with three items contributes Rp 1.500.000; if those
items span two product types the full order total lands in **both** buckets, so
`by_type` partitions nothing. `COUNT(*)` counts item rows, not orders, and
`/admin/revenue` derives both "Jumlah pesanan" and "Rata-rata / pesanan" from it.
The figures on `/admin/revenue` and the `/admin/store` "Pendapatan 30 hari" tile
are all too high.

**D2 — Cursor pagination is incorrect, not merely unwired.** `ListOrders`
filters `AND id > $cursor` while ordering `created_at DESC`, and returns the last
row's UUID as the cursor. The keyset column and the sort column are unrelated, so
following the cursor drops every order whose UUID sorts below it and can repeat
rows already returned. Nothing sends a cursor today — `useAdminOrders` discards
`next_cursor` — so the bug is dormant, and wiring the frontend up as-is would
ship a broken page 2. The same function serves the student `/orders`.

**D3 — Dashboard counters silently cap at 20.** `/admin/store` renders its
pending and processing tiles from `useAdminOrders(...).length`, the length of a
page the handler hard-limits to 20. The counters stop counting at 20.

Smaller: `/admin/orders` shows at most 20 rows with no indication more exist and
no search; three of its eight columns are status; the action column holds 0–3
buttons so the right edge jitters and destructive **Refund** sits inline beside
**Selesai**; `Complete` uses a raw `window.confirm` while every sibling gets a
modal; `filtered` is a no-op `useMemo`; seven strings are hardcoded Indonesian
among `t()` calls. `/admin/revenue`'s "Produk terlaris" table is a hardcoded
empty state with no query behind it. No `/admin/*` route has an access guard —
`app/(admin)/layout.tsx` only picks a nav config by role.

---

## 2. Scope

**In:** the three defects above; `/admin/orders` redesign with server-side search,
date range, working pagination, and column consolidation; a shared order-summary
endpoint; top-products with a real query on both dashboards; removal of
`revenue:read` from `admin_store` and the guards that make it stick.

**Out (filed, not fixed here):** the `fetchItems` N+1 in `ListOrders`;
`OrderStatusBadge`'s hardcoded labels (shared with student pages); any
amount-bearing export.

---

## 3. Decisions

| Decision | Choice | Reason |
|---|---|---|
| Orders layout | Consolidated table, 8 columns → 6 | Keeps scanning density; smallest change that fixes the real problems |
| Per-order `total` for `admin_store` | **Visible** | Confirming a manual bank transfer means matching the receipt to the amount |
| "No revenue" means | No **aggregates** — no report, no period totals, no sum footers | Per-order amounts are operational; aggregates are the restricted thing |
| Enforcement of the revenue rule | **On the wire, via separate routes** | Strict. A shared payload the client filters leaks via the network tab |
| Top products on `/admin/store` | Ranked by **quantity sold**, no rupiah column | Rank-by-revenue leaks earning order even with amounts stripped; quantity is the restock number anyway |
| Access guard | `useHasCapability` hook, two call sites | Two pages need it; a route-guard framework would be speculative |
| Spec + role split | One spec, one PR | Client's call; the summary query is shared between both halves |

---

## 4. Backend

### 4.1 `ListOrders` — keyset (fixes D2)

```sql
ORDER BY created_at DESC, id DESC
-- with a cursor:
AND (created_at, id) < ($ts, $id)
```

Cursor encodes as `<rfc3339nano>_<uuid>`. The handler's `uuid.Parse` validation
is replaced by a decode returning `ErrInvalidCursor` if either half is bad. The
wire format change is safe because no client sends a cursor today.

### 4.2 `OrderFilter` — new fields

- `Search string` — one term matched two ways in a single `OR`: order number as
  `id::text LIKE '%' || $q` (the UI shows the last 8 characters, so it is a
  suffix match), and buyer name as
  `EXISTS (SELECT 1 FROM users u WHERE u.id = orders.student_id AND u.name ILIKE '%' || $q || '%')`.
  Digits-only input skips the name branch.
  Buyer name **must** be matched in SQL: `student_name` is not a column on
  `orders`, it is hydrated after the page query by `attachStudentNames`, so
  post-filtering would filter an already-paginated page.
- `CreatedFrom`, `CreatedTo *time.Time` — half-open `[from, to)` on `created_at`.

### 4.3 `GetRevenue` — fix the fan-out (fixes D1)

Money sums from the item line, orders count distinctly:

```sql
SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty))   -- product revenue
COUNT(DISTINCT o.id)                                -- orders
```

`oi.jumlah` was added nullable in migration `0009`, hence the `COALESCE`.
`BETWEEN` is replaced by `>= $1 AND < $2`: `BETWEEN` is inclusive at both ends,
so adjacent periods double-count an order sitting exactly on the boundary.

Product revenue will **not** equal `SUM(o.total)` — the gap is `shipping_cost`
and promo discount, which live on the order. The UI reconciles the two rather
than implying they match (§5.3).

### 4.4 New: top products

One repository function, ordered differently by each caller:

```sql
SELECT oi.product_id,
       MAX(oi.name)         AS name,
       MAX(oi.product_type) AS product_type,
       SUM(oi.qty)          AS qty_sold,
       COUNT(DISTINCT o.id) AS order_count,
       SUM(COALESCE(oi.jumlah, oi.unit_price * oi.qty)) AS product_revenue
  FROM orders o
  JOIN order_item oi ON oi.order_id = o.id
 WHERE o.status IN ('paid', 'processing', 'completed')
   AND o.created_at >= $1 AND o.created_at < $2
 GROUP BY oi.product_id
 ORDER BY product_revenue DESC   -- /admin/revenue
 -- ORDER BY qty_sold DESC       -- /admin/orders/summary
 LIMIT $3;
```

Same status set as revenue, so the bars and the table can never disagree.

### 4.5 New: `CountOrdersByBucket`

One grouped query returning:

| Bucket | Definition |
|---|---|
| `needs_confirm` | `status = 'payment_pending'` |
| `ready_to_ship` | `status IN ('paid','processing')` with a physical item |
| `shipment_failed` | `shipment_status` in the failure set |
| `in_transit` | `status = 'shipped'`, not a failure |
| `created_this_month` | `created_at` within the current calendar month, Asia/Jakarta |
| `completed_this_month` | `completed_at` within the current calendar month, Asia/Jakarta |
| `total` | all non-cart orders matching the active filters |

The first five and `total` honour the same search and date filters as the list,
so the counts always describe what the table is showing. The two
`*_this_month` figures are deliberately **not** filtered — they are the store
dashboard's fixed month-to-date volume tiles (§5.2) and must not move when
someone types in the orders search box. Replaces the `.length` counters
(fixes D3).

### 4.6 Routes

| Route | Gate | Carries |
|---|---|---|
| `GET /admin/orders` | `orders:write` | + `q`, `from`, `to`, `limit` (default 20, cap 100) |
| `GET /admin/orders/summary` | `orders:write` | counts + top products as `qty_sold`, `order_count`. **The response type has no money field.** |
| `GET /admin/revenue` | `revenue:read` | totals, per-type, top products **with** `product_revenue` |

Both summary shapes come from the same repository functions; the handlers project
different structs. No conditional field-stripping inside one shape — a payload
whose shape depends on the caller is a divergence that bites later.

### 4.7 RBAC

`revenue:read` is removed from `RoleAdminStore` in `internal/service/rbac.go`.
`super_admin` retains it through `*`.

---

## 5. Frontend

### 5.1 `/admin/orders`

`useAdminOrders` becomes `useInfiniteQuery` keyed on `{status, q, from, to}`, so
any filter change resets to page 1. `getNextPageParam` reads `next_cursor`; a
null cursor hides Load-more. Search debounces 300 ms before entering the key. A
sibling `useAdminOrderSummary` fills the count line with the same filters.

Six columns:

| Column | Content |
|---|---|
| Pesanan | `#a3f91c04`, then transaction date + age (reddening rule below) |
| Pembeli | name, then school |
| Item | first item, then `+N lainnya` |
| Total | right-aligned, tabular numerals |
| Status | `OrderStatusBadge` on top, courier state as sub-line |
| Aksi | one primary button + `⋯` |

**Age reddening** — the age turns red once an order has sat **more than 3 days**
in a state that is waiting on an admin: `payment_pending`, `paid`, `processing`,
or any shipment-failure state. Terminal states never redden — a `completed` order
from six weeks ago is finished, not stale, and reddening it would train people to
ignore the colour. The threshold is one exported constant, not a config surface.

`shippingBadge` is deleted — derived from `shipment_status` plus
`tracking_number`, it restated the other two columns. `shipmentStatusCell`
becomes the sub-line, keeping its red treatment for failures.

**Courier sub-line is a link when `tracking_number` exists**, opening
`TrackingModal` directly for that order — no detour through the order detail.
`TrackingModal`, `trackingOrderId` and `useOrderTracking` already live on the
page; the detail modal was merely one caller of `setTrackingOrderId`. Gate is the
same one `OrderDetailModal` uses for `onTrack`, so the two entry points cannot
disagree. Without a resi, and on a failed shipment, it is plain text — no false
affordance, and a courier-not-found parcel has no waybill to query. The click
calls `stopPropagation`, the same contract the Aksi cell has. **Lacak** stays in
the `⋯` menu for keyboard reach and discoverability.

`actionAllowed` remains the state machine, but exactly one *primary* action shows
per state (confirm → ship → complete). Everything else — refund, reconcile,
track, refresh shipment, cancel shipment, packing slip — moves into `⋯`, with
Refund styled destructive inside the menu where a misclick costs a menu open.

Also: `Complete` gets a confirm dialog instead of `window.confirm`; the no-op
`filtered` `useMemo` is deleted; seven hardcoded Indonesian strings move to keys.

**A11y** — today the `<tr>` is `role="button"` *and* contains buttons, held
together by `stopPropagation`. The row keeps click-to-open, but the accessible
affordance becomes the order-number cell as a real `<button>`.

**Contrast** — sub-lines are small muted text, exactly where the known AA
failures bite. The admin shell uses the `muted-foreground` palette rather than
`ink-*`, so the pair is verified against AA rather than assumed to inherit.

**Mobile** — below `md` the table drops to Pesanan, Status, Aksi, with buyer and
total folded into the Pesanan cell as sub-lines. No separate card component, no
horizontal scroll.

**File size** — `page.tsx` is 522 lines and this adds a toolbar, summary line and
menu. `OrdersToolbar.tsx` and `OrderRow.tsx` are extracted to
`components/admin/`, leaving the page at roughly 250 lines of fetching and modal
wiring. Two files, not a package.

### 5.2 `/admin/store` (admin_store)

Two labelled bands, because the two kinds of number answer different questions:

- **Perlu tindakan** — perlu konfirmasi, siap kirim, pengiriman gagal. Each
  clicks through to `/admin/orders` pre-filtered. The failed tile reddens only
  when non-zero, so red always means real work.
- **Volume bulan ini** — pesanan masuk, selesai. Counts, never rupiah.
- **Produk terlaris** — top 5 by `qty_sold`, columns produk / terjual / pesanan,
  a small bar under each name for ranking. **No rupiah column.**
- **Aksi cepat** — kelola produk, lihat pesanan.

Removed: the revenue tile and the `store_action_revenue_report` action.

### 5.3 `/admin/revenue` (super_admin)

- **Date range** — presets plus manual dates, sent as the `from`/`to` the backend
  already accepts. The handler defaults to `now-30d .. now`; the page currently
  states no period at all, presenting a 30-day window as the whole picture.
- **Three stat cards** — total pendapatan, jumlah pesanan, rata-rata per pesanan;
  the latter two still derived from `by_type`, now from corrected numbers.
- **Per jenis produk** — existing bars, corrected source.
- **Produk terlaris** — top 10 by `product_revenue`; columns produk / terjual /
  pesanan / pendapatan. Replaces the hardcoded empty state.
- **Reconciliation block** beneath the table: product revenue + ongkir − diskon =
  total pendapatan. Without it, the first person to add up the column files a bug.

### 5.4 Guard

`useHasCapability(capability)` reads the role already resolved in
`app/(admin)/layout.tsx`. Used in exactly two places: `/admin/revenue` and
`/admin` render a plain no-access state instead of the report. Nav removal hides
the link; this stops a typed URL or a stale bookmark.

`CONTENT_MANAGER_NAV` loses its revenue entry (`lib/nav-config.ts`).
`SUPER_ADMIN_STORE_ITEMS` keeps its own copy — separate arrays, so this is a
one-line delete with no shared-constant fallout.

---

## 6. Testing

Each of these fails against today's code where a defect exists.

**Backend**

- **Fan-out regression (D1)** — one order, Rp 500.000, three items spanning two
  product types. Total is 500.000, not 1.500.000; `by_type` sums to the total.
- **Keyset (D2)** — seed orders sharing an identical `created_at`, the case that
  breaks a date-only cursor. Page with `limit=3`; the union equals the full set
  with no duplicates. One page cannot demonstrate paging correctness.
- **Boundary** — an order at exactly `to` is excluded, at exactly `from`
  included. This is what `BETWEEN` got wrong.
- **qty vs orders** — one order with qty 5 gives `qty_sold=5, order_count=1`,
  guarding against the two columns silently becoming one number.
- **Search** — order-number suffix; buyer-name partial; digits-only skips the
  name branch.
- **Strict revenue rule, asserted on the wire** — the `/admin/orders/summary`
  response body contains no money-bearing key, **paired with** the inverse
  assertion that `/admin/revenue` does contain it, so the check is proven able to
  fire rather than passing because the matcher is wrong.
- **RBAC** — `admin_store` gets 403 on `/admin/revenue`, 200 on
  `/admin/orders/summary`.

**Frontend**

- Hooks: infinite paging; filter changes reset to page 1.
- Orders page: exactly one primary action per state; courier sub-line clickable
  only with `tracking_number` and never on a failed shipment; `⋯` contains
  refund.
- `/admin/store`: no `Rp` renders anywhere on the page.
- Guard: `admin_store` on `/admin/revenue` renders no-access.

**Gates** — `deploy/pipeline/backend.sh` in full rather than a narrower
`go vet ./internal/...`; `npm run build` rather than only `tsc --noEmit`;
`-timeout 25m` on the integration package.

---

## 7. Rollout

**No migration.** Nothing schema-level changes, so there is no migration number
to reserve or race. New indexes are the only candidate; at thousands of rows the
suffix `LIKE` on order number seq-scans acceptably.

Commit order inside the PR, each green on its own:

1. Fix `GetRevenue` + tests
2. Fix cursor keyset + tests
3. Search / date / limit params, summary endpoint, top-products query
4. RBAC removal + route gates + `useHasCapability`
5. `/admin/orders`
6. `/admin/store`
7. `/admin/revenue`

**Tell the client before merge:** revenue figures will *drop* after this ships.
That is the inflation being removed, not lost sales — but seen first on a
dashboard it reads as a regression.

---

## 8. Open

None. "Produk terlaris" was resolved to a real query on both dashboards, with the
strict no-revenue rule holding for `admin_store`.
