---
title: Event series (competition brands) with narrative, schedule and logo
issue: none yet
status: proposed
created: 2026-11-17
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/student-dashboard-v2.md", "docs/specs/landing-v2.md", "docs/specs/admin-list-toolbar.md", "docs/specs/design-tokens-v2.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/event-series-seed.json", "abak-redesign-prototype/index.html (Panca's local copy, 2026-11-26 hero)", "backend/internal/model/exam.go", "backend/internal/model/product.go", "backend/internal/server/routes.go", "backend/internal/handler/admin_uploads.go", "backend/internal/service/student.go", "backend/db/migrations/0071_exam_event_info.up.sql", "docs/specs/exam-event-info.md", "web/lib/nav-config.ts"]
---

# Event series

In scope of the redesign (Panca, 2026-11-26): built on a branch cut from `feat/public-landing`, PR into `feat/public-landing`, not `main`. Its place in the PR order is roadmap decision RD1.

## Problem and outcome

Abak Academy runs recurring competition brands: STEMCO, Einstein Olympiad, ESOLIO, IKLC, HIPPO, KoSSMI, LOKASIA, KREASI, RAMSCO, and INYSIA. Each brand has several categories or levels and a new edition every year. The platform only knows **exams** (packages) and the **products** that sell them. Nothing stores a brand's story, rounds, categories, schedule, or logo, and no exam carries a level. That content lives on hand-maintained WordPress pages; students cannot browse competitions by brand, and the student home carousel cannot say "Sedang berlangsung" or show a level.

Outcome:

- A new **Event** entity ("event series" in code), managed by admins under **Akademik › Event**.
- Each exam can belong to one event and carries a structured level (jenjang + grade range).
- A public read API serves visible events, their schedules, and their open editions (with phase, level, price, image) to the landing page, the Event page, and the student home carousel.

## Terminology

| UI (id / en) | Code / DB | Meaning |
|---|---|---|
| Event / Event | `event_series` | A competition brand, for example ESOLIO |
| Edisi / Edition, or Ujian in admin | `exam` | One exam package inside an event, for example "ESOLIO 2026 · Visionary Flyers" |
| Jadwal / Schedule | `event_series_schedule` | Brand-level milestones: registration batches, technical meeting, rounds |
| Fase / Phase | computed | An edition's `open` / `live` / `closed` / `done`, never stored |

The code says `event_series`, not `event`, because "event" is already used for outbox and analytics events.

## Consumers (what this API must feed)

| Consumer | Needs | Served by |
|---|---|---|
| Landing hero (2026-11-26) | Rotating event names after "ke panggung" (prototype hard-codes STEMCO, Einstein, IKLC, ESOLIO, KoSSMI) | `short_name ?? name` of visible events, `sort_order` (D6) |
| Landing "Pilih panggungmu" + Event page "Kenali event-nya" | Tile (logo/illustration, name, levels, scope); story card (full name, tagline, story, schedule with state, rounds, categories, facts, official site); CTA per open edition "Daftar ⟨level⟩ · price" (max 2) or "Tanya jadwal" | `GET /event-series?lang=` |
| Event page "Edisi yang tersedia" | Edition cards with event name, date, jenjang, scope, price, phase badge; filters Event · Tingkat · Jenjang | `open_editions[]` with `stages`, `scheduled_at`, `phase`; scope from the parent event |
| Student home carousel (`student-dashboard-v2.md` §5) | Open editions the student has not registered for: title, level, price/Gratis, image, open vs live headline, product link, order | Flattened `open_editions[]`; registration match on `exam_id` |
| Student "Event & kompetisi" card | Result released vs waiting per registration | **Not served by this spec** (student-dashboard D3 is independent of event-series) |

## Agreed behavior

Decisions (Panca, 2026-11-17, from prototype review), unchanged:

1. An event has these fields:
   - Identity: name, full name, tagline, scope (national/international), levels label (free text such as "Kelas 1–12"), subjects, official site URL.
   - Display: logo (optional), color tone, fallback illustration.
   - Narrative: story paragraphs, round steps, categories (name + detail), short facts (label + value).
   - Schedule: milestones, each with a label, start date, and optional end date.
   - Visibility: "Tampil di landing page" and a sort order.
2. A visible event shows on the landing page and the Event page as a tile plus a story card. A hidden event is admin-only.
3. Exams link to at most one event. Admins set it in the exam settings ("Event" select) or with **Buat edisi** on the event page, which creates an exam with the event preselected.
4. An event's **open editions** are its linked exams whose product is published and whose phase is not finished. If an event has none, the public card shows "Tanya jadwal" (WhatsApp) instead of a register button. *(Made precise below.)*
5. Schedule milestones are shown in date order. Each has a state computed on read, not stored: *done* (end, or start when there is no end, is before today), *ongoing* (today is inside the range), or *upcoming*. The admin list shows each event's next milestone that is not done.
6. An event that still has linked exams cannot be deleted. Hide it instead. Unlinking is done per exam.
7. A logo is optional. Without one, the illustration is used everywhere.
8. **Narrative is bilingual.** Every text field a visitor reads exists in Indonesian and English: full name, tagline, levels label, story, rounds, categories, facts, and each schedule label. The admin editor has **Indonesia | English** tabs in the narrative card and an EN column in the schedule rows.
9. An event can only be made visible when both languages are complete: tagline and at least one story paragraph in each, and an EN label on every schedule row. Turning on "Tampil di landing page" with EN missing is rejected with a message that names the missing part, and the EN tab shows a "!" badge while it is incomplete. Hidden events can be saved with EN empty.

Added 2026-11-26 (proposed; confirm in align, each tied to a decision below):

10. **Phase of an edition** (prototype `evPhase`, mapped to real fields; Jakarta time): `done` when now > `scheduled_end_at` (or, without an end, after the Jakarta day of `scheduled_at`); else `live` when now ≥ `scheduled_at`; else `open` when the selling product is inside its availability window; else `closed`. An unscheduled exam is `open` or `closed` only.
11. **Open edition, precisely:** a linked exam with a *selling product* (D4) that is `published` and inside `available_from/until` (the same rule `GET /products` applies to guests), and whose phase is `open` or `live`. So every open edition can actually be bought; `closed` and `done` editions are never listed.
12. **Level** (D3): an exam gets `stages` (subset of SD/SMP/SMA) and an optional grade range; web renders the level text from them in both languages ("SD · Kelas 4–6" / "Elementary · Grades 4–6"). This feeds the carousel level, the edition-card jenjang, and the Jenjang filter.
13. Public CTAs and carousel slides link to the existing product page `/catalog/[product_id]`; purchase stays per product.

## Data model

Migration `0075_event_series` (latest on `feat/public-landing` @ 34ea5d32 is `0074`; re-check the next free number on the branch base before creating):

```sql
CREATE TABLE event_series (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  short_name    TEXT NULL,                      -- D6; hero rotator, falls back to name
  scope         TEXT NULL CHECK (scope IN ('national','international')),
  subjects      TEXT[] NOT NULL DEFAULT '{}',
  tone          TEXT NOT NULL DEFAULT 'brand' CHECK (tone IN ('brand','ember','teal','gold','sky','berry','navy')),
  illustration  TEXT NOT NULL DEFAULT 'trophy' CHECK (illustration IN ('math','science','language','coding','trophy')),
  logo_key      TEXT NULL,
  official_url  TEXT NULL,
  content       JSONB NOT NULL DEFAULT '{}',   -- {"id": Narrative, "en": Narrative}, see below
  is_visible    BOOLEAN NOT NULL DEFAULT false,
  sort_order    INT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE event_series_schedule (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  series_id  UUID NOT NULL REFERENCES event_series(id) ON DELETE CASCADE,
  label      TEXT NOT NULL,
  label_en   TEXT NULL,
  starts_on  DATE NOT NULL,
  ends_on    DATE NULL CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX event_series_schedule_series_idx ON event_series_schedule (series_id, starts_on);

ALTER TABLE exam ADD COLUMN IF NOT EXISTS series_id UUID NULL REFERENCES event_series(id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS exam_series_id_idx ON exam (series_id);

-- D3 (a): structured level on the exam
ALTER TABLE exam ADD COLUMN IF NOT EXISTS stages    TEXT[]   NOT NULL DEFAULT '{}' CHECK (stages <@ ARRAY['SD','SMP','SMA']::TEXT[]);
ALTER TABLE exam ADD COLUMN IF NOT EXISTS grade_min SMALLINT NULL CHECK (grade_min BETWEEN 1 AND 12);
ALTER TABLE exam ADD COLUMN IF NOT EXISTS grade_max SMALLINT NULL CHECK (grade_max BETWEEN 1 AND 12 AND (grade_min IS NULL OR grade_max >= grade_min));
```

`content` holds one narrative per locale:

```json
{
  "id": {"full_name": "", "tagline": "", "levels_label": "", "story": ["..."], "rounds": ["..."],
         "categories": [{"name": "", "detail": ""}], "facts": [{"label": "", "value": ""}]},
  "en": { "...same shape..." }
}
```

Why one JSONB keyed by locale instead of `_en` columns: the narrative is display-only, never queried or joined, and edited as a whole per language. A third language later is data, not a migration. In Go, model it as `map[string]EventNarrative` with `id` and `en` as the only accepted keys (422 on others). The schedule gets its own table because the list and the public API compute "next milestone" from it; a schedule label is short, so `label_en` is a plain column.

Exam level is columns, not narrative, because the Event page filters on it and the student carousel reads it per edition.

Down migration drops the exam columns and index, then both tables.

## API

All structs go in `internal/model`, following `docs/standards/go-patterns.md`.

**Admin.** New RBAC permission `event-series:write` for super_admin and admin_exam, and `event-series:read` for those two plus admin_school.

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/event-series` | List with `edition_count`, `open_edition_count`, `registration_count`, `next_milestone {label, starts_on, ends_on, state}` |
| POST | `/admin/event-series` | Create. `slug` is generated from `name` and unique (collision gets a suffix); `is_visible` defaults to false |
| GET | `/admin/event-series/:id` | Full record (`content` with both locales) + schedule + linked editions (id, title, status, phase, scheduled_at, price, registrations) + `en_complete` |
| PATCH | `/admin/event-series/:id` | Partial update. When `schedule` is present, the whole list is replaced in one transaction. Rejects `ends_on < starts_on` (422). Rejects `is_visible: true` while either locale is incomplete (422, `missing: ["en.tagline", "schedule[2].label_en", ...]`) |
| DELETE | `/admin/event-series/:id` | 409 when exams are linked |
| PATCH | `/admin/exams/:id` | Gains `series_id` (nullable), `stages`, `grade_min`, `grade_max`. Allowed at any time; display-only, never touches scoring or sessions |

**Logo storage (verified 2026-11-26).** `POST /admin/uploads/image` signs keys under `question/`, and the anonymous `GET /files/*` serves the prefixes `avatars`, `product`, `question` (`proxyReadPrefixAllowlist` in `service/student.go`). So an uploaded logo is already publicly readable at `<api>/files/<logo_key>`; no allowlist change is needed. Prefix choice is D7.

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/event-series?lang=id\|en` | Visible events by `sort_order`; per event: `slug`, `name`, `short_name`, `scope`, `subjects`, `tone`, `illustration`, `logo_url`, `official_url`, the narrative for `lang` flattened (`full_name`, `tagline`, `levels_label`, `story`, `rounds`, `categories`, `facts`), `schedule [{label, starts_on, ends_on, state}]` (label in `lang`), and `open_editions` (below). Missing or unknown `lang` means `id` |
| GET | `/event-series/:slug` | Same shape for one event; 404 when hidden |

`open_editions[]`, ordered `live` first, then `scheduled_at` ascending (unscheduled last), then title:

```json
{"exam_id": "", "product_id": "", "title": "", "stages": ["SD"], "grade_min": 4, "grade_max": 6,
 "price": 0, "image_url": "", "phase": "open|live", "scheduled_at": null, "scheduled_end_at": null,
 "available_until": null}
```

`title` is the exam title as stored (not translated; edition names are proper names). `price` 0 renders "Gratis". `image_url` is the selling product's image (carousel slide art; fallback = event illustration).

Public responses send `Cache-Control: public, max-age=300`; the language is in the query string so CDN caching stays simple. No student data and no registration or sales counts appear in public responses. Because visible events are always complete in both languages, the public API never falls back between locales. A 5-minute cache means a phase flip (open → live) can lag up to 5 minutes; acceptable for marketing pages, and the student carousel re-checks registration on the client.

## Admin UI (web)

These use the components from `design-tokens-v2.md` (visual reference: the design-system artifact), `admin-list-toolbar.md`, and `COMPONENTS.md`.

- **Nav:** add "Event" as the first item in Akademik (`admin-nav-accordion.md`).
- **List `/admin/event-series`:**
  - `ListToolbar` (D5): search name; filters Landing (shown/hidden), Scope, Schedule (ongoing / upcoming / not filled); sorts Landing order, Name, Registrations.
  - A card per event: logo or illustration, scope and levels, tagline, next milestone with an "Ongoing" badge, counts, a **Kelola** button, and a visibility switch.
- **Editor `/admin/event-series/[id]`** (admin editor template: form cards left, sticky publish card right). Left column:
  1. Identity (incl. short name)
  2. Landing narrative: story as one textarea where a blank line splits paragraphs; rounds one per line; categories and facts as `Label | value` lines
  3. Schedule: editable rows of label, start, optional end, with add and remove
  4. Editions: linked exams with a **Buat edisi** button

  The right sticky column ("Tampilan") holds a preview chip, logo upload with preview and remove, tone, illustration, subjects, the visibility switch, and **Simpan event**. The delete button only shows when there are no editions.
- **Exam settings:** an "Event" select and the level fields (jenjang chips SD/SMP/SMA + grade from/to) at the top. The exam list shows the event name under the title and has an Event filter.
- "Lihat di landing" opens the public Event page anchored to this event (only once the landing exists; hidden before, per the deployment rule in the roadmap).

## Seed content

`docs/prototypes/redesign-v2/event-series-seed.json` holds the 10 events in the `content: {id, en}` shape above (no `short_name`; the loader leaves it null, Einstein Olympiad gets "Einstein" by hand if D6 = a). The Indonesian content was taken from each official site on 2026-11-17 (`content_status`). The English content is a draft translation (`content_status_en`) and must be reviewed by Panca or the client before any event is made visible. INYSIA has no content in either language because its page failed to load. Load the file through the admin API or a one-off script, not a migration, because it is business content that admins will edit. Existing exams get `series_id` and level by hand in admin after the seed.

## Acceptance

1. The migration applies and rolls back cleanly on a fresh database and on a copy of prod data (exam rows keep every existing column value; new exam columns default to `{}`/NULL).
2. Service tests: visibility rejected when a locale is incomplete (one case per required field), unknown locale key rejected, create (slug collision gets a suffix), PATCH schedule replace is atomic, `ends_on < starts_on` is rejected, delete with linked exams returns 409, milestone state at boundary dates (today = start, today = end, no end date).
3. Phase and open-edition tests (Jakarta clock injected): each phase at its boundaries (now = `scheduled_at`, now just after `scheduled_end_at`, no end date, unscheduled exam, product window closed → `closed`); open editions exclude an unpublished product, a product outside its window, `closed` and `done` editions, and an exam without a selling product (D4); ordering live-first then date.
4. Handler tests: RBAC for each role; the public endpoints never return hidden events, registration counts, or consent/student data; `?lang=en` returns English narrative and schedule labels; no `lang` returns Indonesian; `Cache-Control` set.
5. `PATCH /admin/exams/:id` persists `series_id`, `stages`, `grade_min/max`; rejects an unknown stage and `grade_max < grade_min` (422). Linking an exam to an event never changes its sessions, scoring, or product (regression test on an exam with sessions).
6. Web tests: list filters and counts; editor save round-trip of every field in both language tabs; invalid schedule shows the error and does not save; logo upload and remove; turning visibility on with EN incomplete is blocked and switches to the EN tab; exam settings series select and level fields persist; exam list shows the event and filters by it.
7. `./deploy/pipeline/backend.sh`, `npx tsc --noEmit`, `npx vitest run`, `npm run build` pass.

## Non-goals

- Landing and public Event page UI (`landing-v2.md`), and switching the student carousel to this API (`student-dashboard-v2.md`; which PR does it depends on roadmap RD1).
- Registering or checking out per event. Purchase stays per product.
- Popularity, registration, or sales counts in any public payload (the carousel never sorts by popularity).
- Result-release state for registrations (student-dashboard D3).
- Edition venue/city (the prototype's poster card shows one; no field exists and none is added).
- Brand-level analytics.

## Decisions needed

Recommendation first.

- ~~D1. Narrative language.~~ Decided: ID and EN are both required (Agreed behavior 8–9).
- **D2. Admin school role.** Should admin_school see the Event menu? a) No menu item; the API read permission stays for the exam detail page (default). b) Read-only menu item.
- **D3. Exam level.** No level exists on exam or product (student-dashboard D2). a) `exam.stages` (SD/SMP/SMA) + `grade_min/max`, rendered bilingually by web; enables the Jenjang filter. b) Free-text `level_label` + `level_label_en` on exam; no structured filter (filter would match text). c) No level; carousel and edition cards omit it.
- **D4. Selling product of an edition.** An exam can belong to several products (`exam_ids`). a) The cheapest published, in-window product whose `exam_ids` is exactly this exam; exams sold only in bundles are not listed as open editions. b) Any published in-window product containing the exam, cheapest wins (a bundle can surface under several events). c) Admin picks the edition's product explicitly (new `exam.primary_product_id`, one more field to keep in sync).
- **D5. Admin `ListToolbar` dependency.** Phase 2b (`admin-list-toolbar.md`) is not built; only an affiliate-specific `components/affiliate/ListToolbar.tsx` exists. a) This PR builds the generic presentational `components/admin/ListToolbar.tsx` exactly as `admin-list-toolbar.md` § Technical contract describes and uses it for the event list; migrating the other admin lists stays phase 2b. b) Ship phase 2b as its own PR first. c) Event list with a search box only; toolbar comes with 2b.
- **D6. Hero rotator names.** "Einstein Olympiad" does not fit the nowrap hero slot that the prototype fills with "Einstein". a) Nullable `short_name` (falls back to `name`), editable in Identity. b) Landing uses `name` as is and accepts a wider slot. c) Rotator words in landing config, not data.
- **D7. Media prefix for logos (and testimonial photos).** a) Reuse `POST /admin/uploads/image` as is (key under `question/`, already public); zero backend change. b) Add a `series/` (and `testimonial/`) upload prefix + allowlist entry, so content type is legible from the key. Either way an uploaded object is public to anyone holding its key, including for hidden events.

## Implementation plan

Branch `feat/event-series` from `feat/public-landing` (after the PRs RD1 puts before it); one PR into `feat/public-landing`. Tasks for `.agents/work/event-series.json`, test-first per `tdd-implementation`. The plan assumes the recommended options; D5 = b removes T5, D3 = c drops the level columns from T1/T4.

| ID | Title | Files / scope | depends_on | Test-first step | Done-check |
|----|-------|---------------|------------|-----------------|------------|
| T1 | Schema + model + repository | `backend/db/migrations/0075_event_series.{up,down}.sql`, `internal/model/event_series.go`, exam model level/series fields, `internal/repository/event_series.go` (+ tests) | — | Repository tests against Postgres: CRUD, schedule replace in one tx, exam link, `stages` CHECK, delete blocked by FK (fail) | migration up/down/up on a fresh DB; `go test` for repository green |
| T2 | Admin service + handlers | `internal/service/event_series.go`, `internal/handler/admin_event_series.go`, RBAC entries, admin routes (+ tests) | T1 | Service cases of AC 2 (visibility per missing field, locale keys, slug suffix, 409) and handler RBAC per role (fail) | `go test -run` both packages green |
| T3 | Public endpoints + phase | `service/event_series_public.go` (phase + open-edition rules, D4), `handler/event_series_public.go`, public routes (+ tests) | T1 | Phase boundary table and open-edition exclusions of AC 3; no hidden event / no counts in payload; `lang` cases (fail) | tests green; `curl` of both endpoints on the local stack |
| T4 | Exam ↔ event link + level | exam PATCH service/handler fields; `web` exam settings (Event select, level fields), exam list column + Event filter; `lib/types.ts` (+ tests) | T1 | Go: PATCH persists and validates (422 cases); web: settings round-trip test (fail) | tests green; AC 5 |
| T5 | Generic admin ListToolbar (D5 = a) | new `web/components/admin/ListToolbar.tsx` (+ test) per `admin-list-toolbar.md`; no page migrations | — | Tests from `admin-list-toolbar.md` AC 2: count badge, chip removal, reset, outside click, panel stays open on pick, Escape (fail) | tests green |
| T6 | Admin web list + editor + nav | `app/(admin)/admin/event-series/**`, `lib/hooks/event-series.ts`, `NAV_CONFIG` Akademik › Event, i18n (+ tests) | T2, T5 | List filters/counts; editor round-trip both tabs; EN-incomplete block switches tab; logo upload/remove (fail) | tests green; 390/1366 light/dark browser check |
| T7 | Seed loader + content checklist | `scripts/` one-off loader over the admin API reading `event-series-seed.json`; checklist in the PR body (EN review, INYSIA empty, per-exam linking) | T2 | Dry-run mode against the local stack prints 10 creates and no visibility flips (fail on a hidden-only assertion) | loader run on the local stack; all 10 events hidden |
| T8 | Integration verification | whole branch | T3, T4, T6, T7 | — | `backend.sh`, `tsc`, `vitest`, `npm run build`; AC 1–7 each with evidence; `git diff --check` |

Parallel frontiers: **{T1, T5}** → **{T2, T3, T4}** → **{T6, T7}** → T8. T2, T3, T4 all add lines to `internal/server/routes.go` and the RBAC table: each adds its block in its own anchored section (admin event-series, public event-series, exam PATCH) to avoid textual conflicts. `i18n.ts` is shared by T4 and T6: separate anchored blocks per task in both locales.
