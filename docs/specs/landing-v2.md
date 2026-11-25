---
title: Public site v2 — landing, Event, Program, Toko, and guest cart
issue: none yet
status: proposed
created: 2026-11-17
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/design-tokens-v2.md", "docs/specs/event-series.md", "docs/specs/testimonials.md", "docs/specs/student-dashboard-v2.md", "docs/specs/admin-dashboard-v2.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/hero-award.webp", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/layout.tsx", "web/app/(student)/layout.tsx", "web/app/(student)/page.tsx", "web/app/(student)/cart/page.tsx", "web/app/(student)/catalog/page.tsx", "web/app/(student)/catalog/[id]/page.tsx", "web/app/(auth)/login/page.tsx", "web/lib/hooks/orders.ts", "web/lib/hooks/products.ts", "web/lib/auth-redirect.ts", "web/lib/i18n.ts", "web/stores/auth.ts", "web/stores/ui.ts", "backend/internal/server/routes.go", "backend/internal/handler/order.go", "backend/internal/handler/errors.go", "backend/internal/service/store.go", "backend/internal/service/store_digital.go", "backend/internal/repository/order.go"]
---

# Public site v2 — landing, Event, Program, Toko, and guest cart

Roadmap phase 4, the **last** redesign PR (RD1 settled 2026-11-26: PR 1 admin → event-series → student dashboard ‖ testimonials → **landing**). It needs PR 1 (tokens, shell), event-series (Event API), testimonials (testimonial API), and PR 2 (it edits the same `(student)` files).

## Problem and outcome

The public site (abakacademy.id) is a WordPress page: its stats render "0+" to crawlers and slow clients, it shows no prices, testimonials, or partner logos, its social links point to `#`, and event pages are maintained by hand. The platform has no public pages: a guest who opens `hub.abakacademy.id/` or `/catalog` is sent to `/login`, so nobody can browse the shop or put something in a cart before signing up.

Outcome: the Next.js app serves a public site in the prototype's look, in Indonesian and English, built from live data: a landing page at `/` for guests, an Event page, a Program page, and the Toko (`/catalog`) with a cart that works before login. Signed-in students keep their dashboard at `/` and see the same public navbar on public pages, with a Dashboard button.

## Current behavior (verified on `feat/public-landing` @ 38eac948)

- **No public pages.** `/`, `/catalog`, `/catalog/[id]`, `/cart` all live in `web/app/(student)/`. `(student)/layout.tsx` redirects a missing token to `/login` and renders "Memuat…" until the persisted auth store hydrates, so a prerendered `/` contains only "Memuat…". Admins and affiliators at any `(student)` path are redirected to `/admin` / `/affiliate`.
- **Client-only app.** Every page and layout is `"use client"` except `app/layout.tsx` (exports `metadata`, `<html lang="id" translate="no">`) and one admin layout. `NEXT_PUBLIC_API_BASE_URL` is baked into the bundle. No `NEXT_PUBLIC_SITE_URL` exists.
- **Public data reachable without a token:** `GET /products`, `GET /products/:id` (`apiFetch`, guests see only `published` products inside `available_from/until`), `GET /product-groupings`, `POST /promo-codes/validate`. Event and testimonial public endpoints arrive with their PRs (`event-series.md`, `testimonials.md`).
- **Cart is server-side and JWT-only.** `/orders` routes use `JWTMiddleware` and `ForbidRoles(affiliator)`. `useCart()` = `POST /orders` (mint or return the open cart); `useAddToCart()` = `POST /orders/:id/items {product_id, qty}`. `GET /promo-codes/active` is JWT-only.
- **Add-item checks** (`service/store.go` `AddItem`): product exists (404 `product_not_found`), stock-tracked physical product with stock 0 (409 `out_of_stock`), qty rules (`ValidateItemQty`), a digital product already in the cart (400 `invalid_qty`), a digital target already owned or in another live order (409 `digital_target_conflict` with `details.conflicts`, `checkDigitalCandidate`). It does **not** check `published` or the availability window. The repository inserts a new row per call, so adding a physical product already in the cart creates a second line. `unit_price` is snapshotted at add time. Checkout re-checks qty, digital targets, stock (409 `insufficient_stock`), and exam biodata (`biodataMissingFields`).
- **Guest add-to-cart today:** `catalog/[id]/page.tsx` `handleAdd` pushes `/login` when there is no token. `login/page.tsx` always goes to `redirectForRole(role)`; there is no return path.
- **i18n:** `DICT.id` / `DICT.en` in `lib/i18n.ts`; language persists in `useUIStore` (`abak-ui`). User-facing copy still says "Katalog" (`catalog_title`, `product_back_catalog`, `cart_view_catalog`).
- **Components:** `components/catalog/ProductCard`, `CategoryRail`; `components/cart/*` (line item, address, courier, promo, Snap); `components/brand/AbakLogo`. None of `SiteNav`, `SiteFooter`, `MathFloaters`, `EventSeriesPicker`, `EventStoryCard`, `EventSchedule`, `EditionCard`, `LanguageToggle` exist. No motion library is installed.

