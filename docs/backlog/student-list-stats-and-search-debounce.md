# Student module stat cards only count the first page, search fires per keystroke

| | |
|---|---|
| **Status** | Fixed |
| **Date** | 2026-08-30 |
| **Surface** | Admin Sekolah → Siswa (`/admin/school/students`), `GET /admin/students` |
| **Related** | [school-bulk-list-pagination.md](school-bulk-list-pagination.md) — the same bug shape, previously fixed on the schools module |

The Siswa (students) module shipped with two defects that are a close copy of
what the schools module had before its pagination fix
([school-bulk-list-pagination.md](school-bulk-list-pagination.md)):

1. **Stat cards derived from locally loaded rows.** Total / Aktif / Nonaktif
   were computed client-side from the accumulated pages
   (`accumulated.length`), so with more students than one page (default limit
   20) the cards showed "Total ≈ 20" and stayed there no matter how often the
   operator clicked "Load more".
2. **Search was not debounced.** The raw input value was passed straight into
   the list query as the `q` param, so every keystroke fired a new paginated
   request, reset the accumulated list back to page 1, and produced a burst of
   near-identical API calls while typing.

---

## Root cause

### 1. No filter-aware counts from the server

`GET /admin/students` returned only `{ data, next_cursor }`
([admin_students.go](../../backend/internal/handler/admin_students.go)). The
frontend therefore had no way to know the size of the whole filtered set and
fell back to counting the rows it happened to have loaded — the exact mistake
described in the schools backlog's root cause #2 ("Stat cards … count that
local array, not `count(*)` from the DB").

### 2. Search value fed to the query un-debounced

[`web/app/(admin)/admin/school/students/page.tsx`](../../web/app/(admin)/admin/school/students/page.tsx)
bound the search input directly to state that was also used as the `q` query
param and inside the `filterKey` pagination guard. Each keystroke changed the
filter key, which reset the accumulated list and cursor, and triggered a fresh
request — one request per character typed.

---

## What is *not* wrong

- Cursor pagination itself (keyset on `created_at DESC, id DESC`) is correct;
  pages walk every student exactly once.
- The school picker for super_admin and the status chips are unrelated; they
  pass through the same fixed pipeline unchanged.
- The exam participant picker (`ParticipantPicker.tsx`) shares
  `useAdminStudents` but only reads `data`, so the added response fields do
  not affect it.

---

## Fix (applied 2026-08-30)

Mirror the schools module fix:

1. **Server-side filter-aware counts.** `ListStudents` now also returns
   `repository.StudentAdminCounts` via the new
   `Repository.CountStudentsAdmin` — `COUNT(*)` plus
   `COUNT(*) FILTER (WHERE status = 'active' / 'deactivated')` over the same
   school scope and status/q/grade/jenjang predicates as the page query,
   ignoring cursor, limit, and the exam-eligibility filter. The shared
   predicates were extracted into `appendStudentFilterSQL` so the list and
   count queries cannot drift. The handler response now carries
   `total`, `active`, and `deactivated`.
2. **Debounced search on the page.** The input binds to `searchInput`; a
   300 ms timeout (`SEARCH_DEBOUNCE_MS`) produces `debouncedSearch`, which is
   what feeds both the `q` param and `filterKey` — identical to the schools
   page.
3. **Stats from the server.** The page keeps `stats` in state and fills it
   from `total` / `active` / `deactivated` in the accumulate effect (guarded
   by the existing filter-key check so stale pages cannot overwrite it).

---

## Acceptance

- With N students in scope, the Total / Aktif / Nonaktif cards show the
  DB-wide filtered counts, not the number of rows loaded so far.
- Counts respect the active filters (status, q, grade, jenjang, school scope)
  and ignore cursor/limit — requesting `limit=1` still returns the full-set
  counts.
- Typing in the search box fires exactly one request after the user stops
  typing (300 ms), not one per keystroke, and pagination restarts at page 1
  under the final filter.
- Covered by tests: `TestCountStudentsAdmin_matchesFilteredTotals` and
  `TestAdminListStudents_ReturnsFilterAwareCounts` (backend); the students
  page tests assert server-driven stat cards and the debounce window.

---

## Related files

- `backend/internal/repository/admin_students.go` — `ListStudentsBySchool`,
  `appendStudentFilterSQL`, `CountStudentsAdmin`
- `backend/internal/service/admin_students.go` — `ListStudents`
- `backend/internal/handler/admin_students.go` — `AdminListStudents`
- `web/lib/hooks/admin-students.ts` — `useAdminStudents` response type
- `web/app/(admin)/admin/school/students/page.tsx`
- `web/app/(admin)/admin/school/students/page.test.tsx`
