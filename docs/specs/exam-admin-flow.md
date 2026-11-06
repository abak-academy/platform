---
title: Exam admin flow — UI admin ujian yang tidak menyesatkan setelah composable exam
issue: none; related context 185
status: accepted
created: 2026-11-03
sources:
  - docs/specs/composable-exam-format.md
  - .worktrees/exam-flow-prototype/docs/specs/exam-user-flow.md
  - .worktrees/exam-flow-prototype/docs/backlog/exam-user-flow-plan.md
  - backend/internal/model/exam.go
  - backend/internal/repository/exam.go
  - backend/internal/service/exam.go
  - backend/internal/handler/exam_package.go
  - web/app/(admin)/admin/exam/packages/page.tsx
  - web/app/(admin)/admin/exam/packages/[id]/page.tsx
  - web/components/admin/ExamModal.tsx
  - web/lib/hooks/admin-exams.ts
  - web/lib/types.ts
---

# Exam admin flow

## Problem and outcome

Setelah composable exam, UI admin masih menyesatkan:

- Daftar menampilkan `timer_mode` mentah dan badge `draft/published` milik Exam, padahal penjualan/publikasi dikelola di Produk (`PublishProduct` hanya mengubah `product.status`).
- Daftar tidak menunjukkan susunan (jumlah section/soal) maupun waktu pengerjaan efektif; untuk `per_test`, `duration_minutes` Exam null sehingga durasi tampak kosong.
- Detail mengulang judul/navigasi/timer di banyak tempat, menampilkan field check-in/rilis hasil yang tidak berlaku, dan tidak memberi tahu bahwa policy terkunci setelah ada sesi (admin baru tahu dari error server).
- Warning materi `all_on_page` hanya teks statis ("sebelum publish"), tidak menyebut section/soal yang bermasalah.
- `max_attempts` berupa input angka kosong yang tidak menjelaskan arti kosong/0/1.

Outcome: admin melihat susunan, cara pengerjaan, dan waktu yang benar langsung di daftar; satu pintu edit (`Pengaturan`); kontrol policy terkunci dengan penjelasan saat sudah ada sesi; pilihan percobaan eksplisit. Arah UI sudah ditinjau owner (lihat `exam-user-flow.md`, bagian Admin); PR ini hanya memecah bagian admin-nya (T1 read model + T2 admin flow).

## Agreed behavior

Keputusan owner (tidak dibuka ulang):

- **Scope a2:** membuat UI admin tidak menyesatkan memakai komponen existing (`AdminPageHeader`, toolbar/search existing, pola markup tabel Produk). Di luar: WhatsApp, "aturan tambahan", redesign page Tes, halaman siswa, tombol/endpoint "Pratinjau siswa" (milik PR a3).
- **Daftar (5 kolom):**

| Kolom | Isi |
| --- | --- |
| Ujian | judul + `registration_count` (school-scoped, tidak berubah) |
| Susunan | `N section · M soal`; format 1 menampilkan "Format sebelumnya" |
| Cara pengerjaan | navigasi + ringkasan presentasi; format 1 menampilkan mode legacy |
| Waktu pengerjaan | durasi overall, atau total durasi section bila `per_test`; "—" bila tidak diketahui |
| Jadwal akses | `scheduled_at` – `scheduled_end_at` |

  Badge dan filter status Exam dihapus dari UI saja (kolom/filter `status` backend tetap untuk caller lain). Catatan kaki: publikasi/penjualan dikelola lewat Produk. Search mengikuti toolbar existing; tidak ada filter/sort pengganti.
- **Detail:**
  - Satu tombol `Pengaturan` di header membuka `ExamModal`; entry edit di kartu ringkasan dipindah ke sini (tidak ada dua pintu).
  - Tab ringkasan tidak mengulang judul/policy. Jendela/toleransi check-in tampil hanya bila check-in aktif; rilis hasil hanya bila `result_config != hidden`. Field yang tidak tampil tetap tersimpan apa adanya saat save.
  - Tab "Tes" diganti nama "Tes & section"; select presentasi per section dengan label tidak terpotong.
  - Konflik `all_on_page` menyebut section dan soal (posisi/judul, tidak pernah UUID); perbaikan aman: pilih `one_per_page` atau ubah materi sumber. Tidak ada penghapusan materi otomatis. Tidak ada preview, sehingga tidak ada kewajiban simpan-sebelum-preview.
  - Badge status dan `admin_exam_detail_draft_notice` dihapus dari header/detail.
