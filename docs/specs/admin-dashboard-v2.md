---
title: Admin redesign v2 — tokens, sidebar shell, and home dashboard (redesign PR 1)
issue: none yet
status: proposed
created: 2026-11-25
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/design-tokens-v2.md", "docs/specs/admin-nav-accordion.md", "docs/specs/student-shell-v2.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/globals.css", "web/app/layout.tsx", "web/app/(admin)/admin-theme.css", "web/app/(admin)/layout.tsx", "web/app/(admin)/admin/page.tsx", "web/components/shell/AppShell.tsx", "web/components/shell/AppSidebar.tsx", "web/components/shell/AppHeader.tsx", "web/lib/nav-config.ts", "web/lib/nav-config.test.ts", "web/components/admin/charts", "web/lib/types.ts", "backend/internal/service/admin_dashboard.go", "backend/internal/repository/exam.go"]
---

# Admin redesign v2 — tokens, sidebar shell, and home dashboard

PR 1 of the redesign (order: `redesign-v2-roadmap.md`). It bundles roadmap phases **1** (`design-tokens-v2.md`), **2a** (`admin-nav-accordion.md`), and **2c** (admin dashboard) into one PR. Those two specs stay the detailed contract for tokens and nav order; this spec adds the shell, the user menu, the dashboard, and the plan. PR 2 (`student-dashboard-v2.md`, another agent) depends on this PR's tokens and sidebar shell.

## Problem and outcome

The admin console still looks like the pre-v2 app: a separate Material-3 theme, a six-group always-expanded sidebar, the account menu in the top bar, and a dashboard whose digital/physical split is unreadable daily stacked bars. Essays waiting for grading are invisible on the home page.

Outcome: admin pages render with v2 tokens; the sidebar is the prototype's grouped accordion with a user card at the bottom that opens an upward menu; `/admin` matches the prototype's `#/admin` while keeping every analytic it has today. The student PR can reuse the sidebar and user menu by changing config only.

## Current behavior (verified on `feat/public-landing` @ 65136458)

**Tokens**
- `web/app/globals.css` has `brand-*`, `paper`/`surface`/`line`/`ink-*`, `--color-success|warn|gold(-bg)`, `--sh-*`. No terracotta family, no display font. `--color-accent` already exists as the shadcn hover/neutral (`@theme inline`, used by `dropdown-menu`, `select`, `button`, `badge`, `dialog`, `skeleton`).
- `layout.tsx` loads Public Sans, Source Serif 4, Spline Sans Mono, Cinzel, Inter. No Plus Jakarta Sans.
- **Admin does not use those tokens.** `(admin)/layout.tsx` and `(affiliate)/layout.tsx` import `(admin)/admin-theme.css`: an MD3 palette scoped to `.admin-shell` (`--md-sys-color-primary #1A5CFF`, Outfit + Roboto via a Google Fonts `@import`, `md-card-*`, `md-btn-*`, `text-title-*`). 54 non-test files under `app/` and `components/` use `md-sys-*`/`md-*` classes. Restyling `components/ui` alone would barely change admin pages.

**Shell** (`AppShell` is used by `(admin)`, `(student)`, and `(affiliate)` layouts)
- `AppShell.tsx`: fixed sidebar 252px at ≥1024px; below that a 64px icon rail that the header menu button expands. Adds `.admin-shell` for admin roles and affiliator.
- `AppSidebar.tsx`: groups from `NAV_CONFIG[role]`, `open: boolean[]` all `true`, each group toggles independently; group titles are 10px uppercase labels. Bottom: avatar + name + role label + a logout icon button. Logo links to `/`.
- `AppHeader.tsx`: sticky bar with a mobile menu button, a **non-functional search input** (no handler), ID/EN toggle, theme toggle, cart (students only), a **bell with no handler and a permanent red dot**, and an avatar dropdown (Profil → `/admin/profile` | `/affiliate/account` | `/profile`, Keluar).
- `nav-config.ts` super-admin: Beranda · Toko (Ringkasan, Produk, **Grup produk**, Kursus, Pesanan, Promo, Pendapatan, Notifikasi) · Ujian · Sekolah · Affiliator · Sistem. Keys `nav_group_exam`/`nav_group_school` are also used by `ADMIN_EXAM_NAV`/`ADMIN_SCHOOL_NAV`.

