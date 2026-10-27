---
title: Program Affiliator (affiliate klasik) — Fase 1
issue: none (BRD internal)
status: accepted
created: 2026-10-25
sources:
  - BRD-Fitur-Affiliator.md (v0.3)
  - docs/standards/go-patterns.md
  - backend/internal/service/store.go (outbox OrderPaid, refund, complete)
  - backend/internal/worker/outbox.go (loop worker)
  - backend/internal/service/announcement.go (grup penerima broadcast)
  - backend/internal/service/rbac.go (role & capability)
---

# Program Affiliator (affiliate klasik) — Fase 1

## Problem dan outcome

Akuisisi siswa lewat rekomendasi mulut-ke-mulut tidak terukur dan tidak bisa diberi imbalan. Outcome Fase 1: individu yang disetujui (affiliator) membawa calon siswa lewat link/kode referral, siswa terikat 3 bulan (dapat diperpanjang), setiap pembelian lunas menghasilkan komisi dengan masa tunggu refund, komisi masuk ledger append-only, dicairkan manual oleh admin, dan level (Bronze/Silver/Gold) dievaluasi kuartalan. Semua angka aturan diatur super_admin lewat halaman pengaturan.

Aturan bisnis lengkap ada di BRD v0.3 (BR-REG, BR-ATR, BR-KOM, BR-LVL, BR-PCR, BR-PLG, BR-ADJ, BR-PRV, BR-NTF). Dokumen ini hanya menuliskan kontrak teknis; tidak menyalin ulang BRD.

## Agreed behavior

### Alur utama

1. Calon affiliator mendaftar mandiri (form khusus, bukan form siswa) → akun `role=affiliator` aktif + profil `pending_approval`; super_admin setujui (→ kode referral dibuat) atau tolak dengan alasan.
2. Atribusi siswa: klik `/r/{code}` menyimpan cookie `aff_ref` 30 hari; kode manual opsional di form register siswa; kode opsional di checkout. Prioritas: **kode manual > cookie > tidak terikat**; klik terakhir menang di antara beberapa klik.
3. Ikatan dibuat saat akun siswa aktif (selesai OTP) atau saat order ber-kode lunas (checkout). Satu siswa hanya satu binding aktif; pembelian lunas ≥ `min_extend_amount` (default 0 = semua pembelian) memperpanjang 3 bulan dari tanggal bayar.
4. Komisi per item order lunas: status `pending` → (job) `available` + entri ledger → dibayar lewat payout; refund penuh membatalkan `pending` atau menulis entri pengurang ledger; `held` oleh sanksi.
5. Pencairan: affiliator ajukan (≥ min payout, tidak dibekukan) → admin ekspor daftar → transfer manual → tandai Dibayar dengan bukti, atau tolak dengan alasan (saldo kembali).
6. Level dievaluasi kuartal kalender WIB pada hari ke-8 bulan pertama kuartal baru: naik boleh lompat, bertahan lewat ambang hold, turun satu tingkat setelah 2 kuartal gagal; admin bisa ubah/kunci manual.

### Keputusan dari diskusi (perubahan atas BRD v0.2)

