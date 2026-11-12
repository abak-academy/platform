---
title: Program Mitra (Affiliator v2) dan perubahan Program Affiliator
issue: none
status: draft
created: 2026-11-09
updated: 2026-11-10
sources:
  - docs/specs/affiliate-program.md
  - docs/standards/go-patterns.md
  - docs/standards/go-style-and-tests.md
  - docs/standards/web-tests.md
  - backend/internal/server/routes.go
  - backend/internal/handler/middleware.go
  - backend/internal/service/rbac.go
  - backend/internal/service/auth.go (Login, Refresh)
  - backend/internal/service/affiliate_register.go
  - backend/internal/service/affiliate_admin.go (DecideAffiliateApproval, ChangeAffiliateStatus)
  - backend/internal/service/affiliate_settings.go
  - backend/internal/service/system_config.go (UpdateSystemConfig)
  - backend/internal/service/store.go (Checkout, ValidatePromo, AdminRefundOrder, AdminCreatePromoCode)
  - backend/internal/service/bulk_exam_order.go
  - backend/internal/service/admin_students.go (registerStudent, ListStudents)
  - backend/internal/service/exam_revoke.go (revokeOrderExamRegistrationsTx)
  - backend/internal/worker/outbox.go (handleOrderPaid)
  - backend/internal/repository/promo.go, product.go, course.go, affiliator.go, affiliate_jobs.go
  - backend/db/migrations/0031_order_participant.up.sql, 0070_affiliate.up.sql
  - web/lib/nav-config.ts, web/components/shell/AppShell.tsx, web/app/(auth)/affiliate-register/page.tsx
---

# Program Mitra (Affiliator v2) dan perubahan Program Affiliator

## Problem dan outcome

Perantara (Mitra) perlu cara resmi untuk mendaftarkan siswa dan membelikan produk digital atas nama siswa dengan harga khusus, di bawah kendali admin. Outcome: peran `affiliator` yang sudah ada dapat mengikuti program **Mitra** selain (atau sebagai ganti) program **Affiliator**; Mitra mendaftarkan siswa, membeli `exam` dan `course` untuk siswa dari sekolah yang di-allow atau yang ia daftarkan, memakai voucher khusus Mitra, dan membayar lewat Snap sebagai pembeli. Tidak ada komisi, saldo, atau pencairan untuk Mitra. Program Affiliator juga dirapikan: default kosong dan saklar yang butuh pengaturan wajib terisi.

Dokumen ini memegang aturan bisnis dan kontrak teknis; tidak ada BRD terpisah (BRD v0.5 dihapus, riwayatnya ada di git).

## Agreed behavior

### Alur utama

1. Calon Mitra mendaftar lewat form `/affiliate-register` yang sudah ada dengan satu pilihan program; akun `role=affiliator` status `pending_verification` → OTP → `active`, profil `pending_approval`. Admin dapat juga membuat akun langsung.
2. Super admin menyetujui (menentukan program, mengatur sekolah yang di-allow) atau menolak dengan alasan.
3. Admin mengatur akses produk di form produk dan membuat voucher Mitra di halaman promo.
4. Mitra mendaftarkan siswa (`users.registered_by` = Mitra).
5. Mitra memilih produk dan siswa, memasukkan voucher, melihat pratinjau, membuat order, membayar lewat Snap.
6. `OrderPaid` memberi akses ke setiap peserta (ujian: sudah ada; kelas: ditambah).
7. Refund per order penuh mencabut akses semua peserta.

### Keputusan teknis (temuan saat membaca kode)

| # | Temuan di `main` | Keputusan |
|---|---|---|
| 1 | `promo_code.affiliator_only` sudah dipakai Program Affiliator dengan arti "promo non-publik yang ditampilkan ke affiliator untuk dibagikan ke siswa" (`ListAffiliatorOnlyPromos`, `AffiliateMePromos`). Siswa memakainya di checkout dengan kodenya. `ValidatePromo` tidak memeriksa role | Voucher Mitra **tidak** memakai `affiliator_only`. Penandanya `promo_code.partner IS NOT NULL`. `affiliator_only` tidak berubah |
| 2 | Keikutsertaan program butuh dua penanda (Affiliator dan Mitra) plus sekolah. Satu kolom `partner` tidak cukup menandai "bukan Affiliator" | Satu kolom `affiliators.programs` JSONB: NULL berarti Affiliator saja (kompatibel dengan data yang ada, tanpa backfill) |
| 3 | Program tidak dicabut dari akun | Hanya penambahan program; menghentikan akses lewat status akun (ditangguhkan atau nonaktif). Penambahan dikonfirmasi admin dan UI menyatakan program tidak bisa dicabut |

### Failure behavior

- Akun belum terdaftar, menunggu, atau ditolak di sebuah program → semua endpoint program itu 403 dengan kode berbeda (`program_not_enrolled`, `program_pending`, `program_rejected`).
- Akun nonaktif atau saklar program mati → GET tetap jalan, semua tulis 403 (`account_deactivated`, `partner_program_off`).
- Siswa di luar sekolah yang di-allow dan bukan yang didaftarkan Mitra → 403 `student_not_allowed` untuk seluruh permintaan (tidak ada order sebagian).
- Produk tidak tampil untuk Mitra → 403 `product_not_allowed` di pratinjau, pembuatan order, dan checkout.
- Voucher bukan voucher Mitra, bukan untuk Mitra ini, kedaluwarsa, atau kuota habis → 422 `invalid_promo`. Pada checkout, bila voucher berubah sejak order dibuat → 422 sebelum pembayaran dibuat.
- Sisa kuota voucher tidak cukup untuk order (satuan siswa: kursi lebih banyak dari sisa) → 422 `promo_quota_insufficient` dengan `remaining`; tidak ada order parsial.
- Baris CSV siswa yang gagal (email sudah dipakai, sekolah tidak ditemukan, data tidak valid) tidak menggagalkan baris lain; kegagalan per baris tercatat di laporan hasil. Job berstatus `failed` hanya bila semua baris gagal (perilaku yang ada).
- Kuota voucher habis tepat saat checkout bersamaan → satu checkout menang, yang lain 422 `invalid_promo` dan transaksinya dibatalkan seluruhnya.
- Semua siswa permintaan sudah memiliki produk → 422 `zero_net_new_participants`. Sebagian sudah memiliki → dikeluarkan dan dilaporkan di pratinjau.
- Order di bawah `min_students` siswa pada voucher yang mensyaratkannya → 422 `promo_min_students` dengan `min`.
- Akun ditangguhkan, program Mitra mati, atau pendaftaran belum disetujui → `GET /partners/me/vouchers` dan `/products` mengembalikan daftar kosong (bukan voucher yang tidak bisa dipakai), dan hitungan dashboard ikut nol.
- Saklar Affiliator dinyalakan dengan pengaturan wajib kosong → 422 `affiliate_settings_incomplete` dengan daftar `missing`.
- Email pendaftar sudah terdaftar → `email_taken` (kecuali akun affiliator `pending_verification`, yang dilanjutkan ke OTP seperti sekarang).
- Refund order Mitra → status `cancelled`/`refunded`, akses semua peserta `revoked`; tidak ada transaksi komisi yang dibuat atau diubah.

