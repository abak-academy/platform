---
title: Biteship origin coordinate
issue: none
status: proposed
created: 2026-10-07
sources: ["backend/internal/service/system_config.go", "backend/internal/service/shipping_order.go", "backend/internal/service/ports_logistics.go", "backend/internal/adapter/biteship.go", "web/app/(admin)/admin/system/config/page.tsx"]
---

# Biteship origin coordinate

## Problem

Booking a Lion pickup through `POST /admin/orders/:id/ship` fails with Biteship 400 `Lion pickup booking require origin coordinate` (code 40002040). `buildBiteshipOrderRequest` sends only the origin postal code, never `origin_coordinate`, so every Lion booking fails until the request carries one. Prod logs on 2026-10-06 UTC show it on two orders; one of them failed repeatedly since 2026-10-05.

## Agreed behavior

Decisions (Panca, 2026-10-07):

- The sender coordinate is two new `system_config` keys, `app_latitude` and `app_longitude`, edited by the admin on the existing config page. No geocoder, no new dependency, no migration.
- Both empty is valid and keeps today's behavior: the request has no `origin_coordinate`.
- When both are set, every booking sends `origin_coordinate` (all couriers, not only Lion).

## Technical contract

- `configKeyCatalog` gains `app_latitude` and `app_longitude` (group `app`, string).
- Validation on save: a value is empty or a decimal number, latitude in [-90, 90], longitude in [-180, 180]. When both keys are in the submitted values, they are both empty or both set. A submission carrying only one key is not checked against the stored other half.
- `CreateShipmentRequest` gains the sender latitude and longitude; `AdminShipOrder` fills them from `system_config`.
- `buildBiteshipOrderRequest` emits `origin_coordinate: {latitude, longitude}` only when both are present; otherwise the field is omitted.
- Config page: two inputs in the Couriers tab ("Integrasi Kurir"), next to the Biteship credentials, not in the address section (owner decision, 2026-10-07). The hint says it is the sender pickup point, required for Lion Parcel, and that the order is latitude then longitude. Both keys are sent by that tab's Save, and the General save no longer carries them. Labels in id and en.

## Acceptance

1. Saving `app_latitude=-6.2088`, `app_longitude=106.8456` succeeds; empty/empty succeeds.
2. Saving a non-number, a latitude outside [-90, 90], a longitude outside [-180, 180], or exactly one of the two set is rejected.
3. With both configured, the `/v1/orders` body contains `origin_coordinate` with those numbers; with both empty the key is absent.
4. `AdminShipOrder` passes the configured values into `CreateShipmentRequest`.
5. The Couriers tab loads both values and its Save sends both; the General tab does not show or send them.

## Non-goals

- Geocoding the address; clearer error mapping for Biteship 400 (still a 500 today); the two orders already stuck; the unused `app_province_id`/`app_city_id`/`app_district_id` keys.

## Verification

Unit tests for the five criteria above; `deploy/pipeline/backend.sh` then `frontend.sh`, sequentially. Not provable before deploy: that Biteship accepts the booking for Lion with a real coordinate. After deploy the admin fills the coordinate and re-ships the stuck orders.
