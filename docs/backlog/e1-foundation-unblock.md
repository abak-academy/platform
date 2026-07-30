# E1 — Foundation & unblock

| | |
|---|---|
| **Issue** | [#56](https://github.com/abak-academy/platform/issues/56) |
| **Objective** | Remove the three things that block other epics. Nothing here is client-visible. |
| **Source IDs** | B-8, D-1 + one verification-debt item |
| **Client items** | none — this is enabling work |
| **Blocks** | E3, E4 (B-8) · E6 (Gotenberg) · E2 (quality gate) — **not** E5 or E7 |
| **Depends on** | — |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

> **This epic is not a vertical slice.** It has no UI and, apart from one column-free sanitiser, no
> schema. It exists because three separate pieces of later work each stall on something here.
>
> **F-6 (Amazon SES) was removed from this epic on 2026-07-30** — the platform stays on the existing
> Hostinger SMTP for now. It moved to [E7](e7-scale-event-readiness.md), where the event that forces
> it lives. Runbook kept at [`ses-email-migration.md`](ses-email-migration.md).

---

## 1. B-8 — CSV formula injection, three writers

No sanitiser exists anywhere in `backend/`. Three writers emit attacker-supplied student names
straight into CSV:

| Writer | File |
|---|---|
| Exam results export | [`admin_results.go:135`](../../backend/internal/service/admin_results.go) |
| Bulk credentials | [`bulk_credentials.go:96`](../../backend/internal/service/bulk_credentials.go) |
| Student bulk import report | [`student_bulk.go:338`](../../backend/internal/service/student_bulk.go) |

Student names are attacker-supplied at registration. The frontend roster export was fixed long ago
(C-08); the backend never was.

**Why it gates two epics.** `admin_results.go` is the writer behind the results tab **E3** builds a UI
for — shipping the tab first puts a nicer front door on the hole. And **E4**'s school importer is
explicitly "copy the student bulk pattern", which means copying `student_bulk.go` verbatim,
vulnerability included.

**Fix** — mirror C-08: prefix `'` and force-quote any field starting with `= + - @ \t \r`.

**Acceptance**
- A student named `=cmd|' /C calc'!A0` exports as inert text from all three writers.
- One test per writer, each mutation-checked (breaks when the sanitiser is removed).

---

## 2. D-1 — Storage seam, then delete the shims

`internal/service/ports_storage.go` defines a `StorageClient` port that nothing uses. `Service.storage`
is still a concrete `*minio.Client` ([`service.go:28`](../../backend/internal/service/service.go)), and
`storeRepo` is concrete too. With no seam, service tests hand-copy production logic into shims and
assert against the copy:

| File | shim methods |
|---|---|
| `course_test.go` | 18 |
| `store_test.go` | 14 |
| `exam_session_test.go` | 10 |
| `certificate_test.go` | 5 |
| `exam_result_test.go` | 4 |
| `exam_test.go` | 4 |
| `admin_results_test.go`, `exam_leaderboard_test.go` | 3 each |
| `student_test.go` | 1 |
| **total** | **62** |

**26 of those 62 sit in the exact files E2 and E3 touch.** Build the regrade engine on top and its
tests pass against hand-copied logic, not shipping code. Regrade decides scores; a tautological test
there is worse than no test, because it reads as coverage.

**Honest framing:** this is a *quality* gate, not a compile-time one. E2's regrade can physically be
built without the seam. What you lose is any reason to trust its tests.

**Two PRs, in order:** wire the seam, then delete the shims. Do not graft either onto a feature
branch — mixing a mechanical refactor with behavioural change makes both unreviewable.

**Acceptance**
- Shim count is zero.
- Full backend suite green.
- At least one previously shimmed test is mutation-checked to prove it now fails when production code
  breaks.

---

## 3. Prove the Gotenberg sidecar renders

The sidecar is up on staging and health-checks report chromium `up`, but **it has never actually
rendered a document end to end**. Cached certificates masked its total absence for weeks and the api
logs stayed silent throughout.

**E6** renders the shipping label through it. Building on infrastructure with zero proof is how that
epic discovers the problem late.

Not a byte assertion — an actual PDF, opened and looked at. This project has already shipped a
certificate that rendered fully upside-down while byte-level tests stayed green.

**Acceptance**
- A certificate or exam card produced by the staging sidecar, attached to the session notes, visually
  checked.

---

## Out of scope

- Anything client-visible. If a change here shows up in the UI, it belongs to another epic.
- D-5 (gofmt gate) — tempting while touching Go, but 37 files must be reformatted in one commit first
  or the gate lands red. Separate change.
