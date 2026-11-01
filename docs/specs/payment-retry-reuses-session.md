---
title: Payment retry reuses the checkout payment session
issue: none
status: accepted
created: 2026-10-05
sources: ["backend/internal/service/store.go", "backend/internal/adapter/midtrans.go", "backend/internal/service/ports_payment.go", "backend/internal/handler/errors.go", "docs/specs/payment-window-and-settlements.md", "docs/wiki/domain/orders-and-payments.md"]
---

# Payment retry reuses the checkout payment session

## Problem and outcome

`RetryPayment` calls `CreatePayment` with the order UUID as the Midtrans `order_id`, the same value checkout used (`store.go` checkout and retry both call `buildPaymentRequest(oID.String(), …)`). Midtrans Snap reserves an `order_id` when the Snap token is created, before the customer picks a method, so every retry of an order whose checkout reached Snap fails with `400 transaction_details.order_id has already been taken`. Production log, 2026-10-05 08:07–08:08 UTC: five failed retries for one order within 40 seconds. The student sees a generic failure and cannot pay again from the order page.

Outcome: a student who closed the payment popup presses retry and gets the link of the session that checkout already created. No second Snap session exists for an order from this path, so the same order cannot be paid through two sessions. Midtrans itself enforces one transaction per `order_id`.

## Agreed behavior

- After checkout or retry attaches the gateway reference, the payment session (`gateway_ref`, `snap_token`, `payment_url`, advertised expiry) is stored in Redis under one key per order. Its TTL is the time left until the session end, `earlier(session end, order deadline)`, the same value returned as `payment_expires_at`. A TTL of zero or less stores nothing.
- Retry first validates the order exactly as today (owner, `payment_pending`, inside its deadline, digital guard, `payment_deadline_too_close`). Only then it reads the stored session, so a stale entry is never served for an order that has since expired, been paid or been cancelled, or for another student.
  1. Stored session present: return it. No gateway call, nothing attached.
  2. No session and the order has no `gateway_ref` (checkout failed before a Snap session existed): create a session as today, attach the reference, store the session.
  3. No session and the order has a `gateway_ref` (Redis lost it, or the order predates this change): refuse with `payment_session_unavailable` (409). No gateway call, no state change.
- A failed write of the stored session is logged and does not fail checkout or retry: the link was just created and is valid; losing it only degrades a later retry to case 3.
- The 409 reaches the order page through the existing `err.message` fallback in `retryErrorMessage`; no frontend change.

## Technical contract

- No schema change, no port change, no adapter change, no webhook change. `order_id` stays the order UUID, so signature verification and `parseUUID(notif.OrderID)` are untouched.
- The key is derived from the order id only; ownership is established by the retry validation before the read.
- The retry idempotency cache (`idempotency:retry:…`) is unchanged.
- `docs/wiki/domain/orders-and-payments.md`: the `RetryPayment` row describes the three cases above.

## Acceptance and proof

1. Retry on an order with a stored session returns the stored token, URL, reference and expiry, and `CreatePayment` is not called.
2. Retry on an order with a `gateway_ref` and no stored session answers `payment_session_unavailable` (409) without calling `CreatePayment` or changing the order.
3. Retry on an order with no `gateway_ref` and no stored session creates a session, attaches the reference and stores it.
4. Checkout and retry store the session with a TTL greater than zero and not past the session end.
5. A stored session is not returned for an expired, elapsed, paid, cancelled or other student's order; the existing refusals apply first.
6. A failed session write does not fail the request.
7. `go build ./...`, `go vet ./...`, `go test -run '^$' ./...` and the targeted tests pass locally; DB-backed tests that cannot run without a container runtime are listed as not run and proven by CI. The error-mapping row is covered in `TestMapServiceError`.

## Scope

- Out of scope: shortening `paymentWindow` (a separate decision with its own change, to be set from measured payment latency); cancelling or superseding a Midtrans transaction; suffixed `order_id`s; detecting a second settlement for an already paid order; the retry idempotency replay, which returns only `gateway_ref` and no `payment_url`; any frontend change.
- Residual risk, Redis: production Redis has no volume or persistence (`deploy/compose/production.yml`), so a restart or recreated container drops sessions and retry answers 409 until the order expires. No double payment follows from that. Orders created before this change have no stored session and get the same 409 for at most one payment window.
- Residual risk, the link itself: the session stays payable until its end whether or not retry is pressed, as it was before this change.
- Residual risk, empty reference: case 2 assumes an order with no `gateway_ref` has no Snap session. A gateway timeout after Midtrans created the token, or a transient attach error, leaves the reference empty and the `order_id` reserved, so retry still fails with `order_id has already been taken`. This is the behaviour before this change, not a regression.
- Unverified, no sandbox key on the build host: that Midtrans lets a student reopen a Snap token after the popup was closed, or after a method was chosen and the payment failed. This must be checked on staging before merge: create an order, close the popup, press retry, open the returned `payment_url`, pay in the sandbox.
