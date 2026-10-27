---
title: RBAC and Accounts
updated: 2026-10-27
verified_against: c70f8827
---

One-liner: 6 roles, a static `role → capability` map, echo middleware that checks the map on
every request, plus a school-scope resolver for the one role (`admin_school`) whose rows need
narrowing below the role level. Full route-by-route matrix: [../../runbooks/rbac-matrix.md](../../runbooks/rbac-matrix.md)
(disagreements with current code noted below — read that file, don't re-derive it here).

## Roles

`student`, `admin_exam`, `admin_store`, `admin_school`, `super_admin`, `affiliator` — constants in
`backend/internal/service/rbac.go`. Only the 4 admin roles are assignable via
`/admin/system/accounts` (`adminRoles` map, `admin_users.go:14-19`); `affiliator` is created and
promoted only through `/admin/system/affiliators` (see Affiliators below); `student` accounts are
created only via `/auth/register` or `/auth/google`, never by an operator.

## Capabilities (`roleCapabilities`, `rbac.go:13-23`)

| Role | Capabilities |
|---|---|
| `student` | none |
| `admin_exam` | `questions:*`, `tests:*`, `products(exam):*`, `products(course):*`, `sections:*`, `sessions:*`, `uploads:write`, `results:read`, `students:*`, `student-passwords:write`, `student-scope:all`, `results-answers:read`, `results-workspace:read`, `exam-grants:write`, `product-relations:write`, `schools:read` |
| `admin_store` | `products(book\|course):write`, `sections:*`, `orders:*`, `promos:*`, `notifications:*`, `uploads:write` |
| `admin_school` | `students:*`, `results:read`, `bulk-exam-orders:write`, `products(exam):read` |
| `super_admin` | `*`, `schools:write` |
| `affiliator` | `affiliates:self`, `students:*`, `student-scope:affiliator` |

`admin_store` deliberately holds no `revenue:read` — aggregate revenue reporting is
`super_admin`'s; store admins still see per-order totals on the orders page (`rbac.go:16-19`
comment). `schools:write` on `super_admin` is inert — `*` already covers it.

`student-passwords:write` is named outside the `students:` namespace on purpose. `admin_school`
holds `students:*` and `HasCapability` is plain string matching, so a
`students:passwords:write` entry would have silently handed `admin_school` the ability to set
any student's password. `admin_exam` and `super_admin` hold it; `admin_school` keeps
`students:*` (and therefore `students:read`, which is what `adminSchoolDashboard` gates on)
without gaining password control. `student-scope:all` marks "administers students beyond its own
school" for the same reason (outside `students:`). Service-side, `CanManageAllStudents(role)`
(`rbac.go:56-58`) is the single predicate for it — `HasCapability(role, "student-scope:all")`,
so `super_admin` (via `*`) and `admin_exam` — used by the handler, `resolveStudentSchoolScope`
and the registration/bulk services; the students page reads the same capability from its mirror
in `use-capability.ts`. Changing who qualifies means editing `roleCapabilities` only.

`results-answers:read` switches `GET /admin/results/export` from the scores-only CSV to the
per-question `Qn Answer`/`Qn Points` CSV (`admin_results.go`); `admin_exam` and `super_admin` hold
it, `admin_school` does not. The all-attempts variant (`?attempts=all`) needs
`results-workspace:read`, which `super_admin` and `admin_exam` hold; `admin_school` and the other
roles do not. The workspace routes (`/admin/exams/:id/results-workspace*`) are gated by the same
capability and take no school scope, so `admin_exam` sees every school. The exam detail page
reads it from `use-capability.ts` to swap the Results/Leaderboard tabs for the workspace.

