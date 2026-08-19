# Bulk school import succeeds in DB, Sekolah module list only shows ~20

| | |
|---|---|
| **Status** | Confirmed local + production |
| **Date** | 2026-08-19 |
| **Surface** | Admin Sistem → Sekolah (`/admin/system/schools`), `GET /admin/schools` |
| **Not the bug** | Bulk CSV writer (`ProcessSchoolBulkRows` → `CreateSchool`) |

Bulk import of ~122 schools wrote rows (`status=success` in the result CSV; `SELECT count(*) FROM school` matches). The Sekolah list still showed about **20** schools. Search in the UI did not find names that exist in the `school` table.

The import path is fine. The admin **list** is broken.

---

## Root cause

### 1. Cursor pagination does not match `ORDER BY name` (primary)

[`ListSchoolsAdmin`](../../backend/internal/repository/school.go) sorts by **name**, but the cursor is `WHERE s.id > $cursor`. `next_cursor` is the **id** of the extra (21st) row, not a `(name, id)` keyset.

“Load more” means “UUIDs greater than this, then sort by name again” — not the next page by name. Result: skipped rows, possible duplicates, and most bulk-imported schools never appear.

Handler default limit is **20** (max 100). There is no server-side search or status filter.

[`AdminListSchools`](../../backend/internal/handler/admin_school.go) only accepts `cursor` and `limit`.

### 2. Search and status filters are client-only

[`web/app/(admin)/admin/system/schools/page.tsx`](../../web/app/(admin)/admin/system/schools/page.tsx) filters the already-loaded `schools` array. Searching a name/code past the first page shows “No schools found.” Stat cards (Total / Active) count that local array, not `count(*)` from the DB.

### 3. List state is not reset after a successful bulk job

[`SchoolBulkImportModal`](../../web/components/admin/SchoolBulkImportModal.tsx) only `invalidateQueries(adminSchoolsKeys.all)`. The page does not reset `fetchCursor` / `schools`, so the UI can keep showing whatever page was already loaded.

---

## What is *not* wrong

- Bulk and single create use the same insert: `INSERT INTO school (…, status) VALUES (…, 'active')`.
- `school` has no tenant/region/soft-delete columns. Listing is a global registry.
- Result CSV `success` is emitted only after `CreateSchool` returns nil.

---

## Repro (local)

Requires `super_admin` (`GET/POST /admin/schools` is gated on `schools:write`). Local login used: `schooladmin@local.test` / `password123` (role promoted to `super_admin` in this environment).

Postgres (DBeaver / `deploy/compose/local.yml`):

| Field | Value |
|---|---|
| Host | `localhost` |
| Port | `5432` |
| Database | `akademi` |
| User / password | `akademi` / `akademi` |

1. Bulk-import a CSV (`name,code,npsn,school_types,alamat`) with ≥ 21 rows, or use schools already in the DB.
2. Check:

```sql
SELECT count(*) FROM school;
SELECT name, code FROM school ORDER BY name DESC LIMIT 5;
```

3. Open `/admin/system/schools`. Table / Total ≈ **20**, not the DB count.
4. Search a name that sorts late in the alphabet → empty.
5. Compare with `GET /api/v1/admin/schools?limit=100` — JSON includes schools the UI does not show. Click “Load more” several times; completeness/order will not match `ORDER BY name`.

---

## Proposed fix

1. **Keyset pagination** on `(name, id)`: `ORDER BY s.name ASC, s.id ASC`, cursor `(name, id)` — not `id > uuid` alone.
2. **Server-side search and status** (`q`, `status`) so filters are not limited to the first 20 loaded rows.
3. After bulk job `succeeded`: reset `fetchCursor` / `schools` and refetch page 1.
4. Drive Total from a server count, or paginate to exhaustion with a correct cursor.
5. Tests: seed > 20 schools, walk `next_cursor` to the end; each id appears **exactly once**, name order is stable. Frontend: searching a name that is not on page 1 still finds the row.

---

## Acceptance

- After a successful bulk of N schools, the Sekolah page can show all N (correct load-more with no skip/dupes, and/or server-side search).
- Walking `GET /admin/schools` via `next_cursor` visits every row exactly once, ordered by name.
- Searching a name/code that exists in the DB does not return empty only because the row was not loaded on the client yet.
- Single create and bulk still insert `status='active'` as they do today.

---

## Related files

- `backend/internal/repository/school.go` — `ListSchoolsAdmin`
- `backend/internal/handler/admin_school.go` — `AdminListSchools`
- `backend/internal/service/school.go` — `AdminListSchools` / `CreateSchool`
- `backend/internal/service/school_bulk.go` — import (OK)
- `web/app/(admin)/admin/system/schools/page.tsx`
- `web/lib/hooks/admin-schools.ts`
- `web/components/admin/SchoolBulkImportModal.tsx`