### Compatibility & rollout

- Dua rilis. Rilis 1: migrasi `0072` (kolom baru, indeks, pengaturan wajib jadi nullable) dan kode yang berhenti membaca `affiliate_settings.daily_registration_cap`. Rilis 2: migrasi `0073` menghapus kolom itu. Alasan: migrasi berjalan saat API start dan pod lama masih men-`SELECT` kolom itu selama deploy bertahap.
- Nomor migrasi `0072`/`0073` terverifikasi (2026-11-09): `0071_exam_event_info` adalah yang terakhir di `origin/main` dan tidak ada branch remote yang memakai `0072` atau lebih. Cek ulang saat membuat branch. Runner (`infra/migrate.go`) mengurutkan file `*.up.sql` dan menjalankan tiap file dalam satu transaksi, sehingga `CREATE INDEX CONCURRENTLY` tidak bisa dipakai.
- Program Affiliator belum berjalan di production (hanya data uji pemilik produk), jadi baris `affiliate_settings` yang ada tidak ditimpa oleh migrasi dan tidak ada backfill `programs` (NULL = Affiliator saja).
- Saklar Mitra baru `partner_enabled` di `system_config`, default mati. Saklar Affiliator tidak berubah selain pemeriksaan pengaturan wajib saat dinyalakan.
- URL dashboard Affiliator pindah dari `/affiliate` ke `/affiliate/overview`; `/affiliate` menjadi halaman landing (lihat Frontend). Link atau e2e yang menunjuk `/affiliate` perlu disesuaikan.
- Perubahan perilaku pada kode yang sudah ada (di luar fitur) tercantum di "Perubahan kode yang ada" dan harus disebut di badan PR.

## Technical contract

### Model data (migrasi `0072_partner_program`)

Tidak ada tabel baru. Kolom baru:

| Kolom | Tipe | Isi |
|---|---|---|
| `users.registered_by` | `UUID NULL REFERENCES users(id)` | Mitra yang mendaftarkan siswa |
| `affiliators.programs` | `JSONB NULL` | NULL = `{"affiliate": true}`. Bentuk: `{"affiliate": bool, "partner": {"school_ids": [uuid]}}`. Kunci `partner` ada berarti Mitra. `affiliate` yang `false` berarti bukan Affiliator |
| `product.partner_access` | `JSONB NULL` | NULL = `none`. Bentuk: `{"scope": "all"}` atau `{"scope": "selected", "ids": [uuid]}` |
| `promo_code.partner` | `JSONB NULL` | NULL = bukan voucher Mitra. Bentuk: `{"ids": [uuid], "quota_unit": "order" \| "student", "min_students": int}`. `ids` kosong = semua Mitra; berisi id = Mitra terpilih. `quota_unit` wajib, tanpa nilai bawaan. `min_students` opsional (0 atau tidak ada = tanpa minimum): jumlah siswa minimum dalam satu order untuk memakai voucher |

Indeks: `CREATE INDEX IF NOT EXISTS idx_users_registered_by ON users (registered_by) WHERE registered_by IS NOT NULL` (biasa, bukan `CONCURRENTLY`, karena runner membungkus tiap migrasi dalam transaksi). Indeks parsial membuat pembuatannya murah selama hampir semua baris `users` bernilai NULL.

`affiliate_settings`: `cookie_days`, `binding_months`, `digital_wait_days`, `physical_wait_days`, `min_payout`, `eval_day` menjadi `NULL` tanpa `DEFAULT`. `min_extend_amount` tetap `DEFAULT 0` (0 sudah berarti semua pembelian). `0073` men-`DROP COLUMN daily_registration_cap`. Down migrasi dibuat untuk keduanya mengikuti pola `0070`/`0071`.

Tidak ada FK pada elemen array/JSONB; id sekolah, Mitra, dan produk divalidasi di service (keberadaan; sekolah `active`; akun role `affiliator` dengan kunci `partner`). Id yang kemudian terhapus atau nonaktif tidak merusak apa pun: array hanya menyaring.

`model` (`internal/model/`): `Affiliator` menambah `Programs`; `Product` menambah `PartnerAccess`; `PromoCode` menambah `Partner *PartnerVoucher` (`IDs []string`, `QuotaUnit string`); `User` menambah `RegisteredBy`; `AffiliateSettings` field pengaturan wajib menjadi pointer.

### Role, middleware, flag

- Role tetap `affiliator`; `roleCapabilities[RoleAffiliator]` tetap kosong. Mitra **tidak** diberi `students:*` karena itu akan membuka rute admin siswa untuk semua affiliator (termasuk Affiliator saja). Mitra memakai rute `/partners/me/*` sendiri.
- `ProgramStates(ctx, userID)` (service): satu query `affiliators` JOIN `users`, mengembalikan untuk tiap program salah satu dari `not_enrolled | pending | rejected | active`, ditambah `account_active` (`users.status = 'active'`), `rejection_reason`, dan daftar sekolah Mitra. Aturannya: program diikuti bila kunci ada di `programs` (NULL = Affiliator); `program_status` `pending_approval`/`rejected` berlaku untuk program yang diminta; `active` aktif.
- `PartnerGate(svc)` (handler, pola `AffiliatorOnlyMiddleware`): memeriksa role, memuat `ProgramStates`, menolak sesuai Failure behavior. Pada method non-baca juga menolak bila `!account_active` atau `partner_enabled` mati.
- `AffiliateGate(svc)`: sama untuk `/affiliates/me/*` (Affiliator). Perilaku saat saklar Affiliator mati tidak berubah (baca saja, `affiliate_program_off` untuk payout dan bank).
- `AffiliatorWriteGuard`: dipasang di `/partners/me`, `/affiliates/me`. Menolak method tulis akun nonaktif dengan `403 account_deactivated`. Rute `/auth/password/change` dan `/auth/photo` tidak dikenai guard (identitas diri sendiri, bukan tindakan bisnis). Query status hanya untuk role `affiliator`.
- `/orders/*` (rute pembelian siswa) ditutup untuk role `affiliator` dengan `ForbidRoles(RoleAffiliator)`. Mitra hanya membeli lewat `/partners/me/orders/*`. Ini menutup jalur pintas yang melewati validasi Mitra (checkout dan retry generik).
- `canHoldSession(user)` (service, `auth.go`): `active`, atau `deactivated` untuk role `affiliator`. Dipakai di `Login` (`auth.go:129`), `Refresh` (`auth.go:442`), dan login Google (`google.go:56`). Token dikeluarkan seperti biasa; tidak ada sesi yang dicabut saat penangguhan.
- Saklar: kunci `partner_enabled` (bool) ditambahkan ke `configKeyCatalog` di `system_config.go` (di samping `affiliate_enabled`, baris 54).

