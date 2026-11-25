---
title: Student dashboard v2 (sidebar shell, home, order detail + invoice)
issue: none yet
status: proposed
created: 2026-11-25
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/student-shell-v2.md", "docs/specs/admin-dashboard-v2.md", "docs/specs/design-tokens-v2.md", "docs/specs/event-series.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/(student)/layout.tsx", "web/app/(student)/page.tsx", "web/app/(student)/orders/[id]/page.tsx", "web/app/(student)/orders/[id]/timeline.ts", "web/components/shell/AppShell.tsx", "web/components/shell/AppHeader.tsx", "web/components/dashboard", "web/lib/nav-config.ts", "web/lib/hooks/exam.ts", "web/lib/hooks/orders.ts", "web/lib/hooks/students.ts", "web/lib/hooks/products.ts", "web/lib/exam-card-state.ts", "web/lib/exam-schedule.ts", "web/lib/types.ts", "backend/internal/server/routes.go", "backend/internal/service/store.go", "backend/internal/service/exam_leaderboard.go", "backend/internal/service/exam_result.go", "backend/internal/repository/order.go", "backend/internal/model/product.go", "backend/internal/model/exam.go", "/Users/Panca/Documents/MyBook/Project/bimbel-tryout/web/lib/capture-pdf.ts"]
---

# Student dashboard v2

Supersedes `student-shell-v2.md` (2026-11-17). Roadmap phase 5. PR 2 of the redesign; PR 1 is `admin-dashboard-v2.md`.

## Problem and outcome

The 2026-11-17 plan put students on the public top navbar. On 2026-11-25 Panca chose the opposite: students keep the app they use today, in the same sidebar style as admin, and the catalog moves out to the public landing. The home page today mixes a static "latest exam" banner, ranking, exam-progress bars, and popular lessons, none of which say what to do next. The order detail page links an invoice that never exists.

Outcome: a student sees the admin-style sidebar shell with four items, a home page that shows what needs attention now (unpaid order, package on the way, open events, registered events, courses to continue), and an order detail page that can show, print, and download an invoice.

## Current behavior (verified 2026-11-25 on `feat/public-landing` @ 65136458)

**Shell**
- `(student)/layout.tsx` renders `AppShell role="student"` after the auth, admin/affiliator redirect, Google profile-completeness, Snap loader, and temporary-password gates. `AppShell` renders `CartCountSync` (writes `useCart()` item count into `useCartStore`) for students only.
- `NAV_CONFIG.student` (`lib/nav-config.ts`) = Beranda `/` · Kompetisi `/exam` · Kursus Saya `/courses` · Toko `/catalog` · Tagihan `/orders` · Profil `/profile`. i18n already reads `nav_competition` "Kompetisi", `nav_courses` "Kursus Saya", `nav_billing` "Tagihan".
- `AppHeader` holds search, ID/EN toggle, theme toggle, cart (students), a bell, and the avatar dropdown (Profil, Keluar). The bell has no handler and a permanent red dot. **There is no student notification endpoint** (`routes.go` has only `/admin/notifications`).
- Below `lg` the sidebar collapses to a 64px icon rail; there is no off-canvas drawer.

**Home `/`** (`(student)/page.tsx`) uses only `useDashboard()` → `GET /students/dashboard`: `enrolled_courses[] {id,title,progress,total_lessons,done_lessons,cover?}`, `pending_order? {id,product,amount}`, `study_summary {visited_lectures,total_lectures,enrolled_courses_count,completed_courses,total_minutes}`, `ranking`, `exam_progress`, `popular_lessons`. Renders hero banner → `RankingCard` → `PendingBanner` → course cards → `StudySummaryCard` + `ExamProgressCard` → `PopularLessonsSection`.

**Data the student can fetch today**

