---
title: Exam UI parity — admin, Tes, and student screens match the reviewed prototype
issue: none; related context 185
status: accepted
sources:
  - docs/prototypes/exam-flow/README.md
  - docs/prototypes/exam-flow/HANDOFF-2.md
  - docs/prototypes/exam-flow/index.html
  - docs/prototypes/exam-flow/app.js
  - docs/prototypes/exam-flow/tests.js
  - docs/prototypes/exam-flow/style.css
  - docs/prototypes/exam-flow/tests.css
  - docs/specs/exam-admin-flow.md
  - docs/specs/exam-student-flow.md
  - docs/specs/test-authoring-flow.md
  - docs/specs/exam-event-info.md
---

# Exam UI parity

## Problem

PRs #240–#243 delivered the behaviour from the reviewed exam-flow prototype (revision 8) but kept each page's old layout. The prototype was never on the implementation branch, and the specs recorded behaviour tables plus "use existing components", so layout, grouping and hierarchy drifted. Owner reviewed the prototype screen by screen and wants **full visual parity** for admin and student. The admin Ringkasan also renders raw values ("Scheduled", "Requires check-in", "Free", "hidden"), which contradicts the a2 goal of a non-misleading admin.

This spec adds the prototype package to the branch (`docs/prototypes/exam-flow/`) and makes its revision 8 screens the visual acceptance.

## Agreed behavior

Owner decisions (2026-11-03, not reopened):

- **Shell stays.** Keep the real AppShell sidebar/topbar and the shared `AdminPageHeader` styling. Parity applies to page content: layout, grid, cards, tables, sidebars, grouping, copy, icons, badges, modal. The prototype's dark toolbar, simplified sidebar, demo-only copy ("Mode pratinjau", "data contoh") and fixture data are not built.
- **Theme tokens.** Use existing Tailwind/theme tokens and `components/ui/*`; dark mode must keep working. Do not paste prototype CSS. Pixel values in the prototype are targets for spacing/hierarchy, not literal copies.
- **Behaviour is preserved.** Everything delivered in #240–#243 (attempts checkbox, policy lock, conflict messages, extra rules, WhatsApp, leave guard, preview, picker filters, detach confirm, Edit/Hapus on Tes list, Soal baru) stays. Where the prototype lacks an existing capability, keep the capability and place it in the prototype's visual language.
- **One backend addition:** Test detail (and Test list if one grouped query) returns `exam_count` = number of exams the Test is attached to. Shown as "Dipakai pada N ujian"; hidden when 0.
- **Out of scope:** Sertifikat as its own tab (certificate stays where it is), shell/header restyle, new product behaviour not in the prototype, legacy (format 1) visual redesign beyond what shares the same components.

### PR 1 — Admin Exam (`/admin/exam/packages`, `[id]`, `ExamModal`)

