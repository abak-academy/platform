# E6 — Shipping & logistics: design

**Date:** 2026-07-30 · **Branch:** `feat/e6-shipping-logistics` · **Epic:** [`e6-shipping-logistics.md`](e6-shipping-logistics.md) · **Issue:** [#60](https://github.com/abak-academy/platform/issues/60)

Covers all six E6 tracks in one branch. `shipping-estimate-flag.md` is Track B — the
same item, not a parallel one.

## Credentials

Both a Biteship **sandbox** and a **production** key are available. Sandbox is not a
separate host: per Biteship's docs, a staging environment behaves identically except
that *"the courier will not pick up your request"*. `ResolveShippingClient`'s hardcoded
`https://api.biteship.com` therefore needs no change — the key alone selects the
environment, and order creation gets a full dry run for free.

## Ordering

```
A rate audit ──► B D-2 is_estimate ──► C NF-1 Level 3 ──► D FB-11 label
                                  └──► E D-6 webhook tests ──┘
F D-4 one definition — independent
```

Two forced dependencies:

- **B before a real key goes live in prod.** Once real carrier names flow, the badge's
  string check is all that separates a buyer from reading a guessed price as a quoted
  one, and it breaks silently the day a carrier name collides with the fallback label.
- **C before D.** The label is built from the Biteship waybill; building it from the
  manually-typed number first is throwaway work.

**Pre-commit gate.** E6 requires the PRD scope-reversal sign-off recorded *before the
first commit*: `docs/decisions/2026-07-29-logistics-level-3-signoff.md`, plus an
amendment to `prd-trd-drift.md`. This is commit 1.

---

## Track A — make the rate path observable

Three distinct failures currently land on one indistinguishable flat rate: `app_kode_pos`
unset, API key rejected, route unserved by any courier. The behaviour of
`resolveShippingRates` is correct and stays — a flat rate is a legitimate answer. What
changes is that the *reason* stops being invisible.

- Degradation logs at `Warn` with a discriminated cause: `origin_unset`,
  `auth_rejected`, `route_unserved`, `client_noop`.
- `BiteshipClient.GetRates` currently flattens every non-2xx into a generic
  `status %d` error. It gets typed errors so the cause survives to the log line.
- No metrics counter. One honest log line per degraded quote is proportionate at this
  volume.

**Also fixed here:** `buildBiteshipRequest` hardcodes `"value": 1`
(`internal/adapter/biteship.go`). Harmless for a rate quote, wrong for order creation
where it drives insurance valuation. Item value carries the real line price from now on
so Track C inherits a correct payload. This is scope creep against a strict reading of
E6, included deliberately because Track C is wrong without it.

**Deliverable:** a real sandbox quote with `shipping client resolved source=db` logged,
and a deliberately broken key producing a visible `auth_rejected` warning instead of a
silent flat rate.

---

## Track B — D-2, persist `is_estimate`

Migration `0046`, one column plus the backfill in the same file:

```sql
ALTER TABLE orders ADD COLUMN is_estimate BOOLEAN NOT NULL DEFAULT false;
UPDATE orders SET is_estimate = true WHERE selected_courier IN ('Ongkir Flat', 'Flat');
```

Then `model.Order`, the INSERT/SELECT in `internal/repository/order.go`, and one line
inside the loop in `PatchCart` that already has the matched rate in hand:

```go
repoPatch.IsEstimate = rate.IsEstimate
```

**Subtlety that must not be missed:** `repoPatch.IsEstimate` also has to be seeded from
`order.IsEstimate` in the struct literal, exactly as `SelectedCourier` already is.
Otherwise an address-only patch on a physical order silently clears the flag.

Frontend: `Order.is_estimate?: boolean` in `web/lib/types.ts`; `ShippingInfo.tsx` reads
it; **both** `ESTIMATE_COURIERS` sets are deleted (storefront + `OrderDetailModal.tsx`);
`LegacyFallbackCourier` is deleted from Go.

Making the fallback label configurable is unlocked by this but explicitly **out of
scope** — no client item requires it.

---

## Track C — NF-1, Biteship Level 3

### Port

`LogisticsClient` grows two methods; `NoopLogisticsClient` returns
`ErrShippingUnavailable` for both:

```go
CreateOrder(ctx context.Context, req CreateShipmentRequest) (Shipment, error)
GetOrder(ctx context.Context, biteshipOrderID string) (Shipment, error)
```

### Config

Sender details need **no new keys** — `app_name`, `app_contact_phone`, `app_address` and
`app_kode_pos` are already in `configKeyCatalog`. Only `biteship_webhook_secret` is new
(`group: "shipping"`, `secret: true`).

### Idempotency

`reference_id` = our order ID. Biteship's own duplicate detection (error `40002060`,
which echoes the conflicting order id) becomes the retry guard, so a double-click cannot
book two pickups.

