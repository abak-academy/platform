---
title: Composable exam format — section presentation dan shared material
issue: 185
status: accepted
created: 2026-09-30
sources:
  - docs/backlog/exam-format-system-design.md
  - docs/prototypes/exam-format-user-flow-v2.html
  - backend/internal/service/exam.go
  - backend/internal/service/exam_session.go
  - backend/db/migrations/0021_exam_modes.up.sql
---

# Composable exam format

## Problem and outcome

Perilaku ujian ditentukan oleh `exam.mode` (`standard|utbk|ielts`). Dampak terukur: 18 cabang mode non-test di backend (26 termasuk test), 35 referensi `mode` di 12 file frontend, `isSectionedMode` dipanggil di tiga jalur runtime. Akibatnya admin tidak dapat mengonfigurasi bagaimana soal ditampilkan: hari ini setiap section selalu satu soal per layar, dan tidak ada cara memberi konteks bersama (bacaan, tabel, prompt) untuk seluruh soal dalam satu section. Klien meminta IELTS Listening tampil utuh dalam satu halaman.

Outcome yang diinginkan: admin menyusun satu Exam dari beberapa Test, setiap Test yang terlampir menjadi satu Section siswa, dan setiap Section menentukan sendiri `one_per_page` atau `all_on_page` serta material bersamanya. Runtime exam format baru tidak membaca `exam.mode`.

Pengguna yang terpengaruh: admin (authoring Exam/Test) dan siswa (runtime attempt).

## Agreed behavior

**Presentation**

| Aspek | `one_per_page` | `all_on_page` |
|---|---|---|
| Soal | satu Question per layar, navigator dan prev/next seperti sekarang | seluruh ordered Question dalam satu layar scroll |
| Shared material | dirender selama Question aktif tidak punya material sendiri | dirender satu kali sebagai konteks section |
| Material Question | menggantikan shared material hanya pada Question tersebut | tidak dirender |
| Body Question | selalu dirender | selalu dirender |

Material bukan body. Sebuah Question dianggap punya material sendiri bila:

```text
has_question_material =
  material_html is not empty
  OR image_url is not empty
  OR audio_url is not empty
```

Publish untuk `all_on_page` ditolak bila ada Question dengan material sendiri, dengan pesan yang menyebut Question tersebut. Admin memilih: pindahkan material menjadi shared material Test, hapus bila redundan, atau ubah presentation menjadi `one_per_page`. Konten tidak boleh hilang diam-diam. Runtime tetap defensif: bila payload `all_on_page` masih membawa material Question (data lama/invalid), renderer hanya memakai shared material dan mencatat telemetry; body dan kontrol jawaban tetap tampil.

**Randomisasi**

Randomisasi independen dari presentation. Urutan soal untuk satu attempt deterministik dari `(session_id, test_id)`, sehingga refresh/reconnect tidak mengocok ulang. `all_on_page` tidak mematikan randomisasi; shared material tidak ikut diacak.

**Section finish manual**

Siswa boleh menyelesaikan section walau masih ada soal belum dijawab, dengan konfirmasi yang menyebut jumlah soal belum dijawab (perilaku untuk jalur legacy tidak berubah).

**Compatibility**

Semua baris `exam` existing tetap `format_version=1` dan tetap memakai jalur runtime lama, tanpa backfill dan tanpa perubahan perilaku. `exam.mode` tetap ditulis dan dibaca untuk exam legacy sampai PR-3. Perpindahan create path ke `format_version=2` terjadi di PR-2 bersama UI section config: selama PR-1, exam baru tetap `format_version=1` supaya tidak ada exam baru yang berjalan dengan policy default tanpa cara bagi admin untuk mengaturnya.

**Edit safety**

Draft atau exam tanpa session: policy dan section config boleh diubah; bila sudah ada registration (tanpa session), perubahan dicatat di audit log. Begitu exam punya session (aktif atau submitted), perubahan policy (`navigation_policy`) dan section config (daftar Test terlampir beserta `question_presentation`) ditolak. Aturan ini berlaku untuk exam `format_version=2`; exam legacy tetap berperilaku seperti sekarang supaya tidak ada regresi admin. Karena itu snapshot policy di `exam_session`/`exam_session_section` tidak ditambahkan.

## Technical contract

**Kolom baru PR-1: 6 kolom, 0 tabel baru**

| Tabel | Kolom | Constraint |
|---|---|---|
| `exam` | `format_version INT NOT NULL DEFAULT 1` | penanda jalur runtime |
| `exam` | `navigation_policy TEXT NOT NULL DEFAULT 'free'` | CHECK (`free`, `sequential`) |
| `exam_test` | `question_presentation TEXT NOT NULL DEFAULT 'one_per_page'` | CHECK (`one_per_page`, `all_on_page`) |
| `test` | `shared_material_html TEXT NULL` | sanitizer yang sama dengan body soal |
| `test` | `instructions_html TEXT NULL` | sanitizer yang sama dengan body soal |
| `question` | `material_html TEXT NULL` | sanitizer yang sama dengan body soal |

Tidak ada kolom baru di `exam_session` maupun `exam_session_section`.

**Ownership**

- `test`: `title` (menjadi label section), `duration_minutes` (satu-satunya sumber durasi section), `shared_material_html`, `instructions_html`, daftar Question.
- `exam_test`: `question_presentation` dan `sort_order`.
- `exam`: `format_version`, `navigation_policy`; timer tetap memakai field existing `timer_mode` ∈ {`overall`,`per_test`} + `duration_minutes` (`overall` + duration = timer per exam, `per_test` + duration = timer per section, `per_test` tanpa duration = tanpa timer).