| Need | Source | Notes |
|---|---|---|
| Registered exams | `useRegistrations()` → `GET /exam/registrations` (`RegistrationListItem`) | Has `exam_id`, `exam_title`, `status`, `session_id`, `scheduled_at/_end_at`, check-in fields, `timer_mode`, `duration_minutes`, `sections_duration_minutes`. **No** result-release state, **no** `allow_leaderboard`, **no** level. |
| Card state | `computeCardState()` in `lib/exam-card-state.ts` | start / locked / checkin / in_progress / expired / submitted |
| Work duration | `workingMinutes()` in `lib/exam-schedule.ts` | From registration fields; available. |
| Result | `useSessionResult(sessionId)` → `GET /exam/sessions/:id/result` | Returns `state` (`hidden`/`grading`/`locked`) when gated, else the full result with pembahasan. |
| Leaderboard | `/exam/sessions/[id]/leaderboard` page, `useSessionLeaderboard` | 403 `leaderboard_not_available` when `allow_leaderboard` is off or a result gate holds; the page already renders a message for it. |
| Open events | `useProducts("exam")` → `GET /products?type=exam` (public, no auth) | Unauthenticated callers get only `published` products inside `available_from/until`. Payload: `id,name,price,image_url,exam_ids,relations[{id,title}],available_until`. **No** exam schedule, **no** level, **no** sales/registration count. |
| Orders | `useOrders()` → `GET /orders` (first 20, carts filtered out) | Each order carries `items[]` (`product_type`, `name`, `unit_price`, `qty`, `jumlah`), `status`, `paid_at`, `shipped_at`, `completed_at`, `selected_courier/_service`, `courier_code`, `tracking_number`, `shipping_address`, `payment_method`, totals. **No** processing timestamp. `shipment_events` only for Biteship waybills. |
| Courses | dashboard `enrolled_courses` | Progress and counts; **no** next-lesson title. `useMyCourses()` returns enrollment rows without titles. |

**Order detail `/orders/[id]`**: grid `[1fr_380px]` with items + status timeline (`buildTimeline`) on the left and a sticky summary/shipping/payment aside on the right. `PaymentInfo` shows an "Lihat invoice" link from `order.invoice_url`; **no code writes `orders.invoice_url`** (column added in migration 0009, only ever read), so the link never renders.

## Agreed behavior (Panca, 2026-11-25)

### 1. Shell
- Student pages render the PR 1 sidebar shell (admin prototype style). Sidebar items, one ungrouped list: **Beranda** `/` · **Kompetisi** `/exam` (count) · **Kursus Saya** `/courses` · **Tagihan** `/orders` (count). No Katalog/Toko, no Profil.
  - Kompetisi count = registrations whose `computeCardState` is `start`, `checkin`, or `in_progress` (actionable now). Tagihan count = orders with `status === "payment_pending"`. A zero count renders nothing.
- No header icons (theme, cart, bell) for students. The **user card** at the sidebar bottom (avatar, name, "Kelas N") opens an upward menu: Notifikasi (count) · Keranjang (count) · Mode gelap/terang · Profil & akun · Keluar. The cart count also shows as a dot on the card. See D4 for Notifikasi.
- Mobile (< lg): the page header shows only a hamburger that opens the sidebar as a drawer.
- Not built: the prototype's referral-code card (students have no referral code).
- Exam session room and print routes keep their own layouts.

### 2. Catalog
- `/catalog`, `/catalog/[id]`, and `/cart` stay routable inside `(student)` and keep working from in-page links (cart, orders, courses empty states, "Tambah kursus", carousel CTA). They only lose the sidebar entry. Moving the catalog to the public landing is a later phase.

### 3. Home `/` order
1. Date overline + greeting ("Selamat malam, Nadia") + one intro line.
2. `PendingBanner` when `pending_order` exists (unchanged data).
3. **Shipping strip**, only while a physical order is `processing` or `shipped`.
4. Row A: **Event carousel** (65%) | **Event & kompetisi** card (35%).
5. Row B: **Lanjut belajar** (40%) | **Total jam belajar** (60%).
- Rows stack to one column below 1200px.
- Removed from home: hero "Ujian Terbaru" banner, "Peringkat saya", "Progress ujian", "Pelajaran populer", the big course card grid, and the "Kompetisi terdaftar" section.

### 4. Shipping strip
- Picks the newest order (by `created_at`) with status `processing` or `shipped` and at least one item whose `product_type` is not `exam` or `course`.
- Title: "Pesananmu sedang dikemas" (processing) / "Paketmu sedang dikirim" (shipped); subtitle = physical item names.
- Steps **Dibayar → Dikemas → Dikirim → Diterima**; dates from `paid_at`, none (D7), `shipped_at`, `completed_at`. A step is reached when its date exists or the status is at/after it.
- Right side: courier (`selected_courier` + service) and the resi (`tracking_number`) as a copy button (clipboard + toast); without a resi, "Resi muncul setelah paket diserahkan ke kurir". **Detail** links `/orders/[id]`.
- Hidden once the order is `completed` (or for any other status).

