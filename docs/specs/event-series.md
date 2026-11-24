---
title: Event series (competition brands) with narrative, schedule and logo
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/event-series-seed.json", "backend/internal/model/exam.go", "backend/internal/server/routes.go", "backend/db/migrations/0071_exam_event_info.up.sql", "docs/specs/exam-event-info.md", "web/lib/nav-config.ts"]
---

# Event series

## Problem and outcome

Abak Academy runs recurring competition brands: STEMCO, Einstein Olympiad, ESOLIO, IKLC, HIPPO, KoSSMI, LOKASIA, KREASI, RAMSCO, and INYSIA. Each brand has several categories or levels and a new edition every year. The platform only knows **exams** (packages) and the **products** that sell them. Nothing stores a brand's story, rounds, categories, schedule, or logo. That content lives on hand-maintained WordPress pages, and students cannot browse competitions by brand.

Outcome:

- A new **Event** entity ("event series" in code), managed by admins under **Akademik › Event**.
- Each exam can belong to one event.
- A public read API serves visible events, their schedules, and their open editions to the landing page and the Event page (phases 4 and 5).

## Terminology

| UI (id / en) | Code / DB | Meaning |
|---|---|---|
| Event / Event | `event_series` | A competition brand, for example ESOLIO |
| Edisi / Edition, or Ujian in admin | `exam` | One exam package inside an event, for example "ESOLIO 2026 · Visionary Flyers" |
| Jadwal / Schedule | `event_series_schedule` | Brand-level milestones: registration batches, technical meeting, rounds |

The code says `event_series`, not `event`, because "event" is already used for outbox and analytics events.

## Agreed behavior

Decisions (Panca, 2026-11-17, from prototype review):

1. An event has these fields:
   - Identity: name, full name, tagline, scope (national/international), levels label (free text such as "Kelas 1–12"), subjects, official site URL.
   - Display: logo (optional), color tone, fallback illustration.
   - Narrative: story paragraphs, round steps, categories (name + detail), short facts (label + value).
   - Schedule: milestones, each with a label, start date, and optional end date.
   - Visibility: "Tampil di landing page" and a sort order.
2. A visible event shows on the landing page and the Event page as a tile plus a story card. A hidden event is admin-only.
3. Exams link to at most one event. Admins set it in the exam settings ("Event" select) or with **Buat edisi** on the event page, which creates an exam with the event preselected.
4. An event's **open editions** are its linked exams whose product is published and whose phase is not finished. If an event has none, the public card shows "Tanya jadwal" (WhatsApp) instead of a register button.
5. Schedule milestones are shown in date order. Each has a state computed on read, not stored: *done* (end, or start when there is no end, is before today), *ongoing* (today is inside the range), or *upcoming*. The admin list shows each event's next milestone that is not done.
6. An event that still has linked exams cannot be deleted. Hide it instead. Unlinking is done per exam.
7. A logo is optional. Without one, the illustration is used everywhere.
8. **Narrative is bilingual (Panca, 2026-11-17).** Every text field a visitor reads exists in Indonesian and English: full name, tagline, levels label, story, rounds, categories, facts, and each schedule label. The admin editor has **Indonesia | English** tabs in the narrative card and an EN column in the schedule rows.
9. An event can only be made visible when both languages are complete: tagline and at least one story paragraph in each, and an EN label on every schedule row. Turning on "Tampil di landing page" with EN missing is rejected with a message that names the missing part, and the EN tab shows a "!" badge while it is incomplete. Hidden events can be saved with EN empty.

## Data model

Migration `0072_event_series` (next free number; verify before creating):

```sql
CREATE TABLE event_series (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
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

Down migration drops the column, the index, and both tables.

## API

All structs go in `internal/model`, following `docs/standards/go-patterns.md`.

**Admin.** New RBAC permission `event-series:write` for super_admin and admin_exam, and `event-series:read` for those two plus admin_school.

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/event-series` | List with `edition_count`, `open_edition_count`, `registration_count`, `next_milestone {label, starts_on, ends_on, state}` |
| POST | `/admin/event-series` | Create. `slug` is generated from `name` and unique; `is_visible` defaults to false |
| GET | `/admin/event-series/:id` | Full record (`content` with both locales) + schedule + linked editions (id, title, status, phase, scheduled_at, price, registrations) + `en_complete` |
| PATCH | `/admin/event-series/:id` | Partial update. When `schedule` is present, the whole list is replaced in one transaction. Rejects `ends_on < starts_on` (422). Rejects `is_visible: true` while either locale is incomplete (422, `missing: ["en.tagline", "schedule[2].label_en", ...]`) |
| DELETE | `/admin/event-series/:id` | 409 when exams are linked |
| PATCH | `/admin/exams/:id` | Gains `series_id` (nullable). Allowed at any time; it is display-only and never touches scoring or sessions |

