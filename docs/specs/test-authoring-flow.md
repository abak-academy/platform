---
title: Test authoring flow (PR b)
issue: none; related context 185
status: accepted
created: 2026-11-03
sources:
  - docs/prototypes/exam-flow/HANDOFF-2.md
  - docs/prototypes/exam-flow/README.md
  - docs/prototypes/exam-flow/tests.js
  - docs/prototypes/exam-flow/app.js
  - docs/specs/exam-user-flow.md
  - docs/specs/composable-exam-format.md
  - web/components/admin/QuestionPickerModal.tsx
  - web/components/admin/RichTextEditor.tsx
  - backend/internal/handler/exam.go
---

# Test authoring flow (PR b)

## Problem and outcome

Admin membutuhkan alur Tes yang membedakan susunan soal, materi/instruksi, dan identitas/audio. Question bank harus dapat difilter dan dibaca tanpa teks panjang keluar dari komponen. Aturan tambahan Exam dan WhatsApp dikerjakan di PR (c), bukan di sini.

Panca telah meninjau prototype sampai revisi 7; keputusan visual/UX di bawah merekam hasil review tersebut dan disetujui untuk implementasi sebagai PR (b) yang di-merge ke `feat/composable-exam-format`. Basis kode: `d826aa39` (setelah PR a1–a3).

Dokumen ini melanjutkan [exam-admin-flow](exam-admin-flow.md) dan [exam-student-flow](exam-student-flow.md). Ownership dan runtime tetap mengikuti [composable exam](composable-exam-format.md).

## Agreed behavior

### Halaman Tes

| Area | Perilaku yang ditinjau |
| --- | --- |
| Visual | Pertahankan palet, font, brand, shell, dan referensi komponen existing. Header/search/filter/table mengikuti standar yang sama dengan Exam, bukan varian baru per modul. |
| Daftar | Header Penyusun Tes; search judul, filter mata pelajaran dan topik terkait. Kolom Tes, mata pelajaran/topik, jumlah soal, durasi, materi/audio. Tidak menambahkan status publikasi Tes. |
| Pagination | Pertahankan cursor, total, loading/error/retry existing. Reset cursor dan hasil saat search/filter berubah; data fixture bukan seluruh katalog. |
| Create | Judul, mata pelajaran, topik, durasi positif → buat → detail kosong untuk menyusun soal/materi. Gunakan katalog topik existing; input teks fixture bukan katalog produksi. |
| Detail | Tab Soal, Materi & instruksi, Informasi tes. Satu Simpan pengaturan untuk draft metadata/material/instruksi/audio lintas tab. Tab switching tidak menghapus draft. |
| Soal | Tambah dari bank, pindahkan urutan, lepas dari Tes. Operasi ini tersimpan melalui API masing-masing; tombol Simpan pengaturan tidak menunda atau menggabungkannya. |
| Aksi | Ikon preview dan lepas berjajar horizontal, punya accessible name/tooltip. Lepas membutuhkan dialog konfirmasi dan hanya menghapus attachment ke Tes, bukan Question bank. |
| Materi | Sidebar menampilkan rich content dengan paragraf, tabel, gambar, scroll internal, dan Lihat lengkap. Materi kosong punya empty state. Jangan flatten menjadi plain text atau memotong gambar tanpa jalan membaca lengkap. |
| Editor | Materi dan instruksi memakai `RichTextEditor` existing yang digunakan Question, termasuk gambar, tabel, daftar, sub/superscript, dan rumus. Bukan editor `contenteditable`/`execCommand` dari prototype. |
| Informasi | Identitas, durasi, section_type, audio dan batas pemutaran tetap ada. Listening memerlukan audio. Audio limit null=tanpa batas; pertahankan nilai positif existing di luar opsi 1/2/3. |
| Preview | Preview konten Tes bersifat read-only dan tidak membuat sesi/attempt. Mode satu/semua soal adalah simulasi tampilan; policy/presentation tetap milik section di Exam. Materi Question menghalangi `all_on_page`. |
| Sumber bersama | Perubahan konten Tes dapat memengaruhi beberapa Exam, termasuk sesi berjalan. Durasi sesi yang sudah dibuat mengikuti snapshot. Tidak mengklaim revision/immutability baru. |

### Question picker

- Pertahankan search + filter topik dan format dari `QuestionPickerModal`/bank soal existing. API memakai `search`, `topic_id`, `format`; filter topik adalah ID, bukan label nama.
- Lima format filter existing: `mcq`, `multi_answer`, `essay`, `multi_blank`, `true_false`. Legacy `short`/`fill_blank` tetap bisa terbaca melalui Semua/search dan tetap editable sesuai aturan existing; jangan migrasi atau menghapusnya.
- Ringkasan body maksimal tiga baris, wrapping termasuk kata/URL tanpa spasi. Lihat selengkapnya membuka isi di dalam row, Ringkas teks mengembalikan ringkasan. Expand tidak memilih checkbox.
- Pilihan soal bertahan saat filter/search berubah, termasuk item yang sedang tidak terlihat. Footer menunjukkan seluruh jumlah dipilih; submit tidak aktif jika nol. Reset pencarian/filter tidak menghapus pilihan. Tutup/buka ulang memulai selection baru seperti existing.
- Attached questions tidak dapat dipilih ulang. Empty/error/loading dan hasil panjang tetap dapat dibaca; tidak mengeklaim semua bank telah dimuat hanya dari satu halaman API.
- Backend dapat menolak attachment ke sibling Test dalam Exam yang sama. Tampilkan error, pertahankan selection untuk koreksi, refetch bila perlu; fixture tidak memodelkan guard ini.

## Technical contract

### Fakta kode pada baseline