### 5. Event carousel
- Slides = open events the student has not registered for (source and order: D1). An exam product counts as registered when every id in its `exam_ids` has a registration.
- Slide: overline, headline from status ("Pendaftaran dibuka" / "Sedang berlangsung", D1), `name · level · price` or "Gratis" (level: D2), product image or illustration. CTAs: **Daftar sekarang** (open) / **Jelajahi sekarang** (live) → `/catalog/[productId]`; **Kompetisi Saya** → `/exam`.
- No eligible event: one fallback slide ("Belum ada event baru", CTA Kompetisi Saya), no dots.
- Autoplay every 6s; pauses while hovered or focus is inside; no autoplay under `prefers-reduced-motion` or with one slide. Dots evenly spaced, each a button labelled "Event i dari n: name", `aria-current` on the active one.
- All slides stack in one grid cell, so height and dot position never change; transition is a crossfade with a small horizontal slide (none under reduced motion). Hidden slides are `inert` + `aria-hidden`. Region has `aria-roledescription="carousel"`.
- No banner CMS: content comes from admin product/exam data.
- **When event-series (phase 3) lands:** switch the source to `GET /event-series` `open_editions[]` (`product_id`, `title`, `level`, `price`, `phase`, `scheduled_at`), which gives level and open/live phase directly. Only the data adapter changes; the component takes a normalized slide list.

### 6. Event & kompetisi card
- Compact list of the student's registrations, max 4, plus "Lihat semua" → `/exam`.
- Order: not-finished (live or upcoming) by `scheduled_at` ascending (unscheduled last), then finished (submitted or expired) newest first by `scheduled_end_at ?? scheduled_at`.
- Row: date tile (day + short month of `scheduled_at`; "—" when unscheduled), `exam_title`, "{n} menit" from `workingMinutes` (omitted when null), and "Sedang berlangsung" when live.
- Action per row: submitted + result released → **Lihat hasil** → `/exam/sessions/[session_id]/leaderboard` (not the review page); submitted + not released → "Menunggu hasil"; expired → "Terlewat"; live (`start`/`checkin`/`in_progress`) → **Mulai** → `/exam/[registrationId]`; otherwise **Detail** → `/exam/[registrationId]`. Release detection: D3.
- Empty: "Belum ada event yang kamu ikuti." + "Lihat event" → `/catalog` (until the landing exists).

### 7. Lanjut belajar + Total jam belajar
- Lanjut belajar: one row per `enrolled_courses` item: cover thumbnail (`cover`, else a tone placeholder) with a play badge, title, `progress` %, "Materi x dari y · Lanjut: …" (D6) or "Kursus selesai" at 100%, progress bar; row links `/courses/[id]`. "+ Tambah kursus" at the bottom → `/catalog`. Empty: "Belum ada kursus." + the same button.
- Total jam belajar: donut (`visited_lectures / total_lectures`) with `total_minutes` as hours (one decimal, locale separator) in the centre; legend: Materi ditonton, Sisa materi, Kursus selesai (`completed_courses/enrolled_courses_count`). Empty when `total_lectures === 0` (existing copy).

### 8. Order detail + invoice
- Layout: **Ringkasan pesanan** left (`minmax(0,1.5fr)`), **Riwayat status** right (`minmax(0,1fr)`); one column below `md`. The left card holds items, the summary totals, shipping address/courier, and payment; the right card holds the existing timeline. Payment-pending banner and retry flow unchanged.
- Header action **Lihat invoice** opens a preview dialog with an A4-proportioned invoice: Abak logo + "Abak Academy" + current host, "INVOICE" + `INV/<order id>`, "Ditagihkan kepada" (student name + email from the auth user/profile; shipping address when present), order date, stamp **LUNAS** (`paid_at` set) or **BELUM DIBAYAR**, items table (Item · Qty · Harga · Jumlah), subtotal, discount (no code: the order only has `promo_code_id`), shipping (physical orders), total, footer "Pembayaran: {payment_method} · {paid_at}". No legal entity, NPWP, address, or signature is invented.
- Dialog actions: **Cetak** = `window.print()` with print CSS that shows only the invoice paper on one A4 page; **Unduh PDF** = generated in the browser (no Gotenberg, no API call), file `Invoice-<last 8 of id>.pdf`, button disabled while generating, toast on success and failure. Accepted trade-off: PDF text is not selectable.
- The invoice paper is light in both themes.
- The dead `invoice_url` row is removed from `PaymentInfo`. Visibility for cancelled/expired orders: D8.

