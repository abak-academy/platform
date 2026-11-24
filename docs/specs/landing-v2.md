---
title: Public landing page and Event page v2
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/event-series.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "docs/specs/testimonials.md", "web/lib/i18n.ts", "web/components/catalog/ProductCard.tsx", "backend/internal/server/routes.go"]
---

# Public landing page and Event page v2

## Problem and outcome

The public site (abakacademy.id) is a WordPress page. Its stats counters render "0+" to crawlers and slow clients, it shows no prices, testimonials, or partner logos, its social links point to `#`, and its event pages are maintained by hand. The platform (hub.abakacademy.id) has no public pages, so a visitor cannot move from "learn about an event" to "register" without a jump and a second look-and-feel.

Outcome: a fast, SEO-friendly public landing page and Event page in the Next.js app. They are built from live data (events, editions, courses, products), share the navbar and footer with the student experience (phase 5), and are available in Indonesian and English.

## Decisions needed before build

- **Deployment is out of scope (Panca, 2026-11-17).** Which domain serves these pages, and what happens to the WordPress site, is decided by Panca and the client at final deployment. Build the pages so that choice is config only: no hard-coded host, canonical URL and `hreflang` built from `NEXT_PUBLIC_SITE_URL`, and every internal link relative. Do not touch DNS, WordPress, or proxy config in this phase.
- ~~D2. Testimonials.~~ Decided: managed by admins under **Akademik › Testimoni** with a consent gate. See `testimonials.md`.
- **D3. Stats numbers** (20.000+ students, 200+ partner institutions, 10 events, TK–SMA): static config or computed? Default: static config that admins edit later, because the computed counts differ from marketing claims.

## Agreed behavior

Decisions already made (Panca, 2026-11-17). The prototype is the layout reference.

**Navbar (guest).**
- Logo, then Event · Program · Katalog · Tentang, then ID/EN toggle, theme, cart, Masuk, and **Daftar gratis** (accent).
- Transparent at the top of the page. After 8px of scroll it becomes a blurred bar with a hairline border.
- The active item is shown with text color plus a small accent dot.
- Mobile: a hamburger menu with the same items plus a language switch.

**Landing sections, in order:**
1. **Hero:**
   - "Baru" kicker linking to the nearest open edition
   - Headline "Dari ruang kelas ke panggung dunia." / "From the classroom to the world stage."
   - Lede
   - **Cari kompetisimu** (accent) and **Konsultasi gratis** (WhatsApp)
   - Social proof: avatar stack plus stats line
   - Right side: phone mock, stickers, and floating math symbols. The symbols stay inside the illustration area and never overlap text.
2. **Event marquee** of visible event names, or logos when uploaded.
3. **Program:** 6 cards (Kompetisi & Event, Pelatihan Olimpiade, Bimbel & Les Privat, Pelatihan Karya Tulis, Homeschooling, Study Tour & Kuliah Luar Negeri), each with a WhatsApp link prefilled with the program name. The first card links to the Event page.
4. **Pilih panggungmu:** event tiles plus one story card for the selected event (`EventSeriesPicker` + `EventStoryCard`). The card shows logo or illustration, scope and levels chips, full name, tagline, story, schedule timeline, rounds, categories, and facts. Its CTA is "Daftar ⟨category⟩ · price" for each open edition (up to 2) or "Tanya jadwal" (WhatsApp), plus "Situs resmi". There is no "all events" tile and no list of every edition.
5. **Semua di satu akun:** 4 feature cards.
6. **Kursus online:** 4 published courses.
7. **Cerita peserta:** testimonial carousel with the award-ceremony photo, fed by `GET /testimonials?lang=` (`testimonials.md`). When the API returns none, the whole section is not rendered. No placeholder quotes ship.
8. **Toko:** 4 physical products.
9. **Untuk sekolah & lembaga:** a band with a partnership CTA (WhatsApp until a lead form exists).
10. **FAQ** plus the "Belum ketemu jawabannya?" WhatsApp block.
11. **CTA band:** "Medali berikutnya bisa jadi milikmu."
12. **Tentang** (philosophy line plus 4 stats) merged into the top of the footer.
13. **Footer:** address, WhatsApp number, social links (real URLs; hide any without one), link columns, and the wordmark.

**Event page (`/event`):**
- Hero "Semua event, satu tempat".
- "Kenali event-nya" with the same picker and story card.
- "Edisi yang tersedia": `ListToolbar` (Event · Tingkat · Jenjang) over a grid of edition cards. Each card links to the existing product detail page (`/catalog/[id]`).
- The page renders for guests and for logged-in students (inside the phase 5 shell).

**Language:** every string on these pages exists in `id` and `en` in `lib/i18n.ts`. Event narratives come from the public API in the active language (`?lang=`); see event-series Agreed behavior 8–9. The toggle persists and sets `<html lang>`.

**Copy:** use the prototype copy verbatim as the starting point. It is all in `index.html` in both languages. Do not invent claims; every number must come from D3 config or the API.

## Technical contract

- Public route group `web/app/(public)/` with `page.tsx` (landing) and `event/page.tsx`. **Server components** fetch the public API (`/event-series`, `/testimonials`, `/products`) with `revalidate: 300`, so content is in the HTML for crawlers. Interactive parts (picker, nav scroll state, language toggle, carousel) are client islands.
- Metadata: title, description, Open Graph image, and `hreflang` alternates for id/en.
- New components, in the folders listed in `COMPONENTS.md`: `SiteNav`, `SiteFooter`, `MathFloaters`, `EventSeriesPicker`, `EventStoryCard`, `EventSchedule`, `EditionCard`, `LanguageToggle`.
- WhatsApp number and prefilled messages live in one config module. Nothing is hard-coded in components.
- No new dependency.

## Acceptance

1. With JavaScript disabled, the landing HTML contains the hero, program, event story, and FAQ content (SSR check in e2e).
2. Lighthouse on a preview build, mobile: SEO ≥ 95, Accessibility ≥ 95, CLS < 0.1, LCP < 2.5s.
3. Hidden events never appear. An event with no open edition shows "Tanya jadwal".
4. ID/EN toggle: every visible string switches, including event narratives and testimonials (test by asserting there are no `id` strings left in the `en` snapshot of the landing page).
5. With zero visible testimonials, the "Cerita peserta" section is absent from the HTML.
6. Changing `NEXT_PUBLIC_SITE_URL` changes canonical and `hreflang` URLs with no code change.
5. Math symbols do not intersect text boxes at 360, 390, 820, 1100, 1280, and 1440px (Playwright bounding-box check, as done for the prototype).
6. `prefers-reduced-motion` disables float, marquee, and hover-lift animations.

## Non-goals

- Blog migration.
- A school partnership lead form; WhatsApp is used until one is specced.
- Student pages (phase 5).
