---
title: Product relation edit guard
issue: https://github.com/abak-academy/platform/issues/194
status: accepted
created: 2026-10-02
sources: ["backend/internal/handler/product.go", "backend/internal/service/store.go", "backend/internal/repository/product.go", "backend/internal/repository/order_digital.go", "web/components/admin/ProductModal.tsx", "docs/specs/student-digital-purchases.md"]
---

# Product relation edit guard

## Problem and outcome

A `course` or `exam` product sells whatever `product_course` / `product_exam` links it has **at fulfilment time** (decision of #167: no snapshot on the order). Today an admin edit can silently replace those links:

- `GET /admin/products` returns no `exam_ids`/`course_ids`, and `ProductModal` seeds its checklist from that list, so an existing product opens with nothing checked.
- For an exam product `canSubmit` requires at least one exam checked, so the admin ticks one exam and saves. The form sends `exam_ids:[X]`, the handler routes to `UpdateProductWithExams`, and `ReplaceProductExams` deletes every link and inserts only X.
- The same replace runs with no order check, so it can change what an already-paid order grants.

Outcome: an ordinary edit can never touch relations; a relation change is a separate, explicit, `super_admin`-only action that is refused once any order depends on the product.

Correction to the issue text: a PATCH that **omits** the relation fields already preserves links today (`req.ExamIDs != nil` falls through to `UpdateProduct`). The wipe comes from the form sending a partial list, not from omission. A regression test still proves it (criterion 4).

## Agreed behavior

Decisions (Panca, 2026-10-02): relation change is allowed with a guard; only `super_admin`; `cart`, `payment_expired` and `cancelled` orders do not block; a PATCH carrying relations answers `400`; other PATCH intents stay unaffected.

- **PATCH `/admin/products/:id`** no longer accepts relations.
  - `exam_ids` / `course_ids` present with a non-null value (including `[]`) -> `400` (`ErrValidation`), nothing written.
  - Key absent or `null` -> treated as not sent; relations untouched. Name, price, stock, weight, image, specs, status, availability behave exactly as today.
- **New action `PUT /admin/products/:id/relations`**, body `{ "exam_ids": [...] }` or `{ "course_ids": [...] }` matching the product type; replaces the set atomically.
  - Role other than `super_admin` -> `403`.
  - Product type not `course`/`exam`, or the body key does not match the type -> `400`.
  - Empty list -> `400` (a digital product with no target is refused at checkout anyway, #167).
  - Guard (below) fails -> `409` with a stable error code.
- **Guard.** Refuse when any `order_item` for this product belongs to an order whose status is not in (`cart`, `cancelled`, `payment_expired`). Excluding statuses rather than listing them keeps a future status blocking by default, same rule as #167's live-order set. Zero such orders -> change allowed. A product that already has orders is repaired by creating a new product.
- **Admin edit view** loads `GET /admin/products/:id` (already fills `exam_ids`/`course_ids`) and shows the current relations read-only. The edit form never sends relations. `super_admin` gets a "Ubah relasi" action that calls the new endpoint; other roles do not see it.
- **Create** is unchanged (`POST` still takes relations).
- Fulfilment still resolves links at fulfilment time; the guard is what keeps that stable.

## Technical contract

- No migration, no new table or column. The guard is one `EXISTS` over `order_item` -> `orders` using the same status set as `ListLiveDigitalTargets`; reuse that definition instead of a second copy (extract a shared status predicate if needed).
- Layering: role check and guard decision in `service`, the locked read + replace in `repository`, error mapping in `handler/errors.go` (new sentinel needs a `mapServiceError` case, else 500).
- `ReplaceProductExams/Courses` stay, but are reached only from the new action. `UpdateProductWithExams/Courses` and the `CourseIDs/ExamIDs` switch in the PATCH handler are removed (7 files reference them today, mostly tests).
- **Race with checkout: accepted, not closed (Panca, 2026-10-02).** The change transaction locks the product row (`SELECT ... FOR UPDATE`), checks orders, then replaces. Checkout locks the cart row but **not** the product row of a digital line, so an order that reaches `payment_pending` between our order check and our replace is not seen and could end up pointing at the new links. Accepted because only `super_admin` can trigger it and the window is one transaction. Not tested; recorded in the PR body. If it ever matters, the fix is `FOR SHARE` on each digital line's product row in checkout and retry (#167 money path).

## Acceptance and proof

1. PATCH with `exam_ids:[X]`, `exam_ids:[]`, `course_ids:[...]` -> `400`, relations unchanged (DB-backed; fails on today's wipe).
2. PATCH with name/price/status/stock/image/specs and no relation key, and with `exam_ids:null` -> `200`, relations unchanged.
3. `PUT .../relations` as `super_admin` on a product with no orders -> replaced; with only `cart`/`payment_expired`/`cancelled` orders -> replaced; with one order in each of `payment_pending`, `paid`, `processing`, `shipped`, `completed` -> `409`, unchanged.
4. Other roles (`admin_exam`, `admin_store`, `admin_school`, student) -> `403`.
5. Type mismatch, empty list, unknown id -> `400`/`404` as the existing mapping dictates.
6. Two concurrent relation changes on one product serialize on the product row (last writer wins, no partial set). The race with checkout is a documented, accepted gap (see Technical contract), not a criterion.
7. Web: edit view shows current relations; saving an edit sends no `exam_ids`/`course_ids`; `super_admin` sees the action, others do not.
8. Updated existing tests: `update_preserve_test.go`, `store_test.go`, `product_availability_test.go`, `exam_test.go`, `course_handler_test.go`, `product_specs_handler_test.go` move off PATCH-with-relations.
9. Gates: `deploy/pipeline/backend.sh` and `frontend.sh` run sequentially; CI is the authority where the dev box lacks Go/Podman.
10. `docs/backlog/open-issue-map.md` entry for #194 updated in the same PR.

## Scope

Non-goals: snapshotting links on the order, per-order reservations, #192 (school/bulk and manual grants), #185 (modes), editing relations of products that already have orders, a bulk relation tool, changing create.

Decided (Panca, 2026-10-02): the checkout race stays open (option b above); an admin tab opened before deploy that still sends `exam_ids` gets `400` until reload, with no compat shim.

Residual risk: the checkout race above, and the admin UI, `409` copy and the "Ubah relasi" interaction are not exercised in a real browser; only vitest and CI cover them.
