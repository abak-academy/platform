---
title: Program Affiliator (affiliate klasik) — Fase 1
issue: none (BRD internal)
status: accepted
created: 2026-10-25
updated: 2026-10-30
sources:
  - docs/specs/affiliate-program-brd.md (v0.4)
  - docs/standards/go-patterns.md
  - docs/standards/go-style-and-tests.md
  - backend/internal/service/store.go (outbox OrderPaid, refund, complete)
  - backend/internal/worker/outbox.go (loop worker)
  - backend/internal/service/announcement.go (grup penerima broadcast)
  - backend/internal/service/rbac.go (role & capability)
---

# Program Affiliator (affiliate klasik) — Fase 1

## Problem dan outcome

Akuisisi siswa lewat rekomendasi mulut-ke-mulut tidak terukur dan tidak bisa diberi imbalan. Outcome Fase 1: individu yang disetujui (affiliator) membawa calon siswa lewat link/kode referral, siswa terikat 3 bulan (dapat diperpanjang), setiap pembelian lunas menghasilkan komisi dengan masa tunggu refund, komisi dan semua mutasi saldo masuk satu buku transaksi yang dapat direkonsiliasi, dicairkan manual oleh admin, dan level (Bronze/Silver/Gold) dievaluasi kuartalan. Semua angka aturan diatur super_admin lewat halaman pengaturan.

Aturan bisnis lengkap ada di BRD v0.4 (`docs/specs/affiliate-program-brd.md`: BR-REG, BR-ATR, BR-KOM, BR-LVL, BR-PCR, BR-PLG, BR-ADJ, BR-PRV, BR-NTF). Dokumen ini hanya menuliskan kontrak teknis; tidak menyalin ulang BRD.

## Agreed behavior

### Alur utama

1. Calon affiliator mendaftar mandiri (form khusus, bukan form siswa) → akun `role=affiliator` aktif + profil `pending_approval`; super_admin setujui (→ kode referral dibuat) atau tolak dengan alasan.
2. Atribusi siswa: klik `/r/{code}` menyimpan cookie `aff_ref` (hari sesuai pengaturan, default 30); kode manual opsional di form register siswa; kode opsional di checkout. Prioritas: **kode manual > cookie > tidak terikat**; klik terakhir menang di antara beberapa klik. Server tidak mencatat klik.
3. Ikatan dibuat saat akun siswa aktif (selesai OTP) atau saat order ber-kode lunas (checkout). Satu siswa hanya satu binding aktif; pembelian lunas ≥ `min_extend_amount` (default 0 = semua pembelian) memperpanjang masa ikatan dari tanggal bayar.
4. Komisi per item order lunas: transaksi `commission` berstatus `pending` (belum efektif, belum masuk saldo) → (job) `available` dengan `effective_at` terisi; refund penuh membatalkan `pending` atau menulis transaksi `refund_deduction` untuk komisi yang sudah `available`.
5. Pencairan: affiliator ajukan (≥ min payout, status program `active`) → transaksi `payout` negatif langsung efektif → admin ekspor daftar → transfer manual → tandai `paid` dengan bukti, atau tolak dengan alasan (transaksi `payout_reversal` mengembalikan saldo).
6. Level dievaluasi kuartal kalender WIB pada hari ke-8 bulan pertama kuartal baru: naik boleh lompat, bertahan lewat ambang hold, turun satu tingkat setelah 2 kuartal gagal; admin bisa mengubah level manual (tidak ada kunci level).

### Keputusan dari diskusi (perubahan atas BRD v0.2/v0.3)

