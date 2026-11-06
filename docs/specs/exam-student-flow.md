---
title: Exam student flow — card siswa, detail "sebelum mulai", dan pratinjau admin
issue: none; related context 185
status: accepted
created: 2026-11-03
sources:
  - docs/specs/exam-session-resume.md
  - docs/specs/composable-exam-format.md
  - docs/specs/exam-user-flow.md (worktree exam-flow-prototype, bagian Siswa dan Compatibility)
  - docs/backlog/exam-user-flow-plan.md (T1/T4, worktree exam-flow-prototype)
  - docs/prototypes/exam-flow/README.md, HANDOFF.md (worktree exam-flow-prototype)
  - backend/internal/model/exam.go
  - backend/internal/repository/exam.go
  - backend/internal/service/exam_session.go
  - backend/internal/service/rbac.go
  - backend/internal/server/routes.go
  - web/app/(student)/exam/page.tsx
  - web/app/(student)/exam/[id]/page.tsx
  - web/lib/hooks/exam.ts
  - web/lib/types.ts
---

# Exam student flow (PR a3)

## Problem and outcome

Card dan detail siswa belum menjelaskan aturan ujian composable. `RegistrationListItem` (`model/exam.go`) hanya membawa jadwal, `duration_minutes`, dan `max_attempts`; tidak ada navigasi, lingkup timer, jumlah section/soal. `RegistrationDetail.Exam` hanya membawa subset konfigurasi, dan `timer_mode`-nya adalah kolom mentah. Untuk format 1 kolom itu tidak mencerminkan UTBK/IELTS, yang per-section lewat `Mode`. Halaman detail (`web/app/(student)/exam/[id]/page.tsx`) langsung menampilkan gate mulai tanpa penjelasan aturan. Admin tidak punya cara melihat tampilan itu tanpa mendaftar sebagai siswa.

Outcome: siswa membaca aturan (navigasi, timer, urutan section, jumlah soal) sebelum mulai, dan mulai hanya setelah mencentang pemahaman. Membuka detail tidak membuat sesi atau percobaan. Admin `super_admin` dan `admin_exam` dapat melihat tampilan yang sama secara read-only. UI tidak pernah membaca `mode`; semua aturan datang dari backend. Keputusan UI mengikuti prototype yang telah ditinjau owner (`exam-user-flow.md`, bagian Siswa).

## Agreed behavior

**Cakupan a3:** card siswa, detail siswa "sebelum mulai", dan pratinjau admin read-only atas detail itu. Di luar a3: WhatsApp/"aturan tambahan" (PR berikutnya); redesign list/pengaturan admin dan `ExamModal` (milik PR a2; a3 hanya menambah tombol dan route pratinjau); redesign halaman sesi runtime.

### Card siswa

- Hierarki: judul/status, lalu waktu/section/soal, lalu jadwal akses, lalu aturan navigasi singkat, lalu percobaan dan CTA.
- Jadwal satu hari: tanggal di kiri, rentang jam di kanan. Lintas hari menampilkan kedua tanggal. Zona Asia/Jakarta.

| State | Tampilan | CTA |
|---|---|---|
| ready (`start`/`checkin` existing) | aturan ringkas | "Lihat aturan ujian", membuka detail, tidak memulai |
| upcoming (`locked` existing) | badge netral "Belum dibuka", hitung mundur | "Lihat detail" |
| in_progress | tetap | "Lanjutkan ujian" (perilaku resume yang sudah di-merge, tidak berubah) |
| submitted, retake, expired, revoked, kartu, token, hasil | tidak berubah | tidak berubah |

### Detail siswa sebelum mulai

1. Card "Informasi ujian", dua panel. (a) Jadwal akses, dengan catatan bahwa akhir akses hanya membatasi memulai sesi baru. (b) Waktu pengerjaan beserta lingkup timer (satu timer ujian atau per section). Mobile menumpuk panel.
2. Card "Cara mengerjakan": aturan navigasi, lingkup timer, dan section berurutan dengan jumlah soal, presentasi, dan durasi.
3. "Siap memulai?": checkbox pemahaman wajib dicentang sebelum tombol mulai aktif. Tombol tetap memanggil `POST /exam/sessions` (`useStartSession`). Alur check-in dan resume tidak berubah. Penolakan server tampil sebagai error existing.
4. Membuka atau merender detail hanya `GET`. Tidak ada sesi, registrasi, atau percobaan baru.

