# Exam user flow — handoff validation

Tanggal paket: 2026-11-02, Asia/Jakarta. Sumber aplikasi yang dibaca adalah commit `0d2b6ca1a6c2985f903d16fbd94e71ee297a4540`. Prototype merupakan perubahan lokal di atas commit tersebut, bukan aplikasi yang telah dirilis.

## Bukti terkini

Canonical browser evidence: [evidence/report.json](evidence/report.json). Report memuat timestamp UTC, enam kelompok checks, tujuh lebar viewport, error browser, external requests, daftar screenshot, dan SHA-256 empat input (`index.html`, `style.css`, `app.js`, `verify.cjs`). Cocokkan hash dengan file terkini sebelum memakai hasil ini sebagai proof.

| Scope | Hasil |
| --- | --- |
| Admin daftar | Header/table pattern, lima kolom, tanpa badge/filter status, search/clear/empty, dan legacy notice lulus. |
| Admin ringkasan/form | Satu settings/preview entry, tanpa duplicate fields, mapping attempt 1/N/null, dan skenario policy lock lulus. |
| Komposisi section | Konflik materi, fix presentasi, tambah/urut/hapus, save barrier, serta kombinasi policy invalid lulus. |
| WhatsApp | Link opsional, invalid-domain rejection, posisi blok, target tab baru, dan start tanpa klik join lulus. Tidak membuka grup. |
| Student preflight/runtime contoh | Upcoming/countdown, blokir start, konfirmasi, dua presentasi MCQ, own/shared material, unanswered count, dan submit/hasil hidden lulus. |
| Responsive | Admin list/overview dan student detail pada 1713, 1440, 1200, 1024, 768, 390, dan 360 px lulus tanpa overflow dokumen; teks panel informasi tidak terpotong. |
| Browser errors / request keluar | Tidak ada error JS maupun request ke origin di luar preview lokal pada run yang tercatat. |

Screenshots dari run yang sama:

- [Daftar admin desktop](evidence/admin-list-desktop.png)
- [Ringkasan admin desktop](evidence/admin-overview-desktop.png)
- [Invitation WhatsApp contoh](evidence/student-whatsapp-desktop.png)
- [Card siswa mendatang](evidence/student-upcoming-card.png)
- [Informasi ujian siswa desktop](evidence/student-information-desktop.png)
- [Detail siswa mobile](evidence/student-detail-mobile.png)

Screenshot adalah data contoh. Counter “2 ujian”, “12 peserta”, session-lock scenario, jawaban, token `DEMO`, fixed date, dan invite fixture bukan data produksi. Report tidak menyatakan semua skenario produksi telah dicakup. Pemeriksaan sebelumnya di `/tmp` bukan dependency paket; entry point pemeriksaan sekarang adalah `verify.cjs` yang ikut handoff.

## Menjalankan ulang

Prasyarat: dependency Playwright dari `web/` tersedia dan browser Chromium yang sesuai terpasang; tidak memasang dependency/browser otomatis pada handoff ini. Server harus menyajikan **root checkout**, agar relative logo/font dapat dibaca.

Terminal preview, dari root worktree:

```sh
python3 -m http.server 8765 --bind 127.0.0.1
```

Terminal pemeriksaan, dari root checkout yang memiliki `web/node_modules`:

```sh
node --check docs/prototypes/exam-flow/app.js
node --check docs/prototypes/exam-flow/verify.cjs
node docs/prototypes/exam-flow/verify.cjs
```

Default output browser adalah `/private/tmp/exam-flow-evidence`, bukan menimpa evidence yang direview. Override untuk meregenerasi package evidence dengan sengaja:

```sh
EXAM_FLOW_EVIDENCE_DIR="$PWD/docs/prototypes/exam-flow/evidence" node docs/prototypes/exam-flow/verify.cjs
```

