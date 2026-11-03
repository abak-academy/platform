---
title: Exam event info — link grup WhatsApp dan aturan tambahan
issue: none; related context 185
status: accepted
created: 2026-11-03
sources:
  - docs/specs/composable-exam-format.md
  - docs/specs/exam-admin-flow.md
  - .worktrees/exam-flow-prototype/docs/specs/exam-user-flow.md
  - .worktrees/exam-flow-prototype/docs/specs/test-authoring-flow.md
  - .worktrees/exam-flow-prototype/docs/prototypes/exam-flow/HANDOFF-2.md
  - .worktrees/exam-flow-prototype/docs/prototypes/exam-flow/app.js
  - backend/internal/model/exam.go
  - backend/internal/service/exam.go
  - backend/internal/handler/exam_package.go
  - backend/internal/repository/exam.go
  - backend/db/migrations
  - web/components/admin/ExamModal.tsx
  - web/app/(admin)/admin/exam/packages/[id]/page.tsx
  - web/components/exam/ExamBriefing.tsx
  - web/lib/types.ts
---

# Exam event info

## Problem and outcome

Penyelenggara tidak punya tempat untuk mengabarkan informasi event kepada peserta: link grup WhatsApp kompetisi dan aturan tambahan (keterangan/tautan panduan teknis). Hari ini admin harus menyebarkannya di luar aplikasi, sehingga halaman ujian siswa tidak memuat informasi itu.

Outcome: admin dapat mengisi link grup WhatsApp (opsional) dan aturan tambahan (opsional) pada Exam; siswa, dan pratinjau admin yang memakai komponen briefing yang sama, melihat ajakan grup dan aturan tersebut hanya bila tersimpan. Keduanya bukan syarat check-in atau mulai ujian. Arah UI sudah ditinjau owner (prototype `exam-flow`, PR (c)); spec ini hanya mengikat kontrak produksinya.

## Agreed behavior

**Admin: WhatsApp.**
- Field opsional "Link grup WhatsApp kompetisi" hanya ada di `Pengaturan` (`ExamModal`), tidak ada entry edit kedua. Kosong berarti blok siswa tidak tampil.
- Tab Ringkasan mendapat card "Grup WhatsApp kompetisi": `Link tersedia` + URL tersimpan (tautan baru-tab), atau `Tidak dicantumkan` + penjelasan bahwa ajakan tidak ditampilkan kepada siswa. Kosong tidak menyatakan alasan admin mengosongkannya.

**Admin: aturan tambahan** (tab "Tes & section", bagian "Aturan pengerjaan").
- Tanpa nilai tersimpan, hanya checkbox "Tambahkan aturan tambahan" (mati). Aktif membuka field keterangan (textarea) dan link; mengaktifkan butuh minimal salah satu terisi.
- Simpan mengubah form menjadi item aturan tersimpan (ikon, judul, keterangan, link, tombol Edit). Batal membuang draft dan tidak mengubah nilai tersimpan. Sebelum Simpan, siswa tidak melihat perubahan.
- Edit → matikan checkbox → Simpan mengirim kedua field `null` dan menghilangkan blok siswa. Form yang mati tidak pernah memblokir save karena URL draft yang tidak valid.
- Simpan aturan adalah PATCH metadata terpisah dari "Simpan susunan" (attach/urut/presentasi); UI tidak mengaku satu transaksi.
- Aktif diturunkan dari salah satu nilai tersimpan; tidak ada boolean persisten.

**Tidak terkunci oleh sesi.** WhatsApp dan aturan tambahan bukan policy: tetap bisa diedit saat `has_sessions=true`. Kontrol policy dan section yang terkunci (lihat `exam-admin-flow.md`) tidak berubah.

**Siswa dan pratinjau admin** (`ExamBriefing`):
- Blok ajakan WhatsApp: kartu setelah "Informasi ujian" dan sebelum "Cara mengerjakan", hanya bila URL tersimpan, pada kondisi mendatang maupun terbuka. CTA sekunder "Gabung grup WhatsApp ↗" membuka tab baru dengan `rel="noopener noreferrer"`. Teks menyebut opsional dan bukan syarat ujian. Bergabung tidak dilacak.
- Aturan tambahan: satu item tambahan di dalam kartu "Cara mengerjakan", tipografi/jarak/divider sama dengan item existing, tanpa margin blok ekstra. Keterangan tampil sebagai teks (escaped, tanpa HTML) dengan newline dipertahankan; link membuka tab baru dengan `noopener noreferrer`. Tampil hanya bila note atau URL tersimpan; mendukung note saja, link saja, atau keduanya. Checkbox pemahaman existing tetap terpisah; membaca/mengklik link bukan syarat mulai.

## Technical contract

