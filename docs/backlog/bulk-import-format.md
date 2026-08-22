# Bulk import format: templates, field rules, and parser hardening

| | |
|---|---|
| **Status** | Shipped — corrected CSV templates, Bahasa field guide in the modal + downloadable `.txt`, tolerant parsers |
| **Date** | 2026-08-22 |
| **Surface** | Admin student bulk register (`BulkImportModal`); admin school bulk import (`SchoolBulkImportModal`); `ParseStudentBulkCSV` / `ParseSchoolBulkCSV` |
| **Not this change** | NPSN format/uniqueness; DB unique index on `school.code`; TSV / semicolon Excel exports; upsert on re-upload |

Operators could download a “format” CSV, but the shipped example rows were partly unimportable, and the modal did not explain field rules. Filling the template by analogy (short city name, lowercase jenjang, Excel “CSV UTF-8”) failed the import.

Related earlier notes: [student bulk presign school_id](student-bulk-presign-school-id.md), [school bulk list pagination](school-bulk-list-pagination.md).

---

## What was wrong with the old templates

Student example used `sma` and `Jawa Barat,Bandung,Coblong`. Regions are seeded in Kemendagri form (`JAWA BARAT`, `KOTA BANDUNG`, `COBLONG`). City lookup is the full official name (case-insensitive), so `Bandung` is `invalid kota`. `jenjang` vs `school_types` used to be case-sensitive, so `sma` failed against UI-typed `SMA`. School template shipped `sma|smk` — the same casing trap.

Parser issues that failed a whole file or a valid-looking row:

- Excel UTF-8 BOM turned `name` into `\ufeffname` → missing header.
- One ragged row aborted the batch (`encoding/csv` field-count lock) with `invalid csv` and no line number.
- Cells were not trimmed (` SMAN 1 Jakarta` → school not found).
- Result CSV had no `row` column.
- Duplicate email surfaced a raw Postgres `23505` string.

---

## What the operator sees now

Download template (two example rows) and **Unduh panduan**. The modal has a collapsible table: Kolom / Wajib / Aturan / Contoh. Copy is Bahasa first (English locale mirrors). Field spec lives in [`web/lib/bulk-import-format.ts`](../../web/lib/bulk-import-format.ts) so the CSV, the table, and the `.txt` cannot drift.

Student template:

```
name,school,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos
Budi Santoso,SMAN 1 Jakarta,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132
Siti Aminah,SMAN 1 Jakarta,SMA,,,,,,,,,,
```

School template:

```
name,code,npsn,school_types,alamat
SMAN 1 Jakarta,SMAN1JKT,20100001,SMA|SMK,"Jl. Sudirman No. 1"
SMPN 5 Bandung,SMPN5BDG,,SMP,
```

### Field rules (siswa)

| Column | Rule |
|---|---|
| `name` | Wajib. Nama lengkap. |
| `school` | Wajib. Nama sekolah **sudah ada di database** (case-insensitive). Admin sekolah: setiap baris harus sekolah sendiri. |
| `jenjang` | Wajib. Disarankan huruf besar (`SD`…`S2`). Harus cocok `school_types` jika sekolah punya daftar jenis. |
| `email` | Opsional. Jika diisi, belum terdaftar. |
| `dob` | Opsional. `YYYY-MM-DD`. |
| `gender` | Opsional. `male`/`m` atau `female`/`f`. |
| `grade` | Opsional. Bilangan bulat. |
| `target_exam` | Opsional. Teks bebas. |
| `alamat_domisili` | Opsional. Kutip jika ada koma. |
| `provinsi` / `kota` / `kecamatan` | All-or-nothing. Nama resmi Kemendagri yang **sudah ada di database** (contoh `KOTA BANDUNG`, bukan `Bandung`). |
| `kode_pos` | Opsional. Hanya angka. Format sel sebagai teks di Excel. |

Kolom `nis` diabaikan. Maksimal 1000 baris.

### Field rules (sekolah)

| Column | Rule |
|---|---|
| `name` | Wajib. |
| `code` | Wajib. **Belum ada di database** (unik). Re-upload file yang sama gagal. |
| `npsn` | Opsional. Format dan keunikan tidak dicek. |
| `school_types` | Opsional. Dipisah `\|` atau koma. Disarankan huruf besar (`SMA\|SMK`). |
| `alamat` | Opsional. |

---

## Parser / validation changes

Shared helpers: [`backend/internal/service/bulk_csv.go`](../../backend/internal/service/bulk_csv.go) — strip BOM, `FieldsPerRecord = -1`, trim cells, skip blank rows.

- Result CSV leading column `row` (1-based spreadsheet line, header = 1).
- `jenjangInSchoolTypes` uses `strings.EqualFold`.
- `RegisterStudent` maps Postgres `23505` to `ErrEmailTaken`.

Backend tests keep a byte-for-byte copy of each frontend template (`frontendStudentBulkTemplateCSV`, `frontendSchoolBulkTemplateCSV`).

---

## Out of scope (still true)

- NPSN format and uniqueness.
- Unique index on `school.code` (application-level only today).
- Tab/semicolon delimited Excel exports — still comma CSV only.
- Upsert: duplicate school `code` is a per-row failure, not an update.

---

## Related files

- [`web/lib/bulk-import-format.ts`](../../web/lib/bulk-import-format.ts)
- [`web/components/admin/BulkFormatGuide.tsx`](../../web/components/admin/BulkFormatGuide.tsx)
- [`web/components/admin/BulkImportModal.tsx`](../../web/components/admin/BulkImportModal.tsx)
- [`web/components/admin/SchoolBulkImportModal.tsx`](../../web/components/admin/SchoolBulkImportModal.tsx)
- [`web/lib/i18n.ts`](../../web/lib/i18n.ts) — `bulk_format_*`
- [`backend/internal/service/student_bulk.go`](../../backend/internal/service/student_bulk.go)
- [`backend/internal/service/school_bulk.go`](../../backend/internal/service/school_bulk.go)
- [`backend/internal/service/admin_students.go`](../../backend/internal/service/admin_students.go)
