---
title: Exam admin manages students
status: accepted
created: 2026-10-12
sources: ["backend/internal/service/rbac.go", "backend/internal/server/routes.go", "backend/internal/handler/admin_students.go", "backend/internal/handler/admin_students_bulk.go", "backend/internal/handler/school_scope.go", "backend/internal/service/admin_students.go", "backend/internal/service/student_bulk.go", "backend/internal/service/bulk_credentials.go", "web/app/(admin)/admin/school/students/page.tsx", "web/lib/nav-config.ts", "web/lib/hooks/use-capability.ts"]
---

# Exam admin manages students

## Problem and outcome

A client wants `admin_exam` to register exam participants directly and to control their accounts (change password, deactivate, reissue credentials). Today every `/admin/students` route is gated by `students:*`, which only `admin_school` and `super_admin` hold, and the routes are scoped to the caller's JWT school, which `admin_exam` does not have. Setting a password is further restricted to `system:admin`.

Outcome: `admin_exam` can do everything `super_admin` can do to **student accounts** — across every school, including students without a school — from the same web page `admin_school` and `super_admin` already use.

## Agreed behavior

Decisions (Panca / client, 2026-10-12):

- Target is `student` accounts only. Other admin roles stay manageable by `super_admin` alone.
- Reach is global: all schools plus students with no school. No per-exam ownership rule.
- Register: single and bulk CSV. School is chosen per registration (listed school, or the unlisted-school option); a student may have no school.
- Control: deactivate/activate, reissue credentials (single and bulk), set password manually.
- `admin_exam` also gets the registration features that are `super_admin`-only today: manual password at registration, `unlisted_school_name`, and the school/password columns in the bulk CSV.
- Every register, status change, credential reissue and password set is recorded in `audit_log`, for **every** actor role (`super_admin`, `admin_school`, `admin_exam`).
- The students page (`/admin/school/students`) appears in the `admin_exam` navigation and behaves for `admin_exam` as it does for `super_admin` (school picker, set-password action, unlisted school).

Out of scope: managing non-student accounts, per-exam participant ownership, changing what `admin_school` may do, new tables or columns, UI redesign of the students page.

## Technical contract

No migration.

### Authorization

- `service/rbac.go`: `admin_exam` gains `students:*` (satisfies the group gate on `/admin/students`) and two new capabilities, `student-passwords:write` and `student-scope:all` (the single marker for "administers students of every school"). `super_admin` already holds `*`. Neither new capability may start with `students:` — `admin_school` holds `students:*` and would otherwise inherit it.
- `routes.go`: `PATCH /admin/students/:id/password` swaps its per-route gate from `system:admin` to `student-passwords:write`. Result: `super_admin` and `admin_exam` pass; `admin_school` still gets `403`.
- `web/lib/hooks/use-capability.ts` mirrors both changes.
- Side effect, accepted: `students:*` also satisfies `adminSchoolDashboard` (`students:read`), so `admin_exam` reaches `GET /admin/dashboard/school`, which answers `403 missing school scope` because `resolveSchoolScope` needs a JWT school. No exposure; the nav does not link it.

### Scope

- New handler helper (beside `resolveResultsSchoolScope` in `school_scope.go`) `resolveStudentSchoolScope`: `CanManageAllStudents(role)` -> `resolveGlobalSchoolParam`; every other role -> `resolveSchoolScope`. Used by the student handlers that call `resolveSchoolScope` today: list, register, change status, get credentials, bulk presign, bulk reissue. `resolveSchoolScope` itself is not widened (it also guards `/dashboard/school` and other school-bound routes).
- Service methods already treat an empty `schoolID` as id-only (`GetStudentByID`, `UpdateStudentStatus`, `ResetStudentPasswordHash`); no repository change.
- Bulk reissue still requires a `school_id` (existing `school_id is required` rule), so `admin_exam` reissues per school exactly as `super_admin` does.

### `super_admin`-only checks that must also admit `admin_exam`

One service predicate, `CanManageAllStudents(role)` = `HasCapability(role, "student-scope:all")`, so the role list lives only in `roleCapabilities`, replaces the literal `RoleSuperAdmin` comparisons that guard student registration:

- handler `AdminRegisterStudent`: `unlisted_school_name`.
- service `RegisterStudentWithPassword`, `RegisterStudentUnlisted`, `RegisterStudentUnlistedWithPassword`.
- service `ProcessStudentBulkRows`: the `r.Password != nil` check.
- handler `AdminBulkImportStudents`: the `super_admin` branch (school per CSV row, storage prefix from `file_key`).
- any worker/job code that branches on `RoleSuperAdmin` for the same purpose (`EnqueueStudentBulkJob` and the student-bulk worker; read them, change only what is needed).

### Audit

`audit_log` rows via `InsertAuditLogMeta`, target type `user`, same style as `student.set_password`:

- `student.register` — written inside the shared registration path, so single and bulk-row registrations are both covered. Metadata carries `school_id` when set.
- `student.status_change` — metadata `status`.
- `student.reissue_credentials` — written inside `ReissueStudentCredentials`, so the bulk loop is covered per student.
- `student.set_password` — unchanged.

The actor is passed explicitly as a service parameter (`actorID string`, after `ctx`), as `SetStudentPassword` does; no context values. The bulk path takes the actor from the job's `created_by`. An audit-write error is returned, as `account.create` does today. The audit row never contains the password or temp password.

### Web

- `nav-config.ts`: `ADMIN_EXAM_NAV` gets the existing students item (`SCHOOL_NAV_ITEMS`) in a group of its own or the exam group — keep the diff small.
- `students/page.tsx`: today `isSuperAdmin` gates the school picker, the register-time school selection, the unlisted-school field and the set-password menu item. Replace it with one local boolean from `hasCapability(role, "student-scope:all")`, not a role comparison. Any `isSuperAdmin` use that is really about the route's school scoping follows the same boolean.
- `i18n.ts`: no new strings expected; if a string must change, both `id` and `en`.

## Acceptance and proof

Backend (DB-backed tests where the existing neighbours are DB-backed):

1. `admin_exam` token: `GET /admin/students` without `school_id` returns students of several schools plus a school-less student; with `school_id` filters to that school.
2. `admin_exam` can `POST /admin/students` with a listed school, with no school, with `unlisted_school_name`, and with a manual `password`; the manual password logs in. `admin_school` still gets `403` for `password` and `unlisted_school_name`.
3. `admin_exam` can `PATCH /admin/students/:id` (deactivate and reactivate) a student of any school and `GET /admin/students/:id/credentials` for one; the old password stops working, the new temp password works.
4. `admin_exam` can `PATCH /admin/students/:id/password`; `admin_school` still gets `403`; a non-student target id (an admin account) returns not-found, never changes the admin.
5. `admin_exam` bulk import accepts a CSV with school identity and password columns; `admin_school` bulk keeps rejecting a password column.
6. `admin_exam` bulk reissue for a school returns the credentials CSV.
7. Audit rows exist with the right actor, action and target for: single register, a bulk-registered row, status change, credential reissue (single and one bulk row) — for actors `admin_exam` and `admin_school`; set password — for `admin_exam` (`admin_school` is refused by criterion 4). No audit row contains a password.
8. `admin_store` and `student` get `403` on all of the above (existing gate tests stay green).
9. `resolveSchoolScope` still denies `admin_exam` (guard against widening it); `GET /admin/dashboard/school` stays `403` for `admin_exam`.
10. RBAC table test covers `students:*`, `student-passwords:write` for each role.

Web (vitest):

11. `hasCapability("admin_exam", "students:read")`, `"student-passwords:write"` and `"student-scope:all"` are true; `admin_school` has the first and neither of the others.
12. `ADMIN_EXAM_NAV` contains `/admin/school/students`.
13. Students page rendered as `admin_exam` shows the school picker and the set-password action; as `admin_school` it shows neither (existing test, kept).

Docs: `docs/wiki/domain/rbac-and-accounts.md` updated (capability table row for `admin_exam`, the set-password gate, the student scope helper, the audit actions). The wiki currently says `AdminChangeStudentStatus` also needs `system:admin`; `routes.go` shows only the password route does — fix the sentence to match the code.

Gates: `deploy/pipeline/backend.sh` then `frontend.sh`, sequentially. DB-backed tests that cannot run here are proven by CI.

## Notes

- Service signature changes (actor parameter) touch roughly 70 call sites, almost all in tests under `backend/internal/service` and `backend/internal/handler`; this is mechanical.
- Anyone with `admin_exam` can now set any student's password and read newly reissued credentials. That is the requested capability; the audit log is the compensating control.
- No GitHub issue exists for this; no `open-issue-map.md` change.