**Persistence.** Satu migrasi aditif `backend/db/migrations/0071_exam_event_info.{up,down}.sql` (nomor tertinggi saat ini `0070_affiliate`; runner menerapkan berdasarkan nama file via embed `db/embed.go`). Tiga kolom `exam` nullable tanpa default dan tanpa backfill: `whatsapp_group_url TEXT NULL`, `additional_rules_note TEXT NULL`, `additional_rules_url TEXT NULL`. Down melakukan `DROP COLUMN IF EXISTS` untuk ketiganya. Nama wire sama dengan nama kolom; `whatsappUrl`/`rulesNote` di prototype hanya fixture.

**Model.** `model.Exam` mendapat `WhatsAppGroupURL`, `AdditionalRulesNote`, `AdditionalRulesURL` (`*string`, json `whatsapp_group_url`, `additional_rules_note`, `additional_rules_url`). Karena `ExamListItem` dan `ExamDetail` meng-embed `Exam`, admin list/detail otomatis membawanya. `model.RegistrationExam` mendapat tiga field yang sama; `RegistrationListItem` tidak.

**Repository.** Kolom ditambahkan konsisten pada seluruh jalur yang memakai `scanExam`/`scanExamListItem` (urutan scan harus sama dengan SELECT): `CreateExam` (INSERT), `updateExam` (UPDATE, menimpa nilai dari overlay), `GetExamByID`, `GetExamsByProductID`, `ListExams`, `GetExamDetail`, dan `GetExamByIDTx` di `exam_expiry.go`. `GetExamPreview` men-SELECT ketiganya ke `RegistrationExam`. `GetExamRegistrationByID`/`GetRegistrationForPrint` (kartu) tidak disentuh.

**Handler.** `AdminCreateExam` mem-bind `model.Exam` langsung, jadi field terbaca dari body. `examPatchRequest` mendapat tiga `Nullable[string]`, di-overlay lewat `applyNullable` ke `overlay` (disalin dari `existing.Exam`): absent = preserve; `null` atau string blank = clear; nilai = set. PATCH field lain tidak pernah mengubah ketiganya.

**Service (otoritatif; browser hanya mempercepat umpan balik).** `CreateExam` dan `UpdateExam` menormalkan (trim; blank menjadi `nil`) sebelum `validateExam`, mengikuti pola `normalizeCardNotes`. Aturan di `validateExam`, error `ErrValidation` (400):

| Field | Aturan |
| --- | --- |
| `whatsapp_group_url` | ≤2048 karakter; URL absolut, scheme `https`, host persis `chat.whatsapp.com`, path tidak kosong (selain `/`) |
| `additional_rules_url` | ≤2048 karakter; URL absolut, scheme `http` atau `https`, host tidak kosong |
| `additional_rules_note` | teks polos, ≤500 rune; tidak ada sanitasi HTML karena dirender sebagai teks |

Server tidak pernah fetch, join, atau memverifikasi URL. Scheme lain (`javascript:`, `data:`) ditolak.

**Edit-safety.** `checkExamPolicyEditSafety` hanya membandingkan `NavigationPolicy`, `Randomize`, `TimerMode`, `DurationMinutes`; `checkExamTestsEditSafety` hanya dipanggil dari replace tests. Ketiga field baru tidak ikut perbandingan, sehingga edit saat ada sesi tidak menghasilkan `ErrExamHasSessions`. Guard tidak diubah; test regresi membuktikannya. Perubahan juga tidak memicu `notesChanged`/invalidasi kartu.

**Projection.** `GetStudentRegistrationDetail` (`GET /exam/registrations/:id`, owner-scoped via `GetExamRegistration(regID, studentID)`) menyalin tiga field dari `GetExamPreview` ke `detail.Exam`; `GetExamPreview` (service) melayani preview admin `AdminPreviewExam`. Tidak ada endpoint publik baru dan tidak ada konten soal, opsi, kunci, atau materi.

**Frontend.** `lib/types.ts`: tiga field opsional (`string | null`) pada `Exam` (diwarisi `ExamListItem`/`ExamDetail`), `CreateExamPayload`, `UpdateExamPayload`, dan `RegistrationExam`. `ExamModal` mengirim `whatsapp_group_url` (`trim() || null`); bila `exam` yang dibuka belum membawa field (backend lama), payload menghilangkan key agar tidak menghapus nilai. Rules disimpan lewat `useUpdateExam` hanya dengan dua key aturan. Teks UI lewat `lib/i18n.ts` (id dan en). Komponen aturan tambahan dan card WhatsApp boleh tinggal di file page/komponen existing; tidak ada paket baru.

**Rollout.** Migrasi aditif lebih dulu (backend), lalu UI. UI toleran terhadap field absen (diperlakukan sebagai tidak tersimpan), jadi urutan deploy tidak kritis; rollback aman karena down hanya menjatuhkan kolom baru.

