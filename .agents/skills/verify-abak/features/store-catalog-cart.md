# Store catalog and cart

Students browse published products (books, courses, competition packages) at `/catalog`, open one at `/catalog/[id]`, add it to the cart at `/cart` and check out. Admins manage products at `/admin/products`.

## Sub-features

- `catalog-empty` shows the empty or setting-up state when nothing is published.
- `catalog-browse` lists published products and opens a detail page.
- `cart-add` adds a product and shows it in `/cart` with the subtotal.
- `cart-checkout` creates an order and opens Midtrans Snap (sandbox).

## How to get to it (user POV)

- Student nav item `Toko` / `Store`, or open `/catalog` directly.
- From a product page, add to cart, then open `/cart` (heading `Keranjang` / `Cart`).
- `Lanjutkan belanja` / `Continue shopping` returns from the cart to `/catalog`.

## Driving it with abak-verify

Preconditions:

- `abak-verify doctor` prints `OK`.
- At least one **published** product. The fresh DB has none. Create it as the seeded admin: `POST /admin/products` creates a draft, so publish it afterwards. A `course` product also needs `course_ids` from an existing course.

- **Empty state.** Sign in as the student and open `/catalog`. Expect `Toko sedang disiapkan` / `The Store is being set up` or `Belum ada produk pada kategori ini.`.
- **Browse.** After publishing a product, `/catalog` lists its name. Click it, and `/catalog/<id>` shows the same name.
- **Add to cart.** Add the product, open `/cart`, and expect the heading `Keranjang` plus the product line. Second view: `GET /orders` for the student shows the open cart with that item.
- **Checkout.** Proves the request only: Midtrans runs in sandbox, so no real payment settles.

## Gotchas

- `POST /orders` returns the student's existing open cart, so leftover items and promo codes from earlier journeys carry over. Clear items and send `promo_code: ""` in setup.
- A physical item makes checkout require a shipping choice (Biteship, with no key locally). Use a digital product for a clean checkout.
- `/admin/store`'s queue reads 0 in prod, a known issue. Don't treat that as a regression in this flow.
