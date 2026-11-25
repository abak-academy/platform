---
title: Event (competition brand) built on product grouping, with narrative, schedule and logo
issue: none yet
status: agreed
created: 2026-11-17
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/student-dashboard-v2.md", "docs/specs/landing-v2.md", "docs/specs/testimonials.md", "docs/specs/admin-list-toolbar.md", "docs/specs/admin-nav-accordion.md", "docs/specs/design-tokens-v2.md", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/event-series-seed.json", "backend/db/migrations/0073_product_grouping.up.sql", "backend/internal/model/product_grouping.go", "backend/internal/service/product_grouping.go", "backend/internal/repository/product.go", "backend/internal/server/routes.go", "backend/internal/service/rbac.go", "backend/internal/handler/admin_uploads.go", "backend/internal/service/student.go", "backend/db/migrations/0037_participant_number.up.sql", "backend/internal/service/exam.go", "backend/internal/service/card_generate.go", "backend/internal/repository/exam.go", "web/components/admin/ExamRegistrationsTab.tsx", "web/app/(print)/exam/[id]/card/page.tsx", "web/lib/hooks/product-groupings.ts", "web/lib/hooks/admin-product-groupings.ts", "web/app/(admin)/admin/product-groupings", "web/app/(student)/catalog/page.tsx", "web/app/(affiliate)/affiliate/partner/orders/new/page.tsx", "web/lib/nav-config.ts"]
---

# Event

Redesign PR 3, built after PR 1 (admin) and before the student and testimonials PRs (roadmap RD1 = a, Panca 2026-11-26). Branch cut from `feat/public-landing`, PR into `feat/public-landing`.

**The Event is the existing product grouping, extended** (Panca, 2026-11-26: "ganti grouping jadi event"). No parallel `event_series` table: `product_grouping` gains the narrative, logo, schedule, and visibility fields, and an event's editions are its member products through the existing `product.grouping_id`.

## Problem and outcome

Abak Academy runs recurring competition brands: STEMCO, Einstein Olympiad, ESOLIO, IKLC, HIPPO, KoSSMI, LOKASIA, KREASI, RAMSCO, and INYSIA. Each brand has several categories or levels and a new edition every year. Admins already group the competition products of a brand with **product grouping** ("Grup produk"), but a grouping holds only a name, description, status, and order. Nothing stores a brand's story, rounds, categories, schedule, or logo, and no edition carries a level. That content lives on hand-maintained WordPress pages; students cannot browse competitions by brand, and the student home carousel cannot say "Sedang berlangsung" or show a level.

Outcome:

- Product grouping becomes **Event**, managed by admins under **Akademik › Event**; the "Grup produk" nav item disappears.
- An edition is an exam product in the event; it carries a structured level (jenjang + grade range).
- Admins upload **results of rounds not taken on the platform** (offline rounds, organizer Global Round results) per edition and round; each student sees their own result once published (§ External round results).
- A public read API serves visible events, their schedules, and their open editions (with phase, level, price, image) to the landing hero, the landing and Event page story cards, the Event page edition list, and the student home carousel.

## Product grouping today (verified on `feat/public-landing`, 2026-11-26)

| Part | What exists |
|---|---|
| Schema (`0073_product_grouping`) | `product_grouping {id, name, description, status draft\|published, display_order, created_at, updated_at}`; `product.grouping_id` (FK, `ON DELETE SET NULL`) + `product.grouping_order`. A product belongs to at most one grouping |
| Admin API | `/admin/product-groupings`: GET list, POST, `PATCH /order`, GET/PATCH/DELETE `/:id`, `PUT /:id/products` (membership + order). Permission `product-groupings:write`: `super_admin` (`*`) and `admin_store` (not `admin_exam`) |
| Public API | `GET /product-groupings` (published only); products of one grouping via `GET /products?grouping_id=` (draft groupings hidden from guests, ordered by `grouping_order`) |
| Admin web | `/admin/product-groupings` (+ `[id]` member editor), `ProductGroupingModal`, `lib/hooks/admin-product-groupings.ts`; nav "Grup produk" in Toko for `super_admin` and `admin_store` (`CONTENT_MANAGER_NAV`) |
| Other web consumers | Toko `/catalog` grouping filter (`?grouping=`), affiliate partner "new order" product filter (`lib/hooks/product-groupings.ts`) |
| Product types | Any type can be grouped (exam, course, book, merchandise, medal); nothing restricts a grouping to competitions |

Not knowable from the repo, answered by Panca (2026-11-26): grouping is used **only for competitions** today, so every existing grouping becomes an Event (D9).

## Terminology

| UI (id / en) | Code / DB | Meaning |
|---|---|---|
| Event / Event | `product_grouping` (D8) | A competition brand, for example ESOLIO |
| Edisi / Edition | an `exam`-type `product` with `grouping_id` = the event | One sellable edition, for example "ESOLIO 2026 · Visionary Flyers" |
| Produk lain / Other products | a non-exam product in the event | Merch, medals, books of that brand; shown in Toko, never as an edition |
| Jadwal / Schedule | `product_grouping_schedule` | Brand-level milestones: registration batches, technical meeting, rounds |
| Fase / Phase | computed | An edition's `open` / `live` / `closed` / `done`, never stored |

Code keeps `grouping`/`series` names, not `event`, because "event" is already used for outbox and analytics events.

## Consumers (what this API must feed)

