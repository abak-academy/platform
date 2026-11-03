# Prototype alur exam admin & siswa

**Lanjut agent terbaru:** [START HERE / HANDOFF 2](HANDOFF-2.md) · [Spec Tes](../../specs/test-authoring-flow.md) · [Plan Tes](../../backlog/test-authoring-flow-plan.md) · [Validasi 2](VALIDATION-2.md).

Paket sebelumnya: [Handoff Exam](HANDOFF.md) · [Spec Exam](../../specs/exam-user-flow.md) · [Plan Exam](../../backlog/exam-user-flow-plan.md) · [Validasi pertama](VALIDATION.md).

Prototype eksplorasi desain, belum menjadi kontrak implementasi produksi.

Basis: `feat/composable-exam-format` pada `0d2b6ca1a6c2985f903d16fbd94e71ee297a4540`.
Branch: `feat/composable-exam-flow-prototype`.

## Coba prototype

Dari root worktree:

```sh
python3 -m http.server 8765 --bind 127.0.0.1
```

Buka [daftar admin](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html),
[card siswa](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#student-list),
[susunan section](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#admin-detail),
[card belum dibuka](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?access=upcoming#student-list),
atau [sebelum siswa mulai](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#student-detail).

Semua data contoh hanya berada di memori halaman. Reload atau Reset mengembalikan data awal.
Toolbar gelap adalah kontrol prototype, bukan bagian dari rancangan produk.

## Arah desain

Permintaan: perbaiki layout, pengelompokan field, kolom, dan alur admin serta siswa setelah composable exam. Pertahankan warna dan referensi visual aplikasi existing. Gunakan logo dan font lokal existing, palet dari `web/app/globals.css`, serta pola sidebar, header, tab, card, dan navigasi soal yang sudah ada.

| Area | Perubahan yang diusulkan |
| --- | --- |
| Daftar admin | Lima kolom: ujian, susunan, navigasi, waktu pengerjaan, dan jadwal akses. Status/filter status exam dihapus agar tidak memberi kesan publikasi berdiri sendiri. Header, search, dan tabel mengikuti acuan existing di bawah. |
| Detail admin | Satu tombol Pengaturan dan satu Pratinjau siswa di header. Ringkasan kiri hanya jadwal, akses peserta, hasil/komunikasi; judul dan policy pengerjaan tidak diulang. Jendela/toleransi check-in hanya muncul saat check-in aktif, rilis hasil hanya saat hasil ditampilkan. Ringkasan kanan menjelaskan pengalaman siswa; tab Tes menjadi “Tes & section”. |
| Susunan section | Ruang utama untuk urutan, tambah/hapus Test, dan pilihan tampilan soal. Sidebar merangkum navigasi dan timer. Durasi bersumber dari Test, bukan input override per attachment. |
| Pengaturan | Dialog diperlebar. Field dikelompokkan menjadi informasi/jadwal akses, aturan pengerjaan, akses peserta, hasil, serta kartu/layar selesai. Penjelasan membedakan jadwal akses dari durasi pengerjaan. |
| Daftar siswa | Card tiga kolom desktop, dua kolom tablet, dan satu kolom mobile; padding dan jarak antarbagian dirapatkan. Hierarki judul/status, tiga ringkasan waktu–section–soal, jadwal akses tersendiri, dan aturan navigasi ringkas. CTA “Lihat aturan ujian” menuju detail sebelum timer dimulai. Saat berjalan, tampilkan progres jawaban dan CTA lanjut; saat selesai, arahkan ke penyelesaian. |
| Sebelum mulai | Card Informasi ujian memisahkan jadwal akses (ikon kalender, tanggal dan jam) serta durasi pengerjaan (ikon jam, lingkup timer) dalam dua panel yang responsif. Tampilkan urutan section, jumlah soal, cara menampilkan soal, lingkup timer, dan konsekuensi navigasi. Persetujuan pemahaman dan konfirmasi mulai adalah usulan UX baru. |
| Saat mengerjakan | Pertahankan area soal dan navigator kanan. Timer menyebut lingkup ujian/section, section aktif jelas, dan konfirmasi menyebut soal kosong serta konsekuensi mengunci section. |

## Alur yang dapat diklik

1. Buka ujian → tambah Test → cari English Reading → tambah → ubah urutan/hapus → simpan.
2. Pada Literasi, pilih semua soal dalam satu halaman → konflik materi soal 2 muncul → simpan diblokir → perbaiki dengan satu soal per halaman.
3. Buka sumber Test untuk melihat pemilik durasi/materi dan peringatan perubahan konten lintas ujian. Dialog sumber Test pada alur Exam hanya baca saja; editor Tes dapat dicoba melalui submenu Tes. Soal contoh menggunakan pilihan ganda; format jawaban lainnya tidak disimulasikan.
4. Atur navigasi bebas dengan timer per section → kombinasi invalid dijelaskan; simpan ditolak tanpa mengubah pilihan lain diam-diam. Pilih timer seluruh ujian untuk kombinasi yang valid.
5. Pratinjau siswa → pahami aturan → konfirmasi mulai → jawab/tandai soal → konfirmasi akhir section → section selanjutnya → kumpulkan.
6. Aktifkan check-in di pengaturan → token contoh `DEMO` diperlukan sebelum mulai; check-in tidak memulai timer.
7. Checkbox “Izinkan lebih dari satu percobaan” default mati dan menyimpan batas 1 kali. Saat aktif, tampilkan jumlah percobaan minimal 2 (default 2) dan checkbox “Tanpa batas” terpisah. “Tanpa batas” menyimpan nilai null dan menyembunyikan input jumlah; input kosong tanpa checkbox tersebut tidak valid. Mematikan checkbox utama menyimpan kembali 1 kali.
8. Pilih skenario “Sudah ada sesi” atau mulai sesi contoh → aturan pengerjaan dan struktur section terkunci. Pengaturan non-policy tetap dapat disimpan. Reset untuk mencoba konfigurasi berbeda.
9. Pada tampilan siswa, pilih kondisi “Belum dibuka” untuk contoh tanggal akses mendatang. Card menampilkan badge netral, hitung mundur hari/jam/menit, serta CTA “Lihat detail”. Aturan tetap dapat dibaca, tetapi check-in dan mulai sesi belum tersedia. Ubah ke “Akses terbuka” untuk mencoba mulai sesi.
10. Pada Pengaturan → Informasi & jadwal akses, isi Link grup WhatsApp kompetisi (opsional). Simpan lalu pratinjau siswa: ajakan tampil tepat setelah Informasi ujian, baik sebelum akses dibuka maupun saat akses terbuka. Kosongkan link untuk menyembunyikan blok. Join tidak menjadi syarat mulai ujian.
11. Cari judul ujian di toolbar daftar, hapus pencarian untuk reset, buka format sebelumnya, atau coba susunan kosong melalui Buat ujian. Filter status sudah dihapus.

## Kontrak existing yang dipertahankan

- Format baru menggunakan navigasi, timer, dan presentasi section eksplisit; label Standard/UTBK/IELTS tidak menjadi preset format baru.
- Kombinasi navigasi bebas dengan timer per section tidak valid.
- `all_on_page` tidak menerima materi khusus pada Question. Untuk satu soal per halaman, materi Question menggantikan materi bersama hanya pada soal itu.
- Durasi Test merupakan sumber durasi section. Urutan section serta presentasi per attachment milik Exam.
- Setelah ada sesi, navigasi, randomisasi, timer, durasi ujian, dan attachment/presentasi tidak dapat diubah.
- Kompensasi waktu milik sesi terdampak melalui Monitoring; tidak disajikan sebagai edit durasi default Exam.
- Jadwal akhir akses membatasi mulai sesi baru; deadline sesi aktif mengikuti timer sesi.
- Penjualan/publikasi tetap melalui Produk. Tidak ada tombol publish exam baru di prototype.
- Risiko konten Test dibaca live tetap dijelaskan; prototype tidak menjanjikan Test revision/immutability.

## Batas dan catatan implementasi berikutnya

- Link WhatsApp default kosong; gunakan link undangan grup yang dimiliki saat mencoba. Form memeriksa HTTPS dan domain `chat.whatsapp.com`, bukan keberadaan/keaktifan undangan. Tombol siswa membuka link di tab baru dengan `noopener noreferrer`; pengujian memakai link contoh tanpa menghubungi WhatsApp. Field baru ini hanya ada di prototype, belum menjadi field backend/API/database.
- Seluruh aksi adalah simulasi. Timer tidak menghitung mundur, penyimpanan tidak menghubungi API, tidak ada autentikasi, scoring, pengiriman jawaban, atau persistence nyata.
- Acak soal memakai pembalikan urutan tetap untuk satu contoh; bukan implementasi permutasi backend. Urutan tetap sepanjang perpindahan section pada simulasi tersebut.
- Hasil/scoring, editor Question lengkap, upload gambar, kartu PDF, sertifikat, pendaftaran, monitoring, timeout otomatis, reconnect, dan cabang seluruh status akses siswa belum disimulasikan. Kontrol navigasi di luar cakupan dinonaktifkan.
- Skenario admin: konfigurasi belum punya sesi dan sudah punya sesi. Skenario akses siswa: belum dibuka dan akses terbuka. Hitung mundur sebelum dibuka memakai waktu contoh tetap 2 Nov 2026 pukul 08.00 WIB; skenario terbuka menganggap peserta berada dalam jadwal akses. Ini bukan jam berjalan atau pengaman jadwal produksi. Check-in, jawaban kosong, konflik materi, susunan kosong, timer keseluruhan/per section, serta navigasi bebas/berurutan dapat dicoba.
- Kolom susunan/jumlah soal pada daftar admin memakai detail contoh yang sudah tersedia di halaman. Implementasi nyata harus memeriksa atau menambah ringkasan data pada endpoint list; jangan melakukan satu fetch detail untuk setiap baris tanpa keputusan kontrak API.
- Ringkasan aturan pada detail siswa juga perlu payload section/policy yang sesuai. Prototype tidak membuktikan endpoint registrasi existing sudah memuat semua field itu.
- Field catatan kartu disajikan sebagai textarea per baris dan field gambar sebagai URL untuk eksplorasi pengelompokan. Upload dan pembatasan catatan produksi tetap perlu memakai komponen existing saat implementasi.
- Pengaman perubahan section yang belum disimpan pada tombol preview adalah usulan UX. Kebijakan meninggalkan halaman/reset di prototype belum mewakili pengaman produksi lengkap.

## Acuan standar admin dan hasil pengecekan kode

- Header mengikuti struktur dan ukuran [AdminPageHeader](../../../web/components/admin/AdminPageHeader.tsx): ikon 64 px, glyph 32 px, gap 20 px, judul 34/42 px, deskripsi 18/26 px berjarak 84 px dari kiri. Copy daftar memakai `exam_packages_page_title` dan `exam_packages_page_description` existing. Palet dan font prototype yang telah disetujui tetap dipertahankan.
- Search mengikuti [ProductsToolbar](../../../web/components/admin/ProductsToolbar.tsx), [ListToolbar](../../../web/components/affiliate/ListToolbar.tsx), dan primitive [Input](../../../web/components/ui/input.tsx): ikon kiri, input satu baris, tombol hapus pencarian di kanan. Tidak menambahkan filter pengganti ketika filter status dihapus; pilihan filter kosong tidak ditampilkan.
- Tabel mengikuti markup halaman [Produk](../../../web/app/(admin)/admin/products/page.tsx): pembungkus outlined, padding 24 px, teks 14 px, header normal case, cell padding 16/12 px, border antarbaris, hover, scroll horizontal, dan empty state di dalam tabel. Repo pada basis ini tidak memiliki komponen `ui/table.tsx` yang bisa dianggap sebagai shared Table.
- Prototype ini tetap HTML statis dengan referensi sumber pada markup. Ia tidak mengimpor/memasang komponen React produksi, dan tidak menambahkan komponen produksi baru. Implementasi aplikasi berikutnya harus memakai komponen existing di atas, bukan menyalin menjadi varian per modul.
- Pada basis `0d2b6ca1`, `CreateExam` masih mengisi `exam.status=draft`; `PublishProduct` memvalidasi exam yang terlampir lalu memperbarui `product.status` saja. `HasPublishedProduct` merupakan flag terhitung yang juga mempertimbangkan availability, bukan status exam tunggal. Karena itu prototype tidak menampilkan Draft/Published pada daftar/detail exam dan tidak memakai `exam.status` sebagai filter publikasi.
- Sumber pengecekan: [ExamListItem](../../../backend/internal/model/exam.go), [CreateExam](../../../backend/internal/service/exam.go), [ListExams](../../../backend/internal/repository/exam.go), [PublishProduct service](../../../backend/internal/service/store.go), dan [PublishProduct repository](../../../backend/internal/repository/product.go). API/DB/komponen produksi existing belum diubah oleh prototype ini.
- [Buka Ringkasan admin langsung](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html#admin-overview).

## Referensi sumber

- [Palet dan tipografi](../../../web/app/globals.css)
- [Shell aplikasi](../../../web/components/shell/AppShell.tsx)
- [Daftar ujian admin](../../../web/app/(admin)/admin/exam/packages/page.tsx)
- [Detail ujian admin](../../../web/app/(admin)/admin/exam/packages/[id]/page.tsx)
- [Form pengaturan existing](../../../web/components/admin/ExamModal.tsx)
- [Daftar siswa](../../../web/app/(student)/exam/page.tsx)
- [Detail sebelum mulai](../../../web/app/(student)/exam/[id]/page.tsx)
- [Runtime siswa](../../../web/app/(exam-session)/exam/sessions/[id]/page.tsx)
- [Validasi backend](../../../backend/internal/service/exam.go)
- [Kontrak composable](../../specs/composable-exam-format.md)

## Verifikasi

Bukti handoff terkini: [VALIDATION.md](VALIDATION.md), [report.json](evidence/report.json), dan screenshot pada folder [evidence](evidence/). Report memuat SHA-256 source prototype/verifier dan waktu pemeriksaan; cocokkan source hash sebelum menganggap bukti masih berlaku.

`verify.cjs` adalah pemeriksaan portable untuk prototype ini. Ia memeriksa enam kelompok perilaku pada tujuh lebar viewport, termasuk admin list/overview, checkbox percobaan, komposisi section, WhatsApp, upcoming/preflight, dua presentasi soal/konfirmasi, dan responsive metadata. Ini bukan test suite produksi; rincian proven/not proven dan command ada di VALIDATION.

Tidak menjalankan gate aplikasi Next.js, backend, API, atau database pada pekerjaan handoff karena tidak ada kode produksi yang diubah. Pemeriksaan di atas membuktikan perilaku prototype, bukan kompatibilitas API maupun runtime produksi.

## Eksplorasi halaman Tes

[Buka daftar Tes](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?tests=v1#tests-list) · [Buka detail Tes](http://127.0.0.1:8765/docs/prototypes/exam-flow/index.html?tests=v1#tests-detail). Rancangan ini untuk review, belum menjadi spec produksi yang disetujui.

- Daftar memakai referensi header, toolbar search/filter, dan tabel existing; tanpa status publikasi Tes.
- Detail memisahkan Soal, Materi & instruksi, serta Informasi tes. Satu tombol menyimpan pengaturan; urut/tambah/lepas soal diterapkan langsung mengikuti pemisahan operasi existing.
- Tes memiliki materi, instruksi, audio, dan durasi. Navigasi/timer/presentasi tetap di Exam. Pratinjau satu/semua soal hanya simulasi; materi khusus soal menghalangi mode semua soal.
- Buat Tes, bank soal contoh MCQ, urut/lepas/tambah, editor teks sederhana, dan validasi Listening dapat dicoba. Field mapel/topik masih memakai input contoh, bukan katalog API. Upload audio memakai URL lokal sementara; tidak mengunggah berkas. Tidak ada editor Question lengkap, penyimpanan server, scoring, maupun pengujian batas pemutaran audio. Reload mengembalikan data awal.
- Editor Tes memakai sumber data contoh yang sama dengan Exam; perubahan ikut terlihat pada ujian contoh. Pengaman meninggalkan pengaturan yang belum disimpan belum disimulasikan.

Referensi: [daftar Tes](../../../web/app/(admin)/admin/exam/tests/page.tsx), [detail Tes](../../../web/app/(admin)/admin/exam/tests/[id]/page.tsx), [TestModal](../../../web/components/admin/TestModal.tsx), dan validasi Test pada [service exam](../../../backend/internal/service/exam.go).

Verifikasi khusus: `EXAM_FLOW_WEB_ROOT=/path/to/checkout/web node docs/prototypes/exam-flow/verify-tests.cjs`. Memeriksa search/filter, pembuatan Tes, reorder, detach/reattach dari bank, simpan materi/instruksi, durasi, audio wajib untuk Listening, preview, navigasi Exam, dan overflow pada 1440/1024/768/390/360 px. Bukti Exam pada `evidence/report.json` adalah snapshot sebelum ekstensi halaman Tes; hash-nya tidak mewakili source setelah ekstensi ini. Gate aplikasi produksi tidak dijalankan untuk perubahan HTML prototype ini.

Bank soal pada prototype Tes kini mengikuti filter topik dan lima format dari `QuestionPickerModal` existing. Fixture saat ini berupa pilihan ganda; format tanpa data menampilkan empty state. Satu contoh soal panjang tersedia melalui pencarian “pemerintah kota”: teks diringkas tiga baris, dapat dibuka/diringkas tanpa mengganti pilihan checkbox. Pilihan tetap tersimpan lintas pencarian/filter dan jumlah terpilih terlihat di footer. Filter dan pencarian dapat direset bersama.

## Revisi feedback 3 Nov 2026

- Admin → Ujian → Tes & section → Aturan pengerjaan memiliki satu form Aturan tambahan: keterangan dan URL http/https, keduanya opsional. Tersimpan terpisah dari susunan section; kosong menyembunyikan blok dari siswa. Link tampil pada Cara mengerjakan, membuka tab baru, dan tidak menjadi syarat mulai ujian. Field ini masih usulan prototype, belum ada kontrak API/database produksi.
- Ringkasan Hasil & komunikasi menunjukkan URL WhatsApp jika tersedia. Jika kosong, tertulis Tidak dicantumkan dengan penjelasan ajakan tidak tampil; tidak mengasumsikan alasan admin mengosongkannya.
- Toolbar materi/instruksi mengacu [RichTextEditor](../../../web/components/admin/RichTextEditor.tsx) yang dipakai Question: B/I/U, daftar, superscript/subscript, rumus, gambar, dan tabel. Prototype tetap editor HTML sederhana, **bukan mounting TipTap/komponen React produksi**. Gambar memakai URL blob lokal tanpa upload; rumus berupa teks LaTeX dan belum dirender; operasi tabel kompleks belum disimulasikan. Implementasi produksi harus menggunakan komponen existing tersebut.
- Materi bersama di sidebar mempertahankan heading, paragraf, tabel, dan gambar, dengan area scroll dan Lihat lengkap. Aksi soal memakai ikon preview dan lepas berjajar, label aksesibel, serta konfirmasi sebelum melepas. Filter topik/format dan ringkasan tiga baris pada bank soal tetap tersedia.
- Jalankan `verify-feedback.cjs` dengan environment yang sama seperti `verify-tests.cjs` untuk memeriksa optional rules, validasi URL, status WhatsApp, aksi sejajar/konfirmasi, gambar/tabel tersimpan, preview materi, dan lima ukuran layar. Verifikasi ini serta `verify-tests.cjs` dan `verify.cjs` lulus pada revisi ini; bukti lama pada folder evidence tetap snapshot terdahulu. Tidak menjalankan gate produksi karena perubahan terbatas pada prototype HTML.

Aturan tambahan diperbarui: default hanya checkbox tanpa form. Centang untuk menampilkan field. Simpan mengubah form menjadi baris ringkasan di Aturan pengerjaan dengan tombol Edit. Saat Edit, Batal mengembalikan nilai tersimpan; matikan checkbox lalu simpan untuk menghapus aturan dari admin dan siswa. Checkbox aktif memerlukan minimal keterangan atau link.

Revisi 8: card Yang dialami siswa pada Ringkasan admin dihapus karena mengulang tab Tes & section. Sidebar Ringkasan kini khusus Grup WhatsApp kompetisi (status/link/empty state dan petunjuk lokasi pengaturan). Form tetap satu di Pengaturan → Informasi & jadwal akses; posisi invitation siswa tidak berubah.
