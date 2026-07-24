# Store & Checkout UX — Design

| | |
|---|---|
| **Tanggal** | 2026-07-24 |
| **Branch** | `feat/store-catalog-ux` |
| **Base** | `origin/main` @ `dc53214` |
| **Worktree** | `.claude/worktrees/store-catalog-ux` |
| **Status** | Disetujui, siap masuk perencanaan implementasi |

## Konteks

Enam keluhan UX pada sisi produk/e-commerce, di luar cakupan PR #44 yang sedang berjalan.
Penelusuran kode menemukan dua masalah tambahan yang lebih berat dari keenam item awal:
sistem menampilkan tarif pengiriman karangan, dan data pengiriman yang sudah tersimpan
tidak pernah ditampilkan ke user. Keduanya masuk scope.

Branch ini dikerjakan di worktree terpisah karena ada sesi lain yang sedang menyiapkan
CD ke produksi di working directory utama.

## Temuan yang mendasari design

Semua diverifikasi langsung di kode, bukan asumsi.

| # | Temuan | Bukti |
|---|---|---|
| 1 | Katalog student hanya punya 4 tab; merch & medal tidak bisa difilter meski backend tidak memfilternya | `web/app/(student)/catalog/page.tsx:17`, `backend/internal/repository/product.go:97` |
| 2 | `ListProducts` default limit 20 + cursor, tapi `useProducts` tidak pernah ambil halaman berikutnya — katalog terpotong diam-diam | `backend/internal/repository/product.go:99`, `web/lib/hooks/products.ts:13` |
| 3 | Cover produk memakai `background-size: cover` di box `h-32` → cover buku potret ter-crop | `web/components/catalog/ProductCard.tsx:48` |
| 4 | Tidak ada data spesifikasi produk sama sekali di model | `backend/internal/model/product.go:5` |
| 5 | **Bug overcharge**: outbox memenuhi `exam`/`course` dengan mengabaikan qty — beli qty 3 ditagih 3× tapi hanya 1 registrasi | `backend/internal/worker/outbox.go:201` |
| 6 | `AddItem` tidak punya guard qty untuk tipe digital | `backend/internal/service/store.go:425` |
| 7 | **Tarif karangan**: tanpa Biteship key, `NoopLogisticsClient` mengembalikan JNE REG Rp15.000 & TIKI ONS Rp25.000 hardcoded, dengan `error = nil` — sehingga fallback flat-rate tidak pernah tercapai, dan angka fiktif itu masuk ke `total` lalu ditagihkan lewat Midtrans | `backend/internal/service/ports_logistics.go:23`, `backend/internal/service/store.go:399` |
| 8 | `selected_courier`, `selected_service`, `shipping_address` dikembalikan API dan ada di tipe FE, tapi nol referensi di seluruh UI | `backend/internal/repository/order.go:386`, `web/lib/types.ts:197` |
| 9 | Form alamat hanya memakai 4 field wilayah, mengabaikan `name`/`phone`/`alamat_domisili` yang sudah ada di `users` | `web/components/cart/ShippingAddressForm.tsx:100-188`, `backend/internal/model/user.go:9-31` |

Nomor baris di atas mengacu pada base branch ini (`origin/main` @ `dc53214`), bukan pada
branch PR #44 yang isinya sudah bergeser.

Catatan penomoran migration: base ada di 0036; PR #44 membawa 0037–0044 dan belum merge.
Migration di branch ini dinomori **0045** supaya tidak tabrakan apa pun urutan merge-nya.

## Slice 1 — Layout katalog

**Cakupan:** FE saja.

Tabs horizontal diganti rail kategori kiri yang sticky, di dalam content area.

- Enam kategori flat: Semua Produk / Buku / Kursus / Ujian / Merchandise / Medali
- Styling polos — list teks tanpa border atau card, hanya indikator aktif. Ini yang
  membedakannya secara visual dari rail `AppShell` 252px di sebelahnya; dua rail
  bergaya sama akan terbaca sebagai satu kolom nav yang membingungkan.
- `sticky top-6 self-start`, pola yang sudah dipakai di aside detail produk. Ini yang
  menjawab keluhan inti: kategori tetap terjangkau saat scroll panjang.
- Lebar rail ~200px. Content area 1340px dikurangi padding ≈ 1284px, sisa grid ≈ 1060px,
  cukup untuk 5 kolom @ ~199px per card.
