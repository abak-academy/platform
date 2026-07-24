# Outstanding — feat/store-catalog-ux

**Date:** 2026-07-24 · **HEAD:** `3bb18bf` · **Base:** `origin/main` @ `dc53214` · **20 commits**

All 15 planned tasks are implemented, reviewed, and committed, plus 4 fixes from the
whole-branch review and one dead-code removal. This file records what is *not* done, so
none of it is discovered later by surprise.

---

## 1. Blocking before merge

### 1.1 No visual verification has been performed

**Nothing on this branch has been looked at in a running browser.** Every change is covered
by unit tests and reviewed as a diff, but this branch is predominantly visual — a 5-column
grid, a 3:4 card aspect ratio, a sticky category rail, a specification table, an
address-summary/form toggle, and an estimate badge. Automated tests cannot confirm any of
those *look* right.

This matters more than usual here: the project has already been burned by exactly this gap
(see the `pdf-layout-needs-visual-verification` note — green byte-level tests hid a
fully upside-down certificate).

Blocked because the app requires authentication and Claude does not enter passwords.

**To close:** dev server for this branch runs via the `store-catalog-ux-web` entry in
`.claude/launch.json` (repo root). Log in, then check:

| Surface | What to confirm |
|---|---|
| `/catalog` | 5 cards per row at desktop width; book covers whole, not centre-cropped; sticky rail keeps position while the grid scrolls; Merchandise + Medali tabs list products; card shows only badge/title/price |
| `/catalog/[id]` | "Spesifikasi Produk" table renders; blank-value rows absent |
| `/cart` (physical item) | Saved address renders as a summary with **Ubah** and **Cek Ongkir**; editing then re-opening preserves per-order overrides; estimate badge appears on a flat-rate quote |
| `/cart` (exam/course item) | No qty stepper; "Produk digital dibeli 1× per akun." shown |
| `/orders/[id]` (physical) | "Pengiriman" block shows address, courier — service, ongkir, resi |

**Caveat:** the running `akademi-bimbel-api-1` container is the *old* backend (no migration
0045). Product specs will be empty until it is rebuilt, so the spec table cannot be
meaningfully checked until then. The local `web` container is a baked image and does **not**
contain this branch.

---

## 2. Verified

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...` | clean at `3bb18bf` |
| `go test ./internal/... ./integration/` | all packages green (incl. testcontainers Postgres) |
| Frontend `vitest run` | 921 passing |
| `tsc --noEmit` | clean |
| Migration 0045 up → down → up | verified on a throwaway Postgres; column returns `NOT NULL DEFAULT '[]'::jsonb`; sequence applies 34 → 36 → 45, so the gap left for unmerged PR #44 (0037–0044) is inert |
| Commit authorship | no Claude/Anthropic attribution in any of the 20 commits |
| Fabricated rates | none remain in reachable code; the unreferenced `adapter` copy deleted in `3bb18bf` |

---

## 3. Deferred — accepted, not blocking

Raised by review, judged not worth fixing on this branch.

- **`AdminListProducts` still ignores `cursor`.** The storefront now follows the cursor
  (up to 10 pages) but the admin product list remains capped at 20, so an admin cannot
  reach products a student can see. Small fix, but admin-side and out of this branch's scope.
- **`ShippingInfo` detects an estimate via `selected_courier === "Flat"`** — a magic string
  coupled to `resolveShippingRates`. `is_estimate` is not persisted on the order, so
  renaming that literal silently drops the badge from historical orders.
- **`resolveShippingRates` leaves `EstimatedDays: 0`**, so the flat-rate row renders
  "0 days".
- **Four separate definitions of digital-vs-physical on the frontend**
  (`lib/shipping.ts`, `ShippingInfo.tsx`, and inline checks in `CartLineItem.tsx` and
  `catalog/[id]/page.tsx`). A sixth product type means finding all four.
- **`ProductSpecsEditor` custom-row key** is `custom_${rows.length + 1}`, which can collide
  after delete-then-add, allowing two rows to persist with the same semantic key. Harmless
  today (the key is not used for display or lookup).
- **`ProductSpecsEditor` re-seeds on product-type change without calling `onChange`**, so
  switching type after filling specs and saving without touching a row persists the old
  type's rows. Visible in the editor, so not silent.
- **Invalid `?cursor=` on `/products`** reaches Postgres as a malformed UUID → 500 rather
  than 400. Only reachable by hand-crafted requests.
- **`UpdateItemQty` with an unknown `itemId` and `qty: 0`** now returns 204 instead of an
  error. No data impact.
- **Double `GetProduct` fetch** in `AdminUpdateProduct` when both `status` and `specs` are
  omitted. Two reads where one would do.

---

## 4. Pre-existing, surfaced but untouched

- **Digital items already in a cart at qty > 1** (created before this branch) still check
  out and overcharge — `Checkout` does not re-validate quantities, and the stepper is now
  hidden so a buyer cannot correct it downward, only remove the item. Staging-only exposure
  today since production does not exist yet. Closing it needs either a check in `Checkout`
  or a one-off `UPDATE order_item SET qty = 1 WHERE product_type IN ('exam','course') AND qty > 1`.
- **Images cannot be set for digital products.** `ProductModal` drops `image_url` for
  exam/course behind the `showStock` guard, though the backend accepts it for every type.
  Roughly a 3-line fix; deliberately left out of scope (agreed during design).
- **Promo code is never persisted through `PatchCart`** — the service handles
  `patch.PromoCode`, but nothing in the frontend sends it, so the discount is not stored on
  the order.
- **`bluemonday` flagged as indirect in `go.mod`** (`go mod tidy` would move it to direct).
  Pre-existing on `origin/main`, unrelated to this work.

---

## 5. Unrelated bug reported during this session

**Admin student list (`/admin/school/students`) errors / renders blank**, on both local and
staging. This branch never touches admin students (`git diff --stat` over the branch shows
no admin-student files).

Ruled out so far:
- nil-slice → JSON `null` — the service returns `make([]StudentResponse, 0)`, and `de70d1a`
  already fixed that class for admin list endpoints
- `initials(name)` throwing — backend `Name` is a non-pointer string, never null
- an empty-string `SelectItem` value (a known Radix throw) — none on that page
- the pagination accumulation effect — guarded by `if (!query.data) return`

Next step is the browser console error and the `/admin/students` response status; a blank
page is a thrown render error and the console names it exactly.

---

## 6. Follow-up backlog created by this branch

- Facet filtering over product specs. Spec keys are canonical from day one, so this needs
  value normalisation rather than a key clean-up.
- Real pagination for the catalog if it grows past a few hundred products; the current
  10-page cursor loop is an acknowledged stopgap.
- Multi-address book (currently one address per profile, overridable per order).
- Provisioning the Biteship key in production. Until it exists, physical checkout shows a
  flat-rate estimate — clearly labelled — or is blocked outright when no flat rate is
  configured. It no longer invents carrier quotes.
