---
title: Redesign v2 roadmap (landing, student shell, admin)
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/globals.css", "web/lib/nav-config.ts", "web/components/shell/AppSidebar.tsx", "web/components/admin/OrdersToolbar.tsx", "web/components/admin/ProductsToolbar.tsx", "docs/specs/admin-orders-toolbar-redesign.md"]
---

# Redesign v2 roadmap

## Problem and outcome

The approved clickable prototype (`docs/prototypes/redesign-v2/index.html`, reviewed by Panca on 2026-11-17) changes the look of every surface and reorganizes several flows. Building it in one PR would cross layers, touch money and exam flows, and be impossible to review. This roadmap splits it into phases. Each phase has its own spec and runs through `development-workflow`.

The prototype is the reference for **behavior, layout, hierarchy, and copy**. It is not a code source. Build with the tokens in `design-tokens-v2.md`, the primitives in `web/components/ui`, and the rules and inventory in `docs/prototypes/redesign-v2/COMPONENTS.md`.

## Decisions already made (Panca, 2026-11-17)

- **Palette:** about 50% neutral (existing `paper`/`surface`/`ink`), 30% brand indigo (existing `brand-*`, `brand-600 #3d4ddb`), 20% terracotta accent. The accent is only for primary CTAs, highlights, live badges, and counts. Selection, navigation, and focus use brand indigo.
- **Type:** Plus Jakarta Sans 800 for display, Public Sans for body, Spline Sans Mono for overlines.
- **Visuals:** the custom icon set and the floating math symbols (∑ π ∞ √ θ ∫) are kept, and symbols never overlap text. Avatars come from the 35-face sheet.
- **Public navbar:** Event · Program · Katalog · Tentang. Transparent at the top, blurred on scroll, active item marked with a dot. ID/EN toggle.
- **Student experience:** *(superseded 2026-11-25 by `student-dashboard-v2.md`: students keep a sidebar in the admin style, no Katalog in it)* the same navbar and footer as the public site, with no sidebar. Student navbar: Beranda · Kompetisi Saya · Event · Belajar · Katalog. The avatar menu has Tagihan, Profil & akun, and Keluar. Home is a dashboard only.
- **Landing:** competitions are grouped per event, with a story card per event (narrative, schedule, rounds, categories, facts, official site) instead of a list of every edition.
- **Admin sidebar:** an accordion, one group open at a time. Event + Ujian + Sekolah merge into **Akademik**. Group order: Akademik, Toko, Affiliator, Sistem.
- **Admin lists:** every list uses one toolbar (search · Filter popover · sort · active chips · count). Tabs only switch views of a single record.
- **Admin dashboard:** keeps the current analytics (period picker, KPI cards with change vs the previous period, revenue & order trend, student activity). **Digital vs physical** becomes a donut with amounts and shares plus the change in digital share, not stacked daily bars. The attention row (Perlu konfirmasi · Siap kirim · Esai perlu dinilai), upcoming exams, purchase feed, top products, and quick actions stay.

## Phases

| # | Spec | Scope | Layers | Depends on |
|---|------|-------|--------|------------|
| 1 | `design-tokens-v2.md` | Tokens, display font, `components/ui` restyle | web | — |
| 2a | `admin-nav-accordion.md` | Admin sidebar regroup, order, accordion | web | 1 |
| 2b | `admin-list-toolbar.md` | One `ListToolbar` for every admin list | web | 1 |
| 2c | *(no spec; small change)* | Admin dashboard: donut for digital vs physical, "Esai perlu dinilai" attention card | web | 1 |
| 3 | `event-series.md` | Event entity (ID + EN narrative), admin Event menu, public API, exam ↔ event link | migration + API + web | 1, 2a |
| 3b | `testimonials.md` | Testimonial CMS with consent gate, public API | migration + API + web | 1, 2a |
| 4 | `landing-v2.md` | Public landing + Event page in Next.js, ID/EN | web | 1, 3, 3b |
| 5 | `student-dashboard-v2.md` (supersedes `student-shell-v2.md`) | Student sidebar shell, dashboard home, order detail + invoice | web | 1, 2a (PR 1 `admin-dashboard-v2.md`) |
| 6 | `profile-avatars.md` | Avatar sheet, photo upload with preview, password modal | web + small API + ops | 1 |

Update 2026-11-25: phases 1, 2a, and 2c ship as one PR, specified in `admin-dashboard-v2.md` (2c needs a small backend change, no migration); the student work follows in `student-dashboard-v2.md`.

Phases 2a, 2b, 2c, and 6 can run in parallel once 1 is merged; 3 and 3b can run in parallel after 2a. Phases 3 and 3b are the only ones with migrations.

**Deployment** (which domain serves the public pages, the WordPress site, DNS, hosting) is not part of any phase. Panca and the client decide it at final deployment. Every phase must stay deployable to any host through config alone.

## Running a phase in Claude Code

1. Commit `docs/specs/*` and `docs/prototypes/redesign-v2/` on a docs branch first, so every phase branch starts from the same reference.
2. One phase per session. Start with:

   > Jalankan development-workflow untuk `docs/specs/<spec>.md`. Referensi visual: `docs/prototypes/redesign-v2/` (buka `index.html`). Patuhi aturan di `COMPONENTS.md`; prototipe bukan sumber kode. Mulai dari tahap align dan tanyakan setiap "Decisions needed" sebelum build.

3. Let the agent's align step turn the spec's open decisions into questions; answer them, and it records the answers in the spec (status → `agreed`).
4. For specs with a task graph (event-series), the workflow creates `.agents/work/<name>.json` from it.
5. Review each PR against the spec's Acceptance list plus `docs/standards/web-tests.md` (and the Go standards for phase 3).

## Non-goals for the whole roadmap

- No change to exam scoring, payment, or order status logic.
- No new dependencies unless a phase spec names and justifies one.
- The prototype's demo bar, sample data, and fake numbers are not built.

## Open questions (tracked in the phase specs)

- Event admin role: should admin_school see the Event menu? (event-series D2)
- Stats numbers on the landing: static config or computed? (landing-v2 D3)
- EN narrative drafts in the seed need review before events go visible. (event-series, Seed content)

Settled 2026-11-17: event narratives are bilingual (ID + EN required to publish); testimonials get a CMS with a consent gate; the avatar sheet is open-licensed; deployment is decided by Panca and the client at the end.