**Dashboard** (`app/(admin)/admin/page.tsx`, super_admin + `revenue:read` only)
- Order: `DashboardHero` gradient banner → `PeriodBar` → 4 KPI cards (revenue, orders, new students with % delta; schools with "N siswa total") → revenue & orders `AreaLineChart` → [student activity `MultiLineChart` | digital vs physical `StackedBarChart`] → attention row of 4 (`needs_confirm`, `ready_to_ship`, `shipment_failed` red when >0, `active_sessions` without link) → [upcoming exams (not links) | top products (name, digital/physical, revenue)] → [audit log "Log Aktivitas" (2/3) | quick actions: Buat soal, Tambah produk, Daftarkan siswa, Laporan penjualan].
- Chart colours are six contrast-verified hexes declared in the page.
- `GET /admin/dashboard` (`service.AdminDashboard`) returns `kpi` {revenue, order_count, new_students with `prev`; schools; students_total}, daily `series` incl. `revenue_digital`/`revenue_physical`, `attention` {4 counts}, `top_products` (incl. `qty_sold`), `upcoming_exams` {id, title, scheduled_at, registrant_count}. It already loads the previous window's series, but returns no previous digital/physical split and **no essay count**.
- Ungraded essays: only per exam (`ListSessionsNeedingGrading`, `GET /admin/exams/:id/grading`, the exam detail "grading" tab). No global count. The exam detail page does not read a tab from the URL.
- `StackedBarChart` is used only by this page (plus `chart-focus-ring.test.tsx`). `PurchaseNotificationFeed`/`useAdminNotifications` exist and are used on `/admin/notifications`.

## Prototype reference (`docs/prototypes/redesign-v2/index.html`)

- `adminSidebar()`: brand row; Beranda row; groups Akademik · Toko · Affiliator · Sistem, one open, the active route's group opens, a collapsed group header shows the sum of its item counts; active row brand-soft with a brand icon tile. `.side-foot` holds `details.side-umenu`: avatar + name + "Super admin" + chevron; the `.um-pop` opens **upward** (`bottom: 100% + 8px`), header (avatar, name, email · role), then **Akun & peran**, a **Bahasa · ID | EN** row, **Buka situs publik** (hint "landing page seperti dilihat pengunjung"), **Keluar** (dashed separator).
- `appHead()`: for non-student roles the header actions keep **theme toggle + bell**; the bell opens a "Perlu perhatian" list. No search box, no language toggle, no avatar in the header. Below 1024px the sidebar is an off-canvas drawer with a scrim and a close button; a menu button appears in the header.
- `studentNav()` (used by `stu()` for `#/app*`) uses the **same sidebar** with flat items (no referral card) and a user menu of Notifikasi, Keranjang, theme toggle, Bahasa · ID | EN, Profil & akun, Keluar. `publicNav()` is one navbar for everyone; a signed-in admin sees a **Dashboard** button → `#/admin` there (built by the landing PR).
- `#/admin`: greeting + date line "· ringkasan seluruh platform" + "Buat ujian" button → attention row (Perlu konfirmasi · Siap kirim · Esai perlu dinilai; icon, label, count, arrow) → period bar → 4 tinted KPI cards (icon tile, label, value, arrow + "+x% vs periode sebelumnya") → revenue & orders trend → [student activity | **donut**: digital % in the centre; rows Digital "kursus & kompetisi" and Fisik "buku, merchandise, medali" with amount and share; a note "Porsi digital naik/turun N poin dibanding periode sebelumnya"] → [upcoming exams (link, date · level, registrants, quota bar) | purchase feed with a Live chip] → [top products (rank, bar, qty) + link "Ringkasan penjualan" | quick actions: Tambah produk, Buat ujian, Kirim pengumuman, Buat promo].

## Agreed behavior

From the roadmap decisions of 2026-11-17 and the brief for this PR. Items marked *(inferred)* are mine and are repeated under Decisions needed where they are real choices.