| Consumer | Needs | Served by |
|---|---|---|
| Landing hero | Rotating word after "ke panggung": "dunia"/"world" first, then event names (prototype: STEMCO, Einstein, IKLC, ESOLIO, KoSSMI); screen readers and reduced motion get "dunia" | `short_name ?? name` of visible events in `sort_order` (D6) |
| Landing "Pilih panggungmu" + Event page "Kenali event-nya" | Tile (logo/illustration, name, levels, scope); story card (full name, tagline, story, schedule with state, rounds, categories, facts, official site); CTA per open edition "Daftar ⟨level⟩ · price" (max 2) or "Tanya jadwal" | `GET /event-series?lang=` |
| Event page "Edisi yang tersedia" | Edition cards with event name, date, jenjang, scope, price, phase badge; filters Event · Tingkat · Jenjang | `open_editions[]` with `stages`, `scheduled_at`, `phase`; scope from the parent event |
| Program page "Kompetisi & Event" block | A link to the Event page; no data | — |
| Student home carousel (`student-dashboard-v2.md` §5) | Open editions the student has not registered for: title, level, price/Gratis, image, open vs live headline, product link, order | Flattened `open_editions[]`; registered when every id in `exam_ids` has a registration |
| Student "Event & kompetisi" card | Result released vs waiting per registration (student D3); **published external round results** per registration; empty-state link to the Event page | `external_results[]` on `GET /exam/registrations` (§ External round results) |
| Toko filter, affiliate new-order filter | Published groupings by name; the Toko filter is labelled "Event" (D9) | Existing `GET /product-groupings`, unchanged |
| Exam card, admin roster + CSV, external-result upload | Participant number with the event prefix (§ Participant number) | `participant_prefix` of the exam's event |

## Agreed behavior

Decisions of 2026-11-17 (prototype review), restated for the grouping model. Every decision below is settled; the spec was approved for build on 2026-11-26 (Panca accepted every recommended option and proposed behavior).

1. An event has these fields:
   - Identity: name, short name (D6), participant prefix (§ Participant number), full name, tagline, scope (national/international), levels label (free text such as "Kelas 1–12"), subjects, official site URL.
   - Display: logo (optional), colour tone, fallback illustration.
   - Narrative: story paragraphs, round steps, categories (name + detail), short facts (label + value).
   - Schedule: milestones, each with a label, start date, and optional end date.
   - Visibility: "Tampil di landing page" and a sort order (the existing `display_order`).
2. A visible event shows on the landing page and the Event page as a tile plus a story card. A hidden event is admin-only on those pages.
3. A product belongs to at most one event (existing rule). Admins set membership in the event editor's **Edisi & produk** card (the existing member editor).
4. An event with no open edition shows "Tanya jadwal" (WhatsApp) instead of a register button.
5. Schedule milestones are shown in date order. Each has a state computed on read: *done* (end, or start when there is no end, is before today, Jakarta), *ongoing* (today inside the range), or *upcoming*. The admin list shows each event's next milestone that is not done.
6. An event that still has member products cannot be deleted (409); hide it or remove its products first. *This changes today's delete, which silently ungroups members* (settled 2026-11-26).
7. A logo is optional. Without one, the illustration is used everywhere.
8. **Narrative is bilingual.** Every text field a visitor reads exists in Indonesian and English: full name, tagline, levels label, story, rounds, categories, facts, and each schedule label. The editor has **Indonesia | English** tabs in the narrative card and an EN column in the schedule rows.
9. An event can be made visible only when it is `published`, has a `participant_prefix`, and both languages are complete: tagline and at least one story paragraph in each, and an EN label on every schedule row. Turning visibility on otherwise is rejected with a message naming the missing part; the EN tab shows a "!" badge while incomplete. Setting `status` back to draft also sets `is_visible = false` in the same write.
10. `status` keeps its current meaning (the grouping appears in the Toko and affiliate filters); `is_visible` is the landing/Event-page switch. A published, hidden event is a plain Toko grouping.

Added 2026-11-26, settled 2026-11-26 (Panca):

11. **Edition** = a product in the event with `type = exam`. Its exams are `product.exam_ids`; its schedule is the earliest `scheduled_at` and the latest `scheduled_end_at` of those exams. Bundles in the event are editions too.
12. **Phase** (Jakarta time): `done` when now > `scheduled_end_at` (or, without an end, after the Jakarta day of `scheduled_at`); else `live` when now ≥ `scheduled_at`; else `open` when the product is inside `available_from/until`; else `closed`. An unscheduled edition is `open` or `closed` only.
13. **Open edition:** an edition whose product is `published`, inside its availability window (the rule `GET /products` applies to guests), and in phase `open` or `live`. `closed` and `done` editions are never listed.
14. **Level** (D3): `stages` (subset of SD/SMP/SMA) and an optional grade range; web renders the level text in both languages ("SD · Kelas 4–6" / "Elementary · Grades 4–6"). Feeds the carousel level, the edition-card jenjang, and the Jenjang filter.
15. Public CTAs and carousel slides link to the product page `/catalog/[product_id]`; purchase stays per product.
16. **Participant number prefix** (Panca, 2026-11-26): an exam whose products belong to an event shows and prints its participant numbers as `<PREFIX>-YYMMDD-<exam>-NNNNNN` (ESOLIO → `ESOL-…`); exams without an event keep `YYMMDD-<exam>-NNNNNN`. Stored numbers never change (§ Participant number).

## Data model

Migration `0075_event` (latest on `main` and `feat/public-landing` is `0074`; re-check before creating). It **alters** `product_grouping` and creates the schedule table plus the two external-result tables (§ External round results).

