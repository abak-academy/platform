# START HERE — Handoff 2: Tes authoring dan revisi Exam

Disiapkan 2026-11-03, Asia/Jakarta. Prototype dilanjutkan sampai revisi 8. **Paket lokal untuk implementer berikutnya; bukan implementasi produksi atau izin memulai build.**

## Baca berurutan

1. [AGENTS.md](../../../AGENTS.md), [.agents/workflows.md](../../../.agents/workflows.md), skill repo [development-workflow](../../../.agents/skills/development-workflow/SKILL.md).
2. [Spec Test authoring flow](../../specs/test-authoring-flow.md): keputusan UI, fakta kode, kontrak metadata proposed, acceptance dan non-goals.
3. [Plan Test authoring flow](../../backlog/test-authoring-flow-plan.md): tugas T0–T6, file/API map, dependency WhatsApp, gates dan seed JSON yang belum diaktifkan.
4. [VALIDATION-2](VALIDATION-2.md), [report fresh](evidence-handoff-2/report.json), [README](README.md), lalu source prototype dan slice produksi pada map plan.
5. [Handoff pertama](HANDOFF.md), [spec Exam pertama](../../specs/exam-user-flow.md), [spec composable](../../specs/composable-exam-format.md), [keputusan runtime](../../wiki/decisions/composable-exam-format.md).

Gunakan skill repo `.agents/skills/`, tanpa Superpowers. Tidak ada agent/worker/chat baru yang didispatch oleh paket ini. Knowledge reflection: belum ada system truth produksi yang berubah; proposal tidak dipromosikan ke wiki.

## Delta terhadap handoff pertama

| Handoff pertama | Kelanjutan yang sekarang berlaku |
| --- | --- |
| Page Tes belum dibuat; selanjutnya eksplorasi Tes | Page Tes telah diprototype dan ditinjau. Gunakan spec/plan kedua untuk implementasi Tes berikutnya. Jangan mengulang fase desain dari nol. |
| Dialog sumber Tes baca saja | Tetap fixture baca saja dalam alur Exam; submenu Tes sekarang membuka editor prototype tersendiri. Keduanya memakai data contoh yang sama, bukan backend. |
| Tidak ada aturan event tambahan | Aturan opsional di sidebar Aturan pengerjaan: checkbox default off → form → Simpan menjadi item aturan → Edit/Batal/off+save. Student item mengikuti font/spacing item lain. |
| Summary WhatsApp hanya status generik | Card Grup WhatsApp kompetisi di kanan Ringkasan menggantikan Yang dialami siswa yang redundant. URL tersedia atau Tidak dicantumkan ditampilkan jelas; edit tetap satu tempat di Pengaturan. |
| Bukti pada `evidence/report.json` | Snapshot pertama tetap disimpan. Bukti untuk source revisi 8 sekarang di `evidence-handoff-2/`; cocokkan hash sebelum mengandalkannya. |

Paket kedua melengkapi yang pertama, tidak mengganti invariants runtime/composable atau memberi izin implementasi kontrak yang masih proposed. Jangan mengimplementasikan WhatsApp dua kali: task T5 plan kedua bergantung persistence/projection yang direncanakan handoff pertama.

## State repository yang diverifikasi

| Item | Snapshot |
| --- | --- |
| Main checkout | `/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform`, branch `feat/composable-exam-format` |
| Worktree paket | `/Users/Panca/Documents/MyBook/Project/akademi-bimbel/platform/.worktrees/exam-flow-prototype`, branch `feat/composable-exam-flow-prototype` |
| HEAD kedua checkout | `0d2b6ca1a6c2985f903d16fbd94e71ee297a4540` |
| Main dirty sebelum pekerjaan ini | `deploy/compose/local.yml`; jangan restore/stage/copy sebagai perubahan paket. |
| Paket prototype/dokumen | Masih untracked, belum commit/push/PR. Branch name saja tidak membawa file ini ke clone/worktree baru. |
| External state | Tidak refresh remote/CI/GitHub issue/PR pada tugas dokumentasi ini. Tidak ada klaim merged/deployed atau issue selesai. |

Parent `akademi-bimbel` bukan Git root. `.agents/repo-map.md` menyebut `app/` dari konteks lama; pada workspace ini root aktual diverifikasi sebagai `platform` dan worktree di atas. Ulangi status/head/worktree sebelum bekerja. Checkpoint scoped file bila user meminta pindah checkout; jangan menghapus atau mereset untracked paket ini.

## Isi paket dan cara mencoba

| File | Peran |
| --- | --- |
| `index.html`, `style.css`, `app.js` | Shell/alur Exam dan optional event rules. |
| `tests.js`, `tests.css` | Daftar/detail Tes, editor simulasi, picker, rich preview. |
| `verify.cjs`, `verify-tests.cjs`, `verify-feedback.cjs`, `verify-handoff.cjs` | Verifier portable untuk seluruh paket; Playwright menggunakan dependency `web/`. |
| `evidence-handoff-2/` | Log, report hash dan screenshot fresh; tidak membutuhkan file `/tmp`. |
| `HANDOFF-2.md`, `VALIDATION-2.md`, README, spec/plan kedua | Entry point, bukti, kontrak dan urutan kerja. |

Dari root worktree:

```sh
python3 -m http.server 8765 --bind 127.0.0.1
```

- [Daftar Tes](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?revision=8#tests-list)
- [Detail Tes](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?revision=8#tests-detail)
- [Aturan tambahan admin](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?revision=8#admin-detail)
- [Ringkasan admin/WhatsApp](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?revision=8#admin-overview)
- [Detail siswa](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?revision=8#student-detail)

Contoh review: dari Tes → Dari bank soal → filter Topik/Format → cari “pemerintah kota” → expand teks panjang. Dari Exam → centang aturan tambahan → isi note/link → Simpan → Pratinjau siswa. Reload/Reset menghapus fixture; URL bukan deployment. Buka route langsung melalui reload/URL query baru; prototype tidak memakai router aplikasi penuh.

## Boundary implementasi

- **Reuse production:** `RichTextEditor`, `RichContent`, QuestionPicker, hooks dan komponen UI existing. Toolbar prototype hanyalah simulasi HTML; media blob bukan upload dan formula teks bukan renderer produksi.
- **Preserve APIs:** Test save berbeda dari attachment mutations. Filter Test `topic` nama; bank `topic_id` ID. Detach join bukan DELETE Question. Null semantics jangan berubah.
- **Preserve runtime:** seluruh format jawaban, legacy, own/shared material, sibling collision, audio limit, durasi snapshot dan policy lock tetap dipertahankan. Fixture tidak menegakkan semua guard itu.
- **Proposed metadata:** aturan tambahan dan WhatsApp belum ada pada API/DB baseline. Finalisasi nama/length/projection/migration/preview di T0. UI accepted bukan bukti backend sudah mendukung.
- **No fixture claims:** “dipakai pada 1 ujian contoh”, tiga Tes, sembilan bank questions, akun/role/clock simulasi bukan data sistem. Usage count butuh sumber nyata atau dihilangkan.
- **Stop boundary:** dokumen ini selesai saat paket lengkap dan verifikasi lulus. Implementasi, task-state, subagent, commit/push/PR/merge/deploy menunggu instruksi berikutnya sesuai skill repo. Tidak membuat handoff thread/worker eksternal.
