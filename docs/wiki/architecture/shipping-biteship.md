---
title: Shipping (Ongkir) & Biteship Integration
updated: 2026-10-06
verified_against: 1b3cc1ef
---

Logistics Level 3: rate quoting, carrier booking, cancellation, and webhook-driven status sync
through Biteship. See also: [[./system-overview.md]], [[../domain/orders-and-payments.md]].

## Adapter boundary

- Port: `service.LogisticsClient` (`backend/internal/service/ports_logistics.go:104`) —
  `GetRates`, `CreateOrder`, `GetOrder`, `CancelOrder`, `TrackWaybill`.
- Real implementation: `adapter.BiteshipClient` (`backend/internal/adapter/biteship.go:26`).
- Stub: `service.NoopLogisticsClient` (`ports_logistics.go:118`) — every method returns
  `ErrShippingUnavailable`; never fabricates a rate or booking.
- Resolution: `adapter.ResolveShippingClient` (`backend/internal/adapter/shipping_config.go:16`) —
  DB `system_config.biteship_api_key` (encrypted) wins, `cfg.BiteshipAPIKey` (env/secrets.yaml) is
  dev fallback, Noop if neither set. `Service.logistics` is an `atomic.Pointer` swapped by
  `ReloadLogisticsClient` (`service.go:140`) so a key change in admin config takes effect without
  restart.
- Biteship base URL is hardcoded `https://api.biteship.com` (`shipping_config.go:22`); auth header
  is a plain `authorization: <key>` (no Bearer scheme).

## Quote (rate) flow

- `POST /orders/shipping` (no auth) → `Handler.GetShipping` → `Service.GetShippingRates`
  (`store.go:458`).
- `Service.GetShippingRates` reads `system_config` once for `biteship_services`.
  An absent/empty service list, or a config read failure, enables only `jne:reg,jne:yes,jne:oke`.
  The service puts distinct courier codes in catalog order on `ShippingQuoteRequest.Couriers`;
  `BiteshipClient.GetRates` reads origin postal code from `system_config.app_kode_pos` and calls
  `POST /v1/rates/couriers` with that list joined by commas. The adapter holds no courier policy.
  The service then keeps only rates whose courier/service pair is enabled. An empty intersection
  returns `ErrShippingRouteUnserved` and logs `route_unserved` at `Warn`.
- Duration string ("1 - 2 days", "Same day") is parsed by `parseDurationDays`
  (`biteship.go:147`) — takes the **upper** bound of a range; a string with no digits yields `0`
  and is logged at `Info` (not a hard error, since "Same day" legitimately has none).
- Rate resolution: `resolveShippingRates` (`shipping_rates.go:29`) returns carrier quotes or an
  error, with no flat-rate price:

  | Cause | Service error | HTTP / API code |
  |---|---|---|
  | `route_unserved` | `ErrShippingRouteUnserved` | 422 `shipping_route_unserved` |
  | `client_error` | `ErrShippingTemporarilyUnavailable` | 503 `shipping_temporarily_unavailable` |
  | `origin_unset`, `auth_rejected`, `client_noop` | `ErrShippingUnavailable` | 503 `shipping_unavailable` |

- Every degraded quote logs a discriminated `cause` at `Warn` (`store.go:492-502`). Only
  `client_error` includes `err`, truncated to 300 characters. Auth rejection logs no error text
  because the provider error can contain the raw response body. API messages are generic English;
  the cart translates the error code into a plain-language buyer message and disables checkout.
- `GetShippingRates` is called from two places: the standalone quote endpoint above, and again
  inside `buildCartPatch` (below) to validate/price the courier selection — no rate caching exists.

## Quote staleness & re-validation at checkout

- `PatchCart` (`store.go:730`) takes a lock-free snapshot, builds the patch (incl. a live
  `GetShippingRates` call when a courier is selected), then writes with optimistic concurrency —
  guarded by `ExpectedUpdatedAt` (`order.updated_at` as read); a conflict (`ErrOrderChanged`, HTTP
  409 `order_changed`) triggers one retry from a fresh read (`store.go:760-793`).
- `buildCartPatch` (`store.go:801`) voids a stale quote when the *effective* destination changes —
  comparing values ("patch value if sent, else persisted"), mirroring the SQL `COALESCE`, not which
  keys the PATCH body carried (`store.go:875-901`). A no-op address edit keeps a valid quote.
- Courier selection is matched by `(Courier, Service)` case-insensitively against a fresh
  `GetShippingRates` call, and the matched rate's `CourierCode`/`ServiceCode` (not display names)
  are persisted with `is_estimate = false` (`store.go:854-868`) — booking later uses these codes,
  never the display name.