- Mobile: rail menjadi baris chip horizontal yang bisa di-scroll, sticky di bawah header.

Card dirombak:

- Box gambar rasio **3:4**, `object-contain` di atas background netral — cover buku utuh
- Isi card: badge tipe, judul, harga. Deskripsi dibuang.
- Grid responsif 2 / 3 / 4 / 5 kolom (sm / md / lg / xl)
- Produk tanpa gambar tetap memakai gradient + ikon yang mengisi box 3:4 yang sama,
  sehingga tinggi card seragam dan grid tidak belang

`useProducts` mengikuti `next_cursor` sampai habis, dibatasi 10 halaman. Ini stopgap yang
disadari: kalau katalog tembus ratusan produk, butuh pagination sungguhan.

## Slice 2 — Spesifikasi produk

**Cakupan:** migration + backend + admin + FE.

Migration `0045_product_specs`:

```sql
ALTER TABLE product ADD COLUMN specs JSONB NOT NULL DEFAULT '[]';
```

Bentuk data adalah **array**, bukan object, supaya urutan tampilan terjaga:

```json
[{ "key": "penerbit", "label": "Perusahaan Penerbit", "value": "Yayasan Abak Cendekia" }]
```

`label` ikut disimpan supaya rendering tidak perlu lookup konstanta dan baris custom
tetap bisa dirender.

Daftar field kanonik hidup di `web/lib/product-specs.ts`, bukan di DB:

| Tipe | Field |
|---|---|
| `book` | Penerbit, Tahun Terbit, Bahasa, Jenis Cover, Jenis Edisi, Jumlah Halaman, ISBN, Impor/Lokal |
| `merchandise` | Bahan, Ukuran, Warna, Isi Paket |
| `medal` | Bahan, Diameter, Finishing, Kemasan |
| `course`, `exam` | tidak ada field default — hanya baris custom |

Backend tidak tahu daftar field ini. Dia hanya memvalidasi bentuk: harus array, maksimal
30 entri, `key` dan `label` ≤100 karakter, `value` ≤500 karakter. Batas inilah yang
menjaga kolom JSONB tidak menjadi tempat sampah tak terbatas.

Admin `ProductModal` mendapat section "Spesifikasi Produk" dengan baris ter-generate dari
field list sesuai tipe terpilih, plus tombol tambah baris custom. Halaman detail produk
mendapat tabel "Spesifikasi Produk" di bawah deskripsi. Baris dengan value kosong tidak
dirender.

**Kenapa bukan attribute schema penuh ala Shopee:** model Shopee/Tokopedia adalah skema
atribut per-kategori dengan tipe dan controlled vocabulary, karena atributnya harus bisa
jadi facet filter lintas ribuan seller. Katalog ini punya satu seller dan produk
berjumlah puluhan; biaya governance-nya membayar masalah yang belum ada. Dengan key sudah
kanonik sejak awal, jalur upgrade ke facet nanti tinggal normalisasi value — bukan
membersihkan key yang berbeda-beda.

## Slice 3 — Guard qty produk digital

**Cakupan:** backend + FE.