| Keputusan | Isi |
|---|---|
| Role pengelola | Hanya `super_admin` mengelola affiliator (persetujuan, pengaturan, payout, status akun, penyesuaian). Repo punya 5 role (`student`, `admin_school`, `admin_exam`, `admin_store`, `super_admin`) — role lain tidak mendapat akses fitur ini |
| Order manual | Dihapus dari lingkup (BR-KOM-07/FR-KOM-06 v0.3). Pembayaran offline → penyesuaian manual |
| Persetujuan kedua | Tidak dibangun Fase 1 (satu akun super_admin). Ambang, kolom, dan alur persetujuan ditambahkan bersama akun admin kedua |
| Uang | Mengikuti konvensi repo: `NUMERIC(12,2)` di DB, `float64` di Go. Nominal komisi dibulatkan ke rupiah utuh (floor) saat dibuat |
| Barang diterima | `completed_at` order (admin Complete). Event `delivered` kurir hanya fallback tampilan |
| Masa tunggu | Digital default **7 hari** setelah bayar; fisik default **14 hari** setelah `completed_at` (satu parameter `physical_wait_days`). Diatur admin, 0–90; 0 = langsung available |
| Checkout | Hanya kode referral (bukan link) |
| Nama siswa untuk affiliator | Nama pertama + inisial belakang; tanpa email/HP/alamat/nilai |
| Broadcast | Grup penerima baru `affiliators` ditambahkan ke announcement yang ada (email via worker) |
| Feature flag | `system_config` key `affiliate_enabled`; default off |
| Aturan komisi | Setiap aturan berupa `{type: percent|fixed, value}`. `percent` 0–100 maks 2 desimal; `fixed` rupiah utuh **per unit** (qty). Komisi per item dibatasi maksimal `base_amount` item |
| Pencairan | Manual penuh di luar sistem (transfer bank oleh admin, tandai Dibayar + bukti). `payout_day_of_month` hanya informasi untuk affiliator, tidak membatasi pengajuan. Satu payout `requested`/`processing` per affiliator, dijaga DB |
| Status akun | Penangguhan tanpa batas waktu sampai admin mengaktifkan kembali. Pengajuan penarikan ditolak bila status program bukan `active`. Tidak ada pembekuan saldo dan tidak ada kunci level |
| Batas registrasi | `daily_registration_cap` membatasi pendaftar **affiliator** baru per hari WIB (NULL = tanpa batas). Pendaftaran siswa tanpa batas |
| Durasi ikatan | `binding_months` bisa berbeda per level (kosong = ikut global) |
| Ambang level | `rise_threshold` (naik) dan `hold_threshold` (bertahan) per level di `affiliate_settings.levels`; kosong → nilai bawaan kode (Silver 5.000.000, Gold 15.000.000 untuk naik; tanpa batas untuk bertahan) |
| Ditunda dari Fase 1 | Sistem pelanggaran dan banding, penandaan kasus mencurigakan, statistik klik link, riwayat level terstruktur (perubahan manual di audit log), pembekuan saldo, kunci level, skors berjangka |
| Job | Semua job idempoten dan aman di-rerun. Scope rerun: `bindings`, `commissions`, `levels` |

### Failure behavior

- Event `OrderPaid` diproses ulang → idempoten: unique `order_item_id` untuk transaksi komisi; komisi hanya dibuat bila order status bukan `cancelled` pada saat handler membaca (pola yang sudah ada di worker). Kegagalan membuat komisi tidak menggagalkan akses: langkah berjalan di savepoint, dicatat `slog.Error`, dan job rekonsiliasi (jendela 14 hari) membuatnya ulang.
- Refund setelah payout → saldo boleh negatif sementara (transaksi pengurang), ditutup komisi berikutnya.
- Pembatalan order sebelum lunas tidak menyentuh komisi (tidak pernah dibuat).
- Affiliator `suspended`/`deactivated` → komisi baru tidak dibuat; binding siswa tetap tidak dialihkan. Penonaktifan membatalkan komisi `pending`.
- Job dijalankan ulang → tidak menggandakan (day-key guard, `last_evaluated_quarter`, dan status guard).
- Penolakan payout dua kali atau refund dua kali tidak menulis transaksi pembalik/pengurang kedua (unique per `parent_id`).
- Cookie/kode tidak valid atau affiliator non-aktif saat binding dibuat → siswa lanjut tanpa binding, tanpa error bagi siswa.

