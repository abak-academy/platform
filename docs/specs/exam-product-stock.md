---
title: Exam product stock
issue: none
status: accepted
created: 2026-10-10
sources: ["backend/db/migrations/0004_commerce.up.sql", "backend/internal/repository/order.go", "backend/internal/repository/order_lifecycle.go", "backend/internal/repository/product.go", "backend/internal/service/store.go", "backend/internal/handler/product.go", "web/components/admin/ProductModal.tsx", "web/app/(student)/catalog/[id]/page.tsx"]
---

# Exam product stock

## Problem and outcome

An `exam` product is sold on the open marketplace with no cap. `product.stock` exists for every type, but nothing reads it for exams: the admin form hides the input, add-to-cart and checkout skip exams, so the stored value is the unused default `0`.

Outcome: an admin can give an exam product a sales quota. When the quota is spent the exam cannot be bought; when no quota is set the exam stays unlimited, exactly as today.

## Current behavior (read at origin/main 6de7a213, not run)

- `product.stock` is `INT NOT NULL DEFAULT 0` (0004, never altered); every existing exam product holds `0`.
- Add to cart rejects `ErrOutOfStock` only for `isPhysicalType` (`service/store.go:550`).
- `CheckoutOrder` locks and decrements stock only for book, merchandise, medal (`repository/order.go:1053`).
- `ExpirePaymentOrderTx` restores stock only for those three types (`repository/order_lifecycle.go:31`).
- Admin form `showStock` and the catalog "Stok" line are physical-only.
- `isPhysicalType` also drives qty cap, shipping, delete-vs-archive and complete rules, so exam must not be added to it.

## Agreed behavior

Decisions (Panca, 2026-10-10): stock is a sales quota; existing exams stay unlimited (option 1a: `NULL` = unlimited, `0` = sold out); only student self-checkout consumes quota (2a); the catalog shows the count like physical products (3a). Medal-eligibility validation was discussed and is deferred (non-goal below).

- **Unlimited vs limited.** For an exam product, stock `NULL` means unlimited and a number means that many seats left (`0` = sold out). Book, merchandise and medal keep a required number.
- **Admin.** The exam form shows the stock input; empty means unlimited. The products list shows the stock column for exams (`-` when unlimited). Create and edit accept `stock: null` (or absent) for an exam as unlimited; for other types an absent stock stays `0` as today.
- **Add to cart.** A limited exam with stock `0` is rejected `ErrOutOfStock`.
- **Checkout.** Each limited exam line locks the product row (`FOR UPDATE`), rejects `ErrInsufficientStock` when stock is below the line qty, otherwise decrements. Unlimited exams are untouched. Two students racing for the last seat: exactly one succeeds.
- **Expiry.** `payment_expired` restores the seat for limited exams, once, through the existing transition. Unlimited exams stay `NULL`.
- **Student catalog.** A limited exam shows "Stok: N", and "Habis" at `0` with add-to-cart disabled, mirroring physical products. An unlimited exam shows no stock.
- **Not consuming quota (2a).** Admin bulk school orders (`CreateBulkExamOrder`) and manual grants neither check nor change stock.
- **Cancel.** Admin cancel does not restore stock (#72); exams inherit that behavior.

## Technical contract

- Migration `0069`: `ALTER TABLE product ALTER COLUMN stock DROP NOT NULL`. Up only. Existing rows are unchanged, so existing exams hold `0`: the migration must also set `stock = NULL WHERE type = 'exam'` so they stay unlimited. NULL is only ever written for `exam`.
- Model keeps `Stock int` and adds `StockLimited bool` (`json:"stock_limited"`). The repository scans `COALESCE(stock, 0)` and `stock IS NOT NULL`, so no test fixture that builds `model.Product{Stock: n}` changes (measured: the pointer alternative touches ~14 test files, ~60 literals). The repository writes `NULL` only when `type = 'exam' AND NOT StockLimited`.
- Request structs in `handler/product.go` take `Stock *int`: `nil` -> `StockLimited=false` for an exam; otherwise `StockLimited=true, Stock=*value`. A negative exam stock is rejected as validation error if physical types already reject it; mirror that, add nothing new otherwise.
- A new `tracksStock(p)` / equivalent predicate next to `isPhysicalType`: physical, or exam with `StockLimited`. Used by add-to-cart. The repository keeps its own type list (existing, commented duplication) and, for exam, treats a `NULL` stock row as unlimited.
- Layering: rules in `service`; the locked read, decrement and `NULL` mapping in `repository` (the existing chokepoint). No new table, no new error code (`ErrOutOfStock`, `ErrInsufficientStock` already mapped).
- Expiry restore: `NULL + qty` stays `NULL` in SQL, which keeps unlimited exams unlimited. Bulk exam orders must not restore stock they never took: verify whether such an order can reach `ExpirePaymentOrderTx` and exclude it (`order_participant`) if so.
- A PATCH that omits `stock` for a limited exam makes it unlimited, same as omitted stock zeroing a book today; the admin form always sends the field.

## Acceptance and proof

1. Unlimited exam (`NULL`) can be added and checked out any number of times; stock stays `NULL`.
2. Limited exam stock 1: first checkout succeeds and stock becomes `0`; a second student gets `ErrInsufficientStock`. Two concurrent checkouts for the last seat: one succeeds, one fails (DB-backed).
3. Limited exam stock `0`: add to cart returns `ErrOutOfStock`.
4. `payment_expired` of an order with a limited exam restores one seat, once; with an unlimited exam stock stays `NULL`.
5. A bulk exam order never changes stock, including when it expires.
6. Migration: existing exam products are `NULL` after it; book/merchandise/medal rows are unchanged.
7. Create/edit exam with empty stock stores `NULL`; with a number stores it; non-exam types behave as before (`update_preserve_test.go` unchanged and green).
8. Web: exam form shows the input and sends `stock: null` when empty; the catalog shows "Stok: N" / "Habis" for limited exams and nothing for unlimited; products list shows the column.
9. Gates: `deploy/pipeline/backend.sh` then `frontend.sh`, sequentially; CI is the authority where the dev box lacks Podman.

## Scope

Non-goals: medal eligibility or any medal field (deferred: needs a source of "medals a student owns" first), refund or restock on admin cancel (#72), per-cart reservations, counting bulk or manual grants against quota, a stock column for course products.

Residual risk: the admin and catalog UI are covered by vitest and CI only, not seen in a real browser; the checkout race is proven only where DB-backed tests can run.