**Tokens (phase 1).** Exactly `design-tokens-v2.md` (the in-repo copy of the design system's `tokens.json`, updated 2026-11-26): the full colour set with the terracotta family named `ember-*` + `on-ember` (D3), the twelve type styles with Plus Jakarta Sans as `--font-display`, spacing steps, radius tokens with the `--r-sm/md/lg` repoint 9/13/18 → 10/14/20 (D14), shadows, layout/size/z tokens, motion, reduced motion, and the primitive table incl. `Dialog size="wide"` (820px, student R6). The 50/30/20 usage rule holds: ember only for primary CTAs, highlights, live badges, counts; brand for selection, navigation, focus. Admin pages follow through the MD3 bridge (D2 = a).

**Sidebar (phase 2a).** Exactly `admin-nav-accordion.md` (Akademik merge, group order, one-open accordion, route-derived open group, collapsed-group count sum, `aria-expanded`/`aria-controls`, reduced motion), restyled to the prototype rows. Plus:
- A titled group renders as an accordion; an untitled group renders its items as flat rows. This is what lets the student PR keep a flat list.
- **User card** at the bottom of the sidebar for every role. Clicking it opens a menu above the card with a header (avatar, name, email · role label) and the role's items, then Keluar last.
- Admin roles' items: **Akun & peran** (D9), **Bahasa · ID | EN** (inline segmented toggle; same behaviour and persistence as today's header toggle), **Buka situs publik** with its hint (D10), **Keluar** (existing logout, role-correct login path).
- Student and affiliator: today's Profil item (→ their profile route), then **Bahasa · ID | EN**, then Keluar. The student PR replaces `USER_MENU.student`; affiliator keeps this.
- **Below 1024px** (D4 = a): the sidebar is an off-canvas drawer of the same 264px width over the `scrim`, `shadow-pop`, `z-drawer`, with a close icon button; the header menu button opens it; it closes on navigation, Escape, and scrim click. The 64px icon rail is removed. At ≥1024px the sidebar is 264px (was 252px).
- **Content width:** page content is centred and capped at `container-max` 1480px, replacing `max-w-[1340px]` in `AppShell`; gutter 32px, 16px below 680px.
- **Header:** admin roles keep only the **theme toggle and bell** (prototype). The language toggle and the avatar dropdown leave the header for every role; language lives in the user menu (Panca, 2026-11-26). The search box and the bell's behaviour are D11.

**Dashboard (phase 2c).** Keep every analytic and its data: period picker and its custom-range hold-back, KPI cards with change vs the previous period, revenue & order trend, student activity, upcoming exams, top products, quick actions, attention counts, loading/error/retry states. Changes, in render order:

| # | Area | Today | v2 |
|---|------|-------|----|
| 1 | Header | `DashboardHero` gradient banner | Page header: time-of-day greeting with the admin's first name, Jakarta date + "· ringkasan seluruh platform", ember (primary) "Buat ujian" button *(→ `/admin/exam/packages`, inferred: create lives there)* |
| 2 | Attention | Row of 4 below the charts | Row at the top, prototype card shape; composition D5; "Esai perlu dinilai" new (D6) |
| 3 | Period bar | Same component | Restyled only (segmented pill per tokens) |
| 4 | KPI | `md-card-elevated`, "+x%" | Tinted cards (brand/ember/teal/sky tones from tokens), arrow + "+x% vs periode sebelumnya"; same 4 metrics |
| 5 | Revenue trend | `AreaLineChart` | Unchanged chart, v2 card |
| 6 | Student activity | `MultiLineChart` | Unchanged chart, v2 card |
| 7 | Digital vs physical | `StackedBarChart` per day | Donut (digital share in the centre) + two rows with amount and share + "Porsi digital naik/turun/sama N poin dibanding periode sebelumnya"; sub-copy "Komposisi pendapatan periode ini. Fisik = buku, merchandise, medali." |
| 8 | Upcoming exams | Plain rows | Rows link to `/admin/exam/packages/{id}`; date + registrants. No quota bar or level (no data; non-goal) |
| 9 | Activity card | Audit log | D7 |
| 10 | Top products | Name (type) + revenue | Rank, name, relative bar by `qty_sold`, qty, revenue as secondary text; header link "Ringkasan penjualan" → `/admin/store` |
| 11 | Quick actions | 4 current tiles | D8 |