### Compatibility & rollout

- Checkout yang ada tidak berubah kecuali input opsional kode; validasi promo tetap.
- Satu migrasi berpasangan `0070_affiliate` (up + down) menggantikan `0070`–`0073` yang belum pernah dirilis. Database lokal yang sudah menjalankan versi lama harus di-reset sebelum menjalankan migrasi baru; tidak ada database bersama yang terdampak. Deploy migration lalu flag; flag menyala setelah setting awal diisi.
- Launch date (`affiliate_launch_date` lewat `affiliate_settings.launch_date`, wajib terisi saat flag on): akun `users.created_at` sebelum tanggal itu tidak dapat diikat.
- Default flag per lingkungan: `affiliate_enabled` mati di staging dan produksi sampai dinyalakan manual di System Config. Hanya lokal (`APP_ENV=dev`) menyala otomatis saat API start, bersama `launch_date` lampau; penyemaian itu hanya menyisipkan bila belum ada, jadi pilihan manual (termasuk mematikannya) tidak ditimpa. Aturan komisi dan ambang level memakai nilai bawaan kode (`affiliateDefaultCommissionRule` dan ambang naik default) sampai admin mengisi `levels`, jadi cek angka itu sebelum flag produksi dinyalakan.
- Nama field JSON API tidak berubah untuk fitur yang dipertahankan; `id` affiliator di JSON berisi `user_id`.

## Technical contract

### Role, middleware, flag

- `users.role` CHECK ditambah `'affiliator'`; `service/rbac.go`: `RoleAffiliator = "affiliator"` dengan capabilities kosong.
- `AffiliatorOnlyMiddleware` mirror `StudentOnlyMiddleware` (role-based, bukan capability — super_admin `"*"` tidak boleh menembus jalur affiliator).
- Endpoint admin fitur ini di grup `/admin` + `RBACMiddleware("affiliates:*")` — hanya `super_admin` yang memilikinya.
- Flag dibaca lewat `GetSystemConfig` (pola `purchaseNotifyEnabled`): fitur off → endpoint atribusi mengabaikan kode, halaman affiliator ditolak (404/403), checkout tanpa kode tetap normal.

### Model data (satu migrasi `0070_affiliate`)

Kolom uang `NUMERIC(12,2)`. Semua id `UUID`. Empat tabel baru dan dua kolom di tabel existing.

**Tabel existing yang berubah**

| Tabel | Perubahan |
|---|---|
| `users` | Hanya constraint `users_role_check` ditambah `'affiliator'`. Tidak ada kolom baru; `users.status` tetap berarti "boleh login" |
| `orders` | `affiliate_code TEXT` (nullable): kode checkout yang dibawa sampai order lunas |
| `product` | `affiliate_commission JSONB` (nullable): override komisi per produk `{"type":"percent|fixed","value":N,"updated_at":"..."}`. Menggantikan tabel override terpisah; `updated_by` tercatat di `audit_log` |

**Tabel baru**

| Tabel | Kolom kunci & aturan |
|---|---|
| `affiliators` (9 kolom) | `user_id` PK → `users`; `referral_code` TEXT UNIQUE, NULL sampai disetujui (8 char A-Z0-9 tanpa O/0/I/1); `program_status` `pending_approval|active|suspended|deactivated|rejected`; `rejection_reason`; `level` `bronze|silver|gold` default bronze; `missed_quarters` int; `last_evaluated_quarter` TEXT ('2026-Q4', penjaga idempotensi evaluasi); `terms_version`; `bank_account` JSONB `{bank_name, account_number, account_holder}`. Waktu daftar = `users.created_at` |
| `affiliate_bindings` (9 kolom) | `id`, `affiliator_id` FK → `affiliators.user_id`, `student_id` FK users; `status` `active|ended`; `source` `link|manual_code|checkout_code`; `started_at`, `expires_at` (tanggal kedaluwarsa, dimajukan saat perpanjang), `created_at`, `updated_at`. Partial UNIQUE `(student_id) WHERE status='active'`; index `(affiliator_id, status)` |
| `affiliate_transactions` (20 kolom) | Buku transaksi tunggal, lihat bagian berikut |
| `affiliate_settings` (13 kolom, singleton) | `id=1`, `cookie_days=30`, `binding_months=3`, `min_extend_amount=0`, `digital_wait_days=7`, `physical_wait_days=14`, `min_payout=100000`, `payout_day_of_month` (informasi saja), `launch_date`, `eval_day=8`, `daily_registration_cap` NULL, `levels` JSONB, `updated_at`. Tanpa seed: baris yang hilang atau nilai yang kosong memakai bawaan kode |

