# Single student register without email: duplicate `idx_users_email_active`

| | |
|---|---|
| **Status** | Implemented |
| **Date** | 2026-08-20 |
| **Surface** | Admin single student register (`POST /api/v1/admin/students`), register dialog |
| **Not the bug** | Email being required; bulk CSV parser; `?school_id=` scope |

Bulk CSV with empty email succeeds. Single register with email left blank hits unique violation `idx_users_email_active` and is logged as unhandled 500.

---

## Root cause

Partial unique index in [`0002_identity.up.sql`](../../backend/db/migrations/0002_identity.up.sql):

```sql
CREATE UNIQUE INDEX idx_users_email_active ON users (email)
  WHERE email IS NOT NULL AND status != 'deleted';
```

`NULL` is excluded from the index (many students can have no email). `''` is not `NULL`, so the second blank email collides.

Bulk already converts empty cells to `nil` → SQL `NULL` (`optionalStr` in [`student_bulk.go`](../../backend/internal/service/student_bulk.go)). Single register does not:

- UI always posts `email: ""` from the register dialog (`registerForm` init).
- Handler binds `Email *string` and forwards `""`.
- `CreateStudent` only lowercases a non-nil pointer; it never nils blanks.

`RegisterStudent` did not check email uniqueness before insert (self-register and admin-account create do). Real duplicates and a blank-email unique collision both became `unhandled service error`.

If the first single already failed, a row with `email = ''` already exists (earlier UI register). Bulk `NULL` rows do not collide with it; another `''` does.

---

## What is *not* wrong

- Email as a required field. Username still satisfies `CHECK (email IS NOT NULL OR username IS NOT NULL)`.
- Bulk CSV empty-email handling (already `NULL`).
- School scope on the query string.

---

## Proposed fix

1. **Normalize at write.** `normalizeOptionalEmail`: nil / whitespace-only → `nil`, else trim + lowercase. Use in `CreateStudent`, `CreateUser`, `CreateAdminUser`, `UpdateUserProfile`.
2. **Taken email.** When a non-blank email is given, `checkEmailUniqueness` (same helper as admin-account create) returns `ErrEmailTaken` → 409 `email_taken`.
3. **Frontend.** Omit empty optional `email` (and other empty optional strings) from the register JSON. Not sufficient alone.
4. **Do not change `idx_users_email_active`.** Uniqueness on real emails stays as in 0002. Go already stores blanks as `NULL`, so new registers do not occupy the index. Leftover `email = ''` rows (from earlier UI registers) do not block new `NULL` emails; clean them by hand if desired:

```sql
UPDATE users SET email = NULL
WHERE email IS NOT NULL AND btrim(email) = '';
```

Do not require email. Do not only change the React form.

---

## Acceptance

- Two single registers with omitted / `""` / whitespace-only email both succeed; persisted `email IS NULL`.
- Two registers with the same real email return 409 `email_taken`, not 500.
- Bulk empty email column still creates multiple `NULL` emails.

---

## Related files

- [`backend/internal/repository/user.go`](../../backend/internal/repository/user.go) — `normalizeEmail` / `normalizeOptionalEmail`
- [`backend/internal/repository/admin_students.go`](../../backend/internal/repository/admin_students.go) — `CreateStudent`
- [`backend/internal/service/admin_students.go`](../../backend/internal/service/admin_students.go) — `RegisterStudent`
- [`backend/internal/handler/admin_students.go`](../../backend/internal/handler/admin_students.go)
- [`web/app/(admin)/admin/school/students/page.tsx`](../../web/app/(admin)/admin/school/students/page.tsx)