### Pratinjau admin

- Tombol "Pratinjau siswa" di header detail ujian admin (`web/app/(admin)/admin/exam/packages/[id]/page.tsx`, `AdminPageHeader`). Tombol menuju route admin yang merender komponen detail yang sama dalam mode read-only: tombol mulai dan check-in dinonaktifkan, dengan label pratinjau.
- Hanya `super_admin` dan `admin_exam`. `admin_school` dan `admin_store` tidak melihat tombol dan mendapat 403 dari API.

### Kompatibilitas

- Format 1 tetap legacy. Format 2 memakai `navigation_policy`/`timer_mode`. Tidak ada migrasi data.
- id/en harus sejajar (`web/lib/i18n.ts`).

## Technical contract

### Aturan efektif (satu sumber, di service)

Helper baru di `service/exam_session.go`, ditaruh di samping `sequentialSession`/`sectionTimer` dan memakai logika yang sama persis:

| Format | navigation_policy | timer_mode |
|---|---|---|
| 1, `mode` utbk/ielts | `sequential` | `per_test` |
| 1, `mode` standard | `free` | `overall` |
| 2 | kolom `navigation_policy` | kolom `timer_mode` |

Signature helper mengambil `(formatVersion, mode, navigationPolicy, timerMode)`. Fungsi existing `sequentialSession(*model.Exam)` dan `sectionTimer(*model.Exam)` tetap, dan dites paritas terhadap helper baru. Repository hanya membawa kolom mentah (`json:"-"`); service yang mengisi nilai efektif. Aturan bisnis tidak masuk repository.

### Read model

- `RegistrationListItem` (query `GetExamRegistrationsByStudent`, `repository/exam.go`) ditambah: `navigation_policy`, `timer_mode` (efektif), `section_count`, `question_count`, `sections_duration_minutes` (`*int`).
  - Agregat lewat subquery atau `LEFT JOIN LATERAL` pada `exam_test`/`test`/`test_question` dalam query yang sama, tanpa N+1.
  - `sections_duration_minutes` = `SUM(test.duration_minutes)`, null bila tidak ada Test.
- `RegistrationDetail.Exam` ditambah: `navigation_policy`, `max_attempts`, dan `sections[]` terurut `exam_test.sort_order`, berisi `{test_id, title, duration_minutes, question_count, question_presentation}`.
  - `timer_mode` yang sudah ada diganti nilai efektif. Tidak ada konsumen web untuk field ini (hanya tipe di `web/lib/types.ts`), dan jalur cetak kartu tidak membacanya (`Exam.TimerMode` hanya di-scan di repository).
  - Tambah `sections_duration_minutes`, `section_count`, `question_count` agar sama dengan list.
  - Format 1: `question_presentation` selalu `one_per_page`, karena kolom `exam_test.question_presentation` default-nya `one_per_page` dan format 1 tidak memakainya di runtime.
- `GetExamRegistrationByID` juga dipakai `StartSession` (`service/exam_session.go`). Section dibaca lewat repository method terpisah yang dipanggil hanya dari `GetExamRegistration` dan pratinjau, supaya jalur start tidak menambah query.
- Payload tidak boleh memuat body soal, opsi, material, kunci jawaban, atau correctness. Hanya hitungan dan metadata Test.

### Endpoint pratinjau

- `GET /api/v1/admin/exams/:id/preview` mengembalikan bentuk yang sama dengan `RegistrationDetail.Exam` ditambah `sections[]`, tanpa registrasi, token, atau data kartu. Query yang sama dipakai bersama detail siswa (satu fungsi repository per exam id).
- RBAC: kapabilitas `results-workspace:read`. Verifikasi `service/rbac.go`: hanya `admin_exam` (eksplisit) dan `super_admin` (`*`) yang memilikinya; `admin_school` dan `admin_store` tidak. Ini juga idiom group `adminResultsWorkspace` di `server/routes.go`. Namanya salah secara semantik untuk pratinjau. Keputusan: kapabilitas baru `exam-preview:read` ditambahkan ke `RoleAdminExam` saja (`super_admin` otomatis lewat `*`), didaftarkan di group route baru di `routes.go`. Alasannya agar kontrak pratinjau tidak ikut bergeser bila `results-workspace:read` kelak diberikan ke role lain. Alternatif tanpa perubahan `rbac.go` adalah memakai `results-workspace:read`.
- Handler tidak memanggil service sesi atau registrasi. Tidak ada tulis ke DB atau Redis.