`levels` JSONB memuat per level (`bronze|silver|gold`): `commission_rule` `{exam,course,book,merchandise,medal}` masing-masing `{type: percent|fixed, value}`, `binding_months`, `digital_wait_days`, `physical_wait_days`, `min_payout`, `payout_day_of_month`, `rise_threshold`, `hold_threshold`. Nilai kosong mengikuti global (atau bawaan kode untuk aturan komisi dan ambang). Bronze tidak punya ambang naik.

### `affiliate_transactions`

Satu baris per peristiwa uang atau calon peristiwa uang affiliator.

| Kelompok | Kolom |
|---|---|
| Inti | `id`, `affiliator_id`, `type`, `status`, `amount` (bertanda), `effective_at`, `parent_id` (self-FK), `actor_id` (NULL = job), `note`, `created_at`, `updated_at` |
| Komisi | `student_id`, `order_id`, `order_item_id`, `product_type`, `quantity`, `base_amount`, `release_at`, `order_paid_at` |
| JSONB | `detail`: komisi `{rule:{type,value}, level}`; payout `{bank_account, proof_url, reject_reason, processed_by, processed_at}`; penyesuaian `{category}` |

Tipe, tanda, dan status:

| `type` | Tanda | `status` | Catatan |
|---|---|---|---|
| `commission` | + | `pending` → `available` atau `cancelled`; `available` → `cancelled` | Efektif saat `available` |
| `refund_deduction` | − | `posted` | `parent_id` = komisi; efektif seketika |
| `payout` | − | `requested` → `processing` → `paid` atau `rejected` (`requested` boleh langsung `paid`/`rejected`) | Efektif seketika saat diajukan |
| `payout_reversal` | + | `posted` | `parent_id` = payout; ditulis saat ditolak |
| `adjustment` | ± | `posted` | `note` wajib (alasan); `detail.category` `correction|offline|bonus|other` |

Aturan integritas (CHECK, index, trigger `affiliate_transactions_guard()`):

- Saldo pada tanggal T = `SUM(amount)` untuk baris dengan `effective_at <= T`; saldo sekarang = baris dengan `effective_at` terisi. Status tidak memengaruhi saldo.
- `DELETE` ditolak. `id`, `affiliator_id`, `type`, `amount`, `parent_id`, `order_item_id`, `created_at` tidak dapat diubah.
- `effective_at` hanya boleh berubah satu kali, dari NULL menjadi terisi; komisi `pending` → `available` mengisinya pada transisi yang sama. `release_at` hanya boleh NULL → nilai satu kali (komisi fisik).
- Status hanya boleh berpindah mengikuti tabel di atas. `detail` komisi tidak berubah; `detail` payout hanya boleh diisi bersama perpindahan status.
- CHECK memasangkan `type` dengan himpunan `status` yang sah.
- Unique: `order_item_id` untuk `type='commission'`; satu `refund_deduction` per `parent_id`; satu `payout_reversal` per `parent_id`; satu payout `requested`/`processing` per `affiliator_id`.
- Index: `(affiliator_id) WHERE effective_at IS NOT NULL`, `(release_at) WHERE type='commission' AND status='pending'`, `(order_id)`.

### Pipeline komisi (poin integrasi di kode yang ada)