## Technical contract

### Components and files

| File | Change |
|---|---|
| `lib/nav-config.ts` (+test) | `STUDENT_NAV` = 4 items above with `countKey` on Kompetisi and Tagihan; drop `nav_store`, `nav_profile` from it (keys stay; other roles/pages use them). `USER_MENU.student` per R2. |
| `components/shell/SidebarUserMenu.tsx`, `AppHeader.tsx`, `AppShell.tsx` (+tests) | Only the R2/R3/R5 extensions, if PR 1 lacks them. |
| `app/(student)/layout.tsx` (+test) | Keep every gate. Pass student counts and the student user-menu definition into the PR 1 shell API. |
| `lib/student-home.ts` (+test) | Pure helpers, no React: `kompetisiCount`, `pendingCount`, `pickShipment`, `shipmentSteps`, `heroEvents` (products × registrations → slides), `eventRows` (order + action kind), `courseRowText`. Kept out of `page.tsx` (Next page-export rule). |
| `components/dashboard/ShippingStrip.tsx` | New. |
| `components/dashboard/EventCarousel.tsx` | New; takes normalized slides, owns timer/pause/dots. |
| `components/dashboard/MyEventsCard.tsx` | New; registrations + per-row result query (D3). |
| `components/dashboard/ContinueLearningCard.tsx` | New. |
| `components/dashboard/StudySummaryCard.tsx` | Restyle to the 60% layout + legend; keeps `Donut`. |
| `app/(student)/page.tsx` (+test) | New order; delete `RankingCard`, `ExamProgressCard`, `PopularLessonsSection` and the helpers only they use (`GroupedBars`, `Ring`), which this change orphans. Dashboard API fields stay untouched. |
| `components/orders/InvoiceDialog.tsx` | New; uses PR 1 `Dialog`. |
| `lib/capture-pdf.ts` | New; single-page subset of Taruna's file (`toJpeg` with embedded font CSS → hand-written one-page image PDF → download). Port only `imagePdf`, `captureJpeg`, `download`, `savePageAsPdf`. |
| `app/(student)/orders/[id]/page.tsx` (+test) | Grid swap, invoice button, remove `invoice_url` row. |
| `app/globals.css` | `@media print` rule that hides everything but the invoice paper and sets A4 page size. Light-only invoice tokens if needed. |

Use `web/components/ui` primitives and v2 tokens only; no hex in components (roadmap rule). Accent only on primary CTAs and counts.

### Data per widget

| Widget | Hooks | Extra requests vs today |
|---|---|---|
| Sidebar counts | `useRegistrations`, `useOrders`, `useCartStore` | +2 (layout-level, cached by React Query) |
| Pending banner, courses, study | `useDashboard` | 0 |
| Shipping strip | `useOrders` | 0 (shared with sidebar) |
| Carousel | `useProducts("exam")`, `useRegistrations` | +1 |
| Event card | `useRegistrations`, `useSessionResult` per visible submitted row (≤4) | ≤4 (D3) |

### i18n (both locales, `lib/i18n.ts`)
New keys with prefix `stu_home_*` (greeting intro, ship titles/steps/resi/copy/detail, carousel headlines/CTAs/fallback/dot label, events card title/empty/actions/minutes/live, learn title/lesson-of/next/done/add, study legend), `stu_menu_*` (notifications, cart, profile & account), and `invoice_*` (title, bill-to, order date, paid, unpaid, columns, subtotal, discount, shipping, total, payment, print, download, generating, downloaded, failed). Reuse `nav_*`, `light_mode`, `dark_mode`, `logout`, `dash_view_all`, `dash_study_*`. Keys of removed widgets that become unused are deleted in both locales.

### New dependency: `html-to-image` (exact pin `1.11.13`)
- Why: rasterizing a styled DOM node in the browser needs a full CSS-inlining/serialization pass (fonts, computed styles, SVG foreignObject); hand-writing that is hundreds of lines. The PDF container itself is hand-written (~40 lines), so no PDF library is added.
- Why not Gotenberg: it needs an API round trip and server HTML assembly for a document the browser already renders; Panca chose browser-side.
- Precedent: the same version and code path run in Taruna (`bimbel-tryout/web/lib/capture-pdf.ts`).
- Size: ~20 KB min, zero runtime deps; imported with dynamic `import()` inside the download handler so it stays out of the route bundle.