| Screen | Prototype target |
| --- | --- |
| List | Full-width search with clear button and result count ("N ujian") on the right; five columns unchanged; footnote stays. |
| Detail header | Back link, icon, title, actions `Pratinjau siswa` (outline, eye icon) + `Pengaturan` (outline, sliders icon); meta line "N peserta terdaftar · belum ada sesi / sudah ada sesi" from `registration_count`/`has_sessions` replaces the "Berurutan · Timer per section · —" subtitle. Tab "Tes & section (N)". |
| Ringkasan | Two columns. Left: card **Informasi ujian** grouped into *Jadwal akses* (Mulai akses / Batas akhir mulai sesi), *Akses peserta* (Akses gratis, Maksimal percobaan, Check-in + window/grace only when active), *Hasil & informasi tambahan* (Tampilan hasil, rilis only when not hidden, Leaderboard, Aturan tambahan). Then card **Susunan section** (numbered rows: Test title, presentation, duration; `Atur section` opens the tab). Right: **Grup WhatsApp kompetisi** card. Kartu peserta / certificate controls stay, restyled in the same card language below Informasi ujian. All values localized (no raw enums/English keys). |
| Tes & section | Left card **Susunan section**: description, `Tambah Test` opens a dialog (search + library rows, add) instead of the always-visible "Tes Tersedia" list; per section a bordered card with number chip, title, subject · N soal, reorder ↑/↓ and remove, `Tampilan soal` select with helper text, `Waktu pengerjaan` (duration from Test, "Durasi dari sumber Test"), footer "Materi bersama · N materi per soal" + `Lihat sumber Test →` (opens the Tes page). Save bar at card bottom: status text + `Simpan susunan`. Reorder persists through existing `PUT /admin/exams/:id/tests`. Right: **Aturan pengerjaan** card (navigation rule, timer rule with total, "N soal · N section", extra-rules checkbox/item from #243, ownership footnote) + validity notice (green "Susunan section valid" or the existing conflict message). Locked state keeps controls disabled with the existing explanation. |
| Pengaturan modal | Title "Pengaturan ujian" + subtitle; wider modal (prototype ≈900px) with left form and right **Yang perlu dibedakan** panel (hidden on mobile). Sections: *Informasi & jadwal akses* (judul, mulai akses, batas akhir mulai sesi, helper, WhatsApp), *Aturan pengerjaan* (navigation and timer as choice cards with title + description, durasi, acak soal), *Akses peserta* (gratis, check-in fields, attempts checkbox), *Hasil*, *Kartu & layar selesai*. Footer: hint text left, `Batal` + `Simpan pengaturan`. Field names/payload unchanged. |

### PR 2 — Tes authoring (`/admin/exam/tests`, `[id]`, picker)

| Screen | Prototype target |
| --- | --- |
| List | Full-width search, `Filter` button (opens subject/topic filters) and count "N tes"; columns Tes / Mata pelajaran / topik / Soal ("N soal") / Durasi ("N menit") / Materi & audio (text: "Materi bersama" + "Ada materi per soal" / "Tanpa materi per soal", audio line when present). Edit/Hapus kept as row actions. Footnote "Tes adalah sumber soal…". |
| Detail header | Back link, icon, title, subtitle "Mapel · Topik"; actions `Pratinjau` + `Simpan pengaturan`. Meta row: N soal · N menit · Dipakai pada N ujian (from `exam_count`) · save status. Shared-Test notice card. Underline tabs (Soal (N), Materi & instruksi, Informasi tes) spanning full width; both columns below the tabs. |
| Soal tab | Card **Susunan soal** with `Dari bank soal` (primary) and `Soal baru`; table Urutan (number chip + ↑/↓) / Soal / Format badge / Materi ("Materi bersama" or warn badge "Materi sendiri") / Aksi (preview icon, detach). Right column: **Materi bersama** card (rich preview, scroll, `Lihat lengkap`, `Atur materi`) and **Tampilan saat ujian** card (presentation explanation, warn notice when any question has own material, duration footnote). |
| Materi & instruksi | Two cards side by side: **Materi bersama** (wide, helper text) and **Instruksi pengerjaan**, each with header + description and the existing `RichTextEditor`; ownership notice under the right card. |
| Informasi tes | Prototype card layout for identity, duration, section type, audio + play limit; same fields and validation. |
| Picker | Prototype layout for `QuestionPickerModal` (search, topic/format filters, rows with three-line clamp and expand, footer count). Behaviour unchanged. |

### PR 3 — Student (`/exam`, `/exam/[id]`, session runtime)

| Screen | Prototype target |
| --- | --- |
| Card | Kicker "PAKET UJIAN" + status badge (Siap dikerjakan / Belum dibuka / Sedang berjalan / Selesai); title; facts row with vertical dividers (Total waktu large + timer scope, Section, Soal); bordered access panel with 32px calendar icon, date left, hours right; navigation rule line; footer with attempts + hint and one primary CTA (Lihat aturan ujian → / Lanjutkan / result action). Secondary actions (Ulangi, Lihat hasil) kept but follow the footer layout. **Upcoming:** neutral badge, countdown days/hours/minutes from server time, CTA `Lihat detail`. Grid 3/2/1 columns. |
| Detail (before start) | Serif title; **Informasi ujian** with status badge and two panels (Jadwal akses: icon chip, full weekday date, large start–end hours + WIB, caption; Waktu pengerjaan: large minutes + "menit total", timer scope in brand colour, caption) and lock note. WhatsApp invitation after it. **Cara mengerjakan**: navigation, timer (with total and carry-over rule), "N soal · N section", extra rule, then Susunan section rows. **Siap memulai?** card unchanged in behaviour. Token card placement as prototype check-in. |
| Runtime | Header shows exam title, section progress, timer labelled with scope ("Sisa waktu section" / "Sisa waktu ujian"); section rail with active section; question card; right navigator with legend (answered, flagged, current); end-of-section / submit confirmation names unanswered count and that the section locks. Behaviour, autosave and audio unchanged. |

## Settings modal: tabs

Decision (2026-11-04): the admin Exam settings / Buat ujian modal uses tabs (Jadwal, Aturan, Peserta, Hasil, Tampilan; Tampilan only when editing) instead of the prototype's single scroll plus "What to tell apart" sidebar. Reason: product owner reported too much text and overlapping content, especially in Exam results and Card & end screen. Panels stay mounted so values survive tab switches, the body has a fixed height between bordered header and footer, a tab carries an error dot when a field in it blocks Save, and each field or group keeps at most one short helper line. Payloads and student behaviour are unchanged.

## Technical contract

- Backend (PR 2 only): `exam_count int` on Test detail JSON (`json:"exam_count"`), computed by a COUNT over `exam_test` in the repository read; list gets it only if added to the existing list query without N+1. Service/handler shape unchanged otherwise. Go test for 0 / N.
- Backend (PR 1, added during build 2026-11-03): additive read-only `test.has_shared_material` on admin exam detail entries (same non-blank rule as `material_question_positions`, no material HTML), so the section footer only says "Materi bersama" when the Test has it.
- No other API, DB, or payload change. Reorder uses the existing replace-tests endpoint.
- New i18n keys in `web/lib/i18n.ts` (id + en) for all new copy; no hardcoded strings.
- Prefer composing existing `components/ui/*`; add a shared component only when used by ≥2 of the touched pages.

## Verification

Per PR:

1. Vitest for every changed behaviour or new conditional render (meta line, grouped summary visibility rules, localized values, reorder payload, dialog add, countdown formatting, timer scope label, confirmation copy, exam_count display/hidden at 0). Existing tests updated, not deleted.
2. `npx tsc --noEmit`, `npx vitest run` (also on Node 20), `npm run build`; PR 2 also `./deploy/pipeline/backend.sh`.
3. Playwright screenshots of each listed screen at 1440 and 390 against the local stack, attached in the PR next to the matching prototype evidence PNG; no horizontal overflow at 360/390/768/1024/1440; dark mode screenshot of one screen per PR.
4. Owner visual review of the screenshot pairs is the parity acceptance.

Not proven: pixel-identical rendering (shell and tokens intentionally differ), legacy format-1 exams beyond shared components.