| Keputusan | Isi |
|---|---|
| Role pengelola | Hanya `super_admin` mengelola affiliator (persetujuan, pengaturan, payout, sanksi, penyesuaian). Repo punya 5 role (`student`, `admin_school`, `admin_exam`, `admin_store`, `super_admin`) — role lain tidak mendapat akses fitur ini |
| Order manual | Dihapus dari lingkup (BR-KOM-07/FR-KOM-06 v0.3). Pembayaran offline → penyesuaian manual |
| Persetujuan kedua | Tidak dibangun Fase 1 (satu akun super_admin). Tabel penyesuaian menyimpan kolom approval untuk masa depan; pengaturan ambang tetap ada, default nonaktif |
| Uang | Mengikuti konvensi repo: `NUMERIC(12,2)` di DB, `float64` di Go. Nominal komisi dibulatkan ke rupiah utuh (floor) saat dibuat |
| Barang diterima | `completed_at` order (admin Complete). Event `delivered` kurir hanya fallback tampilan |
| Masa tunggu | Digital default **7 hari** setelah bayar; fisik default **14 hari** setelah `completed_at` (satu parameter `physical_wait_days`). Diatur admin, 0–90; 0 = langsung available |
| Checkout | Hanya kode referral (bukan link) |
| Nama siswa untuk affiliator | Nama pertama + inisial belakang; tanpa email/HP/alamat/nilai |
| Broadcast | Grup penerima baru `affiliators` ditambahkan ke announcement yang ada (email via worker) |
| Feature flag | `system_config` key `affiliate_enabled`; default off |
| Aturan komisi | Setiap aturan berupa `{type: percent|fixed, value}`. `percent` 0–100 maks 2 desimal; `fixed` rupiah utuh **per unit** (qty). Komisi per item dibatasi maksimal `base_amount` item |
| Pencairan | Manual penuh di luar sistem (transfer bank oleh admin, tandai Dibayar + bukti). `payout_day_of_month` hanya informasi untuk affiliator, tidak membatasi pengajuan |
| Pembekuan saldo | `freeze_max_days` minimal 1 (0 ditolak). Pengajuan penarikan juga ditolak bila status affiliator bukan `active` |
| Batas registrasi | `daily_registration_cap` membatasi pendaftar **affiliator** baru per hari WIB (NULL = tanpa batas). Pendaftaran siswa tanpa batas |
| Durasi ikatan | `binding_months` bisa berbeda per level (NULL = ikut global) |
| Job | Semua job idempoten dan aman di-rerun, termasuk flags dan lepas suspend/freeze |

### Failure behavior

- Event `OrderPaid` diproses ulang → idempoten: unique `order_item_id` di komisi; komisi hanya dibuat bila order status bukan `cancelled` pada saat handler membaca (pola yang sudah ada di worker).
- Refund setelah payout → saldo boleh negatif sementara (entri pengurang), ditutup komisi berikutnya.
- Pembatalan order sebelum lunas tidak menyentuh komisi (tidak pernah dibuat).
- Affiliator `suspended`/`deactivated` → komisi baru tidak dibuat; binding siswa tetap tidak dialihkan; `suspended_until` lewat → job mengembalikan ke `active`.
- Job dijalankan ulang → tidak menggandakan (day-key guard + status guard).
- Cookie/kode tidak valid atau affiliator non-aktif saat binding dibuat → siswa lanjut tanpa binding, tanpa error bagi siswa.

### Compatibility & rollout

- Checkout yang ada tidak berubah kecuali input opsional kode; validasi promo tetap.
- Migration baru berpasangan (`0070_*`); deploy migration lalu flag; flag menyala setelah setting awal diisi.
- Launch date (`affiliate_launch_date`, wajib terisi saat flag on): akun `users.created_at` sebelum tanggal itu tidak dapat diikat.
- Default flag per lingkungan: `affiliate_enabled` mati di staging dan produksi sampai dinyalakan manual di System Config. Hanya lokal (`APP_ENV=dev`) menyala otomatis saat API start, bersama `launch_date` lampau; penyemaian itu hanya menyisipkan bila belum ada, jadi pilihan manual (termasuk mematikannya) tidak ditimpa. Persentase komisi memakai nilai bawaan kode (`affiliateDefaultCommissionPct`) sampai admin mengisi level, jadi cek angka itu sebelum flag produksi dinyalakan.

## Technical contract

### Role, middleware, flag

