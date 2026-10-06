---
title: Remove the flat-rate shipping fallback
issue: none (reported in Discord by Isaac Panca, 2026-10-06)
status: proposed
created: 2026-10-06
sources: ["backend/internal/service/shipping_rates.go", "backend/internal/service/store.go", "backend/internal/service/system_config.go", "backend/internal/service/ports_logistics.go", "backend/internal/handler/errors.go", "backend/internal/handler/order.go", "web/app/(student)/cart/page.tsx", "web/components/cart/CourierRateList.tsx", "web/components/cart/SnapCheckout.tsx", "web/app/(admin)/admin/system/config/page.tsx", "web/lib/i18n.ts", "docs/wiki/architecture/shipping-biteship.md"]
---

# Remove the flat-rate shipping fallback

## Problem and outcome

When the Biteship rates call fails, `resolveShippingRates` returns one invented rate, "Ongkir Flat / Standar" (`shipping_fallback_flat_rate`, `IsEstimate: true`). A buyer can pick it, `PatchCart` stores it (`courier_code` empty), and `Checkout` accepts it. A far destination then pays a flat ~25k that no carrier quoted; the shop absorbs the difference and the order can only be shipped through `ship-manual`.

Evidence (Grafana Loki, 14 days to 2026-10-06): 67 `shipping quote degraded to fallback` warnings, **all** `cause=client_error` (the rates call itself failed); per day 09-30: 5, 10-03: 23, 10-04: 8, 10-05: 25, 10-06: 6. None is `route_unserved` or `auth_rejected`. The underlying error text is not logged, so the real failure is unknown.

Outcome: a failed or empty carrier quote never becomes a price. The buyer sees why in plain Indonesian and cannot proceed to payment until a real quote is picked.

Corrections to the report:

- `Courier is not available for scheduled delivery` (13 hits) is a **booking** error from `POST /admin/orders/{id}/ship` (admin set a pickup date and time), not a rates error. It never reaches the fallback.
- The per-service filter from #210 is not the trigger: the fallback predates it (first warning 09-30) and `route_unserved` never fires.

## Agreed behavior

Decisions (Panca, 2026-10-06): one PR; remove the fallback; keep the legacy estimate display; make stale flat carts re-pick; buyer-facing messages in plain language.

- `GetShippingRates` returns a carrier-priced list or an error. No config value can produce a price.
- Error mapping (cause labels already exist in `classifyShippingCause`):

  | cause | service error | HTTP / code | buyer message (id) |
  | --- | --- | --- | --- |
  | `route_unserved` | `ErrShippingRouteUnserved` (new) | 422 `shipping_route_unserved` | Alamat tujuan belum terjangkau kurir. Periksa kode pos atau coba alamat lain. |
  | `client_error` | `ErrShippingTemporarilyUnavailable` (new) | 503 `shipping_temporarily_unavailable` | Layanan cek ongkir sedang gangguan. Coba lagi beberapa saat. |
  | `origin_unset`, `auth_rejected`, `client_noop` | `ErrShippingUnavailable` (exists) | 503 `shipping_unavailable` | Pengiriman belum tersedia, hubungi admin. (existing `cart_shipping_unavailable`) |

  Buyer messages never carry Biteship's text; technical detail goes to the log only.
- The `degraded` warning keeps `cause` and, for `client_error` only, adds `err` (truncated to 300 chars). `auth_rejected` still logs no error text, because its error carries the raw Biteship body.
- `Checkout` refuses a physical-item order whose `is_estimate` is true with `ErrShippingRequired` (same guard as a missing courier). `ErrShippingRequired` gets its own code `shipping_required` (status stays 400) and an Indonesian toast, instead of the current English `invalid_request` message.
- A stale flat cart is not rewritten. The buyer re-checks shipping, picks a real rate, and `PatchCart` sets `is_estimate = false`.
- Config: `shipping_fallback_flat_rate` leaves `configKeyCatalog` and the admin "Integrasi Kurir" form. Existing DB rows stay; no migration. A PUT that still carries the key answers `400` (unknown key).
- Unchanged: the `is_estimate` column, the admin and student order views that show the estimate note for legacy orders, `ship-manual`, booking, tracking, webhooks, and the per-service filter from #210.