## Agreed behavior (Panca, 2026-11-17, revised 2026-11-25/26)

The prototype (`docs/prototypes/redesign-v2/index.html`, public routes `#/`, `#/kompetisi`, `#/program`, `#/katalog`, `#/keranjang`) is the layout reference; its copy (both languages) is the starting copy. Do not invent claims: every number comes from config (D1) or the API.

### 1. Routes and who sees what

| Path | Guest | Student | Admin / affiliator |
|---|---|---|---|
| `/` | Landing | Student dashboard (unchanged, PR 2) | Redirect to their home (unchanged; D4) |
| `/event` | Event page | Event page (public navbar) | Event page |
| `/program` | Program page | Program page | Program page |
| `/catalog`, `/catalog/[id]` | Toko (public navbar) | Toko (public navbar) | Toko |
| `/cart` | Guest cart (public navbar) | Server cart in the **student shell** | Unchanged redirect |
| `/login`, `/register`, `/otp` | Unchanged `(auth)` layout | — | — |

"Katalog" is renamed **Toko** / **Shop** in every student- and visitor-facing string; the route stays `/catalog`. Admin labels belong to PR 1.

### 2. Navbar (every public page)

- Logo · **Event** (`/event`) · **Program** (`/program`) · **Toko** (`/catalog`) · ID/EN toggle · account control. No Tentang item, no theme toggle, no cart icon.
- Guest: one split pill **Masuk | Daftar gratis** (Daftar in `ember`). Signed in: a **Dashboard** button (avatar + "Dashboard" + attention count) instead; students → `/`, admins → their admin home, affiliators → `/affiliate`. Student attention count = orders in `payment_pending` + registrations in `start`/`checkin`/`in_progress` (PR 2's count helpers); zero shows no count. Admins get no count.
- Transparent with white text over the landing hero; after 8px of scroll it becomes a floating centered pill (blurred surface, hairline border). On pages without a photo hero it starts in the pill style.
- Past the hero (or 120px on other pages) it slides away while scrolling down and returns on any scroll up, at the hero, and whenever focus is inside it.
- Active item: text color plus accent dot, `aria-current="page"`.
- Mobile: hamburger opening the same links, Masuk (guests), and a language row.

### 3. Landing `/` (guests), in order

1. **Hero.** The award photo full-bleed (`cover`, bottom-anchored) behind the transparent navbar, under a light scrim that fades into the page canvas with no dark edges. Headline "Dari ruang kelas ke panggung ⟨word⟩" / "From the classroom to the ⟨word⟩": the word rotates through `dunia`/`world stage` first, then the Event names (`short_name ?? name` of visible events in sort order, from the Event API); screen readers and reduced motion get only "dunia" / "world stage". Lede, **Cari kompetisimu** (→ `/event`) and **Konsultasi gratis** (WhatsApp). Stats **20.000+ / 200+ / 10** with their labels at the bottom of the hero (D1). Decorative math floaters may stay, never over text. No "Baru" kicker, no phone mockup, no event marquee, no cursor spotlight. Accepted: the photo's banner text shows faintly behind the headline. Needs a high-resolution photo from the client (current 880px wide).
2. **Program:** 6 cards (Kompetisi & Event, Pelatihan Olimpiade, Bimbel & Les Privat, Pelatihan Karya Tulis, Homeschooling, Study Tour & Kuliah Luar Negeri), each "Selengkapnya" → `/program#<id>`; section CTA "Tanya program" (WhatsApp).
3. **Tentang:** three centered lines "Setiap anak punya kapasitas, potensi, dan bakat yang unik." / "Tugas kami merawat dan mengasahnya," / "hingga tumbuh para ahli di bidangnya." (first line on one line from 900px up), signature "© Abak Academy 2026". No eyebrow, no stats. EN uses the prototype draft ("Every child has a unique capacity, potential, and talent." / "Our job is to nurture and sharpen it," / "until it grows into true expertise."), client to confirm.
4. **Event — "Pilih panggungmu":** event tiles + one story card for the selected event (`EventSeriesPicker` + `EventStoryCard`): logo or illustration, scope and level chips, full name, tagline, story, schedule timeline with state, rounds, categories, facts. CTA: "Daftar ⟨level⟩ · price" for each open edition (max 2) → `/catalog/[product_id]`, or "Tanya jadwal" (WhatsApp) when none; plus "Situs resmi" when set. No "all events" tile.
5. **Semua di satu akun:** 4 feature cards (static copy).
6. **Kursus online:** first 4 published courses, "Semua kursus" → `/catalog?type=course`.
7. **Cerita peserta:** one testimonial per slide; the left panel is that person's photo (4:5) and changes with the slide; quote, name, role, up to 3 achievement chips, prev/next buttons, "01/03" counter. Not rendered when the API returns none; no placeholder quotes ship.
8. **Toko Abak:** 4 published physical products (book, merchandise, medal), "Ke toko" → `/catalog`.
9. **Untuk sekolah & lembaga:** band with 4 capability tiles, "Ajukan kerja sama" (D2) and "Lihat jadwal" → `/event`.
10. **FAQ** + "Belum ketemu jawabannya?" WhatsApp block.
11. **CTA band** "Medali berikutnya bisa jadi milikmu." with Daftar gratis + Lihat kompetisi. The landing ends on this band; the footer follows with no gap.
12. **Footer:** blurb, address, WhatsApp number, social links (real URLs only; a link without one is hidden), link columns (Telusuri, Akun, Tim), wordmark. The end-of-page stats band is gone.

### 4. Event page `/event`

- Page hero "Semua event, satu tempat" with lede.
- "Kenali event-nya": the same picker and story card. `/event#<slug>` preselects that event and scrolls to it (the admin "Lihat di landing" link targets this).
- "Edisi yang tersedia": filters **Event · Tingkat · Jenjang** (client-side over the loaded data), count line, grid of `EditionCard` (event name, date, jenjang, scope, price or "Gratis", phase badge) → `/catalog/[product_id]`. Empty result: `EmptyState` with "Reset filter".

### 5. Program page `/program` (static; copy is a draft the client confirms)

- Hero "Pilih jalurmu" + lede + jump chips (one per program, → `#<id>`).
- One block per program, ids `kompetisi`, `olimpiade`, `bimbel`, `karya-tulis`, `homeschooling`, `study-tour`: icon art and number, title, description, facts **Untuk · Format · Jadwal**, "Yang kamu dapat" list, CTA: Kompetisi → "Lihat jadwal event" (`/event`); the others → "Tanya via WhatsApp" with a prefilled message naming the program.
- "Masih bingung pilih yang mana?" band with Konsultasi gratis (WhatsApp).

### 6. Toko `/catalog`, `/catalog/[id]`

- Slim page hero: eyebrow "TOKO ABAK", title "Buku, kursus & perlengkapan belajar", tagline "Semua yang kamu butuhkan buat latihan, belajar, dan tampil di hari lomba." **No promo line.**
- Existing category rail + search + grid, restyled by PR 1 primitives; exam products stay out of the Toko list (they are reached from the Event page). Product detail keeps its current content and behavior except that a guest can add to cart (§7).

### 7. Cart and login at payment

- A guest can add any purchasable product (including an exam edition) from the product detail page; the toast offers "Lihat keranjang". Digital products: qty 1, no duplicate; physical: qty stepper capped at visible stock.
- Guest `/cart`: items, quantities, remove, subtotal, and a note "Alamat dan ongkos kirim diisi setelah kamu masuk, saat pembayaran." when a physical item is present; "Punya kode promo? Pakai saat pembayaran setelah masuk."; the pay button reads **Masuk untuk bayar** with "Keranjangmu tetap tersimpan setelah masuk." Address, shipping, promo, and pay do not render for guests.
- Masuk untuk bayar → `/login?next=/cart`. After login (or register → OTP), the guest cart merges into the server cart (§Technical contract) and the student lands on `/cart` inside the student shell, with a merge notice if anything changed.
- Signed-in cart is today's cart in the student sidebar shell, promo codes entered at payment as today.

### 8. Motion (public pages only)

- Landing hero: photo parallax and content fade on scroll; rotating headline word every 2.6s.
- Every section below the hero reveals once with a different entrance (up, left, right, zoom, blur, tilt; shuffled per page load); its heading and first 12 grid children stagger in (80ms steps). Sections already in view on load render without animation.
- Revealed grid cards tilt toward the cursor (max 4°).
- Pixel-swarm cursor (25 pixels that scatter on fast moves and regroup into a square when still; white over the hero, CTA band, and footer) on every public page.
- All of the above is off under `prefers-reduced-motion: reduce` and on touch (`pointer: coarse`); content is fully visible without it. Student, admin, exam, and auth pages get none of it.

### 9. Context menu

Right-click is blocked on every page rendered in the public shell (including the guest cart); student-shell, admin, exam, and auth pages keep it. Accepted trade-off: it protects nothing (screenshots, DevTools, drag still work) and removes "open in new tab" and copy from the menu.

### 10. Spacing and layout

Public content uses the public `.wrap` width (1200px + gutters per `design-tokens-v2.md`); the student shell keeps 1480px. 80px between the last section and the footer on every public page except the landing (ends on its CTA band). 32px between a toolbar (filters, category rail) and its grid. The page-hero tint fades out with a mask (no hard edge).

### 11. Language

Every string on these pages exists in `id` and `en`. Event narratives and testimonials come from their APIs in the active language (`?lang=`). The toggle persists (`abak-ui`) and sets `<html lang>`.

## Technical contract

### Routes and files (`web/app`)

| Path | File | Notes |
|---|---|---|
| — | `app/(public)/layout.tsx` | Client layout: `PublicShell` (nav, `<main id="main">`, footer, motion, context-menu guard). No auth gate. |
| `/event` | `app/(public)/event/page.tsx` + server `layout.tsx` for metadata | |
| `/program` | `app/(public)/program/page.tsx` + metadata layout | Content from `lib/programs.ts` |
| `/catalog`, `/catalog/[id]` | **moved** from `(student)/catalog` to `(public)/catalog` (`git mv`, tests move along) | URLs unchanged |
| `/` | `(student)/page.tsx` | Guest → `<Landing/>`; student → PR 2 home |
| `/cart` | `(student)/cart/page.tsx` | Guest → `<GuestCart/>`; student → today's cart |

`(student)/layout.tsx` gains `GUEST_PATHS = ["/", "/cart"]`: once hydrated with no token on those paths it renders `PublicShell` around the page instead of redirecting; every other path keeps the redirect. Before hydration on `/` it renders the landing (not "Memuat…") so the static landing is in the prerendered HTML (D3); an inline `<head>` script sets `data-session` on `<html>` from the persisted `abak-auth` token, and CSS hides the public shell while it is set, so signed-in students never see a flash of the landing.

### Components (folders per `COMPONENTS.md`)

- New `components/shell`: `SiteNav` (links, `LanguageToggle`, `AuthPill`, `DashboardButton`, mobile menu, scroll states), `SiteFooter`, `PublicShell`, `LanguageToggle` (wraps `useTranslation`).
- New `components/brand`: `MathFloaters`.
- New `components/landing`: `LandingHero` (+ `RotatingWord`), `ProgramGrid`, `AboutLines`, `FeatureGrid`, `StorySlider`, `SchoolBand`, `Faq`, `CtaBand`, `Landing` (assembly).
- New `components/events`: `EventSeriesPicker`, `EventStoryCard`, `EventSchedule`; `components/catalog`: `EditionCard`.
- New `components/motion`: `PublicMotion` (one client component mounted by `PublicShell`: reveal observer, card tilt, swarm canvas, hero parallax, nav hide; all listeners passive, one rAF loop, torn down on unmount).
- Reuse: `ProductCard`, `CategoryRail`, `CartLineItem`, `Button`, `Badge`, `Card`, `EmptyState`, `Avatar` (PR 1), `AbakLogo`.

### Data per section

| Section | Source | Failure / empty |
|---|---|---|
| Hero words, Event picker, story card, edition grid | Event public API in `event-series.md` (list with narrative for `lang`, schedule, open editions incl. `product_id`, phase, level fields, price) via a new `useEvents(lang)` hook | Error or empty: hero rotates nothing (shows "dunia"), Event section and `/event` grid show `EmptyState` with WhatsApp; never a crash |
| Cerita peserta | `GET /testimonials?lang=` (`testimonials.md`) | Empty or error: section absent |
| Kursus online | `useProducts("course")`, first 4 | Empty: section absent |
| Toko Abak | `useProducts()` filtered to book/merchandise/medal, first 4 | Empty: section absent |
| Stats | `lib/site-config.ts` (D1) | — |
| WhatsApp number, prefilled messages, address, social URLs | `lib/site-config.ts` (one module; nothing hard-coded in components) | Social without URL hidden |
| Programs | `lib/programs.ts` (id, icon, tone, i18n keys) | — |
| Dashboard count | `useOrders()` + `useRegistrations()` + PR 2 count helpers, only when token and role student | Error: no count |

The Event API's exact path and field names are owned by `event-series.md` (now evolving the product grouping, see Dependencies); the landing consumes it only through `useEvents` and a typed adapter, so a rename touches one file.

### Guest cart contract

- **Store:** `stores/guest-cart.ts`, zustand `persist` key `abak-guest-cart`, shape `{ v: 1, lines: [{ product_id, qty, name, type, unit_price, image_url }] }` (name/price are a display snapshot only). Max 20 lines. Rules mirror the server: digital types (`course`, `exam`) qty 1 and no duplicate product; physical qty ≥ 1, capped at `stock` when the product is stock-limited. Corrupt or unknown-version storage resets to empty.
- **Guest cart page:** on load, each line is refreshed with `GET /products/:id`; a 404 or an out-of-window product marks the line "Tidak tersedia lagi" (excluded from subtotal and merge); a changed price updates the line and shows "Harga berubah". Subtotal = Σ current price × qty.
- **Merge trigger:** `useGuestCartMerge()` runs in `(student)/layout.tsx` once a student session is ready (hydrated, role `student`) and the guest cart is non-empty, so it also runs when `next` is lost (e.g. register → OTP). Admin/affiliator sessions never merge and keep the guest cart untouched.
- **Merge algorithm (web-only, existing endpoints):** `POST /orders` (mint/get cart) → skip guest lines whose `product_id` already has a server line (server wins; keeps retries idempotent and avoids a second physical row) → `POST /orders/:id/items {product_id, qty}` sequentially for the rest → invalidate `ordersKeys`, update the cart count.
- **Per-line outcome:**

| Server answer | Outcome | Guest line |
|---|---|---|
| 201 | added; if server `unit_price` ≠ snapshot → "Harga berubah: X → Y" | removed |
| already in server cart (pre-check) or 400 `invalid_qty` | "Sudah ada di keranjangmu" | removed |
| 404 `product_not_found` | "Tidak tersedia lagi" | removed |
| 409 `out_of_stock` | "Stok habis" | removed |
| 409 `digital_target_conflict` | "Sudah kamu miliki atau sedang dibayar" (existing `digitalConflictMessage`) | removed |
| 401 | stop; session handling as today | kept |
| network / 5xx / other | stop; toast "Keranjang belum tersinkron, coba lagi"; retried on next layout mount | this and later lines kept |

- **Notice:** outcomes other than plain success are shown once on `/cart` as a dismissible list (`MergeNotice`).
- **Login return path:** `/login`, `/register`, and `/otp` accept `next`; it is honored only when it is a same-origin relative path starting with `/` and not `//` (no open redirect), and only for the student role; otherwise `redirectForRole`.
- **Not changed:** no backend, migration, or order-status change. Checkout stays authoritative (stock, biodata, digital targets).

### i18n

New prefixes in both locales: `pub_nav_*`, `pub_foot_*`, `land_*` (hero, program, about, event, features, courses, stories, shop, school, faq, cta), `prog_*` (page + 6 programs), `event_page_*`, `shop_*` (hero, tagline), `gcart_*` (guest notes, Masuk untuk bayar, merge outcomes). Rename user-facing "Katalog" values (`catalog_title`, `product_back_catalog`, `cart_view_catalog`, and any visitor/student string found by grep) to Toko/Shop.

### Metadata and deployment

- `metadataBase`, canonical, and Open Graph URLs come from `NEXT_PUBLIC_SITE_URL` (new, optional; when unset, no canonical is emitted). Title and description per route in small server `layout.tsx` files; the landing's live in `app/layout.tsx` defaults. Every internal link is relative.
- Deployment (domain, WordPress, DNS, proxy) is out of scope and must stay config-only.

### Motion without dependencies

CSS transitions + `IntersectionObserver` + one `requestAnimationFrame` loop + a fixed `<canvas aria-hidden>` for the swarm, ported from the prototype (`revealSections`, `swarmStep`, `navScroll`, hero scroll handler). `matchMedia('(prefers-reduced-motion: reduce)')` and `(pointer: coarse)` gate initialization. **No new dependency.**

## Dependencies / cross-spec notes

- **PR 1 (`admin-dashboard-v2.md`, `design-tokens-v2.md`):** tokens incl. `ember`, display font, `Button`/`Badge`/`Card`/`EmptyState`/`Avatar`. Needs a public container width token (1200px `.wrap`) and the page-hero tint; if `design-tokens-v2.md` does not name them, this PR adds them in `globals.css` and the token spec should list them.
- **Event entity (`event-series.md`):** Panca decided (2026-11-26) that the Event evolves the existing **product grouping** (`product_grouping`, migration 0073, `GET /product-groupings`, `product.grouping_id`) instead of a new table. The Toko's competition grouping becomes the Event. This spec relies only on: visible events in sort order with `slug`, display name + short name, scope, levels, tone/illustration/logo, official URL, narrative for `?lang=`, schedule with state, and open editions with `product_id`, phase, level, price, date. Please keep `product_id` and a `slug` usable as a URL fragment; if `GET /product-groupings` changes shape for the Toko's grouping filter, the catalog page needs the compat window.
- **`testimonials.md`:** `GET /testimonials?lang=` with `photo_url` always present (4:5). No change requested.
- **`student-dashboard-v2.md` (PR 2):** (a) its AC1 lists `/catalog` among pages showing the student sidebar; after this PR `/catalog` and `/catalog/[id]` render in the public shell for everyone (prototype `pub('katalog')`), so that AC should drop `/catalog` or note it is superseded here; `/cart` stays in the student shell. (b) This PR reuses PR 2's count helpers (`lib/student-home.ts`) for the Dashboard button; please export them. (c) This PR edits `(student)/layout.tsx`, `(student)/page.tsx`, `(student)/cart/page.tsx` after PR 2.
- **`redesign-v2-roadmap.md`:** phase 4 row: landing is PR 5, last, after RD1 order; scope now includes `/program`, the Toko public move, and the guest cart (web-only, no backend). Open question "Stats static vs computed" is D1 here.

- **Aligned with the realigned specs (2026-11-26):** the `useEvents` adapter reads `GET /event-series?lang=` and `GET /event-series/:slug` from `event-series.md` (Event = extended `product_grouping`, migration `0075_event`). Event fields: `slug`, `name`, `short_name`, `scope`, `subjects`, `tone`, `illustration`, `logo_url`, `official_url`, narrative, schedule with state. Open-edition fields: `product_id`, `exam_ids`, `title`, `stages`, `grade_min`, `grade_max`, `price`, `image_url`, `phase`, `scheduled_at`, `scheduled_end_at`, `available_until`. `GET /product-groupings` is unchanged, so the Toko filter needs no compat work (its label may become "Event" per event-series D9).
- This PR renames the user-facing Toko strings `catalog_title`, `product_back_catalog`, `cart_view_catalog` to Toko/Shop. The Dashboard button reuses `kompetisiCount` and `pendingCount` exported from `lib/student-home.ts` (PR2). Layout uses `container-public` (1200px), the 80px footer gap, and the page-header tint fade from `design-tokens-v2.md`; the hero title size depends on tokens D1 (`display-hero`). Landing D4 (where admins land at `/`) and admin D10 (`NEXT_PUBLIC_SITE_URL`) must be decided together.

## Acceptance criteria

1. As a guest, `/`, `/event`, `/program`, `/catalog`, `/catalog/[id]`, `/cart` render without redirecting to `/login`; as a student, `/` renders the PR 2 dashboard and `/cart` renders in the student sidebar shell; admins at `/` are redirected as today.
2. The prerendered HTML of `/` (fetched without JS) contains the hero headline, Program, Tentang, FAQ, and CTA copy in Indonesian, and not "Memuat…".
3. A signed-in student loading `/` never paints landing content (Playwright: no landing hero visible in any frame screenshot after navigation with a stored session).
4. Navbar shows Event · Program · Toko, ID/EN, and Masuk | Daftar gratis for guests; a Dashboard button with the correct count for a student (pending orders + actionable registrations), no count when zero; no theme or cart icon, no "Katalog" anywhere user-facing (grep of rendered text in both locales).
5. Navbar: transparent with white text over the hero at scroll 0; pill after 8px; hidden after scrolling down past the hero; visible again after a 5px scroll up and when a nav link has keyboard focus.
6. Landing sections appear in the order of §3; no kicker, phone mockup, marquee, or end-of-page stats band.
7. The hero word rotates through "dunia" then the API event names; with reduced motion, and in the accessibility tree, it reads "dunia" only.
8. Program card "Selengkapnya" for program *k* lands on `/program#<id>` with that block in view; Kompetisi CTA → `/event`; other CTAs open `wa.me/<config number>` with the program name in the text.
9. An event with no open edition shows "Tanya jadwal" (WhatsApp); an event with ≥ 3 open editions shows exactly 2 Daftar CTAs linking `/catalog/<product_id>`; hidden events never appear (fixture includes one).
10. `/event#<slug>` selects that event in the picker; edition filters (Event, Tingkat, Jenjang) narrow the grid and "Reset filter" restores it.
11. With zero testimonials the Cerita peserta section is absent; with two, next/prev swap both the quote and the photo.
12. Toko hero shows the agreed tagline and no promo text; exam products are absent from the Toko grid.
13. Guest cart: adding a course twice keeps one line; a physical product's qty cannot exceed stock; reload keeps the cart; address, shipping, promo input, and pay are absent; the button reads "Masuk untuk bayar" and goes to `/login?next=/cart`.
14. Merge: after login, guest lines appear in the server cart and the guest store is empty; a product already in the server cart is not duplicated; owned, out-of-stock, and missing products are dropped with their notice; a changed price is reported; a 5xx keeps the unsent lines and retries on the next mount (unit tests per outcome row).
15. `next` is ignored for `https://evil.example`, `//evil.example`, and non-student roles.
16. With reduced motion or `pointer: coarse`: no swarm canvas in the DOM, no reveal classes hiding content, no parallax transform, no tilt; all section content visible on load.
17. Right-click `contextmenu` is `defaultPrevented` on public-shell pages and not on student-shell, admin, exam, or auth pages.
18. Spacing: 80px between last section and footer on `/event`, `/program`, `/catalog`, guest `/cart`; landing CTA band touches the footer; 32px between toolbar and grid on `/catalog` and `/event` (bounding-box check).
19. ID/EN: switching to EN leaves no Indonesian dictionary string on any public page (snapshot diff against `DICT.id` values), event narratives and testimonials come back in EN, and `<html lang>` follows.
20. Changing `NEXT_PUBLIC_SITE_URL` changes canonical and OG URLs with no code change; unset emits no canonical.
21. Lighthouse mobile on a production build of `/`: SEO ≥ 95, Accessibility ≥ 95, CLS < 0.1, LCP < 2.5s.
22. No new dependency; no hex values outside `globals.css`; every new string in both locales.

## Verification

- Unit (vitest, `docs/standards/web-tests.md`): `stores/guest-cart.test.ts` (rules, version reset), `lib/guest-cart-merge.test.ts` (every outcome row, idempotent retry, stop on 5xx), `lib/safe-next.test.ts`, `SiteNav` (guest/student/admin variants, count), `RotatingWord` (reduced motion), `EventStoryCard` (CTA cases), `StorySlider`, `Landing` (order, absent sections), `GuestCart` page, updated `(student)/layout.test.tsx` (guest paths render public shell, others redirect), `login` / `otp` page tests for `next`. Fixtures copy the Go payload shapes; no real PII.
- e2e (Playwright, mocked API where possible): guest journey landing → Toko → add → cart → Masuk untuk bayar → login → merged cart; nav hide/show; reduced-motion and `hasTouch` contexts; context-menu check; prerender check with `javaScriptEnabled: false`; grep `web/e2e/*.spec.ts` for `/catalog` and login-redirect expectations and update them.
- Gates from `web/`: `npx tsc --noEmit`, `npx vitest run`, `npm run build`.
- Browser (real Chrome, local stack seeded with ≥ 3 visible events incl. one without open editions, 2 testimonials with photos, courses, physical products incl. stock 0 and stock 2): 390px and 1366px, light and dark, every public page, plus reduced motion; student `/` and `/cart`. Run the `verify-abak` skill for the login journey and the guest-cart merge.
- Lighthouse mobile against `npm run build && npm start`.

## Non-goals

- Backend, migration, or order/payment logic changes; a server-side guest cart.
- Restyling `(auth)` pages, the product detail page beyond guest add-to-cart, or the prototype's `/kompetisi/:id` edition page (editions use `/catalog/[id]`).
- Blog migration; a CMS for programs, FAQ, or stats.
- A school lead backend (D2 picks the front-end shape).
- Deployment, domain, WordPress, DNS.
- Image protection beyond the accepted context-menu block.

## Decisions needed

Recommendation first in each.

- **D1. Stats (20.000+ / 200+ / 10).** a) Static values in `lib/site-config.ts`, changed by a code edit; computed counts differ from marketing claims. b) Computed "10" from the Event API (visible events count), the other two static. c) Admin-editable config (needs backend; out of scope).
- **D2. "Ajukan kerja sama".** The prototype opens a form (school, contact, WhatsApp); the 11-17 spec said WhatsApp. No lead endpoint exists. a) Button opens WhatsApp with a prefilled message, no form: no PII stored, no backend. b) The prototype form, whose submit only builds the WhatsApp message (same privacy as a, one more step). c) A real lead form with an endpoint and admin list (backend + PII handling; its own spec).
- **D3. SEO of `/` in a client-only app.** a) Render the landing before auth hydration with the `data-session` head script hiding it for signed-in users (AC 2–3); dynamic sections (events, testimonials, products) still load client-side. b) Keep "Memuat…" until hydration; no static landing HTML (fails the crawler goal). c) First server component fetching the public API at request time (needs a server-reachable API URL; breaks the client-only convention).
- **D4. Admins and affiliators at `/`.** a) Keep today's redirect to their home; they view public pages through `/event`, `/program`, `/catalog`. b) Show them the landing at `/` like the prototype.
- **D5. `hreflang`.** The language is a client preference on one URL, so id/en alternates would point to the same page. a) Drop `hreflang`; canonical only, Indonesian is the indexed language. b) Add a `?lang=en` URL that sets the language, and emit alternates for it.
- **D6. Client inputs (not code decisions; block launch, not build):** high-resolution hero photo (≥ 2400px wide), social URLs, confirmation of program copy and the EN Tentang lines.