### Schema — migration `0047`

`orders` gains:

| Column | Purpose |
|---|---|
| `biteship_order_id` | for the confirmation re-fetch |
| `shipment_status` | latest known carrier status |
| `waybill_source` | `'biteship'` or `'manual'` |

`order_shipment_events` (order_id, status, courier_waybill_id, courier_driver_name,
occurred_at, created_at) with `UNIQUE (order_id, status, occurred_at)` so a replayed
webhook is inert rather than duplicating a timeline row.

### Ship action

`AdminShipOrder` splits in two:

- **Default** — creates the Biteship order, stores `courier.waybill_id`, stamps
  `waybill_source='biteship'`.
- **Escape hatch** — a separate, explicit "input resi manual" action preserving today's
  behaviour, stamping `waybill_source='manual'`.

On failure the admin sees the **real** reason and the order stays unshipped. It does not
silently fall back to manual entry — that is the exact degradation pattern Track A
exists to eliminate, and repeating it for orders would be a regression in kind.

### Webhook — `POST /api/v1/webhooks/shipping`

Biteship publishes **no HMAC scheme**. The only mechanism available is a static custom
header configured in their dashboard. The design compensates rather than pretending
otherwise:

1. Constant-time compare on `X-Biteship-Signature` against the encrypted config value.
2. **401 when the secret is unset** — verification is never skipped. This repo already
   shipped a no-op signature verifier once (`841cd84`); fail-open is not available.
3. Then `GET /v1/orders/:id` and write the status from **Biteship's own answer**.

The payload is treated as a "something changed" ping, not as truth. That is the only
safe reading when the body cannot be cryptographically bound to the sender.

---

## Track D — FB-11, cetak resi

`internal/service/shipping_label_html.go`, mirroring `card_html.go`: Go `html/template`
→ `RenderHTML` → Gotenberg sidecar. Carries waybill, barcode, sender and recipient.

**New dependency:** `github.com/boombuler/barcode` for Code128, embedded as a base64
data URI the way `card_logo.go` already embeds images.

**Stated plainly:** this is a packing slip, not a carrier-issued label. The courier's own
scannable barcode comes from Biteship's dashboard. If the client expects this sheet to be
accepted at a drop-off counter, that expectation is wrong and needs correcting before
printing rather than after.

---

## Track E — D-6, webhook signature tests at the handler boundary

Handler-level tests for **both** webhooks. Payment gets the HMAC-rejection test that
`841cd84` never received above the service layer; shipping gets bad-secret,
absent-header, and unset-config-secret cases.

Named `TestPaymentWebhookRejectsBadSignature` and `TestShippingWebhookRejectsBadSecret`
so the gap is visible in a diff if either is ever deleted.

---

## Track F — D-4, one definition of digital-vs-physical

**Five** definitions exist, not four — `isPhysicalType` in `internal/service/store.go` is
a fifth, on the Go side.

Web collapses onto `web/lib/shipping.ts`, which already exports `isPhysicalType`. It
gains `isDigitalType` as the true complement — safe, because `ProductType` is exactly
five values. `ShippingInfo.tsx`, `CartLineItem.tsx` and
`web/app/(student)/catalog/[id]/page.tsx` all import it and drop their local copies.

A test asserts the two predicates are exhaustive and disjoint over `ProductType`, so a
sixth product type fails loudly instead of being treated as digital in the cart and
physical everywhere else.

---

## Verification

- **Before any change lands:** open `/orders/[id]` for a physical order in a browser and
  record what the "Pengiriman" block shows. E6 changes all five of those values; without
  a before-picture a regression is indistinguishable from a pre-existing defect.
- Sandbox order creation end-to-end, including the duplicate `reference_id` path.
- Webhook rejection cases exercised at the handler.
- **Exactly one** production booking to prove a real waybill and the printed label —
  confirmed with the user before it is spent.
- Backfill proven: an order placed before this branch still shows its estimate badge.

## Out of scope

- Configurable fallback label (unlocked by Track B, required by nothing).
- Promo, manual payment confirmation, order listing → E5.
- F-3 multi-address book.