- `markOrderPaidTx` (service/store.go) **tidak diubah kecuali satu titik**: order checkout dengan `affiliate_code` memanggil pembuatan binding (`paidOrderBindingTx`) di dalam tx yang sama, sebelum event outbox `OrderPaid` ditulis. Komisi itu sendiri dibuat oleh worker dari event. `Checkout` menerima `affiliateCode` sebagai parameter eksplisit (bukan variadic).
- Worker `handleOrderPaid` (worker/outbox.go) diperluas: setelah fulfilment akses, bila flag on dan binding siswa aktif → untuk tiap item kategori `exam|course|book|merchandise|medal`: buat transaksi komisi `ON CONFLICT (order_item_id) DO NOTHING` dengan snapshot: `base_amount` = alokasi net per item, aturan (`detail.rule`) = override produk > level setting > bawaan, `amount` = `percent`: floor(base × value/100); `fixed`: value × quantity; lalu dibatasi `min(amount, base_amount)`; `detail.level`, `release_at` = `paid_at + digital_wait_days` (digital) atau NULL (fisik, menunggu `completed_at`; setelah itu `completed_at + physical_wait_days`).
- `AdminRefundOrder` diperluas (tx sama): komisi `pending` order itu → `cancelled`; komisi `available` → `cancelled` dan satu transaksi `refund_deduction` negatif (refund platform hanya penuh → deduction penuh per item).
- Job harian worker (loop ticker, guard day-key WIB `Asia/Jakarta`): (a) binding `expires_at < now` → `ended`; (b) rekonsiliasi komisi yang hilang (jendela 14 hari); (c) komisi fisik `pending` dengan `completed_at` terisi → hitung `release_at`; (d) komisi `pending` dengan `release_at <= now` → `available` + `effective_at`.
- Job kuartalan: dieksekusi dari loop harian bila hari evaluasi (setting) kuartal baru tercapai; evaluasi BR-LVL-02..07 memakai `users.created_at` untuk kelayakan dan `last_evaluated_quarter` sebagai penjaga per affiliator (tidak menaikkan `missed_quarters` dua kali). Rerun hanya untuk kuartal terakhir yang jatuh tempo.
- Rerun admin: `POST /admin/affiliate-jobs/run {scope: bindings|commissions|levels, quarter?}`; semua scope idempoten.
- Waktu job: ticker di worker (pola `exam_expiry.go`) dengan guard day-key WIB; jam tidak diatur dari UI, admin hanya mengatur `eval_day` dan rerun manual.

### Atribusi & registrasi

- Halaman web `/r/{code}` (publik, client component): set cookie `aff_ref` (`document.cookie`, path=/, 30 hari) → redirect home, lalu memanggil `GET /affiliates/public/validate?code=` untuk mengambil `cookie_days` dari pengaturan dan memasang ulang cookie dengan umur itu. Tidak ada endpoint backend untuk mencatat klik.
- `POST /auth/register` dan `/auth/google` menambah field opsional `referral_code` (kode manual) — keduanya hanya membuat akun; binding tidak dibuat di sini.
- `POST /auth/otp/verify` menambah field opsional `referral_code` (manual) dan `referral_code_cookie`: saat akun jadi aktif, service resolusi manual > cookie; validasi kode ada & affiliator `active`; tolak self-purchase (email/HP identik dengan affiliator) → tidak dibinding; tolak akun pre-launch; buat binding `source=manual_code|link` dalam tx aktivasi.
- `GET /api/v1/affiliates/public/validate?code=` untuk UX register/checkout dan halaman `/r/{code}` (tanpa JWT): mengembalikan `valid`, `affiliator_name`, dan `cookie_days` (umur cookie dari pengaturan, dikirim selama fitur aktif agar kode yang belum aktif pun mendapat umur cookie yang benar).
- Checkout: `POST /orders/:id/checkout` body opsional `affiliate_code` — hanya diproses bila pembeli tak terikat, `created_at ≥ launch_date`, kode milik affiliator aktif, bukan dirinya sendiri; binding dibuat di `markOrderPaidTx` (`source=checkout_code`).
- Registrasi affiliator: `POST /auth/affiliators/register` (email, HP, nama, terms_version) — mirror alur OTP register siswa; tolak dengan error khusus bila `daily_registration_cap` hari ini (WIB) sudah tercapai (hitung dari `users` dengan `role='affiliator'`); tolak email/username yang sudah dipakai akun `admin_school` (BR-REG-03); buat user `role=affiliator` + profil `pending_approval`.
- Kode referral dibuat saat approval (server-generated, coba lagi bila collision).

