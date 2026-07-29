# E6 — Shipping & logistics

| | |
|---|---|
| **Issue** | [#60](https://github.com/abak-academy/platform/issues/60) |
| **Objective** | Ongkir the buyer is quoted is a real carrier price, the parcel is booked with a real waybill, and both the admin and the buyer can see where it is. |
| **Source IDs** | NF-1, FB-11 + D-2, D-4, D-6 |
| **Client items** | 2 |
| **Depends on** | E1 (Gotenberg must be proven before the label is built on it) |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

> ⚠️ **This epic contains a PRD scope reversal.** The PRD lists *"Auto-waybill generation and
> real-time tracking (logistics Level 2/3)"* under **Explicitly Out of Scope (MVP)**, with only rate
> calculation (Level 1) in scope. Level 3 reverses that line. The client approved it verbally on
> 2026-07-29; **record the sign-off before the first commit.**

---

## 1. Start with the rate audit — not with order creation

The client asked to check the price calculation first. That is the right order, and the reason is
specific: **Biteship currently degrades to the flat rate with zero log lines** on three distinct
failures —

- `app_kode_pos` (the shipping origin) is missing,
- the API key is rejected,
- the route is not served by any courier.

All three land on the same silent fallback. Two error-swallowing sites are already identified.

And the fallback is not hypothetical: **the Biteship key is deliberately empty in production**, so
the flat-rate path is the live path. Practically every real order today is a flat estimate.

Layering order creation onto a rate path nobody can observe is how a parcel ships at the wrong price.

**First deliverable:** a real quote from production config with the source proven — the
`shipping client resolved source=` line — plus a deliberately broken key producing a *visible* error
rather than a silent flat rate.

Only then: order creation.

---

## 2. D-2 — persist `is_estimate` **before** courier strings change

*This is not optional and not deferrable within this epic.*

The rate object carries the flag ([`ports_logistics.go:18`](../../backend/internal/service/ports_logistics.go)):

```go
IsEstimate bool `json:"is_estimate"`
```

The order row does not. So the badge is re-derived from the courier **string**, duplicated in two
components:

```ts
const ESTIMATE_COURIERS = new Set(["Ongkir Flat", "Flat"]);
```

— [`ShippingInfo.tsx:7`](../../web/components/orders/ShippingInfo.tsx) (storefront) and
`OrderDetailModal.tsx` (admin). The backend even keeps `LegacyFallbackCourier = "Flat"` alive purely
because one rename already happened.

**Level 3 replaces those strings with Biteship's own.** The day it ships, every historical order
silently loses its badge — the buyer reads a guessed price as a quoted carrier price.

**The main reason this was deferred has evaporated.** The original write-up says persisting the flag
means "new logic on the checkout path… the service has to re-resolve rates at patch time". `PatchCart`
now **already** re-resolves rates to validate the courier choice, and the matched `rate` is in hand
([`store.go:708-714`](../../backend/internal/service/store.go)):

```go
for _, rate := range rates {
    if strings.EqualFold(rate.Courier, patch.Courier) && strings.EqualFold(rate.Service, patch.Service) {
        repoPatch.ShippingCost = float64(rate.Price)
        matched = true
        break
    }
}
```

So it is now: one migration, model + repo columns, **one line** (`repoPatch.IsEstimate = rate.IsEstimate`)
inside that loop, one FE field, delete both sets, plus the backfill:

```sql
UPDATE orders SET is_estimate = true WHERE selected_courier IN ('Ongkir Flat', 'Flat');
```

Full write-up: [`shipping-estimate-flag.md`](shipping-estimate-flag.md) — its *"why it was not fixed"*
section is now stale for the reason above.

Doing this unlocks a second thing: making the fallback label configurable
(`shipping_fallback_label` in `system_config`) is only safe once the badge no longer depends on the
string.

---

## 3. NF-1 — Biteship Level 3

Once rates are trustworthy and the flag is persisted:

1. **Create the order at Biteship** when an admin ships, instead of asking them to type a number.
2. **Store the returned `waybill_id`** as the tracking number.
3. **Consume tracking webhooks** into a shipment status the student can see as a timeline.

---

## 4. FB-11 — cetak resi

A printable shipping label carrying the waybill, barcode, sender and recipient — the thing that goes
on the parcel.

> **Skip the interim version.** An earlier plan had a label rendered from the manually-typed
> `tracking_number` in a separate session. Since this epic replaces that number with a Biteship
> waybill, the interim label is throwaway work. Build it once, here, from the real waybill. The
> `(print)` route group already exists for exam cards.

Renders through the Gotenberg sidecar — which is why **E1 must have proven the sidecar renders at
all** before this item starts.

---

## 5. D-6 — webhook signature test at the handler boundary

This epic adds a **second** unsigned-webhook entry point. There is no handler-level signature
regression test for the *payment* webhook either — the bypass fixed in `841cd84` is only covered at
the service layer.

Write the handler-level test covering **both** webhooks. Adding a second entry point without it
doubles an exposure that has no precedent to copy.

---

## 6. D-4 — one definition of digital-vs-physical

Four definitions exist, and they are not even the same shape:

| Location | Shape |
|---|---|
| [`lib/shipping.ts:4`](../../web/lib/shipping.ts) | lists **physical** types |
| [`ShippingInfo.tsx:7`](../../web/components/orders/ShippingInfo.tsx) | lists **physical** types |
| [`catalog/[id]/page.tsx:77`](../../web/app/(student)/catalog/[id]/page.tsx) | lists **physical** types |
| [`CartLineItem.tsx:34`](../../web/components/cart/CartLineItem.tsx) | lists **digital** types — the complement |

A new physical product type would be treated as **digital in the cart** and physical everywhere else.
Collapse to one definition while shipping code is open anyway.

---

## Acceptance

- A quote for a known address returns a real Biteship rate with the resolved source logged; a
  deliberately broken key produces a visible error, not a silent flat rate.
- Orders shipped **before** this epic still show their estimate badge correctly (D-2 backfill proven).
- Shipping an order creates it at Biteship and stores the returned waybill.
- The label prints with that waybill and is checked against a real parcel, or at minimum opened as a
  PDF.
- A tracking webhook with a bad signature is rejected, with a handler-level test — for both webhooks.
- One definition of digital-vs-physical remains in the frontend.
- The PRD scope-change sign-off is recorded.

## Out of scope

- Promo, manual payment confirmation, order listing → [E5](e5-orders-payments.md).
- Making the fallback label configurable. Unlocked by D-2 but not required by any client item.
- **F-3** multi-address book.
