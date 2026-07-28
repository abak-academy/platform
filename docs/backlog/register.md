# Open work register

Consolidated inventory of known bugs, tech debt, and unbuilt features.
**Every item below was re-verified against the tree at `03d7bf2` on 2026-07-28** — file:line
references are current, and items that had quietly been fixed are listed under
[Closed](#closed-since-the-last-inventory) so they don't get re-opened.

Production is live (`hub.abakacademy.id`) but running `4d69591`, four commits behind main.

---

## 1. Bugs

| ID | Severity | Summary |
|---|---|---|
| B-0 | ~~blocker~~ | White screen when reopening the shipping address in the cart — **fixed, verified in a browser** |
| B-0b | ~~high~~ | The address form collapsed on the last field's first character — **fixed, verified in a browser** |
| B-1 | **high** | Promo discount never reaches the order — buyer sees a discount, pays full price |
| B-2 | high | Admin product list is capped at 20 and cannot page |
| B-3 | high | Digital products can't have an image (live on prod) |
| B-4 | high | `AdminReconcileOrder` queries the gateway with an empty ref |
| B-5 | medium | Checkout commits `payment_pending` before the gateway call |
| B-6 | medium | Malformed `?cursor=` returns 500 instead of 400 |
| B-7 | medium | Legacy digital cart rows at qty>1 still overcharge |
| B-8 | medium | CSV formula injection in three backend exports |

### B-0 — White screen when reopening the address in the cart *(root cause found, fix applied)*

Reported 2026-07-28 20:12 WIB on `stg.abakacademy.id/cart`, Edge/Windows, with a screen recording.
The recording is not kept in the repo — the steps below are what it showed, read off frame by frame.

**Repro, confirmed by the reporter and reproduced in a test:**

1. The buyer's profile has **no postal code**, so they type it into the cart form by hand.
2. The instant the first character lands, every field is non-empty, `addressReady` flips true and the
   form is replaced by the saved summary — the address "saves itself" with a one-character postcode.
   *(That collapse is its own defect — see B-0b.)*
3. Click **Ubah** to correct it → white screen,
   *"Application error: a client-side exception has occurred"*.

**The exception is `Maximum update depth exceeded`** (React error #185 in a production build, which is
why only the generic message reaches the user).

**Root cause — a two-effect feedback loop, one render out of phase.**
`ShippingAddressForm` mirrors the address into its own `useState`, then writes that state back to the
parent, while the parent feeds its copy straight back in as `initialAddress`
([`cart/page.tsx:148`](../../web/app/(student)/cart/page.tsx)). The hydrate effect listed
`initialAddress` in its deps, so each of the two effects re-fired on the other's output:

- On remount (clicking **Ubah**) the child's state starts empty. The hydrate effect fills it from
  `initialAddress`, but the emit effect — running in the same commit, before that state has
  rendered — publishes the still-empty values and **wipes the parent's address**.
- Now `hasAddressValues(initialAddress)` is false, so hydrate takes its `else if (profile)` branch and
  writes `profile.kode_pos`, which is `""`.
- The emit effect then publishes the previous render's `"1"`, hydrate reads it back…

Instrumenting both effects showed `kode_pos` flipping `"1" → "" → "1" → ""` until React gave up.
The loop only ignites when the two hydration sources **disagree** — i.e. when the user typed a field
the profile doesn't have. With a profile that already carries the postal code both branches agree and
nothing oscillates, which is why a first repro attempt using a complete profile passed.

**Fix:** hydrate from props **once per mount** (a `hydrated` ref guard in
[`ShippingAddressForm.tsx`](../../web/components/cart/ShippingAddressForm.tsx)). The fields are the
component's own state afterwards, so there is no path from the parent's copy back into them. Nothing
else changes: on remount `initialAddress` is complete, so the form still opens fully populated.

**Verification:** a regression test in `web/app/(student)/cart/page.test.tsx` follows the sequence
above; it fails with the exact `Maximum update depth exceeded` error before the fix and passes after
(mutation-checked in both directions). Full frontend suite green, 984 tests over 102 files.

Also **driven in a real browser** against the local stack, with a throwaway student whose profile had
no postal code — the condition that ignites the loop. Typing one character kept the form open
(`formStillOpen: true, kodePosValue: "1", summaryShown: false`); finishing the address, pressing
**Cek Ongkir** and then **Ubah** reopened the form fully populated
(`formReopened: true, kodePos: "15310", whiteScreen: false`) with no console errors. The throwaway
user, its cart and the borrowed port were restored afterwards.

*Production was never exposed: `ShippingAddressForm.tsx` was last changed by `03d7bf2`, and the live
prod image `4d69591` predates it. This did block shipping the four pending commits.*

### B-0b — The address form collapsed mid-typing *(fixed)*

The form renders on `editingAddress || !addressReady`
([`cart/page.tsx:145`](../../web/app/(student)/cart/page.tsx)), and `addressReady` is just
"every field non-empty". So the form vanishes on the **first character** of the last field the buyer
fills, taking the keyboard focus with it — which is how a one-character postal code got saved in the
first place.

**Fix:** the form's visibility is now explicit state (`addressFormOpen`, open by default) instead of
being derived from `addressReady`. It closes only when the buyer asks for a quote, and both entry
points to that action — the form's own button and the courier block's — now run the same handler, so
neither can leave the form open over a quoted address.

**Postcode validation (added in the same branch).** `ValidateKodePos`
(`backend/internal/service/kode_pos.go`) rejects anything that is not a digit, wired into the four
places a client can post one: the shipping quote, the cart address patch, a student's own profile
update, and admin student registration. It stays a string — leading zeros are significant — and
length is deliberately not checked, so `"1"` still passes; requiring exactly five digits is a
one-line tightening if wanted, but it would also reject shorter codes already stored.

**Flat-rate fallback is now announced.** The courier API answers an address it cannot place with an
empty rate list rather than an error, so the flat rate standing alone was the only hint the buyer's
region was never priced. `CourierRateList` now says so above the picker when every returned rate is
an estimate.

### B-0c — Saving the address, and marking which one is primary *(done)*

The button said "Cek Ongkir" because that is all it did — the address only reached the order as a
passenger on the courier choice, so abandoning the cart first threw it away. It now saves: the
address is PATCHed onto the order on press, and onto the profile too when
**"Jadikan alamat utama saya"** is ticked. That box defaults on only when the profile has no address
yet, so shipping one order to someone else cannot silently replace the buyer's own.

The summary carries an **"Alamat Utama"** badge when the order's address matches the profile's,
compared field by field — no column, no migration, and the mark that tells addresses apart once a
buyer can have several. The button is auto-width and right-aligned, with the "incomplete" hint beside
it rather than under a full-bleed bar.

**Found on the way, and fixed here:** `app/(student)/layout.tsx` gated the whole student shell on
`isFetching`, so *any* background profile refetch unmounted every student page and lost its local
state. Saving an address made it visible — the buyer was thrown back to a blank form. The gate exists
for a real reason (a Google student's shell depends on completeness read from DB truth, and there is a
test for it), so it is now scoped to Google students instead of everyone. Google students still see
"Memuat…" during a refetch; that trade-off stands.

### B-1 — Promo discount never reaches the order

The backend is now correct: `PatchCart` resolves the code and writes the discount
(`internal/service/store.go:713-721`, `internal/repository/order.go:382-386`). The **frontend
never sends it** — the cart page calls `validatePromo` for display only
(`web/app/(student)/cart/page.tsx:250-253`), and the single `patchCart.mutate` call site
(`:73-95`) omits `promo_code` even though `usePatchCart` supports it
(`web/lib/hooks/orders.ts:219-221`).

Second-order defect to fix in the same pass: when `patch.PromoCode` is nil, `repoPatch.PromoCodeID`
is left nil and the UPDATE writes `promo_code_id = NULL` while carrying `discount` forward. So once
B-1 is fixed, any later shipping patch silently detaches the promo — the money comes off the total
but `IncrementPromoUses` at checkout is skipped, and `max_uses` stops being enforced.

**Fix:** send `promo_code` from the cart; carry `order.PromoCodeID` forward when the patch omits it.

### B-2 — Admin product list capped at 20

`handler/product.go:61-63` builds the filter without `Cursor:`, while the public path at `:21-24`
includes it. The repository defaults `Limit` to 20 (`repository/product.go:126-128`) and returns a
`next_cursor` the admin UI can never spend. An admin cannot reach products a student can see.
**One line.**

### B-3 — Digital products can't have an image

`web/components/admin/ProductModal.tsx:173` and `:190` gate `image_url` behind `showStock` (`:121`,
true only for `book | merchandise | medal`) although the backend accepts it for every type. Visible
in production right now: exam and course cards render a placeholder icon. **~3 lines.**

### B-4 — Reconcile queries the gateway with an empty ref

`internal/service/store.go:1349` — `s.payment.QueryStatus(ctx, "")`. The order is then flipped to
`paid` (`:1354-1357`) on the strength of a status query that never identified which order it was
asking about. Open since the webhook-signature work.
**Fix:** pass the order's `gateway_ref`; refuse to reconcile when it is empty.

### B-5 — Checkout commits `payment_pending` before the gateway call

`internal/service/store.go:964` commits, `:970` calls `CreatePayment`. A gateway failure leaves a
durable `payment_pending` order with no `gateway_ref`. This actually happened on 2026-07-22 when
Midtrans rejected the request; recovery depended entirely on the Continue Payment path.
Either move `CreatePayment` inside the tx (holds it open across a network call) or accept the split
and treat `RetryPayment` as the documented recovery.

### B-6 — Malformed cursor → 500

`repository/product.go:151-155` appends `AND id > $n` with the raw query string, so a non-UUID
reaches Postgres and errors out. Should be a 400.

### B-7 — Legacy digital rows at qty>1

`AddToCart` now blocks it (`store.go:476` `ValidateItemQty`, `:479-484` duplicate-add guard), but
`Checkout` never re-validates and the qty stepper is hidden for digital items — so a row created
before the guard can only be removed, not corrected downward. Shrinking, but closes with:

```sql
UPDATE order_item SET qty = 1 WHERE product_type IN ('exam','course') AND qty > 1;
```

### B-8 — CSV formula injection in three backend exports

No sanitiser exists anywhere in `backend/`. Writers: `internal/service/admin_results.go:135`,
`bulk_credentials.go:96`, `student_bulk.go:338`. Student names are attacker-supplied at
registration. The frontend roster export was fixed (C-08); the backend never was.
*Correction to the earlier note: there are **three** writers, not four — `exam_import.go:54` is a
`csv.NewReader`.*
**Fix:** mirror C-08 — prefix `'` and force-quote any field starting with `= + - @ \t \r`.

---

## 2. Tech debt

### D-1 — Dead `StorageClient` port → 62 shim methods *(the big one)*

`internal/service/ports_storage.go` defines the port, but `Service.storage` is still a concrete
`*minio.Client` (`internal/service/service.go:28`), and `storeRepo` is concrete too. With no seam,
service tests hand-copy production logic into shims instead of driving `*Service`:

| File | shim methods |
|---|---|
| `course_test.go` | 18 |
| `store_test.go` | 14 |
| `exam_session_test.go` | 10 |
| `certificate_test.go` | 5 |
| `exam_result_test.go` | 4 |
| `exam_test.go` | 4 |
| `admin_results_test.go`, `exam_leaderboard_test.go` | 3 each |
| `student_test.go` | 1 |
| **total** | **62** |

Those tests pass against the copy, not the code that ships. Sequence: wire the seam (one PR), then
delete the shims (a second PR). Don't graft either onto a feature branch.

### D-2 — `is_estimate` not persisted

The rate carries it (`internal/service/ports_logistics.go:18`) but the order row doesn't, so both
`ShippingInfo.tsx:10` and `OrderDetailModal.tsx:18` infer the badge from the courier string. Renaming
the flat-rate label again would silently strip the badge from historical orders.
Full write-up incl. the backfill: [`shipping-estimate-flag.md`](shipping-estimate-flag.md).

### D-3 — Dead columns kept on purpose

Keep and wire when the feature is built; do **not** drop:
`exam.bundle_url` / `bundle_generated_at` / `cdn_bundle` (CDN question bundle — no generation
pipeline; the handler explicitly rejects the fields, `handler/exam_package.go:101`),
`orders.invoice_url` (no producer), `orders.payment_method` (never written —
`repository/order.go:43`, `model/order.go:25`; the FE branch that reads it is dead code).

### D-4 — Four frontend definitions of digital-vs-physical

`lib/shipping.ts`, `components/orders/ShippingInfo.tsx`, inline in `CartLineItem.tsx`, and inline in
`catalog/[id]/page.tsx`. A sixth product type means finding all four.

### D-5 — No gofmt gate

**38 files** under `backend/` currently fail `gofmt -l`. Adding the gate means formatting them all in
one commit first, or the gate lands red.

### D-6 — No handler-level webhook signature test

The only handler test touching signatures is `errors_test.go`. The bypass fixed in `841cd84` has no
regression test at the handler boundary.

### D-7 — Fazpass is unused and should be deleted

Still referenced in `cmd/api/main.go`, `cmd/worker/main.go`, `config/config.go` + tests, and six
config/secrets files. OTP goes over SMTP. This supersedes the old "migrate the Fazpass key to
`system_config`" item — that work is moot if the integration is removed.

### D-8 — Certificate rendering consolidation

See [`certificate-rendering-consolidation.md`](certificate-rendering-consolidation.md).

---

## 3. Features (not started)

| ID | Item | Notes |
|---|---|---|
| F-1a | Redesign the 3 built-in certificate templates | Cheap: swap 3 PNGs in `internal/service/assets/` + tune `defaultLayout`. No migration. `modern`/`elegant` have dead white space mid-lower. |
| F-1b | Admin-definable certificate template *types* | Move built-ins from hardcoded Go to a `certificate_template` table (`name, background_key, layout JSONB, is_builtin`); the 3 built-ins become seed rows. Reuses the existing DnD editor + Gotenberg renderer unchanged. |
| F-2a | Admin self-service profile | Admins can't edit their own profile; students can. |
| F-2b | Admin RBAC pass | Review which roles reach which modules (`Capabilities(role)` + `internal/server/routes.go`). Scope before changing. |
| F-2c | Split `/admin/login` from `/login` | **Decided:** FE-only, path-based, not a subdomain. Driver is the Google button — `GoogleLogin` hard-codes `RoleStudent`, so an admin clicking it gets a student account. Admin login = password only. |
| F-3 | Multi-address book | One address per student today. |
| F-4 | Catalog facets + real pagination | Facets need spec-value normalisation; the 10-page cursor loop is a stopgap. |
| F-5 | CD | Still manual `IMAGE_TAG` edit → `pull && up -d` on both VMs. WIF now exists, so the WIF+IAP path is much cheaper than when this was parked. |
| F-6 | **Amazon SES** | Hostinger mailbox caps at **100 emails/day** — one exam-registration morning blows it. SES needs domain verification + DKIM in Cloudflare, so it carries DNS propagation. Don't leave it until you're blocked. Ties to the OTP-cost discussion with the client. |

---

## 4. Verification debt

Shipped without ever being looked at in a running browser:

| Surface | What to confirm |
|---|---|
| `/catalog` | Sticky category rail holds while the grid scrolls; Merchandise + Medali tabs list products *(5-per-row and whole book covers already confirmed on staging)* |
| `/catalog/[id]` | "Spesifikasi Produk" table renders; blank-value rows absent |
| `/cart` physical | Saved address renders with **Ubah** + **Cek Ongkir**; per-order overrides survive edit/reopen; estimate badge on the flat-rate quote |
| `/cart` digital | No qty stepper; "Produk digital dibeli 1× per akun." shown |
| `/orders/[id]` physical | "Pengiriman" block shows address, courier, service, ongkir, resi |
| Certificate / exam card | A real end-to-end Gotenberg render — the sidecar is up on staging but has never actually rendered a document; cached certs previously masked its total absence |

---

## 5. Infra / CI

- **Repo is still public** (`abak-academy/platform`). Going private breaks the staging VM's
  `docker login ghcr.io` and starts metering Actions.
  Runbook: `docs/runbooks/repo-migration-to-client-org.md`.
- **`pipeline.yml` runs twice per push** — bare `on: push` **and** `on: pull_request`, neither
  filtered, ~13 min each. Free while public, the single largest minute drain the day it isn't.
  One-commit fix: `push: branches: [main]` + a `concurrency` group with `cancel-in-progress`.
  Do **not** path-filter the image build — `build-image.sh` publishes api/worker/web under one SHA
  and the compose files deploy them via a single `IMAGE_TAG`.
- `app-staging.yaml` in the repo is **deliberately stale** — the VM's copy is hand-edited, never
  pulled. Its GHCR paths still point at the old org.
- The empty `default` VPC network should be deleted so nobody launches a VM into it by accident.

---

## Closed since the last inventory

Verified fixed on 2026-07-28 — do not re-open:

- **Free products skip the payment page** (was P-B) — end to end: the zero-total path marks the order
  paid and emits `OrderPaid` in the same tx (`store.go:923-963`) with a replay sentinel, and
  `SnapCheckout.tsx:26-31` branches on `free`.
- **Product availability window** (was P-A) — `product.available_from` / `available_until` +
  `productAvailabilityFilter` on the public catalog query.
- **Promo discount persistence, backend half** (was P-D) — `PatchCart` now resolves and writes it.
  The frontend half remains open as **B-1**.
- **Flat-rate estimate days** — the `EstimatedDays: 0` render was replaced by PR #52's duration
  parse and the `IsEstimate` flag.
- **`main` CI red on `images-prod`** — resolved when WIF landed; the last five runs on `main` are
  green.
- **Fazpass key → `system_config`** — moot, superseded by D-7 (delete the integration).