- **Percobaan di `ExamModal`:** checkbox "Izinkan lebih dari satu percobaan". Mati → simpan `max_attempts=1`. Aktif → input integer ≥2 (default 2) plus checkbox terpisah "Tanpa batas" → `null` (input angka disembunyikan). Kosong tanpa "Tanpa batas" tidak valid. Create via UI default 1 (diterima owner). Edit memetakan `null` → keduanya aktif, `0/1` → utama mati, `N≥2` → utama aktif dengan N. Membaca tidak menulis ulang; default API untuk caller lain tidak berubah.
- **Policy lock:** bila `has_sessions`, kontrol navigasi/timer/durasi/randomize dan kontrol section (attach/remove/urut/presentasi) disabled dengan penjelasan; field non-policy tetap bisa diedit. Guard server tetap authoritative: pada 409 `exam_has_sessions` UI menampilkan pesan terlokalisasi (id/en) lalu refetch detail.

## Technical contract

**Read model (aggregate SQL di dalam query existing, tanpa N+1, tanpa konten soal):**

| DTO | Field baru | Definisi |
| --- | --- | --- |
| `model.ExamListItem` | `section_count int` | `COUNT(exam_test)` per exam |
| | `question_count int` | jumlah `test_question` lintas Test terlampir |
| | `sections_duration_minutes *int` | `SUM(test.duration_minutes)` Test terlampir; `null` hanya bila tidak ada Test (`test.duration_minutes` NOT NULL) |
| `model.ExamDetail` | `has_sessions bool` | `EXISTS (SELECT 1 FROM exam_session WHERE exam_id = e.id)`, status apa pun |

- Subquery ditambahkan di `Repository.ListExams` (mengikuti pola subquery `registration_count`/`has_published_product`; urutan placeholder dipertahankan, `$1` tetap `SchoolFilter`) dan `scanExamListItem`. `GetExamDetail` mengisi `has_sessions`; boleh memanggil `ExamHasAnySession` yang sudah ada atau EXISTS inline, tanpa mengubah semantik guard.
- `registration_count` tetap school-scoped dan tidak ada di detail. Sesi, bukan registrasi, adalah dasar lock.
- Guard `checkExamPolicyEditSafety` dan `checkExamTestsEditSafety` (`service/exam.go`) tidak diubah; keduanya mengembalikan `ErrExamHasSessions` → handler 409 kode `exam_has_sessions` (`handler/errors.go`). Pesan server berbahasa Inggris; UI memetakan `ApiError.code === "exam_has_sessions"` ke kunci i18n id/en.
- **Konflik materi (disetujui):** deteksi `all_on_page` + materi saat ini hanya ada di `validateComposablePublishSections` (publish produk) dan menyebut `q.ID` (UUID). Agar UI bisa menyebut soal tanpa UUID dan tanpa membawa konten, `ExamTestEntry` mendapat `material_question_positions []int` (posisi 1-based menurut `test_question.sort_order` untuk soal yang punya `material_html`/`image_url`/`audio_url`, definisi sama dengan `hasQuestionMaterial`). Hanya posisi, tanpa isi materi; endpoint admin saja, bukan projection siswa.
- Frontend: `web/lib/types.ts` menambah field di `ExamListItem` dan `ExamDetail`. Catatan: `ExamDetail extends ExamListItem`, jadi field list ikut terwariskan dan harus tetap opsional; `has_sessions` hanya di `ExamDetail`. Hook (`useExams`, `useExam`, `useUpdateExam`, `useReplaceExamTests`) tidak berubah bentuk; payload attachment tetap `[{test_id, question_presentation}]`.
- Daftar memakai markup tabel native `overflow-x-auto md-card-outlined` + `<table>` seperti `app/(admin)/admin/products/page.tsx`, mengganti baris `div role="button"` saat ini; `data-testid="package-row"` dan navigasi klik/Enter ke detail dipertahankan.
- `ExamModal` mendapat prop untuk status lock (turunan `has_sessions`); state `max_attempts` diganti dua boolean + angka. `nonNegativeInt` tidak lagi dipakai untuk field ini.
- RBAC/school scope tidak berubah; `admin_school` tetap tanpa kontrol authoring (`isSchoolScoped`). Tidak ada migrasi DB.