Urutan rollback: rollback aplikasi lebih dulu, baru jalankan `0071` down; binary baru membaca ketiga kolom pada setiap SELECT exam, sehingga menjatuhkan kolom saat binary baru masih berjalan merusak seluruh query exam.

## Acceptance and proof

1. Migrasi: up lalu down lalu up pada DB nyata (pola `migration_0069_exam_product_stock_test.go`); kolom ada, nullable, baris lama bernilai NULL.
2. Tabel service `validateExam`: host valid/tidak (`chat.whatsapp.com`, `evil.chat.whatsapp.com`, `chat.whatsapp.com.evil.com`, `whatsapp.com`), scheme (`http` untuk WhatsApp ditolak; `javascript:`/`ftp:` ditolak untuk aturan), path kosong, relatif, panjang URL 2048/2049, note 500/501 rune (multibyte), blank dan whitespace menjadi `nil`.
3. Handler: POST absent/blank → null; PATCH absent mempertahankan, `null` dan `""` menghapus, nilai baru menimpa; PATCH field lain (mis. `title`) mempertahankan ketiga nilai; 400 untuk nilai tidak valid.
4. Edit saat `has_sessions` (sesi `in_progress` maupun `submitted`) pada exam format 2 berhasil untuk ketiga field; perubahan policy tetap 409 `exam_has_sessions` (regresi).
5. Projection: admin detail dan list mengembalikan field; `GET /exam/registrations/:id` milik pemilik dan `GET /admin/exams/:id/preview` mengembalikan field; registrasi siswa lain → 404; respons tidak memuat konten soal.
6. Vitest `ExamModal`: field WhatsApp terisi saat edit, blank → `null`, field absen tidak terkirim, URL tidak valid memblokir submit.
7. Vitest card ringkasan: state `Link tersedia` + URL, dan `Tidak dicantumkan` + penjelasan tanpa alasan.
8. Vitest aturan: default hanya checkbox; centang → form; Simpan → item tersimpan dengan Edit; Batal tidak mengubah nilai; off + Simpan mengirim dua `null`; enabled tanpa note/URL ditolak; form mati dengan draft URL invalid tetap bisa Simpan; `has_sessions=true` tidak menonaktifkan; save aturan hanya mengirim dua key.
9. Vitest `ExamBriefing`: blok WhatsApp berada setelah "Informasi ujian" dan sebelum "Cara mengerjakan", hanya bila set, `target="_blank"` dan `rel="noopener noreferrer"`, start/check-in tidak bergantung; aturan tampil di "Cara mengerjakan" untuk note saja, link saja, keduanya, dan kosong; newline terjaga; markup `<b>` pada note tampil sebagai teks; berlaku untuk mode `preview`.
10. Playwright pada 360/390/768/1024/1440 px tanpa overflow horizontal untuk modal, tab Ringkasan, tab Tes & section, dan briefing siswa dengan URL/note panjang tanpa spasi.
11. Gates: `./deploy/pipeline/backend.sh`; frontend `tsc` + vitest di Node 20 (`./deploy/pipeline/frontend.sh`).

## Scope

Non-goals: memverifikasi atau fetch link, tracking bergabung/klik, notifikasi WhatsApp, rich text atau upload dokumen aturan, entry edit WhatsApp kedua, mengubah policy lock, field pada daftar siswa (`RegistrationListItem`), konten soal pada projection apa pun, redesign tab Tes.

Residual risk dan hal yang perlu dikonfirmasi saat build:
- Label prototype tidak sama dengan produksi: `ExamModal` tidak punya heading "Informasi & jadwal akses" (heading existing: Detail Ujian, Format, Akses & Perilaku, Hasil; key `exam_packages_modal_section_*`), dan tab "Tes & section" tidak punya sidebar "Aturan pengerjaan" (kolom kanan adalah picker Test; ringkasan perilaku efektif hanya satu paragraf). Keputusan: field WhatsApp di bagian "Detail Ujian" (tempat jadwal), kartu "Aturan pengerjaan" baru di tab Tes & section. Penempatan ini ditetapkan (2026-11-03): field WhatsApp di "Detail Ujian"; kartu "Aturan pengerjaan" baru di kolom kanan tab Tes & section, di atas picker Test.
- Exam lama yang membuka `ExamModal` dari cache tanpa field baru: ditangani dengan menghilangkan key; tetap ada risiko menimpa bila modal memakai data basi dari tab lain (PATCH last-write-wins, sama seperti field lain).
- Link WhatsApp/aturan dapat kedaluwarsa atau menuju situs berbahaya yang lolos `http(s)`; server tidak memverifikasi, hanya membatasi scheme dan host WhatsApp.
- Dua save terpisah (pengaturan vs aturan) dapat saling mendahului; masing-masing hanya mengirim key miliknya sehingga tidak menimpa satu sama lain, kecuali modal yang selalu mengirim `whatsapp_group_url`.