Ini menutup bug overcharge (temuan #5), bukan sekadar menambah pesan.

- **FE** — stepper qty disembunyikan untuk `exam` dan `course`, diganti teks
  "Produk digital dibeli 1× per akun", tampil sebelum add-to-cart.
- **Backend** — `AddItem` dan `UpdateItemQty` menolak `qty > 1` untuk tipe non-fisik,
  mengembalikan `400 invalid_qty`. Ini chokepoint sebenarnya; guard FE saja masih bisa
  dilewati lewat API langsung.
- **admin_school tidak tersentuh** — pembelian multi-seat mereka lewat
  `CreateBulkExamOrder`, endpoint terpisah dari cart student.

## Slice 4 — Blok pengiriman di detail order

**Cakupan:** FE saja.

Data pada temuan #8 sudah lengkap dari API dan sudah ada di tipe FE, hanya tidak pernah
dipasang ke UI. Kumpulkan menjadi satu blok "Pengiriman" di halaman detail order:

- Alamat tujuan
- Kurir dan layanan (mis. "JNE — REG")
- Ongkir
- Nomor resi, dipindahkan ke sini dari `PaymentInfo` supaya semua hal pengiriman
  berkumpul di satu tempat

Blok hanya tampil untuk order yang berisi item fisik. Halaman daftar order sengaja tetap
menampilkan total saja — rincian ongkir di daftar hanya menambah keramaian.

## Slice 5 — Alamat pengiriman di checkout

**Cakupan:** FE + sedikit backend. **Tanpa migration.**

Alamat tersimpan tampil sebagai ringkasan dengan tombol "Ubah". Form hanya muncul saat
user menambah atau mengubah alamat — bukan selalu terbuka seperti sekarang.

Form dilengkapi field yang selama ini sudah tersimpan di `users` tapi tidak dipakai oleh
form pengiriman: nama penerima, nomor telepon, alamat jalan (`alamat_domisili`). Semuanya
prefill dari profil dan bisa di-override per order.

Override disimpan di snapshot `orders.shipping_address`, yang sudah bertipe JSONB — jadi
tidak mengubah profil user dan tidak butuh perubahan skema. Ini juga yang membuat alamat
historis sebuah order tetap utuh walau user nanti mengganti alamat profilnya.

## Slice 6 — Hentikan tarif karangan

**Cakupan:** backend + FE.

Temuan #7 adalah blocker produksi: sistem menagih ongkir berdasarkan angka fiktif yang
menyamar sebagai kurir asli, tanpa penanda apa pun di UI.

- `NoopLogisticsClient.GetRates` berhenti mengembalikan JNE/TIKI hardcoded. Dia
  mengembalikan error, sehingga jalur fallback flat-rate yang sudah ada di
  `GetShippingRates` benar-benar terpakai.
- `CourierRate` mendapat field `is_estimate`. Tarif dari `shipping_fallback_flat_rate`
  ditandai `true`.
- `CourierRateList` dan blok pengiriman Slice 4 menampilkan penanda
  "Estimasi — bukan tarif kurir" untuk tarif ber-`is_estimate`.
- Kalau `shipping_fallback_flat_rate` juga kosong, `GetShippingRates` mengembalikan error.
  Konkretnya di UI: daftar kurir menampilkan pesan "Pengiriman belum tersedia, hubungi
  admin", dan tombol checkout dinonaktifkan selama cart berisi item fisik. Order digital
  murni tetap bisa checkout seperti biasa.

Pemasangan Biteship key di produksi adalah pekerjaan ops di sesi CD, di luar branch ini.
Perubahan ini memastikan ketiadaan key gagal secara jujur dan terlihat, bukan diam-diam.

## Testing

**Frontend**

- `ProductCard`: contain-fit tidak meng-crop, isi card hanya badge/judul/harga, tinggi
  seragam saat gambar tidak ada
- Rail kategori: state aktif, perilaku sticky, fallback chip di mobile
- `useProducts`: mengikuti cursor melewati batas 20 produk, berhenti di batas 10 halaman
- Qty lock: stepper hilang untuk exam/course, tetap ada untuk tipe fisik
- Tabel spec: baris ber-value kosong tidak dirender
- Alamat checkout: ringkasan tampil secara default, form muncul saat "Ubah"
- Badge estimasi muncul saat `is_estimate` bernilai true

**Backend**

- Qty guard: tolak >1 untuk digital, terima >1 untuk fisik, `CreateBulkExamOrder` lolos
- Specs: round-trip create/update/get; validasi bentuk menolak non-array, >30 entri,
  string melebihi batas
- `NoopLogisticsClient` tidak lagi mengembalikan tarif
- Fallback flat-rate terpakai saat client mengembalikan error, dan ditandai `is_estimate`
- Checkout fisik diblokir saat client dan flat-rate sama-sama kosong
- Migration test di boundary 0036→0045

## Di luar scope

Dicatat sebagai backlog, sengaja tidak dikerjakan di branch ini:

| Item | Alasan |
|---|---|
| Gambar untuk produk digital | `ProductModal` membuang `image_url` untuk exam/course (`showStock` guard di `ProductModal.tsx:142` dan `:156`); backend sudah menerimanya untuk semua tipe. Perbaikannya ~3 baris, tapi di luar enam item yang diminta. |
| Facet filter | Butuh controlled vocabulary; key spec sudah kanonik jadi jalur upgrade-nya terbuka |
| Pagination sungguhan | Loop cursor cukup sampai skala ratusan produk |
| Address book multi-alamat | Satu alamat profil sudah memenuhi kebutuhan sekarang |
| Pemasangan Biteship key di produksi | Pekerjaan ops di sesi CD-produksi |
