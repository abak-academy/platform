---
title: Exam session resume — lanjutkan sesi yang terputus di perangkat yang sama
issue: none; related context 185
status: accepted
created: 2026-11-03
sources:
  - backend/internal/service/exam_session.go
  - backend/internal/handler/exam.go
  - web/app/(student)/exam/page.tsx
  - web/app/(student)/exam/[id]/page.tsx
  - docs/specs/composable-exam-format.md
---

# Exam session resume

## Problem and outcome

Siswa yang terputus di tengah ujian (tab tertutup, Wi-Fi mati, pindah ke data seluler) harus bisa melanjutkan sesi yang masih berjalan sampai timernya habis. Saat ini ada tiga jalan buntu:

1. `StartSession` memeriksa jendela akses (`scheduled_end_at`), check-in, dan device **sebelum** mencari sesi `in_progress`, sehingga resume ditolak `window closed` walau sesi masih berjalan.
2. Halaman detail siswa menyembunyikan tombol mulai untuk `in_progress` pada ujian wajib check-in; card hanya menawarkan "Lihat Detail".
3. Device dikenali sebagai `sha(IP + User-Agent)` dengan TTL Redis = durasi ujian **dihitung dari check-in**. Perubahan IP (penyebab umum terputus) atau check-in lebih awal membuat resume ditolak `device mismatch`.

Outcome: sesi yang sudah berjalan dapat dilanjutkan dari perangkat (browser) yang sama, terlepas dari jendela akses dan perubahan IP; sesi baru tetap mengikuti semua aturan existing.

## Agreed behavior

- **Resume didahulukan.** Bila registrasi punya sesi `in_progress`, `StartSession` mengembalikan sesi itu tanpa memeriksa jendela akses, check-in window, jumlah soal, maupun batas percobaan. Revoked tetap ditolak.
- **Perangkat yang sama (keputusan owner).** Untuk ujian `requires_checkin`, resume dan reconnect (`GET /exam/sessions/:id` pada sesi `in_progress`) tetap mensyaratkan perangkat yang sama dengan saat check-in. Ujian tanpa check-in tidak memiliki pengikatan perangkat (tidak berubah).
- **Identitas perangkat = ID perangkat browser (opsi A).** Browser membuat UUID acak sekali (localStorage) dan mengirimnya di header `X-Exam-Device` pada check-in, start, dan reconnect. Server menyimpannya per registrasi di Redis (`exam:device:<registration_id>`). IP tidak lagi menjadi bagian identitas.
- **Kompatibilitas rollout.** Bila header tidak ada (klien lama), server memakai fingerprint lama `IP+UA`. Bila nilai tersimpan adalah fingerprint lama (check-in sebelum deploy) dan cocok dengan fingerprint request, server menerimanya dan menulis ulang kunci ke ID perangkat baru.
- **Masa berlaku kunci.** TTL kunci perangkat 24 jam sejak check-in, dan diperbarui menjadi 24 jam saat sesi dimulai atau dilanjutkan, sehingga tidak habis di tengah sesi.
- **UI siswa.** Card `in_progress` menampilkan CTA "Lanjutkan ujian" yang melanjutkan sesi langsung; halaman detail menampilkan gate "Lanjutkan ujian" untuk `in_progress` termasuk ujian wajib check-in. Penolakan server (device mismatch, revoked, dll.) tampil sebagai pesan error existing.

## Technical contract

- Handler membaca `X-Exam-Device` (harus UUID; selain itu diabaikan) dan meneruskan `{deviceID, legacyFingerprint}` ke service. Tidak ada perubahan CORS: Echo memantulkan `Access-Control-Request-Headers` saat `AllowHeaders` kosong.
- Service tetap authoritative; tidak ada perubahan skema DB atau migrasi.
- Tidak ada perubahan format payload sesi selain penerimaan header baru.

## Acceptance and proof

1. Sesi `in_progress` dapat di-resume via `POST /exam/sessions` setelah `scheduled_end_at` lewat (real-DB service test).
2. Resume ujian check-in dengan ID perangkat sama → OK; ID berbeda → `device mismatch`; ujian tanpa check-in → OK tanpa header.
3. Perubahan IP dengan ID perangkat sama → OK (handler/service test).
4. Kunci lama berisi fingerprint `IP+UA` diterima sekali lalu dimigrasikan.
5. TTL diperbarui saat start/resume.
6. Reconnect `GET` ujian check-in menolak perangkat berbeda; ujian tanpa check-in tidak terpengaruh.
7. Sesi baru tetap ditolak di luar jendela akses, tanpa check-in, dan saat percobaan habis (regresi).
8. UI: card dan detail menampilkan "Lanjutkan ujian" untuk `in_progress` (termasuk check-in); semua request check-in/start/reconnect mengirim `X-Exam-Device` (vitest).
9. Gates: `./deploy/pipeline/backend.sh`, `./deploy/pipeline/frontend.sh`.

## Non-goals

Pemeriksaan perangkat pada setiap autosave/advance/submit; auto-submit saat jendela tutup; perubahan interval worker expiry; penegakan batas putar audio; perubahan aturan check-in untuk sesi baru.

## Rollout

Sebelum mengalihkan traffic, perpanjang kunci perangkat yang ada menjadi 24 jam (`SCAN` pada `exam:device:*`, lalu `EXPIRE <key> 86400`), dan deploy di luar jendela aktif ujian yang mewajibkan check-in. Alasan: kunci yang ditulis kode lama kedaluwarsa setelah durasi ujian dihitung dari check-in, sedangkan reconnect kini menegakkan perangkat untuk sesi check-in `in_progress`. Klien dengan bundle lama tanpa header memakai fallback IP+UA sampai halaman dimuat ulang.

## Residual risk

ID perangkat di localStorage dapat disalin manual ke perangkat lain (setara dengan JWT yang juga di localStorage). Menghapus data browser di tengah ujian check-in membuat resume ditolak; pemulihan memerlukan bantuan admin (alur pemulihan perangkat di luar scope).

Selama masa transisi (kunci lama, maksimal 24 jam), perangkat kedua milik siswa yang sama dengan IP+UA identik dapat mengklaim kunci format lama dan mengunci perangkat pertama; cakupannya tetap terbatas pada registrasi siswa itu sendiri.
