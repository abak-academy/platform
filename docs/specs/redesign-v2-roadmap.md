---
title: Redesign v2 roadmap (landing, student shell, admin)
issue: none yet
status: proposed
created: 2026-11-17
updated: 2026-11-26
sources: ["Claude artifact 'Abak Academy Design System' https://claude.ai/artifact/7vi4s85XFJ3BZfK1cpeiyp (private)", "docs/prototypes/redesign-v2/index.html", "docs/specs/admin-dashboard-v2.md", "docs/specs/student-dashboard-v2.md", "docs/specs/event-series.md", "docs/specs/testimonials.md", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/globals.css", "web/lib/nav-config.ts", "web/components/shell/AppSidebar.tsx", "web/components/admin/OrdersToolbar.tsx", "web/components/admin/ProductsToolbar.tsx", "docs/specs/admin-orders-toolbar-redesign.md"]
---

# Redesign v2 roadmap

## Problem and outcome

The approved clickable prototype (`docs/prototypes/redesign-v2/index.html`, reviewed by Panca on 2026-11-17) changes the look of every surface and reorganizes several flows. Building it in one PR would cross layers, touch money and exam flows, and be impossible to review. This roadmap splits it into phases. Each phase has its own spec and runs through `development-workflow`.

The prototype is the reference for **behavior, layout, hierarchy, and copy**. It is not a code source. Build with the tokens in `design-tokens-v2.md` (the in-repo copy of the design system's `tokens.json`), the primitives in `web/components/ui`, and the rules and inventory in `docs/prototypes/redesign-v2/COMPONENTS.md`. The design-system artifact ("Abak Academy Design System", https://claude.ai/artifact/7vi4s85XFJ3BZfK1cpeiyp, private) is the visual reference for components.

## Decisions already made (Panca, 2026-11-17)

- **Palette:** about 50% neutral (existing `paper`/`surface`/`ink`), 30% brand indigo (existing `brand-*`, `brand-600 #3d4ddb`), 20% terracotta accent (named `ember-*` since 2026-11-26, because `accent` is shadcn's neutral hover). The accent is only for primary CTAs, highlights, live badges, and counts. Selection, navigation, and focus use brand indigo.
- **Type:** Plus Jakarta Sans 800 for display, Public Sans for body, Spline Sans Mono for overlines.
- **Visuals:** the custom icon set and the floating math symbols (∑ π ∞ √ θ ∫) are kept, and symbols never overlap text. Avatars come from the 35-face sheet.
- **Public navbar:** Event · Program · Katalog · Tentang. Transparent at the top, blurred on scroll, active item marked with a dot. ID/EN toggle.
- **Student experience:** *(superseded 2026-11-25 by `student-dashboard-v2.md`: students keep a sidebar in the admin style, no Katalog in it)* the same navbar and footer as the public site, with no sidebar. Student navbar: Beranda · Kompetisi Saya · Event · Belajar · Katalog. The avatar menu has Tagihan, Profil & akun, and Keluar. Home is a dashboard only.
- **Landing:** competitions are grouped per event, with a story card per event (narrative, schedule, rounds, categories, facts, official site) instead of a list of every edition.
- **Admin sidebar:** an accordion, one group open at a time. Event + Ujian + Sekolah merge into **Akademik**. Group order: Akademik, Toko, Affiliator, Sistem.
- **Admin lists:** every list uses one toolbar (search · Filter popover · sort · active chips · count). Tabs only switch views of a single record.
- **Admin dashboard:** keeps the current analytics (period picker, KPI cards with change vs the previous period, revenue & order trend, student activity). **Digital vs physical** becomes a donut with amounts and shares plus the change in digital share, not stacked daily bars. The attention row (Perlu konfirmasi · Siap kirim · Esai perlu dinilai), upcoming exams, purchase feed, top products, and quick actions stay.

## Phases

| # | Spec | Scope | Layers | Depends on | State (2026-11-26) |
|---|------|-------|--------|------------|-------|
| 1 | `design-tokens-v2.md` | Tokens (= DS `tokens.json`), display font, `components/ui` restyle | web | — | In PR 1 |
| 2a | `admin-nav-accordion.md` | Admin sidebar regroup, order, accordion | web | 1 | In PR 1 |
| 2b | `admin-list-toolbar.md` | One `ListToolbar` for every admin list | web | 1 | Component built in the event-series PR (event-series D5 = a); migrating existing lists stays a later PR |
| 2c | *(in `admin-dashboard-v2.md`)* | Admin dashboard: donut, "Esai perlu dinilai" | web + small API | 1 | In PR 1 |
| 3 | `event-series.md` | Event entity (ID + EN), exam level + link, admin Event menu, public API incl. open editions with phase | migration `0075` + API + web | PR 1 | **In redesign scope**; plan ready |
| 3b | `testimonials.md` | Testimonial CMS with consent gate, required 4:5 photo, public API | migration `0076` + API + web | PR 1; admin `ListToolbar` | **In redesign scope**; plan ready |
| 4 | `landing-v2.md` | Public landing + Event page, ID/EN | web | PR 1, 3, 3b | Spec stale vs the 2026-11-26 hero; under review with Panca |
| 5 | `student-dashboard-v2.md` (supersedes `student-shell-v2.md`) | Student sidebar shell, home, order detail + invoice | web (+ html-to-image) | PR 1; 3 if RD1 puts it first | PR 2; plan ready |
| 6 | `profile-avatars.md` | Avatar sheet, photo upload with preview, password modal | web + small API + ops | 1 | Not scheduled; testimonials no longer use the avatar sheet (photo required since 2026-11-26) |

**Branching (Panca, 2026-11-26).** Every redesign PR is cut from `feat/public-landing` and targets it, not `main`, until all redesign work including the landing is done; then `feat/public-landing` goes to `main` as one release. Each PR branch is cut after the PRs it depends on have merged into `feat/public-landing`.

**Settled for PR 1 (Panca, 2026-11-26):** `admin-dashboard-v2.md` D2 (MD3 bridge), D3 (`ember-*`), D4 (drawer below 1024px), D13 (count slot, no data), D14 (full token set + radius repoint 10/14/20); content width 1480px; student wide Dialog 820px.

Migrations: only event-series (`0075`) and testimonials (`0076`); the latest on `feat/public-landing` @ 34ea5d32 is `0074`. Numbers are reserved in this order; whichever PR lands second re-checks.

### Decision needed — RD1: PR split and order after PR 1

Recommendation first. Panca decides.

- **a) Separate PRs, event-series before the student dashboard** (recommended): PR 1 admin → **PR 3 event-series** → PR 2 student dashboard ‖ **PR 4 testimonials** (parallel; both only need PR 1 and PR 3's `ListToolbar`) → PR 5 landing. The carousel is built once on real event data (open/live headline and level, student D1–D2 resolved by event-series); each PR has one migration and one admin menu item, so reviews stay small. Cost: the student PR waits for event-series.
- **b) One PR for event-series + testimonials, before the student dashboard:** PR 1 → PR 3 (both, two migrations, two admin pages, ~14 tasks) → PR 2 student → landing. Fewest PRs and one shared `ListToolbar`/media decision; the largest review in the redesign and it delays the student PR most.
- **c) Separate PRs, student dashboard first:** PR 1 → PR 2 student (carousel from `GET /products?type=exam`, "Pendaftaran dibuka" only, no level) → PR 3 event-series (+ swap the carousel adapter) → PR 4 testimonials → landing. Student work ships soonest; the carousel data path is built twice and the live headline/level arrive later.

Whatever is picked, `student-dashboard-v2.md` D1–D2 follow it (they are written as conditional), and the landing PR comes last because it needs 3 and 3b.

```text
(a)  PR1 ─▶ PR3 event-series ─┬─▶ PR2 student ─────┐
                              └─▶ PR4 testimonials ─┴─▶ PR5 landing ─▶ feat/public-landing → main
```

**Deployment** (which domain serves the public pages, the WordPress site, DNS, hosting) is not part of any phase. Panca and the client decide it at final deployment. Every phase must stay deployable to any host through config alone.

## Running a phase in Claude Code

1. Commit `docs/specs/*` and `docs/prototypes/redesign-v2/` on a docs branch first, so every phase branch starts from the same reference.
2. One phase per session. Start with:

   > Jalankan development-workflow untuk `docs/specs/<spec>.md`. Referensi visual: `docs/prototypes/redesign-v2/` (buka `index.html`). Patuhi aturan di `COMPONENTS.md`; prototipe bukan sumber kode. Mulai dari tahap align dan tanyakan setiap "Decisions needed" sebelum build.

3. Let the agent's align step turn the spec's open decisions into questions; answer them, and it records the answers in the spec (status → `agreed`).
4. Every phase spec now has an **Implementation plan** table; the workflow creates `.agents/work/<name>.json` from it.
5. Review each PR against the spec's Acceptance list plus `docs/standards/web-tests.md` (and the Go standards for phases 2c, 3, 3b).

## Non-goals for the whole roadmap

- No change to exam scoring, payment, or order status logic.
- No new dependencies unless a phase spec names and justifies one.
- The prototype's demo bar, sample data, and fake numbers are not built.

## Open questions (tracked in the phase specs)

- **RD1 — PR split and order** (above).
- Landing hero title is up to 92px, beyond `display-xl` 64: add `display-hero`? (`design-tokens-v2.md` D1)
- Event-series: admin_school menu (D2), exam level shape (D3), selling product of an edition (D4), `ListToolbar` in that PR (D5), hero `short_name` (D6), media prefix (D7, shared with testimonials D3).
- Testimonials: who manages them (D1), photo format and server-side check (D2).
- Stats numbers on the landing: static config or computed? (landing-v2 D3)
- EN narrative drafts in the seed need review before events go visible. (event-series, Seed content)

Settled 2026-11-17: event narratives are bilingual (ID + EN required to publish); testimonials get a CMS with a consent gate; the avatar sheet is open-licensed; deployment is decided by Panca and the client at the end. Settled 2026-11-26: event-series and testimonials are in the redesign scope; redesign PRs target `feat/public-landing`; PR 1 decisions listed under Phases.