## Dependencies (on PR 1, `admin-dashboard-v2.md`)

PR 2 is cut from `feat/public-landing` after PR 1 merges into it. It reuses PR 1's tokens, `AppShell` + `AppSidebar` + `SidebarUserMenu`, and the `NAV_CONFIG` / `USER_MENU` config (PR 1 D1 = (a), students keep the sidebar), and does not re-spec them. PR 1 as written (2026-11-25) gives: untitled group = flat rows; `NavItem.countKey?` + `AppSidebar counts?` (only if its D13 builds the slot); `USER_MENU[role]: { labelKey, icon, href, hintKey?, external? }[]` with Keluar appended; header keeps language, theme, cart (student), bell.

Requirements on PR 1 (R1, R4, R6 must be decided there; R2 and R3 are now part of PR 1's shell contract as of 2026-11-25; R5 stays in PR 2):

- R1. PR 1 D13 builds the count slot (`countKey` + `counts`), and `AppShell` forwards `counts` to `AppSidebar`.
- R2. `UserMenuItem` gains an action form (theme toggle; no `href`) and `countKey?`; `SidebarUserMenu` takes `counts?` and an optional card-dot count key. PR 2 sets `USER_MENU.student` = Keranjang `/cart` (cart count, also the card dot), Mode gelap/terang (action), Profil & akun `/profile`, plus Notifikasi per D4.
- R3. `AppHeader` hides theme, cart, and bell for the student role (PR 1 keeps them). Language toggle stays where PR 1 puts it (D5).
- R4. PR 1 D4 = (a) off-canvas drawer below 1024px, opened by the header menu button, closing on navigation and Escape.
- R5. `CartCountSync` keeps running for students; the cart count moves from the header badge to the menu.
- R6. `Dialog` v2 supports a wide variant (invoice preview ~820px) with its own scroll.

Any extension PR 2 makes to shared shell files is listed in its PR body with admin/affiliator tests still green.

## Acceptance criteria

1. As a student at `/`, `/exam`, `/courses`, `/orders`, `/orders/[id]`, `/catalog`, `/cart`, `/profile`, the sidebar shows exactly Beranda, Kompetisi, Kursus Saya, Tagihan (both locales); no Toko/Katalog/Profil item; no theme/cart/bell icon in the header.
2. Kompetisi badge equals the number of registrations in `start`/`checkin`/`in_progress`; Tagihan badge equals `payment_pending` orders; zero shows no badge.
3. The user card menu lists Keranjang (with count), Mode gelap/terang (toggles the theme), Profil & akun (`/profile`), Keluar (logs out to the student login), plus Notifikasi per D4; the card shows a dot with the cart count when > 0.
4. At 390px the header shows only the hamburger; it opens the sidebar drawer, which closes after navigating.
5. Links to `/catalog` from cart, orders, courses empty states, "Tambah kursus", and the carousel still load the catalog.
6. Home renders, in order: greeting, pending banner (when `pending_order`), shipping strip (when eligible), Row A, Row B. No ranking, exam-progress, popular-lessons, or "Ujian Terbaru" content remains.
7. Shipping strip appears for a newest `processing`/`shipped` physical order, shows the 4 steps with available dates, courier, and a resi copy button that writes the resi to the clipboard; it is absent when the newest such order is completed or the order has only exam/course items.
8. Carousel lists only eligible unregistered events in the agreed order; a registered one is absent; with none, the fallback slide shows and no dots render.
9. Carousel advances every 6s, stops while hovered or focused, does not autoplay under reduced motion; switching slides leaves the card height and dot positions unchanged (browser check); non-active slides are `inert`.
10. Event card shows ≤ 4 rows in the agreed order with the agreed action per state; "Lihat hasil" links to `/exam/sessions/<session_id>/leaderboard`.
11. Lanjut belajar rows show title, %, progress text, bar, and link to the course; "+ Tambah kursus" links `/catalog`. Total jam belajar shows hours and the three legend values from `study_summary`.
12. At 1366px Row A is 65/35 and Row B 40/60; at < 1200px both stack.
13. Order detail at ≥ md shows Ringkasan pesanan left (wider) and Riwayat status right; one column at 390px.
14. Lihat invoice opens the preview with all agreed fields, the correct LUNAS/BELUM DIBAYAR stamp, and no invented legal details.
15. Cetak prints only the invoice on one A4 page (print preview check, both themes).
16. Unduh PDF downloads `Invoice-<id8>.pdf` that opens as one A4 page matching the preview, without any network call except fonts already loaded; failures show a toast and re-enable the button.
17. No hex values added outside `globals.css`; `html-to-image` is pinned and loaded only on download.
18. Every new string exists in `id` and `en`; no orphaned i18n keys from removed widgets.

## Verification

- Unit (vitest, `docs/standards/web-tests.md`): `lib/student-home.test.ts` for every helper (counts, shipment pick + steps, hero eligibility/order, event row order/action, course text); component tests for `ShippingStrip`, `EventCarousel` (fake timers: autoplay, hover/focus pause, reduced-motion off, dots, `inert`), `MyEventsCard` (each action state, loading/error of the result query), `ContinueLearningCard`, `InvoiceDialog` (fields, stamp, print called, PDF helper called with the paper node; `html-to-image` mocked); updated `page.test.tsx`, `layout.test.tsx`, `nav-config.test.ts`, `orders/[id]/page.test.tsx`. Fixtures copy the Go payload shapes (nullable pointers as `null`), no real PII.
- `grep -rn` `web/e2e/*.spec.ts` for removed home/nav text and update any match.
- Gates from `web/`: `npx tsc --noEmit`, `npx vitest run`, `npm run build`.
- Browser (real Chrome against a local stack, seeded student with: a pending order, a processing physical order with resi, ≥ 2 open exam products, registrations in live/upcoming/submitted/expired states, 2 courses): 390px and 1366px, light and dark: shell, drawer, menu, home rows, carousel motion and stable height, order detail, invoice preview.
- PDF: download, open in a PDF viewer, confirm one A4 page, readable text at 100%, matching preview; print preview shows only the invoice.

## Non-goals

- Restyling `/exam`, `/courses`, `/catalog`, `/cart`, `/profile` beyond what the PR 1 shell and primitives give.
- Public catalog/landing (phase 4) and event-series data (phase 3).
- Any backend, migration, or dashboard-API change (unless D1/D3/D6 pick a backend option).
- Student notifications backend; referral code for students; "Pesanan sudah diterima" button and access buttons from the prototype order page.
- Server-side or text-selectable invoice PDF; invoice numbering scheme beyond `INV/<order id>`.

## Decisions needed

Recommendation first in each.

- **D1 Carousel source, "live" headline, and order.** The public product payload has no exam schedule and no popularity count.
  - a) Web-only now: slides from `GET /products?type=exam` (already "registration open" by its availability window); headline always "Pendaftaran dibuka" + "Daftar sekarang"; order by `available_until` ascending (closing soonest), nulls last, then name. "Sedang berlangsung" and level arrive when the source switches to event-series. Cheapest; no popularity ever without (c).
  - b) Add `scheduled_at`/`scheduled_end_at` to exam `relations` in the product payload (small backend read change) to enable the live headline now. Makes this PR cross-layer.
  - c) Add a public registration/sales count for "most popular first". Exposes sales volume to anyone; not recommended.