- `users.role` CHECK ditambah `'affiliator'`; `service/rbac.go`: `RoleAffiliator = "affiliator"` dengan capabilities kosong.
- `AffiliatorOnlyMiddleware` mirror `StudentOnlyMiddleware` (role-based, bukan capability — super_admin `"*"` tidak boleh menembus jalur affiliator).
- Endpoint admin fitur ini di grup `/admin` + `RBACMiddleware("affiliates:*")` — hanya `super_admin` yang memilikinya.
- Flag dibaca lewat `GetSystemConfig` (pola `purchaseNotifyEnabled`): fitur off → endpoint atribusi mengabaikan kode, halaman affiliator ditolak (404/403), checkout tanpa kode tetap normal.

### Model data (migrasi baru)

Kolom uang `NUMERIC(12,2)`. Semua id `UUID`. `created_at`/`updated_at` di semua tabel.

| Tabel | Kolom kunci & aturan |
|---|---|
| `affiliators` | `user_id` FK users UNIQUE; `code` TEXT UNIQUE (8 char A-Z0-9 tanpa O/0/I/1); `status` `pending_approval|active|suspended|deactivated|rejected`; `rejection_reason`; `level` `bronze|silver|gold` default bronze; `level_locked_until`; `warn_quarters` int; `suspended_until`; `frozen_until`; `terms_version` + `terms_accepted_at`; `bank_name`, `bank_account_number`, `bank_account_holder` |
| `affiliate_bindings` | `affiliator_id` FK, `student_id` FK users; `status` `active|ended`; `source` `link|manual_code|checkout_code`; `started_at`, `ended_at`, `last_extended_at`. Partial UNIQUE `(student_id) WHERE status='active'`; index `(affiliator_id, status)` |
| `affiliate_commissions` | `affiliator_id`, `student_id`, `order_id` FK, `order_item_id` FK UNIQUE, `product_type`, `quantity`, `base_amount`, `commission_type` `percent|fixed`, `commission_value NUMERIC(12,2)`, `level_snapshot`, `amount`, `status` `pending|available|paid|cancelled|held`, `wait_until`, `paid_at`, `received_at` (fisik). Index `(affiliator_id, status)`, `(status, wait_until)` |
| `affiliate_ledger_entries` | Append-only (trigger DB larang UPDATE/DELETE): `affiliator_id`, `entry_type` `commission|refund_deduction|adjustment|payout|payout_reversal`, `amount` signed, `commission_id`/`payout_id`/`adjustment_id` nullable, `note`, `actor_id` (NULL = job). UNIQUE `(commission_id, entry_type)` WHERE `commission_id IS NOT NULL` |
| `affiliate_payouts` | `affiliator_id`, `amount`, `status` `requested|processing|paid|rejected`, snapshot rekening saat ajukan, `proof_url`, `processed_by/at`, `reject_reason` |
| `affiliate_adjustments` | `affiliator_id`, `category` `correction|offline|bonus|violation|other`, `amount` signed, `reason` NOT NULL, `note`, `approval_status` NULL|`pending|approved` (Fase 1 langsung jadi entri ledger), `approver_id/at`, `created_by` |
| `affiliate_violations` | `affiliator_id`, `category`, `description`, `sanction` `warning|commission_cancel|balance_freeze|level_change|suspend|deactivate`, `suspension_until`, `status` `open|resolved`, `decided_by/at`, `appeal_text`, `appeal_status` `none|pending|upheld|rejected`, `appeal_decided_by/at` |
| `affiliate_level_history` | `affiliator_id`, `quarter` ('2026-Q4'), `from_level`, `to_level`, `net_revenue`, `source` `auto|manual`, `reason`, `created_by` |
| `affiliate_settings` | Singleton id=1: `cookie_days=30`, `binding_months=3`, `min_extend_amount=0`, `digital_wait_days=7`, `physical_wait_days=14`, `min_payout=100000`, `payout_day_of_month` (informasi saja), `adjustment_threshold=1000000` (nonaktif Fase 1), `adjustment_threshold_enabled=false`, `freeze_max_days=14` (min 1), `soon_end_days=30`, `launch_date`, `eval_day=8`, `silver_threshold=5000000`, `gold_threshold=15000000`, `hold_thresholds` jsonb, `daily_registration_cap` NULL (batas pendaftar affiliator per hari WIB), `updated_by/at` |
| `affiliate_level_settings` | PK `level`; `commission_rule` jsonb `{exam,course,book,merchandise,medal}` masing-masing `{type: percent|fixed, value}`, NULL=global; `binding_months`, `digital_wait_days`, `physical_wait_days`, `min_payout`, `payout_day_of_month` NULL=global; `rise_threshold`, `hold_threshold` (bronze NULL) |
| `affiliate_product_overrides` | `product_id` FK UNIQUE; `type` `percent|fixed`; `value NUMERIC(12,2)` (percent 0–100 maks 2 desimal, fixed rupiah utuh ≥ 0) |
| `affiliate_clicks` | `affiliator_id`, `code`, `ip_hash`, `user_agent` — statistik dasar FR-ATR-05 |
| `affiliate_flags` | `affiliator_id`, `rule` `self_purchase|registration_spike|small_repeat_purchase|high_refund_ratio|similar_contacts`, `payload` jsonb, `status` `open|reviewed|valid|ignored`, `note`, `reviewed_by/at` |

