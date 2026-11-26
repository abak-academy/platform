---
title: Admin exam/store improvements — registrant summary, school self-service, checkout agreements
issue: none (sourced from local improvement.md notes)
status: accepted
created: 2026-11-24
sources:
  - improvement.md
  - backend/internal/service/rbac.go
  - backend/internal/server/routes.go
  - backend/internal/model/product.go
  - backend/internal/model/order.go
  - backend/internal/service/partner_orders.go
  - web/app/(admin)/admin/exam/packages/page.tsx
  - web/components/admin/ExamRegistrationsTab.tsx
  - web/components/admin/ProductModal.tsx
  - web/components/admin/ProductSpecsEditor.tsx
  - web/app/(student)/cart/page.tsx
  - web/app/(admin)/admin/system/schools/page.tsx
  - web/lib/nav-config.ts
---

# Admin exam/store improvements — registrant summary, school self-service, checkout agreements

## Problem and outcome

Three independent gaps from `improvement.md`:

1. Admins can't see where exam registrants come from (province/city/school diversity) — only a flat count.
2. `admin_exam` and `admin_store` cannot add School records themselves; only `super_admin` has `schools:write`, which blocks onboarding when a new school registers.
3. Checkout has no way to require a custom, product-specific agreement checkbox before payment.

Outcome: admins get registrant diversity on two existing screens; `admin_exam`/`admin_store` get self-service school *creation* (not edit); buyers (student and partner/mitra checkout) must accept product-specific agreement text, snapshotted per order so later edits never rewrite history.

## Agreed behavior

**Feature 1 — registrant diversity summary**

- Opsi A (`web/app/(admin)/admin/exam/packages/page.tsx`): the existing per-row registrant count stays eager and unchanged. The diversity breakdown (distinct provinsi/kota/sekolah) is **not** computed by default — it only appears once the admin sets the new provinsi/kota/sekolah filter. This avoids an unconditional `COUNT(DISTINCT …)` per exam on every list load.
- Opsi B (`ExamRegistrationsTab`, already scoped to one exam): add 3 new stat cards (Provinsi/Kota/Sekolah) beside the existing 5, the same provinsi/kota/sekolah filter dropdowns next to search, and a new "Asal sekolah" column on the roster table. Always computed — the query is already bounded to one exam's roster.
- Scope: exam registrants only, not non-exam product buyers.

**Feature 2 — admin_exam/admin_store school self-service**

- New capability `schools:create`: create + bulk import only, never update or status change.
- Granted to `admin_exam` and `admin_store`; `admin_school`/`super_admin` behavior unchanged.
- `POST /admin/schools`, `POST /admin/schools/bulk/presign`, `POST /admin/schools/bulk` accept either `schools:write` or `schools:create`. `PUT /admin/schools/:id` and `PATCH /admin/schools/:id` stay `schools:write`-only.
- Frontend: nav entry to the schools page added for `admin_exam`/`admin_store`; Edit/Deactivate actions hidden for a role that only holds `schools:create`.

**Feature 3 — checkout agreement checkbox**

- Admin configures 0..N checkboxes per product (title + body text).
- At checkout, every checkbox on every product in the order must be `agreed: true`, or checkout is rejected (400).
- The agreed text is snapshotted onto the order item at checkout time — editing a product's checkbox text later never changes past orders.
- Applies to storefront checkout (`POST /orders/:id/checkout`) and partner checkout (`PartnerCheckoutOrder` → `CheckoutPartnerOrder` → shared `Checkout`). Deliberately excluded: `AdminCheckoutBulkOrder` — deferred, not silently dropped.

## Technical contract

- New column `product.agreement_checkboxes JSONB`, same marshal/unmarshal pattern as the existing `product.specs` column (new `ProductAgreementCheckbox{ID, Title, Body}`, mirroring `ProductSpec`).
- New column `order_item.agreement_snapshot JSONB`, written only at checkout: array of `{checkbox_id, title, body, agreed_at}` for that item's product.
- `Service.Checkout` gains an `agreements` parameter; validated before the existing idempotency/payment-gateway flow, in the same place `revalidatePartnerOrder` already runs for partner orders.
- `PartnerCheckoutOrder` handler currently binds no request body — add body binding for `agreements` and forward through `CheckoutPartnerOrder` → `Checkout`.
- `rbac.go`: add `"schools:create"` to `RoleAdminExam` and `RoleAdminStore`; `RoleSuperAdmin`'s `"*"` is unaffected.
- `routes.go`: split the `/admin/schools` write group — a create-oriented group (`schools:create` OR `schools:write`) for POST/bulk endpoints, a write-only group for PUT/PATCH.
- Registrant diversity: new repository query joining registration → student → school → city → province, `COUNT(DISTINCT province_id)/COUNT(DISTINCT city_id)/COUNT(DISTINCT school_id)`, filterable by province/city/school. Reused by the roster-counts query (Opsi B, always called) and a new on-demand path for the exam list (Opsi A, only when a filter is set).

## Acceptance and proof

| Criterion | Verification |
|---|---|
| Opsi A list shows the unchanged registrant count by default; no diversity query fires until a provinsi/kota/sekolah filter is set | frontend test: no diversity fetch on mount; backend test: endpoint only computes diversity when a filter param is present |
| Opsi B shows 3 new stat cards + filter + "Asal sekolah" column, matching a seeded roster's actual distinct counts | backend repository/service test with students seeded across >1 province/city/school; frontend render test |
| `admin_exam`/`admin_store` can `POST`/bulk-import schools, cannot `PUT`/`PATCH` them | backend handler/service tests per role; frontend test hiding Edit/Deactivate for a `schools:create`-only role |
| Checkout (storefront and partner) rejects when any product's checkbox isn't agreed; succeeds and snapshots text when all are agreed; a later product edit doesn't change the stored snapshot | backend service tests: missing agreement → 400; present → `order_item.agreement_snapshot` persisted; edit-after-checkout → snapshot unchanged |
| Admin bulk exam order checkout is unaffected | existing `AdminCheckoutBulkOrder` tests stay green; new test proves it succeeds with no `agreements` payload even when the product has checkboxes configured |

Verification commands:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh
```

```bash
cd web && npx tsc --noEmit && npx vitest run && npm run build
```

## Scope

- Non-goals: non-exam product buyer diversity; `admin_exam`/`admin_store` editing or deactivating schools; agreement enforcement on admin bulk exam order checkout; checkbox text version history beyond the per-order snapshot.
- Deferred: admin bulk exam order checkout agreement enforcement — explicitly raised and declined for now.
- Residual risk: `schools:create` still lets `admin_exam`/`admin_store` see the full, unscoped school directory — pre-existing via `schools:read`, not a new regression, but worth being aware of. Opsi B's `COUNT(DISTINCT …)` is bounded to one exam's roster (acceptable); Opsi A's lazy-filter design removes the equivalent risk there.