## Implementation plan

Branch `feat/landing-v2` from `feat/public-landing` after PR 1, event-series, PR 2 student, and testimonials have merged into it (RD1); one PR into `feat/public-landing`; it is the last redesign PR before `feat/public-landing` goes to `main`. The plan assumes the recommended options.

| id | title | files / scope | depends_on | test-first step | done-check |
|---|---|---|---|---|---|
| T0 | Config + i18n contract | `lib/site-config.ts`, `lib/programs.ts`, `lib/i18n.ts` (new keys, Katalog → Toko) | — | — (data only) | `tsc --noEmit`; keys in both locales; grep shows no user-facing "Katalog" |
| T1 | Public shell + nav + footer | `components/shell/{PublicShell,SiteNav,SiteFooter,LanguageToggle}.tsx`(+tests), `app/(public)/layout.tsx`, nav scroll state, context-menu guard | T0 | SiteNav guest/student/admin variants + count; contextmenu prevented only inside `PublicShell` (red) | tests green; 390/1366 browser check |
| T2 | Route moves + guest gate + metadata | `git mv (student)/catalog → (public)/catalog`, `(student)/layout.tsx` `GUEST_PATHS` + pre-hydration landing + `data-session` head script in `app/layout.tsx`, metadata layouts, `NEXT_PUBLIC_SITE_URL` | T1 | layout test: guest on `/`, `/cart` renders public shell; guest on `/orders` redirects; student unchanged (red) | tests green; e2e redirect specs updated; prerender check (AC 2) |
| T3 | Guest cart store + page | `stores/guest-cart.ts`(+test), `(student)/cart/page.tsx` guest branch, `components/cart/GuestCart.tsx`(+test), `catalog/[id]` guest add | T2 | store rule cases; guest page hides address/promo/pay and shows Masuk untuk bayar (red) | tests green |
| T4 | Merge + safe `next` | `lib/guest-cart-merge.ts`(+test), `useGuestCartMerge` in `(student)/layout.tsx`, `MergeNotice`, `lib/safe-next.ts`(+test), `login`/`register`/`otp` pages | T3 | one test per outcome row; open-redirect cases (red) | tests green; `verify-abak` login + merge journey |
| T5 | Event components + `/event` | `lib/hooks/events.ts` (adapter over the event-series API), `components/events/*`, `components/catalog/EditionCard.tsx`, `app/(public)/event/page.tsx`(+tests) | T1 | story-card CTA cases (0/1/3 editions), `#slug` preselect, filters (red) | tests green |
| T6 | Program page | `app/(public)/program/page.tsx`(+test) | T1 | test: 6 blocks with ids, CTA per program (red) | tests green; anchor check |
| T7 | Landing sections | `components/landing/*`(+tests), `(student)/page.tsx` guest branch | T2, T5 | Landing test: section order, absent stories/products when empty, rotating word reduced-motion (red) | tests green |
| T8 | Toko header | `(public)/catalog/page.tsx` hero/tagline, spacing | T2 | page test: tagline, no promo text, no exam products (red) | tests green |
| T9 | Public motion | `components/motion/PublicMotion.tsx`(+test), CSS in `globals.css` | T1 | test: nothing initialized under reduced motion / coarse pointer (red) | tests green; browser check of each effect |
| T10 | Integration verification | whole branch | T4, T6, T7, T8, T9 | — | `tsc`, `vitest run`, `npm run build` green; e2e; browser matrix; Lighthouse; `git diff --check` |

Parallel frontier after T1: **T2, T5, T6, T9** (disjoint files; T5 and T6 add only their own i18n usage since T0 owns the keys). After T2: **T3, T7 (once T5 is done), T8**. Then **T4**, then **T10**. `(student)/layout.tsx` is touched by T2 and T4 and `(student)/cart/page.tsx` by T3 only, so T2 → T3 → T4 stays sequential.