### Frontend

- `RegistrationListItem` dan `RegistrationDetail` di `web/lib/types.ts` diperluas. Hook baru `useExamPreview(id)` di `web/lib/hooks/admin-exams.ts`. `useRegistrations` dan `useRegistration` tidak berubah.
- Komponen presentasional detail diekstrak satu file dan dipakai halaman siswa dan route pratinjau. State `computeCardState` di `(student)/exam/page.tsx` dipertahankan, ditambah rendering metadata. Tidak ada fetch per baris.

## Acceptance and proof

| # | Kriteria | Bukti |
|---|---|---|
| 1 | Helper efektif memetakan format 1 standard, utbk, ielts, dan format 2 (`free+overall`, `sequential+overall`, `sequential+per_test`) sesuai tabel, dan sama dengan `sequentialSession`/`sectionTimer` untuk semua kombinasi | unit test service (tabel) |
| 2 | Agregat list dan detail benar untuk: tanpa Test (`section_count`=0, durasi null, soal 0), beberapa Test, Test tanpa soal; urutan `sections[]` mengikuti `sort_order` | repository test real-DB |
| 3 | Registrasi milik siswa lain: `GET /exam/registrations/:id` mengembalikan 404 | handler test |
| 4 | JSON list, detail, dan pratinjau tidak memuat key `body`, `options`, `material_html`, `correct_answer`, `is_correct` | handler test (cek key terlarang di JSON) |
| 5 | Pratinjau: `super_admin` dan `admin_exam` 200; `admin_school`, `admin_store`, `student` 403; exam tidak ada 404 | handler/RBAC matrix test |
| 6 | Tanpa efek samping: jumlah baris `exam_session` dan `exam_registration` sama sebelum dan sesudah memanggil detail siswa dan pratinjau | real-DB test |
| 7 | Card: state ready, upcoming, in_progress, submitted menampilkan CTA dan teks sesuai tabel; submitted/retake/expired tidak regresi | vitest `(student)/exam/page.test.tsx` |
| 8 | Detail: tombol mulai nonaktif sampai checkbox dicentang; resume `in_progress` tetap "Lanjutkan ujian"; mode pratinjau tidak memanggil `useStartSession`/check-in | vitest `[id]/page.test.tsx` |
| 9 | Tombol "Pratinjau siswa" ada di header detail admin untuk super_admin/admin_exam saja | vitest `admin/exam/packages/[id]/page.test.tsx` |
| 10 | Paritas key id/en untuk semua teks baru | tes i18n existing atau tambahan |
| 11 | Tanpa overflow horizontal dokumen di 360/390/768/1024/1440 px untuk card dan detail (judul panjang, locale terpanjang) | Playwright |
| 12 | Gates | `./deploy/pipeline/backend.sh`, `./deploy/pipeline/frontend.sh` |

Perubahan dari prototype: hitung soal dan agregat durasi berasal dari backend, bukan fixture; pratinjau memakai endpoint admin sendiri, bukan endpoint siswa atau registrasi palsu.

## Scope

- **Non-goals:** WhatsApp/"aturan tambahan"; redesign list admin, pengaturan, dan `ExamModal` (PR a2); perubahan runtime sesi; perubahan aturan start/check-in/resume server; migrasi DB.
- **Residual risk:**
  - Hitung soal dibaca live dari `test_question` (Test bisa diedit setelah registrasi). Angka di card dapat berubah sebelum mulai; ini sejalan dengan runtime yang juga membaca Test live.
  - `Test.duration_minutes` bersifat `NOT NULL` (`0014_exam.up.sql`, `model.Test.DurationMinutes int`). Jadi `sections_duration_minutes` hanya null bila tidak ada Test; kasus "durasi Test null" tidak dapat terjadi. Tipe `*int` tetap dipertahankan sesuai keputusan owner.
  - Pratinjau hanya menggambarkan konfigurasi, bukan state registrasi nyata (jadwal check-in, percobaan). State itu disimulasikan netral ("ready") di mode pratinjau.