Share rules: share = round(digital / (digital + physical) × 100). The change line compares with the previous window's share in whole points. No change line when the previous window has no revenue. When the current window has no revenue, the card shows the existing empty label instead of a 0% donut.

## Technical contract

**Tokens**
- `globals.css` `@theme` + `html[data-theme="dark"]` only; no new CSS file. Reuse `--color-success|warn|gold` per `design-tokens-v2.md`.
- `layout.tsx`: add Plus Jakarta Sans (500–800) via `next/font/google` as `--font-display`. No new dependency.
- D2 bridge (settled): in `admin-theme.css`, point the `--md-sys-*` colour and typescale variables at v2 tokens and drop the Outfit/Roboto `@import`. Class names stay, so the 54 MD3 call sites change look without edits. Affiliate inherits it.
- `components/ui/*` keep their public props. List every call site whose meaning changes (Button `default` → ember) in the PR body.

**Nav config** (`web/lib/nav-config.ts`)
- Per `admin-nav-accordion.md`: `nav_group_academic` replaces `nav_group_exam` + `nav_group_school` in every admin role; super-admin order Beranda · Akademik · Toko · Affiliator · Sistem; optional `NavGroup.icon`; `notifications` nav entry moves to Akademik labelled by a new `nav_announcements` key (do not change the value of the shared `notifications` key). "Grup produk" stays in Toko after Produk in this PR (D12).
- Add `NavItem.countKey?: string`; `AppSidebar` takes `counts?: Record<string, number>` and `AppShell` forwards a `counts` prop to it (D13 = a: slot built and tested, no admin data wired; student R1).
- Add `USER_MENU: Record<UserRole, UserMenuItem[]>` next to `NAV_CONFIG`. `UserMenuItem` = `{ labelKey, icon, href?, action?: 'theme' | 'language', countKey?, hintKey?, external? }` (an item has `href` or `action`; `countKey` shows a count badge; `language` renders the "Bahasa · ID | EN" row with an inline ID | EN switch that reuses the existing language store, and does not close the menu). `SidebarUserMenu` takes `counts?` and an optional `dotCountKey` shown on the closed card — student PR requirement R2. Keluar is not an item; the menu always appends it. The student PR changes only `USER_MENU.student` and `NAV_CONFIG.student`.

**Shell** (`web/components/shell`)
- `AppSidebar` (changed): props `role` plus `counts?`; `collapsed?` goes with the rail (D4 = a), so remove it and its callers' use. Holds `openKey: string | null` derived from the pathname on route change; keeps `isNavActive`. Renders `SidebarUserMenu` in the footer.
- `SidebarUserMenu` (new, `components/shell/SidebarUserMenu.tsx`): props `{ role }`; reads user and role label the same way `AppSidebar` does today, renders `USER_MENU[role]` with the existing `DropdownMenu` (`side="top"`, `align="start"`, trigger width) so Escape, outside click, and keyboard navigation come from radix. External items open in a new tab with `rel="noopener noreferrer"`.
- `AppShell` (changed): drawer below 1024px per D4 = a (focus moves into the drawer on open and back to the menu button on close); content wrapper `mx-auto w-full max-w-page` (1480px) with the v2 gutters; forwards `counts`. Every role keeps one shell.
- `AppHeader` (changed): drop the avatar dropdown and the language toggle for all roles (the user card and its Bahasa row replace them); keep the menu button. Theme and bell stay for admin roles; for the student role the header shows no theme, cart, or bell (they move into the student user menu in PR 2 — requirement R3). Search box per D11.
- The student PR reuses `AppShell` + `AppSidebar` + `SidebarUserMenu` as is (D1 settled: sidebar shell).