### Status akun dan penangguhan

`ChangeAffiliateStatus` menulis `affiliators.program_status` (seperti sekarang, dibaca jalur uang v1) **dan** `users.status` dalam transaksi yang sama: `suspended`/`deactivated` → `users.status = 'deactivated'`; `active` → `'active'`. `users.status` adalah sumber kebenaran untuk gerbang Mitra dan login; `program_status` tetap dicerminkan hanya untuk query v1. Penonaktifan lewat halaman Akun (`AdminChangeAccountStatus`) mencabut sesi (perilaku yang ada); setelah login ulang akun tetap hanya-baca.

### Registrasi dan persetujuan

- `POST /auth/affiliators/register`: body menambah `program: "affiliate" | "partner"` (default `affiliate` agar klien lama tetap jalan). `RegisterAffiliator` menulis `programs` sesuai pilihan lewat `CreateAffiliatorAccount(user, termsVersion, programs)`. Gerbang `affiliateEnabled` diganti: pendaftaran diterima bila saklar program yang dipilih menyala. Pemeriksaan `daily_registration_cap` dan `ErrAffiliateRegistrationCap` dihapus.
- `GET /affiliates/public/status`: respons `{enabled, partner_enabled}` (`enabled` tetap untuk klien lama).
- `POST /admin/affiliates/:id/approval` (approve): body menambah opsional `programs` untuk mengubah pilihan pendaftar. Kode referral dibuat hanya bila `affiliate` true; `ApproveAffiliatorTx` menerima kode kosong untuk Mitra saja (`referral_code` sudah nullable dan `UNIQUE`).
- `POST /admin/affiliates`: body menambah `programs`; akun aktif tanpa persetujuan. Pembuatan massal (`/bulk`) tidak berubah dan membuat Affiliator saja.
- `POST /admin/affiliates/:id/programs` `{program: "affiliate" | "partner"}`: **menambah** program ke akun aktif. Tidak ada pencabutan; program yang sudah ada diabaikan (idempoten). Menambah `affiliate` ke akun yang belum punya kode referral membuat kodenya (`generateUniqueAffiliateCode`) dan menyetel `programs.affiliate = true`. Menambah `partner` membuat kunci `partner` dengan `school_ids` kosong. Penghentian akses dilakukan dengan penangguhan akun.
- `PUT /admin/affiliates/:id/partner/schools` `{school_ids: []}`: mengganti daftar sekolah. Validasi: sekolah ada dan `active`; akun memiliki kunci `partner`.
- `GET /admin/affiliates?program=affiliate|partner` memfilter; baris daftar membawa `programs`.
- Semua rute admin di atas di bawah `affiliates:*` (hanya `super_admin`) dan menulis audit log: `affiliate.approval` (meta menambah `programs`), `affiliate.status_change`, `affiliate.programs_change`, `partner.schools_set`.
- Pemisahan dari v1: `ListAffiliatorsForLevelEval` dan penerima broadcast `affiliators` (`announcement.go`) menambah `AND (a.programs IS NULL OR (a.programs->>'affiliate')::boolean IS NOT FALSE)`. Atribusi tidak terpengaruh: `GetAffiliatorByCode` butuh `referral_code`, yang tidak dimiliki Mitra saja.

### Siswa (Mitra)

- `POST /partners/me/students` memanggil `registerStudent` (kode yang sudah ada) dengan `actorID` Mitra, sekolah opsional dan bebas (sekolah harus ada dan `active` bila diisi), dan `registeredBy`. `registerStudent` mendapat parameter `registeredBy *string` yang diteruskan ke `CreateStudent` (kolom `registered_by`). Mitra tidak menentukan kata sandi: tanpa kata sandi dari klien, `registerStudent` membuat kata sandi sementara yang dikembalikan sekali di `temp_password` dan akun dibuat dengan `password_temporary = true` (perilaku yang ada; `RegisterStudentWithPassword` tetap hanya untuk `CanManageAllStudents`). Permintaan Mitra yang membawa `password` ditolak 400.
- `GET /partners/me/students`: filter `q`, `school_id`, `status`, `registered_by_me`. Dua mode: tanpa `page`/`per_page` memakai kursor (pemilih siswa di halaman beli); dengan `page`/`per_page` mengembalikan `{data, page, per_page, total, hint?}` (tabel siswa). `StudentFilter` menambah `PartnerScope{SchoolIDs []string, RegisteredBy string}`; SQL: `(school_id = ANY($1) OR registered_by = $2)`. Mitra tanpa sekolah dan tanpa siswa terdaftar-oleh-dia menerima daftar kosong dan `hint: "no_schools"` untuk pesan UI.
- Kolom daftar lewat DTO sendiri `PartnerStudentResponse` (bukan `StudentResponse`, yang memuat email): `id`, `name`, `username`, `school_name`, `grade`, `jenjang`, `status`, `activated` (= `NOT password_temporary`), `registered_by_me`. Tidak ada email, HP, tanggal lahir, alamat, nilai, atau kredensial. Rute `/admin/students/*` tidak dibuka untuk Mitra.
- `POST /partners/me/students/:id/credentials`: menerbitkan ulang kata sandi sementara. Tidak memakai `ReissueStudentCredentials` yang ada karena pembatasannya hanya `school_id` (siswa yang didaftarkan Mitra bisa tanpa sekolah) dan tidak memeriksa apakah siswa sudah mengganti kata sandi. Metode repo baru `ReissueStudentCredentialsByRegistrar(id, partnerID, hash)`: `UPDATE users SET password_hash = $1, password_temporary = true, updated_at = now() WHERE id = $2 AND role = 'student' AND status = 'active' AND registered_by = $3 AND password_temporary = true` dan periksa `RowsAffected`, sehingga cek dan tulis atomik (tanpa jendela balapan dengan siswa yang sedang mengganti kata sandi). 0 baris: siswa tidak ada atau bukan milik Mitra → 404 `student_not_found`; siswa milik Mitra tetapi sudah mengganti kata sandi → 409 `student_already_activated` (dibedakan lewat satu SELECT sebelum UPDATE). Sukses: `revokeRefreshSessions`, audit `student.reissue_credentials` dengan meta `{"by": "partner"}`, respons `{username, temp_password}`.
- Impor CSV, memakai job `student_bulk` yang ada:
  - `GET /partners/me/students/bulk/presign` dan `POST /partners/me/students/bulk` meniru `AdminPresignStudentBulkUpload`/`AdminBulkImportStudents`. Folder penyimpanan = id Mitra (`student-bulk/{partnerID}/...`), bukan UUID acak seperti admin global, sehingga `EnqueueStudentBulkJob` (yang memeriksa awalan `student-bulk/{schoolID}/`) hanya menerima berkas dari folder Mitra itu sendiri.
  - Parser: `enqueueStudentBulkJobFromData` dan worker (`runStudentBulkJob`) saat ini mewajibkan identitas sekolah per baris bila pengunggah tidak terikat sekolah (`ParseStudentBulkCSVForWorker(data, schoolBound == nil)`). Untuk Mitra identitas sekolah opsional: parser dipanggil dengan `requireSchoolIdentity = false`, dan baris tanpa sekolah memanggil `registerStudent` dengan sekolah kosong. Baris dengan NPSN/kode sekolah diselesaikan seperti jalur role global (sekolah mana pun, tidak dibatasi).
  - `ProcessStudentBulkRows` menerima `registeredBy *string`, diisi id Mitra dan diteruskan ke `registerStudent`. Kolom `password` di CSV sudah ditolak untuk role non-global (`r.Password != nil && !CanManageAllStudents(actorRole)`), jadi Mitra otomatis tidak bisa menentukan kata sandi. Templat CSV di `web/lib/bulk-import-format.ts` untuk Mitra tanpa kolom `password`.
  - Laporan hasil (`BuildStudentBulkResultCSV`) memuat `temp_password` per baris berhasil. Tautan unduhan diperoleh dari `GET /partners/me/jobs/:id` (membungkus `GetJobStatus(jobID, claims.Sub)`, yang hanya mengembalikan job milik pemanggil dan menandatangani URL 15 menit). `PartnerGate` melindungi rute itu; `/admin/jobs/:id` tidak dibuka untuk alur Mitra di UI.
  - Residual: berkas hasil CSV berisi kata sandi sementara tetap tersimpan di bucket privat setelah dibuat (hanya berkas masukan yang dihapus worker). Ini perilaku yang sudah ada untuk admin; tidak diubah di sini, dicatat sebagai risiko sisa.

