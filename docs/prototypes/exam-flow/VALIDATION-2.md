# Handoff 2 — validation

Tanggal paket 2026-11-03, Asia/Jakarta. Baseline aplikasi `0d2b6ca1a6c2985f903d16fbd94e71ee297a4540`. Validasi ini untuk **HTML prototype dan integritas paket dokumen**, bukan hasil build/test produksi.

## Bukti source revisi 8

[Report](evidence-handoff-2/report.json) memuat timestamp UTC, SHA-256 sembilan input prototype/verifier, hasil tiga suite dan exit code. Timestamp capture report adalah awal run; hasil disimpan setelah suite selesai. Source berubah → cocokkan hash dan jalankan ulang sebelum mengklaim hasil tetap berlaku.

| Suite | Hasil fresh dan batas |
| --- | --- |
| [Exam log](evidence-handoff-2/exam.log) | PASS: header/search/table, attempts, lock scenario, susunan section, WhatsApp, upcoming/preflight/runtime MCQ dan tujuh lebar 1713/1440/1200/1024/768/390/360. Tidak memanggil API nyata. |
| [Tes log](evidence-handoff-2/tests.log) | PASS: list/search/filter, create kosong, tab/save, Listening validation, reorder/attach/detach/reattach, preview satu/semua, bank filter intersection, retained selection, teks panjang expand/collapse, lima lebar 1440/1024/768/390/360. |
| [Feedback log](evidence-handoff-2/feedback.log) | PASS: checkbox default hidden, empty/URL rejection, save menjadi item, edit/cancel, off+save, student visibility, card WhatsApp tanpa policy card redundant, satu Pengaturan, URL WhatsApp, aksi sejajar, gambar/tabel lokal round-trip, rich preview, responsive. Student extra-rule font/spacing dibandingkan dengan item tetangga pada 1713/390; checkbox geometry pada empat lebar. |

Screenshot representatif dari run yang sama:

- [Daftar Tes desktop](evidence-handoff-2/tests/tests-list-desktop.png)
- [Detail Tes desktop](evidence-handoff-2/tests/tests-detail-desktop.png)
- [Question picker desktop](evidence-handoff-2/tests/tests-bank-desktop.png), [mobile](evidence-handoff-2/tests/tests-bank-mobile.png)
- [Detail Tes mobile](evidence-handoff-2/tests/tests-detail-mobile.png)
- [Aturan tersimpan admin](evidence-handoff-2/feedback/admin-extra-rules.png), [aturan siswa](evidence-handoff-2/feedback/student-extra-rules.png)
- [Checkbox mobile](evidence-handoff-2/feedback/rules-checkbox-360.png)
- [WhatsApp summary](evidence-handoff-2/feedback/admin-communication.png)
- [Editor media/table](evidence-handoff-2/feedback/test-editor.png), [rich sidebar preview](evidence-handoff-2/feedback/test-material-preview.png)

Semua konten screenshot adalah data contoh. Gambar logo dalam editor berasal dari file lokal sebagai fixture uji upload; bukan konten materi default atau bukti object storage. URL aturan `example.com` dan invite WhatsApp fixture tidak dibuka. Test/feedback suite memeriksa page errors; hanya suite Exam yang secara eksplisit mengassert tidak ada request keluar origin.

## Menjalankan ulang

Prasyarat: server menyajikan root checkout, Playwright dan Chromium tersedia dari dependency `web/`. Verifier tidak memasang package/browser atau membuka grup/link external.

```sh
EXAM_FLOW_WEB_ROOT=/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform/web \
  node docs/prototypes/exam-flow/verify-handoff.cjs
```

`EXAM_FLOW_WEB_ROOT` opsional jika `web/node_modules` tersedia di checkout. `EXAM_FLOW_URL` mengubah base URL tanpa query/hash jika memakai port lain. `EXAM_FLOW_EVIDENCE_DIR` opsional untuk lokasi output lain; default folder `evidence-handoff-2/` di paket. Wrapper menjalankan tiga suite berurutan, menyimpan log, screenshot dan report hash. Masing-masing suite juga bisa dijalankan langsung dengan environment yang sama.

Verifikasi source syntax:

```sh
node --check docs/prototypes/exam-flow/app.js
node --check docs/prototypes/exam-flow/tests.js
node --check docs/prototypes/exam-flow/verify-handoff.cjs
```

## Integritas dokumentasi

- Spec menggunakan lima bagian H2 sesuai skill repo; status proposed membedakan UI yang ditinjau dari kontrak produksi yang belum dimulai.
- Plan memuat map file/API yang dibaca, acceptance, dependency lintas handoff, boundary, gates, dan seed JSON dengan schema repo. Semua status pending, execution null; belum ada `.agents/work/` aktif untuk paket kedua.
- Local relative Markdown links dan path file map diperiksa; source hash report cocok dengan sembilan file saat paket ditulis. Handoff pertama ditandai snapshot agar instruksi lama tidak dibaca sebagai status terbaru.
- Git/whitespace diperiksa tanpa stage/reset main yang sudah dirty. Seluruh paket masih lokal/untracked; tidak mengklaim clean checkout.

## Yang tidak dibuktikan

Tidak menjalankan Next typecheck/Vitest/build, Go CI gate, API/auth/role enforcement, DB/migration/nullable persistence, real storage upload, rich editor TipTap round-trip, semua answer format, timer/audio limits/scoring/reconnect atau production concurrency. Proposal metadata Exam belum diimplementasikan. Real production proof mengikuti plan dan `.agents/verification.md` saat build diminta.

Bukti lama [evidence/report.json](evidence/report.json) adalah snapshot handoff pertama. Jangan memakai hash/report tersebut untuk source setelah revisi Tes dan aturan tambahan.