```sql
ALTER TABLE product_grouping
  ADD COLUMN slug          TEXT NULL,
  ADD COLUMN short_name    TEXT NULL,                              -- D6
  ADD COLUMN scope         TEXT NULL CHECK (scope IN ('national','international')),
  ADD COLUMN subjects      TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN tone          TEXT NOT NULL DEFAULT 'brand' CHECK (tone IN ('brand','ember','teal','gold','sky','berry','navy')),
  ADD COLUMN illustration  TEXT NOT NULL DEFAULT 'trophy' CHECK (illustration IN ('math','science','language','coding','trophy')),
  ADD COLUMN logo_key      TEXT NULL,
  ADD COLUMN official_url  TEXT NULL,
  ADD COLUMN content       JSONB NOT NULL DEFAULT '{}',            -- {"id": Narrative, "en": Narrative}
  ADD COLUMN is_visible    BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN participant_prefix TEXT NULL CHECK (participant_prefix ~ '^[A-Z]{2,6}$'),
  ADD CONSTRAINT product_grouping_visible_published CHECK (NOT is_visible OR status = 'published'),
  ADD CONSTRAINT product_grouping_visible_prefix CHECK (NOT is_visible OR participant_prefix IS NOT NULL),
  ADD CONSTRAINT product_grouping_participant_prefix_key UNIQUE (participant_prefix);

-- data migration: every existing grouping gets a unique slug from its name, stays hidden
UPDATE product_grouping g SET slug = s.slug FROM (
  SELECT id, base || CASE WHEN rn > 1 THEN '-' || rn ELSE '' END AS slug FROM (
    SELECT id, trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')) AS base,
           row_number() OVER (PARTITION BY trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')) ORDER BY created_at, id) AS rn
    FROM product_grouping) x) s
WHERE g.id = s.id;
ALTER TABLE product_grouping ALTER COLUMN slug SET NOT NULL, ADD CONSTRAINT product_grouping_slug_key UNIQUE (slug);

-- data migration: default participant prefix = first 4 letters of the name, uppercased; NULL when
-- shorter than 2 letters or shared with another grouping (the admin fills those before making it visible)
UPDATE product_grouping g SET participant_prefix = d.p FROM (
  SELECT id, p, count(*) OVER (PARTITION BY p) AS n FROM (
    SELECT id, left(upper(regexp_replace(name, '[^A-Za-z]', '', 'g')), 4) AS p FROM product_grouping) x) d
WHERE g.id = d.id AND length(d.p) >= 2 AND d.n = 1;

-- cached card PDFs of exams in an event re-render with the prefix on next download
UPDATE exam_registration SET card_key = NULL
WHERE card_key IS NOT NULL AND exam_id IN (
  SELECT pe.exam_id FROM product_exam pe JOIN product p ON p.id = pe.product_id WHERE p.grouping_id IS NOT NULL);

CREATE TABLE product_grouping_schedule (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  grouping_id UUID NOT NULL REFERENCES product_grouping(id) ON DELETE CASCADE,
  label       TEXT NOT NULL,
  label_en    TEXT NULL,
  starts_on   DATE NOT NULL,
  ends_on     DATE NULL CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX product_grouping_schedule_grouping_idx ON product_grouping_schedule (grouping_id, starts_on);

-- D3 (a): structured level on the edition product
ALTER TABLE product ADD COLUMN IF NOT EXISTS stages    TEXT[]   NOT NULL DEFAULT '{}' CHECK (stages <@ ARRAY['SD','SMP','SMA']::TEXT[]);
ALTER TABLE product ADD COLUMN IF NOT EXISTS grade_min SMALLINT NULL CHECK (grade_min BETWEEN 1 AND 12);
ALTER TABLE product ADD COLUMN IF NOT EXISTS grade_max SMALLINT NULL CHECK (grade_max BETWEEN 1 AND 12 AND (grade_min IS NULL OR grade_max >= grade_min));
```

- Existing rows (all competitions, D9) keep `name`, `description`, `status`, `display_order` (= the event sort order), and their member products; they get `content = '{}'`, `is_visible = false`, and a derived `participant_prefix` for the admin to review, so nothing appears on the landing until an admin completes both languages. A name that slugifies to empty (all symbols) must be caught by the migration test; whether prod has one stays unknown (no prod inventory is run now that D9 is settled).
- `description` stays for the Toko filter context; the landing uses `content`, never `description`.
- `content` holds one narrative per locale:

```json
{
  "id": {"full_name": "", "tagline": "", "levels_label": "", "story": ["..."], "rounds": ["..."],
         "categories": [{"name": "", "detail": ""}], "facts": [{"label": "", "value": ""}]},
  "en": { "...same shape..." }
}
```

Why one JSONB keyed by locale instead of `_en` columns: the narrative is display-only, never queried or joined, and edited as a whole per language. In Go, `map[string]EventNarrative` with `id` and `en` as the only accepted keys (422 otherwise). The schedule gets its own table because the list and public API compute "next milestone" from it; a schedule label is short, so `label_en` is a plain column. Level is product columns because the edition is a product and the Event page filters on it.

Down migration drops the external-result tables, the product level columns, the schedule table, the constraints, and the added grouping columns (incl. `participant_prefix`); the original grouping data survives. Cards re-rendered after the up migration keep the prefix until their `card_key` is cleared again (accepted).

## API

Structs in `internal/model` (`ProductGrouping` gains the fields), following `docs/standards/go-patterns.md`. Naming per D8 = a (no renames).

**Admin** (existing group `/admin/product-groupings`, permission per D2):

| Method | Path | Change |
|---|---|---|
| GET | `/admin/product-groupings` | Adds `slug`, `participant_prefix`, `is_visible`, `logo_url`, `edition_count`, `open_edition_count`, `next_milestone {label, starts_on, ends_on, state}` |
| POST | `/admin/product-groupings` | `slug` generated from `name`, unique (collision gets a suffix); `participant_prefix` defaults to the migration rule over `short_name ?? name` (left NULL on collision); `is_visible` false |
| GET | `/admin/product-groupings/:id` | Adds `participant_prefix`, full `content` (both locales), schedule, `en_complete`, and per member product its `phase` and level |
| PATCH | `/admin/product-groupings/:id` | Accepts the new fields. `schedule` present → whole list replaced in one transaction. 422 on `ends_on < starts_on`, unknown locale key, a `participant_prefix` not matching `^[A-Z]{2,6}$` (the web uppercases before sending), or `is_visible: true` while draft or incomplete (`missing: ["participant_prefix", "en.tagline", "schedule[2].label_en", ...]`); 409 when the prefix belongs to another event. A prefix change clears the cached cards of the event's exams (§ Participant number) |
| DELETE | `/admin/product-groupings/:id` | 409 while member products exist (behavior 6) |
| PUT | `/admin/product-groupings/:id/products` | Unchanged contract; also clears the cached cards of every exam whose event changes (§ Participant number) |
| PATCH | `/admin/products/:id` | Gains `stages`, `grade_min`, `grade_max` (exam products; 422 on an unknown stage or `grade_max < grade_min`). Display-only; never touches scoring, sessions, or stock. A change of `exam_ids` that moves an exam into or out of an event clears that exam's cached cards |

