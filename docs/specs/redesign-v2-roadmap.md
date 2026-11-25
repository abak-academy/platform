---
title: Redesign v2 roadmap (admin, events, testimonials, student, public site)
issue: none yet
status: proposed
created: 2026-11-17
updated: 2026-11-26
sources: ["Claude artifact 'Abak Academy Design System' https://claude.ai/artifact/7vi4s85XFJ3BZfK1cpeiyp (private)", "docs/prototypes/redesign-v2/index.html", "docs/specs/design-tokens-v2.md", "docs/specs/admin-dashboard-v2.md", "docs/specs/event-series.md", "docs/specs/testimonials.md", "docs/specs/student-dashboard-v2.md", "docs/specs/landing-v2.md", "docs/prototypes/redesign-v2/COMPONENTS.md", "docs/specs/admin-nav-accordion.md", "docs/specs/admin-list-toolbar.md"]
---

# Redesign v2 roadmap

## Problem and outcome

The approved clickable prototype (`docs/prototypes/redesign-v2/index.html`, the reference as of 2026-11-26) changes the look of every surface and reorganizes several flows. Building it in one PR would cross layers, touch money and exam flows, and be impossible to review. This roadmap splits it into five PRs, each with its own spec and implementation plan, each run through `development-workflow`.

The prototype is the reference for **behavior, layout, hierarchy, and copy**, not a code source. Build with the tokens in `design-tokens-v2.md` (the in-repo copy of the design system's `tokens.json`), the primitives in `web/components/ui`, and the rules in `docs/prototypes/redesign-v2/COMPONENTS.md`. The design-system artifact (private link above) is the visual reference for components.

## Decisions already made (Panca)

- **Palette (11-17):** ~50% neutral (`paper`/`surface`/`ink`), ~30% brand indigo (`brand-600 #3d4ddb`), ~20% terracotta `ember-*` (named so on 11-26 because `accent` is shadcn's neutral hover). Ember only for primary CTAs, highlights, live badges, counts; selection, navigation, focus are brand.
- **Type (11-17):** Plus Jakarta Sans 800 display, Public Sans body, Spline Sans Mono overlines.
- **Visuals (11-17):** custom icon set (swap deferred), floating math symbols that never overlap text.
- **Layout (11-26):** dashboard shell content max 1480px, gutter 32/16, sidebar 264px; public site 1200px container, 80px before the footer, page-header tint fades out (`design-tokens-v2.md` § Layout).
- **Dashboard shell (11-25/26):** admin and student share one sidebar shell with a user card menu at the bottom; the language toggle lives in that menu (Bahasa · ID | EN) for every role; the admin header keeps theme + bell only; students get no header icons. Below 1024px the sidebar is a drawer.
- **Admin:** accordion sidebar, groups Akademik · Toko · Affiliator · Sistem; every list uses one toolbar; dashboard keeps today's analytics with a digital/physical donut and an "Esai perlu dinilai" card.
- **Student (11-25/26):** sidebar Beranda · Kompetisi · Kursus Saya · Tagihan; Beranda = greeting → pending banner → shipping strip → event carousel | Event & kompetisi → Lanjut belajar | Total jam belajar → Dari Toko; logged-in cart inside the shell; order detail with invoice preview, print, browser PDF.
- **Public site (11-26):** one navbar for everyone (Event · Program · Toko, ID/EN, Masuk | Daftar gratis; signed-in users get a **Dashboard** button → `/` or `/admin`); full-bleed photo hero; Program page; Toko public with a client-side guest cart; "Katalog" is called **Toko / Shop** in every user-facing string (route `/catalog`).
- **Events (11-26):** product grouping **becomes** the Event (no parallel table); admins also upload results of off-platform rounds per edition.
- **Testimonials (11-17/26):** CMS with a consent gate and a required 4:5 JPEG photo; `super_admin` only.
- **Branching (11-26):** every redesign PR is cut from `feat/public-landing` and targets it; when all five are merged, `feat/public-landing` goes to `main` as one release. Each PR branch is cut after the PRs it depends on have merged.
- **RD1 — PR order (11-26):** settled as option (a) below.
- **Deployment** (domain, WordPress, DNS, hosting) is not part of any PR; Panca and the client decide it at final deployment. Every PR stays deployable to any host by config.

## PRs

| PR | Spec | Scope | Layers | Depends on |
|----|------|-------|--------|------------|
| 1 | `admin-dashboard-v2.md` (+ `design-tokens-v2.md`, `admin-nav-accordion.md`) | Full token set + primitives restyle, MD3 bridge, shared sidebar shell + user menu + drawer, admin nav regroup, admin dashboard | web + small API | — |
| 3 | `event-series.md` | Product grouping → Event (narrative ID/EN, schedule, logo, visibility), edition level, public `/event-series` API, generic admin `ListToolbar` (its D5), off-platform round results upload + student `external_results`; "Grup produk" → Akademik › Event | migration `0075` (alters `product_grouping`, adds schedule + external-result tables) + API + web | PR 1 |
| 2 | `student-dashboard-v2.md` | Student nav + user menu, Beranda, Dari Toko, order detail + invoice | web (+ `html-to-image`) | PR 1, PR 3 |
| 4 | `testimonials.md` | Testimonial CMS with consent gate and photo, public API | migration `0076` + API + web | PR 1, PR 3 (`ListToolbar`) |
| 5 | `landing-v2.md` (spec + implementation plan, one PR) | Public navbar/footer, landing, `/event`, `/program`, Toko public, client-side guest cart (no backend), Dashboard button | web | PR 1, PR 2 (shell, count helpers), PR 3, PR 4 |

```text
PR1 admin ─▶ PR3 event-series ─┬─▶ PR2 student ──────┐
                               └─▶ PR4 testimonials ─┴─▶ PR5 public site ─▶ feat/public-landing → main
```

Not scheduled: phase 2b (`admin-list-toolbar.md`, migrating existing admin lists to the toolbar), `profile-avatars.md` (testimonials no longer use the avatar sheet).

**Migrations:** `0075` (event-series) and `0076` (testimonials); the latest on `main` and `feat/public-landing` is `0074`. `0075` **alters** `product_grouping` (from `0073`) instead of creating an `event_series` table. Numbers are reserved in merge order; whichever lands second re-checks.

### RD1 — PR order after PR 1 (settled, Panca 2026-11-26: a)

- **a) chosen:** PR 1 → PR 3 event-series → PR 2 student ‖ PR 4 testimonials → PR 5 public site. The carousel is built once on real event data; each PR has one migration and small reviews; the student PR waits for event-series.
- Not chosen: b) one PR for event-series + testimonials (largest review, delays the student PR most); c) student first with a products-based carousel adapter swapped later (data path built twice).

