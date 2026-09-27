---
title: RBAC and Accounts
updated: 2026-09-25
verified_against: e6cf227c
---

One-liner: 5 roles, a static `role → capability` map, echo middleware that checks the map on
every request, plus a school-scope resolver for the one role (`admin_school`) whose rows need
narrowing below the role level. Full route-by-route matrix: [../../runbooks/rbac-matrix.md](../../runbooks/rbac-matrix.md)
(disagreements with current code noted below — read that file, don't re-derive it here).

## Roles

`student`, `admin_exam`, `admin_store`, `admin_school`, `super_admin` — constants in
`backend/internal/service/rbac.go:6-11`. Only the 4 admin roles are assignable via
`/admin/system/accounts` (`adminRoles` map, `admin_users.go:14-19`); `student` accounts are
created only via `/auth/register` or `/auth/google`, never by an operator.

## Capabilities (`roleCapabilities`, `rbac.go:13-23`)

| Role | Capabilities |
|---|---|
| `student` | none |
| `admin_exam` | `questions:*`, `tests:*`, `products(exam):*`, `products(course):*`, `sections:*`, `sessions:*`, `uploads:write`, `results:read` |
| `admin_store` | `products(book\|course):write`, `sections:*`, `orders:*`, `promos:*`, `notifications:*`, `uploads:write` |
| `admin_school` | `students:*`, `results:read`, `bulk-exam-orders:write`, `products(exam):read` |
| `super_admin` | `*`, `schools:write` |

`admin_store` deliberately holds no `revenue:read` — aggregate revenue reporting is
`super_admin`'s; store admins still see per-order totals on the orders page (`rbac.go:16-19`
comment). `schools:write` on `super_admin` is inert — `*` already covers it.

`HasCapability(role, required)` (`rbac.go:33-53`): true if the role's list contains `"*"`, an
exact match for `required`, or an entry `x:*` where `required` starts with `x:`. This is purely
mechanical string matching — there is no notion of a capability hierarchy beyond the `:*` suffix
convention.

**The JWT's `capabilities` claim is decorative.** `mintSession` bakes `Capabilities(role)` into
the access token at sign time (`auth.go:419-420`), but `RBACMiddleware` never reads it back — it
calls `service.HasCapability(claims.Role, required)` fresh against the live map on every request
(`middleware.go:45`). A role's capabilities can change (deploy) without invalidating outstanding
tokens either way, since only `Role` — not `Capabilities` — is ever consulted.

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
  by middleware here, only by this inline switch.

## School scoping (`handler/school_scope.go`)

- `resolveSchoolScope(c, claims)`: `super_admin` reads an optional `?school_id=` (empty = "every
  school", not a 400; a present value is validated as an existing UUID). Every other role is
  pinned to `claims.SchoolID` — a mismatched `?school_id=` query param is rejected with 403
  ("cannot widen school scope"), not silently ignored. A role with `SchoolID == nil` (any non-
  `admin_school` staff role) gets a 403 ("missing school scope") if a handler calls this resolver
  on it.
- `resolveResultsSchoolScope`: same, except `admin_exam` is also treated as global/unscoped — it
  owns the exam catalogue, not a school (`school_scope.go:76-85`).
- Row-scoping is enforced in the handler/repo layer (`WHERE school_id = ...`), not by
  `RBACMiddleware` — the middleware only proves the role holds the capability, never which rows.
- `AdminSetStudentPassword` and `AdminChangeStudentStatus` sit inside the `students:*`-gated
  group but additionally require `system:admin` per-route (`routes.go:232`), which only
  `super_admin` holds — `admin_school`, despite owning the students group, cannot reset a
  student's password itself.

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
  - `ReissueStudentCredentials` (`admin_students.go:466-488`, used by the single and bulk
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
- **Admin-issued temporary passwords**: when `admin_school`/`super_admin` registers a student
  without an explicit password, or reissues credentials (`ReissueStudentCredentials`), a random
  password is generated (`genTempPassword`, `admin_students.go:42-52`) and the row is flagged
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
current `routes.go` has **27** `admin.Group(...)` calls. The 4 groups missing from the matrix
entirely: `adminResultsWorkspace` (`/admin/exams/:id/results-workspace`, `results-workspace:read`
— super_admin only), and the three dashboard groups `adminDashboard` (`revenue:read`),
`adminExamDashboard` (`sessions:read`), `adminSchoolDashboard` (`students:read`, satisfied by
`admin_school`'s `students:*` via prefix match) — all under `/admin/dashboard*`
(`routes.go:349-378`). None of the 23 documented rows are wrong for the routes they cover; the
matrix is just incomplete against HEAD `e6cf227c`. Everything else checked in this page (roles,
`roleCapabilities`, the C-1 course-read fix, the `admin_store` bulk-exam-orders gap) matches the
matrix as written.

## See also

- [../architecture/system-overview.md](../architecture/system-overview.md)
- [./exam-lifecycle-and-scoring.md](./exam-lifecycle-and-scoring.md)
- [../../runbooks/rbac-matrix.md](../../runbooks/rbac-matrix.md)