| Area | Kontrak existing yang harus dipertahankan |
| --- | --- |
| Test fields | `Test` sudah punya `shared_material_html`, `instructions_html`, `duration_minutes`, audio fields dan `section_type`; tidak perlu schema baru untuk layout/tab/editor. |
| Detail save | Page detail saat ini hanya mengirim identitas/durasi/audio/section_type. Backend sudah menerima material/instructions; UI baru perlu menghubungkan kedua draft ke PATCH, bukan menganggap existing detail save telah melakukannya. |
| Nullable | `AdminUpdateTest` memakai `Nullable[T]` untuk audio/section/material/instructions: absent preserve, null clear. TypeScript input audio masih belum memodelkan null, sementara page mengirim `Record<string, unknown>`; perbaiki typing pada slice yang diubah tanpa mengubah wire behavior. |
| Sanitization | Create/Update Test menjalankan `sanitizeOptionalHTML`. Gunakan pipeline rich-text/media/math existing dan renderer aman; whitelist JS prototype bukan sanitizer produksi. |
| Mutasi soal | Attach append/idempotent untuk ID sudah terpasang; sibling-test collision guard ada. Reorder membutuhkan set ID lengkap sama dengan attachment terkini. Detach menghapus join saja. Jangan mengganti dengan DELETE Question. |
| Data daftar | Test list sudah memiliki `question_count`, filtered total, next cursor. Field jumlah ujian yang memakai Test tidak ada pada DTO Test; angka “1 ujian contoh” bukan data produksi. Omit indikator itu kecuali kontrak real disetujui. |
| Permissions | `/admin/tests` memakai `tests:*`; `/admin/questions` dan `/admin/topics` memakai `questions:*`. Pertahankan akses existing, termasuk akses picker; jangan memperluas role untuk meniru fixture. |
| Kepemilikan | Tes memiliki konten/durasi; Exam attachment memiliki urutan/presentasi. Exam memiliki navigation/timer dan metadata event. Guard policy setelah ada sesi tidak menjadikan konten Test immutable. |

Save metadata dan mutasi attachment tidak menjadi satu transaksi baru. Pertahankan draft saat refetch/invalidation akibat attach/reorder; jangan reset form hanya karena query refresh. Loading/pending/error mengikuti server; jangan tampilkan “tersimpan” sebelum mutation sukses. Preview tidak memakai perubahan yang belum disimpan: minta simpan lebih dahulu, seperti prototype. Kebijakan meninggalkan halaman dengan draft perlu difinalisasi sebelum implementasi; prototype belum membuktikannya.

## Acceptance and proof

| ID | Outcome | Proof implementasi yang diperlukan |
| --- | --- | --- |
| T1 | List sesuai standar; search/filter/cursor/total benar | Component/hook request tests + browser empty/loading/error/retry dan reset pagination. |
| T2 | Create membuka Tes kosong; tiga tab dan satu save metadata | Validasi required/Listening/duration; draft bertahan lintas tab/refetch; material/instructions terkirim, persisted dan terbaca setelah reload. |
| T3 | Editor/preview materi memakai komponen existing | Round-trip paragraf/gambar/tabel/math melalui sanitizer, upload error, rendering aman dan responsive; tidak menyalin editor prototype. |
| T4 | Picker filter dan teks panjang usable | Search+topic_id+format intersection, legacy search, tiga baris/expand, unbroken text, selection lintas filter, empty state dan submit jumlah benar. |
| T5 | Reorder/attach/detach mengikuti server | ID lengkap/order request; confirmation cancel/confirm; bank Question tetap ada; duplicate/sibling collision/stale order/network error tests. |
| T6 | Preview tidak membuat sesi atau mengubah policy | Admin read authorization; satu/semua/materi precedence; zero start/attempt calls; seluruh format existing tidak regresi. |

Prototype evidence hanya membuktikan interaksi fixture dan layout. Proof produksi membutuhkan tests/API/database nyata sesuai [verification](../../.agents/verification.md), bukan report prototype.

## Scope

Termasuk page Tes dan picker/material preview serta addendum metadata Exam yang ditinjau. Tidak membangun ulang QuestionEditor, bank CRUD/import, engine scoring/session/audio, publikasi produk, atau modul lain. Existing create/edit Question, format legacy, delete Test dan endpoint lain tidak dihapus karena tidak ada pada fixture; pertahankan kemampuan existing atau minta keputusan eksplisit bila placement berubah.

Tidak mengadopsi local data, MCQ-only, fixed clock, reverse shuffle, blob upload, `execCommand`, atau perhitungan “used by 1 exam” sebagai produksi. Tidak ada snapshot/revision Test baru, penguncian konten baru, runtime compensation baru, atau migrasi format legacy.

Keputusan yang ditetapkan untuk build (2026-11-03):

- **Scope:** halaman Tes, question picker, dan pratinjau Tes. Aturan tambahan Exam dan WhatsApp (termasuk kartu status WhatsApp) pindah ke PR (c).
- **Pratinjau Tes:** read-only untuk role yang sudah memiliki akses Tes (`tests:*`); tidak membuat sesi/attempt/registrasi; memakai renderer aman existing dan semua format jawaban.
- **Meninggalkan halaman dengan draft:** bila draft metadata/materi/instruksi/audio belum disimpan, navigasi dalam aplikasi meminta konfirmasi "buang perubahan?" dan `beforeunload` memperingatkan saat tab ditutup/reload. Mutasi soal (attach/reorder/detach) tidak termasuk draft karena langsung tersimpan. Browser Back/Forward adalah batasan yang diketahui: tidak dicegat (App Router tidak dapat memblokir navigasi).
- **Usage count:** indikator "dipakai di N ujian" dihilangkan sampai ada kontrak data nyata.

Knowledge reflection: none untuk system truth.