### Endpoint

Affiliator (JWT + `AffiliatorOnlyMiddleware`, flag on):

```
GET    /api/v1/affiliates/me                     profil, kode, link, level, progres kuartal, peringatan hold
PATCH  /api/v1/affiliates/me/bank                data rekening (audit perubahan)
GET    /api/v1/affiliates/me/bindings            siswa binaan: nama pertama+inisial, tgl daftar, status ikatan, total belanja agregat
GET    /api/v1/affiliates/me/commissions         filter status/periode/kategori
GET    /api/v1/affiliates/me/ledger              riwayat mutasi saldo (transaksi efektif) + saldo
GET    /api/v1/affiliates/me/payouts             riwayat penarikan
POST   /api/v1/affiliates/me/payouts             ajukan (tolak bila status program bukan active, di bawah min, atau sudah ada payout berjalan)
GET    /api/v1/affiliates/me/adjustments         riwayat penyesuaian + alasan
GET    /api/v1/affiliates/me/stats               ringkasan siswa, omzet, komisi per periode
```

Super admin (grup `/admin`, `RBACMiddleware("affiliates:*")`):

```
GET    /admin/affiliates?status&level&q          daftar + antrean persetujuan (status filter)
POST   /admin/affiliates/:id/approval            {approve|reject, reason}
GET    /admin/affiliates/:id                     detail (profil, siswa, komisi, saldo); :id = user id
PATCH  /admin/affiliates/:id/status              {status, reason} (suspend tanpa batas waktu)
POST   /admin/affiliates/:id/level               {level, reason}
POST   /admin/affiliates/:id/adjustments         {category, amount, reason, note} → transaksi adjustment + audit
GET    /admin/affiliates/:id/adjustments, /commissions, /commissions/export
GET    /admin/affiliate-payouts                  antrean + filter
GET    /admin/affiliate-payouts/export           CSV transfer manual (pola csv_safe.go)
POST   /admin/affiliate-payouts/:id/process      requested → processing
POST   /admin/affiliate-payouts/:id/decide       {paid + proof_url | reject + reason}
GET/PUT /admin/affiliate-settings                pengaturan global + levels; PUT menulis audit (diff nilai lama/baru)
GET/PUT/DELETE /admin/affiliate-product-overrides  override komisi per produk (kolom product.affiliate_commission)
GET    /admin/affiliate-reports/{summary,liability,orders,orders/export}
POST   /admin/affiliate-jobs/run                 rerun job idempoten (scope: bindings|commissions|levels)
```

Dihapus dari kontrak sebelumnya: `POST /affiliates/clicks`, `/affiliates/me/violations` dan banding, `/admin/affiliates/:id/{freeze,unfreeze,violations}`, grup `/admin/affiliate-flags` dan `/admin/affiliate-appeals`. Rate limiter publik (`AffiliateRateLimiter`) dipakai register affiliator dan validasi kode.

Announcement: enum grup penerima + `affiliators`; `ListActiveUserEmails` menangani role `affiliator` (program_status aktif, bukan semua user).

### Frontend (web/)

