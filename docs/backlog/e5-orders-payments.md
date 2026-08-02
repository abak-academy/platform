# E5 — Orders & payments

| | |
|---|---|
| **Issue** | [#57](https://github.com/abak-academy/platform/issues/57) |
| **Objective** | An admin can tell who bought what and prove how it was paid; and the money the buyer is shown is the money the buyer is charged. |
| **Source IDs** | FB-15, FB-19, FB-19a, FB-19b, FB-19c, NF-2 + B-1…B-7, D-3 (`payment_method`) |
| **Unscheduled, same surface** | FB-29 — the "Cara pembayaran" button has no handler and the page does not exist; needs client copy or deletion. [H1](h1-live-bugs-2026-07-30.md) |
| **Client items** | 6 |
| **Depends on** | — |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

Everything here is about **money and its record**. Physical fulfilment — couriers, waybills, labels,
tracking — is [E6](e6-shipping-logistics.md).

This epic carries the largest cluster of pre-existing bugs, and one of them is live on production in a
money-losing direction.

---

## 1. B-1 + NF-2 — promo is not "off", it is broken

The client's ask reads *"aktifkan fitur promo code"*, as if the feature were switched off. It is not.

**The backend is correct.** `PatchCart` resolves the code and writes the discount
([`store.go:723`](../../backend/internal/service/store.go),
[`order.go:382-386`](../../backend/internal/repository/order.go)).

**The frontend never sends it.** The cart calls `validatePromo` for display only
([`cart/page.tsx:309`](../../web/app/(student)/cart/page.tsx)), and **both** `patchCart.mutate` call
sites omit `promo_code` even though `usePatchCart` supports it — `handleSaveAddress` (`:79`) and
`persistShipping` (`:134`).

**So the buyer sees a discount and pays full price.** That is live on production today.

**Second-order defect, same pass.** When `patch.PromoCode` is nil, `repoPatch.PromoCodeID` is left nil
and the UPDATE writes `promo_code_id = NULL` while carrying `discount` forward. Once B-1 is fixed, a
later shipping patch silently detaches the promo — the money comes off the total but
`IncrementPromoUses` at checkout is skipped, and **`max_uses` stops being enforced**.

And `persistShipping` fires on **every courier selection**, so that detach is the normal path through
checkout, not an edge case.

Fix both halves, then — and only then — build the new part.

**The new part (NF-2)** — list currently-active promos at checkout. No FR covers it; FR-STORE-06 only
covers *applying* a code. Today only `POST /promo-codes/validate` exists
([`routes.go:84-85`](../../backend/internal/server/routes.go)), so this needs a new public endpoint.

**Visibility is opt-in — decided 2026-07-30.** Listing every active code would expose staff and
partner codes, so the endpoint returns only codes explicitly published:

```sql
ALTER TABLE promo_code ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT false;
```

Default `false` means nothing leaks the moment the endpoint ships, including every code that already
exists. The client opens codes one at a time, and the choice is reversible per code. The admin promo
form needs a toggle for it.

Note this is the **only** field controlling public listing — an expired or exhausted `is_public` code
must still be filtered out by the same rules `ValidatePromo` already applies, or the checkout page
will advertise codes that fail on use.

---

## 2. FB-19 — show the buyer's name, not the id

The list renders a truncated UUID
([`orders/page.tsx:32`](../../web/app/(admin)/admin/orders/page.tsx)):

```ts
function buyerLabel(order: Order): string {
  return `...${order.student_id.slice(-12)}`;
}
```

and the detail modal shows the raw UUID (`OrderDetailModal.tsx:151`). Join the name through. Keep the
id available — just not as the primary label.

---

## 3. FB-19a / FB-19b / FB-19c — manual confirmation, with proof

**FB-19a/b — nothing of the kind exists.** Manual confirmation needs an uploaded payment proof, and a
way to view it from the order afterwards, with the order clearly marked as manually confirmed. The
presign pattern in [`ImageUploadInput.tsx`](../../web/components/admin/ImageUploadInput.tsx) already
does the upload half.

> **Wire the dead column rather than adding a parallel one.** `orders.payment_method` is a live column
> that **nothing ever writes** (D-3), with dead frontend code branching on it. Manual-confirm-with-proof
> is the feature it was waiting for.

**FB-19c — largely already true.** The audit row is written inside the transaction
([`store.go:1193`](../../backend/internal/service/store.go)):

```go
s.storeRepo.InsertAuditLogMeta(ctx, tx, actor, "order", id.String(), "order.confirm", …)
```

The work is verifying it, extending the metadata to carry the proof reference, and **surfacing it** —
an audit row nobody can read is not an audit trail.

---

## 4. FB-15 — the checkout address overlaps Cek Ongkir

**Very probably already fixed.** `9e309fd` (PR #54) made the button save the address, gave it auto
width and right alignment, and moved the "incomplete" hint beside it instead of under a full-bleed bar.
Production runs `4d69591` — **6 commits behind `main`**.

**So the first action is not a fix: deploy current `main` and re-check.** If it still reproduces it
becomes real work. If it does not, the finding is F-5 (no CD), not a layout bug — and that is worth
saying out loud, because until F-5 lands every "still broken" report carries the same ambiguity.

---

## 5. Absorbed bugs

| ID | Summary | Detail |
|---|---|---|
| **B-4** | Reconcile queries the gateway with an empty ref | `s.payment.QueryStatus(ctx, "")` at [`store.go:1359`](../../backend/internal/service/store.go). The order is then flipped to `paid` on the strength of a query that never identified which order it was asking about. Pass the order's `gateway_ref`; refuse when it is empty. Same surface as manual confirmation. |
| **B-2** | Admin product list capped at 20 | `handler/product.go` builds the admin filter without `Cursor:` while the public path a few lines above includes it. The repository returns a `next_cursor` the admin UI can never spend. **One line.** |
| **B-6** | Malformed `?cursor=` returns 500 | [`repository/product.go:151-155`](../../backend/internal/repository/product.go) appends `AND id > $n` with the raw query string, so a non-UUID reaches Postgres and errors. Should be 400. Fix with B-2 — same query builder. |
| **B-3** | Digital products can't have an image | `image_url` is gated behind `showStock` in [`ProductModal.tsx`](../../web/components/admin/ProductModal.tsx) (true only for `book \| merchandise \| medal`) although the backend accepts it for every type. **Visible on production now** — exam and course cards render a placeholder. ~3 lines. |
| **B-5** | Checkout commits `payment_pending` before the gateway call | [`store.go:974`](../../backend/internal/service/store.go) commits, `:980` calls `CreatePayment`. A gateway failure leaves a durable `payment_pending` order with no `gateway_ref`. This happened on 2026-07-22 when Midtrans rejected the request; recovery depended entirely on Continue Payment. Either move `CreatePayment` inside the transaction (holding it open across a network call) or accept the split and document `RetryPayment` as the recovery. |
| **B-7** | Legacy digital rows at qty > 1 overcharge | `AddToCart` and `UpdateItemQty` both guard (`ValidateItemQty` at `store.go:480` and `:567`) but **`Checkout` never re-validates**, and the qty stepper is hidden for digital items — so a pre-guard row can only be removed, not corrected downward. Closes with:<br>`UPDATE order_item SET qty = 1 WHERE product_type IN ('exam','course') AND qty > 1;` |

---

## 6. Deferred, same surface — F-3, F-4

Carried from the old register. **Not scheduled**, and neither is a client ask — but both live on
pages this epic already opens, so they belong here rather than in a list of their own.

**F-3 — multi-address book.** A student has exactly one address, stored on the user row
(`address TEXT`, [`0002_identity.up.sql:6`](../../backend/db/migrations/0002_identity.up.sql), with
the hierarchy reference fields added in `0030_user_biodata_changes`). Orders snapshot it into
`orders.shipping_address` JSONB at checkout
([`0004_commerce.up.sql:41`](../../backend/db/migrations/0004_commerce.up.sql)), so historical orders
are already safe from an address book being introduced later — the snapshot is the record. What is
missing is *choosing between saved addresses* at checkout.

**F-4 — catalog facets and real pagination.** The storefront does not paginate; it drains the cursor
into one array, up to a hard ceiling
([`web/lib/hooks/products.ts:13,22`](../../web/lib/hooks/products.ts)):

```ts
const MAX_PRODUCT_PAGES = 10;
…
for (let page = 0; page < MAX_PRODUCT_PAGES; page++) {
```

Past ten pages products **silently disappear from the catalog** with no error and no "load more".
That is a stopgap with a real failure mode, not just a missing feature.

Facets are the larger half: they need spec-value normalisation first, because the values are
free text today and faceting on unnormalised strings produces one bucket per typo.

> Do B-2 and B-6 first regardless — they are in §5, they are small, and all three touch the same
> query builder. F-4 without them just moves the cap.

---

## 7. Verification debt

Shipped without ever being looked at in a running browser. The `/cart` physical row is this epic's
own work; the other three are older store-catalog surfaces that landed with zero visual verification
and have no better owner.

| Surface | What to confirm |
|---|---|
| `/cart` physical | Saved address renders with **Ubah** + **Cek Ongkir**; per-order overrides survive edit and reopen; estimate badge on the flat-rate quote |
| `/catalog` | Sticky category rail holds while the grid scrolls; Merchandise and Medali tabs list products |
| `/catalog/[id]` | "Spesifikasi Produk" table renders; blank-value rows absent |
| `/cart` digital | No qty stepper; "Produk digital dibeli 1× per akun." shown |

---

## Acceptance

- Applying a promo produces an order whose `total` **and** `promo_code_id` both reflect it — after a
  subsequent shipping patch, not just immediately.
- `max_uses` is enforced: the (N+1)th use is refused.
- The active-promo list shows only `is_public = true` codes, and an expired or exhausted public code
  does not appear.
- Order list and detail show the buyer's name.
- A manually confirmed order carries a visible "confirmed manually" mark, the proof image opens, and
  an audit row references it.
- Reconcile refuses an order with an empty `gateway_ref`, and passes the real ref otherwise.
- The admin product list pages past 20; a junk cursor returns 400.
- An exam or course product can carry an image, and it renders in the catalog.
- Cart and checkout re-checked in a browser against current `main` **before** any layout change is
  written.

## Out of scope

- Couriers, rates, waybills, labels, tracking → [E6](e6-shipping-logistics.md).
- Refund semantics per product type — undefined since the API review (open item #5) and still
  undefined. Not opened here.
- **F-3** and **F-4** are recorded in §6 but are **not** part of this epic's acceptance.

## Verification log 2026-08-01

Driven in a real browser against the shared local stack (dev `api-1` + `web-1` Docker containers on
this branch, port 3000/8080). `web-1`'s image was built 2026-08-01 11:52 WIB from this branch and was
confirmed (via `docker exec … grep` for "Spesifikasi Produk" / "Cek Ongkir" / "dibeli 1×" inside
`/app/.next`) to already contain the strings under test, so it was reused in place of `npm run dev` —
stopping the shared `web-1` container to free port 3000 was blocked by the session's own permission
guard (shared infra, another implementer's session depends on it) and was not forced. No `web/.next`
was created and no docker container was touched. Logged in as a freshly-registered throwaway student
(`sf-verify-e5-task5@example.test`, synthetic — not real PII) since no student fixture existed;
OTP read from `docker logs akademi-bimbel-api-1` (`[noop-otp]` line). Test cart items removed after
each check. Four other implementers were committing to this same branch/worktree throughout the
session, so `HEAD` moved during the run (observed range `0c6d441`..`cf263dc`); shas below are the
commit at the moment each check was performed.

| # | Surface | URL | Viewport | Commit | Verdict |
|---|---|---|---|---|---|
| 1 | FB-15 (address block vs Cek Ongkir) | `/cart` | 1280×720 desktop; ~585×1267 and ~462×844 (mobile-preset / 390px requested — the Browser-pane tool reported `window.innerWidth` as 585/462 instead, a tool-side DPR quirk, noted so the number isn't taken as exact) | `0c6d441`–`cf263dc` | **Not reproduced.** Checked both the open (unsaved) address-edit form and the collapsed saved-address summary. Zero overlaps found between any visible `<label>` and any visible `<button>` (`getBoundingClientRect()` intersection test run over every label/button pair at each width) — labels/inputs sit at a stable `left:129` with no button ever entering that rectangle. Closes as **F-5 (no CD)** per the brief's pre-agreed disposition: production runs `4d69591`, current `main` already carries PR #54's fix. |
| 2 | `/cart` physical | `/cart` | 1280×720 | `cf263dc` | **Confirmed.** Saved address renders collapsed with **Ubah**; courier list renders under "Pilihan Pengiriman". Selected courier (Tiki · Same Day Service, Rp60.000) survived (a) reopening the address edit form without saving, and (b) a full page reload — Ringkasan Pesanan still showed Ongkos kirim Rp60.000 / Total Rp70.000 after reload, confirming the per-order override is server-persisted, not just client state. Flat-rate fallback forced with an unroutable postcode (`99999`): banner "Kami tidak menemukan tarif kurir…" appeared and the fallback option rendered as "Ongkir Flat · Standar" carrying the badge **"Estimasi — bukan tarif kurir"** — the estimate badge is present. |
| 3 | `/cart` digital | `/cart`, `/catalog/af0d4721-cebc-425d-9aff-516b9cd2297c` | 1280×720 | `cf263dc` | **Confirmed.** The digital item ("Tryout TIK", an exam product) shows no qty stepper in the cart — only a delete icon — and "Produk digital dibeli 1× per akun." renders inline. Same text/no-stepper behavior also holds on the product detail page itself. |
| 4 | `/catalog` | `/catalog` | 1280×720 (also tried 1280×450 and mobile widths to force scroll) | `cf263dc` | **Partially confirmed / data-limited.** The category rail's computed `position` is `sticky` (verified via `getComputedStyle`, active above the Tailwind `md` breakpoint at this viewport) — but visual scroll-independence could not be confirmed because the local catalog has only 4 products total (3 Buku + 1 Ujian), not enough content to scroll the grid past the rail at any width tried. **Merchandise and Medali tabs both list zero products** — confirmed as a data gap, not a display bug: `GET /api/v1/products?type=merchandise` and `?type=medal` both return `{"data":[],"next_cursor":""}` from the API itself. The "Merchandise and Medali tabs list products" acceptance bullet is **not confirmable in this environment** without seeding products of those types. |
| 5 | `/catalog/[id]` | `/catalog/065c269a-cd92-4dc0-a609-8d4b0943a731`, `.../8b6abc7b-b1cc-4652-988a-e41e6117e062`, `.../9e5f440f-9d17-45eb-9aee-9a87e1e5963c` | 1280×720 | `cf263dc` | **Confirmed.** "Spesifikasi Produk" renders on all 3 book products checked, with 6, 6, and 5 populated rows respectively (field sets differ per product — e.g. one has no "Jumlah Halaman" row, another no "Jenis Edisi" row). No row with a blank value was observed on any of the three; rows are conditionally omitted, not rendered empty. |

**Incidental finding, out of scope for this task, not fixed:** the phone number typed into the `/cart`
shipping address form is silently **not** persisted when the address is saved as the default
(`GET /students/profile` returned `"phone": null"` after save, while name, address, province, city,
kecamatan, and postal code all round-tripped correctly). Distinct from FB-15 — flagging for a future
task, not filed as a fix here.