### Produk dan akses (Mitra)

- `PUT /admin/products/:id/partner-access` `{scope: "none|all|selected", ids: []}` (gerbang `affiliates:*`). Endpoint tersendiri agar `UpdateProduct` (daftar kolom eksplisit) tidak menimpa `partner_access` dan sebaliknya. `none` menyimpan NULL. `ids` divalidasi (akun Mitra); `selected` wajib `ids` tidak kosong. Audit: `product.partner_access_set`.
- `GET /partners/me/schools?q=&status=&page=&per_page=`: sekolah yang di-allow admin untuk Mitra (`{data, page, per_page, total}`; pencarian nama, NPSN, atau kode). Dipakai halaman Sekolah dan menjadi acuan NPSN/kode di impor CSV.
- `GET /partners/me/products`: `exam` dan `course` terbit, terlihat siswa (ujian memakai `OrderableExamOnly`, yaitu ujian yang jendelanya masih terbuka), dan lolos akses (`scope = 'all'` atau `scope = 'selected'` dan `ids @> to_jsonb(<partnerID>::text)`). Memakai `ProductFilter` yang ada ditambah kondisi `PartnerID`.
- Cek akses yang sama dijalankan di `planPartnerOrder` (pratinjau dan pembuatan) dan `CheckoutPartnerOrder`.

### Voucher

- `AdminCreatePromoCode` dan `AdminUpdatePromoCode`: body menambah `partner_voucher` (objek opsional `{enabled: bool, ids: [], quota_unit: "order"|"student"}`; tidak dikirim berarti tidak berubah pada update). `enabled` mengisi `promo_code.partner`; tidak enabled mengosongkan jadi NULL. `quota_unit` wajib bila `enabled` (422 `validation_error` bila kosong atau di luar dua nilai). Voucher Mitra memaksa `is_public = false` dan tidak boleh bersama `affiliator_only` (422 `validation_error`). Validasi: tiap id adalah akun Mitra.
- `ValidatePromo` (generik, dipakai `/promo-codes/validate` dan `PatchCart`): bila `partner IS NOT NULL` → `ErrInvalidPromo`. Siswa tidak pernah dapat memakai voucher Mitra, walau kodenya diketahui.
- `ValidatePartnerPromo(ctx, partnerID, code, subtotal, seats)`: voucher harus voucher Mitra, `partner.ids` kosong atau memuat `partnerID`, serta aturan yang sama dengan `ValidatePromo` (kedaluwarsa, kuota, minimum, diskon). Bila `partner.min_students` > 0 dan jumlah kursi lebih sedikit → `promo_min_students`, diperiksa sebelum kuota. Jalur Mitra menerima voucher Mitra saja (asumsi: promo publik biasa tetap hanya untuk siswa). Satuan kuota:
  - `order`: perilaku `evaluatePromo` yang ada. Pemakaian = 1; diskon dan `max_discount_amount` untuk seluruh subtotal.
  - `student`: pemakaian = `seats`; wajib `max_uses IS NULL OR used_count + seats <= max_uses` (kalau tidak `promo_quota_insufficient` dengan `remaining`). Diskon dihitung per kursi dari harga satuan (persen dikenai `max_discount_amount`; nominal dibatasi harga satuan) lalu dikali `seats`. `min_order_amount` tetap terhadap subtotal order pada kedua satuan.
- `GET /partners/me/vouchers`: voucher Mitra yang masih berlaku dan memuat Mitra ini (atau kosong); kosong bila akun tidak bisa membeli (lihat Failure behavior). Kuota dipotong saat checkout, bukan saat order dibuat, dan tidak dikembalikan saat refund. Terpisah dari `GET /affiliates/me/promos` (v1), yang tetap mengembalikan promo `affiliator_only` untuk dibagikan ke siswa. Voucher Mitra tidak pernah muncul di daftar v1 karena `affiliator_only = false`, dan promo v1 tidak muncul di daftar Mitra karena `partner IS NULL`. Jalur order Mitra menolak promo yang bukan voucher Mitra. Tidak ada penyaringan produk; halaman promo admin memperingatkan voucher yang tidak bisa dipakai karena akses produk, dihitung di klien dari data yang sudah ada.
- `ListActivePublicPromos` tidak berubah (Mitra `is_public = false`). `ListAffiliatorOnlyPromos` tidak berubah.

### Order Mitra

Semua di `/partners/me/orders*`, gerbang `PartnerGate`:

| Endpoint | Fungsi |
|---|---|
| `POST /orders/preview` | `{product_id, student_ids, promo_code?}` → `{net_new_count, excluded[], students[], unit_price, subtotal, discount, total}` |
| `POST /orders` | Membuat order `cart` dengan satu item (qty = jumlah siswa bersih) dan baris `order_participant` |
| `POST /orders/:id/checkout` | `CheckoutPartnerOrder` |
| `POST /orders/:id/retry` | Retry pembayaran, tervalidasi sebagai pemilik |
| `GET /orders?q=&status=&from=&to=&page=&per_page=` | Daftar bernomor halaman milik Mitra (`{data, page, per_page, total}`), tanpa cart dari order lain; `q` mencocokkan produk, siswa, atau akhiran id order; `from`/`to` hari pembuatan (`to` inklusif); `status` salah satu status order |
| `GET /orders/:id` | Detail milik Mitra: item, peserta, dan `promo_code` (kode voucher yang dipakai) |
| `GET /dashboard` | Hitungan beranda: siswa terdaftar, order lunas dan belum dibayar, voucher tersedia, produk terbuka |

`planPartnerOrder(partnerID, productID, studentIDs, promo)` (dipakai preview dan create, tidak pernah mempercayai pratinjau lama):
1. `studentIDs` tidak kosong; tidak duplikat (`duplicate_participant`).
2. Produk ada, terbit, tipe `exam` atau `course` (`partner_product_type`), lolos akses produk.
3. `ListPartnerOrderStudents(ids, allowedSchoolIDs, partnerID)`: `role='student' AND status='active' AND (school_id = ANY(allowed) OR registered_by = partner)`. Jumlah baris harus sama dengan jumlah id, kalau tidak 403 `student_not_allowed`. Daftar sekolah dibaca dari DB tiap permintaan, bukan dari token.
4. Siswa yang sudah memiliki produk dikeluarkan. Untuk ujian: `FilterAlreadyRegistered` (sudah ada, mengabaikan registrasi `revoked`). Untuk kelas: `FilterAlreadyEnrolled` **tidak ada di `main`** dan ditambahkan di `repository/course.go` (`course_session` berstatus `active`; indeks unik `idx_course_session_student_course` bersifat parsial `WHERE status = 'active'`, jadi sesi `revoked` tidak menghalangi pembelian ulang setelah refund, terverifikasi di `0005_course.up.sql`).
5. Subtotal = `price × jumlah bersih`; voucher via `ValidatePartnerPromo`.

`CreatePartnerOrder`: satu transaksi: `CreateOrderTx(partnerID)`, `InsertOrderItemTx`, `InsertOrderParticipantsTx`, `SetOrderPromoTx(orderID, promoID, discount)` (metode repo baru: `UPDATE orders SET promo_code_id, discount, total = subtotal - discount + shipping_cost WHERE id AND status='cart'`). Satu cart terbuka per pembeli (`idx_orders_student_cart`): bila ada cart lama dengan peserta, dibatalkan (`cancelled`/`superseded`); cart lama tanpa peserta tidak mungkin ada karena `/orders` tertutup untuk affiliator.

`CheckoutPartnerOrder(partnerID, orderID, key)`: memastikan pemilik, lalu validasi ulang dalam jalur yang sama: akun aktif, saklar menyala, akses produk, siswa (sekolah/pendaftar) masih lolos, voucher (`ValidatePartnerPromo` terhadap `promo_code_id` order: kedaluwarsa, kuota, dan `discount` tidak berubah), **kepemilikan peserta**, baru memanggil `Checkout` (kode referral kosong). Menggantikan validasi yang di PR #238 bisa dilewati lewat rute generik.

Pengecekan kepemilikan dilakukan di sini karena `Checkout` melewatkan `checkDigitalTargets` untuk order berpeserta (`hasDigital && len(participants) == 0`, `store.go`) dan `recheckDigitalTargets` juga membiarkan order berpeserta ("bulk orders are left to their own flow"). Tanpa ini dua order untuk siswa dan produk yang sama (dari dua Mitra, atau Mitra dan siswa sendiri) bisa sama-sama dibayar. Peserta yang sudah memiliki → 409 `participant_already_owns` dengan daftar siswa. Sisa risiko: dua pembayaran yang sudah berjalan bersamaan sebelum salah satu lunas (sama seperti order massal admin sekolah sekarang).

### Perubahan kode yang ada

Semua ini mengubah perilaku di luar fitur dan harus disebut di badan PR.

1. **Checkout menolak digital qty > 1** (`store.go`, loop `ValidateItemQty` sebelum pemuatan peserta). Orde peserta memakai qty sebagai jumlah kursi, jadi pemeriksaan dipindah setelah `GetOrderParticipants` dan dilewati bila ada baris peserta. Ini juga memengaruhi pesanan ujian massal admin sekolah (`CreateBulkExamOrder`, qty = jumlah peserta) yang menurut pembacaan kode kena aturan ini; belum dijalankan.
2. **Fan-out kelas** (`worker/outbox.go`, cabang `course`): sekarang hanya mendaftarkan `order.StudentID`. Ditambah mengikuti cabang `exam`: bila ada baris `order_participant`, daftarkan tiap peserta (`order_id` terisi di tiap `course_session` agar `RevokeEnrollmentsByOrder` mencabutnya).
3. **Kuota voucher** (`IncrementPromoUsesTx`): menerima jumlah pemakaian `n`: `UPDATE promo_code SET used_count = used_count + $2 WHERE id = $1 AND (max_uses IS NULL OR used_count + $2 <= max_uses)` dan periksa `RowsAffected = 1`; bila 0 → galat kuota (`ErrInvalidPromo` untuk voucher biasa, `promo_quota_insufficient` untuk voucher Mitra satuan siswa) dan transaksi checkout dibatalkan. `Checkout` menentukan `n`: jumlah kursi (total qty item) bila voucher Mitra berkuota `student`, selain itu 1; untuk itu `Checkout` memuat promo lewat `GetPromoByID` (metode repo baru). Pemanggilnya terverifikasi hanya dua, keduanya di dalam `Checkout` (`store.go:1234` jalur gratis, `store.go:1260` jalur bayar), jadi berlaku juga untuk checkout siswa (dengan `n = 1`, perilaku sama selain batas atas). `IncrementPromoUses` (tanpa Tx) tidak punya pemanggil di kode produksi dan tidak diubah. **Tanda tangan `IncrementPromoUsesTx` berubah** (tambah `n int`): interface repo, stub di `backend/integration/`, dan test double (`promo_test.go`, `store_test.go`) disesuaikan.
4. **`ValidatePromo`** menolak voucher Mitra (lihat Voucher).
5. **Login/Refresh/Google** mengizinkan `deactivated` untuk role `affiliator` (`canHoldSession`).
6. **`AdminRefundOrder`** tidak diubah. Hanya diverifikasi bahwa `settleAffiliateRefundTx` tidak membuat apa pun untuk order Mitra (tidak ada transaksi komisi) dan `revokeOrderExamRegistrationsTx`/`RevokeEnrollmentsByOrder` mencabut semua peserta.
7. **Default Program Affiliator** (`affiliate_settings.go`): `affiliateDefaultCommissionRule` menjadi kosong (`resolveAffiliateCommissionRule` jatuh ke `{percent, 0}`), `affiliateDefaultRiseThreshold` dihapus (`affiliateRiseThreshold` sudah mengembalikan `MaxFloat64` untuk nil), `defaultAffiliateSettings()` mengembalikan struct tanpa nilai (hanya `ID`, `MinExtendAmount = 0`). Kode pemakai nilai pengaturan wajib menangani nil; `buildAffiliateEffectiveRules` tidak lagi memasukkan angka bawaan untuk masa tunggu, ikatan, payout.
8. **Saklar Affiliator**: `UpdateSystemConfig` (titik tunggal; dipakai rute config umum dan `SetAffiliateProgramEnabled`, `system_config.go:340`) menolak perubahan `affiliate_enabled` false→true bila ada pengaturan wajib kosong: `launch_date`, `cookie_days`, `binding_months`, `digital_wait_days`, `physical_wait_days`, `min_payout`, `eval_day`. Galat `affiliate_settings_incomplete` dengan `missing: [...]`. `PUT /admin/affiliate-settings` menolak mengosongkan pengaturan wajib selagi program menyala. `GET /admin/affiliate-settings` mengembalikan nilai apa adanya (null) dan flag `commission_unset` bila tidak ada aturan komisi (peringatan non-blokir di UI).
9. **Pendaftaran Affiliator**: pemeriksaan `daily_registration_cap` dihapus dari `RegisterAffiliator`; `CountAffiliatorsCreatedSince` dan field `DailyRegistrationCap` dihapus; bagian terkait di halaman pengaturan, `docs/specs/affiliate-program.md`, disesuaikan.