| Permukaan | Isi |
|---|---|
| `app/r/[code]/page.tsx` | Publik: set cookie `aff_ref`, redirect `/` |
| `(auth)/register` | Kolom kode referral opsional (prefill dari cookie), kirim `referral_code` + `referral_code_cookie` di register dan verify |
| `(auth)/affiliate-register` | Form pendaftaran affiliator + Ketentuan Program (FR-REG-01/05) |
| `(student)` checkout | Input kode opsional (tampil bila memenuhi BR-ATR-05; validasi via endpoint publik) |
| `(affiliate)` portal | Ringkasan, kode/link, siswa binaan, komisi (filter), saldo & penarikan (mutasi + payout), akun/rekening. Tanpa halaman sanksi dan banding |
| `(admin)/admin/affiliates` | Daftar+persetujuan; detail bertab (profil, siswa, komisi, saldo); payout antrean+export; settings (global, per level termasuk ambang naik/bertahan, override produk, job); reports. Tanpa halaman flags dan appeals |
| Settings admin | Tiap baris komisi (global, level, override produk) punya pilihan Persen/Nominal; ambang naik dan bertahan hanya di tab per level; `payout_day_of_month` berlabel informasi |
| web/lib | `types.ts`, `api.ts`, hooks React Query per page-group; test vitest mengikuti `docs/standards/web-tests.md` |

### Privasi & keamanan

- Pembatasan affiliator di level query (repo memfilter `affiliator_id` dari JWT `user_id`), bukan hanya UI (BR-PRV-01).
- Payload siswa binaan tanpa email/HP/alamat/nilai ujian (BR-PRV-02); nama = nama pertama + inisial.
- Semua aksi admin → `audit_log` lewat `InsertAuditLogMeta` (approval, status, level, adjustment, payout, settings, override produk) termasuk perpindahan status transaksi.
- Rekening bank hanya di `affiliators.bank_account` dan snapshot `detail.bank_account` payout; tidak ada di `users` dan tidak dikirim di respons profil umum.
- Rate limit (`LoginRateLimiter` pattern) pada `/affiliates/public/validate` dan `POST /auth/affiliators/register` (NFR-07).
- Log worker per pembuatan komisi/kegagalan job; counter metrics komisi dibuat/dibatalkan/payout (NFR-11).

## Acceptance and proof

