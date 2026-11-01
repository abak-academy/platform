---
title: Biteship courier services config
issue: none (reported in Discord by Isaac Panca, 2026-10-02)
status: accepted
created: 2026-10-02
sources: ["backend/internal/adapter/biteship.go", "backend/internal/service/store.go", "backend/internal/service/system_config.go", "backend/internal/service/shipping_order.go", "web/app/(admin)/admin/system/config/page.tsx", "https://biteship.com/en/docs/api/couriers/overview", "https://biteship.com/en/docs/api/rates/retrieve"]
---

# Biteship courier services config

## Problem and outcome

Abak always asks Biteship for `anteraja,jne,sicepat,tiki` (`adapter/biteship.go:122`, hardcoded). The Biteship dashboard ("Konfigurasi Kurir") has its own per-service activation, and the two disagree. Screenshot of the account under test: JNE 4/4, Lion 2/2, SiCepat 1/3 active; Anteraja 0/2 and TIKI 0/8 inactive. So checkout can offer couriers Biteship will not serve (Anteraja, TIKI) and never offers one it does (Lion).

Outcome: an admin chooses, per courier **service**, what buyers can pick at checkout, from a new "Integrasi Kurir" tab. Nothing is hardcoded in the adapter any more.

Not yet proven (needs a Biteship key): what Biteship returns when a requested courier is inactive in the dashboard (omitted, error, or priced anyway). The design below does not depend on the answer, because abak filters the result itself.

## Agreed behavior

Decisions (Panca, 2026-10-02):

- Toggle **per service** (e.g. `jne` + `reg`), not per courier.
- Selectable couriers are a fixed catalog: current four (JNE, SiCepat, Anteraja, TIKI) plus Lion Parcel (`lion`). Service lists follow the dashboard (Panca's screenshots): JNE Reguler/YES/OKE/JTR, Lion Reg Pack/Big Pack, SiCepat Reguler/Best/GOKIL, Anteraja Reguler/Same Day; TIKI shows 8 on the dashboard (EKO, SDS, REG, ONS, T15, T25, T60, Trucking) but abak offers only REG, EKO, ONS, SDS; the motor (T15/T25/T60) and trucking services are out of the catalog. Service codes are mapped from those names via Biteship's public courier docs and are unverified until reconciled with `GET /v1/couriers`.
- Default-on services: JNE `reg`, `yes`, `oke`; everything else off (JNE trucking `jtr` off).
- Rollout accepted: until an admin enables others, checkout offers only the default JNE services.
- No "enable all services of a courier" shortcut; one switch per service.
- Default when nothing was ever saved: only JNE on; every other service off.
- If the admin turns every toggle off the UI shows a warning that at least one must stay on, and the backend refuses the save with `400`.
- Unpaid cart whose chosen service is later turned off: unchanged, `PatchCart` re-quotes and answers `ErrInvalidCourierSelection`, buyer re-picks.
- Paid order: unchanged, it ships with the `courier_code`/`courier_service_code` already stored on the order.
- UI: new tab **Integrasi Kurir**; the current "Integrasi" tab becomes **Integrasi Pembayaran**; each courier shows its logo, and each service has an animated on/off switch (logo goes from grey/dim to full colour, 200 ms, none under `prefers-reduced-motion`).

Derived behavior:

- Buyer-facing quote = Biteship rates **intersected** with the enabled services. A service that is enabled here but not active in the Biteship dashboard simply never appears.
- If the intersection is empty, return `ErrShippingRouteUnserved` (422 `shipping_route_unserved`) and log `route_unserved` at `Warn`; see [the carrier-only quote spec](no-flat-rate-shipping-fallback.md).

## Technical contract

- **Storage:** one new `system_config` key `biteship_services` (group `shipping`, non-secret), value = comma list of `courier:service`, e.g. `jne:reg,jne:yes`. No migration, no table (`configKeyCatalog` is code).
- **Unset = default:** an empty/absent value means the default set. Saving an empty set is rejected, so "empty" can never mean "admin chose nothing"; no sentinel is needed.
- **Catalog:** a Go map in `service/system_config.go` of `courier code -> service codes`, the only source of valid values. `validateConfigKeys` rejects unknown pairs and an empty set (it only runs when the key is in the payload, so a PUT of other keys cannot trip it).
- **Where logic lives (usecase, not adapter):** `Service.GetShippingRates` reads the config, puts the distinct courier codes on `ShippingQuoteRequest.Couriers`, and filters the returned rates by `(CourierCode, ServiceCode)`. `BiteshipClient.buildBiteshipRequest` only joins `req.Couriers`; it holds no list and no policy.
- **Frontend:** the catalog is mirrored in `web/` (labels, logo path); the backend remains the validator.
- **Tab move:** the whole "Shipping" group (flat rate, `biteship_api_key`, `biteship_webhook_secret`) moves to the new tab with its own save; the API contract of `PUT /admin/system/config` is unchanged.
- **Logos:** static files under `web/public/couriers/`, taken from each courier's own site, no monograms.

## Acceptance and proof

1. Fresh install (key absent): quote request to Biteship carries `couriers=jne`, and only the default JNE services are offered. Go test with a fake `LogisticsClient` capturing `req.Couriers`.
2. Enabling `lion:reg_pack` adds `lion` to the request and its rate to the quote; disabling `jne:oke` removes only that rate. Service-level tests.
3. Rates returned for a service that is not enabled are dropped (fake client returns Anteraja; result excludes it).
4. PUT with an empty set, or an unknown `courier:service`, answers `400`; PUT of unrelated keys still succeeds.
5. Unpaid cart with a now-disabled service gets `ErrInvalidCourierSelection` from `PatchCart` (existing behavior, pinned by a test).
6. A `paid` order whose service was turned off still passes `AdminShipOrder` and books with the stored codes (new test; no quote call).
7. Web: new tab renders courier logos and per-service switches; all-off shows the warning and disables Save; the old Midtrans fields are under "Integrasi Pembayaran"; `page.test.tsx` updated.
8. Gates: repo backend and frontend gate scripts, run sequentially (`deploy/pipeline/*.sh`).
9. Not provable locally: live Biteship behavior with a real key; recorded as residual risk, to be checked once with `GET /v1/couriers` plus one `rates` call.

## Scope

Non-goals: fetching the catalog from `GET /v1/couriers`; per-seller or per-product courier rules; price markup; changing booking, webhooks or tracking; migrating existing carts.

Residual risks:

- **Service codes are mapped, not read from the API.** Names come from the dashboard, codes from Biteship's docs (`reg_pack`, `big_pack`, `gokil`, `same_day`, ...).  A wrong code means a switch with no effect (the service never appears), not a wrong quote. Reconcile with `GET /v1/couriers` once.
- **Rollout:** after deploy only the default JNE services are offered until an admin enables others; Anteraja/TIKI/SiCepat disappear from checkout at that moment.
- **Logo rights:** courier logos are trademarks used only to identify the courier; sourced from the couriers' own sites, not redistributed assets.