- `Checkout` (`store.go:1058`) re-reads the order `FOR UPDATE` inside a transaction
  (`GetOrderByIDForUpdateTx`, `store.go:1106`) — judged against the locked row, not the
  pre-transaction read — then requires, for every physical item, `shipping_cost > 0` and a non-empty
  `selected_courier`, and refuses `is_estimate = true` (`ErrShippingRequired`, HTTP 400
  `shipping_required`, `store.go:1120-1123`). A stale flat cart stays unchanged until the buyer
  re-checks shipping and picks a real rate through `PatchCart`; the checkout toast asks them to
  re-pick. Checkout does **not** re-call Biteship; it trusts the persisted real quote. Legacy paid
  orders keep their estimate note in the admin and student views.

## Order shipment creation

- `POST /admin/orders/:id/ship` → `Service.AdminShipOrder` (`shipping_order.go:114`).
- Gates: `shippable(order)` — `status` `paid`/`processing`, or `shipped` **and**
  `isShipmentFailureStatus(shipment_status)` (dead booking gets a re-ship path;
  `shipping_order.go:48-53`); non-empty courier/service code; complete address
  (`parseShipmentDestination`, `shipping_order.go:24`); complete sender config
  (`app_name`/`app_contact_phone`/`app_address`/`app_kode_pos`).
- `bookingReference` (`shipping_order.go:63`): attempt 1 uses the bare order UUID as
  `reference_id`; a re-book gets a numeric suffix (`nextBookingAttempt`, line 78) — first attempt
  stays idempotent against Biteship's own dedup, re-books get distinct references.
- Duplicate-booking response (Biteship error `40002060`) is caught (`ShipmentAlreadyBookedError`)
  and **adopts** the existing Biteship order id via `GetOrder` rather than booking twice
  (`shipping_order.go:192-211`). If the echoed id is empty, refuses rather than calling
  `GetOrder("")` — flagged `TODO: uncertain` in-code (`biteship.go:326-333`,
  `shipping_order.go:198-206`): the echoed field name and the tracking-endpoint response shape are
  unverified against a real Biteship response.
- Manual escape hatch: `POST /admin/orders/:id/ship-manual` → `AdminShipOrderManual`
  (`shipping_order.go:220`) — admin-entered tracking number, no Biteship call,
  `waybill_source='manual'`.

## Cancel

- `POST /admin/orders/:id/shipment/cancel` → `AdminCancelShipment` (`shipping_admin_actions.go:45`).
- Requires an existing `biteship_order_id` (`orderWithBiteshipBooking`, line 58; else
  `ErrNoBiteshipBooking`, 404-mapped).
- Cancels at Biteship (`CancelOrder`, `biteship.go:281` — sends
  `cancellation_reason_code: "others"` + free-text `cancellation_reason`; the code enum's other
  values are Biteship's own fixed reasons, not something an admin's free-text reason is choosing
  between), then re-syncs via the same `syncShipmentFromBiteship` routine the webhook uses.
- Deliberately does **not** touch `orders.status` or money — refund is a separate, tracked-elsewhere
  decision (issue reference in code comment, `shipping_admin_actions.go:37-44`).

## Webhook

- `POST /webhooks/shipping` (no route-level auth) → `HandleShippingWebhook`
  (`backend/internal/handler/webhook.go:44`) → `Service.HandleShippingWebhook`
  (`shipping_webhook.go:52`).
- Auth: Biteship publishes no signing scheme — only a static dashboard-configured header,
  `X-Biteship-Signature`, compared against `system_config.biteship_webhook_secret` with
  `crypto/subtle.ConstantTimeCompare` (`shipping_webhook.go:57`). An **unset or empty** secret is
  refused before any compare runs — the empty-vs-empty case that would otherwise return "equal"
  is explicitly special-cased (in-code note references a prior fail-open regression).
