# E8 — Admin accounts & access

| | |
|---|---|
| **Issue** | not filed |
| **Objective** | An admin is a first-class account holder — can maintain their own profile, signs in through a door meant for them, and reaches exactly the modules their role should reach. |
| **Source IDs** | F-2a, F-2b, F-2c |
| **Client items** | none — carried from the old register |
| **Depends on** | — |
| **Verified against** | `main` @ `d2efa3a`, 2026-07-30 |

The three items share one root: **the account model was built for students and admins were bolted
on.** Every symptom below follows from that.

---

## 1. F-2c — Google sign-in silently makes everyone a student

The strongest item, and the reason to do this epic at all. `google.go:43` hard-codes the role on
first sign-in:

```go
Role:         RoleStudent,
```

([`service/google.go:43`](../../backend/internal/service/google.go))

So an admin who clicks **Sign in with Google** on the shared `/login` page does not get a degraded
session — they get a *student account created for their address*. There is no recovery in the UI; it
takes a `UPDATE users SET role = …`.

**Decided:** split `/admin/login` from `/login`, **frontend-only and path-based, not a subdomain.**
A subdomain drags in a second certificate, a second Cloudflare origin rule and cookie-domain scoping,
for a problem that is entirely about which buttons appear on which page.

Two halves, and the second is the one that actually closes the hole:

- `/admin/login` renders credentials only — no `GoogleSignInButton`
  ([`web/components/auth/GoogleSignInButton.tsx`](../../web/components/auth/GoogleSignInButton.tsx)).
- The **backend** must stop minting accounts on an unknown Google identity, or refuse to when the
  address matches an existing non-student user. Hiding the button is presentation; on its own it
  leaves the endpoint just as reachable.

> **Open question — not yet decided.** What should Google sign-in do when the address belongs to an
> existing admin: sign them in with their real role, or refuse and send them to `/admin/login`?
> Linking is friendlier; refusing is the smaller change and cannot promote anyone by accident.

---

## 2. F-2a — admins cannot edit their own profile

Students can. There is **no admin profile route at all** — `web/app/(admin)/admin/` has `courses`,
`exam`, `notifications`, `orders`, `products`, `promos`, `revenue`, `school`, `store`, `system`, and
nothing for the signed-in user.

Scope it as *maintain your own account*, not as user administration: display name, email, password.
Reuse the student profile form rather than authoring a second one.

---

## 3. F-2b — RBAC review

**Scope before changing.** This is an audit first: enumerate every admin module and record which roles
reach it today, then decide which of those are wrong. Producing that table *is* the first deliverable —
changing a guard before it exists is how a role review turns into an outage.

Two constraints on the review:

- [E4](e4-participants-schools.md) makes exactly one role decision and documents it. It does **not**
  do this review, and this review must not silently reverse it.
- The route-split pattern already in the codebase — sibling Echo groups on one path prefix for
  read vs write — is the mechanism to reach for, not a new middleware layer.

---

## Acceptance

- An admin signing in with Google either lands with their real role or is refused — never as a new
  student. Covered by a test that fails if `RoleStudent` is reintroduced as a constant on that path.
- `/admin/login` offers no Google button; `/login` is unchanged for students.
- An admin can change their own display name, email and password, and the change survives a re-login.
- A written role × module table exists and is checked in, with any deliberate mismatch annotated.

## Out of scope

- User administration — creating, suspending or re-roling *other* accounts. Not asked for.
- Subdomain-based admin isolation. Explicitly rejected above.
- SSO or 2FA.