### Pipeline komisi (poin integrasi di kode yang ada)

- `markOrderPaidTx` (service/store.go) **tidak diubah kecuali satu titik**: order checkout dengan `affiliate_code` memanggil pembuatan binding (BR-ATR-05: binding + komisi lahir saat lunas) di dalam tx yang sama, sebelum event outbox `OrderPaid` ditulis. Komisi itu sendiri dibuat oleh worker dari event.
- Worker `handleOrderPaid` (worker/outbox.go) diperluas: setelah fulfilment akses, bila flag on dan binding siswa aktif → untuk tiap item kategori `exam|course|book|merchandise|medal`: buat komisi `ON CONFLICT (order_item_id) DO NOTHING` dengan snapshot: `base_amount` = alokasi net per item, aturan (`commission_type` + `commission_value`) = product override > level setting > global (percent 0 / fixed 0 atau produk di luar 5 kategori → skip), `amount` = `percent`: floor(base × value/100); `fixed`: value × quantity; lalu dibatasi `min(amount, base_amount)`; `level_snapshot`, `wait_until` = `paid_at + digital_wait_days` (digital) atau NULL (fisik, menunggu `completed_at`; setelah itu `completed_at + physical_wait_days`).
- `AdminRefundOrder` diperluas (tx sama): komisi `pending` order itu → `cancelled`; komisi `available|paid` → entri ledger `refund_deduction` negatif proporsional per item (refund platform hanya penuh → deduction penuh per item).
- Job harian worker (loop ticker baru, guard day-key WIB `Asia/Jakarta`): (a) binding `ended_at < now` → `ended`; (b) komisi `pending` dengan `wait_until <= now` → `available` + entri ledger `commission`; (c) komisi fisik `pending` dengan `received_at` baru terisi → hitung `wait_until`; (d) `suspended_until`/`frozen_until` lewat → reaktif/unfreeze; (e) jalankan aturan flag (BR-PLG-08) → tulis `affiliate_flags`, dedup lewat unique `(affiliator_id, rule, day_key)` untuk flag `open`.
- Job kuartalan: dieksekusi dari loop harian bila hari evaluasi (setting) kuartal baru tercapai; evaluasi BR-LVL-02..07 → tulis level + `affiliate_level_history`. Unique `(affiliator_id, quarter)` di history; bila baris sudah ada, evaluasi dilewati (tidak menaikkan `warn_quarters` dua kali). Rerun hanya untuk kuartal terakhir yang jatuh tempo.
- Rerun admin: `POST /admin/affiliate-jobs/run {scope: bindings|commissions|levels|flags|releases, quarter?}`; semua scope idempoten (status guard, unique key, dan day-key). `releases` = lepas `suspended_until`/`frozen_until`.
- Waktu job: ticker baru di worker (pola `exam_expiry.go`) dengan guard day-key WIB; jam tidak diatur dari UI, admin hanya mengatur `eval_day` dan rerun manual.