**Dashboard**
- Backend (no migration): `service.AdminDashboard` adds `kpi.revenue_digital` and `kpi.revenue_physical` via `makeKPI` over the series it already loads (current + `prev`); `attention.essays_to_grade` = number of submitted sessions with at least one ungraded essay answer across all exams (same predicate as `ListSessionsNeedingGrading`), plus the D6 field. New repository count lives beside `CountActiveExamSessions` in `dashboard_counts.go`. `web/lib/types.ts` `AdminDashboard` mirrors the new fields (`prev` optional, new attention fields required numbers; a nullable Go pointer is `string | null`).
- Web: `components/ui/ring.tsx` (new, per `COMPONENTS.md`: conic-gradient ring, `value` 0–100, centre slot, tokens only, `role="img"` + label). `components/admin/RevenueMixCard.tsx` (new) owns the share maths and copy so the page stays declarative and the maths is unit-testable outside `page.tsx`. `StackedBarChart.tsx`, its test, its `chart-focus-ring` case, and its `.chart-grow` CSS are deleted only if nothing else imports them after the change (grep first). `DashboardHero` stays (used by exam/school/store/affiliate dashboards).
- Existing chart hexes in the page stay (contrast-verified; the token rule forbids *adding* hexes).

**i18n** (`web/lib/i18n.ts`, both `id` and `en`)
- Add: `nav_group_academic` (Akademik / Academics), `nav_announcements` (Pengumuman / Announcements), `user_menu_label` (Menu akun / Account menu), `user_menu_account_role` (Akun & peran / Account & role), `user_menu_language` (Bahasa / Language), `user_menu_public_site` (Buka situs publik / Open public site), `user_menu_public_site_hint` (landing page seperti dilihat pengunjung / the landing page as visitors see it), `admin_home_greet_morning|afternoon|evening|night` (Selamat pagi/siang/sore/malam; boundaries 11, 15, 18 Jakarta time as in the prototype), `admin_home_overview_suffix` (ringkasan seluruh platform / platform overview), `admin_home_create_exam` (Buat ujian / Create exam), `admin_home_essays_to_grade` (Esai perlu dinilai / Essays to grade), `admin_home_vs_prev` (vs periode sebelumnya / vs previous period), `admin_home_mix_digital_hint` (kursus & kompetisi / courses & competitions), `admin_home_mix_physical_hint` (buku, merchandise, medali / books, merchandise, medals), `admin_home_mix_share_up|down|same` (with `{n}`), `admin_home_sales_summary` (Ringkasan penjualan / Sales summary), plus D7/D8 labels.
- Change: `admin_home_chart_mix_sub` to the v2 sub-copy.
- Remove only keys this PR orphans (`nav_group_exam`, `nav_group_school`, `admin_home_badge`, `admin_home_subtitle`, …) after grepping `web/` incl. tests and `e2e/`.

**Branching.** Worktree `.worktrees/admin-dashboard-v2`, branch `feat/admin-dashboard-v2` cut from `feat/public-landing`; the PR targets `feat/public-landing`, not `main`. One PR for this whole spec.

## Acceptance criteria