Consequences: `student-dashboard-v2.md` D1–D2 are settled on the event-series source; testimonials reuses PR 3's `ListToolbar`.

## Running a PR in Claude Code

1. Specs and the prototype live on `feat/public-landing`; every PR branch starts from it.
2. One PR per session: "Jalankan development-workflow untuk `docs/specs/<spec>.md`. Referensi visual: `docs/prototypes/redesign-v2/index.html`. Patuhi `COMPONENTS.md`; prototipe bukan sumber kode. Mulai dari tahap align dan tanyakan setiap Decision needed sebelum build."
3. The align step turns each open decision below into a question; answers are recorded in the spec.
4. Each spec's **Implementation plan** becomes `.agents/work/<name>.json`.
5. Review against the spec's Acceptance list, `docs/standards/web-tests.md`, and the Go standards for PRs with backend work.

## Non-goals for the whole roadmap

- No change to exam scoring, payment, or order status logic.
- No new dependency unless a spec names and justifies it (so far only `html-to-image` in PR 2).
- The prototype's demo bar, sample data, and fake numbers are not built.

## Open decisions across the redesign

One line each, recommendation in brackets. Settled items are recorded in each spec.

**design-tokens-v2.md**
- D1 — Landing hero title (~92px) beyond `display-xl` 64: add `display-hero`? [a: add it, landing hero only]

**admin-dashboard-v2.md (PR 1)**
- D5 — Attention row composition [a: three cards + "Gagal kirim" only when > 0; active sessions as a chip on Upcoming exams]
- D6 — Where "Esai perlu dinilai" links [a: oldest exam with ungraded essays, `?tab=grading`]
- D7 — Activity card [a: purchase feed with Live chip; audit log stays at Sistem › Audit]
- D8 — Quick actions [a: prototype set]
- D9 — "Akun & peran" target [a: `/admin/profile`]
- D10 — "Buka situs publik" URL [a: `NEXT_PUBLIC_SITE_URL`, hidden when unset; must match landing D4]
- D11 — Header leftovers [a: remove the dead search box; bell unchanged]

**event-series.md (PR 3)**
- D2 — Who manages events [a: `super_admin` + `admin_store` (today) + `admin_exam`]
- D3 — Level location [a: on the edition product]
- D5 — Admin `ListToolbar` in this PR [a: build the generic component here]
- D6 — Hero rotator short names [a: nullable `short_name`]
- D8 — Naming of the evolved grouping [a: keep DB/API/hook names, UI says Event, new `/event-series` public endpoints; no compat window]
- D9 — Non-competition groupings in prod [a: all become hidden events; inventory prod first (T0)]
- D10 — Results file format [a: CSV only]
- D11 — Participant identifier for uploaded results [a: `participant_no` from the exam card]
- D12 — Uploaded results on the leaderboard [a: separate, never merged]
- D13 — Upload mechanics [a: presigned upload + synchronous preview/apply, no async job]
- Proposed behaviors 11–15 (edition, phase, open edition, level, CTA link) and the delete-blocked change (behavior 6) need confirmation in align.
- EN narrative drafts in the seed need review before any event goes visible.

**student-dashboard-v2.md (PR 2)**
- D3 — On-platform result released vs waiting [a: per-row result call, ≤ 4]
- D4 — Notifikasi in the user menu [a: leave out until an endpoint exists]
- D6 — "Lanjut: <next lesson>" [a: "Materi x dari y" without the title]
- D7 — "Dikemas" date [a: step reached, no date]
- D8 — Invoice for cancelled/expired orders [a: hide]

**testimonials.md (PR 4)**
- None open (D1–D3 and the photo-removal rule settled 2026-11-26); proposed behaviors 8–9 (EN chips never fall back; storage objects kept) need confirmation in align.

**landing-v2.md (PR 5)** — see that spec: D1 stats static vs computed, D2 partnership form, D3 SEO of `/` in a client-only app, D4 admins/affiliators at `/`, D5 `hreflang`, D6 client inputs (hero photo, social URLs, copy).