### Atribusi & registrasi

- Halaman web `/r/{code}` (publik, client component): set cookie `aff_ref` (`document.cookie`, path=/, 30 hari per setting) → redirect home. Tidak ada endpoint backend untuk klik selain pencatat statistik `POST /api/v1/affiliates/clicks` (rate-limited, dedup per hari per IP-hash).
- `POST /auth/register` dan `/auth/google` menambah field opsional `referral_code` (kode manual) — keduanya hanya membuat akun; binding tidak dibuat di sini.
- `POST /auth/otp/verify` menambah field opsional `referral_code` (manual) dan `referral_code_cookie`: saat akun jadi aktif, service resolusi manual > cookie; validasi kode ada & affiliator `active`; tolak self-purchase (email/HP identik dengan affiliator) → tidak dibinding + tulis flag; tolak akun pre-launch; buat binding `source=manual_code|link` dalam tx aktivasi.
- `POST /api/v1/affiliates/public/validate?code=` untuk UX register/checkout (tanpa JWT).
- Checkout: `POST /orders/:id/checkout` body opsional `affiliate_code` — hanya diproses bila pembeli tak terikat, `created_at ≥ launch_date`, kode milik affiliator aktif, bukan dirinya sendiri; binding dibuat di `markOrderPaidTx` (`source=checkout_code`).
- Registrasi affiliator: `POST /auth/affiliators/register` (email, HP, nama, terms_version) — mirror alur OTP register siswa; tolak dengan error khusus bila `daily_registration_cap` hari ini (WIB) sudah tercapai (form menampilkan "pendaftaran hari ini penuh"); tolak email/username yang sudah dipakai akun `admin_school` (BR-REG-03); buat user `role=affiliator` + profil `pending_approval`.
- Kode referral dibuat saat approval (server-generated, coba lagi bila collision).

### Endpoint

Affiliator (JWT + `AffiliatorOnlyMiddleware`, flag on):

```
GET    /api/v1/affiliates/me                     profil, kode, link, level, progres kuartal, peringatan, min_payout efektif, payout_day_of_month efektif (info)
PATCH  /api/v1/affiliates/me/bank                data rekening (audit perubahan)
GET    /api/v1/affiliates/me/bindings            siswa binaan: nama pertama+inisial, tgl daftar, status ikatan, total belanja agregat
GET    /api/v1/affiliates/me/commissions         filter status/periode/kategori
GET    /api/v1/affiliates/me/ledger              riwayat mutasi + saldo
GET    /api/v1/affiliates/me/payouts             riwayat penarikan
POST   /api/v1/affiliates/me/payouts             ajukan (tolak bila status bukan active, di bawah min, atau dibekukan)
GET    /api/v1/affiliates/me/adjustments         riwayat penyesuaian + alasan
GET    /api/v1/affiliates/me/violations          riwayat pelanggaran + sanksi
POST   /api/v1/affiliates/me/violations/:id/appeal  ajukan banding
GET    /api/v1/affiliates/me/stats               klik, pendaftaran, konversi (FR-ATR-05/FR-DSH-08)
```

Super admin (grup `/admin`, `RBACMiddleware("affiliates:*")`):