- **D2 Level on slides.** No level field exists on exam or product. a) Omit the level segment until event-series `open_editions.level`. b) Read a product spec row named "Jenjang"/"Kelas" if present: fragile label matching.
- **D3 Result released vs waiting.** The registration list has no release state or `allow_leaderboard`.
  - a) For each visible submitted row (≤ 4), call the existing result endpoint: `grading`/`locked` → "Menunggu hasil", `hidden` → "Selesai" chip without link, otherwise "Lihat hasil" → leaderboard (which already explains `leaderboard_not_available`). Web-only; cost ≤ 4 calls that load full pembahasan on each home visit.
  - b) Backend adds `result_state` + `allow_leaderboard` to `GET /exam/registrations` (needs an essay-graded check per row). One call, cross-layer.
- **D4 Notifikasi in the user menu.** No student notification endpoint exists; today's bell is decorative. a) Leave Notifikasi out until an endpoint exists. b) Show it without a count opening an empty "Belum ada notifikasi" dialog (dead UI).
- **D5 Language toggle.** Decision 2 removes the header icons and does not mention ID/EN; PR 1 keeps ID/EN in the header for all roles. a) Keep it there for students too: on desktop the header holds only ID/EN; at 390px it is already hidden (`sm:`), so mobile shows only the hamburger as agreed. b) Move ID/EN into the student user menu as a row.
- **D6 "Lanjut: <next lesson>".** The dashboard has no next-lesson title. a) Show "Materi {min(done+1,total)} dari {total}" without the lesson title. b) Fetch `GET /courses/:id` per enrolled course (N calls). c) Backend adds `next_lesson_title` to `enrolled_courses`.
- **D7 "Dikemas" date.** Orders have no processing timestamp. a) Mark the step reached with no date. b) Use the earliest `shipment_events.occurred_at`: Biteship only; manual waybills have none.
- **D8 Invoice for cancelled/expired orders.** a) Hide "Lihat invoice" for `cancelled` and `payment_expired`; show it for `payment_pending` (BELUM DIBAYAR) and paid statuses (LUNAS). b) Always show it, with a DIBATALKAN stamp.