**Logo storage** (settled with testimonials D3, Panca 2026-11-26): reuse `POST /admin/uploads/image` as is; keys land under `question/`, which the anonymous `GET /files/*` already serves (`proxyReadPrefixAllowlist`, `service/student.go`), so no backend change. Any uploaded logo is public to whoever holds its key, including for hidden events.

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/product-groupings` | **Unchanged** (Toko and affiliate filters) |
| GET | `/event-series?lang=id\|en` | New. Visible events by `display_order`; per event: `slug`, `name`, `short_name`, `scope`, `subjects`, `tone`, `illustration`, `logo_url`, `official_url`, the `lang` narrative flattened (`full_name`, `tagline`, `levels_label`, `story`, `rounds`, `categories`, `facts`), `schedule [{label, starts_on, ends_on, state}]` (label in `lang`), and `open_editions`. Missing or unknown `lang` means `id` |
| GET | `/event-series/:slug` | Same shape for one event; 404 when hidden |

`open_editions[]`, ordered `live` first, then `scheduled_at` ascending (unscheduled last), then title:

```json
{"product_id": "", "exam_ids": [""], "title": "", "stages": ["SD"], "grade_min": 4, "grade_max": 6,
 "price": 0, "image_url": "", "phase": "open|live", "scheduled_at": null, "scheduled_end_at": null,
 "available_until": null}
```

`title` is the product name as stored (edition names are proper names). `price` 0 renders "Gratis". `image_url` is the product image (fallback = the event illustration).

Public responses send `Cache-Control: public, max-age=300`. No student data and no registration or sales counts appear in public responses. Visible events are always complete in both languages, so the API never falls back between locales. A phase flip can lag up to 5 minutes; the student carousel re-checks registration on the client.

## Admin UI (web)

Uses `design-tokens-v2.md`, `admin-list-toolbar.md`, and `COMPONENTS.md`.

- **Nav:** "Event" is the first item of Akademik for `super_admin`; "Grup produk" is removed from Toko (settles `admin-dashboard-v2.md` D12). Other roles per D2. Route per D8.
- **List** (today's `/admin/product-groupings` page, reworked): `ListToolbar` (D5): search name; filters Landing (shown/hidden), Status (published/draft), Scope, Schedule (ongoing / upcoming / not filled); sorts Landing order (drag reorder kept), Name, Editions. A card per event: logo or illustration, scope and levels, tagline, next milestone with an "Ongoing" badge, counts, **Kelola**, and a visibility switch.
- **Editor** (today's `[id]` page, admin editor template: form cards left, sticky publish card right). Left: 1. Identity (incl. short name and **Prefiks nomor peserta**, uppercase A–Z, 2–6 letters, with the hint "Dipakai di depan nomor peserta, mis. ESOL-251012-0007-000123. Mengubahnya mengubah nomor di kartu yang dibuat berikutnya."); 2. Landing narrative (story as one textarea where a blank line splits paragraphs; rounds one per line; categories and facts as `Label | value` lines) in Indonesia | English tabs; 3. Schedule rows (label, label EN, start, optional end; add/remove); 4. **Edisi & produk**: today's member editor, with exam products shown as editions (phase badge + level fields inline) and other products listed below. Right "Tampilan": preview chip, logo upload/preview/remove, tone, illustration, subjects, status, visibility switch, **Simpan event**; delete only when there are no members.
- **Product form** (exam products): level fields (jenjang chips SD/SMP/SMA + grade from/to).
- "Lihat di landing" opens the public Event page anchored to the event; hidden until the landing PR has merged.
- Copy: admin strings "Grouping Produk"/"grouping" become "Event". The Toko filter label becomes **Event** in this PR (D9): `catalog_grouping_label` "Event" / "Event", `catalog_grouping_all` "Semua event" / "All events". The affiliate new-order filter's copy is unchanged.

## External round results

Added by Panca, 2026-11-26. Today results exist only for on-platform sessions (`/admin/results`, `/admin/exams/:id/results-workspace`, the session leaderboard); nothing imports results the organizer produces elsewhere.

**Behavior.**
1. Results are uploaded **per exam and round**: the admin opens an edition in the event editor, picks the exam (when the product has several) and a round (new label, e.g. "Global Round", or an existing one to replace), and uploads a CSV (D10).
2. Columns: `participant_no` (the participant number printed on the exam card, `[<PREFIX>-]YYMMDD-<exam_number>-NNNNNN`, accepted **with or without** the event prefix; D11, § Participant number), `score` (number, required), `rank` (integer ≥ 1, optional), `award` (text ≤ 60, optional, e.g. "Medali emas"), `certificate_url` (optional, `https://` only). Header row required; column order free; extra columns refused.
3. **Preview** before anything is written: totals (matched, errors) and every bad row with its reason: unknown participant (no registration of this exam with that number, or revoked), unreadable participant number, participant number of another exam, duplicate participant in the file, bad score/rank/URL, award too long. **Confirm** is possible only with zero errors (the admin fixes the file and re-uploads).
4. Confirm writes the round in one transaction. Re-uploading a round **replaces all its rows** (idempotent: the same file twice gives the same state); other rounds are untouched.
5. A round is **hidden until the admin publishes it**; publish and unpublish are explicit actions. Replacing the rows of a published round keeps it published (the preview warns "Ronde ini sudah tayang").
6. Every apply, publish, unpublish, and round delete is written to the audit log (`InsertAuditLogMeta`, entity `exam_external_round`, with row counts and the uploaded object key).
7. Student side: a published round gives the student's registration row a **Lihat hasil** action showing round label, score, rank, award, and the certificate link (`student-dashboard-v2.md` §6). Unpublished rounds are invisible to students.
8. Uploaded results never change on-platform sessions, scores, or the leaderboard (D12).

