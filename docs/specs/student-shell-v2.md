---
title: Seamless student shell (top navigation) and navigation cleanup
issue: none yet
status: deprecated
superseded_by: docs/specs/student-dashboard-v2.md
created: 2026-11-17
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/landing-v2.md", "docs/prototypes/redesign-v2/index.html", "web/app/(student)/layout.tsx", "web/app/(student)/page.tsx", "web/components/shell/AppShell.tsx", "web/components/shell/AppHeader.tsx", "web/components/shell/AppSidebar.tsx", "web/lib/nav-config.ts", "web/components/dashboard", "web/components/catalog/CategoryRail.tsx", "web/lib/i18n.ts"]
---

# Seamless student shell

> Superseded 2026-11-25 by [student-dashboard-v2.md](student-dashboard-v2.md): students keep an admin-style sidebar, not the public top navbar.

## Problem and outcome

A student who logs in leaves the public site and lands in a different-looking app with a sidebar. Two names point to the same place ("Kursus Saya" in the sidebar, "Belajar" elsewhere). The catalog still lists courses the student already owns. Competitions appear under two different meanings, "browse" and "mine".

Outcome: the student experience uses the same top navbar and footer as the public site, so logging in feels like continuing on one website. Every destination has exactly one name.

## Agreed behavior

Decisions (Panca, 2026-11-17):

**Shell**
- Student pages (everything in `(student)` except the exam session and print routes) render `SiteNav` + content + `SiteFooter`, with no sidebar.
- The exam session keeps its own full-screen focus layout.
- Admin roles keep `AppShell` with the sidebar. Nothing changes for them in this phase.

**Navbar (student)**
- Items: **Beranda** (`/`) · **Kompetisi Saya** (`/exam`) · **Event** (`/event`) · **Belajar** (`/courses`) · **Katalog** (`/catalog`).
- Right side: ID/EN toggle, theme, cart with count, notification bell, and an avatar button with first name and a badge (pending payments + live exams).
- The **avatar menu** shows a header (photo, name, grade · school), then **Tagihan** (`/orders`, with pending count), **Profil & akun** (`/profile`), and **Keluar**.
- There is no "Kompetisi saya" or "Kursus saya" entry in the menu; the navbar already has them.
- Mobile: a hamburger shows the five items; the avatar menu stays at top right.

**Names (i18n)**
- `nav_courses`: "Kursus Saya" → **"Belajar"** / "Learn".
- `nav_competition` (student): → **"Kompetisi Saya"** / "My Competitions".
- New `nav_event`: "Event" / "Event".
- Page titles match the nav label.

**Beranda (`/`) is a dashboard only, in this order:**
1. Date overline + greeting ("Selamat malam, Nadia") with the squiggle highlight, and one line of intro.
2. `PendingBanner` (unpaid order).
3. Latest-exam banner (brand gradient) with "Jelajahi sekarang" and "Kompetisi Saya".
4. Three stat cards: `RankingCard`, `StudySummaryCard`, `ExamProgressCard`.
5. **Kompetisi terdaftar**: up to 3 active registrations as the existing exam card, a "Lihat semua" link, and an empty state "Lihat event".
6. **Lanjut belajar**: in-progress courses plus an "add course" card.

There are no landing sections (program, events, shop) and no quick-link pills. `PopularLessonsSection` is removed from the home page; its component can stay for later use.

**Kompetisi Saya (`/exam`):** the title is renamed. The header CTA "Lihat event" goes to `/event`. The back link from an exam detail reads "Kembali ke Kompetisi Saya".

**Event (`/event`):** the public Event page from `landing-v2.md`, rendered inside the student shell.

**Katalog (`/catalog`):**
- No Kompetisi tab. Competition bundles are reached from the event and edition pages.
- For a logged-in student, courses they own are hidden from every tab, and tab counts exclude them. A note above the grid reads "N kursus yang sudah kamu miliki tidak ditampilkan · Buka Belajar".
- The detail page `/catalog/[id]` still works for owned items and shows "Buka di Belajar" instead of buy.

## Technical contract

- `(student)/layout.tsx` switches from `AppShell` to a new `SiteShell` (`SiteNav` + `SiteFooter`) for the student role. Its auth, profile-completeness, and admin-redirect logic stays as it is. The exam-session route group is not wrapped.
- `SiteNav` is shared with phase 4. A `variant="student"` prop provides the student items and the avatar menu.
- Student items move out of `NAV_CONFIG.student` into `SiteNav`'s config. `nav-config.test.ts` is updated.
- `CategoryRail`: drop the `exam` category. Owned-course filtering uses the student's enrollments, which the catalog page already receives or can fetch through an existing hook. **Uncertain:** whether the catalog API returns ownership. If it does not, filter client-side against the enrollments hook and note the extra request.
- **Not built:** the prototype's "kode referral" box in the avatar menu. Students have no referral code today; the referral code belongs to affiliates and is captured in the cart.

## Acceptance

1. A logged-in student at `/`, `/exam`, `/event`, `/courses`, `/catalog`, `/orders`, and `/profile` sees the top navbar and footer, with no sidebar. `/exam/sessions/*` is unchanged.
2. Nav labels per the list above in both languages. No string "Kursus Saya" remains (grep in both locales).
3. The avatar menu contains exactly Tagihan, Profil & akun, and Keluar. The badge equals pending payments + live exams.
4. The catalog never shows an owned course, and counts match.
5. Visual check at 390px and 1366px in light and dark mode.

## Non-goals

- Student analytics changes.
- Profile page redesign (phase 6).
- Admin shell (phase 2a).
