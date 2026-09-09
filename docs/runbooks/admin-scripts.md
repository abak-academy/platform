# Admin ops scripts (`scripts/`)

One folder per script under `scripts/`, each a standalone `package main`
driven by a CSV. Before running, edit the const block at the top of
`main.go` (`baseURL`, `token`, `inputFile`, `outputFile`). Run from the
repo root:

```sh
go run scripts/<folder>/main.go
```

Token: log in as an admin in the web app, then copy `state.token` from
the localStorage key `abak-auth` (DevTools → Application → Local
Storage). Scopes needed are listed per script. Exit codes: `0` all rows
processed, `1` some rows failed (check the result CSV), `2` config/IO
error (e.g. token const empty).

Exam/test ID candidates can be pulled from vm-db first:

```sql
SELECT id, title FROM test WHERE title ILIKE '%0209%';
SELECT id, title FROM exam;
```

---

## 1. Add test to exam — `scripts/add_exam_tests/`

Attaches tests to exams via `PUT /api/v1/admin/exams/:id/tests`. That
endpoint is replace-semantics (the body is the FULL attached list), so
the script GETs the exam detail first, merges the requested test IDs
(already-attached ones are skipped), then PUTs the merged list once per
exam.

Scopes: `products(exam):write` (PUT) + `products(exam):read` (GET).

Input `scripts/add_exam_tests.csv` (header required; `test_id` accepts a
single UUID, a comma-separated list in quotes, or a JSON array):

```csv
exam_id,test_id
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,22222222-2222-2222-2222-222222222222
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,11111111-1111-1111-1111-111111111111
cccccccc-cccc-cccc-cccc-cccccccccccc,"44444444-4444-4444-4444-444444444444,55555555-5555-5555-5555-555555555555"
```

Output `scripts/add_exam_tests_result.csv`:

```csv
exam_id,test_id,status,http_status,error
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,22222222-2222-2222-2222-222222222222,attached,204,
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,11111111-1111-1111-1111-111111111111,skipped,204,already attached
cccccccc-cccc-cccc-cccc-cccccccccccc,44444444-4444-4444-4444-444444444444,attached,204,
cccccccc-cccc-cccc-cccc-cccccccccccc,55555555-5555-5555-5555-555555555555,attached,204,
```

`attached` = included in the successful PUT; `skipped` = already
attached before this run; `failed` rows carry the API error message.

---

## 2. Grant student to exam — `scripts/grant_exam/`

Bulk-enrolls students into an exam. Resolves each username via
`GET /api/v1/admin/exam-grants/students/search?q=<username>` (exact
username match, paginated), then batches
`POST /api/v1/admin/exam-grants` (20 students per call) per exam.

Scope: `exam-grants:write` — `super_admin` only (see
[rbac-matrix.md](rbac-matrix.md)).

Input `scripts/exam_grants.csv` (header required; `username` accepts a
single username, a comma-separated list in quotes, or a JSON array):

```csv
exam_id,username
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,zalf6539
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,"user-a,user-b"
```

Output `scripts/exam_grants_result.csv`:

```csv
exam_id,username,student_id,status,http_status,name,error
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,zalf6539,cccccccc-cccc-cccc-cccc-cccccccccccc,granted,201,Zalfaa,
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,user-a,dddddddd-dddd-dddd-dddd-dddddddddddd,skipped,201,User A,already registered
aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,user-b,,failed,200,,username not found
```

`granted` = enrolled by this run; `skipped` = already registered;
`failed` rows carry the reason (lookup failure with the search HTTP
status, or the grant batch error).