1. Tokens: every row of `design-tokens-v2.md` § Acceptance holds (screenshots, no new hex outside `globals.css` by grep of the diff, token values match the tables, contrast table incl. `on-ember` on `ember-600`); `rounded-md` resolves to 14px; no `accent-50|500|600|700` class exists (grep).
2. Admin pages using MD3 classes (e.g. `/admin/orders`, `/admin/exam/packages`) render in v2 colours and fonts in light and dark, and no request goes to `fonts.googleapis.com` for Outfit/Roboto (D2 = bridge).
3. `nav-config.test.ts` asserts the super-admin order Beranda · Akademik · Toko · Affiliator · Sistem with the item order of `admin-nav-accordion.md`, and the same group names/order for `admin_exam`, `admin_school`, `admin_store`; no group with zero items renders.
4. Opening one sidebar group closes the others; clicking the open header closes it; visiting `/admin/exam/tests` directly shows Akademik open and the rest closed; a collapsed group given counts shows their sum; under `prefers-reduced-motion` no transition class is applied (component tests).
5. The user card shows avatar, name, role label; activating it opens a menu positioned above it containing exactly, in order, Akun & peran, Bahasa · ID | EN, Buka situs publik (with hint), Keluar for every admin role; switching ID ↔ EN in the row changes the UI language and persists like today's toggle; Escape closes the menu and returns focus to the card; Keluar calls logout and lands on `/admin/login`.
6. Student and affiliator sidebars still show their current items and a user menu with Profil, Bahasa · ID | EN, Keluar; their routes and logout destinations are unchanged (existing `AppShell`/layout tests pass).
7. Below 1024px the header menu button opens a 264px drawer over a scrim that closes on navigation, Escape, and scrim click, with focus returned to the menu button; at ≥1024px no rail exists. Page content is capped at 1480px and centred (`AppShell` test + 1600px browser check). `Dialog size="wide"` renders at 820px with an inner scroll.
8. The admin header shows theme and bell only (no language toggle, no avatar dropdown); no role's header has a language toggle; the student header shows no theme, cart, or bell. `SidebarUserMenu` renders action items, item counts, and the card dot from `counts`.
9. `GET /admin/dashboard` returns `kpi.revenue_digital` and `kpi.revenue_physical` with `prev` when the previous window has data, and `attention.essays_to_grade` equal to sessions with ≥1 ungraded essay across exams (Go repository + service tests against Postgres, incl. a fully-graded session that must not count and a resubmitted attempt rule matching `ListSessionsNeedingGrading`).
10. `/admin` renders sections in the v2 order of the table above; existing page tests for deltas, attention links, upcoming exams, quick actions, loading skeletons, error, and retry still pass or are rewritten with the same assertions.
11. Digital vs physical: given a fixture, the card shows the centre share, both amounts formatted with `formatRupiah`, both shares summing to 100, and the correct up/down/same line; no change line without previous revenue; the empty label when current revenue is 0.
12. "Esai perlu dinilai" shows `essays_to_grade` and links per D6.
13. Every new string exists in `id` and `en`; no removed key is still referenced (grep).
14. `npx tsc --noEmit`, `npx vitest run`, `npm run build` pass in `web/`; `./deploy/pipeline/backend.sh` passes.

## Verification

- Unit/component (vitest, `docs/standards/web-tests.md`): `nav-config.test.ts`, `AppSidebar.test.tsx`, new `SidebarUserMenu.test.tsx`, `AppHeader.test.tsx`, `AppShell.test.tsx`, `(admin)/layout.test.tsx`, `ui/ring.test.tsx`, `RevenueMixCard.test.tsx`, `(admin)/admin/page.test.tsx` + `page.error*.test.tsx`. Fixtures copy the Go response shape. Grep `web/e2e/*.spec.ts` for any flow whose unit expectation changes (none found today for dashboard/sidebar; re-check).
- Go: repository test for the essay count in `dashboard_counts_test.go`; service test for the new KPI keys in `admin_dashboard_test.go`; full `backend.sh` (set `GOROOT`, `DOCKER_HOST` per `.agents/verification.md`).
- Frontend gates after the last edit: `npx tsc --noEmit`, `npx vitest run`, `npm run build`.
- Browser (real Chrome, not jsdom): `/admin`, `/admin/orders`, `/admin/exam/tests`, and one student page at 390px and 1366px, light and dark; check the user menu opens upward without clipping at 390px, the accordion, the donut, and the longest-locale (EN) labels.
- `verify-abak` skill: bring up the disposable stack, log in as super admin, capture `/admin` screenshots + ARIA snapshot of the sidebar and user menu, open and close a group, sign out through the menu.
- `git status --short`, `git diff --check`.

## Non-goals

- Admin list toolbar (phase 2b), Event/Testimoni menu items and their migrations (phases 3, 3b), landing (4), student nav, student menu items, and student pages (PR 2).
- Upcoming-exam quota bar and level, sidebar attention counts wired to data (D13), the bell's "Perlu perhatian" panel, an icon-set swap.
- The public navbar and its Dashboard button for signed-in admins (`landing-v2.md`).
- Per-page migration off `md-*` classes (the D2 bridge covers looks only).
- Any change to order, payment, grading, or scoring logic.

## Decisions needed

Recommendation first in each.

