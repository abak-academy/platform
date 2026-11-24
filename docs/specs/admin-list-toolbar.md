---
title: One list toolbar for every admin list page
issue: none yet
status: proposed
created: 2026-11-17
sources: ["web/components/admin/OrdersToolbar.tsx", "web/components/admin/ProductsToolbar.tsx", "docs/specs/admin-orders-toolbar-redesign.md", "web/components/ui/popover.tsx", "web/components/ui/select.tsx", "web/components/ui/data-table.tsx", "web/app/(admin)/admin", "docs/prototypes/redesign-v2/index.html"]
---

# One list toolbar for every admin list page

## Problem and outcome

Each admin list has its own search, filter, and sort controls. Orders has `OrdersToolbar` (search, Filters popover, sort, chips). Products has `ProductsToolbar` with a different layout. Other pages use tab strips, inline selects, or nothing. Admins relearn the controls on every page.

Outcome: one `ListToolbar` component used by every admin list page, with the layout already approved for orders in `admin-orders-toolbar-redesign.md`, generalized.

## Agreed behavior

Decision (Panca, 2026-11-17): every admin list page uses the same toolbar layout. Tabs are only for switching views of a single record, never for filtering a list.

Layout, in this order:

1. **Primary row**: search input (pill, clear button when filled) · **Filter** button showing the active-filter count · **sort** select (only when the page has more than one sort) · result count right-aligned ("12 PESANAN").
2. **Filter panel** (popover from the Filter button): one group per filter, each a single-select chip row whose first option is "Semua". Footer has "Reset filter" and "Tampilkan N hasil", which closes the panel. The panel closes on Escape and outside click. Picking a chip keeps the panel open.
3. **Active-filter chips** below the row when any filter is set: "Label: value ×", plus "Hapus semua". Search and sort are not chips.
4. Table pages render the toolbar as the card header above the table. Grid pages (cards) render it as a standalone bar above the grid.
5. Empty result: the same empty state everywhere ("Tidak ada hasil · Ubah kata kunci atau filter").

Per-page configuration (from the prototype; adjust to fields the API actually returns):

| Page | Search | Filters | Sorts |
|---|---|---|---|
| Pesanan | no. pesanan, pembeli, produk | Status (7, per orders spec), Jenis (fisik/digital), Kurir, Tanggal | per orders spec |
| Produk | nama, kategori | Jenis, Status (terbit/draf) | Nama, Harga ↑↓, Stok ↑↓ |
| Kursus | judul, pengajar | Status, Mata pelajaran | Siswa terbanyak, Judul |
| Promo | kode, keterangan | Status (aktif/nonaktif/kedaluwarsa) | Paling dipakai, Berakhir tercepat |
| Ujian (packages) | nama, event | Event, Status fase, Produk (terbit/draf) | Jadwal terdekat/terjauh, Pendaftar |
| Tes | nama | Pemakaian (dipakai/belum) | Nama, Jumlah soal |
| Bank Soal | isi soal, topik | Topik, Tipe, Tingkat | — |
| Sekolah | nama, NPSN | Provinsi, Admin sekolah (ada/belum) | Siswa, Nama |
| Siswa | nama, sekolah | Jenjang, Sekolah | Bergabung, Nama, Ujian |
| Affiliator | nama, kode | Level, Status | Penjualan, Nama |
| Penarikan | nama affiliator | Status | Terbaru, Jumlah |
| Akun | nama | Peran | — |
| Audit | aktivitas | Pelaku (sistem/admin) | Terbaru, Terlama |

Monitor Sesi keeps its event picker in the page header; it is a context switch, not a list filter.

Dashboard cards that deep-link to a filtered list (for example "Perlu konfirmasi") set the matching filter, as they do for orders today.

## Technical contract

- New `web/components/admin/ListToolbar.tsx`. It is presentational and controlled: `value: { q: string; filters: Record<string, string>; sort?: string }`, `onChange`, `filters: { key; label; options: { value; label }[] }[]`, `sorts?: { value; label }[]`, `count: number`, `unit: string`, `variant: "card-header" | "standalone"`.
- Pages own the query. Pages with server-side lists (orders) map `value` to their existing API query; pages that filter in memory (products) keep a pure `apply*Query` helper like `applyProductsQuery`.
- `OrdersToolbar` becomes a thin wrapper over `ListToolbar` (its date filter stays a custom panel group), and `ProductsToolbar` is replaced. Their existing tests are migrated, not deleted.
- Uses `ui/popover`, `ui/select`, `ui/input`, `ui/badge`. No new dependency.
- i18n: shared keys (`list_filter`, `list_reset_filters`, `list_show_n_results`, `list_clear_all`, `list_no_results`, `list_no_results_hint`, `list_all`) in id and en. Page-specific filter labels stay page-scoped.
- Page size and Prev/Next stay in the table footer (per the orders spec); not part of the toolbar.

**Uncertain:** which admin lists are server-paginated today apart from orders. Verify each page's hook before deciding where filtering happens. A server-paginated list must not be filtered client-side.

## Acceptance

1. Every page in the table above renders `ListToolbar` and no other search/filter control (grep: no `ProductsToolbar` and no ad-hoc search inputs in `app/(admin)/admin/**/page.tsx`).
2. `ListToolbar` tests: count badge, chip removal, reset, outside-click close, panel stays open on chip pick, Escape closes.
3. Orders behavior and its existing acceptance criteria are unchanged.
4. Pesanan, Produk, and Bank Soal screenshots in light and dark mode match the prototype layout.

## Non-goals

- No new backend filter parameters. If a filter in the table needs one, drop it from this phase and list it as follow-up.
- No table/column redesign.
