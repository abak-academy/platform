---
title: Testimonials managed by admins, with a consent gate
issue: none yet
status: agreed
created: 2026-11-17
updated: 2026-11-26
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/specs/landing-v2.md", "docs/specs/event-series.md", "docs/specs/profile-avatars.md", "docs/specs/admin-list-toolbar.md", "docs/specs/design-tokens-v2.md", "docs/prototypes/redesign-v2/index.html", "abak-redesign-prototype/index.html (Panca's local copy, 2026-11-26)", "backend/internal/server/routes.go", "backend/internal/service/student.go", "backend/internal/service/admin_users.go", "backend/internal/handler/admin_uploads.go", "backend/internal/service/card_logo.go"]
---

# Testimonials

In scope of the redesign (Panca, 2026-11-26): built on a branch cut from `feat/public-landing` after the event-series PR merges (roadmap RD1 = a; it reuses that PR's admin `ListToolbar`), in parallel with the student PR; PR into `feat/public-landing`, not `main`.

## Problem and outcome

The landing page's "Cerita peserta" section needs real student, parent, and teacher stories. Today there is nowhere to store them. The prototype's three quotes are hard-coded sample data, and the WordPress site has none. Publishing a minor's name, photo, and words also needs written consent from a parent or guardian, and nothing records that.

Outcome: admins add and edit testimonials under **Akademik › Testimoni**. A testimonial shows on the landing page only when it has recorded consent and both language versions. The landing page reads them from a public API.

## Consumer: landing "Cerita peserta" (prototype `storySlide`, 2026-11-26)

One slide at a time. The left panel is **the photo of the person speaking** (4:5) and changes with each slide; there is no shared award-ceremony photo (Panca, 2026-11-26; `landing-v2.md` update block). Beside it: eyebrow "CERITA PESERTA · 01/03", the quote in the visitor's language, name and school/role, up to 3 gold trophy chips (achievements), and previous/next icon buttons. The section renders only when at least one testimonial is visible, which implies consent and a photo. Every field above is served by the public API below.

## Agreed behavior

Decisions (Panca, 2026-11-17, from prototype review at `#/admin/testimoni`), unchanged:

1. A testimonial has:
   - Person: name, school or role (free text, for example "SMA Negeri 1 Depok" or "Guru BK, SMA Kornita IPB"), and a photo (behaviors 10–11). The avatar-sheet alternative was dropped on 2026-11-26.
   - Quote in Indonesian and English.
   - Achievements in Indonesian and English, up to 3 short chips each (for example "Medali perak olimpiade 2025").
   - Consent: a checkbox "Ada persetujuan tertulis" plus a required note on where the consent is kept (for example "Formulir izin via WhatsApp, 12 Nov 2026"). The consent is from the person, or from a parent or guardian when the person is under 18.
   - Visibility ("Tampil di landing page") and a sort order.
2. **Consent gate.** A testimonial can be made visible only when consent is recorded, the consent note is filled, the EN quote is filled, and a photo is uploaded (photo added 2026-11-26). The switch and the save are both rejected otherwise, with a message that names the missing part.
3. Unchecking consent on a visible testimonial hides it in the same save.
4. The admin page is a card grid with the shared `ListToolbar` (`admin-list-toolbar.md`; built by the event-series PR, its D5 = a): search name or quote; filters Landing (shown/draft), Consent (yes/no), EN (complete/missing). Each card shows the person, quote, chips, flags (consent, ID + EN), a visibility switch, Edit, and Delete (with a confirm modal). The prototype's "Data contoh" flag marks its own sample data only; it has no column and is not built.
5. The landing carousel shows visible testimonials in sort order, in the visitor's language, each slide with the speaker's own photo. With none visible, the section is not rendered.
6. Every change to consent or visibility is written to the audit log (actor, testimonial id, old and new values).
7. Only `super_admin` manages testimonials (D1, Panca 2026-11-26): consent of minors is a legal matter, so access stays narrow.

Added 2026-11-26, settled 2026-11-26 (Panca accepted both):

8. Achievements per language are independent: `?lang=en` returns the EN chips, possibly none, and never falls back to the ID chips (a mixed-language slide reads as a bug).
9. Replacing or removing a photo, or deleting a testimonial, leaves the old storage object in place (no delete call); see the privacy note under Photo.

Decided 2026-11-26 (Panca):

10. **Photo required to show.** A testimonial is visible on the landing only with a photo. Drafts may have none.
11. **Standard photo** ("resolusi nanti dibuat standar"): portrait **4:5**, stored at **1200 × 1500** as **JPEG** (canvas quality about 0.85). The admin picks a JPG, PNG, or WebP file of at most **5 MB** and at least **1200 × 1500 px**; the admin web center-crops it to 4:5, resizes it to 1200 × 1500, encodes JPEG in the browser, shows a live preview, and uploads only on Save. The browser crop is the gate; the server checks only the key prefix (D2 = a).
12. Removing the photo of a visible testimonial hides it in the same save, like unchecking consent (3), because a removal request may be a privacy request.

## Data model

Migration `0076_testimonials` (after `0075_event` from the event-series PR, which merges first; the latest on `main` is `0074`, so re-check the next free number on the branch base before creating):

```sql
CREATE TABLE testimonial (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           TEXT NOT NULL,
  role_label     TEXT NOT NULL,
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
  CHECK (NOT is_visible OR photo_key IS NOT NULL)
);
CREATE INDEX testimonial_visible_idx ON testimonial (sort_order) WHERE is_visible;
```

`content` uses the same locale-keyed shape as the event `content` (`event-series.md`) (`id` and `en` only; 422 on other keys; at most 3 achievements per locale). `consent_by` and `consent_at` are set by the server when `has_consent` changes to true, not sent by the client. The table CHECKs back up the service rule, so a bad write cannot publish a testimonial without consent or without a photo. `photo_key` stores the object key, not a URL; the service accepts only a key under the `question/` upload prefix (D3), the same guard `UpdatePhoto` applies to student avatars (`avatarKeyFromStored`, `service/exam.go`), so no arbitrary URL is stored.

Down migration drops the table.

## API

**Admin.** New RBAC permission `testimonials:write` (and `:read`), `super_admin` only (D1; `super_admin` already holds `*`, so no role table change beyond naming the permission).

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/testimonials` | All, including drafts, with `consent_by` name |
| POST | `/admin/testimonials` | Create. `is_visible` defaults to false |
| PATCH | `/admin/testimonials/:id` | Partial update. 422 with `missing: ["has_consent", "consent_note", "content.en.quote", "photo_key"]` (the ones absent) when `is_visible` would be true without them. `has_consent: false` or `photo_key: null` also sets `is_visible: false` in the same write (behaviors 3, 12) |
| DELETE | `/admin/testimonials/:id` | Hard delete; it is marketing content, not a record of a transaction. Audit entry kept |

Audit: `InsertAuditLogMeta(ctx, nil, actor, "testimonial", id, "testimonial.consent_change" \| "testimonial.visibility_change" \| "testimonial.delete", {old, new})`, the same helper `admin_users.go` uses.

**Photo.** Uses `POST /admin/uploads/image` with no backend change; the key lands under `question/`, which the anonymous `GET /files/*` already serves (verified 2026-11-26; D3, shared with event-series logos). Privacy: a **draft** photo of a minor is fetchable by anyone holding its key, and stays so after removal (behavior 9). Keys are not listed anywhere public; the T1 PR confirms they are unguessable and records it here.

What the upload path checks today (verified 2026-11-26): `AdminUploadImage` (`handler/admin_uploads.go`) only requires `content_type` to start with `image/`; `GeneratePresignedUploadURL` (`service/student.go`) returns a 15-minute presigned **PUT** with no size cap and no dimension check, and the bytes go straight to storage, never through the API. So nothing server-side enforces 5 MB, 1200 × 1500, or the format; the admin web's crop is the only gate (accepted, D2 = a: only `super_admin` can write, and a direct API caller with that token could store any image). `GET /files/*` serves `image/jpeg` and `image/webp` inline (`safeInlineImageTypes`, `handler/student.go`).

**Public, no auth:**

| Method | Path | Notes |
|---|---|---|
| GET | `/testimonials?lang=id\|en` | Visible only (`is_visible`, which the CHECKs tie to consent and a photo), `sort_order` then `created_at`; each `{id, name, role_label, photo_url, quote, achievements}` for `lang` (default `id`). `photo_url` (`<api>/files/<photo_key>`, as event-series `logo_url`) is always present. Never returns consent fields, `photo_key`, or `sort_order`. `Cache-Control: public, max-age=300` |

## Admin UI (web)

- Nav: "Testimoni" in Akademik, right after Event (`admin-nav-accordion.md`; `super_admin` only).
- Page `/admin/testimonials` (admin list template: header + ListToolbar + grid + EmptyState): the card grid and toolbar from Agreed behavior 4; "Tambah testimoni" opens the editor.
- Editor `Dialog` (default 480px; scrolls inside): name, school/role, the photo field, quote ID, quote EN, achievements ID and EN (one per line, max 3), a highlighted consent box (checkbox + note), the visibility switch, and Save (`brand` button). Delete confirmation uses the `destructive` button.
- Photo field ("Foto peserta"): file input accepting `image/jpeg,image/png,image/webp`; a 4:5 preview of the cropped result; help text "Wajib untuk tayang. JPG/PNG/WebP, maks 5 MB, minimal 1200 × 1500 px. Dipotong otomatis ke 4:5 dan disimpan 1200 × 1500. Hanya unggah dengan izin peserta/orang tua." Too-large and too-small files are refused at pick time ("Foto lebih dari 5 MB", "Foto minimal 1200 × 1500 px"). The cropped file is uploaded on Save, before the PATCH that stores its key, so a cancelled edit leaves no object.
- The visibility switch, when the photo is missing, is blocked with "Unggah foto peserta sebelum tampil di landing" (prototype wording), alongside the existing consent and EN messages.
- Prototype reference: `#/admin/testimoni` in `docs/prototypes/redesign-v2/index.html`; visual reference: the design-system artifact (`design-tokens-v2.md`). Its three entries are sample data and are not seeded.

## Acceptance

1. The migration applies and rolls back cleanly; the table CHECKs reject `is_visible = true` with no consent and with no `photo_key` at the SQL level; a draft without a photo is accepted.
2. Service tests: visibility rejected for each missing part, photo included; removing the photo of a visible testimonial hides it in the same write; a `photo_key` outside the upload prefix is rejected; unchecking consent hides the testimonial in the same write; `consent_by` and `consent_at` set by the server and not overwritable by the client; unknown locale key and a 4th achievement rejected; audit entries for consent, visibility, and delete with old/new values.
3. Handler tests: only `super_admin` passes RBAC (D1); the public endpoint never returns drafts, consent fields, `photo_key`, or `sort_order`, and every item has `photo_url`; `?lang=en` returns English quote and EN achievements only (empty EN chips stay empty); no `lang` returns Indonesian.
4. Web tests: create, edit, delete round-trip; the visibility switch is blocked with the right message per missing part, photo included; filters work; a file over 5 MB or under 1200 × 1500 is refused with its message; an accepted file is cropped to a 1200 × 1500 `image/jpeg` blob (a 3000 × 2000 landscape input keeps its center); the upload happens on Save, not on pick.
5. Landing (`landing-v2.md`): zero visible means no "Cerita peserta" section in the HTML.
6. `./deploy/pipeline/backend.sh`, `npx tsc --noEmit`, `npx vitest run`, `npm run build` pass.

## Non-goals

- Students submitting their own testimonials.
- Storing the signed consent file. The note points to where it is kept; uploading the document can be added later.
- Deleting storage objects on photo replace or testimonial delete.
- Video testimonials.
- The landing section itself (`landing-v2.md`).
- An avatar-sheet face for testimonials (dropped 2026-11-26). `profile-avatars.md` may still build the sheet for student profiles.
- A crop editor (drag/zoom): the crop is always centered; an admin who needs another framing crops the file before upload.
- A server-side size or dimension gate on the upload itself: the presigned PUT has none today, and this spec adds none (D2 = a).

## Decisions

All settled (Panca, 2026-11-26):

- **D1. Who manages testimonials:** `super_admin` only (behavior 7).
- **D2. Photo format and server-side checking:** (a) JPEG from the canvas, trust the browser crop; the server checks only the key prefix. No new dependency, no backend change on the upload path. Rejected: (b) a save-time `image.DecodeConfig` check (storage read in the save path), (c) WebP (Go stdlib cannot decode it; `toBlob('image/webp')` can silently return PNG).
- **D3. Photo/media prefix:** reuse `question/` via `POST /admin/uploads/image` (same as event-series logos, event-series D7).
- **Rule 12 (photo removal hides):** accepted.
- **Behaviors 8–9:** settled 2026-11-26: EN achievements never fall back to ID; replaced or removed photos and deleted testimonials leave their storage objects in place.

## Implementation plan

Branch `feat/testimonials` from `feat/public-landing` after the event-series PR merges; one PR into `feat/public-landing`, in parallel with the student PR. Tasks for `.agents/work/testimonials.json`, test-first per `tdd-implementation`. Uses the generic admin `ListToolbar` from the event-series PR (event-series T4; event-series D5 = a is settled).

| ID | Title | Files / scope | depends_on | Test-first step | Done-check |
|----|-------|---------------|------------|-----------------|------------|
| T1 | Schema + model + repository | `backend/db/migrations/0076_testimonials.{up,down}.sql`, `internal/model/testimonial.go`, `internal/repository/testimonial.go` (+ tests) | — | Repository tests against Postgres: CRUD, both CHECKs reject at SQL level (no consent, no photo), photo-less draft accepted, visible-only ordered list (fail) | migration up/down/up; repository tests green |
| T2 | Admin service + handlers + audit | `internal/service/testimonial.go`, `internal/handler/admin_testimonials.go`, RBAC (D1), admin routes (+ tests) | T1 | Service cases of AC 2 (each missing part incl. photo, consent-off and photo-removal hide, key-prefix guard, server-set consent fields, locale/achievement limits, audit rows) and RBAC: only `super_admin` (fail) | `go test -run` both packages green |
| T3 | Public endpoint | `handler/testimonials_public.go`, public route, flattening per `lang` (+ tests) | T1 | No drafts, no consent fields or `photo_key`, `photo_url` on every item, EN chips not falling back (fail) | tests green; `curl` on the local stack |
| T4 | Admin web page + editor + nav | `app/(admin)/admin/testimonials/**`, `lib/hooks/testimonials.ts`, a small crop helper beside the editor, `NAV_CONFIG` Akademik › Testimoni, i18n (+ tests) | T2 | Round-trip; blocked visibility message per missing part (photo included); 5 MB and 1200 × 1500 refusals; crop to a 1200 × 1500 JPEG; upload on Save only (fail) | tests green; 390/1366 light/dark browser check including a real photo upload and its `/files/` URL |
| T5 | Integration verification | whole branch | T3, T4 | — | `backend.sh`, `tsc`, `vitest`, `npm run build`; AC 1–6 each with evidence; `git diff --check` |

Parallel frontiers: **T1** → **{T2, T3}** → **T4** → T5. T2 and T3 both add to `routes.go` and the RBAC table: separate anchored blocks.