`exam-grants:write` gates the whole `/admin/exam-grants` group (grant, revoke, student search);
`admin_exam` and `super_admin` hold it, so both enrol participants directly with no order or
payment, and each grant writes an `exam_grant.create` audit row. The four bulk CSV routes
(`bulk`, `bulk/presign`, `revoke/bulk`, `revoke/bulk/presign`) add `exam-grants-bulk:write` per
route, which only `super_admin`'s `*` satisfies, so `admin_exam` grants and revokes one batch of
picked students at a time but cannot upload a CSV. The registrations tab reads both capabilities
(`ExamRegistrationsTab.tsx`). `product-relations:write` gates `PUT /admin/products/:id/relations`
at the route and again in `UpdateProductRelations`; the "no live order" guard still applies to
every role.

`schools:read` gates only `GET /admin/schools` (the sibling group `adminSchoolsRead`); school
create/update/status/bulk and `/admin/schools/options` stay `schools:write`, so `admin_exam` can
list schools but not change them. It exists for the school dropdown in the results workspace and
in the grant/revoke picker (`SchoolFilterPicker`), which reads that list.

`HasCapability(role, required)` (`rbac.go:33-53`): true if the role's list contains `"*"`, an
exact match for `required`, or an entry `x:*` where `required` starts with `x:`. This is purely
mechanical string matching — there is no notion of a capability hierarchy beyond the `:*` suffix
convention.

**The JWT's `capabilities` claim is decorative.** `mintSession` bakes `Capabilities(role)` into
the access token at sign time (`auth.go:419-420`), but `RBACMiddleware` never reads it back — it
calls `service.HasCapability(claims.Role, required)` fresh against the live map on every request
(`middleware.go:45`). A role's capabilities can change (deploy) without invalidating outstanding
tokens either way, since only `Role` — not `Capabilities` — is ever consulted.

## Affiliators

An `affiliator` is a role-level account bound to several schools, not one: `users.school_ids UUID[]`
(migration `0070`), no `school_id` and no foreign key (the service validates every id against
`school`, active only, when super admin sets them). `student-scope:affiliator` is named outside
`students:` for the same reason as `student-scope:all`, and `super_admin`'s `*` also satisfies it,
so scope code compares the role (`claims.Role == RoleAffiliator`), never the capability.

- **Accounts** (`service/affiliator.go`, routes under `/admin/system/affiliators`, `system:admin`
  so super admin only): create, bulk create (JSON rows, at most 200, per-row result), promote an
  `admin_school` account (clears its `school_id`, revokes its sessions), replace the school list,
  list with student/order/paid figures, detail with the latest orders. Active state reuses
  `PATCH /admin/system/accounts/:id/status`. Every change writes an `affiliator.*` audit row.
- **School scope**: `resolveStudentSchoolScope` sends an affiliator to `resolveAffiliatorSchool`
  (`school_scope.go`): `?school_id=` is required for single-school actions and must be in the
  list, which is read from the database on every request, not from the token (403
  `school_not_assigned`). The list endpoint instead covers all assigned schools
  (`ListAffiliatorStudents`, `StudentFilter.SchoolIDs`), narrowable by `school_id`,
  `registered_by_me` and `not_purchased`. `registerStudent` re-checks the school and records
  `users.registered_by`, so single and bulk CSV registration share one enforcement point.
- **Not available to affiliators**: status change, credential reissue (single, bulk) and set
  password (`ForbidRoles` / `student-passwords:write`), manual password and unlisted school at
  registration.
- **Deactivation is read-only, not locked out.** `canHoldSession` lets a deactivated affiliator log
  in and refresh (every other deactivated role is refused). `AffiliatorWriteGuard` on `/admin`,
  `/orders` and `/affiliates` answers non-read requests with 403 `account_deactivated`, looking the
  status up in the database; deactivating still revokes sessions, so the affiliator signs in again
  to get a read-only one.

## Enforcement chain

- `JWTMiddleware(svc, jwtSigner)` (`middleware.go:17-36`): parses the bearer token, then calls
  `svc.SessionActive(ctx, claims.ID)` — a Redis existence check on `session:access:<jti>`. A
  structurally valid, unexpired JWT is still rejected if its session was revoked (logout,
  password change/reset).