- ~~**D1 — Student shell direction.**~~ **Settled (Panca, 2026-11-25):** students keep the sidebar shell; PR 2 configures `NAV_CONFIG.student` / `USER_MENU.student` (see `student-dashboard-v2.md`). `student-shell-v2.md` is deprecated.
- ~~**D2 — Admin MD3 theme.**~~ **Settled (Panca, 2026-11-26): (a) bridge.** Was: (a) Bridge: remap `--md-sys-*` and the typescale in `admin-theme.css` to v2 tokens, drop the Outfit/Roboto import; one file, all admin + affiliate pages follow. (b) Migrate only the shell and dashboard; other admin pages stay MD3 blue until later (visible inconsistency). (c) Migrate all 54 files off `md-*` (large, out of scope).
- ~~**D3 — Terracotta token name.**~~ **Settled (Panca, 2026-11-26): (a) `ember-*`** (+ `on-ember`); `design-tokens-v2.md` already updated. Was: (a) `ember-50/500/600/700` (the prototype's name), because `--color-accent`/`bg-accent` is already the shadcn neutral hover used by six primitives. (b) `accent-*` as written in `design-tokens-v2.md`, accepting `bg-accent` ≠ `bg-accent-600` confusion. Either way, update `design-tokens-v2.md` in this PR.
- ~~**D4 — Below 1024px.**~~ **Settled (Panca, 2026-11-26): (a) off-canvas drawer** (264px, scrim, `z-drawer`). Was: (a) Off-canvas drawer with scrim and close button (prototype; the user card and accordion stay usable on phones). (b) Keep the 64px icon rail (`admin-nav-accordion.md` says "unchanged"); the user menu shrinks to an avatar.
- **D5 — Attention row.** (a) Perlu konfirmasi · Siap kirim · Esai perlu dinilai, plus Gagal kirim only when > 0; active sessions move to a "N sesi berlangsung" chip on the Upcoming exams header linking `/admin/exam/monitor`. (b) Exactly the prototype's three; shipment-failed and active sessions leave the home page. (c) All five cards.
- **D6 — Where "Esai perlu dinilai" goes.** (a) Backend also returns `essays_exam_id` (the exam with the oldest ungraded submission, `null` when 0); the card links to `/admin/exam/packages/{id}?tab=grading`, and the exam detail page learns to read `tab` from the URL (small, tested). (b) Link to `/admin/exam/packages` (the admin hunts for the exam). (c) Count only, no link.
- **D7 — Activity card.** (a) Purchase feed from the existing `useAdminNotifications` with a Live chip (prototype; the roadmap says the "purchase feed stays", but today's page has an **audit log**, not a feed); the audit log remains at Sistem › Audit. (b) Keep the audit log, restyled. (c) Both.
- **D8 — Quick actions.** (a) Prototype set: Tambah produk, Buat ujian, Kirim pengumuman, Buat promo. (b) Keep today's: Buat soal, Tambah produk, Daftarkan siswa, Laporan penjualan.
- **D9 — "Akun & peran" target.** (a) `/admin/profile` (own account and role; reachable by every admin role). (b) `/admin/system/accounts` as in the prototype (super-admin only; duplicates the Sistem item).
- **D10 — "Buka situs publik" URL.** No public landing exists in Next yet and `/` redirects admins to `/admin`; the landing PR (last in the redesign, same release) adds it, and its public navbar gives a signed-in admin a **Dashboard** button back to `/admin`. (a) Build-time `NEXT_PUBLIC_SITE_URL`; the item is hidden when unset, so any host works by config (roadmap deployment rule; the landing spec uses the same variable for canonical URLs). (b) Hard-code the landing route and let the landing PR make it render for admins. (c) Omit the item until the landing PR.
- **D11 — Header leftovers.** (a) Remove the non-functional search box; keep the bell as is and flag that its red dot is permanent. (b) Keep both as today. Wiring the bell is a non-goal either way.
- ~~**D12 — "Grup produk" (`/admin/product-groupings`).**~~ **Settled (Panca, 2026-11-26):** product grouping becomes the Event (`event-series.md`). This PR keeps "Grup produk" in Toko right after Produk, unchanged; the event-series PR removes it and adds "Event" as the first Akademik item.
- ~~**D13 — Sidebar counts.**~~ **Settled (Panca, 2026-11-26): (a) build the count slot, wire no data.** Was: (a) Build and test the count slot only; wire no data (no count endpoint exists for non-super-admin roles). (b) Wire super_admin counts from the dashboard query (extra request on every admin page).
- ~~**D14 — Token scope in this PR.**~~ **Settled (Panca, 2026-11-26): (a) the full token set, incl. the radius repoint.** Was: (a) All of `design-tokens-v2.md`, including Button `default` → accent on every call site, because PR 2 depends on it. (b) Tokens + only the primitives the shell and dashboard use; the rest later.

## Implementation plan

Tasks for `.agents/work/admin-dashboard-v2.json` (parent creates it after both approvals). Test-first per `tdd-implementation`.

| ID | Title | Files / scope | depends_on | Test-first step | Done-check |
|----|-------|---------------|------------|-----------------|------------|
| T1 | Tokens v2 + primitives | `web/app/globals.css` (full `design-tokens-v2.md` set: `ember-*`, repoints, radius 10/14/20, shadows, layout/size/z, `type-*` utilities), `web/app/layout.tsx`, `web/components/ui/*` incl. `Dialog size="wide"` | — | Update/add class assertions in existing `components/ui/*.test.tsx` for the new variants and the wide Dialog (fail first) | ui tests green; grep shows no new hex outside `globals.css` and no `accent-<n>` class; contrast table in PR body; dark check of the `brand-800`/`brand-900` call sites |
| T2 | Admin MD3 bridge | `web/app/(admin)/admin-theme.css` (D2) | T1 | A theme test asserting `--md-sys-color-primary` resolves to the brand token in light **and** dark (values must differ between themes) | `/admin/orders` light/dark screenshot; no Outfit/Roboto request |
| T3 | Nav regroup | `web/lib/nav-config.ts`, `web/lib/nav-config.test.ts`, `web/lib/i18n.ts` (nav keys), `USER_MENU` | — | Rewrite `nav-config.test.ts` order/group assertions per role (fail) | test green; grep: no `nav_group_exam`/`nav_group_school` left |
| T4 | Dashboard API fields | `backend/internal/service/admin_dashboard.go`, `backend/internal/repository/dashboard_counts.go` (+ tests), handler unchanged; D6 field | — | Repository test: fully graded, ungraded, other-exam, and resubmitted sessions → expected count (fail) | `go test -run` for both packages, then `backend.sh` green |
| T5 | Sidebar accordion + user menu | `AppSidebar.tsx` (264px, `counts` slot), new `SidebarUserMenu.tsx` (incl. the `language` action row), their tests, i18n user-menu keys | T1, T3 | Component tests for AC 4 and 5 (one-open, route-open, count sum, reduced motion, menu items in order, language switch, Escape, logout path) | tests green; AC 4–6 |
| T6 | Shell + header | `AppShell.tsx` (drawer, 1480px content cap, 32/16 gutters, `counts` forwarding), `AppHeader.tsx`, their tests, `(admin)/layout.test.tsx` (D11) | T5 | Header test: admin has theme + bell and no language toggle or avatar dropdown; student has no theme/cart/bell/language; SidebarUserMenu action item + counts + card dot; drawer opens/closes on navigation, Escape, scrim; no rail; content cap class | tests green; AC 6–8 |
| T7 | Ring + revenue mix card | new `ui/ring.tsx`, `components/admin/RevenueMixCard.tsx`, tests | T1 | Share maths cases: up/down/same, no previous revenue, zero current revenue, shares sum to 100 | tests green; AC 11 |
| T8 | Dashboard page v2 | `app/(admin)/admin/page.tsx` + its tests, `web/lib/types.ts`, i18n dashboard keys; delete `StackedBarChart` + test + CSS if orphaned; exam detail `tab` param if D6 = a | T2, T4, T7 | Rewrite page tests for the new section order, attention composition (D5), essay link (D6), feed (D7), actions (D8) | tests green; AC 10–13 |
| T9 | Integration verification | whole branch | T6, T8 | — | `tsc`, `vitest`, `npm run build`, `backend.sh`; browser 390/1366 light/dark; `verify-abak` evidence; AC 1–14 each with evidence |

Parallel frontiers (scopes do not overlap): **{T1, T3, T4}** → **{T2, T5, T7}** → **{T6, T8}** → T9. `i18n.ts` is shared: T3 adds nav keys, T5 user-menu keys, T7 the mix-card keys, T8 the rest of the dashboard keys. T5 and T7 run concurrently, so each adds its keys in its own anchored block in both locales to avoid textual conflicts.