### Frontend (web/)

- Registrasi `(auth)/affiliate-register`: dua kartu program dengan keterangan, hanya program yang `enabled`/`partner_enabled`; satu pilihan terkirim sebagai `program`. Bagian lain form (nama, email, HP, kata sandi, syarat, OTP) tidak berubah. Syarat program yang sama untuk kedua program, dengan teks diperbaiki (kunci i18n `aff_terms_*`): kunci i18n `aff_terms_1` sampai `aff_terms_8` diganti set baru (umum, Affiliator, Mitra) untuk `id` dan `en`, judul `/affiliate-terms` menjadi "Ketentuan Program Affiliator dan Mitra" (`aff_terms_title`, tautan `aff_reg_terms_link`), dan `AFFILIATE_TERMS_VERSION` naik dari `2026-10` ke `2026-11`. `terms_version` yang sudah tersimpan tidak diubah; hanya pendaftar baru menyimpan versi baru. Akun yang dibuat admin tetap bertanda `admin_created` dan tidak melewati persetujuan teks. Draf teks perlu ditinjau pemilik produk sebelum dirilis.
- `GET /auth/me` untuk role `affiliator` menambah `affiliator_programs`: `{affiliate: state, partner: state, account_active, rejection_reason}` (dari `ProgramStates`). Menu dan landing memakai ini; tidak ada panggilan baru.
- `lib/nav-config.ts`: `AFFILIATOR_NAV` menjadi grup bertajuk Affiliator (Ringkasan, Siswa binaan, Komisi, Voucher siswa, Saldo, Penyesuaian), grup Mitra (Ringkasan, Sekolah, Siswa, Pembelian, Voucher), dan Akun di luar grup. Tiap grup punya `program`; `AppSidebar` menyembunyikan grup program yang `not_enrolled` menurut `affiliator_programs` (keadaan belum diketahui menampilkan keduanya; menunggu atau ditolak tetap tampil karena halamannya menjelaskan).
- Halaman Mitra di `app/(affiliate)/affiliate/partner/*`: `page` (ringkasan dengan kartu angka dan pintasan beli, kelola siswa, riwayat), `schools`, `students`, `orders`, `orders/[id]` (rincian: produk, siswa, kode voucher, subtotal, diskon, total, tombol bayar untuk order menunggu pembayaran), `orders/new`, `vouchers` (potongan, maksimal potongan, minimal pembelian, minimal siswa, satuan kuota dengan penjelasan, sisa kuota, berlaku untuk siapa, sampai kapan).
- Daftar sekolah, siswa, dan pembelian memakai pola yang sama dengan "Siswa binaan" Affiliator: `ListToolbar` (pencarian, popover filter, rentang tanggal, jumlah per halaman), `AffiliateList` dengan ringkasan "x–y dari total" dan nomor halaman, state di URL (`useListParams`, parameter `school` ditambahkan untuk filter sekolah). Tombol aksi (Unduh CSV, Impor, Daftarkan, Buat pembelian) berada di baris rata kanan di bawah judul dan keterangan, di atas baris pencarian/filter. Siswa memiliki kolom "Didaftarkan oleh" dan filter sumber, sekolah, dan status akun.
- Unduh CSV (sekolah, siswa, pembelian) dibuat di browser dari semua halaman yang cocok dengan filter (`fetchAllPartnerPages`, 100 baris per permintaan); sel yang diawali `=`, `+`, `-`, `@` diberi apostrof (`lib/csv.ts`).
- Pembayaran memakai `SnapCheckout` dengan base path `/partners/me/orders` dan `invalidateKey` ke cache Mitra; order menunggu pembayaran memakai `RetryPaymentButton` (`POST /partners/me/orders/:id/retry`).
- Komponen `ProgramGate` membungkus halaman tiap program dan menampilkan pesan per keadaan dengan warna: biru belum terdaftar, kuning menunggu, merah ditolak (dengan alasan); akun nonaktif hanya-baca dan saklar mati tampil sebagai banner. `RouteProgramGate` memberi key eksplisit pada anak yang diserahkan router agar React tidak memperingatkan key hilang.
- Landing `/affiliate`: satu program → redirect ke programnya; dua program → ringkasan dua kartu. Kartu Affiliator memakai `GET /affiliates/me/stats`; kartu Mitra memakai `GET /partners/me/dashboard`. Program yang bukan `active` menampilkan pesan keadaan dari `affiliator_programs` (tanpa memanggil endpoint programnya, yang akan 403). Dashboard Affiliator pindah ke `/affiliate/overview`. `redirectForRole` (`auth-redirect.ts`) tetap `/affiliate`.
- `PromoModal`: pilihan jenis diskon dan satuan kuota memakai komponen `Select` aplikasi; "Tampilkan di checkout" dan "Khusus affiliator" masing-masing dalam kotak berpenjelasan seperti "Voucher Mitra". Bagian Voucher Mitra: pemilih Mitra yang dapat dicari lewat `GET /admin/affiliates?program=partner&q=`, satuan kuota wajib tanpa nilai awal dengan keterangan, dan "Minimal siswa per order" opsional (0 atau kosong = tanpa minimum).
- `ProductModal`: bagian "Akses Mitra" (pilihan `none/all/selected` + pemilih), hanya untuk pemegang `affiliates:*`. Satu tombol Simpan: akses disimpan dulu lewat `PUT /admin/products/:id/partner-access` bila berubah, lalu produk; akses yang ditolak menghentikan seluruh simpan. Endpoint tetap terpisah.
- Admin Affiliator/Mitra (grup menu "Affiliator/Mitra" pada `id`, "Affiliates/Partners" pada `en`; judul dan keterangan halaman daftar, laporan, dan pengaturan menyebut kedua program): daftar dengan tab program (Semua, Affiliator, Mitra) sejajar dengan Unduh CSV dan Tambah; kolom level hanya bermakna untuk Affiliator ("—" untuk Mitra saja); dialog persetujuan dengan pilihan program. Tidak ada sub menu Mitra terpisah. Detail akun: untuk Mitra saja tidak ada level, saldo, ikatan, komisi, maupun penyesuaian; bagian Tindakan memuat tombol "Tambah program ..." yang meminta konfirmasi dan menyatakan program tidak bisa dicabut; sekolah yang di-allow diatur di panel program.
- Pengaturan: kartu saklar Mitra (`PartnerSwitchCard`) di halaman pengaturan Affiliator/Mitra, di bawah kartu saklar Affiliator; halaman menampilkan kolom kosong, daftar pengaturan wajib yang belum terisi, dan peringatan komisi kosong.
- Laporan: tab "Mitra" di halaman laporan (lihat Laporan admin Mitra). Kartu total berada di atas deretan tab, menggantikan kartu liabilitas Affiliator pada tab itu.
- i18n `id` dan `en` untuk semua string baru. Tes mengikuti `docs/standards/web-tests.md`.