**Data model** (same migration `0075`):

```sql
CREATE TABLE exam_external_round (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  exam_id       UUID NOT NULL REFERENCES exam(id) ON DELETE CASCADE,
  label         TEXT NOT NULL,
  label_en      TEXT NULL,
  source_key    TEXT NULL,                  -- private object key of the last applied file
  published_at  TIMESTAMPTZ NULL,
  updated_by    UUID NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (exam_id, label)
);
CREATE TABLE exam_external_result (
  round_id         UUID NOT NULL REFERENCES exam_external_round(id) ON DELETE CASCADE,
  registration_id  UUID NOT NULL REFERENCES exam_registration(id) ON DELETE CASCADE,
  score            NUMERIC(8,2) NOT NULL,
  rank             INT NULL CHECK (rank >= 1),
  award            TEXT NULL CHECK (char_length(award) <= 60),
  certificate_url  TEXT NULL CHECK (certificate_url IS NULL OR certificate_url LIKE 'https://%'),
  PRIMARY KEY (round_id, registration_id)
);
```

Rows point at the registration, not the student, so a revoked registration's result is not shown and the match is per exam.

**Upload path** — reuse the existing presign pattern, not the async job (D13 = a): `POST /admin/exams/:id/external-rounds/presign` signs a private PUT with the exam id in the key (`presignPrivatePut`, like `exam-grant-bulk/{examID}/…`); `POST …/external-rounds/preview {key, round_label}` reads the object (≤ 2 MB, `LimitReader`), parses with `encoding/csv`, matches participant numbers (`parseParticipantNo`, § Participant number), and returns the preview synchronously; `POST …/external-rounds/apply {key, round_label}` re-parses the same object and writes in one transaction (the preview is not trusted as state). The worker/job machinery (`Job` model, `AdminGetJob`, `worker.pollJobs`) runs fire-and-forget imports with a result CSV and has no preview step; a few thousand rows parse in well under a second, so a job adds polling UI for no gain.