```
GET    /admin/affiliates?status&level&q          daftar + antrean persetujuan (status filter)
POST   /admin/affiliates/:id/approval            {approve|reject, reason}
GET    /admin/affiliates/:id                     detail lengkap (profil, siswa, komisi, ledger, level history, pelanggaran)
PATCH  /admin/affiliates/:id/status              {status, reason} (suspend N hari via suspended_until)
POST   /admin/affiliates/:id/freeze              {days, reason} — batas maksimum dari setting
POST   /admin/affiliates/:id/level               {level|locked_until, reason}
POST   /admin/affiliates/:id/adjustments         {category, amount, reason, note} → langsung jadi entri ledger + audit
GET    /admin/affiliate-payouts                  antrean + filter
GET    /admin/affiliate-payouts/export           CSV transfer manual (pola csv_safe.go)
POST   /admin/affiliate-payouts/:id/decide       {paid + proof_url | reject + reason}
GET/PUT /admin/affiliate-settings                pengaturan global + per level + override produk; PUT menulis history + audit
GET    /admin/affiliate-settings/history         riwayat (siapa, kapan, lama, baru)
GET    /admin/affiliate-flags, POST /admin/affiliate-flags/:id/review
GET    /admin/affiliate-appeals, POST /admin/affiliate-appeals/:id/decide
GET    /admin/affiliate-reports                  komisi per periode/affiliator/kategori; liabilitas; kinerja
POST   /admin/affiliate-jobs/run                 rerun job idempoten (scope: bindings|commissions|levels|flags|releases)
```

Announcement: enum grup penerima + `affiliators`; `ListActiveUserEmails` menangani role `affiliator` (status aktif affiliator, bukan semua user).

### Frontend (web/)

| Permukaan | Isi |
|---|---|
| `app/r/[code]/page.tsx` | Publik: set cookie `aff_ref`, redirect `/` |
| `(auth)/register` | Kolom kode referral opsional (prefill dari cookie), kirim `referral_code` + `referral_code_cookie` di register dan verify |
| `(auth)/affiliate-register` | Form pendaftaran affiliator + Ketentuan Program (FR-REG-01/05) |
| `(student)` checkout | Input kode opsional (tampil bila memenuhi BR-ATR-05; validasi via endpoint publik) |
| `(affiliate)` portal baru | Dashboard: ringkasan, kode/link, siswa binaan, komisi (filter), ledger, penarikan, penyesuaian & sanksi, statistik, form banding (FR-DSH-01..09); layout role `affiliator` |
| `(admin)/admin/affiliates` | Daftar+persetujuan; detail bertab; payout antrean+export; settings (global/per level/override); flags; appeals; reports |
| Settings admin | Tiap baris komisi (global, level, override produk) punya pilihan Persen/Nominal; ambang penyesuaian + toggle tampil disabled dengan keterangan "Aktif setelah ada akun admin kedua"; `freeze_max_days` min 1; `payout_day_of_month` berlabel informasi |
| web/lib | `types.ts`, `api.ts`, hooks React Query per page-group; test vitest mengikuti `docs/standards/web-tests.md` |

### Privasi & keamanan

- Pembatasan affiliator di level query (repo memfilter `affiliator_id` dari JWT `user_id`), bukan hanya UI (BR-PRV-01).
- Payload siswa binaan tanpa email/HP/alamat/nilai ujian (BR-PRV-02); nama = nama pertama + inisial.
- Semua aksi admin → `audit_log` lewat `InsertAuditLogMeta` (approval, status, freeze, level, adjustment, payout, settings, violations, flags, appeals).
- Rate limit (`LoginRateLimiter` pattern) pada `/affiliates/clicks`, `/affiliates/public/validate`, dan `POST /auth/affiliators/register` (NFR-07).
- Log worker per pembuatan komisi/kegagalan job; counter metrics komisi dibuat/dibatalkan/payout (NFR-11).

## Acceptance and proof

