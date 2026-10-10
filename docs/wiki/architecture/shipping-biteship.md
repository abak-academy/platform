---
title: Shipping (Ongkir) & Biteship Integration
updated: 2026-10-10
verified_against: b16975af
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
  `ReloadLogisticsClient` (`service.go:147`) so a key change in admin config takes effect without
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
  `GetOrder("")` — flagged `TODO: uncertain` in-code (`biteship.go:327-334`,
  `shipping_order.go:198-206`): the echoed field name and the tracking-endpoint response shape are
  unverified against a real Biteship response.
- Manual escape hatch: `POST /admin/orders/:id/ship-manual` → `AdminShipOrderManual`
  (`shipping_order.go:220`) — admin-entered tracking number, no Biteship call,
  `waybill_source='manual'`.

## Cancel

- `POST /admin/orders/:id/shipment/cancel` → `AdminCancelShipment` (`shipping_admin_actions.go:45`).
- Requires an existing `biteship_order_id` (`orderWithBiteshipBooking`, line 58; else
  `ErrNoBiteshipBooking`, 404-mapped).
- Cancels at Biteship (`CancelOrder`, `biteship.go:282` — sends
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

## Packing slips

- Both endpoints require `orders:write`; the bulk route is registered before `/:id`
  (`backend/internal/server/routes.go:180`). They return `200 application/pdf`, keep the
  packing-slip footer, and change no order or shipment state.
- Single: `GET /api/v1/admin/orders/:id/label?size=roll80` →
  `GetShippingLabelSized` (`backend/internal/service/shipping_label_html.go:499`). An absent
  size defaults to `100x150`; an unknown size is `400 invalid_request`. Single print requires
  only a tracking number (`422 no_tracking_number` otherwise), including for failed shipments.
- Bulk: `POST /api/v1/admin/orders/labels`, JSON
  `{"order_ids":["<first uuid>","<second uuid>"],"size":"roll80"}` → `GetShippingLabels`
  (`shipping_label_html.go:448`). Size is required. Empty lists, more than 100 ids, duplicate
  UUIDs, malformed ids, and unknown sizes return `400 invalid_request`.
- Bulk eligibility: the order exists, `tracking_number` is non-empty (manual or Biteship), and
  `isShipmentFailureStatus` is false (`shipment_status.go:61`). The shared failure rule matches
  rejected, courier_not_found, cancelled, return_in_transit, returned, and disposed in snake_case
  or camelCase. `on_hold`, nil, and empty shipment statuses remain printable with a tracking number.
- The service re-reads every order through sequential `GetOrderByID` calls. Any ineligible id
  aborts the whole batch before rendering: `409 labels_not_printable` with
  `details: [{"order_id":"<uuid>","reason":"shipment_failed"}]`; reasons are `not_found`,
  `no_tracking_number`, or `shipment_failed`.
  Eligible batches make one Gotenberg call, with one slip per page in request order. Concurrent
  bulk requests share one process-wide render slot; a waiter gives up when its request context ends.
- Both sizes share the same template and styles, with an explicit Liberation Sans / Arial /
  Helvetica / sans-serif font stack, thin rules, and whitespace. The reading order is courier/service,
  waybill text and Code128 barcode, recipient, sender, full order UUID as text with quantity/
  weight, items, shipping cost, and the packing-slip footer with optional sender email
  (`shipping_label_html.go:55`). The waybill PNG includes ten white modules on each side.
  Panca's v3 decision removes the UUID barcode to give the recipient address and header more room.
- The left header embeds a courier logo from `backend/internal/service/assets/couriers/`,
  mirroring the web catalog for jne, sicepat, lion, tiki, and anteraja. Lowercased stored
  `CourierCode` selects the SVG/PNG data URI; nil, empty, unknown, or missing assets use the
  courier name as bold text (`shipping_label_html.go:339`). The asset equality test resolves
  paths from its own source directory and compares all five copies byte-for-byte to
  `web/public/couriers/` (`shipping_label_html_test.go:676`). Gotenberg fetches no images.
  The right header keeps the canonical Abak mark and hardcodes `abak academy` on one line and
  `hub.abakacademy.id` under it. Print navy/teal supplement size, weight, and position;
  CSS darkens both images for 1-bit output without modifying source assets. Recipient type is
  12 pt bold name, 9.5 pt phone, and 10 pt address with 13 pt leading; the compact sender uses
  8.2 pt bold name/phone and 7.8 pt address with 10 pt leading.
- `100x150` keeps 100 × 150 mm pages and distributes spare space between sections. `roll80`
  keeps an 80 mm paper width and 78 mm printable container with inner padding giving 73 mm text
  width and a 70 mm waybill barcode. Inline browser measurement assigns
  each slip a named CSS page with its own fitted height (`shipping_label_html.go:121`); the batch
  remains one PDF. Both sizes cap the printed item list at five and state the omitted count while
  keeping totals over all physical items.
- The orders list selects only rows on the active page; query or pagination changes clear the
  selection. `ShippingLabelReviewDialog` presents selected/ready/skipped counts first, size radio
  cards explaining fixed sticker and fitted thermal-roll pages, then buyer names and short order ids
  with printable waybills or specific skipped reasons. Skipped orders appear first; the footer
  download action states the printable count and remains reachable while the body scrolls.
  Only printable ids are sent in original row order. A 409 keeps the review open with fresh reasons
  and a review-again message; zero printable disables download. Pending blocks duplicate downloads
  and close; download failures offer retry inside the dialog.
- Single print closes `OrderDetailModal` and opens this same review in single-order mode through
  the orders page. It uses the existing GET helper and tracking-number-only eligibility, so failed
  shipments still print. The dialog defaults to `100x150`, does not persist size, traps keyboard
  focus, and returns focus to the list action or the order-row button after single print.
- `TestShippingLabelRender_PageGeometryAndContent`
  (`backend/internal/service/shipping_label_render_gate_test.go:23`) checks page boxes, page
  counts, different roll heights, both addresses, the waybill barcode, and a footer on each page against
  real Gotenberg. Its ink check locates the waybill text anchor and inspects the band below it.
  `deploy/pipeline/backend-render-gate.sh` includes its
  `TestShippingLabelRender_` prefix. The revised gate is compiled locally but awaits CI execution;
  local Chromium output is separate evidence. Physical barcode scanning and roll feed/cut remain
  unverified.

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
  buttons (shown only when `order.tracking_number` is set), print button (hands off to `ShippingLabelReviewDialog` in single mode via `onPrint`), shipment status badge
  with a failure flag (`isShipmentFailure`), `ShipmentTimeline` render.
- `app/(admin)/admin/orders/page.tsx` / `components/admin/ShippingLabelReviewDialog.tsx` —
  page-local selection and a review before downloading one packing-slip PDF.
- `components/admin/ShipOrderModal.tsx` — auto-book vs. manual-mode toggle.
  `components/admin/CancelShipmentModal.tsx` — free-text reason, required.
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
`backend/internal/handler/errors.go`; `ErrInvalidLabelRequest` → 400 `invalid_request`;
`ErrLabelsNotPrintable` → 409 `labels_not_printable` with per-id details;
`ErrInvalidSignature` → 401 (webhook rejection).

## Unverified

- `TrackWaybill`'s response shape (`biteship.go:322-333`) — Biteship's docs page for this endpoint
  renders client-side; parsing degrades to an empty log on an unrecognized shape rather than
  failing loudly.
