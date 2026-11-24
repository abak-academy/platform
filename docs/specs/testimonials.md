---
title: Testimonials managed by admins, with a consent gate
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/landing-v2.md", "docs/specs/event-series.md", "docs/specs/profile-avatars.md", "docs/prototypes/redesign-v2/index.html", "backend/internal/server/routes.go"]
---

# Testimonials

## Problem and outcome

The landing page's "Cerita peserta" section needs real student, parent, and teacher stories. Today there is nowhere to store them. The prototype's three quotes are hard-coded sample data, and the WordPress site has none. Publishing a minor's name, photo, and words also needs written consent from a parent or guardian, and nothing records that.

Outcome: admins add and edit testimonials under **Akademik › Testimoni**. A testimonial shows on the landing page only when it has recorded consent and both language versions. The landing page reads them from a public API.

## Agreed behavior

Decisions (Panca, 2026-11-17, from prototype review at `#/admin/testimoni`):

1. A testimonial has:
   - Person: name, school or role (free text, for example "SMA Negeri 1 Depok" or "Guru BK, SMA Kornita IPB"), and either an avatar from the sheet (`profile-avatars.md`) or an uploaded photo.
   - Quote in Indonesian and English.
   - Achievements in Indonesian and English, up to 3 short chips each (for example "Medali perak olimpiade 2025").
   - Consent: a checkbox "Ada persetujuan tertulis" plus a required note on where the consent is kept (for example "Formulir izin via WhatsApp, 12 Nov 2026"). The consent is from the person, or from a parent or guardian when the person is under 18.
   - Visibility ("Tampil di landing page") and a sort order.
2. **Consent gate.** A testimonial can be made visible only when consent is recorded, the consent note is filled, and the EN quote is filled. The switch and the save are both rejected otherwise, with a message that names the missing part.
3. Unchecking consent on a visible testimonial hides it in the same save.
4. The admin page is a card grid with the shared `ListToolbar` (`admin-list-toolbar.md`): search name or quote; filters Landing (shown/draft), Consent (yes/no), EN (complete/missing). Each card shows the person, quote, chips, flags (consent, ID + EN, sample), a visibility switch, Edit, and Delete (with a confirm modal).
5. The landing carousel shows visible testimonials in sort order, in the visitor's language. With none visible, the section is not rendered.
6. Every change to consent or visibility is written to the audit log (actor, testimonial id, old and new values).

## Data model

Migration `0073_testimonials` (after `0072_event_series`; verify the next free number before creating):

```sql
CREATE TABLE testimonial (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           TEXT NOT NULL,
  role_label     TEXT NOT NULL,
  avatar_index   SMALLINT NULL CHECK (avatar_index BETWEEN 1 AND 35),
  photo_key      TEXT NULL,
  content        JSONB NOT NULL DEFAULT '{}',  -- {"id": {"quote": "", "achievements": [""]}, "en": {...}}
  has_consent    BOOLEAN NOT NULL DEFAULT false,
  consent_note   TEXT NULL,
  consent_by     UUID NULL REFERENCES users(id),
  consent_at     TIMESTAMPTZ NULL,
  is_visible     BOOLEAN NOT NULL DEFAULT false,
  sort_order     INT NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (NOT is_visible OR (has_consent AND consent_note IS NOT NULL)),
  CHECK (avatar_index IS NULL OR photo_key IS NULL)
);
CREATE INDEX testimonial_visible_idx ON testimonial (sort_order) WHERE is_visible;
```

`content` uses the same locale-keyed shape as `event_series.content`. `consent_by` and `consent_at` are set by the server when `has_consent` changes to true, not sent by the client. The table CHECK backs up the service rule, so a bad write cannot publish a testimonial without consent.

Down migration drops the table.

## API

**Admin.** New RBAC permission `testimonials:write` for super_admin. *Default: super_admin only; widen during align if marketing staff use another role.*

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/testimonials` | All, including drafts, with `consent_by` name |
| POST | `/admin/testimonials` | Create. `is_visible` defaults to false |
| PATCH | `/admin/testimonials/:id` | Partial update. 422 with `missing: ["has_consent", "consent_note", "content.en.quote"]` when `is_visible` would be true without them |
| DELETE | `/admin/testimonials/:id` | Hard delete; it is marketing content, not a record of a transaction |

**Photo.** Uses the admin image upload (`POST /admin/uploads/image`). It has the same public-read question as the event logo (`event-series.md`, Logo). Resolve both with one decision in whichever phase lands first.

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/testimonials?lang=id\|en` | Visible only, sorted, flattened to `{name, role_label, image_url, quote, achievements}` for `lang` (default `id`). Never returns consent fields. `Cache-Control: public, max-age=300` |

## Admin UI (web)

- Nav: "Testimoni" in Akademik, right after Event (`admin-nav-accordion.md`).
- Page `/admin/testimonials`: the card grid and toolbar from Agreed behavior 4; "Tambah testimoni" opens the editor.
- Editor modal: name, school/role, avatar picker or photo upload with preview, quote ID, quote EN, achievements ID and EN (one per line), a highlighted consent box (checkbox + note), the visibility switch, and Save.
- Prototype reference: `#/admin/testimoni` in `docs/prototypes/redesign-v2/index.html`. Its three entries are sample data, marked "Data contoh", and are not seeded.

## Acceptance

1. The migration applies and rolls back cleanly; the table CHECK rejects `is_visible = true` with no consent at the SQL level.
2. Service tests: visibility rejected for each missing part; unchecking consent hides the testimonial; `consent_by` and `consent_at` are set by the server; audit entries for consent and visibility changes.
3. Handler tests: RBAC; the public endpoint never returns drafts or consent fields; `?lang=en` returns English.
4. Web tests: create, edit, and delete round-trip; the visibility switch is blocked with the right message; filters work.
5. Landing (`landing-v2.md`): zero visible means no section in the HTML.

## Non-goals

- Students submitting their own testimonials.
- Storing the signed consent file. The note points to where it is kept; uploading the document can be added later.
- Video testimonials.

## Suggested task graph

| Task | Outcome | Depends on |
|---|---|---|
| T1 | Migration, model, repository | — |
| T2 | Admin service + handlers + RBAC + audit + tests | T1 |
| T3 | Public endpoint + tests | T1 |
| T4 | Admin web page + editor + nav item | T2 |