**Invariant**

- `format_version=1` → jalur runtime lama tidak tersentuh.
- `format_version=2` → runtime tidak membaca `exam.mode`, dan `exam.mode` tidak lagi menentukan timer, navigation, atau presentasi.
- Backend authoritative: section aktif, sisa waktu, izin menulis jawaban, izin transition, dan izin submit ditentukan backend.
- Timer section tetap dibaca dari snapshot `exam_session_section.duration_minutes` (existing), bukan dari `test.duration_minutes` live.
- `shared_material_html` dan `material_html` memakai allowlist dan sanitizer yang sama dengan `question.body` (bluemonday, `questionBodyAllowedTags`), termasuk render/lockstep test frontend yang sudah ada (`web/lib/question-html.ts`).
- Publish validation untuk `format_version=2`:
  - `navigation_policy=sequential` memerlukan minimal satu Test terlampir;
  - `timer_mode=overall` memerlukan `exam.duration_minutes` positif;
  - `timer_mode=per_test` memerlukan setiap Test terlampir punya `duration_minutes` positif;
  - `question_presentation=all_on_page` menolak setiap Question dengan `has_question_material`;
  - Test terlampir harus punya minimal satu Question;
  - `shared_material_html`/`instructions_html`/`material_html` harus lolos sanitization;
  - kombinasi `navigation_policy=free` + `timer_mode=per_test` ditolak (semantics belum jelas).
- Exam dengan `format_version=2` tanpa Test terlampir tidak dapat dipublish.

**Yang secara sadar tidak diadopsi dari design doc**

- `exam_test.duration_minutes`: durasi tetap milik Test, satu sumber, tidak dipindah ke attachment.
- `timer_scope`: duplikat 1:1 dari `timer_mode` yang sudah ada.
- `section_transition_policy`: perilaku hari ini sudah `manual_or_timeout`; `timeout_only` belum ada kebutuhan.
- `result_grouping`: hanya relevan saat result/monitor/export dibongkar (PR-3).
- Snapshot policy per attempt: redundan selama blokir edit di atas ditegakkan.
- `section_label` di `exam_test`: label memakai `Test.title`.

## Acceptance and proof

Kriteria yang bisa diperiksa:

1. Admin dapat menempelkan Test ke Exam sebagai Section dan memilih `one_per_page` atau `all_on_page` (kontrak API di PR-1, UI di PR-2).
2. Runtime exam `format_version=2` tidak membaca `exam.mode`; jalur legacy tetap membacanya.
3. Shared material mendukung paragraf/rich text, tabel, dan gambar, dan divalidasi sanitizer yang sama dengan body soal.
4. Material Question terbedakan secara mesin dari body Question (`has_question_material`).
5. `one_per_page` memakai material Question lebih dulu, shared material sebagai fallback, dan body selalu dirender.
6. `all_on_page` merender shared material saja, dan publish ditolak bila ada Question bermaterial.
7. Randomisasi stabil selama satu attempt dan independen dari presentation.
8. Autosave, grading, result, dan transition tidak regresi pada exam legacy maupun exam baru.
9. Exam legacy tetap berperilaku identik tanpa backfill.
10. Perubahan dari issue: audio (field + policy + play-limit) dikeluarkan dari scope → kriteria shared material dan audio policy dipisahkan; kriteria "runtime tidak lagi bergantung `exam.mode`" berlaku untuk exam format baru, konversi exam lama masuk PR-3.

Bukti yang dijalankan sebelum PR-1 disebut selesai:

```bash
# gate repo (CI memakai skrip yang sama)
./deploy/pipeline/backend.sh
# loop fokus
cd backend && go build ./... && go vet ./... && go test -race -timeout 20m ./internal/service/... ./internal/repository/...
# hygiene
git status --short && git diff --check
```

Test yang harus ada di PR-1: migration up (+ down bila repo memakai pasangan), contract table-driven untuk setiap kombinasi publish yang diterima/ditolak, validasi material (`has_question_material`), determinisme urutan randomisasi, dan regression test bahwa exam `format_version=1` tetap memakai jalur legacy.

## Scope

**Ditunda ke PR-2:** student renderer `one_per_page`/`all_on_page`, resolusi material di UI, section transition + konfirmasi jumlah soal belum dijawab, admin section config + editor shared material, effective behavior summary.

**Ditunda ke PR-3:** konversi exam lama (preview mapping `mode → format_version=2`), result/monitor/export memakai label section dan `result_grouping`, retirement `isSectionedMode` dan kolom legacy.

**Non-goals:** audio section (field, policy, play-limit enforcement), Page/Question Group, named preset, konversi scoring/band resmi, perubahan minimum opsi MCQ, render budget `all_on_page`, Test revision/immutability, snapshot policy per attempt.

**Residual risk:**

- Konten Test (`Test.title`, daftar soal, `shared_material_html`) dibaca live; mengubahnya saat exam berjalan tetap mengubah attempt yang sedang jalan. Penutupnya adalah Test revision/immutability yang ditunda. Timer section sendiri sudah aman karena disnapshot.
- Blokir edit adalah pengecekan sebelum penulisan, bukan lock: sesi yang mulai tepat bersamaan dengan penulisan admin masih bisa memakai policy yang baru. Penutupnya butuh row lock saat session start (atau snapshot yang sengaja tidak dipakai di desain ini); perilakunya tetap lebih ketat daripada sebelumnya, ketika perubahan berlaku ke semua attempt yang berjalan.
- Label section mengikuti `Test.title`, sehingga mengganti title Test juga mengganti label di hasil/monitor.
- Kombinasi `free` + timer per section tidak didukung; admin harus memilih `sequential`.