| # | Kriteria | Bukti |
|---|---|---|
| A1 | Register siswa dengan kode manual/cookie menghasilkan binding `active` 3 bulan; manual menang atas cookie; klik lain saat terikat tidak mengubah binding; akun pre-launch tidak terikat | Integration test (Postgres testcontainer) di service binding |
| A2 | Order lunas → tepat 1 transaksi komisi per item (duplicate `OrderPaid` tetap 1); snapshot rule/level/base tersimpan; perubahan setting setelahnya tidak mengubah komisi lama | Unit + integration test worker; query `count(*)` |
| A3 | Refund: `pending` → `cancelled` tanpa menyentuh saldo; `available` → `cancelled` + satu `refund_deduction`; `saldo == SUM(amount) WHERE effective_at IS NOT NULL` setiap kasus; refund dua kali tidak menulis dua deduction | Integration test refund path |
| A4 | Masa tunggu: digital `paid_at + 7d` (setting), fisik `completed_at + physical_wait_days` (default 14d); job transisi `pending→available` dengan `effective_at`; fisik tanpa `completed_at` tetap pending | Unit test job (fake clock) + integration |
| A5 | Perpanjangan ikatan saat pembelian ≥ `min_extend_amount`; `min_extend_amount=0` default; order gratis tidak memperpanjang | Integration test binding extension |
| A6 | Evaluasi level kuartalan: naik lompat, bertahan, turun setelah 2 kuartal gagal; rerun idempoten via `last_evaluated_quarter`; ambang naik/bertahan dari `levels` dengan bawaan kode | Unit test evaluator + rerun integration |
| A7 | Payout: tolak di bawah min, saat status program bukan `active`, dan saat sudah ada payout `requested/processing`; reject menulis satu `payout_reversal` dan mengembalikan saldo; mark paid wajib bukti; export CSV | Handler + service test |
| A8 | Penyesuaian: langsung efektif, alasan wajib, tolak target diri sendiri, audit row tercipta | Service test |
| A9 | Broadcast `recipients=affiliators` hanya email akun role affiliator dengan program aktif | Service test announcement |
| A10 | Flag off: atribusi diabaikan, endpoint affiliator 403/404, checkout tanpa kode berjalan normal | Integration test gate |
| A11 | Affiliator hanya melihat datanya sendiri; payload siswa tanpa PII; nama pertama+inisial | Handler test authz + payload assertion |
| A12 | Siswa tidak melihat apa pun tentang ikatan (tidak ada endpoint/field baru untuk student) | Review diff + handler test |
| A13 | Komisi `fixed` = nominal × qty, dibatasi `base_amount`; `percent` floor; override produk > level > bawaan; snapshot type/value tersimpan di `detail.rule` | Unit test kalkulator + integration worker |
| A14 | Rerun tiap scope job (`bindings|commissions|levels`) dua kali menghasilkan state sama (tanpa transaksi ganda, `missed_quarters` tidak bertambah dua kali) | Integration test per scope |
| A15 | Penarikan ditolak bila status program bukan `active`; registrasi affiliator ditolak saat cap tercapai (hitung dari `users`) | Service + handler test |
| A16 | `binding_months` level dipakai saat ikatan dibuat/diperpanjang; kosong ikut global; ikatan lama tidak berubah | Integration test binding |
| A17 | Trigger `affiliate_transactions_guard`: `UPDATE amount`/`type`/`affiliator_id`/`parent_id`, `DELETE`, perubahan kedua `effective_at`, dan transisi status yang tidak sah ditolak; satu payout berjalan per affiliator ditegakkan DB | Migration/DB test (testcontainer) |
| A18 | Saldo pada tanggal T stabil: setelah refund atau penolakan payout, saldo untuk T sebelum peristiwa itu tidak berubah | Integration test rekonsiliasi |
| A19 | Endpoint yang dihapus (clicks, violations, appeals, flags, freeze, unfreeze) tidak lagi terdaftar; scope job `flags` dan `releases` ditolak | Route test + handler test |
| A20 | Migrasi `0070_affiliate`: tabel, kolom `orders`/`product`, constraint role, dan down yang mengembalikan skema | Migration test |

Strategi verifikasi (mengikuti `.agents/verification.md`): gate penuh backend `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh` (background, >10 menit) sebelum PR; loop fokus per paket saat build; frontend `npx tsc --noEmit && npx vitest run && npm run build` di `web/`; satu spec Playwright (`web/e2e/affiliate-program.spec.ts`) untuk alur referral dijalankan lokal terhadap compose dev bila stack tersedia, bila tidak dicatat tidak dijalankan.

## Scope

**Non-goals Fase 1**: order manual; alur persetujuan kedua; pajak; payout otomatis; notifikasi otomatis (broadcast manual saja) termasuk status "Segera Berakhir"; bonus target; multi-level; aturan khusus per affiliator; fitur agen; refund parsial; sistem poin/loyalty; sistem pelanggaran, sanksi bertingkat, dan banding; penandaan kasus mencurigakan; pembekuan saldo; kunci level; skors berjangka; statistik klik link; riwayat level terstruktur.

**Keputusan tertunda (nilai bisnis, tidak menghambat)**: angka komisi per kategori (placeholder), ambang level (placeholder), tanggal peluncuran & jadwal payout (Q-12), biaya transfer (Q-11), retensi data (Q-16), perlakuan saldo affiliator nonaktif permanen (Q-10 — default: saldo tetap tercatat, penarikan hanya untuk status `active`).

**Residual risk**: satu akun super_admin mengelola dana (mitigasi audit log + trigger buku transaksi + rekonsiliasi saldo per tanggal); tanpa deteksi fraud otomatis (hanya penolakan self-purchase saat binding); cookie bisa hilang (kode manual & checkout sebagai cadangan); JSONB `detail` dan `levels` tidak punya CHECK per field (divalidasi service, `detail` komisi dikunci trigger); Playwright belum terbukti tanpa stack lokal.
