---
title: Testimonials managed by admins, with a consent gate
issue: none yet
status: proposed
created: 2026-11-17
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/landing-v2.md", "docs/specs/event-series.md", "docs/specs/profile-avatars.md", "docs/specs/admin-list-toolbar.md", "docs/specs/design-tokens-v2.md", "docs/prototypes/redesign-v2/index.html", "abak-redesign-prototype/index.html (Panca's local copy, 2026-11-26)", "backend/internal/server/routes.go", "backend/internal/service/student.go", "backend/internal/service/admin_users.go"]
---

# Testimonials

In scope of the redesign (Panca, 2026-11-26): built on a branch cut from `feat/public-landing`, PR into `feat/public-landing`, not `main`. Its place in the PR order is roadmap decision RD1.

## Problem and outcome

The landing page's "Cerita peserta" section needs real student, parent, and teacher stories. Today there is nowhere to store them. The prototype's three quotes are hard-coded sample data, and the WordPress site has none. Publishing a minor's name, photo, and words also needs written consent from a parent or guardian, and nothing records that.

Outcome: admins add and edit testimonials under **Akademik › Testimoni**. A testimonial shows on the landing page only when it has recorded consent and both language versions. The landing page reads them from a public API.

## Consumer: landing "Cerita peserta" (prototype `storySlide`, 2026-11-26)

One slide at a time beside a fixed award-ceremony photo (a landing asset, not per testimonial): eyebrow "CERITA PESERTA · 01/03", the quote in the visitor's language, the person (uploaded photo **or** a face from the 35-avatar sheet, name, school/role), up to 3 gold trophy chips (achievements), and previous/next icon buttons. The section renders only when at least one testimonial is visible **and** consented. Every field above is served by the public API below; the avatar needs the sheet asset in `web/` (D2).

## Agreed behavior

Decisions (Panca, 2026-11-17, from prototype review at `#/admin/testimoni`), unchanged:

1. A testimonial has:
   - Person: name, school or role (free text, for example "SMA Negeri 1 Depok" or "Guru BK, SMA Kornita IPB"), and either an avatar from the sheet (`profile-avatars.md`) or an uploaded photo.
   - Quote in Indonesian and English.
   - Achievements in Indonesian and English, up to 3 short chips each (for example "Medali perak olimpiade 2025").
   - Consent: a checkbox "Ada persetujuan tertulis" plus a required note on where the consent is kept (for example "Formulir izin via WhatsApp, 12 Nov 2026"). The consent is from the person, or from a parent or guardian when the person is under 18.
   - Visibility ("Tampil di landing page") and a sort order.
2. **Consent gate.** A testimonial can be made visible only when consent is recorded, the consent note is filled, and the EN quote is filled. The switch and the save are both rejected otherwise, with a message that names the missing part.
3. Unchecking consent on a visible testimonial hides it in the same save.
4. The admin page is a card grid with the shared `ListToolbar` (`admin-list-toolbar.md`; availability per event-series D5): search name or quote; filters Landing (shown/draft), Consent (yes/no), EN (complete/missing). Each card shows the person, quote, chips, flags (consent, ID + EN), a visibility switch, Edit, and Delete (with a confirm modal). The prototype's "Data contoh" flag marks its own sample data only; it has no column and is not built.
5. The landing carousel shows visible testimonials in sort order, in the visitor's language. With none visible, the section is not rendered.
6. Every change to consent or visibility is written to the audit log (actor, testimonial id, old and new values).

Added 2026-11-26 (proposed; confirm in align):

7. Achievements per language are independent: `?lang=en` returns the EN chips, possibly none, and never falls back to the ID chips (a mixed-language slide reads as a bug).
8. Replacing or removing a photo, or deleting a testimonial, leaves the old storage object in place (no delete call); see the privacy note under Photo.

## Data model

Migration `0076_testimonials` (after `0075_event_series` if that lands first; the latest on `feat/public-landing` @ 34ea5d32 is `0074`, so re-check the next free number on the branch base before creating):

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

`content` uses the same locale-keyed shape as `event_series.content` (`id` and `en` only; 422 on other keys; at most 3 achievements per locale). `consent_by` and `consent_at` are set by the server when `has_consent` changes to true, not sent by the client. The table CHECK backs up the service rule, so a bad write cannot publish a testimonial without consent. Neither avatar nor photo is allowed: the landing then shows the sheet's default face (D2).

Down migration drops the table.

## API

**Admin.** New RBAC permission `testimonials:write` (and `:read`), roles per D1.

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/testimonials` | All, including drafts, with `consent_by` name |
| POST | `/admin/testimonials` | Create. `is_visible` defaults to false |
| PATCH | `/admin/testimonials/:id` | Partial update. 422 with `missing: ["has_consent", "consent_note", "content.en.quote"]` when `is_visible` would be true without them. `has_consent: false` also sets `is_visible: false` in the same write |
| DELETE | `/admin/testimonials/:id` | Hard delete; it is marketing content, not a record of a transaction. Audit entry kept |

Audit: `InsertAuditLogMeta(ctx, nil, actor, "testimonial", id, "testimonial.consent_change" \| "testimonial.visibility_change" \| "testimonial.delete", {old, new})`, the same helper `admin_users.go` uses.

**Photo.** Uses `POST /admin/uploads/image`; the key lands under `question/`, which the anonymous `GET /files/*` already serves (verified 2026-11-26, see event-series "Logo storage"). Prefix choice is event-series D7, decided once for both. Privacy: a **draft** photo of a minor is fetchable by anyone holding its key, and stays so after removal (behavior 8). Keys are not listed anywhere public; the T1 PR confirms they are unguessable and records it here.

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/testimonials?lang=id\|en` | Visible only (`is_visible`, which the CHECK ties to consent), `sort_order` then `created_at`; each `{id, name, role_label, photo_url, avatar_index, quote, achievements}` for `lang` (default `id`). Exactly one of `photo_url`/`avatar_index` is non-null, or both null (default face). Never returns consent fields or `sort_order`. `Cache-Control: public, max-age=300` |

## Admin UI (web)

- Nav: "Testimoni" in Akademik, right after Event (`admin-nav-accordion.md`; visible to the D1 roles only).
- Page `/admin/testimonials` (admin list template: header + ListToolbar + grid + EmptyState): the card grid and toolbar from Agreed behavior 4; "Tambah testimoni" opens the editor.
- Editor `Dialog` (default 480px; scrolls inside): name, school/role, avatar picker (D2) or photo upload with preview, quote ID, quote EN, achievements ID and EN (one per line, max 3), a highlighted consent box (checkbox + note), the visibility switch, and Save (`brand` button). Delete confirmation uses the `destructive` button.
- Prototype reference: `#/admin/testimoni` in `docs/prototypes/redesign-v2/index.html`; visual reference: the design-system artifact (`design-tokens-v2.md`). Its three entries are sample data and are not seeded.

## Acceptance

1. The migration applies and rolls back cleanly; the table CHECK rejects `is_visible = true` with no consent and a row with both avatar and photo at the SQL level.
2. Service tests: visibility rejected for each missing part; unchecking consent hides the testimonial in the same write; `consent_by` and `consent_at` set by the server and not overwritable by the client; unknown locale key and a 4th achievement rejected; audit entries for consent, visibility, and delete with old/new values.
3. Handler tests: RBAC per role (D1); the public endpoint never returns drafts, consent fields, or `sort_order`; `?lang=en` returns English quote and EN achievements only (empty EN chips stay empty); no `lang` returns Indonesian.
4. Web tests: create, edit, delete round-trip; the visibility switch is blocked with the right message per missing part; filters work; avatar picker and photo upload are mutually exclusive.
5. Landing (`landing-v2.md`): zero visible means no "Cerita peserta" section in the HTML.
6. `./deploy/pipeline/backend.sh`, `npx tsc --noEmit`, `npx vitest run`, `npm run build` pass.

## Non-goals

- Students submitting their own testimonials.
- Storing the signed consent file. The note points to where it is kept; uploading the document can be added later.
- Deleting storage objects on photo replace or testimonial delete.
- Video testimonials.
- The landing section itself (`landing-v2.md`).

## Decisions needed

Recommendation first.

- **D1. Who manages testimonials.** a) `super_admin` only (default from 2026-11-17; consent of minors is a legal matter, keep it narrow). b) Also `admin_exam` (whoever runs events collects stories). c) A new marketing role (out of scope here).
- **D2. Avatar sheet.** The 35-face sheet and its renderer are not in `web/` yet (`profile-avatars.md`, phase 6, not built). a) This PR adds the sheet asset (`web/public`, open-licensed per the roadmap) and one small `AvatarFace` component (index → sprite position) that phase 6 reuses; the picker in the editor is a grid of 35 faces. b) Photo-only until phase 6 lands; `avatar_index` stays in the schema but the editor hides the picker and the landing shows initials. c) Land phase 6's avatar part first.
- **D3. Photo/media prefix.** Shared with event-series D7; decide once.

## Implementation plan

Branch `feat/testimonials` from `feat/public-landing` (after the PRs RD1 puts before it); one PR into `feat/public-landing` (or folded into the event-series PR if RD1 = b). Tasks for `.agents/work/testimonials.json`, test-first per `tdd-implementation`. Assumes the recommended options and that the generic admin `ListToolbar` already exists (event-series T5); if testimonials lands first, that task moves here unchanged.

| ID | Title | Files / scope | depends_on | Test-first step | Done-check |
|----|-------|---------------|------------|-----------------|------------|
| T1 | Schema + model + repository | `backend/db/migrations/0076_testimonials.{up,down}.sql`, `internal/model/testimonial.go`, `internal/repository/testimonial.go` (+ tests) | — | Repository tests against Postgres: CRUD, both CHECKs reject at SQL level, visible-only ordered list (fail) | migration up/down/up; repository tests green |
| T2 | Admin service + handlers + audit | `internal/service/testimonial.go`, `internal/handler/admin_testimonials.go`, RBAC (D1), admin routes (+ tests) | T1 | Service cases of AC 2 (each missing part, consent-off hides, server-set consent fields, locale/achievement limits, audit rows) and RBAC per role (fail) | `go test -run` both packages green |
| T3 | Public endpoint | `handler/testimonials_public.go`, public route, flattening per `lang` (+ tests) | T1 | No drafts, no consent fields, EN chips not falling back, photo vs avatar exclusivity (fail) | tests green; `curl` on the local stack |
| T4 | Avatar face (D2 = a) | sheet asset in `web/public`, `web/components/ui/avatar-face.tsx` (+ test) | — | Index 1 and 35 map to the right sprite cells; out-of-range renders the default face (fail) | test green; asset licence noted in PR body |
| T5 | Admin web page + editor + nav | `app/(admin)/admin/testimonials/**`, `lib/hooks/testimonials.ts`, `NAV_CONFIG` Akademik › Testimoni, i18n (+ tests) | T2, T4 | Round-trip, blocked visibility message per missing part, filters, picker/photo exclusivity (fail) | tests green; 390/1366 light/dark browser check |
| T6 | Integration verification | whole branch | T3, T5 | — | `backend.sh`, `tsc`, `vitest`, `npm run build`; AC 1–6 each with evidence; `git diff --check` |

Parallel frontiers: **{T1, T4}** → **{T2, T3}** → **T5** → T6. T2 and T3 both add to `routes.go` and the RBAC table: separate anchored blocks.