- `RBACMiddleware(required)` (`middleware.go:38-51`): reads claims set by `JWTMiddleware`,
  403s if `HasCapability` fails.
- `StudentOnlyMiddleware()` (`middleware.go:56-69`): gates on `role == student` directly, not a
  capability — students hold none, and `super_admin`'s `*` would otherwise satisfy any capability
  invented for them.
- **The `/admin` group itself has no role floor** — `admin.Use(handler.JWTMiddleware(...))` only
  (`routes.go:35`). Any authenticated role, including `student`, passes the group; access is
  entirely decided by what each subgroup opts into. Recorded as deliberate, not a gap
  ([rbac-matrix.md](../../runbooks/rbac-matrix.md) "The `/admin` floor").
- Course reads/writes are gated by one shared predicate, `canAuthorCourses(role)` = `{admin_exam,
  admin_store, super_admin}` (`service/course.go:18-20`) — `ListCourses`/`GetCourse` take a `role`
  argument and call it too, so read and write boundaries can't drift apart (a prior merge did
  exactly that; see `integration/course_role_gate_test.go`'s `TestCourseReadWriteBoundariesMatch`).
- Product type writes go through a second, separate gate: `checkTypeRBAC(role, productType)`
  (`service/store.go:1956`) on the `/admin/products` group, which carries **no** `RBACMiddleware`
  at all — the capability string `products(book|course):write` on `admin_store` is never checked
  by middleware here, only by this inline switch. The one exception is `PUT /:id/relations`,
  gated per route by `product-relations:write` (`routes.go:42`).

## School scoping (`handler/school_scope.go`)

