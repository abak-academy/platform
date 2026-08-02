# Backlog: persist `is_estimate` on the order

**Raised:** 2026-07-27 · **Status:** ✅ **CLOSED 2026-07-31 — shipped as [E6](e6-shipping-logistics.md)
Track B** ([PR #66](https://github.com/abak-academy/platform/pull/66), migration
`0047_order_is_estimate`). The column is persisted on `orders` and backfilled for historical rows
(`UPDATE orders SET is_estimate = true WHERE selected_courier IN ('Ongkir Flat', 'Flat')`), so the
frontend no longer infers the badge from the courier name.

> Everything below is the original problem statement, kept as the record of why the change was made.
> It is **history, not open work** — do not schedule from it.

## The problem

The order detail page decides whether to show the "Estimasi — bukan tarif kurir" badge by comparing
the stored `selected_courier` against a known set of strings:

```ts
const ESTIMATE_COURIERS = new Set(["Ongkir Flat", "Flat"]);
```

The rate itself carries a proper `is_estimate` boolean — the cart uses it directly and is correct.
But **nothing persists that flag on the order**, so once the quote is gone the page has only the
courier name to go on.

Consequences:

- Renaming the fallback label breaks the badge on every historical order unless the frontend set is
  updated in the same change. `TestResolveShippingRates` now asserts the backend constants to make
  that failure loud, but the coupling is still real.
- The set has to keep accumulating legacy labels forever.
- A real carrier that ever happened to be named the same as the fallback would be mislabelled.

## Why it was not fixed when the label was made human-readable

`selected_courier` is not written when rates are listed — it is written when the buyer picks one, via
`PatchCart` (`internal/repository/order.go:384`), which carries only two strings. Persisting
`is_estimate` therefore means more than adding a column: the service has to re-resolve rates at patch
time and work out whether the buyer's choice was the fallback. That is new logic on the checkout
path.

It was deferred because the change landed hours after production first went live, with **zero orders
and zero users** to verify against, and the visible defect was cosmetic (the fallback displayed as
"FLAT / Standard"). Adding untested logic to checkout at that moment bought nothing.

## What the fix looks like

1. Migration: `ALTER TABLE orders ADD COLUMN is_estimate BOOLEAN NOT NULL DEFAULT false`.
2. `model.Order` + the repository INSERT/SELECT in `internal/repository/order.go`.
3. `PatchCart` resolves the current rates and sets the flag when the chosen courier/service pair is
   the fallback.
4. `web/lib/types.ts` gains `is_estimate?: boolean`; `ShippingInfo.tsx` reads it and
   `ESTIMATE_COURIERS` is deleted.
5. Optionally then make the label itself configurable (`shipping_fallback_label` in system_config,
   plus a field on the admin System Config page) — safe only once the badge no longer depends on the
   string.

Backfill for existing rows: `UPDATE orders SET is_estimate = true WHERE selected_courier IN
('Ongkir Flat', 'Flat')`.

## Related

`shipping_fallback_flat_rate` is already a system_config key and is set to 25000 in production, with
the Biteship key deliberately left empty so the fallback path is the live one.