- An empty body (Biteship's install-time URL validation ping) is answered as a no-op, but only
  *after* the signature check — never an unauthenticated 200 (`shipping_webhook.go:61-68`).
- The body itself is **never trusted for state** — only `order_id` is read to locate the row; status
  and any timestamp are ignored. The authoritative state comes from a `GetOrder` re-fetch
  (`syncShipmentFromBiteship`, `shipping_webhook.go:94`), shared by the webhook, the admin "refresh"
  button, and cancel — all three converge on identical rows.
- `orders.status` is **never** advanced by the webhook (a `delivered` event does not auto-complete
  an order) — only `shipment_status`, `tracking_number` (guarded: only overwritten when the
  re-fetched waybill is non-empty, `shipping_webhook.go:108-112`), and an
  `order_shipment_events` row.
- `occurred_at` is deterministic per order (`shipment.StatusUpdatedAt`, else `order.ShippedAt`, else
  `order.CreatedAt` — never `time.Now()`), so a webhook replay lands on the same
  `UNIQUE(order_id, status, occurred_at)` row instead of duplicating events
  (`shipping_webhook.go:114-130`).

## Tracking (admin dialog)

- `GET /admin/orders/:id/tracking` → `GetOrderTracking` (`backend/internal/service/shipping_tracking.go:42`).
- Tries `TrackWaybill` (works for any waybill, including manual-resi orders with no Biteship
  booking) first; on success, `view.Source = "courier"`.
- **Silent fallback found**: `shipping_tracking.go:70` — `if tracking, tErr := ...; tErr == nil && len(tracking.History) > 0`.
  On any `TrackWaybill` error (or an empty-but-successful history), `tErr`/the failure is **discarded
  with no logging** — the file imports no `log/slog` at all — and the view silently degrades to
  `Source: "local"` (the stored `order_shipment_events`). Contrast with rate-quote failures
  (`store.go`), which logs a discriminated cause on every degrade; this path has no
  equivalent signal, so an admin sees a thinner "local" history with no indication the courier call
  failed vs. was simply never queried.

## Config keys (`system_config`, group `shipping`)

| Key | Type | Notes |
|---|---|---|
| `biteship_services` | non-secret string | comma-separated `courier:service` pairs; empty/unset defaults to JNE REG/YES/OKE; empty or unknown pairs and duplicates rejected on save |
| `biteship_api_key` | secret string | encrypted at rest, `"***"` on read; env fallback `cfg.BiteshipAPIKey` |
| `biteship_webhook_secret` | secret string | unset ⇒ webhook always rejects (fail-closed) |

`shipping_fallback_flat_rate` is no longer in the managed catalog. A PUT containing it returns
400 (unknown key); stale database rows remain and are ignored by GET. No migration is needed.

Origin address for booking comes from the `app` group, not `shipping`: `app_kode_pos` (rate-quote
origin), `app_name`/`app_contact_phone`/`app_address`/`app_kode_pos` (booking sender fields) —
`backend/internal/service/system_config.go:30-53`.
Optional `app_latitude` and `app_longitude` provide the sender coordinate; `origin_coordinate`
is sent for every courier only when both are set.

## Admin UI touchpoints (`web/`)

- `components/admin/OrderDetailModal.tsx` — ship button (opens `ShipOrderModal`), track/refresh/cancel
  buttons (shown only when `order.tracking_number` is set), print-label link, shipment status badge
  with a failure flag (`isShipmentFailure`), `ShipmentTimeline` render.
- `components/admin/ShipOrderModal.tsx` — auto-book vs. manual-mode toggle; optional
  `deliveryDate`/`deliveryTime` schedule fields. `components/admin/CancelShipmentModal.tsx` —
  free-text reason, required.
- `components/admin/TrackingModal.tsx` / `components/orders/ShipmentTimeline.tsx` — render the
  `TrackingView`/`order_shipment_events` history.
- `lib/shipment-status.ts` — full Biteship status enum, `SHIPMENT_FAILURE_STATUSES` (deliberately
  excludes `on_hold`, which self-resolves), snake_case normalisation for Biteship's inconsistent casing.
- `lib/shipping.ts` — single source of truth for physical vs. digital (`isPhysicalType`:
  book/merchandise/medal), reused by cart, checkout gating, and admin order rendering.
- `app/(admin)/admin/system/config/page.tsx` — origin address in General; Biteship
  key/secret, and per-service switches in Courier Integration; Midtrans in Payment Integration.
- Cart-side: `components/cart/ShippingAddressForm.tsx`, `CourierRateList.tsx` — manual "check
  shipping cost" trigger only; no automatic/live quoting.

## Error sentinels → HTTP (selected)

`ErrShippingRequired` → 400 `shipping_required`; `ErrInvalidCourierSelection` → 422;
quote errors map as listed above;
`ErrIncompleteSchedule`/`ErrNoCarrierCode`/`ErrOrderNotShippable`/`ErrIncompleteAddress`/
`ErrNoBiteshipBooking`/`ErrNoTrackingNumber` each have a dedicated case in
`backend/internal/handler/errors.go`; `ErrInvalidSignature` → 401 (webhook rejection).

## Unverified

- `TrackWaybill`'s response shape (`biteship.go:322-333`) — Biteship's docs page for this endpoint
  renders client-side; parsing degrades to an empty log on an unrecognized shape rather than
  failing loudly.