## Implementation plan

Branch `feat/student-dashboard-v2` from `feat/public-landing` after PR 1 merges; one PR into `feat/public-landing`. The plan assumes the recommended options; a backend option in D1/D3/D6 adds one backend task before T3/T4/T5.

| id | title | files / scope | depends_on | test-first step | done-check |
|---|---|---|---|---|---|
| T0 | i18n contract | `lib/i18n.ts` (id + en keys listed above) | PR 1 | — (data only) | `tsc --noEmit` passes; keys exist in both locales |
| T1 | Student nav + shell wiring | `lib/nav-config.ts`(+test), `app/(student)/layout.tsx`(+test), `lib/student-home.ts` count helpers(+test), R2/R3/R5 shell extensions(+tests) | T0 | nav test expects the 4 hrefs; layout test expects badges and menu items (red) | tests green; 390/1366 browser check of sidebar, drawer, menu |
| T2 | Shipping strip | `lib/student-home.ts` `pickShipment`/`shipmentSteps`(+test), `components/dashboard/ShippingStrip.tsx`(+test) | T0 | helper cases: newest pick, digital-only excluded, completed hidden, step reach (red) | tests green |
| T3 | Event carousel | `heroEvents`(+test), `components/dashboard/EventCarousel.tsx`(+test) | T0 | eligibility/order cases; fake-timer autoplay/pause/reduced-motion (red) | tests green |
| T4 | Event & kompetisi card | `eventRows`(+test), `components/dashboard/MyEventsCard.tsx`(+test) | T0 | order + action-per-state cases incl. result states (red) | tests green |
| T5 | Learn + study cards | `courseRowText`(+test), `ContinueLearningCard.tsx`(+test), `StudySummaryCard.tsx` | T0 | row text/percent/empty cases (red) | tests green |
| T6 | Home assembly + cleanup | `app/(student)/page.tsx`(+test), delete `RankingCard`, `ExamProgressCard`, `PopularLessonsSection`, `GroupedBars`, `Ring`, orphan i18n keys | T2, T3, T4, T5 | page test asserts section order and removed widgets absent (red) | tests green; grep shows no imports of deleted files; e2e grep done |
| T7 | Invoice dialog + PDF | `package.json`/lock (`html-to-image@1.11.13`), `lib/capture-pdf.ts`, `components/orders/InvoiceDialog.tsx`(+test), print CSS in `globals.css` | T0 | dialog test: fields, stamp, Cetak calls print, Unduh calls the capture helper (red) | tests green; manual PDF + print preview check |
| T8 | Order detail layout | `app/(student)/orders/[id]/page.tsx`(+test) | T7 | test expects Lihat invoice button and no `invoice_url` row (red) | tests green; 390/1366 check |
| T9 | Integration verification | whole branch | T1, T6, T8 | — | `tsc`, `vitest run`, `npm run build` green; browser matrix and PDF check per Verification; `git diff --check` |

Parallel frontier after T0: **T1, T2, T3, T4, T5, T7** (disjoint files except `lib/student-home.ts`; T1–T5 each add their own functions and tests to it, so either run them sequentially on that file or let the first task create it and the others append in separate, non-overlapping sections). Then **T6** and **T8** in parallel, then **T9**.