- `resolveSchoolScope(c, claims)`: `super_admin` reads an optional `?school_id=` (empty = "every
  school", not a 400; a present value is validated as an existing UUID). Every other role is
  pinned to `claims.SchoolID` — a mismatched `?school_id=` query param is rejected with 403
  ("cannot widen school scope"), not silently ignored. A role with `SchoolID == nil` (any non-
  `admin_school` staff role) gets a 403 ("missing school scope") if a handler calls this resolver
  on it.
- `resolveResultsSchoolScope`: same, except `admin_exam` is also treated as global/unscoped — it
  owns the exam catalogue, not a school (`school_scope.go:76-85`).
- `resolveStudentSchoolScope`: the same trade for the student handlers — a role with
  `CanManageAllStudents` reads `?school_id=` (empty = every school, including registrants with no school); every other role
  falls through to `resolveSchoolScope` (`school_scope.go:88-94`). Used by list, register,
  change status, get credentials, bulk presign and bulk reissue. `resolveSchoolScope` itself is
  deliberately not widened — it also guards `/admin/dashboard/school`, which must keep 403ing
  for a role with no school on its token.
- Row-scoping is enforced in the handler/repo layer (`WHERE school_id = ...`), not by
  `RBACMiddleware` — the middleware only proves the role holds the capability, never which rows.
- `AdminSetStudentPassword` sits inside the `students:*`-gated group and additionally requires
  `student-passwords:write` per-route (`routes.go:235`) — held by `super_admin` and
  `admin_exam`, but not `admin_school`, so `admin_school` cannot reset a student's password
  despite owning the students group. `AdminChangeStudentStatus` carries no extra per-route
  gate: `students:*` alone admits it (`routes.go:236`).
- Bulk reissue still requires an explicit `school_id` (`school_id is required`, an empty global
  scope is a 400), so `admin_exam` reissues per school exactly as `super_admin` does.

## Student account audit

Every student account mutation is recorded in `audit_log` with target type `user`, for every
actor role — `super_admin`, `admin_exam` and `admin_school` — through `InsertAuditLogMeta`
(`repository/audit_log.go:134`). The actor is an explicit `actorID string` service parameter
(after `ctx`, the shape `SetStudentPassword` already used), never a context value; the
bulk-import worker takes it from the job's `created_by`.

| Action | Written by | Metadata |
|---|---|---|
| `student.register` | the shared registration path (`registerStudent`, `admin_students.go:161`), so single registrations and bulk rows are both covered | `school_id` when the row has a school |
| `student.status_change` | `ChangeStudentStatus` (`admin_students.go:457`) | `status` |
| `student.reissue_credentials` | `ReissueStudentCredentials` (`admin_students.go:479`), so the bulk reissue loop is audited per student | none |
| `student.set_password` | `SetStudentPassword` (`admin_students.go:514`) | none |

Metadata never carries a password or a temp password. An audit-write error is returned to the
caller, the same shape `account.create` uses.

## Account model

`model.User` (`internal/model/user.go`) covers both students and admins in one table/struct;
student-only fields (`Jenjang`, `DOB`, address, `TargetExam`, etc.) are `nil` for admin rows.
Relevant fields: `Role`, `SchoolID *string` (only meaningful for `admin_school` and students),
`AuthProvider` (`""`/`"local"` vs `"google"`), `Status` (`pending_verification` / `active` /
`deactivated` / `deleted`), `PasswordTemporary bool`.

## Auth methods

- **Password** — `/auth/login` (`identifier` = email or username, resolved by
  `lookupByIdentifier`, `auth.go:121-134`). `active` users get a session; `pending_verification`
  users get bounced into the OTP challenge (403 `verification_pending`); any other status is a
  flat `ErrInvalidCredentials` (no status leaked).
- **Google OAuth** — `/auth/google` → `GoogleLogin` (`service/google.go:20-60`), **student-only**:
  verifies the ID token against Google's tokeninfo endpoint, requires `aud == GoogleClientID` and
  `email_verified == "true"`. An existing account with `role != student` gets `ErrGoogleNotStudent`
  — Google sign-in cannot be used to reach an admin role even if the email matches one. A new
  Google user is created `active`, `OTPEnabled: false`, no password hash. `ChangeAccountRole`
  separately refuses to promote a passwordless Google account (`admin_users.go:187-189`,
  `ErrGoogleAccountNoPassword`) — the only path back to a password is
  `AdminResetAccountPassword`/`TriggerAccountPasswordReset`.
- **OTP** — used for (a) email verification after `/auth/register` and (b) as a second factor
  when a `pending_verification` user attempts `/auth/login`. Registration OTP-send is additionally
  rate-limited via `limitOTPSend` (Redis `SETNX`, 60s per user). OTP itself: 6-digit, 5 min TTL
  (`otp_ttl: 5m` in `config/env/*/config.yaml`), single-use (deleted on verify).

## Sessions and tokens

- Access token: a signed JWT (HS256, `infra/jwt.go`), 15 min TTL in every env
  (`access_token_ttl: 15m`). Claims: `sub`, `role`, `school_id`, `capabilities` (decorative, see
  above), `jti`.
- Refresh token: **not** a JWT — an opaque string `"<userID>-<UnixNano>"` (`auth.go:427`), looked
  up in Redis (`session:refresh:<token>` → userID), 168h (7d) TTL, single-use (deleted on refresh,
  reissued).
- Both access and refresh tokens are indexed per-user (`user_access_sessions:<uid>`,
  `user_refresh_sessions:<uid>` sets) so all of a user's sessions can be revoked together.
- Revocation is not uniform across every credential-change path:
  - `ResetPassword`, `ChangePassword` (excepting the caller's own access `jti`), and
    `SetStudentPassword` all call `revokeAllSessions` — kills every access **and** refresh session.
  - `ReissueStudentCredentials` (`admin_students.go:479-512`, used by the single and bulk
    credential-reissue endpoints) calls only `revokeRefreshSessions` — it does **not** kill
    existing access sessions. A student whose credentials were just reissued keeps any
    already-issued access token live for up to 15 more minutes. This looks like an oversight
    relative to `SetStudentPassword`'s behavior for the same underlying action (both write a new
    password hash via `ResetStudentPasswordHash`); `Unverified` whether this asymmetry is
    intentional.

## Password rules

- Min length: 8 chars (`minPasswordLen`, `auth.go:46`), enforced on register, reset, change, and
  admin-set passwords. No complexity requirement beyond length.
- Hashing: bcrypt, cost 10 in every deployed env (`productionBcryptCost`, `service/password.go:10`);
  tests force `bcrypt.MinCost` to keep the suite off the CPU-bound path.
- **Admin-issued temporary passwords**: when any admin that may register a student
  (`admin_school`, `admin_exam`, `super_admin`) omits the password, or reissues credentials
  (`ReissueStudentCredentials`), a random password is generated (`genTempPassword`,
  `admin_students.go:42-52`) and the row is flagged
  `password_temporary = true` (set inside `ResetStudentPasswordHash`,
  `repository/admin_students.go:395-398`). Any self-service password write
  (`UpdatePasswordHash`, i.e. `ResetPassword`/`ChangePassword`) clears the flag
  (`repository/user.go:132`). `super_admin`-created admin accounts and `CreateAdminAccount`
  (`admin_users.go:99-`) never set the flag — an operator-chosen admin password is treated as the
  account's real password, not temporary.
- **Forgot password**: `/auth/password/forgot` never reveals whether the email exists (returns
  200 either way, `auth.go:297-318`); on a hit it emails a 6-digit code + token, both required to
  `/auth/password/reset`. `AdminResetAccountPassword` (`/admin/system/accounts/:id/reset-password`)
  triggers the identical code+token-via-email flow against the *target's own* email
  (`TriggerAccountPasswordReset`, `admin_users.go:270-`) rather than setting a password directly —
  it fails with `ErrAccountNoEmail` for an account with no email on file. There is still no
  frontend page for the forgot-password flow (no route under `web/app` references it) — backend
  endpoints exist and are exercised only via direct API calls today.

## Login rate limiting

`LoginRateLimiter()` (`middleware.go:71-98`) wraps only `POST /auth/login` (`routes.go:19`) — not
`/auth/google`, `/auth/otp/*`, or `/auth/refresh`. Echo's in-memory limiter, per-IP
(`c.RealIP()`), `Rate: 10` tokens/**second**, `Burst: 10`, idle-eviction `ExpiresIn: 1m` (this is
bucket cleanup, not a penalty window — a burst refills in ~1s). `Retry-After` header is fixed at
`1`. Being in-memory and per-process, the limiter's state is per-instance and does not survive a
restart or get shared across replicas — a multi-pod deployment gets the configured rate multiplied
by the pod count, not a global one.

## rbac-matrix.md vs current code

The runbook's own verification step (`grep -c "admin.Group(" routes.go` → 23) no longer holds:
current `routes.go` has **28** `admin.Group(...)` calls. The 5 groups missing from the matrix
entirely: `adminSchoolsRead` (`GET /admin/schools`, `schools:read` — super_admin and admin_exam; a
sibling of row 11, which the runbook now annotates), `adminResultsWorkspace`
(`/admin/exams/:id/results-workspace`, `results-workspace:read` — super_admin and admin_exam), and
the three dashboard groups `adminDashboard` (`revenue:read`),
`adminExamDashboard` (`sessions:read`), `adminSchoolDashboard` (`students:read`, satisfied by
`admin_school`'s `students:*` via prefix match) — all under `/admin/dashboard*`
(`routes.go:368-385`). Rows 11 (`adminSchools`) and 21 (`adminExamGrants`) were updated for
`admin_exam`; the other documented rows are not wrong for the routes they cover; the
matrix is just incomplete against HEAD `a5d92558`. Everything else checked in this page (roles,
`roleCapabilities`, the C-1 course-read fix, the `admin_store` bulk-exam-orders gap) matches the
matrix as written.

## See also

- [../architecture/system-overview.md](../architecture/system-overview.md)
- [./exam-lifecycle-and-scoring.md](./exam-lifecycle-and-scoring.md)
- [../../runbooks/rbac-matrix.md](../../runbooks/rbac-matrix.md)
