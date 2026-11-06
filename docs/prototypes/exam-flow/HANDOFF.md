# START HERE — Exam user flow handoff

> Snapshot handoff pertama. Kelanjutan page Tes dan revisi Exam terbaru ada di [HANDOFF-2](HANDOFF-2.md); status “Tes belum dibuat” di bawah hanya berlaku pada snapshot ini.

Disiapkan 2026-11-02 (Asia/Jakarta). Ini handoff prototype + dokumentasi, **bukan implementasi produksi**.

## Baca berurutan

1. Root [AGENTS.md](../../../AGENTS.md), [.agents/workflows.md](../../../.agents/workflows.md), dan skill repo [development-workflow](../../../.agents/skills/development-workflow/SKILL.md).
2. [Spec Exam user flow](../../specs/exam-user-flow.md): keputusan UI yang telah ditinjau, fakta kode, kontrak tambahan yang masih usulan, acceptance, dan batas scope.
3. [Plan bertahap](../../backlog/exam-user-flow-plan.md): jalur A untuk page Tes berikutnya dan jalur B untuk implementasi setelah persetujuan. Seed JSON mengikuti schema task graph repo; belum diaktifkan.
4. [README prototype](README.md), [VALIDATION.md](VALIDATION.md), [laporan browser](evidence/report.json), lalu prototype dan source aplikasi yang dipetakan dalam plan.
5. [Spec composable existing](../../specs/composable-exam-format.md) dan [keputusan runtime](../../wiki/decisions/composable-exam-format.md). Jangan memakai fixture untuk menggantikan kontrak runtime tersebut.

Tidak menggunakan Superpowers untuk handoff/planning ini. Ikuti skill `.agents/skills/`; user secara eksplisit meminta pola repo. Tidak perlu dispatch/handoff ke agent/chat lain untuk membaca paket ini.

## State yang harus dipertahankan

| Item | Snapshot saat handoff |
| --- | --- |
| Git root sumber | `/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform` — parent `akademi-bimbel` bukan checkout. |
| Main checkout | Branch `feat/composable-exam-format`, HEAD `0d2b6ca1a6c2985f903d16fbd94e71ee297a4540`. |
| Worktree prototype | `/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform/.worktrees/exam-flow-prototype` |
| Branch prototype | `feat/composable-exam-flow-prototype`, dibuat dari `feat/composable-exam-format`, HEAD masih commit basis yang sama. |
| Perubahan main yang sudah ada | `deploy/compose/local.yml` modified sebelum pekerjaan ini. Jangan restore, stage, copy, atau commit perubahan itu sebagai bagian prototype. |
| Delivery | Prototype dan dokumen paket ini adalah file lokal belum dikomit/push saat ditulis. Tidak ada PR/deploy dari pekerjaan ini. Branch name saja tidak membawa file untracked ke clone/worktree baru. |
| Remote | Local `origin/feat/composable-exam-format` sama pada pemeriksaan; remote/CI/PR live tidak direfresh pada tugas dokumentasi. |

Sebelum melanjutkan, ulangi `git status --short`, `git branch --show-current`, `git log -1`, dan `git worktree list` memakai path eksplisit. Jangan membuat checkout baru dari branch kosong lalu menganggap prototype ikut terbawa. Jangan membersihkan untracked/worktree ini tanpa checkpoint/izin user.

## Apa yang sudah disepakati

- Scope desain awal: admin dan siswa; perbaiki layout/field/kolom, pertahankan warna/referensi existing.
- Admin list/table/header/search mengikuti standar existing. Status/filter status Exam dihapus karena dapat menyesatkan terhadap produk. Satu Pengaturan dan satu Pratinjau di header.
- Ringkasan admin tidak mengulang judul/policy. Field yang tidak relevan tidak memenuhi ringkasan, tetapi tetap dipertahankan oleh form/API.
- Percobaan memakai checkbox utama, jumlah N>=2, dan checkbox Tanpa batas terpisah; off=1.
- Card siswa compact; tanggal kiri, jam kanan; kalender besar 32 px. Upcoming memakai “Belum dibuka” dan countdown; detail tetap dapat dibaca tanpa start.
- Informasi ujian siswa memakai panel kalender/jadwal dan jam/durasi. Card Cara mengerjakan dipertahankan.
- Link grup WhatsApp opsional dari admin. Invitation di bawah Informasi ujian, sebelum Cara mengerjakan. Join bukan syarat memulai.
- Ownership composable existing: Test memiliki durasi/materi/instruksi; attachment memiliki urutan/presentasi; Exam memiliki navigation/timer. Legacy dan guard setelah ada sesi dipertahankan.