**Admin API** (permission `exam-results-external:write`; roles per D2, same as event editing):

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/exams/:id/external-rounds` | Rounds with row count, `published_at`, `updated_at` |
| POST | `/admin/exams/:id/external-rounds/presign` | Private PUT URL + key |
| POST | `/admin/exams/:id/external-rounds/preview` | `{matched, errors: [{line, participant_no, reason}]}` |
| POST | `/admin/exams/:id/external-rounds/apply` | 422 if the file has errors; creates or replaces the round's rows |
| POST | `/admin/external-rounds/:round_id/publish` · `/unpublish` | Toggle `published_at` |
| GET | `/admin/external-rounds/:round_id/results` | Rows with student name and participant no in the current display format (admin view / CSV export) |
| DELETE | `/admin/external-rounds/:round_id` | Deletes round and rows |

**Student API.** `GET /exam/registrations` items gain `external_results: [{round_id, round_label, score, rank, award, certificate_url, published_at}]` (published rounds only, `round_label` in the request language when `label_en` is set, ordered by `published_at`). One call, no per-row fetch; `[]` when none.

## Participant number

Added by Panca, 2026-11-26. **Only formatting changes; stored integers do not.**

**Today (verified on `feat/public-landing` @ 3857a47f).** `exam_registration.participant_number` is a per-exam `INT` (migration `0037_participant_number`), unique per exam (`uq_examregistration_participant`), assigned as `MAX+1` under a per-exam advisory lock by both insert paths (`repository/exam.go` `CreateExamRegistration` ~L1741–1766, `service/exam_grant.go` ~L146–163). `0037` backfilled every existing row (`ROW_NUMBER() OVER (PARTITION BY exam_id ORDER BY created_at, id)` where NULL) and no insert path leaves it NULL, so every registration has a number; the nil checks in the formatter callers are defensive only. The human string is composed at read time by one helper, `formatParticipantNo` (`service/exam.go` L931): `YYMMDD-<exam_number %04d>-<participant_number %06d>`, `YYMMDD` = the exam's `scheduled_at` in Jakarta, falling back to the registration's `created_at`.

**Rule.**
- An exam's event = the event (`product.grouping_id`) of the products that sell it (`product_exam`). When those products belong to exactly one event and it has a `participant_prefix`, the display string is `<PREFIX>-YYMMDD-<exam>-NNNNNN` (e.g. `ESOL-251012-0007-000123`). No event, or no prefix, keeps today's string. An exam sold by products of two different events keeps today's string, and the event editor flags it on the edition row.
- The prefix applies whether or not the event is visible (every grouping is an event, D9), so it shows from the deploy of this PR for every grouped exam whose derived prefix is set.
- `participant_prefix`: uppercase `A–Z`, 2–6 letters, unique across events, required before `is_visible` can be true. The `0075` migration derives a default (first 4 letters of the name) for the admin to review; POST derives it from `short_name ?? name`.
- Changing an event's prefix, or moving an exam's product into or out of an event, changes the number shown on the next render. The service clears the cached card PDFs (`ClearRegistrationCardsByExamTx`) of the affected exams in the same transaction, so the web card and the PDF agree.
- **Accepted consequence (Panca, 2026-11-26):** cards and PDFs already printed or downloaded show the old string; re-rendered ones show the new one. Students and organizers may hold both forms of one number, which is why the upload accepts both.
- Not affected: the certificate number `ABK/YYYY/<exam>/<participant>` (`AllocateCertificateNumber`, `repository/exam.go` L3009–3063) is a separate format over the same integer and stays as is.

**Formatter contract** (`service/exam.go`, no new package):
- `formatParticipantNo(eventPrefix string, date time.Time, examNumber, participantNumber int) string`: today's output when `eventPrefix == ""`, else `eventPrefix + "-" +` today's output. The three callers pass the prefix they read with the registration; nothing else builds the string.
- `parseParticipantNo(s string) (examNumber, participantNumber int, err error)` for the upload: trims spaces and accepts `^(?:[A-Za-z]{2,6}-)?\d{6}-(\d{4,})-(\d{6,})$`. The match uses the trailing number within the uploaded exam; the exam-number part must equal that exam's `exam_number` (otherwise "participant number of another exam"); the prefix and date parts are not compared, so old and new card strings both match.
- The prefix is read in the same query as the registration: `GetExamRegistrationByID`, `GetRegistrationForPrint`, `GetExamRoster` / `ListExamRoster` select `event_prefix` (nullable) via `product_exam` → `product.grouping_id` → `product_grouping.participant_prefix`, NULL when more than one distinct event is found.

**Touch-points** (every place the display string is produced or shown):

| Where | What | Change |
|---|---|---|
| `backend/internal/service/exam.go` L926–933 | `formatParticipantNo` | New `eventPrefix` parameter; `parseParticipantNo` beside it |
| `backend/internal/service/exam.go` L1618–1633 (`GetExamRegistration`) | `RegistrationDetail.ParticipantNo` for the student exam detail and the web card print page; also `GetExamCard` L1788 | Pass the event prefix |
| `backend/internal/service/exam.go` L1683–1701 (`formatExamRosterRows`) | Roster of `AdminGetExamRoster` / `AdminListExamRoster` (admin exam "Peserta" tab) | Pass the event prefix per row |
| `backend/internal/service/exam.go` L1704–1762 (`ExportExamRosterCSV` → `BuildExamRosterCSV`) | First CSV column | Follows the roster; no own change |
| `backend/internal/service/exam.go` L1947–1964 (`GetCardPrintData`) → `service/card_generate.go` L68 → `service/assets/exam_card_template.html` L652 (`{{participant_no}}`, twice) | Gotenberg card PDF, cached by `card_key` (`GetExamCard` L1798) | Pass the event prefix; cache cleared per the rule |
| `backend/internal/repository/exam.go` L1853 (`GetExamRegistrationByID`), L1970 (`GetRegistrationForPrint`), L2025 / L2030 (`GetExamRoster` / `ListExamRoster`) | Registration reads | Select `event_prefix` |
| `backend/internal/model/exam.go` L217–219, L440–443, L466–468, L487–488 | `RegistrationDetail` / `ExamRosterEntry` fields and comments | Add `EventPrefix *string` (`json:"-"`); update the format comments; drop the stale "rows predating FR-24" wording |
| `web/app/(print)/exam/[id]/card/page.tsx` L115 → `web/components/exam/ExamCardPrintable.tsx` L296, L309 | Web card | Display only; no change (string from the API) |
| `web/components/admin/ExamRegistrationsTab.tsx` L142–155 | Roster column "No. Peserta" | Display only; no change |
| `web/lib/types.ts` L1265–1266, L1366–1367 | `participant_no: string` | No change |
| External rounds (§ External round results) | Preview errors and `GET /admin/external-rounds/:id/results` | Show the current display string; the upload parses with `parseParticipantNo` |
| Event editor (`web/app/(admin)/admin/product-groupings/[id]`) | Identity card, edition rows | Prefix field; "ada di dua event" badge on an edition whose exam is in two events |

Tests that pin today's string and gain a prefixed case (the unprefixed expectations stay): `backend/internal/service/exam_test.go`, `certificate_render_gate_test.go`, `backend/internal/handler/exam_roster_handler_test.go`, `backend/integration/participants_schools_test.go`, `web/app/(print)/exam/[id]/card/page.test.tsx`, `web/components/admin/ExamRegistrationsTab.test.tsx`, `web/lib/hooks/admin-exams.test.tsx`; grep `web/e2e/exam-event-info.spec.ts`, `exam-registrations-modal-bulk.spec.ts`, `exam-student-flow.spec.ts` for the `YYMMDD-NNNN-NNNNNN` shape.

## Seed content

`docs/prototypes/redesign-v2/event-series-seed.json` holds the 10 events in the `content: {id, en}` shape (no `short_name`; the loader leaves it null, Einstein Olympiad gets "Einstein" by hand). Indonesian taken from each official site on 2026-11-17; English is a draft that Panca or the client must review before any event is made visible. INYSIA has no content in either language. Load through the admin API with a one-off script, not a migration. The loader **matches an existing grouping by slug (then by case-insensitive name)** and fills it instead of creating a duplicate; unmatched seed events are created hidden and draft. It sets `short_name` where the seed has one and leaves `participant_prefix` to the derived default (the admin reviews it). It prints the match table, including each event's prefix, for review. Exam products are put into their event and given a level by hand in admin.

## Acceptance

1. The migration applies and rolls back cleanly on a fresh database and on a copy of prod data: every grouping keeps name, description, status, order, and members; gets a unique non-empty slug; `is_visible` false; a derived `participant_prefix` where unique (NULL on collision or < 2 letters). Product rows keep every value; new columns default to `{}`/NULL. Every registration's `participant_number` is identical before and after.
2. Existing behaviour unchanged: `GET /product-groupings`, `GET /products?grouping_id=` (order and draft hiding), the Toko filter, and the affiliate filter pass their existing tests.
3. Service tests: visibility rejected while draft, without a `participant_prefix`, and per missing locale field; a bad prefix (lowercase, 1 or 7 letters, digits) is 422 and a duplicate prefix 409; draft sets `is_visible` false; unknown locale key rejected; slug collision suffix; schedule replace atomic; `ends_on < starts_on` rejected; delete with members returns 409; milestone state at boundaries (today = start, today = end, no end).
4. Phase and open-edition tests (Jakarta clock injected): each phase at its boundaries (now = `scheduled_at`, just after `scheduled_end_at`, no end, unscheduled, window closed → `closed`); a multi-exam product uses earliest start / latest end; open editions exclude unpublished, out-of-window, `closed`, `done`, and non-exam products; ordering live-first then date.
5. Handler tests: RBAC per role (D2); the public endpoints never return hidden events, counts, or student data; `?lang=en` returns English narrative and labels; no `lang` returns Indonesian; `Cache-Control` set.
6. `PATCH /admin/products/:id` persists `stages`, `grade_min/max`; 422 on an unknown stage and `grade_max < grade_min`; never changes sessions, scoring, or stock.
7. Web tests: list filters/counts and reorder; editor round-trip of every field in both tabs; invalid schedule blocked; logo upload and remove; visibility with EN incomplete blocked and switches to the EN tab; level fields persist; nav shows Event in Akademik and no "Grup produk".
8. External rounds: a participant number matches both with and without the event prefix (and with an old prefix after a prefix change); preview reports each error kind (unknown or revoked participant, unreadable number, another exam's number, duplicate, bad score, rank < 1, non-https URL, long award, missing/extra column) with its line; apply with errors is 422; apply twice with the same file gives identical rows; re-upload replaces only that round; a student sees a round only after publish and only their own row, via `external_results`; audit rows for apply, publish, unpublish, delete; on-platform leaderboard unchanged (regression test).
9. Admin web: upload → preview table → confirm → publish round-trip; errors block confirm. The event editor saves the prefix (uppercased) and shows the API's format and duplicate errors.
10. Participant number: for an exam in an event with prefix `ESOL`, the student registration detail, the web card, the card PDF, the admin roster, and the roster CSV all show `ESOL-YYMMDD-<exam>-NNNNNN`; an exam in no event, in an event without a prefix, or in two events shows today's string; changing the prefix or the exam's event clears that exam's cached cards so the next PDF shows the new string; stored `participant_number` and certificate numbers are unchanged (service + repository tests).
11. `./deploy/pipeline/backend.sh`, `npx tsc --noEmit`, `npx vitest run`, `npm run build` pass.

## Non-goals

- Landing, Event page, and Program page UI (`landing-v2.md`); the student carousel adapter (`student-dashboard-v2.md`).
- Registering or checking out per event; purchase stays per product.
- Popularity, registration, or sales counts in any public payload.
- Result-release state for on-platform sessions (student D3).
- Generating certificates for uploaded rounds (the CSV links an existing one); showing uploaded results on any public page.
- Edition venue/city; brand-level analytics.
- Renaming the affiliate new-order grouping filter (the Toko filter becomes "Event", D9).
- Changing stored participant numbers, the certificate number format, or cards already downloaded.

## Decisions

All settled (Panca, 2026-11-26: the recommended option of each, plus the D9 and participant-number answers). Kept with their rejected options for the record.

- ~~D1. Narrative language.~~ Settled 2026-11-17: ID and EN both required (behaviors 8–9).
- ~~D2. Who manages events.~~ **Settled: (a)** `super_admin`, `admin_store` (keeps today's access), and `admin_exam` (runs competitions); each role sees "Event" in its own nav (an Akademik group for `admin_store` holding only Event); `admin_school` none. The same roles hold `exam-results-external:write`. Not chosen: (b) `super_admin` + `admin_exam` only (regression for store staff); (c) today's roles only.
- ~~D3. Level location.~~ **Settled: (a)** on the edition product (`stages`, `grade_min/max`). Not chosen: (b) on `exam`, aggregated; (c) no level.
- ~~D4. Selling product of an edition.~~ Dissolved: the edition **is** the product in the event (behavior 11).
- ~~D5. Admin `ListToolbar`.~~ **Settled: (a)** this PR builds the generic `components/admin/ListToolbar.tsx` per `admin-list-toolbar.md` § Technical contract and uses it for the event list (testimonials reuses it); migrating other admin lists stays phase 2b. Not chosen: (b) 2b first; (c) search box only.
- ~~D6. Hero rotator names.~~ **Settled: (a)** nullable `short_name` (falls back to `name`), editable in Identity. Not chosen: (b) `name` only; (c) landing config.
- ~~D7. Media prefix.~~ Settled (Panca, 2026-11-26): reuse `question/` via `POST /admin/uploads/image`.
- ~~D8. Naming of the evolved grouping.~~ **Settled: (a)** keep DB (`product_grouping`, `grouping_id`), Go/TS types, admin API `/admin/product-groupings`, web hooks, and the admin route; only UI copy says "Event"; new public `/event-series` endpoints. No alias or compat window. Not chosen: (b) rename throughout with aliases; (c) rename only the admin route and nav.
- ~~D9. Non-competition groupings.~~ **Settled (Panca, 2026-11-26): grouping is used only for competitions.** Every existing grouping becomes an Event, hidden until an admin fills its narrative (and prefix) and makes it visible; no `kind` column; no prod inventory task. The Toko grouping filter is labelled "Event" (this PR; `landing-v2.md` keeps it). Not chosen: (b) a `kind` column; (c) client dissolves groupings first.
- ~~D10. Results file format.~~ **Settled: (a)** CSV only (`encoding/csv`). Not chosen: (b) CSV + XLSX (`excelize`).
- ~~D11. Participant identifier.~~ **Settled: (a)** `participant_no` as printed on the exam card, with or without the event prefix (§ Participant number); the upload is per exam. Correction of the earlier risk note: no registration lacks a number — `0037` backfilled every existing row and both insert paths always assign one (verified 2026-11-26). Not chosen: (b) email or username; (c) either column.
- ~~D12. Uploaded results and the leaderboard.~~ **Settled: (a)** separate: uploaded rounds never enter the platform leaderboard or result pages; the student sees them in "Lihat hasil" only. Not chosen: (b) own leaderboard tab; (c) merge.
- ~~D13. Upload mechanics.~~ **Settled: (a)** presigned private upload + synchronous preview and apply. Not chosen: (b) async `Job`.
- **Participant number prefix** (Panca, 2026-11-26): settled as § Participant number. Accepted: already printed cards keep the old string.

## Launch blockers (open; do not block the build)

- The EN narrative drafts in the seed must be reviewed by Panca or the client before any event is made visible.
- Each event's derived `participant_prefix` should be reviewed by an admin soon after deploy: it is shown on cards from the deploy on, and changing it later re-labels numbers on the next render.

## Implementation plan

Branch `feat/event-series` from `feat/public-landing` after PR 1 merges; one PR into `feat/public-landing`. Tasks for `.agents/work/event-series.json`, test-first per `tdd-implementation`, on the settled options (D2 a, D3 a, D5 a, D6 a, D8 a, D9 all-events, D10–D13 a). The old T0 (prod grouping inventory) is dropped: D9 is settled.

| ID | Title | Files / scope | depends_on | Test-first step | Done-check |
|----|-------|---------------|------------|-----------------|------------|
| T1 | Migration + model + repository | `backend/db/migrations/0075_event.{up,down}.sql` (incl. `participant_prefix` + derivation + card-cache clear), `internal/model/product_grouping.go` + product level fields, `internal/repository/product_grouping.go`, `product.go` (+ tests) | — | Repository tests against Postgres: data migration on pre-seeded groupings (slug uniqueness, empty-slug case, members kept, derived prefix incl. collision → NULL, `participant_number` unchanged, grouped exams' `card_key` cleared), new-field CRUD, schedule replace in one tx, visible-requires-published and visible-requires-prefix CHECKs, prefix format + UNIQUE, level CHECKs (fail) | migration up/down/up on fresh DB and a prod-shaped fixture; repository tests green |
| T2 | Admin service + handlers | `internal/service/product_grouping.go`, `internal/handler/product_grouping.go`, product PATCH level fields, RBAC (D2), prefix validation + card-cache clear on prefix/membership change (+ tests) | T1 | AC 3 and AC 6 cases; RBAC per role; prefix 422/409; cache cleared on prefix change, membership PUT, and product `exam_ids` change; existing grouping tests still green (fail) | `go test -run` both packages green |
| T3 | Public endpoints + phase | `service/event_series_public.go` (phase + open-edition rules), `handler/event_series_public.go`, public routes (+ tests) | T1 | AC 4 table; no hidden event / no counts; `lang` cases (fail) | tests green; `curl` both endpoints on the local stack |
| T4 | Generic admin ListToolbar (D5 = a) | new `web/components/admin/ListToolbar.tsx` (+ test) per `admin-list-toolbar.md` | — | `admin-list-toolbar.md` AC 2 cases (fail) | tests green |
| T5 | Admin web: list, editor, product level, nav, Toko label | `app/(admin)/admin/product-groupings/**` (incl. prefix field, two-event badge), `ProductGroupingModal`, `lib/hooks/admin-product-groupings.ts`, product form level fields, `lib/types.ts`, `NAV_CONFIG` (Event in Akademik, no "Grup produk"), i18n incl. `catalog_grouping_label`/`catalog_grouping_all` → Event (+ tests) | T2, T4 | AC 7 cases + prefix field (uppercased, API errors shown) + Toko filter label "Event" (fail) | tests green; 390/1366 light/dark browser check |
| T6 | Seed loader | `scripts/` one-off loader over the admin API (match by slug/name, create hidden, print prefixes) | T2 | Dry run on the local stack prints the match table and no visibility flips (fail on a hidden-only assertion) | run on the local stack; all events hidden |
| T7 | External results backend | migration tables (in `0075`), `internal/service/exam_external_result.go` (uses `parseParticipantNo` from T9), handler, routes, RBAC, `external_results` on `GET /exam/registrations` (+ tests) | T1, T9 | AC 8 cases incl. prefixed/unprefixed match, idempotent re-apply, and leaderboard regression (fail) | `go test -run` green; `curl` preview/apply/publish on the local stack |
| T8 | External results admin web | edition row in the event editor → "Hasil ronde luar" dialog: exam + round pick, upload, preview table, confirm, publish/unpublish, delete; `lib/types.ts` (`external_results`) (+ tests) | T5, T7 | AC 9 round-trip; confirm disabled with errors (fail) | tests green; browser check with a sample CSV (one prefixed, one unprefixed number) |
| T9 | Participant number prefix | `internal/service/exam.go` (`formatParticipantNo` + `parseParticipantNo`, the three callers), `internal/repository/exam.go` (four reads select `event_prefix`), `internal/model/exam.go`, the tests listed in § Participant number (+ e2e grep) | T1 | AC 10 cases: prefixed and unprefixed output per surface, two-event exam unprefixed, parse table (with/without prefix, lowercase prefix, other exam's number, garbage) (fail) | `go test -run` green; card PDF rendered on the local stack shows `PREFIX-…`; roster CSV checked |
| T10 | Integration verification | whole branch | T3, T5, T6, T8, T9 | — | `backend.sh`, `tsc`, `vitest`, `npm run build`; AC 1–11 with evidence; `git diff --check` |

Parallel frontiers: **{T1, T4}** → **{T2, T3, T9}** → **{T5, T6, T7}** → T8 → T10. T2, T3, and T7 touch `routes.go` and the RBAC table: separate anchored blocks. T2 (card-cache clear on membership change) and T9 both touch exam-registration code in different files; T9 owns `service/exam.go`.
