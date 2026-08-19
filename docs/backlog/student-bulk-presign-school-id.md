# Bulk student register: super_admin gets `school_id is required` on Register

| | |
|---|---|
| **Status** | Confirmed from code path (click Register, before the file is parsed) |
| **Date** | 2026-08-19 |
| **Surface** | Admin student bulk register (`BulkImportModal`), `POST /admin/students/bulk/presign` |
| **Not the bug** | CSV `school` column (name such as `MAN 1 Brebes`); enqueue + worker name lookup |

Clicking Register with a bulk siswa file that has a `school` **name** column returns **400 `school_id is required`**. The file is never uploaded or parsed. This hits **super_admin** (no JWT school). `admin_school` is unaffected (scope comes from the token).

Example row (school is a name, as the template intends):

```
name,school,jenjang,email,...
RAAFI NUGRAHA,MAN 1 Brebes,SMA,raafinugrahaaa@gmail.com,...
```

---

## Root cause

Bulk register is two calls:

1. `POST /admin/students/bulk/presign?filename=&content_type=` — get a MinIO PUT URL
2. `POST /admin/students/bulk` `{ file_key }` — enqueue the job

The 400 is **step 1**, from `resolveSchoolScope` in [`school_scope.go`](../../backend/internal/handler/school_scope.go): for `super_admin`, school is `?school_id=` on the query string. Empty → `school_id is required`.

The UI never sends `school_id`. Product design is per-row school **name** in the CSV (`BulkImportModal` template; worker `GetSchoolByNameCI`).

**Enqueue already matches that design.** `AdminBulkImportStudents` skips `resolveSchoolScope` for super_admin and treats school as per-row. **Presign was never updated**, so the two endpoints disagree.

Object keys are still `student-bulk/{schoolID}/…`. Enqueue for super_admin extracts the second path segment as a UUID for prefix checks only — leftover from when presign required a real school id.

---

## What is *not* wrong (for this 400)

- Missing `school_id` column in the CSV. The template uses `school` (name).
- `RegisterStudent` requiring school: school is optional there; bulk resolves name → id before calling it.
- `admin_school` JWT scope.

---

## Follow-on risks (after this 400 is fixed)

These will not cause `school_id is required`, but can fail the job next:

1. **Delimiter.** Operators often paste Excel as **TSV**. Parser is comma-only (`encoding/csv`). TSV → one header cell → missing CSV header.
2. **School name.** Must match `school.name` case-insensitively. Unknown name → per-row school-not-found.
3. **`jenjang` casing.** UI uses `SMA`; many `school_types` / tests use `sma`. Comparison is exact → invalid jenjang if types are set.

Empty `dob` / address columns are already optional.

---

## Proposed fix

1. **Align presign with enqueue for super_admin (the actual fix)**
   - Super_admin: do not call `resolveSchoolScope` on presign.
   - Object key still needs a folder. Prefer **A** (smallest change): random UUID folder `student-bulk/{upload-uuid}/file.csv` — storage namespace, not a real school. Enqueue already UUID-parses the second segment.
   - Alternative **B**: no school segment (`student-bulk/{uuid}-filename`, like school-bulk). Cleaner, but enqueue, worker result-key derivation, and prefix checks all change.
   - `admin_school` stays on `resolveSchoolScope` (JWT).
   - Test: super_admin presign without `?school_id=` must not return `school_id is required`.
2. **Keep the frontend as-is.** Do not add a school picker just to satisfy presign; that fights the per-row `school` column.
3. **Parser (separate):** detect tab vs comma, or fail with a clear “comma CSV only” message; `TrimSpace` name/school/jenjang; optionally compare `jenjang` case-insensitively to `school_types`.
4. **Operator errors:** map parse failures (wrong delimiter, missing `school` header) to a message that names the template columns.

**Ship order:** (1) + test first — unblocks Register. Then (3) if Excel/TSV should work. Then (4). (2) is no change.

---

## Acceptance

- Super_admin can click Register on a valid comma CSV with `name,school,jenjang` and **not** get `school_id is required` from presign.
- Each row still resolves `school` by name; `admin_school` remains bound to JWT school.
- Frontend still does not send `?school_id=` on presign/enqueue.

---

## Related files

- [`backend/internal/handler/school_scope.go`](../../backend/internal/handler/school_scope.go) — `resolveSchoolScope` (`school_id is required`)
- [`backend/internal/handler/admin_students_bulk.go`](../../backend/internal/handler/admin_students_bulk.go) — `AdminPresignStudentBulkUpload` vs `AdminBulkImportStudents`
- [`backend/internal/service/job.go`](../../backend/internal/service/job.go) — `GeneratePresignedPrivateUploadURL` / `EnqueueStudentBulkJob` key prefix
- [`backend/internal/service/student_bulk.go`](../../backend/internal/service/student_bulk.go) — parse + `GetSchoolByNameCI`
- [`backend/internal/worker/student_bulk.go`](../../backend/internal/worker/student_bulk.go) — result key from input path
- [`web/lib/hooks/admin-students-bulk.ts`](../../web/lib/hooks/admin-students-bulk.ts)
- [`web/components/admin/BulkImportModal.tsx`](../../web/components/admin/BulkImportModal.tsx)