| # | Kriteria | Bukti |
|---|---|---|
| A1 | Register siswa dengan kode manual/cookie menghasilkan binding `active` 3 bulan; manual menang atas cookie; klik lain saat terikat tidak mengubah binding; akun pre-launch tidak terikat | Integration test (Postgres testcontainer) di service binding |
| A2 | Order lunas → tepat 1 komisi per item (duplicate `OrderPaid` tetap 1); snapshot pct/level/base tersimpan; perubahan setting setelahnya tidak mengubah komisi lama | Unit + integration test worker; query `count(*)` |
| A3 | Refund: `pending` → `cancelled`; `available/paid` → entri ledger pengurang; `saldo == SUM(ledger)` setiap kasus | Integration test refund path |
| A4 | Masa tunggu: digital `paid_at + 7d` (setting), fisik `completed_at + physical_wait_days` (default 14d); job transisi `pending→available` + entri ledger; fisik tanpa `completed_at` tetap pending | Unit test job (fake clock) + integration |
| A5 | Perpanjangan ikatan saat pembelian ≥ `min_extend_amount`; `min_extend_amount=0` default | Integration test binding extension |
| A6 | Evaluasi level kuartalan: naik lompat, bertahan, turun setelah 2 kuartal gagal; rerun idempoten; manual override + lock | Unit test evaluator + rerun integration |
| A7 | Payout: tolak di bawah min & saat frozen; reject mengembalikan saldo; mark paid wajib bukti; export CSV | Handler + service test |
| A8 | Penyesuaian: langsung efektif (Fase 1), alasan wajib, tolak target diri sendiri, audit row tercipta | Service test |
| A9 | Broadcast `recipients=affiliators` hanya email akun role affiliator aktif | Service test announcement |
| A10 | Flag off: atribusi diabaikan, endpoint affiliator 403/404, checkout tanpa kode berjalan normal | Integration test gate |
| A11 | Affiliator hanya melihat datanya sendiri; payload siswa tanpa PII; nama pertama+inisial | Handler test authz + payload assertion |
| A12 | Siswa tidak melihat apa pun tentang ikatan (tidak ada endpoint/field baru untuk student) | Review diff + handler test |
| A13 | Komisi `fixed` = nominal × qty, dibatasi `base_amount`; `percent` floor; override produk > level > global; snapshot type/value tersimpan | Unit test kalkulator + integration worker |
| A14 | Rerun tiap scope job dua kali menghasilkan state sama (tanpa ledger/flag/history ganda, `warn_quarters` tidak bertambah dua kali) | Integration test per scope |
| A15 | Penarikan ditolak bila status bukan `active`; `freeze_max_days=0` ditolak; registrasi affiliator ditolak saat cap tercapai | Service + handler test |
| A16 | `binding_months` level dipakai saat ikatan dibuat/diperpanjang; NULL ikut global; ikatan lama tidak berubah | Integration test binding |

Strategi verifikasi (mengikuti `.agents/verification.md`): gate penuh backend `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh` (background, >10 menit) sebelum PR; loop fokus per paket saat build; frontend `npx tsc --noEmit && npx vitest run && npm run build` di `web/`; satu spec Playwright baru untuk alur referral (klik → register → binding muncul di dashboard affiliator) dijalankan lokal terhadap compose dev.

## Scope

**Non-goals Fase 1**: order manual; alur persetujuan kedua (kolom siap); pajak; payout otomatis; notifikasi otomatis (broadcast manual saja); bonus target; multi-level; aturan khusus per affiliator (tabel level-only Fase 1); fitur agen; refund parsial (platform hanya refund penuh — alokasi per item sudah disiapkan di ledger bila kelak ada); sistem poin/loyalty.

**Keputusan tertunda (nilai bisnis, tidak menghambat)**: angka komisi per kategori (placeholder), ambang level (placeholder), tanggal peluncuran & jadwal payout (Q-12), biaya transfer (Q-11), retensi data (Q-16), perlakuan saldo affiliator nonaktif permanen (Q-10 — default: saldo tetap dapat dicairkan, dikunci kebijakan sanksi).

**Residual risk**: satu akun super_admin mengelola dana (mitigasi audit log + rekonsiliasi); deteksi fraud hanya penandaan manual; cookie bisa hilang (kode manual & checkout sebagai cadangan).
