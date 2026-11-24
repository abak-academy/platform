---
title: Profile avatars from the illustration sheet, photo upload with preview, password modal
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/specs/redesign-v2-roadmap.md", "docs/prototypes/redesign-v2/avatars/", "docs/prototypes/redesign-v2/index.html", "web/app/(student)/profile/page.tsx", "web/components/profile/ChangePasswordForm.tsx", "backend/internal/model/user.go", "backend/internal/server/routes.go", "backend/internal/service/exam.go"]
---

# Profile avatars and profile page modals

## Problem and outcome

Today a student's picture is either an uploaded photo or nothing, so most leaderboards and cards show initials. The prototype introduces 35 illustrated avatars a student can pick, keeps photo upload, and moves photo and password changes into modals so the profile page shows only profile data.

Outcome:

- Students pick an avatar or upload a photo in one modal, with pick → preview → Save.
- Users without either show a stable default avatar.
- Password change is a modal opened from the profile card.

## Agreed behavior

Decisions (Panca, 2026-11-17):

1. The **profile card** (left column) shows the large avatar with an edit badge, name, grade · school, and verification chip. Below them are two buttons: **Ganti foto profil** and **Ubah kata sandi**. The avatar grid and the password form are no longer inline on the page.
2. The **Foto profil modal** has a preview panel on the left ("Foto saat ini" or "Pratinjau — belum disimpan") and two tabs:
   - **Pilih avatar**: a 35-tile grid. Clicking a tile only selects it (brand ring) and updates the preview.
   - **Upload foto**: a file picker for JPG, PNG, or WebP up to 5 MB, center-cropped to a square, shown in the preview. Picking another file replaces the preview.
   - **Simpan** is disabled until something changed. **Batal** discards the change. **Hapus foto** (shown only when a photo is set) returns to the default avatar.
3. Choosing an avatar replaces an uploaded photo, and uploading a photo replaces an avatar. Only one is active.
4. **Default avatar:** a user with neither sees a deterministic avatar from the sheet, chosen by a hash of their user ID. This is display-only; nothing is written.
5. **Password modal:** current password, new password (minimum 8, with the existing strength rules), and confirm. On success the modal closes and shows a toast. It reuses `ChangePasswordForm` logic.
6. The same picture appears everywhere a user is shown: navbar, leaderboards, admin tables, and the participant card and certificate, which are server-rendered.

## Technical contract

**Storage, without a migration.** Upload the 35 files from `docs/prototypes/redesign-v2/avatars/01.webp…35.webp` to the storage bucket under `avatars/sheet/01.webp…35.webp`. That prefix is already served anonymously by `GET /files/*`, which allows `avatars/` only. Choosing avatar N sets `users.photo_url` to the key `avatars/sheet/NN.webp`.

Why: the participant card and certificate are rendered by the worker from `photo_url`, so the avatar works there with no change. It also needs no new column. The trade-off: "is this an avatar or a photo?" is answered by the key prefix. Keep that check in one helper (`isSheetAvatar(key)`) on each side.

**API.**
- The existing photo update endpoint (the one the profile page calls after presigned upload) also accepts an `avatars/sheet/NN.webp` key.
- The service validates that a sheet key is one of the 35, and that any other key belongs to the caller's own upload prefix.
- **Uncertain:** the exact endpoint name and its current validation. Read the handler behind `updatePhoto` in `app/(student)/profile/page.tsx` before changing it.

**Default avatar.** A pure function `defaultAvatarIndex(userId) → 1..35` (FNV-1a mod 35), shared by web and, if the worker needs it for cards, Go. Both implementations must have the same test vectors.

**Web.** New `ProfilePhotoDialog` and `ChangePasswordDialog` in `components/profile`. `Avatar` gains a fallback to the default sheet avatar instead of initials.

**Ops.** A one-off script under `backend/scripts/` or `web/scripts/` uploads the 35 files idempotently. Its run is noted in the PR body.

**Licensing.** The avatar sheet is open-licensed (confirmed by Panca, 2026-11-17). No attribution is required in the UI.

## Acceptance

1. Pick avatar → Simpan: `photo_url` = sheet key, and the navbar avatar updates without reload.
2. Upload photo → preview shows → Simpan: the existing upload flow runs, and an avatar previously set is replaced.
3. Batal and closing the modal discard the selection (test).
4. Hapus foto shows the default avatar, and the default is identical across reloads and devices for the same user.
5. A participant card PDF for a user with a sheet avatar shows the avatar (worker test or manual check noted in the PR).
6. The password modal shows the existing validation and errors; on success it closes.

## Non-goals

- Admin avatar picker (admins keep upload only).
- Generating new avatar art.
