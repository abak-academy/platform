---
title: Admin orders toolbar redesign
issue: none yet
status: proposed
created: 2026-10-06
sources: ["web/components/admin/OrdersToolbar.tsx", "web/app/(admin)/admin/orders/page.tsx", "web/lib/hooks/admin-orders.ts", "web/lib/i18n.ts", "docs/specs/admin-orders-status-filter.md", "docs/specs/admin-orders-filters-sort-paging.md"]
---

# Admin orders toolbar redesign

## Problem and outcome

The toolbar merged in #213 carries seven controls in one wrapping row (search, status, product type, courier, sort select, direction button, page size) next to a separate date row. The owner rejected it on sight: nothing is grouped, the row is crowded, and the active filters are invisible once set.

Outcome (prototype by Han, `order-filters-prototype-v2.html`, approved by Panca): the primary row has only search, a Filters button and sort. Status, product type, courier and date live in one Filters panel. Every active filter stays visible as a removable chip outside the panel. Page size moves to the table footer next to Prev/Next.

This is almost entirely frontend: the query model, hooks, API params and Prev/Next behavior do not change. The one backend change is the `shipped` summary bucket described under "Delivery status merge".

## Agreed behavior

Decisions (Panca, 2026-10-05/06): status moves into the Filters panel and is not a separate tab strip; Han's v2 prototype is the layout.

- **Primary row**: a search input (with a clear button when it has text), a **Filters** button that shows the number of active filters, and a **sort** select. Nothing else.
- **Sort** is one select with six combined options instead of a select plus a direction button: Order date newest / oldest, Paid date newest / oldest, Item count most / fewest. It maps onto the existing `sort` and `dir` query fields (default Order date newest = `order_at` desc).
- **Filters panel** (opens from the Filters button, closes on Escape and outside click, has a "Show orders" button that closes it):
  1. Status: single-select with 7 options, no counts shown on the options (decision, Panca, 2026-10-06): All, Awaiting payment, Ready to ship, In transit (`status = shipped`, id "Sedang dalam pengiriman", en "In transit", merges the former In transit and Delivery problem options by decision, Panca, 2026-10-06), Completed, Payment expired, Cancelled. Single-select because `status` and `queue` are mutually exclusive in the query.
  2. Product type: All types, book, merchandise, medal, course, exam.
  3. Courier: All couriers plus `summary.couriers`.
  4. Order date: single date or range, same behavior as today (a single day is `from === to`; an inbound span opens in range mode).
  5. "Reset filters": clears status, type, courier and dates. It does not clear search, sort or page size.
- **Active-filter chips** below the primary row, visible while the panel is closed: Status, Type, Courier and Date, each with a remove button; the row is hidden when none are active. Search and sort are not chips (they are visible in their own controls).
- **Table footer**: "Rows per page" select (20 / 50 / 100) on the left; on the right the "a-b of N" line and Prev / Next. Behavior of the cursor stack, the count N and disabled states is unchanged.
- Changing search, any filter, sort or page size returns to page 1 (unchanged; all of them already go through `changeQuery`).
- The dashboard cards and old bookmarks keep working: the mount-time `?queue=` / `?status=` handling is untouched.
- One extra click for the queues that used to be the second control (Ready to ship); the dashboard cards already deep-link to them.

Differences from the prototype, on purpose: the prototype's sample data, "All orders / N sample orders" heading and demo banner are not built (the footer already carries a-b of N); colors, spacing and type come from the app's design tokens and existing UI components, not the prototype's CSS; the prototype's empty-state "Clear filters" action is not added (not requested; the panel has Reset filters).

## Technical contract

- `OrdersToolbar` takes `value`, `onChange` and `couriers` (the `counts` prop goes, nothing in the toolbar shows counts any more); the exported `chipCount` stays because the footer's "of N" still reads it. `AdminOrderQuery` is unchanged.
- New `web/components/ui/popover.tsx` wrapping `Popover` from the `radix-ui` package the app already depends on (same import style as `ui/select.tsx`). No new dependency. Select controls inside the panel must keep working while the popover is open.
- The Select components in the panel use the existing `ui/select`; the test helpers already open them with `pointerDown`.
- The page-size select moves from the toolbar into the footer in `page.tsx`; the `limit` field and its reset-to-page-1 behavior stay.
- i18n: every new string in `id` and `en` (real English). Remove keys this change orphans (the sort direction button keys and the old single-sort labels if unused) after grepping every locale and test.
- No new request and no migration; the only backend change is the `shipped` summary bucket.

## Acceptance

1. The primary row contains exactly: search, Filters, sort. The Filters button shows the active-filter count (0 shows no number).
2. The panel contains status (7 options, no counts), product type, courier, order date (single/range) and Reset filters; Escape and outside click close it; "Show orders" closes it; a Select inside it can be opened and chosen without closing the panel.
3. Each control sends the same query as before (status vs queue mutually exclusive, `type`, `courier`, `from`/`to`, `sort`/`dir`, `limit`, `q`), and any change returns to page 1 with Prev disabled.
4. Every active status/type/courier/date appears as a chip with an accessible remove button; removing one clears just that filter; no chips renders no chip row; search and sort never become chips.
5. Reset filters clears status, type, courier, dates only.
6. The six sort options map to the right `sort`/`dir`; the default shows Order date newest.
7. The page-size select is in the table footer, offers 20/50/100, and Prev/Next and "a-b of N" behave as before.
8. id and en strings exist for every new label; no orphaned i18n keys remain; student pages and the dashboard are untouched.
9. The real component is rendered with the app CSS and looked at (closed, panel open, chips visible) at a desktop width and a narrow width before the PR is opened; the PR body states that the live app was not seen.
10. Frontend gate passes (`deploy/pipeline/frontend.sh` with Go on PATH) and `npm run build`.

## Delivery status merge

- The In transit option filters `status = shipped`, so orders whose shipment failed stay in it; their row badge still reads "Pengiriman bermasalah" and the courier line stays red. The failed-shipment queue is no longer an option of its own.
- `?queue=shipment_failed`, `?queue=in_transit` and legacy `?status=paid|processing` links (the dashboard card still links to the first) keep filtering and the status control shows their label even though no option matches (the select renders the label of the selected value itself).
- Backend: the summary buckets gain `shipped` (all `status='shipped'`, same predicate builder as the list) so the footer's "of N" is exact; `in_transit` and `shipment_failed` stay for the deep links and the dashboard.
- Row badge for a shipped order that has not failed uses the same words as the option: "Sedang dalam pengiriman" (en "In transit").

## Out of scope

Backend or API changes beyond the `shipped` summary bucket, new filters, a class filter, an empty-state clear action, column changes in the table, the cancel/refund flow.