### Laporan admin Mitra

- `GET /admin/partner-reports/orders?q=&status=&from=&to=&partner_id=&page=&per_page=` dan `/export` (CSV), gerbang `affiliates:*`. Sumber: order dengan pembeli berperan `affiliator` yang punya baris `order_participant`, kecuali `cart`. Baris: `order_id`, `created_at`, `status`, `partner_id`, `partner_name`, `products`, `seats`, `students` (string) dan `student_list` (array nama), `promo_code`, `subtotal`, `discount`, `total`. Respons memuat `summary`: `orders`, `paid_orders`, `seats`, `paid_total`, `paid_discount` (nilai hanya order lunas: `paid`, `processing`, `shipped`, `completed`) untuk seluruh hasil filter.
- CSV maksimal 5000 baris, kolom teks diamankan dari injeksi formula (`csvSafeField`). Tanggal `to` inklusif seperti laporan Affiliator.
- UI: kolom siswa berisi tombol "N siswa" yang membuka popup daftar nama; status memakai kunci i18n status order Mitra.

### Privasi dan keamanan

- Semua pembatasan (program, status, sekolah, produk, voucher) ditegakkan di server pada setiap permintaan; menu hanya penunjuk.
- Mitra hanya melihat siswa dalam cakupannya, dengan kolom terbatas. Satu-satunya kredensial yang ia terima adalah kata sandi sementara saat mendaftarkan atau menerbitkan ulang siswa yang ia daftarkan dan belum mengganti kata sandinya; tidak pernah di daftar, tidak pernah untuk siswa lain, dan tidak pernah setelah siswa mengganti kata sandi.
- Jawaban pendaftaran untuk email yang sudah ada tidak menyebutkan role akun.
- Audit log untuk persetujuan, penolakan, perubahan program, sekolah, akses produk, dan penangguhan.

## Acceptance and proof