## Acceptance and proof

1. Repository test (real DB): list dengan 0 Test → `section_count=0`, `question_count=0`, `sections_duration_minutes=null`; >1 Test → jumlah dan SUM benar; `registration_count` dengan `SchoolFilter` tidak berubah (regresi); registrasi `revoked` tetap tidak dihitung.
2. Repository/handler test: `has_sessions` false tanpa sesi; true untuk sesi `in_progress` maupun hanya `submitted`; registrasi tanpa sesi tetap false. `GET /admin/exams` dan `GET /admin/exams/:id` mengembalikan field baru; filter `status` backend masih berfungsi.
3. Test posisi materi: posisi soal ber-materi benar, soal tanpa materi tidak muncul, respons tidak memuat isi materi.
4. Vitest daftar: 5 kolom; baris format 2 menampilkan section/soal dan waktu overall atau total per-section; baris format 1 menampilkan "Format sebelumnya" + mode; tanpa badge status; request tidak mengirim `status`; clear search mereset kursor; loading/error/empty tetap.
5. Vitest detail: satu tombol `Pengaturan`; ringkasan tanpa judul/policy duplikat; check-in/grace hanya saat check-in aktif; rilis hasil hanya saat bukan `hidden`; field tersembunyi ikut terkirim saat save; tab "Tes & section"; pesan konflik menyebut section dan soal tanpa UUID.
6. Vitest `ExamModal` attempts: create default → `1`; centang + 2 → `2`; "Tanpa batas" → `null`; kosong tanpa "Tanpa batas" → submit disabled; edit `0`, `1` → utama mati; `null` → dua-duanya aktif; `N≥2` → aktif dengan N; membuka lalu menutup tanpa save tidak memanggil PATCH.
7. Vitest policy lock: `has_sessions=true` → kontrol policy dan section disabled + penjelasan, field non-policy aktif; `has_sessions=false` → aktif. Respons 409 `exam_has_sessions` menampilkan pesan id dan en (locale diganti) dan memicu refetch.
8. Playwright: daftar dan detail pada 390/1024/1440 px tanpa overflow horizontal dokumen; kontainer tabel boleh scroll internal. `web/e2e/exam-packages-toolbar.spec.ts` diperbarui karena select status dihapus.
9. Gates: `./deploy/pipeline/backend.sh`, `./deploy/pipeline/frontend.sh`.

## Non-goals

WhatsApp, "aturan tambahan", redesign page Tes, halaman siswa, tombol/endpoint "Pratinjau siswa" (PR a3); menghapus atau menyinkronkan kolom/filter `status` backend; migrasi DB; mengubah default `max_attempts` API atau backfill nilai existing; guard server; framework tabel baru; total lintas halaman daftar (tetap cursor-based).

## Residual risk

- Ujian tanpa `scheduled_at` tidak muncul di Monitor Sesi (`ListExamMonitorCandidates` memfilter `scheduled_at IS NOT NULL`). Sudah diketahui dan diterima owner; tidak diperbaiki di PR ini.
- `has_sessions` adalah snapshot saat baca; sesi yang mulai setelahnya hanya tertangkap guard server (409 → pesan lokal + refetch).
- Satu Exam dapat dipakai banyak Produk dan satu Test banyak Exam; `sections_duration_minutes` membaca durasi Test live, bukan override per-attachment.
- Create via UI default 1 percobaan mengubah perilaku form (sebelumnya kosong = tanpa batas); dampaknya hanya pada exam baru dari UI.