Rincian final ada pada spec; jangan mengembalikan revisi lama seperti badge Draft, card siswa besar, input attempt kosong=unlimited tanpa checkbox, atau tombol edit informasi berulang.

## Langkah berikutnya yang diminta user

**Berpindah ke page Tes untuk desain/prototype, sesudah handoff ini.** Baca file pada jalur A plan dan minta arahan terhadap existing page sebelum mengubah perilaku yang belum dibahas. Page Tes baru belum dibuat dan belum disetujui; modal “Lihat sumber Test” pada prototype hanyalah fixture baca saja.

Jangan otomatis memulai jalur implementasi produksi. Proposal backend/API di spec belum menjadi keputusan final: metadata pre-start, flag lock admin, read-only preview admin, WhatsApp persistence, serta state dekat check-in/resume perlu diselesaikan terlebih dahulu.

## Menjalankan preview

Dari root worktree prototype, di terminal terpisah:

```sh
python3 -m http.server 8765 --bind 127.0.0.1
```

- [Daftar admin](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html)
- [Ringkasan admin](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#admin-overview)
- [Susunan section](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#admin-detail)
- [Card siswa mendatang](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?access=upcoming#student-list)
- [Detail siswa mendatang](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?access=upcoming#student-detail)

URL localhost bergantung server tersebut; bukan deployment. Reload/Reset menghapus semua perubahan fixture. Data dan jam simulasi bukan akun, session, atau clock produksi. Jika port 8765 terpakai, periksa proses/isi yang disajikan; jangan mematikan layanan lain tanpa memastikan ownership. Pakai port lain dan override URL verifier bila perlu.

## Paket yang dapat dibawa

| File | Peran |
| --- | --- |
| `index.html`, `style.css`, `app.js` | Prototype interaktif. Tidak untuk di-copy menjadi stack/frontend produksi baru. |
| `README.md` | Cara mencoba dan referensi keputusan/pola existing. |
| `verify.cjs` | Browser checks portable, memakai Playwright dari dependency `web/`; tidak bergantung `/tmp` scripts. |
| `evidence/report.json`, PNG | Bukti fresh prototype pada saat handoff; bukan hasil test API/DB. |
| `VALIDATION.md` | Proven/not proven, perintah ulang, dan verifikasi paket dokumen. |
| `docs/specs/exam-user-flow.md` | Spec dengan lima bagian sesuai skill repo. |
| `docs/backlog/exam-user-flow-plan.md` | Durable plan + task graph seed; execution belum dimulai. |

Saat membawa ke checkout lain, sertakan folder prototype dan dua dokumen tersebut, serta pertahankan relative asset paths ke `web/public/fonts` dan `web/public/brand/logo.svg`. Bukti PNG tidak memerlukan asset/font untuk dilihat. Tidak ada token/invite grup nyata yang dibutuhkan atau ditambahkan.

## Pemeriksaan sebelum mengklaim selesai

- Untuk desain berikutnya: browser dan visual check pada artifact yang benar, bukan source JSX atau file lama.
- Untuk implementasi: ikuti TDD repo, standards Go/Web, dan gates `.agents/verification.md`. Pakai real read/write/authorization tests untuk kontrak baru.
- Jangan menyebut mock/browser-only evidence sebagai produksi benar, menganggap popup WhatsApp sudah dihubungkan ke grup, atau menganggap Test source aman immutable karena policy Exam terkunci.
- Dokumen ini tidak menyatakan issue #185 selesai/merged/deployed. Re-read issue dan comments live sebelum PR terkait; jangan menyalin status map lama sebagai fakta terbaru.
- No automatic commit/push/PR/merge/deploy; authorization user harus sesuai scope kerja berikutnya. Tidak memberi attribution agent pada metadata Git/PR.