**Logo.** Uses the existing admin image upload (`POST /admin/uploads/image`); `logo_key` stores the object key. **Uncertain:** the anonymous `GET /files/*` proxy only serves the `avatars/` prefix. Verify where `AdminUploadImage` writes. If the logo is not publicly readable, either upload logos under a dedicated public prefix (for example `series/`) and add that prefix to the `ServeFile` allowlist, or serve logos from a public bucket URL. Decide this in the T1 PR and record it here.

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/event-series?lang=id\|en` | Visible events ordered by `sort_order`, each with the narrative for `lang` flattened to top-level fields (`full_name`, `tagline`, `story`, ...), schedule (label in `lang`, with computed state), logo URL, and `open_editions [{exam_id, product_id, title, level, price, phase, scheduled_at}]`. Missing or unknown `lang` means `id` |
| GET | `/event-series/:slug` | Same shape for one event; 404 when hidden |

Public responses send `Cache-Control: public, max-age=300` and `Vary: Accept-Language` is not used; the language is in the query string so CDN caching stays simple. No student data appears in public responses. Because visible events are always complete in both languages, the public API never falls back between locales.

## Admin UI (web)

These use the components from `design-tokens-v2.md`, `admin-list-toolbar.md`, and `COMPONENTS.md`.

- **Nav:** add "Event" as the first item in Akademik (see `admin-nav-accordion.md`).
- **List `/admin/event-series`:**
  - `ListToolbar`: search name; filters Landing (shown/hidden), Scope, Schedule (ongoing / upcoming / not filled); sorts Landing order, Name, Registrations.
  - A card per event: logo or illustration, scope and levels, tagline, next milestone with an "Ongoing" badge, counts, a **Kelola** button, and a visibility switch.
- **Editor `/admin/event-series/[id]`.** Left column:
  1. Identity
  2. Landing narrative: story as one textarea where a blank line splits paragraphs; rounds one per line; categories and facts as `Label | value` lines
  3. Schedule: editable rows of label, start, optional end, with add and remove
  4. Editions: linked exams with a **Buat edisi** button

  The right sticky column ("Tampilan") holds a preview chip, logo upload with preview and remove, tone, illustration, subjects, the visibility switch, and **Simpan event**. The delete button only shows when there are no editions.
- **Exam settings:** an "Event" select at the top. The exam list shows the event name under the title and has an Event filter.
- "Lihat di landing" opens the public Event page anchored to this event.

## Seed content

`docs/prototypes/redesign-v2/event-series-seed.json` holds the 10 events in the `content: {id, en}` shape above. The Indonesian content was taken from each official site on 2026-11-17 (`content_status`). The English content is a draft translation (`content_status_en`) and must be reviewed by Panca or the client before any event is made visible. INYSIA has no content in either language because its page failed to load. Load the file through the admin API or a one-off script, not a migration, because it is business content that admins will edit.

## Decisions needed before build

- ~~D1. Narrative language.~~ Decided: ID and EN are both required (Agreed behavior 8–9).
- **D2. Admin school role.** Should admin_school see the Event menu (read-only)? Default is no menu item; the API read permission is still granted for the exam detail page.

## Acceptance

1. The migration applies and rolls back cleanly on a copy of staging.
2. Service tests: visibility rejected when a locale is incomplete (one case per required field), unknown locale key rejected, create (slug collision gets a suffix), PATCH schedule replace is atomic, `ends_on < starts_on` is rejected, delete with linked exams returns 409, milestone state at boundary dates (today = start, today = end, no end date), open-editions filtering (unpublished product excluded, finished phase excluded).
3. Handler tests: RBAC for each role; the public endpoints never return hidden events.
4. Web tests:
   - List: filters and counts.
   - Editor: save round-trip of every field in both language tabs; invalid schedule shows the error and does not save; logo upload and remove; turning visibility on with EN incomplete is blocked and switches to the EN tab.
   - Public API: `?lang=en` returns English narrative and schedule labels; no `lang` returns Indonesian.
   - Exam settings: series select persists.
5. Linking an exam to an event never changes its sessions, scoring, or product (regression test on an exam with sessions).

## Non-goals

- Landing and public Event page UI (phase 4) and the student Event page (phase 5).
- Registering or checking out per event. Purchase stays per product.
- Brand-level analytics.

## Suggested task graph

| Task | Outcome | Depends on |
|---|---|---|
| T1 | Migration, model, repository, logo storage decision | — |
| T2 | Admin service + handlers + RBAC + tests | T1 |
| T3 | Public endpoints + cache headers + tests | T1 |
| T4 | Admin web: list + editor + nav item | T2 |
| T5 | Exam settings series select, exam list column and filter | T2 |
| T6 | Seed loader script + content verification checklist | T2 |