## Technical contract

- `service/shipping_rates.go`: drop `FallbackCourier`/`FallbackService` and the `flatRate` parameter. `resolveShippingRates` returns `(rates, cause, error)` where the error follows the table. Logic stays in the usecase, not in the adapter.
- `service/store.go`: `GetShippingRates` stops reading `shipping_fallback_flat_rate`; `PatchCart` sets `IsEstimate = false`; `Checkout` guard adds `|| locked.IsEstimate`.
- `service/ports_logistics.go`: remove `CourierRate.IsEstimate`.
- `service/errors.go` and `handler/errors.go`: two new errors and three mappings above, plus the `shipping_required` code.
- `web/`: `CourierRateList.tsx` loses the flat banner and the rate-level estimate badge; `cart/page.tsx` shows the message chosen by `ApiError.code` (checkout stays disabled on any rates error, as today); `SnapCheckout.tsx` maps `shipping_required`; `i18n.ts` gets three buyer messages plus `shipping_required` in id and en, and loses `cart_shipping_flat_fallback`; the admin config form drops the flat-rate field.
- `docs/wiki/architecture/shipping-biteship.md` and `docs/specs/biteship-courier-services.md` references to the fallback are updated.

## Acceptance and proof

1. Rates client returns an error: `GetShippingRates` returns `ErrShippingTemporarilyUnavailable` and no rate, even with a legacy `shipping_fallback_flat_rate` row present. Go test with a fake `LogisticsClient`.
2. Empty rates, or rates all dropped by the enabled-services filter: `ErrShippingRouteUnserved`.
3. Origin unset, auth rejected, noop client: `ErrShippingUnavailable`, and the logged `err` for auth rejection contains no response body (assert on the log record).
4. Handler maps each error to the status and code in the table.
5. `PATCH` cart with a real rate sets `is_estimate = false`; a cart that holds a flat selection is refused by `Checkout` with `ErrShippingRequired` (DB-backed service test), and after re-picking a real rate it checks out.
6. `PUT /admin/system/config` with `shipping_fallback_flat_rate` answers `400`; a `GET` with the stale row in the table still succeeds.
7. Web: each of the three codes shows its message and keeps checkout disabled; the flat banner and badge are gone from the rate list; the admin form has no flat-rate field; `page.test.tsx` and `CourierRateList.test.tsx` updated.
8. Gates: backend and frontend gate scripts, run sequentially (`deploy/pipeline/*.sh`).
9. Not provable locally: what Biteship actually returns when the rates call fails in prod. The new `err` log field answers this after deploy.

## Scope

Non-goals: fixing the cause of the 67 failures (unknown until `err` is logged); Biteship balance ("Bite points tidak cukup", 55 booking failures); rewriting or refunding existing flat orders; dropping the `is_estimate` column; per-zone tariffs.

Residual risks:

- **Checkout blocks whenever Biteship fails.** About 4-5 failed quotes per day over 14 days, with spikes on 10-03 and 10-05. This is the intended trade (no wrong price), but the real cause should be read from the new log right after deploy.
- **`client_error` is a catch-all.** It includes timeouts, 5xx, non-2xx other than 401/403, and parse failures. A persistent Biteship 4xx (for example a bad postal code) would show "coba lagi", which is misleading. The `err` field will show whether to split it.
- **Stale admin tab.** An admin form opened before deploy still PUTs the removed key and gets `400` until refreshed.
- **Legacy flat orders** already paid stay as they are; the shortfall on them is a business decision, not a code one.
- Interim, before this ships: emptying `shipping_fallback_flat_rate` in the admin config has the same effect on new quotes and is reversible.
