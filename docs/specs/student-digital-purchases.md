---
title: Student digital purchase protection
issue: https://github.com/abak-academy/platform/issues/167
status: accepted
created: 2026-09-29
sources: ["backend/internal/service/store.go", "backend/internal/repository/order.go", "backend/internal/worker/outbox.go", "docs/wiki/domain/orders-and-payments.md"]
---

# Student digital purchase protection

## Problem and outcome

A student can be charged for a course/exam entitlement they already own or are buying through another checkout. Different SKUs can point to the same target; fulfillment silently skips an existing enrollment/registration. Student self-purchase must reject conflicts before creating a payment, including concurrent checkouts, while preserving physical purchase behavior.

## Agreed behavior

- Digital product types are `course` and `exam`, each quantity one; physical types `book`, `merchandise`, `medal` keep their existing stock, quantity, shipping, and fulfillment behavior. Mixed carts are allowed.
- A student's live course enrollment or non-revoked exam registration, or a live digital purchase for any linked target, makes an entire digital product ineligible. Compare target IDs, not SKU IDs. No partial bundle price or grant.
- Cart-add and any item replacement/update that could introduce a digital target provide early validation, but checkout is authoritative. A cart is not a reservation.
- If any target is ineligible or repeated across cart lines, checkout rejects the entire order before gateway interaction or durable stock decrement. Return a stable, actionable conflict to the client.
- Checkout atomically reserves each `(student, target kind, target ID)` for digital lines; a paid/fulfilled purchase keeps its claim while access remains live. Cancelled, expired, or refunded orders release reservations, with refund revocation atomic with release. Revoked/refunded course access can be bought again.
- Payment retries on expired orders must revalidate/reacquire claims before another gateway request. Late success for an expired/cancelled/refunded order must not resurrect it or fulfill entitlements; record/flag it for reconciliation or manual refund. Free checkouts follow the same uniqueness rules.
- The targets that checkout reserves are the targets fulfillment grants even if an administrator later edits product links. Previously placed orders without a snapshot must remain fulfillable.

## Technical contract

- Use a durable Postgres claim per student/entitlement target, with transactionally enforced uniqueness for live claims and an order/item reference. Do not rely on unlocked eligibility reads, Redis locks, or UI checks for correctness. Validate existing access as well as claims.
- Resolve and snapshot digital target links while creating claims in the same checkout transaction as `cart → payment_pending` and physical stock changes. Roll back the full transaction on conflict. Paid orders retain their live claim until access is revoked/refunded; claims are not just short-lived payment locks.
- Synchronize claim release with guarded order lifecycle transitions (expiry, cancellation, refund), and guard payment/webhook and async fulfillment against late transitions. Do not broaden physical inventory/shipping rules beyond necessary atomicity.
- Existing active/pending orders need a safe migration/backfill strategy before the new uniqueness guarantee can be asserted for historical transactions; detect conflicts instead of silently discarding them. Only add migration files and local test fixtures here; no shared/live data migration or deployment without explicit authorization.
- Business decisions live in service, persistence/constraints in repository/DB, HTTP conflict mapping in handler. Preserve authz by deriving the recipient from the authenticated student/order, never accepting a recipient from the request.

## Acceptance and proof

1. Existing ownership or an earlier pending/paid purchase rejects same-SKU, different-SKU, and any-target-of-bundle purchases; focused DB-backed service/handler tests must fail without this guard.
2. Concurrent separate student carts/checkouts for the same target cannot both reserve/pay; assert one committed claim and no gateway request for the loser.
3. Duplicate targets among cart lines reject at mutation or checkout before payment, including mixed physical+digital rollback; existing physical-only tests stay green.
4. Expiry/cancellation/refund/revocation allow legitimate repurchase; retry and delayed webhook/outbox events cannot bypass the invariant. Exercise free orders and product-link edits between checkout and fulfillment.
5. Targeted Go tests and repository backend pipeline pass, or blockers are reported accurately. Frontend typecheck, tests, build, and relevant browser behavior are verified if the UI changes.

The original issue included a school marketplace and exam bulk ordering. Those acceptance criteria moved to #192; #167 alone does not prove cross-flow protection against school/bulk checkouts.

## Scope

- In scope: student cart/checkout, reservation persistence and lifecycle, payment retry/webhook safety, fulfillment snapshot, focused client conflict handling, migration files and tests.
- Out of scope: admin-school marketplace, participant pricing/fulfillment, changing existing exam bulk behavior, shipping redesign, partial bundle repricing, and running a live migration/deployment.
- Residual risks: existing orders may contain duplicate active targets and require an audited rollout; gateway settlement arriving after release may require a manual refund because `PaymentClient` has no refund operation. #192 must adapt school/bulk paths to the same claim invariant before cross-flow safety can be claimed.