| # | Kriteria | Bukti |
|---|---|---|
| P1 | Migrasi `0072`: kolom baru, indeks, kolom pengaturan nullable tanpa default, down mengembalikan; `0073` menghapus `daily_registration_cap` | Migration test (testcontainer, pola `migration_0070_test.go`) |
| P2 | `ProgramStates` benar untuk kombinasi: data lama (`programs` NULL), Mitra saja, dua program, menunggu, ditolak, nonaktif; gerbang menolak dengan kode yang tepat dan mengizinkan GET saat nonaktif/saklar mati | Service + handler test |
| P3 | Akun nonaktif: login dan refresh berhasil untuk `affiliator`, ditolak untuk role lain; semua tulis di `/partners/me`, `/affiliates/me` 403; `/auth/password/change` tetap jalan; `ChangeAffiliateStatus` menulis `program_status` dan `users.status` bersamaan | Service + handler test |
| P4 | Pendaftaran: `program` terkirim dan tersimpan; email sudah ada ditolak; saklar program yang dipilih menentukan diterima/tidak; tidak ada batas harian | Service + handler test |
| P5 | Persetujuan: kode referral hanya dibuat untuk Affiliator; admin dapat mengubah/menambah program; menolak butuh alasan yang terlihat oleh pendaftar | Service test |
| P6 | Isolasi v1: job evaluasi level dan broadcast `affiliators` melewati akun Mitra saja; order Mitra tidak membuat komisi, ikatan, atau perpanjangan ikatan, walau peserta terikat Affiliator lain; ikatan siswa tetap | Integration test (worker + binding) |
| P21 | Kredensial: pendaftaran oleh Mitra mengembalikan `temp_password` sekali dan akun `password_temporary = true`; permintaan membawa `password` ditolak; penerbitan ulang berhasil untuk siswa milik Mitra yang belum mengganti, 409 `student_already_activated` setelah siswa mengganti, 404 untuk siswa milik Mitra lain atau tanpa `registered_by`; kondisi diuji atomik terhadap penggantian kata sandi bersamaan; daftar siswa tidak pernah memuat email atau kata sandi | Service + repository + handler test (testcontainer) |
| P22 | Impor CSV Mitra: baris dengan sekolah, tanpa sekolah, dan sekolah di luar cakupan allow semuanya berhasil; kolom `password` ditolak per baris; `registered_by` terisi di tiap siswa; baris gagal tidak menggagalkan yang lain; hanya berkas dari folder Mitra yang diterima saat enqueue; laporan hasil hanya terbaca oleh pembuat job (`/partners/me/jobs/:id` untuk job Mitra lain = 404) | Service + worker + handler test (testcontainer) |
| P23 | Teks ketentuan: halaman `/affiliate-terms` menampilkan semua klausa id dan en, versi `2026-11`; pendaftaran mengirim `terms_version` baru; paritas kunci `id`/`en` | Vitest + test paritas i18n yang ada |
| P7 | Siswa: `registered_by` terisi; daftar siswa Mitra = sekolah yang di-allow ∪ didaftarkan-sendiri; `EXPLAIN` memakai kedua indeks pada data besar | Repository test + catatan EXPLAIN |
| P8 | Order: sekolah di luar cakupan dan bukan terdaftar-oleh-dia → 403 untuk seluruh permintaan; yang sudah memiliki dikeluarkan; produk di luar akses → 403 di preview, create, checkout; produk fisik ditolak | Service + handler test |
| P9 | Voucher: siswa dan `/promo-codes/validate` menolak voucher Mitra; Mitra lain menolak voucher untuk Mitra terpilih; kosong = semua Mitra; kedaluwarsa/kuota/berubah saat checkout → 422 sebelum pembayaran; `affiliator_only` v1 tidak berubah | Service + handler test |
| P10 | Kuota: dua checkout bersamaan pada sisa 1 → satu berhasil, satu 422 dengan rollback. Satuan `student`: order 50 siswa memakai 50 kuota; order melebihi sisa ditolak seluruhnya dengan `remaining`; diskon maksimum berlaku per siswa. Satuan `order`: perilaku lama. `quota_unit` kosong ditolak saat membuat voucher | Integration test (goroutine + testcontainer) + service test |
| P11 | `/orders/*` menolak role `affiliator`; jalur Mitra tidak bisa dilewati dengan checkout atau retry generik | Handler test routes |
| P12 | Checkout order peserta dengan qty > 1 berhasil; order biasa tanpa peserta dengan digital qty > 1 tetap ditolak | Service test |
| P13 | Fan-out: setelah `OrderPaid`, tiap peserta punya `course_session` dengan `order_id`; event ganda tidak menggandakan | Worker test |
| P14 | Refund order Mitra: status `cancelled`/`refunded`, registrasi ujian dan `course_session` semua peserta `revoked`; tidak ada transaksi komisi | Integration test (`AdminRefundOrder`) |
| P15 | Default kosong: tanpa pengaturan tidak ada komisi, tidak ada yang naik/turun; saklar Affiliator ditolak dengan `missing` lengkap lewat rute config umum maupun `/program`; mengosongkan pengaturan wajib saat program menyala ditolak | Service + handler test (menggantikan tes yang menegaskan default lama) |
| P16 | Akses produk: `none`/`all`/`selected` benar di daftar; `UpdateProduct` tidak menimpa `partner_access` | Repository + service test |
| P17 | Frontend: menu dua grup, pesan per keadaan, landing satu/dua program, `PromoModal`/`ProductModal` section, registrasi dengan pilihan program | Vitest sesuai `docs/standards/web-tests.md` |
| P19 | Kepemilikan saat checkout: peserta yang kini sudah memiliki produk (dibelikan pihak lain atau beli sendiri) membuat checkout 409 `participant_already_owns`; pembelian ulang setelah refund berhasil (sesi `revoked` tidak menghalangi) | Service + integration test |
| P20 | Voucher terpisah: `/partners/me/vouchers` hanya voucher Mitra, `/affiliates/me/promos` tidak berubah dan tidak memuat voucher Mitra; order Mitra menolak promo publik dan promo `affiliator_only` | Handler + service test |
| P24 | Pagination bernomor: `/partners/me/schools`, `/students?page=`, dan `/orders` mengembalikan `total`, `page`, `per_page`; pencarian, status, tanggal, dan sekolah menyaring; `page=0` → 400; daftar order hanya milik Mitra sendiri | Service + handler test |
| P25 | Akun ditangguhkan, program mati, atau belum disetujui: `/vouchers` dan `/products` kosong, hitungan dashboard nol | Handler test |
| P26 | `min_students`: order di bawah minimum → 422 `promo_min_students` dengan `min`; tepat minimum lolos; nilai negatif ditolak saat membuat voucher; tersimpan dan dapat diubah | Service + handler test |
| P27 | Laporan admin Mitra: baris memuat Mitra, produk, siswa, voucher; `summary` benar; pencarian, status, dan Mitra menyaring; ekspor CSV; hanya admin dengan `affiliates:*` | Handler test (testcontainer) + Vitest tab laporan |
| P28 | Frontend admin: tambah program meminta konfirmasi dan tidak dijalankan bila dibatalkan; Mitra saja tanpa level/saldo/penyesuaian; satu tombol Simpan produk menyimpan akses dulu dan berhenti bila akses ditolak; menu menyembunyikan grup program yang tidak diikuti | Vitest |
| P18 | Alur Mitra penuh: daftar, setujui, atur sekolah/produk/voucher, daftarkan siswa, beli dengan voucher 100% atau Snap no-op, akses terbentuk, refund mencabut | Satu spec Playwright `web/e2e/partner-program.spec.ts` terhadap compose dev bila stack tersedia, bila tidak dicatat tidak dijalankan |

Strategi verifikasi mengikuti `.agents/verification.md`: gate penuh backend `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh` (background, lebih dari 10 menit) sebelum PR; loop fokus per paket dengan `-run` saat membangun; frontend `npx tsc --noEmit`, `npx vitest run`, `npm run build` dari `web/`. Tes yang butuh Postgres membutuhkan `DOCKER_HOST` yang benar.

## Scope

**Dalam lingkup**: program Mitra (akun, persetujuan, sekolah, siswa, impor CSV, akses produk, voucher, order, refund), laporan admin Mitra, perapian Program Affiliator, dan perubahan kode yang ada di atas.

**Non-goals**: produk fisik; komisi/saldo/pencairan Mitra; harga jual dan penagihan siswa; refund per siswa; pengembalian kuota voucher saat refund; alur promote admin sekolah; pembuatan Mitra massal; aturan voucher tambahan (tanggal mulai, minimal jumlah order sebelumnya, lingkup produk); promo publik biasa untuk Mitra; grup broadcast `partners`; pencabutan v1; pencabutan program dari akun; aturan khusus untuk referral ke siswa yang didaftarkan akun yang sama.

**Keputusan tertunda**: tidak ada pertanyaan terbuka. Perlu ditinjau pemilik produk: kata-kata draf teks ketentuan (kunci i18n `aff_terms_*`) sebelum dirilis.

**Residual risk**: JSONB (`programs`, `partner_access`, `partner`) tidak punya FK/CHECK per elemen (divalidasi service); `ChangeAffiliateStatus` mencerminkan status ke dua tempat (satu transaksi, satu fungsi, tapi tetap dua sumber untuk v1); perubahan checkout dan kuota voucher menyentuh alur siswa yang sudah berjalan; Playwright belum terbukti tanpa stack lokal; perilaku checkout ujian massal admin sekolah belum dijalankan.