Jika worktree tidak memiliki dependency, pakai `EXAM_FLOW_WEB_ROOT` ke folder `web` checkout yang dependency-nya tersedia. Pada run handoff ini dependency dibaca dari checkout sumber; tidak ada install/symlink baru. Contoh invocation yang dipakai dari worktree ini:

```sh
EXAM_FLOW_WEB_ROOT=/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform/web \
EXAM_FLOW_EVIDENCE_DIR="$PWD/docs/prototypes/exam-flow/evidence" \
node docs/prototypes/exam-flow/verify.cjs
```

`EXAM_FLOW_URL` dapat mengganti host/port/path preview jika 8765 tidak dipakai. Targetkan prototype lokal sendiri; verifier mengharuskan seluruh request tetap pada origin preview dan tidak dimaksudkan untuk dijalankan terhadap aplikasi/data nyata.

## Pemeriksaan paket dokumentasi

Paket diperiksa untuk:

1. Seluruh tautan file lokal pada spec, plan, handoff, validation, dan README benar-benar ada, termasuk path Next.js yang mengandung tanda kurung dan `[id]`.
2. Spec memiliki frontmatter serta lima bagian sesuai template repo: Problem and outcome, Agreed behavior, Technical contract, Acceptance and proof, Scope.
3. Seed JSON plan memakai schema `task-graph.md`: enam field top-level, lima field per task, `stage=align`, `execution=null`, seluruh task pending, dependencies valid dan tanpa cycle.
4. Tidak ada active task state/dispatch yang diklaim dibuat; seed masih template. Tidak ada folder planning Superpowers yang ditambahkan.
5. Syntax `app.js` dan `verify.cjs`, JSON report, PNG evidence, source hashes, dan whitespace dokumen/source baru valid.
6. `git diff --exit-code HEAD -- backend web deploy loadtest` pada worktree exit 0. Semua perubahan handoff berada di docs; tracked production files tidak berubah.

File untracked tidak tercakup oleh `git diff --check` biasa. Pemeriksaan whitespace juga dilakukan terhadap masing-masing file teks baru dengan `git diff --no-index --check /dev/null <file>` dan memeriksa diagnostics. Source tree tetap **tidak clean** karena artifact belum dikomit; ini tidak disamarkan sebagai clean checkout.

## Yang tidak dibuktikan / tidak dijalankan

- Backend build/vet/test/real DB, migration, API create/PATCH/GET nyata, RBAC/ownership production, dan data persistence.
- Typecheck/vitest/build Next.js atau browser terhadap aplikasi Next.js/Go. Prototype HTML tidak mengimpor React components; kemiripan visual tidak membuktikan reuse komponen produksi.
- Kesetaraan theme/font di shell aplikasi nyata. `admin-theme.css` merupakan acuan produksi; jangan menimpa theme untuk memaksakan stylesheet standalone.
- Countdown real time, autosave jaringan, deterministic shuffle backend, timeout worker, reconnect, kompensasi Monitoring, semua answer formats, official scoring, retake matrix lengkap, atau semua legacy/role branch. Timer prototype statis; randomisasi contoh bukan algoritme backend.
- Preview admin aman pada aplikasi nyata. Prototype hanya berpindah fixture view, belum memiliki kontrak preview berizin dan read-only.
- Keberadaan atau keaktifan grup WhatsApp, join/pengiriman pesan, serta link real. Link baru belum ada di DB/API produksi.
- Remote HEAD/CI/PR/issue comments live. Entri lokal #185 lama tidak direvalidasi atau ditandai selesai.
- Desain page Tes baru; source Test modal adalah fixture baca saja.
- Commit, push, PR, merge, deploy, load test, atau perubahan environment bersama.

## Knowledge reflection

Klasifikasi sistem produksi: **none** — tugas ini menghasilkan prototype dan dokumen usulan; tidak mengubah system truth runtime. Karena itu tidak menulis keputusan implementasi ke wiki dan tidak memperbarui issue map seolah fitur telah dirilis. Spec/plan/handoff menyimpan keputusan UI dan batas usulan untuk pekerjaan berikutnya.
